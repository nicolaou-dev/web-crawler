package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/nicolaou-dev/web-crawler/internal/frontier"
)

// The rate limiter must pace requests regardless of worker count. With a fake
// site of 10 instant-responding pages, delay is 200 ms (rate 5 req /s ) means
// the crawl cannot finish faster than
// pages - burst (hard coded in limiter) / rate = (10 - 1) / 5 = 1.8s.
// We assert the crawler takes at least 1.5s (lower than 1.8s).
// If the rate limiter was not working it would finish almost instantly
func TestCrawlRespectsRateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Return 9 links on root
		if r.URL.Path == "/" {
			for i := 0; i < 9; i++ {
				fmt.Fprintf(w, `<a href="/p%d">x</a>`, i)
			}
		}
		// Rest of the pages just return an empty body
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	seed, _ := url.Parse(srv.URL)
	f := frontier.NewInMemory(seed, 0)
	defer f.Close()
	// Assuming 1 ms per request (given it's instant here)
	// 10 workers gives 10 / 0.001 = 10000 concurrent requests/s
	// which is deliberately more than the rate (5 req/ s) to prove that
	// workers do not bypass the limiter
	c := NewCrawler(Config{
		Client:    srv.Client(),
		Workers:   10,
		Delay:     200 * time.Millisecond,
		AllowList: map[string]bool{seed.Host: true},
	}, f)

	start := time.Now()
	pages := slices.Collect(c.Pages(context.Background()))
	elapsed := time.Since(start)

	n := len(pages)
	if n != 10 {
		t.Fatalf("crawled %d pages, want 10 (root + 9)", n)
	}
	const min = 1500 * time.Millisecond // expected ~1.8s; assert >= 1.5s with margin
	if elapsed < min {
		t.Errorf("crawl took %v, want >= %v — rate limit not enforced", elapsed, min)
	}
}
