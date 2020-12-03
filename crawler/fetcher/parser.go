// Package fetcher defines and implement the fetching and parsing utilities
// for remote resources
package fetcher

import (
	"io"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// GoqueryParser is just an algorithm `Parser` definition that uses
// `github.com/PuerkitoBio/goquery` as a backend library
type GoqueryParser struct {
	excludedExts map[string]bool
	seen         *sync.Map
}

// NewGoqueryParser create a new parser with goquery as backend
func NewGoqueryParser() GoqueryParser {
	return GoqueryParser{
		excludedExts: make(map[string]bool),
		seen:         new(sync.Map),
	}
}

// ExcludeExtensions add extensions to be excluded to the default exclusion
// pool
func (p *GoqueryParser) ExcludeExtensions(exts ...string) {
	for _, ext := range exts {
		p.excludedExts[ext] = true
	}
}

// Parse is the implementation of the `Parser` interface for the
// `GoqueryParser` struct, read the content of an `io.Reader` (e.g.
// any file-like streamable object) and extracts all anchor links.
// It returns a `ParserResult` object or any error that arises from the goquery
// call on the data read.
func (p GoqueryParser) Parse(baseURL string, reader io.Reader) ([]*url.URL, error) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}
	links := p.extractLinks(doc, baseURL)
	return links, nil
}

// extractLinks retrieves all anchor links inside a `goquery.Document`
// representing an HTML content.
// It returns a slice of string containing all the extracted links or `nil` if\
// the passed document is a `nil` pointer.
func (p *GoqueryParser) extractLinks(doc *goquery.Document, baseURL string) []*url.URL {
	if doc == nil {
		return nil
	}
	foundURLs := []*url.URL{}
	doc.Find("a,link").FilterFunction(func(i int, element *goquery.Selection) bool {
		hrefLink, hrefExists := element.Attr("href")
		linkType, linkExists := element.Attr("rel")
		anchorOk := hrefExists && !p.excludedExts[filepath.Ext(hrefLink)]
		linkOk := linkExists && linkType == "canonical" && !p.excludedExts[filepath.Ext(linkType)]
		return anchorOk || linkOk
	}).Each(func(i int, element *goquery.Selection) {
		res, _ := element.Attr("href")
		if link, ok := resolveRelativeURL(baseURL, res); ok {
			if present, _ := p.seen.LoadOrStore(link.String(), false); !present.(bool) {
				foundURLs = append(foundURLs, link)
				p.seen.Store(link.String(), true)
			}
		}
	})
	return foundURLs
}

// resolveRelativeURL just correctly join a base domain to a relative path
// to produce an absolute path to fetch on.
// It returns a tuple, a string representing the absolute path with resolved
// paths and a boolean representing the success or failure of the process.
func resolveRelativeURL(baseURL string, relative string) (*url.URL, bool) {
	u, err := url.Parse(relative)
	if err != nil {
		return nil, false
	}
	if u.Hostname() != "" {
		return u, true
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, false
	}

	return base.ResolveReference(u), true
}
