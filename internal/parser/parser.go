// Package parser extracts page information from an HTML document.
package parser

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Page holds the raw data extracted from one HTML document.
type Page struct {
	Title       string
	Description string
	// Base is the URL relative links should be resolved against: the page URL,
	// or the document's <base href> if present.
	Base *url.URL
	// Hrefs are the raw href values of every <a> element, in document order.
	Hrefs []string
}

// Parse reads an HTML document and walks its node tree to collect the title,
// meta description, <base href> and all <a href> values. pageURL is used as
// the default base for relative links.
func Parse(r io.Reader, pageURL *url.URL) (*Page, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	p := &Page{Base: pageURL}
	var (
		foundTitle, foundDesc, foundBase bool
	)

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Title:
				if !foundTitle {
					foundTitle = true
					p.Title = strings.TrimSpace(textContent(n))
				}
			case atom.Meta:
				if !foundDesc && strings.EqualFold(attr(n, "name"), "description") {
					foundDesc = true
					p.Description = strings.TrimSpace(attr(n, "content"))
				}
			case atom.Base:
				if !foundBase {
					if href, ok := attrOK(n, "href"); ok {
						if b, err := url.Parse(strings.TrimSpace(href)); err == nil {
							foundBase = true
							if pageURL != nil {
								b = pageURL.ResolveReference(b)
							}
							p.Base = b
						}
					}
				}
			case atom.A:
				if href, ok := attrOK(n, "href"); ok {
					p.Hrefs = append(p.Hrefs, href)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return p, nil
}

// textContent concatenates all text nodes below n.
func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func attr(n *html.Node, key string) string {
	v, _ := attrOK(n, key)
	return v
}

func attrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}
