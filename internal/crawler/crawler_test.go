package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
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

	cfg := Config{
		Client: srv.Client(),
	}
	seed, _ := url.Parse(srv.URL)
	var pages []Page
	for p := range NewCrawler(cfg, seed).Run() {
		pages = append(pages, p)
	}

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

	cfg := Config{
		Client: srv.Client(),
		Max:    10,
	}
	seed, _ := url.Parse(srv.URL)

	pages := 0
	for range NewCrawler(cfg, seed).Run() {
		pages++
	}

	if pages != 10 {
		t.Errorf("crawled %d pages, want 10 (Max budget)", pages)
	}
}
