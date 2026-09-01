package parser

import (
	"io"
	"iter"
	"net/url"

	"golang.org/x/net/html"
)

func ParseLinks(body io.Reader, base *url.URL) (links []*url.URL, err error) {
	root, err := html.Parse(body)

	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}

	for href := range hrefs(root) {
		if link, ok := resolveLink(href, base); ok {
			key := link.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			links = append(links, link)
		}
	}

	return links, nil
}

// Relative urls -> absolute urls. /about -> https://books.toscrape.com/about
func resolveLink(raw string, base *url.URL) (*url.URL, bool) {
	if base == nil {
		return nil, false
	}

	href, err := url.Parse(raw)
	if err != nil {
		return nil, false
	}

	link := base.ResolveReference(href)

	if link.Scheme != "http" && link.Scheme != "https" {
		// skip tel:, mailto:, etc
		return nil, false
	}

	// Clear the fragment so /page#a and /page#b resolve to /page avoiding duplicates
	link.Fragment = ""

	return link, true

}

// Creates a range-over-function iterator for hrefs so we can do
// for href := range hrefs(root) {...}
func hrefs(root *html.Node) iter.Seq[string] {
	return func(yield func(string) bool) {
		stack := []*html.Node{root}

		for len(stack) > 0 {
			n := stack[len(stack)-1]

			stack = stack[:len(stack)-1]

			if raw, ok := hrefAttr(n); ok {
				// yield returns the href (iteration variable) to the consumer
				// it's the href in "for href := range hrefs(root) {...}"
				// if yield returns false, the consumer wants to break out of the loop (e.g. break/return)
				// therefore we capture the result here and return early
				if !yield(raw) {
					return
				}
			}

			// Add the children to the stack so they can be checked too
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				stack = append(stack, child)
			}
		}
	}
}

// Check if the node is an anchor and an href
func hrefAttr(n *html.Node) (string, bool) {
	if n.Type != html.ElementNode || n.Data != "a" {
		return "", false
	}

	for _, a := range n.Attr {
		if a.Key == "href" {
			return a.Val, true
		}
	}

	return "", false
}
