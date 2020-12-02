// Package crawler containing the crawling logics and utilities to scrape
// remote resources on the web
package crawler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func serverMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(
			`User-agent: *
	Disallow: */baz/*
	Crawl-delay: 2`,
		))
	})

	server := httptest.NewServer(handler)
	return server
}

func serverWithoutCrawlingRules() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(handler)
	return server
}

func TestCrawlingRules(t *testing.T) {
	server := serverMock()
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	r := NewCrawlingRules(serverURL, newMemoryCache(), 100*time.Millisecond)
	testLink, _ := url.Parse(server.URL + "/foo/baz/bar")
	if !r.Allowed(testLink) {
		t.Errorf("CrawlingRules#IsAllowed failed: expected true got false")
	}
	r.GetRobotsTxtGroup("test-agent", serverURL)
	if r.Allowed(testLink) {
		t.Errorf("CrawlingRules#IsAllowed failed: expected false got true")
	}
	if r.CrawlDelay() != 2*time.Second {
		t.Errorf("CrawlingRules#CrawlDelay failed: expected 2 got %d", r.CrawlDelay())
	}
}

func TestCrawlingRulesNotFound(t *testing.T) {
	server := serverWithoutCrawlingRules()
	defer server.Close()
	serverURL, _ := url.Parse(server.URL)
	r := NewCrawlingRules(serverURL, newMemoryCache(), 100*time.Millisecond)
	if r.GetRobotsTxtGroup("test-agent", serverURL) {
		t.Errorf("CrawlingRules#GetRobotsTxtGroup failed")
	}
}
