// Package crawler implements a concurrent, same-host web crawler.
//
// Architecture:
//
//	                 jobs (chan job)
//	+-------------+ ------------------> +-----------+
//	| coordinator |                     | worker xN |  fetch + parse
//	|  queue      | <------------------ +-----------+
//	|  seen       |   done (chan workerDone)
//	|  inFlight   |
//	+------+------+ ---> results (chan PageResult) ---> caller
//
// The coordinator goroutine is the only owner of the queue, the seen set and
// the counters, so no mutex is needed. Workers never enqueue work themselves;
// they report discovered links back to the coordinator.
package crawler

import (
	"context"
	"net/url"
	"sync"
	"time"

	"go-web-crawler/internal/fetcher"
	"go-web-crawler/internal/urlutil"
)

// Config controls a crawl.
type Config struct {
	Seed     *url.URL
	MaxDepth int
	Workers  int
	MaxPages int
	Timeout  time.Duration
}

// PageResult describes one fetched URL.
type PageResult struct {
	URL           string   `json:"url"`
	Depth         int      `json:"depth"`
	StatusCode    int      `json:"status"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	InternalLinks []string `json:"internal_links"`
	ExternalLinks []string `json:"external_links"`
	Error         string   `json:"error"`
}

// Failed reports whether the page could not be fetched successfully.
func (r PageResult) Failed() bool {
	return r.Error != "" || r.StatusCode == 0
}

type job struct {
	url   string
	depth int
}

type workerDone struct {
	result PageResult
}

// Crawl starts crawling cfg.Seed and returns a channel that receives one
// PageResult per fetched URL. The channel is closed when the crawl finishes
// or after ctx is cancelled and in-flight requests have drained.
// The caller must consume the channel until it is closed.
func Crawl(ctx context.Context, cfg Config) <-chan PageResult {
	results := make(chan PageResult)
	go coordinate(ctx, cfg, results)
	return results
}

func coordinate(ctx context.Context, cfg Config, results chan<- PageResult) {
	defer close(results)

	seed, ok := urlutil.Normalize(nil, cfg.Seed.String())
	if !ok {
		results <- PageResult{URL: cfg.Seed.String(), Error: "invalid seed URL"}
		return
	}

	w := &worker{
		seed:    seed,
		fetcher: fetcher.New(seed, cfg.Timeout),
	}

	jobs := make(chan job)
	done := make(chan workerDone)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				done <- w.process(ctx, j)
			}
		}()
	}

	var (
		queue     []job
		seen      = make(map[string]bool)
		scheduled int // URLs accepted into the queue (counts toward MaxPages)
		inFlight  int // jobs handed to workers and not yet reported back
	)

	// enqueue applies depth, page-limit and duplicate rules. Depth and
	// max-pages are enforced here, at enqueue time, so the first discovery of
	// a URL determines its depth.
	enqueue := func(u string, depth int) {
		if depth > cfg.MaxDepth || scheduled >= cfg.MaxPages || seen[u] {
			return
		}
		seen[u] = true
		scheduled++
		queue = append(queue, job{url: u, depth: depth})
	}

	enqueue(seed.String(), 0)

	ctxDone := ctx.Done()
	for len(queue) > 0 || inFlight > 0 {
		// Nil-channel trick: sending on a nil channel blocks forever, so the
		// send case is only enabled when there is something to send.
		var sendCh chan job
		var next job
		if len(queue) > 0 {
			sendCh = jobs
			next = queue[0]
		}

		select {
		case sendCh <- next:
			queue = queue[1:]
			inFlight++

		case d := <-done:
			inFlight--
			results <- d.result
			if ctx.Err() == nil {
				for _, link := range d.result.InternalLinks {
					enqueue(link, d.result.Depth+1)
				}
			}

		case <-ctxDone:
			// Stop scheduling; keep looping only to drain in-flight workers
			// (their requests are cancelled through ctx).
			queue = nil
			ctxDone = nil
		}
	}

	// Nothing queued and nothing in flight: no worker can produce more work.
	close(jobs)
	wg.Wait()
}
