package fetcher

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func serverMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/foo/bar", resourceMock)

	server := httptest.NewServer(handler)
	return server
}

func resourceMock(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(
		`<head>
			<link rel="canonical" href="https://example.com/sample-page/" />
			<link rel="canonical" href="/sample-page/" />
		 </head>
		 <body>
			<a href="foo/bar"><img src="/baz.png"></a>
			<img src="/stonk">
			<a href="foo/bar">
		 </body>`,
	))
}

func TestStdHttpFetcherFetch(t *testing.T) {
	server := serverMock()
	defer server.Close()
	f := New("test-agent", nil, 10*time.Second)
	target := fmt.Sprintf("%s/foo/bar", server.URL)
	_, res, err := f.Fetch(target)
	if err != nil {
		t.Errorf("StdHttpFetcher#Fetch failed: %v", err)
	}
	if res.StatusCode != 200 {
		t.Errorf("StdHttpFetcher#Fetch failed: %#v", res)
	}
	_, res, err = f.Fetch("testUrl")
	if err == nil {
		t.Errorf("StdHttpFetcher#Fetch failed: %v", err)
	}
}

func TestStdHttpFetcherFetchLinks(t *testing.T) {
	server := serverMock()
	defer server.Close()
	f := New("test-agent", NewGoqueryParser(), 10*time.Second)
	target := fmt.Sprintf("%s/foo/bar", server.URL)
	firstLink, _ := url.Parse("https://example.com/sample-page/")
	secondLink, _ := url.Parse(server.URL + "/sample-page/")
	thirdLink, _ := url.Parse(server.URL + "/foo/bar")
	expected := []*url.URL{firstLink, secondLink, thirdLink}
	_, res, err := f.FetchLinks(target)
	if err != nil {
		t.Errorf("StdHttpFetcher#FetchLinks failed: expected %v got %v", expected, err)
	}
	if !reflect.DeepEqual(res, expected) {
		t.Errorf("StdHttpFetcher#FetchLinks failed: expected %v got %v", expected, res)
	}
}
