package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

func ResolveAPISettings() (string, string, string) {
	apiKey := os.Getenv("VB_API_KEY")
	base := os.Getenv("VB_API_BASE")
	model := os.Getenv("VB_MODEL")
	cfg, _ := config.Load()
	if (apiKey == "" || base == "" || model == "") && cfg.Active != "" && len(cfg.Models) > 0 {
		if m, ok := config.ActiveModel(cfg); ok {
			if apiKey == "" {
				apiKey = m.APIKey
			}
			if base == "" {
				base = m.APIBase
			}
			if model == "" {
				model = m.Model
			}
		}
	}
	if model == "" && base != "" {
		if m := config.InferDefaultModel(base); m != "" {
			model = m
		}
	}
	return apiKey, base, model
}

// InferDefaultModel moved to ai/providers.go

func SupportsVision(modelName string) bool {
	if modelName == "" {
		return false
	}
	// 优先使用模型目录检查
	if SupportsVisionFromCatalog(modelName) {
		return true
	}
	// 回退到原有关键词检测
	n := strings.ToLower(modelName)
	if strings.Contains(n, "gpt-4o") {
		return true
	}
	if strings.Contains(n, "omni") {
		return true
	}
	if strings.Contains(n, "claude-3") {
		return true
	}
	if strings.Contains(n, "sonnet") {
		return true
	}
	if strings.Contains(n, "4.5") && strings.Contains(n, "claude") {
		return true
	}
	return false
}

func VisionParseWithHTTP(ctx context.Context, base, key, model string, images [][]byte, mimes []string, prompt string) (string, string, error) {
	if base == "" || key == "" || model == "" {
		return "", "VISION_UNAVAILABLE", fmt.Errorf("missing settings")
	}
	url := strings.TrimRight(base, "/") + "/v1/chat/completions"
	var parts []map[string]any
	if strings.TrimSpace(prompt) != "" {
		parts = append(parts, map[string]any{"type": "text", "text": prompt})
	}
	for i, img := range images {
		b64 := base64.StdEncoding.EncodeToString(img)
		mime := "image/png"
		if i < len(mimes) && strings.TrimSpace(mimes[i]) != "" {
			mime = mimes[i]
		}
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + mime + ";base64," + b64}})
	}
	body := map[string]any{"model": model, "messages": []map[string]any{{"role": "user", "content": parts}}}
	bs, _ := json.Marshal(body)

	// 使用重试机制执行 HTTP 请求
	retryPolicy := utils.RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bs))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	result := utils.DoHTTPRetryWithClient(ctx, http.DefaultClient, req, retryPolicy)
	if result.Error != nil {
		return "", "VISION_UNAVAILABLE", result.Error
	}
	defer func() { _ = result.Response.Body.Close() }()

	rb, _ := io.ReadAll(result.Response.Body)
	if result.Response.StatusCode != 200 {
		return "", "VISION_UNAVAILABLE", fmt.Errorf("%s", string(rb))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	_ = json.Unmarshal(rb, &out)
	if len(out.Choices) == 0 {
		return "", "VISION_UNAVAILABLE", fmt.Errorf("no choices")
	}
	return out.Choices[0].Message.Content, "", nil
}
