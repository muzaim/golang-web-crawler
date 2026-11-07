package crawler

import (
	"bytes"
	"context"
	"net/url"

	"go-web-crawler/internal/fetcher"
	"go-web-crawler/internal/parser"
	"go-web-crawler/internal/urlutil"
)

// worker fetches and parses a single job. It holds no mutable state, so one
// instance is safely shared by all worker goroutines.
type worker struct {
	seed    *url.URL
	fetcher *fetcher.Fetcher
}

func (w *worker) process(ctx context.Context, j job) workerDone {
	res := PageResult{
		URL:           j.url,
		Depth:         j.depth,
		InternalLinks: []string{},
		ExternalLinks: []string{},
	}

	resp, err := w.fetcher.Fetch(ctx, j.url)
	if resp != nil {
		res.StatusCode = resp.StatusCode
	}
	if err != nil {
		res.Error = err.Error()
		return workerDone{result: res}
	}

	// Only successful HTML responses are parsed and have links extracted.
	if resp.StatusCode < 200 || resp.StatusCode > 299 || !resp.IsHTML {
		return workerDone{result: res}
	}

	pageURL, err := url.Parse(j.url)
	if err != nil {
		res.Error = err.Error()
		return workerDone{result: res}
	}

	page, err := parser.Parse(bytes.NewReader(resp.Body), pageURL)
	if err != nil {
		res.Error = "parse: " + err.Error()
		return workerDone{result: res}
	}

	res.Title = page.Title
	res.Description = page.Description
	res.InternalLinks, res.ExternalLinks = w.classify(page.Base, page.Hrefs)
	return workerDone{result: res}
}

// classify normalizes hrefs and splits them into de-duplicated internal and
// external link lists, preserving first-seen order. Non-http(s) links are dropped.
func (w *worker) classify(base *url.URL, hrefs []string) (internal, external []string) {
	internal, external = []string{}, []string{}
	seen := make(map[string]bool, len(hrefs))
	for _, href := range hrefs {
		u, ok := urlutil.Normalize(base, href)
		if !ok {
			continue
		}
		s := u.String()
		if seen[s] {
			continue
		}
		seen[s] = true
		if urlutil.SameHost(w.seed, u) {
			internal = append(internal, s)
		} else {
			external = append(external, s)
		}
	}
	return internal, external
}
