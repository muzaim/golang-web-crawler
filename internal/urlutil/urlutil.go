// Package urlutil normalizes URLs and decides crawl scope.
package urlutil

import (
	"net/url"
	"strings"
)

// Normalize resolves href against base and returns a canonical form used for
// duplicate detection. It returns false when href cannot be parsed or uses a
// scheme other than http/https (e.g. mailto:, tel:, javascript:).
//
// Canonical form: scheme and host lowercased, fragment removed, empty path
// turned into "/". Query string and trailing slash are preserved because they
// can identify distinct resources.
func Normalize(base *url.URL, href string) (*url.URL, bool) {
	ref, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return nil, false
	}

	u := ref
	if base != nil {
		u = base.ResolveReference(ref)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false
	}
	if u.Host == "" {
		return nil, false
	}

	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	u.RawFragment = ""
	u.User = nil
	if u.Path == "" {
		u.Path = "/"
		u.RawPath = ""
	}
	return u, true
}

// SameHost reports whether a and b share a hostname (case-insensitive).
// Scheme and port are ignored; subdomains (including "www.") are different hosts.
func SameHost(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Hostname(), b.Hostname())
}
