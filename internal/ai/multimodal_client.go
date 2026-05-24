package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

type GeneratedMedia struct {
	Bytes         []byte
	MIMEType      string
	URL           string
	RevisedPrompt string
	RequestID     string
}

type VideoOptions struct {
	Prompt          string
	DurationSeconds int
	AspectRatio     string
	ImageInput      []byte
	ImageMimeType   string
}

func GenerateImage(ctx context.Context, base, key, model, prompt, size string, count int) ([]GeneratedMedia, error) {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("missing API settings")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if count <= 0 {
		count = 1
	}

	body := map[string]any{
		"model":           model,
		"prompt":          prompt,
		"n":               count,
		"response_format": "b64_json",
	}
	if strings.TrimSpace(size) != "" {
		body["size"] = strings.TrimSpace(size)
	}

	respBody, contentType, _, err := postJSONWithFallback(ctx, key, body, multimodalEndpointCandidates(base, "/images/generations")...)
	if err != nil {
		return nil, err
	}
	if media, ok := parseDirectMediaResponse(respBody, contentType); ok {
		return []GeneratedMedia{media}, nil
	}
	return parseGeneratedImages(respBody)
}

func SynthesizeSpeech(ctx context.Context, base, key, model, text, voice, format string) (GeneratedMedia, error) {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(model) == "" {
		return GeneratedMedia{}, fmt.Errorf("missing API settings")
	}
	if strings.TrimSpace(text) == "" {
		return GeneratedMedia{}, fmt.Errorf("text is required")
	}
	voice = firstNonEmptyString(voice, "alloy")
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "mp3"
	}

	body := map[string]any{
		"model":           model,
		"input":           text,
		"voice":           voice,
		"response_format": format,
	}

	respBody, contentType, _, err := postJSONWithFallback(ctx, key, body, multimodalEndpointCandidates(base, "/audio/speech")...)
	if err != nil {
		return GeneratedMedia{}, err
	}
	if media, ok := parseDirectMediaResponse(respBody, contentType); ok {
		return media, nil
	}
	return parseGeneratedSpeech(respBody, format)
}

func GenerateVideo(ctx context.Context, base, key, model string, opts VideoOptions) (GeneratedMedia, error) {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(model) == "" {
		return GeneratedMedia{}, fmt.Errorf("missing API settings")
	}
	if strings.TrimSpace(opts.Prompt) == "" {
		return GeneratedMedia{}, fmt.Errorf("prompt is required")
	}

	body := map[string]any{
		"model":  model,
		"prompt": opts.Prompt,
	}
	if opts.DurationSeconds > 0 {
		body["duration_seconds"] = opts.DurationSeconds
	}
	if strings.TrimSpace(opts.AspectRatio) != "" {
		body["aspect_ratio"] = strings.TrimSpace(opts.AspectRatio)
	}
	if len(opts.ImageInput) > 0 {
		mimeType := firstNonEmptyString(opts.ImageMimeType, http.DetectContentType(opts.ImageInput))
		body["image"] = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(opts.ImageInput)
	}

	endpoints := multimodalEndpointCandidates(base, "/videos/generations", "/video/generations")
	respBody, contentType, usedEndpoint, err := postJSONWithFallback(ctx, key, body, endpoints...)
	if err != nil {
		return GeneratedMedia{}, err
	}
	if media, ok := parseDirectMediaResponse(respBody, contentType); ok {
		return media, nil
	}
	if media, ok, err := parseGeneratedVideoResult(ctx, key, usedEndpoint, respBody); ok || err != nil {
		return media, err
	}
	return GeneratedMedia{}, fmt.Errorf("video generation response did not contain media output")
}

