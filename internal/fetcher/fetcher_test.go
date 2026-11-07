package fetcher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newFetcher(t *testing.T, seed string, timeout time.Duration) *Fetcher {
	t.Helper()
	u, err := url.Parse(seed)
	if err != nil {
		t.Fatal(err)
	}
	return New(u, timeout)
}

func TestFetchHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != UserAgent {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<title>ok</title>"))
	}))
	defer srv.Close()

	resp, err := newFetcher(t, srv.URL, time.Second).Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || !resp.IsHTML || string(resp.Body) != "<title>ok</title>" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestFetchNonHTMLNotRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF-1.4"))
	}))
	defer srv.Close()

	resp, err := newFetcher(t, srv.URL, time.Second).Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || resp.IsHTML || resp.Body != nil {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestFetch404(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	resp, err := newFetcher(t, srv.URL, time.Second).Fetch(context.Background(), srv.URL+"/missing")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestFetchTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	_, err := newFetcher(t, srv.URL, 100*time.Millisecond).Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	var ne interface{ Timeout() bool }
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Errorf("expected timeout error, got %v", err)
	}
}

func TestFetchBodyLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(strings.Repeat("a", 1000)))
	}))
	defer srv.Close()

	f := newFetcher(t, srv.URL, time.Second)
	f.MaxBodyBytes = 100
	resp, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != 100 {
		t.Errorf("body len = %d, want 100", len(resp.Body))
	}
}

func TestSameHostRedirectFollowed(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/old", http.RedirectHandler("/new", http.StatusFound))
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("new"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := newFetcher(t, srv.URL, time.Second).Fetch(context.Background(), srv.URL+"/old")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || string(resp.Body) != "new" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestCrossHostRedirectRejected(t *testing.T) {
	var otherHits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherHits.Add(1)
	}))
	defer other.Close()

	// httptest servers all listen on 127.0.0.1; address the other server as
	// "localhost" so it counts as a different host.
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)

	seed := httptest.NewServer(http.RedirectHandler(otherURL+"/", http.StatusFound))
	defer seed.Close()

	_, err := newFetcher(t, seed.URL, time.Second).Fetch(context.Background(), seed.URL+"/out")
	if !errors.Is(err, ErrCrossHostRedirect) {
		t.Fatalf("err = %v, want ErrCrossHostRedirect", err)
	}
	if n := otherHits.Load(); n != 0 {
		t.Errorf("other host was hit %d times", n)
	}
}

func TestIsHTML(t *testing.T) {
	cases := map[string]bool{
		"text/html":                true,
		"text/html; charset=UTF-8": true,
		"TEXT/HTML":                true,
		"application/xhtml+xml":    false,
		"application/pdf":          false,
		"":                         false,
		"garbage;;;":               false,
	}
	for ct, want := range cases {
		if got := isHTML(ct); got != want {
			t.Errorf("isHTML(%q) = %v, want %v", ct, got, want)
		}
	}
}
