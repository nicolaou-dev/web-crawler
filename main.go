package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/nicolaou-dev/web-crawler/internal/crawler"
)

const (
	// seed is the page the crawl starts from.
	seed = "https://books.toscrape.com"

	// maxPages bounds the total number of pages crawled.
	maxPages = 500

	// Sets the number of workers, controls concurency
	workers = 5

	delay = 20 * time.Millisecond
)

func main() {
	client := &http.Client{Timeout: 15 * time.Second}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	seedURL, err := url.Parse(seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	config := crawler.Config{
		Client:  client,
		Logger:  logger,
		Workers: workers,
		Max:     maxPages,
		Delay:   delay,
	}

	c := crawler.NewCrawler(config, seedURL)

	start := time.Now()
	pages := 0
	for p := range c.Run() {
		pages++
		fmt.Println(p.URL)
		for _, l := range p.Links {
			fmt.Printf("  %s\n", l)
		}
	}

	logger.Info("crawl finished",
		"pages", pages,
		"failures", c.Failures(),
		"duration", time.Since(start),
	)
}
