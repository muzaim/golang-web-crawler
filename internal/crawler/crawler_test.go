package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-web-crawler/internal/fetcher"
)

// fixtureSite serves a small site with cycles, shared links, a 404, a PDF and
// an external link, and counts requests per path.
//
//	/          -> /a /b /a#x /missing /doc.pdf mailto: external
//	/a         -> / /b /a/child
//	/b         -> /a /c
//	/a/child   -> /deep
//	/c         -> (none)
//	/deep      -> (none)
//	/missing   404
//	/doc.pdf   application/pdf
//
// Depths: / 0; /a /b /missing /doc.pdf 1; /a/child /c 2; /deep 3.
type fixtureSite struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
}

func newFixtureSite(t *testing.T) *fixtureSite {
	t.Helper()
	s := &fixtureSite{hits: map[string]int{}}
	pages := map[string]string{
		"/": `<title>Home</title><meta name="description" content="home page">
			<a href="/a">a</a><a href="b">b</a><a href="/a#x">a again</a>
			<a href="/missing">404</a><a href="/doc.pdf">pdf</a>
			<a href="mailto:x@y.z">mail</a><a href="https://external.example/">ext</a>`,
		"/a":       `<title>A</title><a href="/">home</a><a href="/b">b</a><a href="/a/child">child</a>`,
		"/b":       `<title>B</title><a href="/a">a</a><a href="/c">c</a>`,
		"/a/child": `<title>Child</title><a href="/deep">deep</a>`,
		"/c":       `<title>C</title>`,
		"/deep":    `<title>Deep</title>`,
	}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits[r.URL.Path]++
		s.mu.Unlock()

		switch r.URL.Path {
		case "/doc.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF"))
			return
		}
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *fixtureSite) hitCounts() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int, len(s.hits))
	for k, v := range s.hits {
		out[k] = v
	}
	return out
}

func testConfig(t *testing.T, seed string) Config {
	t.Helper()
	u, err := url.Parse(seed)
	if err != nil {
		t.Fatal(err)
	}
	return Config{Seed: u, MaxDepth: 10, Workers: 4, MaxPages: 100, Timeout: 2 * time.Second}
}

func collect(t *testing.T, ch <-chan PageResult) []PageResult {
	t.Helper()
	var out []PageResult
	timeout := time.After(10 * time.Second)
	for {
		select {
		case r, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, r)
		case <-timeout:
			t.Fatal("crawl did not terminate (possible deadlock)")
		}
	}
}

func paths(t *testing.T, results []PageResult) []string {
	t.Helper()
	var ps []string
	for _, r := range results {
		u, err := url.Parse(r.URL)
		if err != nil {
			t.Fatal(err)
		}
		ps = append(ps, u.Path)
	}
	sort.Strings(ps)
	return ps
}

func TestPageResultJSONFields(t *testing.T) {
	b, err := json.Marshal(PageResult{})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	want := []string{"depth", "description", "error", "external_links", "internal_links", "status", "title", "url"}
	var got []string
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("json fields = %v, want %v", got, want)
	}
}

func TestWorkerProcessExtractsAndClassifiesLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		absAbout := "http://" + strings.ToUpper(r.Host) + "/about#team" // same page as /about
		w.Write([]byte(`<title> Docs </title><meta name="description" content="d">
			<a href="intro">rel</a>
			<a href="/about">abs path</a>
			<a href="/about">dup</a>
			<a href="` + absAbout + `">abs url dup</a>
			<a href="https://github.com/x">ext</a>
			<a href="https://github.com/x#readme">ext dup</a>
			<a href="mailto:a@b.c">mail</a>
			<a href="#top">self</a>`))
	}))
	defer srv.Close()

	cfg := testConfig(t, srv.URL)
	w := &worker{seed: cfg.Seed, fetcher: fetcher.New(cfg.Seed, cfg.Timeout)}
	d := w.process(context.Background(), job{url: srv.URL + "/docs/", depth: 1})
	r := d.result

	if r.Error != "" || r.StatusCode != 200 {
		t.Fatalf("unexpected result: %+v", r)
	}
	if r.Title != "Docs" || r.Description != "d" || r.Depth != 1 {
		t.Errorf("title/desc/depth = %q/%q/%d", r.Title, r.Description, r.Depth)
	}
	wantInternal := []string{srv.URL + "/docs/intro", srv.URL + "/about", srv.URL + "/docs/"}
	if !reflect.DeepEqual(r.InternalLinks, wantInternal) {
		t.Errorf("internal = %q\nwant       %q", r.InternalLinks, wantInternal)
	}
	if !reflect.DeepEqual(r.ExternalLinks, []string{"https://github.com/x"}) {
		t.Errorf("external = %q", r.ExternalLinks)
	}
}

