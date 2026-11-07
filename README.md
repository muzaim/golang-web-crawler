# go-web-crawler

A small concurrent web crawler written in Go, built as a learning project for
**HTTP, goroutines, channels and HTML parsing**.

Starting from a seed URL, it crawls pages **on the same host**, follows links
up to a depth limit, skips duplicate URLs, and reports basic information about
each page: status code, title, meta description, and internal/external links.

## Usage

```sh
go run ./cmd/go-web-crawler -url https://go.dev -depth 1 -max-pages 20
```

Or build a binary:

```sh
go build -o go-web-crawler ./cmd/go-web-crawler
./go-web-crawler -url https://go.dev -depth 1 -max-pages 20 -json
```

### Flags

| Flag         | Default | Description                                     |
|--------------|---------|-------------------------------------------------|
| `-url`       | (none)  | Seed URL (required, absolute `http`/`https`)    |
| `-depth`     | `2`     | Maximum link depth from the seed (seed = 0)     |
| `-workers`   | `5`     | Number of concurrent fetchers                   |
| `-max-pages` | `100`   | Maximum number of pages to fetch                |
| `-timeout`   | `10s`   | Per-request timeout                             |
| `-json`      | `false` | Print all results as one JSON array at the end  |

### Example output

```text
$ go run ./cmd/go-web-crawler -url https://go.dev -depth 1 -max-pages 5
200  d=0  https://go.dev/  "The Go Programming Language"  int=41 ext=51
200  d=1  https://go.dev/learn/  "Get Started - The Go Programming Language"  int=32 ext=38
200  d=1  https://go.dev/solutions/use-cases  "Use Cases - The Go Programming Language"  int=31 ext=21
200  d=1  https://go.dev/security/  "Security - The Go Programming Language"  int=39 ext=30
200  d=1  https://go.dev/doc  "Documentation - The Go Programming Language"  int=110 ext=26

crawled 5 pages, 0 errors
```

Failed fetches are shown as `ERR  d=<depth>  <url>  <error>`. With `-json`,
each page becomes an object with `url`, `depth`, `status`, `title`,
`description`, `internal_links`, `external_links` and `error`.

The exit status is `0` when the crawl completes, even if some pages fail.
It is `1` when the seed URL itself cannot be fetched, and `2` for invalid flags.
Press **Ctrl+C** to stop early. Results collected so far are still printed.

## Crawl rules

- **Same host only:** `example.com` and `www.example.com` count as different
  hosts. Links to other hosts are listed as external but never fetched.
  Redirects that leave the seed host are rejected.
- **Duplicate detection:** URLs are normalized before checking (relative links
  resolved, scheme and host lowercased, `#fragment` removed). `/about` and
  `/about/` are kept distinct because they can be different resources.
- **Only HTML is parsed:** responses that are not `text/html`, or that have a
  non-2xx status, are reported but not parsed for links. Bodies are capped at 5 MiB.

## How it works

```
                 jobs (chan job)
  +-------------+ ------------------> +-----------+
  | coordinator |                     | worker xN |  fetch + parse
  |  queue      | <------------------ +-----------+
  |  seen map   |   done (chan workerDone)
  |  inFlight   |
  +------+------+ ---> results (chan PageResult) ---> printer
```

- One **coordinator goroutine** owns all shared state: the queue, the `seen`
  set, and the counters. No mutex is needed ("share memory by communicating").
- **Workers** only fetch and parse. They report discovered links back to the
  coordinator, which applies the scope, duplicate, depth and page-limit rules.
- The coordinator uses the **nil-channel `select` trick**. The send-to-worker
  case is enabled only when the queue is non-empty.
- **Termination:** when the queue is empty and no job is in flight, no more
  work can appear. The coordinator closes `jobs`, waits for the workers, and
  closes `results`.

### Project layout

```
cmd/go-web-crawler/   CLI: flags, validation, Ctrl+C handling, output selection
internal/urlutil/     URL normalization and same-host check
internal/parser/      HTML parsing with golang.org/x/net/html
internal/fetcher/     HTTP client: timeout, redirect policy, size limit, content-type
internal/crawler/     coordinator + worker pool
internal/output/      text and JSON printers
```

## Development

```sh
go vet ./...
go test -race ./...
```

Tests run offline against `httptest` servers.
