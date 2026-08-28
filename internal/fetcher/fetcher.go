package fetcher

import (
	"context"
	"io"
	"net/http"
)

func Fetch(ctx context.Context, client *http.Client, url string) (status int, body io.ReadCloser, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}

	resp, err := client.Do(req)

	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode, resp.Body, nil

}
