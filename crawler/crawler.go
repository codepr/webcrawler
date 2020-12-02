// Package crawler containing the crawling logics and utilities to scrape
// remote resources on the web
package crawler

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/codepr/webcrawler/crawler/env"
	"github.com/codepr/webcrawler/crawler/fetcher"
	"github.com/codepr/webcrawler/messaging"
)

const (
	// Default fetcher timeout before giving up an URL
	defaultFetchTimeout time.Duration = 10 * time.Second
	// Default crawling timeout, time to wait to stop the crawl after no links are
	// found
	defaultCrawlingTimeout time.Duration = 30 * time.Second
	// Default politeness delay, fixed delay to calculate a randomized wait time
	// for subsequent HTTP calls to a domain
	defaultPolitenessDelay time.Duration = 500 * time.Millisecond
	// Default depth to crawl for each domain
	defaultDepth int = 16
	// Default number of concurrent goroutines to crawl
	defaultConcurrency int = 8
	// Default user agent to use
	defaultUserAgent string = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
)

// ParsedResult contains the URL crawled and an array of links found, json
// serializable to be sent on message queues
type ParsedResult struct {
	URL   string   `json:"url"`
	Links []string `json:"links"`
}

// CrawlerSettings represents general settings for the crawler and his
// dependencies
type CrawlerSettings struct {
	// FetchingTimeout is the time to wait before closing a connection that does not
	// respond
	FetchingTimeout time.Duration
	// CrawlingTimeout is the number of second to wait before exiting the crawling
	// in case of no links found
	CrawlingTimeout time.Duration
	// Concurrency is the number of concurrent goroutine to run while fetching
	// a page. 0 means unbounded
	Concurrency int
	// Parser is a `fetcher.Parser` instance object used to parse fetched pages
	Parser fetcher.Parser
	// Cachable to be used as visit tracker for each domain crawled
	Cache Cachable
	// MaxDepth represents a limit on the number of pages recursively fetched.
	// 0 means unlimited
	MaxDepth int
	// UserAgent is the user-agent header set in each GET request, most of the
	// times it also defines which robots.txt rules to follow while crawling a
	// domain, depending on the directives specified by the site admin
	UserAgent string
	// PolitenessFixedDelay represents the delay to wait between subsequent
	// calls to the same domain, it'll taken into consideration against a
	// robots.txt if present and against the last response time, taking always
	// the major between these last two. Robots.txt has the precedence.
	PolitenessFixedDelay time.Duration
}

// CrawlerOpt is a type definition for option pattern while creating a new
// crawler
type CrawlerOpt func(*CrawlerSettings)

// WebCrawler is the main object representing a crawler
type WebCrawler struct {
	// logger is a private logger instance
	logger *log.Logger
	// queue is a simple message queue to forward crawling results to other
	// components of the architecture, decoupling business logic from processing,
	// storage or presentation layers
	queue messaging.Producer
	// settings is a pointer to `CrawlerSettings` containing some crawler
	// specifications
	settings *CrawlerSettings
}

// New create a new Crawler instance, accepting a maximum level of depth during
// crawling all the anchor links inside each page, a concurrency limiter that
// defines how many goroutine to run in parallel while fetching links and a
// timeout for each HTTP call.
func New(userAgent string,
	queue messaging.Producer, opts ...CrawlerOpt) *WebCrawler {
	// Default crawler settings
	settings := &CrawlerSettings{
		FetchingTimeout:      defaultFetchTimeout,
		Parser:               fetcher.NewGoqueryParser(),
		Cache:                newMemoryCache(),
		UserAgent:            userAgent,
		CrawlingTimeout:      defaultCrawlingTimeout,
		PolitenessFixedDelay: defaultPolitenessDelay,
		Concurrency:          defaultConcurrency,
	}

	// Mix in all optionals
	for _, opt := range opts {
		opt(settings)
	}

	crawler := &WebCrawler{
		logger:   log.New(os.Stderr, "crawler: ", log.LstdFlags),
		queue:    queue,
		settings: settings,
	}

	return crawler
}

// NewFromEnv create a new webCrawler by reading values from environment
func NewFromEnv(queue messaging.Producer, opts ...CrawlerOpt) *WebCrawler {
	crawler := New(env.GetEnv("USERAGENT", defaultUserAgent), queue,
		func(s *CrawlerSettings) {
			s.MaxDepth = env.GetEnvAsInt("MAX_DEPTH", defaultDepth)
			s.FetchingTimeout = time.Duration(env.GetEnvAsInt("FETCHING_TIMEOUT", 10)) * time.Second
			s.Concurrency = env.GetEnvAsInt("CONCURRENCY", 1)
			s.CrawlingTimeout = time.Duration(env.GetEnvAsInt("CRAWLING_TIMEOUT", 30)) * time.Second
			s.PolitenessFixedDelay = time.Duration(env.GetEnvAsInt("POLITENESS_DELAY", 500)) * time.Millisecond
		})
	// Mix in all optionals
	for _, opt := range opts {
		opt(crawler.settings)
	}
	return crawler
}

// NewFromSettings create a new webCrawler with the settings passed in
func NewFromSettings(queue messaging.ChannelQueue, settings *CrawlerSettings) *WebCrawler {
	return &WebCrawler{
		queue:    queue,
		logger:   log.New(os.Stderr, "crawler: ", log.LstdFlags),
		settings: settings,
	}
}

