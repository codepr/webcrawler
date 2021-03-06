// Package crawler containing the crawling logics and utilities to scrape
// remote resources on the web
package crawler

import (
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

// Cachable defines the behavior expected by a simple cache, for now just to
// track already visited links.
// We won't use interface{} as types as we're reasonably sure that we'd use
// the implementations just to track URL in string form
type Cachable interface {
	Set(string, string)
	Contains(string, string) bool
}

// Default /robots.txt path on server
const robotsTxtPath string = "/robots.txt"

// CrawlingRules contains the rules to be obeyed during the crawling of a single
// domain, including allowances and delays to respect.
//
// There are a total of 3 different delays for each domain, the robots.txt has
// always the precedence over the fixedDelay and the lastDelay.
// If no robots.txt is found during the crawl, a random delay will be calculated
// based on the response time of the last request, if a fixedDelay is set, the
// major between a random value between 1.5 * fixedDelay and 0.5 * fixedDelay
// and the lastDelay will be chosen.
type CrawlingRules struct {
	// baseDomain represents the domain where we start the crawling process
	baseDomain *url.URL
	// Cachable store, just to keep track of visited URLs
	cache Cachable
	// temoto/robotstxt backend is used to fetch the robotsGroup from the
	// robots.txt file
	robotsGroup *robotstxt.Group
	// A fixed delay to respect on each request if no valid robots.txt is found
	fixedDelay time.Duration
	// The delay of the last request, useful to calculate a new delay for the
	// next request
	lastDelay time.Duration
	// A RWmutex is needed to make the delya calculation threadsafe as this
	// struct will be shared among multiple goroutines
	rwMutex sync.RWMutex
}

// NewCrawlingRules creates a new CrawlingRules struct
func NewCrawlingRules(baseDomain *url.URL, cache Cachable,
	fixedDelay time.Duration) *CrawlingRules {
	return &CrawlingRules{
		baseDomain: baseDomain,
		cache:      cache,
		fixedDelay: fixedDelay,
	}
}

// Allowed tests for eligibility of an URL to be crawled, based on the rules
// of the robots.txt file on the server. If no valid robots.txt is found all
// URLs in the domain are assumed to be allowed, returning true.
func (r *CrawlingRules) Allowed(url *url.URL) bool {
	if r.cache.Contains(r.baseDomain.String(), url.String()) {
		return false
	}
	defer r.cache.Set(r.baseDomain.String(), url.String())
	if r.robotsGroup != nil {
		return r.robotsGroup.Test(url.RequestURI()) && subdomain(r.baseDomain, url)
	}
	return subdomain(r.baseDomain, url)
}

// CrawlDelay return the delay to be respected for the next request on a same
// domain. It chooses from 3 different possible delays, the most important one
// is the one defined by the robots.txt of the domain, then it proceeds
// generating a random delay based on the last request response time and a
// fixed delay set by configuration of the crawler.
//
// It follows these steps:
//
// - robots.txt delay
// - delay = random 0.5*fixedDelay and 1.5*fixedDelay
// - max(lastResponseTime^2, delay, robots.txt delay)
func (r *CrawlingRules) CrawlDelay() time.Duration {
	r.rwMutex.RLock()
	defer r.rwMutex.RUnlock()
	var delay time.Duration
	if r.robotsGroup != nil {
		delay = r.robotsGroup.CrawlDelay
	}
	// We calculate a random value: 0.5*fixedDelay < value < 1.5*fixedDelay
	randomDelay := randDelay(int64(r.fixedDelay.Milliseconds())) * time.Millisecond
	baseDelay := time.Duration(
		math.Max(float64(randomDelay.Milliseconds()), float64(delay.Milliseconds())),
	) * time.Millisecond
	// We return the max between the random value calculated and the lastDelay
	return time.Duration(
		math.Max(float64(r.lastDelay.Milliseconds()), float64(baseDelay.Milliseconds())),
	) * time.Millisecond
}

// SetDelay just pow(2) the lastTime response in seconds and set it as the
// lastDelay value
func (r *CrawlingRules) UpdateLastDelay(lastResponseTime time.Duration) {
	r.rwMutex.Lock()
	r.lastDelay = time.Duration(
		math.Pow(float64(lastResponseTime.Seconds()), 2.0),
	) * time.Second
	r.rwMutex.Unlock()
}

// GetRobotsTxtGroup tryes to fetch the robots.txt from the domain and parse
// it. Returns a boolean based on the success of the process.
func (r *CrawlingRules) GetRobotsTxtGroup(f Fetcher,
	userAgent string, domain *url.URL) bool {
	u, _ := url.Parse(robotsTxtPath)
	targetURL := domain.ResolveReference(u)
	// Try to fetch the robots.txt file
	_, res, err := f.Fetch(targetURL.String())
	if err != nil || res.StatusCode == http.StatusNotFound {
		return false
	}
	body, err := robotstxt.FromResponse(res)

	// If robots data cannot be parsed, will return nil, which will allow access by default.
	// Reasonable, since by default no robots.txt means full access, so invalid
	// robots.txt is similar behavior.
	if err != nil {
		return false
	}
	r.robotsGroup = body.FindGroup(userAgent)
	return r.robotsGroup != nil
}

// Return a random value between 1.5*value and 0.5*value
func randDelay(value int64) time.Duration {
	if value == 0 {
		return 0
	}
	max, min := 1.5*float64(value), 0.5*float64(value)
	return time.Duration(rand.Int63n(int64(max-min)) + int64(max))
}

func subdomain(domain *url.URL, link *url.URL) bool {
	return (link.Hostname() == domain.Hostname() || link.Hostname() == "")
}
