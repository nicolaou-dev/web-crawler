package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/nicolaou-dev/web-crawler/internal/frontier"
	"go.uber.org/goleak"
)

func TestCancelStopsPromptlyWithNoLeaks(t *testing.T) {
	defer goleak.VerifyNone(t)

	mux := http.NewServeMux()
	// Create an infinite site so the crawl is still running when we cancel
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		n := 0
		// Take the page number out of the path so we can use it for the next page link
		fmt.Sscanf(r.URL.Path, "/page/%d", &n)
		fmt.Fprintf(w, `<a href="/page/%d">next</a>`, n+1)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	seed, _ := url.Parse(srv.URL)
	client := srv.Client()
	client.Transport.(*http.Transport).DisableKeepAlives = true
	f := frontier.NewInMemory(seed, 0)
	c := NewCrawler(Config{
		Client:    client,
		Workers:   4,
		Delay:     10 * time.Millisecond,
		AllowList: map[string]bool{seed.Host: true},
	}, f)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		n := 0
		for range c.Pages(ctx) {
			n++
			if n == 3 {
				cancel() // cancel mid-crawl
			}
		}
	}()

	select {
	case <-done:
	// If context cancelling is not wired correctly the crawler will keep going on
	// forever given the infinite site.
	// 1 second is plenty of time to ensure context cancel stops the crawler and cleans up
	case <-time.After(1 * time.Second):
		t.Fatal("Pages did not return promptly after ctx cancel")
	}
}

func TestEarlyBreakNoLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		n := 0
		// Take the page number out of the path so we can use it for the next page link
		fmt.Sscanf(r.URL.Path, "/page/%d", &n)
		fmt.Fprintf(w, `<a href="/page/%d">next</a>`, n+1)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	seed, _ := url.Parse(srv.URL)
	client := srv.Client()
	client.Transport.(*http.Transport).DisableKeepAlives = true
	f := frontier.NewInMemory(seed, 0)
	c := NewCrawler(Config{
		Client:    client,
		Workers:   4,
		Delay:     10 * time.Millisecond,
		AllowList: map[string]bool{seed.Host: true},
	}, f)

	for range c.Pages(context.Background()) {
		break // this should trigger defer cancel() inside Pages iter
	}

	// goleak will verify if shutdown was graceful with no leaks
}
