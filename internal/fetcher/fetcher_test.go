package fetcher

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	status, body, err := Fetch(context.Background(), srv.Client(), srv.URL)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer body.Close()

	if status != 200 {
		t.Fatalf("got %d, want 200", status)
	}

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if string(data) != "hello" {
		t.Errorf("got body %q, want %q", string(data), "hello")
	}
}

func TestFetchReturnsNon2xxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	status, body, err := Fetch(context.Background(), srv.Client(), srv.URL)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer body.Close()

	if status != 404 {
		t.Fatalf("got %d, want 404", status)
	}
}

func TestFetchTransportErr(t *testing.T) {

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))

	// Kill server so fetch fails
	srv.Close()

	status, body, err := Fetch(context.Background(), &http.Client{}, srv.URL)

	if err == nil {
		t.Fatalf("expected an error, got nil")
	}

	if status != 0 {
		t.Fatalf("got %d, want 0", status)
	}

	if body != nil {
		t.Errorf("expected nil body, got %v", body)

	}
}
