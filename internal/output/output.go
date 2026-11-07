// Package output renders crawl results as streaming text lines or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"

	"go-web-crawler/internal/crawler"
)

// Printer consumes page results as they arrive and finalizes output at the end.
type Printer interface {
	Page(r crawler.PageResult) error
	Finish() error
}

// Text prints one line per page as it completes, then a summary line.
type Text struct {
	w      io.Writer
	pages  int
	errors int
}

// NewText returns a streaming text printer writing to w.
func NewText(w io.Writer) *Text { return &Text{w: w} }

// Page prints a single result line, e.g.
//
//	200  d=1  https://example.com/about  "About Us"  int=12 ext=3
//	ERR  d=2  https://example.com/x  Get "...": context deadline exceeded
func (t *Text) Page(r crawler.PageResult) error {
	t.pages++
	if r.Failed() {
		t.errors++
		msg := r.Error
		if msg == "" {
			msg = "unknown error"
		}
		_, err := fmt.Fprintf(t.w, "ERR  d=%d  %s  %s\n", r.Depth, r.URL, msg)
		return err
	}
	_, err := fmt.Fprintf(t.w, "%d  d=%d  %s  %q  int=%d ext=%d\n",
		r.StatusCode, r.Depth, r.URL, r.Title, len(r.InternalLinks), len(r.ExternalLinks))
	return err
}

// Finish prints the summary line.
func (t *Text) Finish() error {
	_, err := fmt.Fprintf(t.w, "\ncrawled %d pages, %d errors\n", t.pages, t.errors)
	return err
}

// JSON buffers all results and prints a single indented JSON array at the end.
type JSON struct {
	w       io.Writer
	results []crawler.PageResult
}

// NewJSON returns a JSON printer writing to w.
func NewJSON(w io.Writer) *JSON {
	return &JSON{w: w, results: []crawler.PageResult{}}
}

// Page buffers r; nothing is written until Finish.
func (j *JSON) Page(r crawler.PageResult) error {
	j.results = append(j.results, r)
	return nil
}

// Finish writes the JSON array of all results.
func (j *JSON) Finish() error {
	b, err := json.MarshalIndent(j.results, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(j.w, "%s\n", b)
	return err
}
