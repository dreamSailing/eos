package utils

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClientRejectsNonHTTPSByDefault(t *testing.T) {
	client := NewHTTPClient()

	_, err := client.Do(context.Background(), RequestSpec{
		URL: "http://example.com",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
	if clientErr.Kind != ErrNonHTTPSURL {
		t.Fatalf("expected ErrNonHTTPSURL, got %s", clientErr.Kind)
	}
}

func TestHTTPClientReadsSuccessResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := NewHTTPClient()
	resp, err := client.Do(context.Background(), RequestSpec{
		URL:            srv.URL,
		AllowLocalHTTP: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "ok" {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
	if !strings.Contains(resp.ContentType, "text/plain") {
		t.Fatalf("unexpected content type: %q", resp.ContentType)
	}
}

func TestHTTPClientReturnsStructuredStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	client := NewHTTPClient()
	resp, err := client.Do(context.Background(), RequestSpec{
		URL:            srv.URL,
		AllowLocalHTTP: true,
	})
	if resp == nil {
		t.Fatal("expected response")
	}
	if err == nil {
		t.Fatal("expected error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
	if clientErr.Kind != ErrHTTPStatus {
		t.Fatalf("expected ErrHTTPStatus, got %s", clientErr.Kind)
	}
	if clientErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", clientErr.StatusCode)
	}
	if !strings.Contains(clientErr.BodySnippet, "bad gateway") {
		t.Fatalf("unexpected body snippet: %q", clientErr.BodySnippet)
	}
}

func TestHTTPClientFollowsSameHostRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("done"))
	}))
	defer srv.Close()

	client := NewHTTPClient()
	resp, err := client.Do(context.Background(), RequestSpec{
		URL:            srv.URL + "/redirect",
		AllowLocalHTTP: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Body) != "done" {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
	if !strings.HasSuffix(resp.FinalURL, "/final") {
		t.Fatalf("expected final url to end with /final, got %q", resp.FinalURL)
	}
}

func TestHTTPClientRejectsCrossHostRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("target"))
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	client := NewHTTPClient()
	_, err := client.Do(context.Background(), RequestSpec{
		URL:            source.URL,
		AllowLocalHTTP: true,
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
	if clientErr.Kind != ErrCrossHostRedirect {
		t.Fatalf("expected ErrCrossHostRedirect, got %s", clientErr.Kind)
	}
	if clientErr.RedirectURL != target.URL {
		t.Fatalf("expected redirect url %q, got %q", target.URL, clientErr.RedirectURL)
	}
}

func TestHTTPClientMarksTruncatedResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("123456789"))
	}))
	defer srv.Close()

	client := NewHTTPClient()
	resp, err := client.Do(context.Background(), RequestSpec{
		URL:            srv.URL,
		AllowLocalHTTP: true,
		MaxBytes:       4,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Truncated {
		t.Fatal("expected truncated response")
	}
	if string(resp.Body) != "1234" {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
}
