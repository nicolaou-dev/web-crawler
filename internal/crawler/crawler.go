package crawler

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/nicolaou-dev/web-crawler/internal/fetcher"
	"github.com/nicolaou-dev/web-crawler/internal/frontier"
	"github.com/nicolaou-dev/web-crawler/internal/parser"
	"github.com/nicolaou-dev/web-crawler/internal/pool"
	"golang.org/x/time/rate"
)

type Page struct {
	URL   *url.URL
	Links []*url.URL
}

type Config struct {
	Client  *http.Client
	Logger  *slog.Logger
	Workers int
	Max     int
	Delay   time.Duration
}

type Crawler struct {
	cfg     Config
	seed    *url.URL
	f       *frontier.Frontier
	limiter *rate.Limiter
}

func NewCrawler(cfg Config, seed *url.URL) *Crawler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 10
	}
	if cfg.Delay <= 0 {
		cfg.Delay = 20 * time.Millisecond
	}

	f := frontier.NewFrontier(cfg.Max)

	limiter := rate.NewLimiter(rate.Every(cfg.Delay), 1)

	return &Crawler{
		cfg:     cfg,
		seed:    seed,
		f:       f,
		limiter: limiter,
	}
}

func (c *Crawler) Pages(ctx context.Context) iter.Seq[Page] {
	return func(yield func(Page) bool) {
		// On break, cancel the context to trigger the cleanup
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for p := range c.run(ctx) {
			if !yield(p) {
				return
			}
		}
	}
}

func (c *Crawler) run(ctx context.Context) <-chan Page {
	c.cfg.Logger.Info("crawl started", "seed", c.seed.String())

	results := make(chan Page)

	go func() {
		defer close(results)
		// Add the seed url so we can start the crawl
		c.f.Enqueue(ctx, frontier.Task{URL: c.seed})
		c.f.Watch()
		pool.Run(ctx, c.cfg.Workers, c.f.Tasks(), func(t frontier.Task) {
			if p, ok := c.process(ctx, t); ok {
				// If the context is cancelled stop pushing processed tasks.
				// The consumer stopped receiving from results, so send
				// would block forever
				select {
				case results <- p:
				case <-ctx.Done():
				}
			}
		})
	}()

	return results
}

func (c *Crawler) Failures() int { return c.f.Failures() }

func (c *Crawler) process(ctx context.Context, t frontier.Task) (Page, bool) {
	c.cfg.Logger.Debug("fetching", "url", t.URL.String())
	links, err := fetchAndParse(ctx, c.cfg.Client, c.limiter, t.URL)

	if err != nil {
		c.cfg.Logger.Warn("fetch failed", "url", t.URL.String(), "err", err)
		c.f.Fail()
		return Page{}, false
	}

	for _, l := range links {
		if inScope(l, c.seed) {
			c.f.Enqueue(ctx, frontier.Task{URL: l})
		}
	}

	c.cfg.Logger.Debug("page crawled", "url", t.URL.String(), "links", len(links))
	c.f.Ack()
	return Page{URL: t.URL, Links: links}, true
}

func inScope(link *url.URL, seed *url.URL) bool {
	// If it's external we don't want to crawl it
	if link.Host != seed.Host {
		return false
	}

	return true

}

func fetchAndParse(ctx context.Context, client *http.Client, limiter *rate.Limiter, pageURL *url.URL) ([]*url.URL, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	status, body, err := fetcher.Fetch(ctx, client, limiter, pageURL.String())
	if err != nil {
		return nil, err
	}
	defer body.Close()

	if status < 200 || status >= 300 {
		// Here would check if it's a transient error. If it is
		// we would retry the task.
		// After x retries, we would move the task to a DLQ
		return nil, fmt.Errorf("unexpected status %d", status)
	}

	return parser.ParseLinks(body, pageURL)
}
