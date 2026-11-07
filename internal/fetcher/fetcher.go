// Package fetcher performs HTTP requests for the crawler with timeouts,
// a same-host redirect policy, a response size limit and a content-type check.
package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"

	"go-web-crawler/internal/urlutil"
)

const (
	// DefaultMaxBodyBytes caps how much of a response body is read.
	DefaultMaxBodyBytes = 5 << 20 // 5 MiB
	// MaxRedirects is the maximum number of same-host redirect hops.
	MaxRedirects = 10
	// UserAgent is sent with every request.
	UserAgent = "go-web-crawler/0.1"
)

// ErrCrossHostRedirect is returned when a redirect would leave the seed host.
var ErrCrossHostRedirect = errors.New("redirect to another host rejected")

// Response is the outcome of a successful HTTP round trip.
type Response struct {
	StatusCode int
	// IsHTML is true when the Content-Type media type is text/html.
	IsHTML bool
	// Body is only populated for HTML responses (up to MaxBodyBytes).
	Body []byte
}

// Fetcher issues GET requests restricted to a single host.
type Fetcher struct {
	client       *http.Client
	MaxBodyBytes int64
}

// New returns a Fetcher whose redirects may not leave seed's host.
func New(seed *url.URL, timeout time.Duration) *Fetcher {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", MaxRedirects)
			}
			if !urlutil.SameHost(seed, req.URL) {
				return fmt.Errorf("%w: %s", ErrCrossHostRedirect, req.URL)
			}
			return nil
		},
	}
	return &Fetcher{client: client, MaxBodyBytes: DefaultMaxBodyBytes}
}

// Fetch performs a GET request for u. Network errors, timeouts and rejected
// redirects are returned as errors; HTTP error statuses (4xx/5xx) are not.
func (f *Fetcher) Fetch(ctx context.Context, u string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	out := &Response{StatusCode: resp.StatusCode, IsHTML: isHTML(resp.Header.Get("Content-Type"))}
	if !out.IsHTML {
		// Drain a little so the connection can be reused, but don't parse.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return out, nil
	}

	out.Body, err = io.ReadAll(io.LimitReader(resp.Body, f.MaxBodyBytes))
	if err != nil {
		return out, err
	}
	return out, nil
}

func isHTML(contentType string) bool {
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "text/html"
}
