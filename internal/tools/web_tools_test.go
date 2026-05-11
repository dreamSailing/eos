package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchStructuredReturnsPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer srv.Close()

	result := (&Manager{}).webFetchStructured(context.Background(), map[string]interface{}{
		"url":    srv.URL,
		"format": "text",
	})
	if result.Status != "success" {
		t.Fatalf("expected success, got %s (%s)", result.Status, result.Error)
	}
	if result.Data["content"] != "hello world" {
		t.Fatalf("unexpected content: %#v", result.Data["content"])
	}
	if result.Data["status_code"] != http.StatusOK {
		t.Fatalf("unexpected status code: %#v", result.Data["status_code"])
	}
}

func TestWebFetchStructuredConvertsHTMLToMarkdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`
			<html>
				<body>
					<main>
						<h1>Title</h1>
						<p>Hello <strong>world</strong>.</p>
						<ul><li>First</li><li>Second</li></ul>
						<p><a href="https://example.com/docs">Docs</a></p>
					</main>
				</body>
			</html>`))
	}))
	defer srv.Close()

	result := (&Manager{}).webFetchStructured(context.Background(), map[string]interface{}{
		"url":    srv.URL,
		"format": "markdown",
	})
	if result.Status != "success" {
		t.Fatalf("expected success, got %s (%s)", result.Status, result.Error)
	}

	content, _ := result.Data["content"].(string)
	for _, expected := range []string{"# Title", "Hello **world**.", "- First", "Docs (https://example.com/docs)"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected markdown content to contain %q, got %q", expected, content)
		}
	}
}

func TestWebFetchStructuredRejectsBinaryContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	defer srv.Close()

	result := (&Manager{}).webFetchStructured(context.Background(), map[string]interface{}{
		"url":    srv.URL,
		"format": "markdown",
	})
	if result.Status != "error" {
		t.Fatalf("expected error, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "unsupported content type") {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestWebFetchStructuredRejectsNonHTTPSRemoteURL(t *testing.T) {
	result := (&Manager{}).webFetchStructured(context.Background(), map[string]interface{}{
		"url":    "http://example.com",
		"format": "text",
	})
	if result.Status != "error" {
		t.Fatalf("expected error, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "non-HTTPS") {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestWebFetchStructuredReportsCrossHostRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("target"))
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	result := (&Manager{}).webFetchStructured(context.Background(), map[string]interface{}{
		"url":    source.URL,
		"format": "text",
	})
	if result.Status != "error" {
		t.Fatalf("expected error, got %s", result.Status)
	}
	redirectURL, _ := result.Data["redirect_url"].(string)
	if redirectURL != target.URL {
		t.Fatalf("expected redirect url %q, got %q", target.URL, redirectURL)
	}
}
