# Proposal

## Why

This is a learning project for Go's core concurrency and networking concepts (HTTP, goroutines, channels, HTML parsing). The repository is currently empty, so we are building `go-web-crawler`: a CLI that automatically collects basic information about the pages of a single website.

## What Changes

- New Go module `go-web-crawler` with a CLI entry point.
- Crawl a website starting from a seed URL, following links **only on the same host** as the seed.
- Fetch pages concurrently using a configurable number of workers.
- Detect and skip duplicate URLs after normalization (resolve relative links, lowercase host, drop fragment).
- Enforce a crawl depth limit and a maximum page count.
- Parse HTML to extract per-page data: URL, depth, HTTP status, title, meta description, internal links, external links, and error.
- Report results as one human-readable line per page on stdout, or as JSON via a `-json` flag.
- Safety measures: request timeout, parse only `text/html` responses, limit response body size, reject redirects that leave the seed host, and support graceful cancellation (Ctrl+C).

Out of scope for this change: robots.txt, per-host rate limiting, cross-domain crawling, extracting headings/images, and broken-link reports.

## Capabilities

### New Capabilities
- `site-crawling`: Traversal behavior. Covers same-host scoping, URL normalization and duplicate detection, depth and page limits, concurrent fetching, termination, and cancellation.
- `page-extraction`: Data extracted from each fetched HTML page (title, meta description, internal/external links) and how non-HTML or failed responses are reported.
- `crawler-cli`: Command-line interface. Covers flags, defaults, input validation, and output formats (text lines and JSON).

### Modified Capabilities
<!-- None: no existing specs. -->

## Impact

- New code: Go module at the repository root (`go.mod`, `cmd/` entry point, internal packages).
- New dependency: `golang.org/x/net/html` for HTML parsing.
- Network: performs outbound HTTP(S) requests to the user-supplied host.
