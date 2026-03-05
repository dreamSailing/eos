package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var contextWindowOverrides sync.Map

func SetContextWindowOverride(model string, window int) {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" || window <= 0 {
		return
	}
	contextWindowOverrides.Store(m, window)
}

func getContextWindowOverride(model string) (int, bool) {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return 0, false
	}
	if v, ok := contextWindowOverrides.Load(m); ok {
		if n, ok2 := v.(int); ok2 && n > 0 {
			return n, true
		}
	}
	return 0, false
}

func PrimeContextWindowFromProvider(ctx context.Context, baseURL string, apiKey string, model string) int {
	if ctx == nil {
		ctx = context.Background()
	}
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return 0
	}
	if v, ok := getContextWindowOverride(m); ok {
		return v
	}
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" || apiKey == "" {
		return 0
	}
	c2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if n, ok := fetchContextWindowOpenAICompatible(c2, baseURL, apiKey, m); ok {
		SetContextWindowOverride(m, n)
		return n
	}
	return 0
}

func fetchContextWindowOpenAICompatible(ctx context.Context, baseURL string, apiKey string, model string) (int, bool) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return 0, false
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/models"
	u.RawQuery = ""
	u.Fragment = ""

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, false
	}

	var raw any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return 0, false
	}
	root, ok := raw.(map[string]any)
	if !ok {
		return 0, false
	}
	data, ok := root["data"].([]any)
	if !ok {
		if r2, ok2 := root["result"].([]any); ok2 {
			data = r2
		} else {
			return 0, false
		}
	}

	for _, it := range data {
		mm, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id, _ := mm["id"].(string)
		idTrim := strings.TrimSpace(id)
		modelTrim := strings.TrimSpace(model)
		if !strings.EqualFold(idTrim, modelTrim) {
			if name, _ := mm["model"].(string); strings.EqualFold(strings.TrimSpace(name), modelTrim) {
				id = name
			} else {
				continue
			}
		}
		if n, ok := findAnyInt(mm,
			"context_window",
			"contextWindow",
			"context_window_tokens",
			"contextWindowTokens",
			"context_length",
			"contextLength",
			"max_context_tokens",
			"maxContextTokens",
			"max_prompt_tokens",
			"maxPromptTokens",
		); ok && n > 0 {
			return n, true
		}
		if meta, ok := mm["meta"].(map[string]any); ok {
			if n, ok := findAnyInt(meta,
				"context_window",
				"contextWindow",
				"context_window_tokens",
				"contextWindowTokens",
				"context_length",
				"contextLength",
				"max_context_tokens",
				"maxContextTokens",
				"max_prompt_tokens",
				"maxPromptTokens",
			); ok && n > 0 {
				return n, true
			}
		}
		if caps, ok := mm["capabilities"].(map[string]any); ok {
			if n, ok := findAnyInt(caps,
				"context_window",
				"contextWindow",
				"context_window_tokens",
				"contextWindowTokens",
				"context_length",
				"contextLength",
				"max_context_tokens",
				"maxContextTokens",
				"max_prompt_tokens",
				"maxPromptTokens",
			); ok && n > 0 {
				return n, true
			}
		}
	}
	return 0, false
}

func findAnyInt(m map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case int:
			return n, true
		case int64:
			return int(n), true
		case float64:
			return int(n), true
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return int(i), true
			}
		case string:
			s := strings.TrimSpace(n)
			if s == "" {
				continue
			}
			if j, err := json.Number(s).Int64(); err == nil {
				return int(j), true
			}
		}
	}
	return 0, false
}
