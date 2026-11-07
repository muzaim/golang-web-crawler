package urlutil

import (
	"net/url"
	"testing"
)

func mustParse(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

func TestNormalize(t *testing.T) {
	base := mustParse(t, "https://example.com/docs/page")

	tests := []struct {
		name string
		href string
		want string
		ok   bool
	}{
		{"root-relative", "/about", "https://example.com/about", true},
		{"uppercase host", "https://EXAMPLE.com/about", "https://example.com/about", true},
		{"uppercase scheme", "HTTPS://example.com/x", "https://example.com/x", true},
		{"fragment stripped", "https://example.com/about#team", "https://example.com/about", true},
		{"fragment only", "#team", "https://example.com/docs/page", true},
		{"empty href", "", "https://example.com/docs/page", true},
		{"relative", "intro", "https://example.com/docs/intro", true},
		{"parent relative", "../x", "https://example.com/x", true},
		{"empty path", "https://example.com", "https://example.com/", true},
		{"query kept", "/s?q=go", "https://example.com/s?q=go", true},
		{"trailing slash kept", "/about/", "https://example.com/about/", true},
		{"protocol relative", "//example.com/p", "https://example.com/p", true},
		{"whitespace trimmed", "  /about  ", "https://example.com/about", true},
		{"mailto", "mailto:hi@example.com", "", false},
		{"javascript", "javascript:void(0)", "", false},
		{"tel", "tel:+62123", "", false},
		{"ftp", "ftp://example.com/f", "", false},
		{"invalid", "http://[::1", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Normalize(base, tt.href)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (got %v)", ok, tt.ok, got)
			}
			if ok && got.String() != tt.want {
				t.Errorf("got %q, want %q", got.String(), tt.want)
			}
		})
	}
}

func TestNormalizeEquivalentURLsCollapse(t *testing.T) {
	base := mustParse(t, "https://example.com/")
	hrefs := []string{"/about", "https://EXAMPLE.com/about", "https://example.com/about#team"}
	seen := map[string]bool{}
	for _, h := range hrefs {
		u, ok := Normalize(base, h)
		if !ok {
			t.Fatalf("normalize %q failed", h)
		}
		seen[u.String()] = true
	}
	if len(seen) != 1 {
		t.Errorf("expected 1 unique URL, got %v", seen)
	}
}

func TestSameHost(t *testing.T) {
	seed := mustParse(t, "https://example.com")
	tests := []struct {
		u    string
		want bool
	}{
		{"https://example.com/blog", true},
		{"http://example.com/about", true},
		{"https://EXAMPLE.COM/", true},
		{"https://example.com:8443/", true},
		{"https://www.example.com/", false},
		{"https://blog.example.com/", false},
		{"https://other.com/", false},
	}
	for _, tt := range tests {
		if got := SameHost(seed, mustParse(t, tt.u)); got != tt.want {
			t.Errorf("SameHost(seed, %q) = %v, want %v", tt.u, got, tt.want)
		}
	}
	if SameHost(nil, seed) {
		t.Error("SameHost(nil, x) should be false")
	}
}
