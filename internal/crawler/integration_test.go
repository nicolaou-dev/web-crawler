//go:build integration

package crawler

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"
)

// Integration test — hits the live books.toscrape.com. It is network-dependent,
// slower, and can be flaky (external site), so it is kept OUT of the normal unit
// run behind a build tag. Run it explicitly:
//
//	go test -tags integration ./internal/crawler/
//
// A small Max + a polite rate limit keep it fast and well-behaved.
func TestCrawlLiveSite(t *testing.T) {
	seed, err := url.Parse("https://books.toscrape.com")
	if err != nil {
		t.Fatal(err)
	}

	c := NewCrawler(Config{
		Client:  &http.Client{Timeout: 15 * time.Second},
		Workers: 5,
		Max:     20,                     // bound the trap; keep the test quick
		Delay:   100 * time.Millisecond, // Be polite: at least 100 ms between successive requests = 10 req/s
	}, seed)

	pages := slices.Collect(c.Pages(context.Background()))

	// crawled something, and respected the budget
	if len(pages) == 0 {
		t.Fatal("crawled 0 pages, want > 0")
	}
	if len(pages) > 20 {
		t.Errorf("crawled %d pages, want <= Max (20)", len(pages))
	}

	// core requirement: never leave the seed host
	for _, p := range pages {
		if p.URL.Host != seed.Host {
			t.Errorf("visited off-host URL: %s", p.URL)
		}
	}

	// the homepage (canonicalized) should be one of the visited pages
	home := "https://books.toscrape.com/"
	found := false
	for _, p := range pages {
		if p.URL.String() == home {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("homepage %s not visited", home)
	}

	t.Logf("crawled %d pages, failures=%d", len(pages), c.Failures())
}
