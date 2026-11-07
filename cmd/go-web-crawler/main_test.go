package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseFlagsDefaults(t *testing.T) {
	var stderr bytes.Buffer
	cfg, jsonOut, err := parseFlags([]string{"-url", "https://example.com"}, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxDepth != 2 || cfg.Workers != 5 || cfg.MaxPages != 100 || cfg.Timeout != 10*time.Second || jsonOut {
		t.Errorf("unexpected defaults: %+v json=%v", cfg, jsonOut)
	}
}

func TestInvalidInput(t *testing.T) {
	cases := map[string][]string{
		"missing url":    {},
		"ftp scheme":     {"-url", "ftp://example.com"},
		"relative url":   {"-url", "/just/a/path"},
		"negative depth": {"-url", "https://example.com", "-depth", "-1"},
		"zero workers":   {"-url", "https://example.com", "-workers", "0"},
		"zero max pages": {"-url", "https://example.com", "-max-pages", "0"},
		"zero timeout":   {"-url", "https://example.com", "-timeout", "0s"},
		"unknown flag":   {"-url", "https://example.com", "-nope"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), args, &stdout, &stderr)
			if code == 0 {
				t.Errorf("exit code = 0, want non-zero")
			}
			if !strings.Contains(stderr.String(), "Usage") {
				t.Errorf("stderr should contain usage, got %q", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout should be empty, got %q", stdout.String())
			}
		})
	}
}

func newSite(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<title>Home</title><a href="/a">a</a><a href="/gone">gone</a>`))
		case "/a":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<title>A</title>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRunTextOutput(t *testing.T) {
	srv := newSite(t)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"-url", srv.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"Home"`) || !strings.Contains(out, "404") ||
		!strings.Contains(out, "crawled 3 pages, 0 errors") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestRunJSONOutput(t *testing.T) {
	srv := newSite(t)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"-url", srv.URL, "-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	var results []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		t.Fatalf("stdout is not a JSON array: %v\n%s", err, stdout.String())
	}
	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}
}

func TestRunSeedUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL
	srv.Close()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"-url", addr, "-timeout", "1s"}, &stdout, &stderr)
	if code == 0 {
		t.Errorf("exit code = 0, want non-zero for unreachable seed")
	}
	if !strings.Contains(stdout.String(), "ERR") {
		t.Errorf("error should be reported, stdout: %s", stdout.String())
	}
}
