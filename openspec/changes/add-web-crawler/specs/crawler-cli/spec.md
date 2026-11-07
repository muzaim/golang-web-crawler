# Spec Delta

## Purpose

Defines the command-line interface of `go-web-crawler`: the flags it accepts, their defaults and validation, and the formats in which crawl results are reported.

## ADDED Requirements

### Requirement: Command-line flags with defaults
The CLI SHALL accept these flags: `-url` (required), `-depth` (default 2), `-workers` (default 5), `-max-pages` (default 100), `-timeout` (duration, default 10s), and `-json` (boolean, default false).

#### Scenario: Run with only the URL
- **WHEN** the user runs `go-web-crawler -url https://example.com`
- **THEN** the crawl uses depth 2, 5 workers, max 100 pages, a 10s timeout, and text output

### Requirement: Input validation
The CLI SHALL exit with a non-zero status and print a usage message to stderr in any of these cases: `-url` is missing, `-url` is not an absolute `http`/`https` URL, `-depth` is negative, or `-workers`, `-max-pages`, or `-timeout` is not positive.

#### Scenario: Missing URL
- **WHEN** the user runs `go-web-crawler` without `-url`
- **THEN** an error and usage text are printed to stderr and the exit status is non-zero

#### Scenario: Invalid scheme
- **WHEN** the user runs `go-web-crawler -url ftp://example.com`
- **THEN** an error is printed to stderr and the exit status is non-zero

#### Scenario: Non-positive workers
- **WHEN** the user runs `go-web-crawler -url https://example.com -workers 0`
- **THEN** an error is printed to stderr and the exit status is non-zero

### Requirement: Text output
In the default text mode, the CLI SHALL print one line per page to stdout as each page completes. Each line SHALL contain the status code, depth, URL, title, and the internal and external link counts, or the error if the fetch failed. After the crawl, the CLI SHALL print a summary line with the total number of pages crawled and errors.

#### Scenario: Streaming text lines
- **WHEN** a crawl in text mode fetches 3 pages, one of which fails
- **THEN** stdout has 3 page lines (the failed one showing its error) followed by a summary line reporting 3 pages and 1 error

### Requirement: JSON output
When `-json` is set, the CLI SHALL print nothing to stdout during the crawl. After the crawl, it SHALL print a single JSON array to stdout containing every page result, with fields `url`, `depth`, `status`, `title`, `description`, `internal_links`, `external_links`, and `error`.

#### Scenario: JSON is machine-parseable
- **WHEN** the user runs with `-json` and pipes stdout to a JSON parser
- **THEN** the output parses as a JSON array with one object per fetched page

### Requirement: Exit status
The CLI SHALL exit with status 0 when the crawl completes, even if some individual pages failed. It SHALL exit with a non-zero status for invalid input, or when the seed URL itself cannot be fetched.

#### Scenario: Some pages fail
- **WHEN** the seed succeeds but some linked pages return 404
- **THEN** the exit status is 0

#### Scenario: Seed unreachable
- **WHEN** the seed URL cannot be fetched
- **THEN** the error is reported and the exit status is non-zero