func parseGeneratedImages(respBody []byte) ([]GeneratedMedia, error) {
	var payload struct {
		Data []struct {
			B64JSON       string `json:"b64_json"`
			Base64Data    string `json:"base64_data"`
			URL           string `json:"url"`
			MIMEType      string `json:"mime_type"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, decodeAPIError(respBody, err)
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("image generation returned no data")
	}
	results := make([]GeneratedMedia, 0, len(payload.Data))
	for _, item := range payload.Data {
		media, err := materializeGeneratedMedia(item.B64JSON, item.Base64Data, item.URL, item.MIMEType)
		if err != nil {
			return nil, err
		}
		media.RevisedPrompt = item.RevisedPrompt
		results = append(results, media)
	}
	return results, nil
}

func parseGeneratedSpeech(respBody []byte, fallbackFormat string) (GeneratedMedia, error) {
	var payload struct {
		Audio      string `json:"audio"`
		B64JSON    string `json:"b64_json"`
		Base64Data string `json:"base64_data"`
		URL        string `json:"url"`
		MIMEType   string `json:"mime_type"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return GeneratedMedia{}, decodeAPIError(respBody, err)
	}
	media, err := materializeGeneratedMedia(payload.B64JSON, firstNonEmptyString(payload.Audio, payload.Base64Data), payload.URL, payload.MIMEType)
	if err != nil {
		return GeneratedMedia{}, err
	}
	if media.MIMEType == "" {
		media.MIMEType = audioFormatToMIME(fallbackFormat)
	}
	return media, nil
}

func parseGeneratedVideoResult(ctx context.Context, key, endpoint string, respBody []byte) (GeneratedMedia, bool, error) {
	var payload map[string]any
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return GeneratedMedia{}, false, decodeAPIError(respBody, err)
	}

	if media, ok, err := mediaFromPayload(payload); ok || err != nil {
		return media, true, err
	}

	taskID := firstStringValue(payload, "id", "task_id", "request_id", "job_id")
	if strings.TrimSpace(taskID) == "" {
		return GeneratedMedia{}, false, nil
	}
	return pollVideoTask(ctx, key, endpoint, taskID)
}

func pollVideoTask(ctx context.Context, key, endpoint, taskID string) (GeneratedMedia, bool, error) {
	pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	statusEndpoints := []string{
		strings.TrimRight(endpoint, "/") + "/" + url.PathEscape(taskID),
		strings.TrimRight(strings.TrimSuffix(endpoint, "/generations"), "/") + "/tasks/" + url.PathEscape(taskID),
	}
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for {
		for _, statusURL := range dedupeStrings(statusEndpoints) {
			respBody, _, err := getWithAuth(pollCtx, key, statusURL)
			if err != nil {
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal(respBody, &payload); err != nil {
				continue
			}
			if media, ok, err := mediaFromPayload(payload); ok || err != nil {
				if ok && media.RequestID == "" {
					media.RequestID = taskID
				}
				return media, true, err
			}

			status := strings.ToLower(strings.TrimSpace(firstStringValue(payload, "status", "state")))
			switch status {
			case "", "queued", "pending", "submitted", "processing", "running", "in_progress":
			case "succeeded", "success", "completed", "done", "finished", "ready":
				if media, ok, err := mediaFromPayload(payload); ok || err != nil {
					if media.RequestID == "" {
						media.RequestID = taskID
					}
					return media, true, err
				}
				return GeneratedMedia{}, true, fmt.Errorf("video task completed without output")
			case "failed", "error", "canceled", "cancelled":
				return GeneratedMedia{}, true, fmt.Errorf("video task failed: %s", firstNonEmptyString(firstStringValue(payload, "error", "message"), status))
			}
		}

		select {
		case <-pollCtx.Done():
			return GeneratedMedia{}, true, fmt.Errorf("video generation timed out")
		case <-ticker.C:
		}
	}
}

func mediaFromPayload(payload map[string]any) (GeneratedMedia, bool, error) {
	if len(payload) == 0 {
		return GeneratedMedia{}, false, nil
	}
	requestID := firstStringValue(payload, "id", "task_id", "request_id", "job_id")
	if data, ok := payload["data"].([]any); ok {
		for _, item := range data {
			if m, ok := item.(map[string]any); ok {
				media, ok, err := mediaFromMap(m)
				if ok || err != nil {
					if ok && media.RequestID == "" {
						media.RequestID = requestID
					}
					return media, true, err
				}
			}
		}
	}
	return mediaFromMap(payload)
}

func mediaFromMap(payload map[string]any) (GeneratedMedia, bool, error) {
	if len(payload) == 0 {
		return GeneratedMedia{}, false, nil
	}
	media, err := materializeGeneratedMedia(
		firstStringValue(payload, "b64_json"),
		firstStringValue(payload, "base64_data", "audio", "video"),
		firstStringValue(payload, "url", "output_url", "video_url", "audio_url", "download_url"),
		firstStringValue(payload, "mime_type"),
	)
	if err == nil {
		if len(media.Bytes) > 0 || strings.TrimSpace(media.URL) != "" {
			media.RevisedPrompt = firstStringValue(payload, "revised_prompt")
			media.RequestID = firstStringValue(payload, "id", "task_id", "request_id", "job_id")
			return media, true, nil
		}
	}
	if err != nil {
		return GeneratedMedia{}, true, err
	}
	return GeneratedMedia{}, false, nil
}

func materializeGeneratedMedia(primaryB64, secondaryB64, mediaURL, mimeType string) (GeneratedMedia, error) {
	if raw := firstNonEmptyString(primaryB64, secondaryB64); raw != "" {
		bs, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return GeneratedMedia{}, err
		}
		return GeneratedMedia{
			Bytes:    bs,
			MIMEType: firstNonEmptyString(mimeType, http.DetectContentType(bs)),
		}, nil
	}
	if strings.TrimSpace(mediaURL) != "" {
		bs, fetchedMIME, err := downloadGeneratedMedia(mediaURL)
		if err != nil {
			return GeneratedMedia{}, err
		}
		return GeneratedMedia{
			Bytes:    bs,
			MIMEType: firstNonEmptyString(mimeType, fetchedMIME),
			URL:      strings.TrimSpace(mediaURL),
		}, nil
	}
	if strings.TrimSpace(mimeType) != "" {
		return GeneratedMedia{MIMEType: strings.TrimSpace(mimeType)}, nil
	}
	return GeneratedMedia{}, nil
}

