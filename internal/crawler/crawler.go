package crawler

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/nicolaou-dev/web-crawler/internal/fetcher"
	"github.com/nicolaou-dev/web-crawler/internal/parser"
)

type Page struct {
	URL   *url.URL
	Links []*url.URL
}

type Config struct {
	Client *http.Client
	Logger *slog.Logger
}

func Crawl(seed string, cfg Config) ([]Page, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	seedURL, err := url.Parse(seed)
	if err != nil {
		cfg.Logger.Error("invalid seed url", "seed", seed, "err", err)
		return nil, err
	}

	seedURL = canonical(seedURL)

	cfg.Logger.Info("crawl started", "seed", seedURL.String())
	start := time.Now()

	var pages []Page
	var failures int
	queue := []*url.URL{seedURL}
	seen := map[string]struct{}{}
	seen[seedURL.String()] = struct{}{}

	for len(queue) > 0 {
		pageURL := queue[0]
		queue = queue[1:]

		cfg.Logger.Debug("fetching", "url", pageURL.String())
		links, err := fetchAndParse(cfg.Client, pageURL)

		if err != nil {
			failures++
			cfg.Logger.Warn("fetch failed", "url", pageURL.String(), "err", err)
			continue
		}

		var linkURLs []*url.URL

		for _, link := range links {
			if link.String() == pageURL.String() {
				continue // skip self-links
			}

			linkURLs = append(linkURLs, link)

			// If it's external we don't want to crawl it, so skip adding it to the queue
			if link.Host != seedURL.Host {
				continue
			}

			key := link.String()
			if _, ok := seen[key]; ok {
				continue
			}

			seen[key] = struct{}{}
			queue = append(queue, link)
		}

		cfg.Logger.Debug("page crawled", "url", pageURL.String(), "links", len(linkURLs))
		pages = append(pages, Page{URL: pageURL, Links: linkURLs})

	}

	cfg.Logger.Info("crawl finished",
		"pages", len(pages),
		"failures", failures,
		"duration", time.Since(start),
	)

	return pages, nil
}

func fetchAndParse(client *http.Client, pageURL *url.URL) ([]*url.URL, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, body, err := fetcher.Fetch(ctx, client, pageURL.String())
	if err != nil {
		return nil, err
	}
	defer body.Close()

	return parser.ParseLinks(body, pageURL)

}

func canonical(u *url.URL) *url.URL {
	if u.Path == "" {
		u.Path = "/"
	}
	return u
}
