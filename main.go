package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/nicolaou-dev/web-crawler/internal/crawler"
	"github.com/nicolaou-dev/web-crawler/internal/frontier"
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
		Delay:   delay,
		AllowList: map[string]bool{
			seedURL.Host:          true,
			"www." + seedURL.Host: true,
		},
	}

	start := time.Now()
	pages := 0
	// If Ctrl-C, cancel the context instead of killing the process
	// this will cleanup the goroutines and close the channels.
	// The rest of the program will be able to finish executing
	// and we will get the summary from logger.Info below
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// after 1st signal, restore default so a second Ctrl-C hard-kills
	// and stops the OS process. Without it we would be forced to use
	// kill -9
	go func() { <-ctx.Done(); stop() }()

	f := frontier.NewInMemory(seedURL, maxPages)
	defer f.Close()

	c := crawler.NewCrawler(config, f)

	logger.Info("crawl started", "seed", seedURL.String())

	failures := map[string]error{}
	for p := range c.Pages(ctx) {
		if p.Err != nil {
			failures[p.URL.String()] = p.Err
			continue
		}
		pages++
		fmt.Println(p.URL)
		for _, l := range p.Links {
			fmt.Printf("  %s\n", l)
		}
	}

	logger.Info("crawl finished",
		"pages", pages,
		"failures", failures,
		"duration", time.Since(start),
	)
}
