# Design

## Context

The repository is greenfield: it contains only OpenSpec scaffolding and no Go code yet. The toolchain is Go 1.25. This is a learning project (see proposal.md, Why), so the design deliberately prefers the standard library and explicit concurrency patterns over higher-level libraries. Requirements are in `specs/site-crawling`, `specs/page-extraction`, and `specs/crawler-cli`.

## Goals / Non-Goals

**Goals:**
- Make the goroutine/channel structure explicit and easy to reason about. In particular, termination must be provably deadlock-free.
- Keep fetching, parsing, URL handling, and output in separate, unit-testable packages.
- Make the whole crawl testable offline against `httptest.Server`.

**Non-Goals:**
- Deterministic crawl order across runs. Concurrency makes ordering non-deterministic; only the *set* of fetched URLs is guaranteed.
- Persistence, resumable crawls, distributed crawling.
- Reuse as a library with a stable public API. Everything lives under `internal/`.

## Decisions

### 1. Package layout

```
go.mod                      module go-web-crawler
cmd/go-web-crawler/main.go  flags, validation, signal ctx, wiring
internal/urlutil/           Normalize(base, href), SameHost(a, b)
internal/parser/            Parse(io.Reader, pageURL) -> title, desc, hrefs
internal/fetcher/           HTTP client: timeout, redirect policy, body limit, content-type
internal/crawler/           coordinator + worker pool, PageResult
internal/output/            text (streaming) and JSON printers
```

Rationale: each concept the project is meant to teach (HTTP, parsing, concurrency) lives in its own package with its own tests. Alternative considered: a single `main` package, which is simpler but mixes concerns and makes unit tests awkward.

### 2. Concurrency: a single coordinator goroutine owns all shared state

```
                 jobs (chan Job)
  +-------------+ ------------------> +-----------+
  | coordinator |                     | worker xN |  fetch + parse
  |  queue []Job| <------------------ +-----------+
  |  seen map   |   done (chan workerDone{result, links})
  |  inFlight   |
  |  scheduled  | ---> results (chan PageResult) ---> printer
  +-------------+
```

- The coordinator is the **only** goroutine that touches `seen`, `queue`, `inFlight`, and `scheduled`. No mutex is needed ("share memory by communicating").
- Workers never send to `jobs` directly. They report discovered links back over `done`, and the coordinator decides whether to enqueue them (scope, normalization, dedup, depth, max-pages).
- The coordinator keeps an **unbounded internal slice queue** and uses the nil-channel `select` trick. The send case is enabled only when the queue is non-empty:

  ```go
  var sendCh chan Job; var next Job
  if len(queue) > 0 { sendCh = jobs; next = queue[0] }
  select {
  case sendCh <- next: queue = queue[1:]; inFlight++
  case d := <-done:    inFlight--; results <- d.result; enqueue(d.links)
  case <-ctx.Done():   queue = nil  // stop scheduling, drain in-flight
  }
  ```

- **Termination:** when `len(queue) == 0 && inFlight == 0`, the coordinator closes `jobs`. Workers exit their `range jobs` loops. The coordinator waits on a `sync.WaitGroup` for them, then closes `results`. Deadlock is impossible because workers only ever block sending on `done`, which the coordinator always reads in the same `select`.

Alternative considered: a worker pool with a shared `map` + `sync.Mutex` and a `WaitGroup` counting pending URLs. This works, but it hides the producer/consumer cycle that is the main lesson. It is also easy to deadlock if workers send to a bounded `jobs` channel that only they consume.

### 3. Depth and max-pages are enforced at enqueue time

A URL is marked `seen` and counted toward `scheduled` when it is enqueued, not when it is fetched. A URL is rejected at enqueue time if `depth > maxDepth` or `scheduled >= maxPages`. As a result, the first discovery of a URL determines its depth. Because of concurrency, a URL may occasionally be recorded at a deeper depth than its shortest path. This is accepted (see Non-Goals).

### 4. URL normalization (`urlutil`)

Steps: `base.Parse(href)`, then reject non-http(s) schemes, lowercase the scheme and host, strip the fragment, and turn an empty path into `/`. The query string and trailing slash are kept as-is, because `/a` and `/a/` can be different resources. `SameHost` compares `url.Hostname()` case-insensitively, so the port and scheme are ignored.

### 5. Parsing with `golang.org/x/net/html`

Use `html.Parse` to build the node tree, then walk it recursively. The walk collects the first `<title>` text, the first `<meta name=description>`, the `<base href>`, and all `<a href>`. The parser returns raw hrefs plus the effective base URL. Normalization and classification happen in the crawler via `urlutil`, so the parser stays independent of scope rules.

Alternatives considered:
- Regex: fragile, an anti-pattern.
- `goquery`: convenient, but it hides the tree structure we want to learn.
- `html.Tokenizer`: streaming and lower memory, but handling `<title>` text across tokens is fiddlier. The tree API is clearer for a first version.

### 6. Fetcher

- `http.Client{Timeout: cfg.Timeout}` uses a `CheckRedirect` that returns an error when `req.URL` is not on the seed host, or after 10 hops.
- Requests are built with `http.NewRequestWithContext(ctx, ...)` so that Ctrl+C cancels in-flight requests.
- The fetcher sets a `User-Agent: go-web-crawler/0.1` header.
- If the `Content-Type` media type (via `mime.ParseMediaType`) is not `text/html`, the body is discarded and not parsed.
- The body is read through `io.LimitReader(resp.Body, 5 MiB)` and always closed.
- The page result records the originally requested URL. Final redirect targets are not separately listed.

### 7. Output

The `results` channel is consumed by a printer in `main`:
- **Text mode:** prints a line as each result arrives, for example `200  d=1  https://example.com/about  "About Us"  int=12 ext=3`, or `ERR  d=2  https://example.com/x  <error>`. A summary line follows at the end.
- **JSON mode:** collects all results into a slice and calls `json.MarshalIndent` once at the end. `PageResult` carries the JSON struct tags defined in the CLI spec.

### 8. Cancellation and exit codes

`signal.NotifyContext(ctx, os.Interrupt)` produces the root context. On cancel, the coordinator drops the queue and drains in-flight workers, so results collected so far are still printed. `main` exits with status 1 when the seed result has a non-empty error or a status of 0, and with status 0 otherwise.

## Risks / Trade-offs

- [Non-deterministic order and depth attribution] → Tests assert on sets of URLs, not order. The behavior is documented in Non-Goals.
- [Unbounded internal queue can grow large on big sites] → It is bounded in practice by `-max-pages`, because URLs are counted at enqueue time.
- [No rate limiting may hammer small servers] → The default of 5 workers and 100 pages is modest. Per-host rate limiting is listed as future work.
- [Results channel backpressure: a slow printer could stall the coordinator] → Printing is cheap. If needed, `results` can be buffered with size equal to the worker count.
- [HTML served with a wrong Content-Type is skipped] → This is accepted. The rule is simple and predictable.

## Open Questions

- Should a crawl that is cancelled partway exit with status 0 or 130? This can be decided at implementation time without affecting the specs. The default for now is the regular exit-status rule.