func parseDirectMediaResponse(respBody []byte, contentType string) (GeneratedMedia, bool) {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if strings.HasPrefix(contentType, "image/") || strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/") {
		return GeneratedMedia{
			Bytes:    respBody,
			MIMEType: firstNonEmptyString(contentType, http.DetectContentType(respBody)),
		}, true
	}
	return GeneratedMedia{}, false
}

func postJSONWithFallback(ctx context.Context, key string, body map[string]any, endpoints ...string) ([]byte, string, string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", "", err
	}
	var lastErr error
	for _, endpoint := range dedupeStrings(endpoints) {
		respBody, contentType, err := doRequestWithBody(ctx, http.MethodPost, endpoint, key, "application/json", payload)
		if err == nil {
			return respBody, contentType, endpoint, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoint available")
	}
	return nil, "", "", lastErr
}

func getWithAuth(ctx context.Context, key, endpoint string) ([]byte, string, error) {
	return doRequestWithBody(ctx, http.MethodGet, endpoint, key, "", nil)
}

func doRequestWithBody(ctx context.Context, method, endpoint, key, contentType string, body []byte) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "*/*")

	result := utils.DoHTTPRetryWithClient(ctx, &http.Client{Timeout: 90 * time.Second}, req, utils.DefaultRetryPolicy)
	if result.Error != nil {
		return nil, "", result.Error
	}
	defer func() { _ = result.Response.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(result.Response.Body, 64<<20))
	if err != nil {
		return nil, "", err
	}
	respContentType := result.Response.Header.Get("Content-Type")
	if result.Response.StatusCode < 200 || result.Response.StatusCode >= 300 {
		return nil, respContentType, decodeAPIError(respBody, fmt.Errorf("http %d", result.Response.StatusCode))
	}
	return respBody, respContentType, nil
}

func decodeAPIError(respBody []byte, fallback error) error {
	var payload struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(respBody, &payload); err == nil && payload.Error != nil {
		switch v := payload.Error.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return errors.New(strings.TrimSpace(v))
			}
		case map[string]any:
			msg := firstStringValue(v, "message", "msg", "error")
			if strings.TrimSpace(msg) != "" {
				return errors.New(strings.TrimSpace(msg))
			}
		}
	}
	if trimmed := strings.TrimSpace(string(respBody)); trimmed != "" {
		return errors.New(trimmed)
	}
	return fallback
}

func downloadGeneratedMedia(mediaURL string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, "", decodeAPIError(body, fmt.Errorf("http %d", resp.StatusCode))
	}
	bs, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, "", err
	}
	contentType := firstNonEmptyString(resp.Header.Get("Content-Type"), detectMIMEFromURL(mediaURL), http.DetectContentType(bs))
	return bs, contentType, nil
}

func multimodalEndpointCandidates(base string, suffixes ...string) []string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil
	}
	var out []string
	for _, suffix := range suffixes {
		suffix = "/" + strings.TrimLeft(strings.TrimSpace(suffix), "/")
		out = append(out, base+suffix)
		parsed, err := url.Parse(base)
		if err == nil && !strings.Contains(parsed.Path, "/v1") {
			out = append(out, strings.TrimRight(base, "/")+"/v1"+suffix)
		}
	}
	return dedupeStrings(out)
}

func detectMIMEFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(path.Base(parsed.Path)))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".ogg":
		return "audio/ogg"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	default:
		return ""
	}
}

func audioFormatToMIME(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "aac":
		return "audio/aac"
	case "ogg":
		return "audio/ogg"
	case "m4a":
		return "audio/mp4"
	default:
		return "audio/mpeg"
	}
}

func firstStringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch vv := v.(type) {
			case string:
				if s := strings.TrimSpace(vv); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