func TestWorkerNon2xxAndNonHTML(t *testing.T) {
	site := newFixtureSite(t)
	cfg := testConfig(t, site.URL)
	w := &worker{seed: cfg.Seed, fetcher: fetcher.New(cfg.Seed, cfg.Timeout)}

	r := w.process(context.Background(), job{url: site.URL + "/missing"}).result
	if r.StatusCode != 404 || len(r.InternalLinks) != 0 || r.Error != "" {
		t.Errorf("404 result: %+v", r)
	}
	r = w.process(context.Background(), job{url: site.URL + "/doc.pdf"}).result
	if r.StatusCode != 200 || r.Title != "" || len(r.InternalLinks) != 0 {
		t.Errorf("pdf result: %+v", r)
	}
}

func TestWorkerConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL
	srv.Close() // nothing listening any more

	cfg := testConfig(t, addr)
	w := &worker{seed: cfg.Seed, fetcher: fetcher.New(cfg.Seed, cfg.Timeout)}
	r := w.process(context.Background(), job{url: addr + "/"}).result
	if r.Error == "" || r.StatusCode != 0 || !r.Failed() {
		t.Errorf("expected connection error, got %+v", r)
	}
}

func TestCrawlFetchesEachURLOnceAndTerminates(t *testing.T) {
	site := newFixtureSite(t)
	results := collect(t, Crawl(context.Background(), testConfig(t, site.URL)))

	want := []string{"/", "/a", "/a/child", "/b", "/c", "/deep", "/doc.pdf", "/missing"}
	if got := paths(t, results); !reflect.DeepEqual(got, want) {
		t.Errorf("crawled %v, want %v", got, want)
	}
	for p, n := range site.hitCounts() {
		if n != 1 {
			t.Errorf("path %s fetched %d times, want 1", p, n)
		}
	}
	for _, r := range results {
		if strings.Contains(r.URL, "external.example") {
			t.Errorf("external URL was crawled: %s", r.URL)
		}
		if r.URL == site.URL+"/" && !reflect.DeepEqual(r.ExternalLinks, []string{"https://external.example/"}) {
			t.Errorf("home external links = %v", r.ExternalLinks)
		}
	}
}

func TestCrawlDepthLimit(t *testing.T) {
	tests := []struct {
		depth int
		want  []string
	}{
		{0, []string{"/"}},
		{1, []string{"/", "/a", "/b", "/doc.pdf", "/missing"}},
		{2, []string{"/", "/a", "/a/child", "/b", "/c", "/doc.pdf", "/missing"}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("depth=%d", tt.depth), func(t *testing.T) {
			site := newFixtureSite(t)
			cfg := testConfig(t, site.URL)
			cfg.MaxDepth = tt.depth
			results := collect(t, Crawl(context.Background(), cfg))
			if got := paths(t, results); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("crawled %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCrawlMaxPages(t *testing.T) {
	site := newFixtureSite(t)
	cfg := testConfig(t, site.URL)
	cfg.MaxPages = 3
	results := collect(t, Crawl(context.Background(), cfg))
	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}
	total := 0
	for _, n := range site.hitCounts() {
		total += n
	}
	if total != 3 {
		t.Errorf("server saw %d requests, want 3", total)
	}
}

func TestCrawlConcurrencyBoundedByWorkers(t *testing.T) {
	var cur, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cur.Add(1)
		defer cur.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/" {
			for i := 0; i < 20; i++ {
				fmt.Fprintf(w, `<a href="/p%d">p</a>`, i)
			}
		}
	}))
	defer srv.Close()

	cfg := testConfig(t, srv.URL)
	cfg.Workers = 3
	results := collect(t, Crawl(context.Background(), cfg))
	if len(results) != 21 {
		t.Fatalf("got %d results, want 21", len(results))
	}
	if p := peak.Load(); p > 3 {
		t.Errorf("peak concurrency %d exceeds workers 3", p)
	}
	if p := peak.Load(); p < 2 {
		t.Errorf("peak concurrency %d: fetching does not look concurrent", p)
	}
}

func TestCrawlCancellation(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/" {
			for i := 0; i < 50; i++ {
				fmt.Fprintf(w, `<a href="/slow%d">s</a>`, i)
			}
			return
		}
		select { // slow pages block until the test ends or the request is cancelled
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := testConfig(t, srv.URL)
	cfg.Timeout = 30 * time.Second
	ch := Crawl(ctx, cfg)

	first := <-ch // seed result
	if first.Error != "" {
		t.Fatalf("seed failed: %+v", first)
	}
	time.Sleep(50 * time.Millisecond) // let workers block on slow pages
	cancel()

	start := time.Now()
	rest := collect(t, ch)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("cancellation took %v", elapsed)
	}
	if len(rest) > cfg.Workers {
		t.Errorf("got %d results after cancel, want at most %d in-flight", len(rest), cfg.Workers)
	}

	// No goroutine leak: allow time for HTTP transport goroutines to wind down.
	srv.CloseClientConnections()
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before+2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before+2 {
		t.Errorf("goroutines: before=%d after=%d (possible leak)", before, n)
	}
}
