package crawler

import (
	"context"
	"errors"
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
	Err   error
}

type Config struct {
	Client    *http.Client
	Logger    *slog.Logger
	Workers   int
	Delay     time.Duration
	AllowList map[string]bool
}

type Crawler struct {
	cfg     Config
	f       frontier.Frontier
	limiter *rate.Limiter
}

func NewCrawler(cfg Config, f frontier.Frontier) *Crawler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 10
	}
	if cfg.Delay <= 0 {
		cfg.Delay = 20 * time.Millisecond
	}

	limiter := rate.NewLimiter(rate.Every(cfg.Delay), 1)

	return &Crawler{
		cfg:     cfg,
		limiter: limiter,
		f:       f,
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
	// Trigger cleanup if context was cancelled
	context.AfterFunc(ctx, c.f.Close)

	results := make(chan Page)

	// Use a goroutine so we don't block on pool.Run
	// and can return results
	go func() {
		defer close(results)
		pool.Run(c.cfg.Workers, c.f.Next, func(t frontier.Task) {
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

func (c *Crawler) process(ctx context.Context, t frontier.Task) (Page, bool) {
	c.cfg.Logger.Debug("fetching", "url", t.URL.String())
	links, err := fetchAndParse(ctx, c.cfg.Client, c.limiter, t.URL)

	if err != nil {
		if errors.Is(err, context.Canceled) {
			return Page{}, false // shutting down: not a real failure
		}
		c.cfg.Logger.Warn("fetch failed", "url", t.URL.String(), "err", err)
		c.f.Fail(t)
		return Page{URL: t.URL, Err: err}, true
	}

	// Enqueue page links before Ack, to ensure links are added
	// to the crawl before it terminates. Ack first might terminate
	// the crawl if this was the last page
	for _, l := range links {
		cu := frontier.Canonical(l)
		if c.cfg.AllowList[cu.Host] {
			c.f.Enqueue(frontier.Task{URL: cu})
		}
	}

	c.cfg.Logger.Debug("page crawled", "url", t.URL.String(), "links", len(links))
	c.f.Ack(t)

	return Page{URL: t.URL, Links: links}, true
}

func fetchAndParse(ctx context.Context, client *http.Client, limiter *rate.Limiter, pageURL *url.URL) ([]*url.URL, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// TODO: Check redirects. The client could follow a redirect to a domain
	// that is not in AllowList
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

	// TODO: Skip parsing for non-html pages (fetcher.Fetch would need to expose content type)
	// && cap size of body reading (prevents OOM)
	return parser.ParseLinks(body, pageURL)
}
