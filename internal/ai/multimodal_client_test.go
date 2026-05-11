package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
