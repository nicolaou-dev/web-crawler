# web-crawler

A concurrent, single-host web crawler.

Given a start/seed URL, it visits every page on the **same host**, printing each URL it visits and the links
found on that page.

It stays on one subdomain — from `https://books.toscrape.com` it crawls that
host but never follows external links (`toscrape.com`, `quotes.toscrape.com`, `facebook.com`, …).

There are ~1.2k pages on books.toscrape.com, a sandbox bookstore built for practising crawling
Most of them are book pages under /catalogue

## Run

1.  Update seed url in main.go. Currently set at books.toscrape.com
2.  Set maxPages to 0 if you want to crawl the whole site.
    Takes approximately 45 seconds with 5 workers and Delay set to 20ms.

    Currently maxPages is set at 500

3.  Set the number of workers. Currently set at 5

    Note:
    In a real crawl we would want to be polite and respect the limits of the website set in robots.txt.
    Setting workers to a high number with a short Delay would likely lead to rate limiting.

4.  Set the Delay. Currently set at 20ms
5.  Run the crawler

    ```bash
    go run .
    ```

    Results go to stdout, logs to stderr.

    Output is each visited URL followed by its links, indented:

    ```
    https://books.toscrape.com/
      https://books.toscrape.com/catalogue/page-2.html
      https://books.toscrape.com/catalogue/olio_984/index.html
    ```

## Test

Run with -race so it can test concurrency as well

```bash
go test -race ./...
```

### Integration test

There is an integration test that crawls the live `books.toscrape.com`. It is
network-dependent and slower, so it is kept out of the default run behind a
build tag. Run it explicitly with `-tags integration`:

```bash
go test -tags integration -race ./internal/crawler/
```
