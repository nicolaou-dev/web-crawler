package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/nicolaou-dev/web-crawler/internal/fetcher"
	"github.com/nicolaou-dev/web-crawler/internal/frontier"
	"github.com/nicolaou-dev/web-crawler/internal/parser"
	"github.com/nicolaou-dev/web-crawler/internal/pool"
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
}

type Crawler struct {
	cfg  Config
	seed *url.URL
	f    *frontier.Frontier
}

func NewCrawler(cfg Config, seed *url.URL) *Crawler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 10
	}
	seed = canonical(seed)
	f := frontier.NewFrontier(cfg.Max)
	return &Crawler{
		cfg:  cfg,
		seed: seed,
		f:    f,
	}
}

func (c *Crawler) Run() <-chan Page {
	c.cfg.Logger.Info("crawl started", "seed", c.seed.String())

	results := make(chan Page)

	go func() {
		defer close(results)
		// Add the seed url so we can start the crawl
		c.f.Enqueue(frontier.Task{URL: c.seed})
		c.f.Watch()
		pool.Run(c.cfg.Workers, c.f.Tasks(), func(t frontier.Task) {
			if p, ok := c.process(t); ok {
				results <- p
			}
		})
	}()

	return results
}

func (c *Crawler) Failures() int { return c.f.Failures() }

func (c *Crawler) process(t frontier.Task) (Page, bool) {
	c.cfg.Logger.Debug("fetching", "url", t.URL.String())
	links, err := fetchAndParse(c.cfg.Client, t.URL)

	if err != nil {
		c.cfg.Logger.Warn("fetch failed", "url", t.URL.String(), "err", err)
		c.f.Fail()
		return Page{}, false
	}

	for _, l := range links {
		if inScope(l, c.seed) {
			c.f.Enqueue(frontier.Task{URL: l})
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

func fetchAndParse(client *http.Client, pageURL *url.URL) ([]*url.URL, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, body, err := fetcher.Fetch(ctx, client, pageURL.String())
	if err != nil {
		return nil, err
	}
	defer body.Close()

	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("unexpected status %d", status)
	}

	return parser.ParseLinks(body, pageURL)
}

func canonical(u *url.URL) *url.URL {
	if u.Path == "" {
		u.Path = "/"
	}
	return u
}
