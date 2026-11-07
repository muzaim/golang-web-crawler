// Command go-web-crawler crawls a single website concurrently and reports
// basic information about each page.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"time"

	"go-web-crawler/internal/crawler"
	"go-web-crawler/internal/output"
)

const (
	exitOK        = 0
	exitSeedError = 1
	exitUsage     = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

// run parses args, performs the crawl and returns the process exit status.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg, jsonOut, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	var printer output.Printer = output.NewText(stdout)
	if jsonOut {
		printer = output.NewJSON(stdout)
	}

	seedFailed := false
	for r := range crawler.Crawl(ctx, cfg) {
		if r.Depth == 0 && r.Failed() {
			seedFailed = true
		}
		if err := printer.Page(r); err != nil {
			fmt.Fprintln(stderr, "error writing output:", err)
		}
	}
	if err := printer.Finish(); err != nil {
		fmt.Fprintln(stderr, "error writing output:", err)
	}

	if seedFailed {
		fmt.Fprintln(stderr, "error: seed URL could not be fetched")
		return exitSeedError
	}
	return exitOK
}

func parseFlags(args []string, stderr io.Writer) (crawler.Config, bool, error) {
	fs := flag.NewFlagSet("go-web-crawler", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: go-web-crawler -url <http(s)://host/...> [flags]")
		fmt.Fprintln(stderr, "\nCrawls pages on the same host as -url and reports title, status and links.\n\nFlags:")
		fs.PrintDefaults()
	}

	var (
		rawURL   = fs.String("url", "", "seed URL to start crawling (required)")
		depth    = fs.Int("depth", 2, "maximum link depth from the seed (seed = 0)")
		workers  = fs.Int("workers", 5, "number of concurrent fetchers")
		maxPages = fs.Int("max-pages", 100, "maximum number of pages to fetch")
		timeout  = fs.Duration("timeout", 10*time.Second, "per-request timeout")
		jsonOut  = fs.Bool("json", false, "print results as a JSON array at the end")
	)

	if err := fs.Parse(args); err != nil {
		return crawler.Config{}, false, err
	}

	fail := func(format string, a ...any) (crawler.Config, bool, error) {
		err := fmt.Errorf(format, a...)
		fmt.Fprintln(stderr, "error:", err)
		fs.Usage()
		return crawler.Config{}, false, err
	}

	if fs.NArg() > 0 {
		return fail("unexpected arguments: %v", fs.Args())
	}
	if *rawURL == "" {
		return fail("-url is required")
	}
	seed, err := url.Parse(*rawURL)
	if err != nil || (seed.Scheme != "http" && seed.Scheme != "https") || seed.Host == "" {
		return fail("-url must be an absolute http or https URL, got %q", *rawURL)
	}
	if *depth < 0 {
		return fail("-depth must be >= 0, got %d", *depth)
	}
	if *workers <= 0 {
		return fail("-workers must be > 0, got %d", *workers)
	}
	if *maxPages <= 0 {
		return fail("-max-pages must be > 0, got %d", *maxPages)
	}
	if *timeout <= 0 {
		return fail("-timeout must be > 0, got %s", *timeout)
	}

	return crawler.Config{
		Seed:     seed,
		MaxDepth: *depth,
		Workers:  *workers,
		MaxPages: *maxPages,
		Timeout:  *timeout,
	}, *jsonOut, nil
}
