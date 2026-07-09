package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestMultimodalEndpointCandidates(t *testing.T) {
	got := multimodalEndpointCandidates("https://api.example.com", "/images/generations")
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0] != "https://api.example.com/images/generations" {
		t.Fatalf("got[0] = %q", got[0])
	}
	if got[1] != "https://api.example.com/v1/images/generations" {
		t.Fatalf("got[1] = %q", got[1])
	}

	got = multimodalEndpointCandidates("https://api.openai.com/v1", "/images/generations")
	if len(got) != 1 {
		t.Fatalf("len(got) with v1 base = %d, want 1", len(got))
	}
}

func TestParseGeneratedImages_Base64(t *testing.T) {
	want := []byte("image-bytes")
	body, err := json.Marshal(map[string]any{
		"data": []map[string]any{{
			"b64_json":  base64.StdEncoding.EncodeToString(want),
			"mime_type": "image/png",
		}},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	got, err := parseGeneratedImages(body)
	if err != nil {
		t.Fatalf("parseGeneratedImages() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if string(got[0].Bytes) != string(want) {
		t.Fatalf("bytes = %q, want %q", string(got[0].Bytes), string(want))
	}
	if got[0].MIMEType != "image/png" {
		t.Fatalf("mime = %q, want image/png", got[0].MIMEType)
	}
}

func TestParseGeneratedVideoResult_CompletedPayload(t *testing.T) {
	want := []byte("video-bytes")
	body, err := json.Marshal(map[string]any{
		"id":     "task-1",
		"status": "completed",
		"data": []map[string]any{{
			"b64_json":  base64.StdEncoding.EncodeToString(want),
			"mime_type": "video/mp4",
		}},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	got, ok, err := parseGeneratedVideoResult(context.Background(), "token", "https://api.example.com/videos/generations", body)
	if err != nil {
		t.Fatalf("parseGeneratedVideoResult() error = %v", err)
	}
	if !ok {
		t.Fatal("parseGeneratedVideoResult() ok = false, want true")
	}
	if got.RequestID != "task-1" {
		t.Fatalf("request id = %q, want task-1", got.RequestID)
	}
	if string(got.Bytes) != string(want) {
		t.Fatalf("bytes = %q, want %q", string(got.Bytes), string(want))
	}
}

func TestValidateGeneratedMediaURLRejectsLocalAddresses(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"http://127.0.0.1/image.png",
		"http://[::1]/image.png",
		"http://169.254.169.254/latest/meta-data/",
	} {
		if _, err := validateGeneratedMediaURL(context.Background(), rawURL); err == nil {
			t.Fatalf("validateGeneratedMediaURL(%q) error = nil, want rejection", rawURL)
		}
	}
}

func TestValidateGeneratedMediaURLRejectsUnsupportedSchemes(t *testing.T) {
	t.Parallel()

	if _, err := validateGeneratedMediaURL(context.Background(), "file:///tmp/test.png"); err == nil {
		t.Fatal("validateGeneratedMediaURL() error = nil, want invalid scheme")
	}
}

func TestGeneratedMediaHTTPClientRejectsBadRedirects(t *testing.T) {
	t.Parallel()

	client := newGeneratedMediaHTTPClient()
	if err := client.CheckRedirect(&http.Request{URL: mustParseURL(t, "http://127.0.0.1/redirect")}, []*http.Request{{}}); err == nil {
		t.Fatal("CheckRedirect() error = nil, want disallowed address rejection")
	}
	via := []*http.Request{{}, {}, {}}
	err := client.CheckRedirect(&http.Request{URL: mustParseURL(t, "https://example.com/image.png")}, via)
	if err == nil || !strings.Contains(err.Error(), "too many redirects") {
		t.Fatalf("CheckRedirect() error = %v, want too many redirects", err)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", raw, err)
	}
	return parsed
}
