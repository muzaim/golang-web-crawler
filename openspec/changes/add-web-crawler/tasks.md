# Tasks

## 1. Setup

- [x] 1.1 Initialize the Go module `go-web-crawler` at the repo root (`go mod init go-web-crawler`) and create the directories `cmd/go-web-crawler` and `internal/{urlutil,parser,fetcher,crawler,output}`. Verify that `go build ./...` succeeds.
- [x] 1.2 Add the dependency `golang.org/x/net/html` (`go get golang.org/x/net/html`). Verify that it appears in `go.mod` and that `go build ./...` succeeds.

## 2. URL handling (`internal/urlutil`)

- [x] 2.1 Implement `Normalize(base *url.URL, href string) (*url.URL, bool)`. It resolves relative URLs, rejects non-http(s) schemes, lowercases the scheme and host, strips the fragment, and turns an empty path into `/`. Verify with table-driven tests covering `/about`, `https://EXAMPLE.com/about`, `#team`, `""`, `mailto:`, `javascript:`, and `../x`.
- [x] 2.2 Implement `SameHost(a, b *url.URL) bool`, comparing hostnames case-insensitively and ignoring scheme and port. Verify with tests: same host with different scheme returns true; `www.` and other subdomains return false.

## 3. HTML parsing (`internal/parser`)

- [x] 3.1 Implement `Parse(r io.Reader, pageURL *url.URL)` using `html.Parse` and a recursive tree walk. It returns the trimmed title, the trimmed meta description (`name` matched case-insensitively), the effective base URL (honoring `<base href>`), and raw `<a href>` values. Verify with fixture HTML tests covering whitespace in the title, a missing title, `name="Description"`, a `<base>` tag, and malformed HTML.

## 4. HTTP fetching (`internal/fetcher`)

- [x] 4.1 Implement a fetcher with a configurable timeout, a `User-Agent` header, requests built with `NewRequestWithContext`, a 5 MiB `io.LimitReader`, a guaranteed `Body.Close`, and a `text/html` check via `mime.ParseMediaType`. Verify with `httptest.Server` tests: an HTML 200 response is parsed, `application/pdf` is not parsed, 404 returns its status, and a slow handler returns a timeout error.
- [x] 4.2 Add a `CheckRedirect` policy that allows same-host redirects (max 10 hops) and rejects redirects to other hosts. Verify with two `httptest.Server` instances: a same-host redirect is followed, and a cross-host redirect returns an error and never hits the second server.

## 5. Crawler engine (`internal/crawler`)

- [x] 5.1 Define `Config` (seed, max depth, workers, max pages, timeout) and `PageResult` with JSON tags `url`, `depth`, `status`, `title`, `description`, `internal_links`, `external_links`, and `error`. Verify that the JSON field names match the spec via a marshal test.
- [x] 5.2 Implement the worker. It receives a `Job`, fetches it, parses it on HTML 2xx, normalizes and classifies links into deduplicated internal and external lists, and sends `workerDone{result, internalLinks}` back. Verify with a unit test against an `httptest.Server` page containing relative, absolute, external, duplicate, and `mailto:` links.
- [x] 5.3 Implement the coordinator. It holds the slice queue, `seen` map, `inFlight` and `scheduled` counters, and uses a nil-channel `select`. Depth and max-pages are enforced at enqueue time. It closes `jobs` when the queue is empty and nothing is in flight, waits for workers, and then closes `results`. Verify with an `httptest.Server` fixture site (cyclic links, shared links, 404 page, PDF, external link). The tests check: each URL is fetched exactly once (count requests per path), the crawl terminates, depth 0 fetches only the seed, depth 1 excludes grandchildren, and max-pages 3 yields exactly 3 results.
- [x] 5.4 Add context cancellation. On `ctx.Done()`, drop the queue, drain in-flight workers, and close `results`. Verify with a test that cancels during a crawl against a slow server: `Crawl` returns promptly with partial results and no goroutine leak.
- [x] 5.5 Verify that `go test -race ./internal/...` passes with no data races.

## 6. CLI and output (`cmd/go-web-crawler`, `internal/output`)

- [x] 6.1 Implement the text printer (one line per result plus a summary of pages and errors) and the JSON printer (a single indented array at the end). Verify with tests that write to a `bytes.Buffer`: the line format, the summary counts, and that the JSON output unmarshals into `[]PageResult`.
- [x] 6.2 Implement `main`. It parses the flags `-url`, `-depth` (2), `-workers` (5), `-max-pages` (100), `-timeout` (10s), and `-json`, validates them, and prints the error and usage to stderr with exit 2 on bad input. It also creates the context with `signal.NotifyContext`, runs the crawl, and pipes results to the selected printer. Exit status is 0 on completion and 1 if the seed fails. Verify by running `go run ./cmd/go-web-crawler` with no flags, with `-url ftp://x`, and with `-workers 0`; each should exit non-zero with a message.
- [x] 6.3 Write `README.md` with a project description, usage, flags, example output, and the concurrency diagram from design.md. Verify that the documented commands run as written.

## 7. Integration check

- [x] 7.1 Run `go vet ./...` and `go test -race ./...`. Both must pass.
- [x] 7.2 Smoke-test against a real site (for example, `go run ./cmd/go-web-crawler -url https://go.dev -depth 1 -max-pages 20`) and against the same site with `-json | jq length`. Verify that only `go.dev` URLs are fetched, the output is well-formed, and Ctrl+C mid-crawl prints partial results and exits.