// Crawl a single page by fetching the starting URL, extracting all anchors
// and exploring each one of them applying the same steps. Every image link
// found is forwarded into a dedicated channel, as well as errors.
//
// A waitgroup is used to synchronize it's execution, enabling the caller to
// wait for completion.
func (c *WebCrawler) crawlPage(rootURL *url.URL, wg *sync.WaitGroup, ctx context.Context) {
	// First we wanna make sure we decrease the waitgroup counter at the end of
	// the crawling
	defer wg.Done()
	fetchClient := fetcher.New(c.settings.UserAgent,
		c.settings.Parser, c.settings.FetchingTimeout)

	var (
		// semaphore is just a value-less channel used to limit the number of
		// concurrent goroutine workers fetching links
		semaphore chan struct{}
		// New found links channel
		linksCh chan []*url.URL
		stop    bool
		depth   int
		fetchWg sync.WaitGroup = sync.WaitGroup{}
		// An atomic counter to make sure that we've already crawled all remaining
		// links if a timeout occur. Initialized at 1 as it's counting the start URL
		// before crawling all subdomains.
		linkCounter int32 = 1
	)

	// Set the concurrency level by using a buffered channel as semaphore
	if c.settings.Concurrency > 0 {
		semaphore = make(chan struct{}, c.settings.Concurrency)
		linksCh = make(chan []*url.URL, c.settings.Concurrency)
	} else {
		// we want to disallow the unlimited concurrency, to avoid being banned from
		// the ccurrent crawled domain and also to avoid running OOM or running out
		// of unix file descriptors, as each HTTP call is built upon a  socket
		// connection, which is in-fact an opened descriptor.
		semaphore = make(chan struct{}, 1)
		linksCh = make(chan []*url.URL, 1)
	}

	// Just a kickstart for the first URL to scrape
	linksCh <- []*url.URL{rootURL}
	// We try to fetch a robots.txt rule to follow, being polite to the
	// domain
	crawlingRules := NewCrawlingRules(rootURL,
		c.settings.Cache, c.settings.PolitenessFixedDelay)
	if crawlingRules.GetRobotsTxtGroup(c.settings.UserAgent, rootURL) {
		c.logger.Printf("Found a valid %s/robots.txt", rootURL.Host)
	} else {
		c.logger.Printf("No valid %s/robots.txt found", rootURL.Host)
	}

	// Every cycle represents a single page crawling, when new anchors are
	// found, the counter is increased, making the loop continue till the
	// end of links
	for !stop {
		select {
		case links := <-linksCh:
			for _, link := range links {
				// Skip already visited links or disallowed ones by the robots.txt rules
				if !crawlingRules.Allowed(link) {
					atomic.AddInt32(&linkCounter, -1)
					continue
				}
				// Spawn a goroutine to fetch the link, throttling by
				// concurrency argument on the semaphore will take care of the
				// concurrent number of goroutine.
				fetchWg.Add(1)
				go func(link *url.URL, stopSentinel bool, w *sync.WaitGroup) {
					defer w.Done()
					defer atomic.AddInt32(&linkCounter, -1)
					// 0 concurrency level means we serialize calls as
					// goroutines are cheap but not that cheap (around 2-5 kb
					// each, 1 million links = ~4/5 GB ram), by allowing for
					// unlimited number of workers, potentially we could run
					// OOM (or banned from the website) really fast
					semaphore <- struct{}{}
					defer func() {
						time.Sleep(crawlingRules.CrawlDelay())
						<-semaphore
					}()
					// We fetch the current link here and parse HTML for children links
					responseTime, foundLinks, err := fetchClient.FetchLinks(link.String())
					crawlingRules.UpdateLastDelay(responseTime)
					if err != nil {
						c.logger.Println(err)
						return
					}
					// No errors occured, we want to enqueue all scraped links
					// to the link queue
					if stopSentinel || foundLinks == nil || len(foundLinks) == 0 {
						return
					}
					atomic.AddInt32(&linkCounter, int32(len(foundLinks)))
					// Send results from fetch process to the processing queue
					c.enqueueResults(link, foundLinks)
					// Enqueue found links for the next cycle
					linksCh <- foundLinks

				}(link, stop, &fetchWg)
				// We want to check if a level limit is set and in case, check if
				// it's reached as every explored link count as a level
				if c.settings.MaxDepth == 0 || !stop {
					depth++
					stop = c.settings.MaxDepth > 0 && depth >= c.settings.MaxDepth
				}
			}
		case <-time.After(c.settings.CrawlingTimeout):
			// c.settings.CrawlingTimeout seconds without any new link found, check
			// that the remaining links have been processed and stop the iteration
			if atomic.LoadInt32(&linkCounter) <= 0 {
				stop = true
			}
		case <-ctx.Done():
			return
		}
	}
	fetchWg.Wait()
}

// enqueueResults enqueue fetched links through the Producer queue in order to
// be processed (in this case, printe to stdout)
func (c *WebCrawler) enqueueResults(link *url.URL, foundLinks []*url.URL) {
	foundLinksStr := []string{}
	for _, l := range foundLinks {
		foundLinksStr = append(foundLinksStr, l.String())
	}
	payload, _ := json.Marshal(ParsedResult{link.String(), foundLinksStr})
	if err := c.queue.Produce(payload); err != nil {
		c.logger.Println("Unable to communicate with message queue:", err)
	}
}

// Crawl will walk through a list of URLs spawning a goroutine for each one of
// them
func (c *WebCrawler) Crawl(URLs ...string) {
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	// Sanity check for URLs passed, check that they're in the form
	// scheme://host:port/path, adding missing fields
	for _, href := range URLs {
		url, err := url.Parse(href)
		if err != nil {
			c.logger.Fatal(err)
		}
		if url.Scheme == "" {
			url.Scheme = "https"
		}
		// Spawn a goroutine for each URLs to crawl, a waitgroup is used to wait
		// for completion
		wg.Add(1)
		go c.crawlPage(url, &wg, ctx)
	}
	// Graceful shutdown of workers
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		cancel()
		os.Exit(1)
	}()
	wg.Wait()
	c.logger.Println("Crawling done")
}
