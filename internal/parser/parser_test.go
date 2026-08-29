package parser

import (
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestResolveLink(t *testing.T) {
	base, _ := url.Parse("https://books.toscrape.com/page")
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"relative", "/about", "https://books.toscrape.com/about"},
		{"absolute same host", "https://books.toscrape.com/about", "https://books.toscrape.com/about"},
		{"external", "https://facebook.com", "https://facebook.com"},
		{"mailto", "mailto:a@b.com", ""},
		{"tel", "tel:123", ""},
		{"javascript", "javascript:void(0)", ""},
		{"fragment cleared", "/about#section", "https://books.toscrape.com/about"},
		{"dot dot", "../other", "https://books.toscrape.com/other"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			link, ok := resolveLink(c.raw, base)

			if !ok && c.want != "" {
				t.Fatalf("expected ok for %q", c.raw)
			}

			if ok && c.want == "" {
				t.Fatalf("expected skip, got %v", link)
			}

			if ok && link.String() != c.want {
				t.Errorf("got %q, want %q", link.String(), c.want)
			}
		})
	}
}

func TestParseLinks(t *testing.T) {
	base, _ := url.Parse("https://books.toscrape.com/page")

	html := `
	<html>
	<body>
		<a href="/about">About</a>
		<a href="/about">Dup</a>
		<a href="https://facebook.com">Ext</a>
		<a href="mailto:x@y.com">Mail</a>
		<a name="anchor">No href</a>
		<a href="#top">Frag</a>
	</body>
	</html>`

	body := strings.NewReader(html)

	links, err := ParseLinks(body, base)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []string

	for _, link := range links {
		got = append(got, link.String())
	}

	want := []string{
		"https://books.toscrape.com/about",
		"https://facebook.com",
		"https://books.toscrape.com/page",
	}

	slices.Sort(got)
	slices.Sort(want)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}
