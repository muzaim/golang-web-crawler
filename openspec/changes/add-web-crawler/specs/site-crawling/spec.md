# Spec Delta

## Purpose

Defines how the crawler traverses a single website from a seed URL: which URLs it visits, how duplicates are avoided, how depth and page limits apply, and how a crawl runs concurrently and terminates.

## ADDED Requirements

### Requirement: Same-host scope
The crawler SHALL only fetch URLs whose host equals the seed URL's host (case-insensitive). The URL scheme is ignored, and subdomains, including `www.`, count as different hosts.

#### Scenario: Internal link is crawled
- **WHEN** the seed is `https://example.com` and a page links to `https://example.com/blog`
- **THEN** the crawler fetches `https://example.com/blog`

#### Scenario: Different scheme on same host is crawled
- **WHEN** the seed is `https://example.com` and a page links to `http://example.com/about`
- **THEN** the crawler treats the link as internal and fetches it

#### Scenario: Subdomain is not crawled
- **WHEN** the seed is `https://example.com` and a page links to `https://www.example.com/` or `https://blog.example.com/`
- **THEN** the crawler does not fetch that URL and records it as an external link

### Requirement: Only HTTP(S) links are followed
The crawler SHALL ignore links whose scheme is not `http` or `https`, such as `mailto:`, `tel:`, and `javascript:`.

#### Scenario: Mailto link ignored
- **WHEN** a page contains `<a href="mailto:hi@example.com">`
- **THEN** the crawler neither fetches it nor lists it as an internal or external link

### Requirement: Redirects stay on the seed host
The crawler SHALL follow redirects only while the redirect target stays on the seed host. A redirect to another host SHALL stop the fetch and record an error for that page.

#### Scenario: Redirect to same host followed
- **WHEN** `https://example.com/old` redirects to `https://example.com/new`
- **THEN** the crawler fetches `https://example.com/new` and reports it as the result for the requested page

#### Scenario: Redirect to other host rejected
- **WHEN** `https://example.com/out` redirects to `https://other.com/`
- **THEN** the crawler does not fetch `https://other.com/` and reports an error for `https://example.com/out`

### Requirement: URL normalization for duplicate detection
Before checking for duplicates, the crawler SHALL normalize every discovered URL. It resolves the URL against the page's base URL, lowercases the scheme and host, removes the fragment, and treats an empty path as `/`.

#### Scenario: Equivalent URLs collapse to one
- **WHEN** pages link to `/about`, `https://EXAMPLE.com/about`, and `https://example.com/about#team` on seed host `example.com`
- **THEN** the crawler fetches `https://example.com/about` exactly once

### Requirement: Each URL fetched at most once
The crawler SHALL fetch each normalized URL at most once per run, even when many concurrent workers discover it.

#### Scenario: Link discovered by many pages
- **WHEN** ten different pages all link to `https://example.com/contact`
- **THEN** exactly one request is made for `https://example.com/contact`

### Requirement: Depth limit
The seed URL SHALL have depth 0, and a link found on a page at depth `d` SHALL have depth `d+1`. The crawler SHALL NOT fetch URLs whose depth exceeds the configured maximum.

#### Scenario: Links beyond max depth are not fetched
- **WHEN** max depth is 1 and the seed links to `/a`, which links to `/b`
- **THEN** the crawler fetches the seed and `/a`, and does not fetch `/b`

#### Scenario: Depth zero fetches only the seed
- **WHEN** max depth is 0
- **THEN** only the seed URL is fetched

### Requirement: Maximum page limit
The crawler SHALL fetch no more than the configured maximum number of pages per run.

#### Scenario: Page limit reached
- **WHEN** max pages is 3 and the site has 10 reachable pages within the depth limit
- **THEN** exactly 3 pages are fetched and the crawl terminates normally

### Requirement: Concurrent fetching
The crawler SHALL fetch pages concurrently, with no more than the configured number of requests in flight at once.

#### Scenario: Worker count bounds concurrency
- **WHEN** workers is 5 and 20 URLs are pending
- **THEN** at most 5 HTTP requests are in flight at any moment

### Requirement: Automatic termination
The crawl SHALL finish by itself, without deadlocking, once no URLs are pending or in flight.

#### Scenario: Crawl finishes when frontier is exhausted
- **WHEN** every reachable in-scope page within the limits has been fetched
- **THEN** the crawler stops, emits all results, and the process exits with status 0

### Requirement: Request timeout and body limit
Each HTTP request SHALL be bounded by the configured timeout. The crawler SHALL read at most a fixed maximum number of bytes from each response body.

#### Scenario: Slow server times out
- **WHEN** a page does not respond within the configured timeout
- **THEN** that page's result contains a timeout error and the crawl continues with other URLs

### Requirement: Graceful cancellation
On an interrupt signal (Ctrl+C), the crawler SHALL stop scheduling new requests, cancel in-flight requests, emit results already collected, and exit.

#### Scenario: User interrupts the crawl
- **WHEN** the user presses Ctrl+C during a crawl
- **THEN** no new URLs are fetched, results gathered so far are printed, and the process exits
