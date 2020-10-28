// Package fetcher defines and implement the downloading and parsing utilities
// for remote resources
package fetcher

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/PuerkitoBio/rehttp"
)

// Fetcher is an interface exposing methods to fetch resources, Fetch enable
// download and parsing of resources based on the `Parser` instance passed in,
// Download allow to just download raw contents.
type Fetcher interface {
	// Fetch makes an HTTP GET request to an URL returning a `*http.Response` or
	// any error occured
	Fetch(string) (time.Duration, *http.Response, error)
	// FetchLinks makes an HTTP GET request to an URL, parse the HTML in the
	// response and returns an array of URLs or any error occured
	FetchLinks(string) (time.Duration, []*url.URL, error)
}

// stdHttpFetcher is a simple Fetcher with std library http.Client as a
// backend for HTTP requests.
type stdHttpFetcher struct {
	userAgent string
	parser    Parser
	client    *http.Client
}

// New create a new Fetcher specifying a timeout and a concurrency level.
// 0 concurrency means an unbounded Fetcher. By default it retries when
// a temporary error occurs (most temporary errors are HTTP ones) for a
// specified number of times by applying an exponential backoff strategy.
func New(userAgent string, parser Parser, timeout time.Duration) Fetcher {
	transport := rehttp.NewTransport(
		&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		rehttp.RetryAll(rehttp.RetryMaxRetries(3), rehttp.RetryTemporaryErr()),
		rehttp.ExpJitterDelay(1, 10*time.Second),
	)
	client := &http.Client{Timeout: timeout, Transport: transport}
	return stdHttpFetcher{userAgent, parser, client}
}

// Parse an URL extracting the protion <scheme>://<host>:<port>
// Returns a string with the base domain of the URL
func parseStartURL(u string) string {
	parsed, _ := url.Parse(u)
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

// Fetch is a private function used to make a single HTTP GET request
// toward an URL.
// It returns an `*http.Response` or any error occured during the call.
func (f stdHttpFetcher) Fetch(url string) (time.Duration, *http.Response, error) {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return time.Duration(0), nil, err
	}
	req.Header.Set("User-Agent", f.userAgent)
	// We want to time the request
	start := time.Now()
	res, err := f.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return elapsed, nil, err
	}

	return elapsed, res, nil
}

// Fetch contact and download raw data from a specified URL and parse the
// content into a `ParserResult` struct.
// It returns a `*ParserResult` or any error occuring during the call or the
// parsing of the results.
func (f stdHttpFetcher) FetchLinks(targetURL string) (time.Duration, []*url.URL, error) {
	if f.parser == nil {
		return time.Duration(0), nil, fmt.Errorf("fetching links from %s failed: no parser set", targetURL)
	}
	// Extract base domain from the url
	baseDomain := parseStartURL(targetURL)

	elapsed, resp, err := f.Fetch(targetURL)
	if err != nil || resp.StatusCode >= http.StatusBadRequest {
		return elapsed, nil, fmt.Errorf("fetching links from %s failed: %w", targetURL, err)
	}
	defer resp.Body.Close()

	links, err := f.parser.Parse(baseDomain, resp.Body)
	if err != nil {
		return elapsed, nil, fmt.Errorf("fetching links from %s failed: %w", targetURL, err)
	}
	return elapsed, links, nil
}
