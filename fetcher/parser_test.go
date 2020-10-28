package fetcher

import (
	"bytes"
	"net/url"
	"reflect"
	"testing"
)

func TestGoqueryParsePage(t *testing.T) {
	parser := NewGoqueryParser()
	firstLink, _ := url.Parse("http://localhost:8787/sample-page/")
	secondLink, _ := url.Parse("http://localhost:8787/foo/bar")
	expected := []*url.URL{firstLink, secondLink}
	content := bytes.NewBufferString(
		`<head>
			<link rel="canonical" href="https://example.com/sample-page/" />
			<link rel="canonical" href="http://localhost:8787/sample-page/" />
		 </head>
		 <body>
			<a href="foo/bar"><img src="/baz.png"></a>
			<img src="/stonk">
			<a href="foo/bar">
		</body>`,
	)
	res, err := parser.Parse("http://localhost:8787", content)
	if err != nil {
		t.Errorf("GoqueryParser#ParsePage failed: expected %v got %v", expected, err)
	}
	if !reflect.DeepEqual(res, expected) {
		t.Errorf("GoqueryParser#ParsePage failed: expected %v got %v", expected, res)
	}
}
