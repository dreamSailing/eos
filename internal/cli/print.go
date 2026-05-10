package cli

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
)

// PrintOptions holds options for headless print mode
type PrintOptions struct {
	Query        string
	OutputFormat string // "text", "json", "stream-json"
}

// PrintResult holds the result of a print mode execution
type PrintResult struct {
	Content     string   `json:"content"`
	Model       string   `json:"model"`
	InputTokens *int     `json:"input_tokens,omitempty"`
	ReplyTokens *int     `json:"reply_tokens,omitempty"`
	TotalTokens *int     `json:"total_tokens,omitempty"`
	DurationMs  int      `json:"duration_ms"`
	CostUSD     *float64 `json:"cost_usd,omitempty"`
}

// RunPrintMode executes a single query in headless mode and outputs the result
func RunPrintMode(opts PrintOptions) error {
	if opts.OutputFormat == "" {
		opts.OutputFormat = "text"
	}

	cm := session.NewContextManager()
	tm := tools.NewManager()
	rc := bridge.NewRuntimeCore(cm, tm, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	start := time.Now()
	msg, resErr := rc.GraphInvoke(ctx, opts.Query)
	elapsed := time.Since(start)

	if resErr != nil {
		if opts.OutputFormat == "json" {
			errResult := map[string]string{"error": resErr.Error()}
			bs, _ := json.Marshal(errResult)
			fmt.Fprintln(os.Stdout, string(bs))
		} else {
			fmt.Fprintln(os.Stderr, "Error:", resErr.Error())
		}
		rc.Shutdown()
		return resErr
	}

	content := ""
	if msg != nil {
		content = msg.Content
	}

	// Get token stats
	stats := rc.GetTokenStats()
	modelName := rc.ModelName()

	result := PrintResult{
		Content:     content,
		Model:       modelName,
		InputTokens: stats.Input,
		ReplyTokens: stats.Reply,
		TotalTokens: stats.Total,
		DurationMs:  int(elapsed.Milliseconds()),
		CostUSD:     stats.TotalCostUSD,
	}

	switch opts.OutputFormat {
	case "json":
		bs, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			rc.Shutdown()
			return jsonErr
		}
		fmt.Fprintln(os.Stdout, string(bs))

	case "stream-json":
		// Output as NDJSON events
		events := []map[string]interface{}{
			{"type": "start", "model": modelName, "timestamp": start.Unix()},
			{"type": "content", "text": content},
			buildDoneEvent(elapsed, stats),
		}
		for _, evt := range events {
			bs, _ := json.Marshal(evt)
			fmt.Fprintln(os.Stdout, string(bs))
		}

	default: // text
		fmt.Fprintln(os.Stdout, content)
		// Print metadata to stderr
		parts := []string{
			fmt.Sprintf("Model: %s", modelName),
			fmt.Sprintf("Duration: %v", elapsed.Round(time.Millisecond)),
		}
		if stats.Total != nil {
			parts = append(parts, fmt.Sprintf("Tokens: %d", *stats.Total))
		}
		if stats.TotalCostUSD != nil {
			parts = append(parts, fmt.Sprintf("Cost: $%.6f", *stats.TotalCostUSD))
		}
		fmt.Fprintf(os.Stderr, "\n---\n%s\n", strings.Join(parts, " | "))
	}

	rc.Shutdown()
	return nil
}

func buildDoneEvent(elapsed time.Duration, stats bridge.TokenStats) map[string]interface{} {
	event := map[string]interface{}{
		"type":        "done",
		"duration_ms": elapsed.Milliseconds(),
	}
	if stats.Total != nil {
		event["tokens"] = *stats.Total
	}
	if stats.TotalCostUSD != nil {
		event["cost_usd"] = *stats.TotalCostUSD
	}
	return event
}

// RunPrintModeStream writes streaming output to the given writer
func RunPrintModeStream(ctx context.Context, query string, w io.Writer) error {
	cm := session.NewContextManager()
	tm := tools.NewManager()
	rc := bridge.NewRuntimeCore(cm, tm, nil)
	defer rc.Shutdown()

	msg, err := rc.GraphInvoke(ctx, query)
	if err != nil {
		return err
	}
	if msg != nil {
		fmt.Fprintln(w, msg.Content)
	}
	return nil
}
