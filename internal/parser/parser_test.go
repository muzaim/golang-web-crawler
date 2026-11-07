package parser

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func parse(t *testing.T, doc, page string) *Page {
	t.Helper()
	u, err := url.Parse(page)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(strings.NewReader(doc), u)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestTitleTrimmed(t *testing.T) {
	p := parse(t, "<html><head><title>\n  Hello World  \n</title></head></html>", "https://example.com/")
	if p.Title != "Hello World" {
		t.Errorf("title = %q", p.Title)
	}
}

func TestFirstTitleWins(t *testing.T) {
	p := parse(t, "<title>One</title><title>Two</title>", "https://example.com/")
	if p.Title != "One" {
		t.Errorf("title = %q", p.Title)
	}
}

func TestMissingTitle(t *testing.T) {
	p := parse(t, "<html><body><p>hi</p></body></html>", "https://example.com/")
	if p.Title != "" {
		t.Errorf("title = %q, want empty", p.Title)
	}
}

func TestMetaDescriptionCaseInsensitive(t *testing.T) {
	doc := `<head><meta name="Description" content=" A sample site "><meta name="description" content="second"></head>`
	p := parse(t, doc, "https://example.com/")
	if p.Description != "A sample site" {
		t.Errorf("description = %q", p.Description)
	}
}

func TestHrefsCollectedInOrder(t *testing.T) {
	doc := `<body>
		<a href="intro">a</a>
		<a>no href</a>
		<a href="/about">b</a>
		<div><a href="https://github.com/x">c</a></div>
		<a href="">empty</a>
	</body>`
	p := parse(t, doc, "https://example.com/docs/")
	want := []string{"intro", "/about", "https://github.com/x", ""}
	if !reflect.DeepEqual(p.Hrefs, want) {
		t.Errorf("hrefs = %q, want %q", p.Hrefs, want)
	}
	if p.Base.String() != "https://example.com/docs/" {
		t.Errorf("base = %v", p.Base)
	}
}

func TestBaseHref(t *testing.T) {
	doc := `<head><base href="/v2/"></head><body><a href="x">x</a></body>`
	p := parse(t, doc, "https://example.com/docs/page")
	if p.Base.String() != "https://example.com/v2/" {
		t.Errorf("base = %v", p.Base)
	}
}

func TestMalformedHTML(t *testing.T) {
	// Missing </head>, </body>, </html>, unclosed <a> and <p>.
	doc := `<html><title>Broken</title><body><a href="/a">unclosed <a href="/b"><p>text`
	p := parse(t, doc, "https://example.com/")
	if len(p.Hrefs) != 2 {
		t.Errorf("hrefs = %q, want 2 links", p.Hrefs)
	}
}
