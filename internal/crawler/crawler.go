package crawler

import (
	"context"
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

func Crawl(seed string, client *http.Client) ([]Page, error) {
	seedURL, err := url.Parse(seed)
	if err != nil {
		return nil, err
	}

	seedURL = canonical(seedURL)

	var pages []Page
	queue := []*url.URL{seedURL}
	seen := map[string]struct{}{}
	seen[seedURL.String()] = struct{}{}

	for len(queue) > 0 {
		pageURL := queue[0]
		queue = queue[1:]

		links, err := fetchAndParse(client, pageURL)

		if err != nil {
			continue
		}

		var linkURLs []*url.URL

		for _, link := range links {
			if link.String() == pageURL.String() {
				continue // skip self-links
			}

			linkURLs = append(linkURLs, link)

			// If it's external we don't want to crawl it, so skip adding it to the queu
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

		pages = append(pages, Page{URL: pageURL, Links: linkURLs})

	}

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
