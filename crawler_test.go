// Package crawler containing the crawling logics and utilities to scrape
// remote resources
package crawler

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

type testQueue struct {
	bus chan []byte
}

func (t testQueue) Produce(data []byte) error {
	t.bus <- data
	return nil
}

func (t testQueue) Consume(events chan<- []byte) error {
	for event := range t.bus {
		events <- event
	}
	return nil
}

func (t testQueue) Close() {
	close(t.bus)
}

func consumeEvents(queue *testQueue) []ParsedResult {
	wg := sync.WaitGroup{}
	events := make(chan []byte)
	results := []ParsedResult{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for e := range events {
			var res ParsedResult
			if err := json.Unmarshal(e, &res); err == nil {
				results = append(results, res)
			}
		}
	}()
	_ = queue.Consume(events)
	close(events)
	wg.Wait()
	return results
}

func serverMockWithoutRobotsTxt() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/foo", resourceMock(
		`<head>
			<link rel="canonical" href="https://example-page.com/sample-page/" />
		 </head>
		 <body>
			<img src="/baz.png">
			<img src="/stonk">
			<a href="foo/bar/baz">
		</body>`,
	))
	handler.HandleFunc("/foo/bar/baz", resourceMock(
		`<head>
			<link rel="canonical" href="https://example-page.com/sample-page/" />
			<link rel="canonical" href="/foo/bar/test" />
		 </head>
		 <body>
			<img src="/baz.png">
			<img src="/stonk">
		</body>`,
	))
	handler.HandleFunc("/foo/bar/test", resourceMock(
		`<head>
			<link rel="canonical" href="https://example-page.com/sample-page/" />
		 </head>
		 <body>
			<img src="/stonk">
		</body>`,
	))

	server := httptest.NewServer(handler)
	return server
}

func serverMockWithRobotsTxt() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/robots.txt", resourceMock(
		`User-agent: *
	Disallow: */test
	Crawl-delay: 1`,
	))
	handler.HandleFunc("/", resourceMock(
		`<head>
			<link rel="canonical" href="https://example-page.com/sample-page/" />
		 </head>
		 <body>
			<img src="/baz.png">
			<img src="/stonk">
			<a href="foo/bar/baz">
		</body>`,
	))
	handler.HandleFunc("/foo/bar/baz", resourceMock(
		`<head>
			<link rel="canonical" href="https://example-page.com/sample-page/" />
			<link rel="canonical" href="/foo/bar/test" />
		 </head>
		 <body>
			<img src="/baz.png">
			<img src="/stonk">
		</body>`,
	))
	handler.HandleFunc("/foo/bar/test", resourceMock(
		`<head>
			<link rel="canonical" href="https://example-page.com/sample-page/" />
		 </head>
		 <body>
			<img src="/stonk">
		</body>`,
	))

	server := httptest.NewServer(handler)
	return server
}

func resourceMock(content string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	}
}

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func withMaxDepth(depth int) CrawlerOpt {
	return func(s *CrawlerSettings) {
		s.MaxDepth = depth
	}
}

func withCrawlingTimeout(timeout time.Duration) CrawlerOpt {
	return func(s *CrawlerSettings) {
		s.CrawlingTimeout = timeout
	}
}

func TestCrawlPages(t *testing.T) {
	server := serverMockWithoutRobotsTxt()
	defer server.Close()
	testbus := testQueue{make(chan []byte)}
	results := make(chan []ParsedResult)
	go func() { results <- consumeEvents(&testbus) }()
	crawler := New("test-agent", &testbus, withCrawlingTimeout(100*time.Millisecond))
	crawler.Crawl(server.URL + "/foo")
	time.Sleep(1)
	testbus.Close()
	time.Sleep(1)
	res := <-results
	time.Sleep(1)
	close(results)
	expected := []ParsedResult{
		{
			server.URL + "/foo",
			[]string{"https://example-page.com/sample-page/", server.URL + "/foo/bar/baz"},
		},
		{
			server.URL + "/foo/bar/baz",
			[]string{server.URL + "/foo/bar/test"},
		},
	}
	if !reflect.DeepEqual(res, expected) {
		t.Errorf("Crawler#Crawl failed: expected %v got %v", expected, res)
	}
}

func TestCrawlPagesRespectingRobotsTxt(t *testing.T) {
	server := serverMockWithRobotsTxt()
	defer server.Close()
	testbus := testQueue{make(chan []byte)}
	results := make(chan []ParsedResult)
	go func() { results <- consumeEvents(&testbus) }()
	crawler := New("test-agent", &testbus, withCrawlingTimeout(100*time.Millisecond))
	crawler.Crawl(server.URL)
	testbus.Close()
	res := <-results
	expected := []ParsedResult{
		{
			server.URL,
			[]string{"https://example-page.com/sample-page/", server.URL + "/foo/bar/baz"},
		},
		{
			server.URL + "/foo/bar/baz",
			[]string{server.URL + "/foo/bar/test"},
		},
	}
	if !reflect.DeepEqual(res, expected) {
		t.Errorf("Crawler#Crawl failed: expected %v got %v", expected, res)
	}
}

func TestCrawlPagesRespectingMaxDepth(t *testing.T) {
	server := serverMockWithoutRobotsTxt()
	defer server.Close()
	testbus := testQueue{make(chan []byte)}
	results := make(chan []ParsedResult)
	go func() { results <- consumeEvents(&testbus) }()
	crawler := New("test-agent", &testbus, withCrawlingTimeout(100*time.Millisecond), withMaxDepth(3))
	crawler.Crawl(server.URL + "/foo")
	testbus.Close()
	res := <-results
	expected := []ParsedResult{
		{
			server.URL + "/foo",
			[]string{"https://example-page.com/sample-page/", server.URL + "/foo/bar/baz"},
		},
		{
			server.URL + "/foo/bar/baz",
			[]string{server.URL + "/foo/bar/test"},
		},
	}
	if !reflect.DeepEqual(res, expected) {
		t.Errorf("Crawler#Crawl failed: expected %v got %v", expected, res)
	}
}
