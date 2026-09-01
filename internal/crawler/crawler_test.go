package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nicolaou-dev/web-crawler/internal/frontier"
)

func TestCrawl(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
			<a href="/about">About Us</a>
			<h1 id="top">Home Page</h1>
			<p>This is the home page</p>

			<p>Contact</p>
			<a href="https://facebook.com">Facebook</a>
			<a href="https://twitter.com">Twitter</a>
			<a href="mailto:x@y.com">Mail</a>
			<a href="#top">back to top</a>
		</body>
		</html>`

		fmt.Fprint(w, html)
	})

	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
			<a href="/">Home</a>
			<h1 id="top">About Page</h1>
			<p>This is the about page</p>

			<p>Contact</p>
			<a href="https://facebook.com">Facebook</a>
			<a href="https://twitter.com">Twitter</a>
			<a href="mailto:x@y.com">Mail</a>
			<a href="#top">back to top</a>
		</body>
		</html>`

		fmt.Fprint(w, html)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	seed, _ := url.Parse(srv.URL)
	cfg := Config{
		Client:    srv.Client(),
		AllowList: map[string]bool{seed.Host: true},
	}
	f := frontier.NewInMemory(seed, 0)
	defer f.Close()
	c := NewCrawler(cfg, f)
	pages := slices.Collect(c.Pages(context.Background()))

	want := map[string][]string{
		srv.URL + "/": {
			srv.URL + "/",
			srv.URL + "/about",
			"https://facebook.com",
			"https://twitter.com"},
		srv.URL + "/about": {
			srv.URL + "/",
			srv.URL + "/about",
			"https://facebook.com",
			"https://twitter.com",
		},
	}
	for _, v := range want {
		slices.Sort(v)
	}

	got := map[string][]string{}

	for _, page := range pages {
		var links []string
		for _, l := range page.Links {
			links = append(links, l.String())
		}
		slices.Sort(links)
		got[page.URL.String()] = links
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("crawl mismatch (-want +got):\n%s", diff)
	}
}

// Test infinite site (/page/N links to /page/N+1 forever) terminates when
// max budget is set
func TestCrawlMaxTerminates(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		n := 0
		fmt.Sscanf(r.URL.Path, "/page/%d", &n)
		fmt.Fprintf(w, `<a href="/page/%d">next</a>`, n+1)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	seed, _ := url.Parse(srv.URL)
	cfg := Config{
		Client:    srv.Client(),
		AllowList: map[string]bool{seed.Host: true},
	}

	f := frontier.NewInMemory(seed, 10)
	defer f.Close()
	c := NewCrawler(cfg, f)
	pages := slices.Collect(c.Pages(context.Background()))

	if len(pages) != 10 {
		t.Errorf("crawled %d pages, want 10 (Max budget)", len(pages))
	}
}

func TestCrawlNon2xxIsFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<a href="/good">good</a><a href="/bad">bad</a>`)
	})
	mux.HandleFunc("/good", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<p>ok</p>`)
	})
	mux.HandleFunc("/bad", func(w http.ResponseWriter, r *http.Request) {
		// Return status 500 so /bad is logged as a failure
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `<a href="/should-not-follow">trap</a>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	seed, _ := url.Parse(srv.URL)

	f := frontier.NewInMemory(seed, 0)
	defer f.Close()
	c := NewCrawler(Config{
		Client:    srv.Client(),
		AllowList: map[string]bool{seed.Host: true},
	}, f)

	failures := 0
	var pages []Page
	for p := range c.Pages(context.Background()) {
		if p.Err != nil {
			failures++
			continue
		}
		pages = append(pages, p)
	}
	if failures != 1 {
		t.Errorf("failures = %d, want 1", failures)
	}

	if len(pages) != 2 {
		t.Errorf("emitted %v, want root + /good only", pages)
	}
}
