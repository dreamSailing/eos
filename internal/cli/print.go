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

	"github.com/dreamSailing/eos/pkg/core"
)

// PrintOptions holds options for headless print mode
type PrintOptions struct {
	Query           string
	OutputFormat    string // "text", "json", "stream-json"
	AccessMode      string
	ApprovalMode    string
	SandboxMode     string
	SkipPermissions bool
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

	rt := core.NewRuntime()
	defer rt.Close()

	rt.ApplyStartupOptions(core.StartupOptions{
		AccessMode:      opts.AccessMode,
		ApprovalMode:    opts.ApprovalMode,
		SandboxMode:     opts.SandboxMode,
		SkipPermissions: opts.SkipPermissions,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	start := time.Now()
	events, invokeErr := rt.Invoke(ctx, opts.Query)

	var content string
	var resErr error
	if invokeErr != nil {
		resErr = invokeErr
	} else {
		for ev := range events {
			switch ev.Type {
			case "TextFinal":
				content = ev.Message
			case "Error":
				if resErr == nil {
					resErr = fmt.Errorf("%s", ev.Message)
				}
			}
		}
	}
	elapsed := time.Since(start)

	if resErr != nil {
		if opts.OutputFormat == "json" {
			errResult := map[string]string{"error": resErr.Error()}
			bs, _ := json.Marshal(errResult)
			fmt.Fprintln(os.Stdout, string(bs))
		} else {
			fmt.Fprintln(os.Stderr, "Error:", resErr.Error())
		}
		return resErr
	}

	usage := rt.UsageSummary()
	modelName := rt.ModelName()

	result := PrintResult{
		Content:     content,
		Model:       modelName,
		InputTokens: usage.InputTokens,
		ReplyTokens: usage.ReplyTokens,
		TotalTokens: usage.TotalTokens,
		DurationMs:  int(elapsed.Milliseconds()),
		CostUSD:     usage.CostUSD,
	}

	switch opts.OutputFormat {
	case "json":
		bs, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			return jsonErr
		}
		fmt.Fprintln(os.Stdout, string(bs))

	case "stream-json":
		events := []map[string]interface{}{
			{"type": "start", "model": modelName, "timestamp": start.Unix()},
			{"type": "content", "text": content},
			buildDoneEvent(elapsed, usage),
		}
		for _, evt := range events {
			bs, _ := json.Marshal(evt)
			fmt.Fprintln(os.Stdout, string(bs))
		}

	default:
		fmt.Fprintln(os.Stdout, content)
		parts := []string{
			fmt.Sprintf("Model: %s", modelName),
			fmt.Sprintf("Duration: %v", elapsed.Round(time.Millisecond)),
		}
		if usage.TotalTokens != nil {
			parts = append(parts, fmt.Sprintf("Tokens: %d", *usage.TotalTokens))
		}
		if usage.CostUSD != nil {
			parts = append(parts, fmt.Sprintf("Cost: $%.6f", *usage.CostUSD))
		}
		fmt.Fprintf(os.Stderr, "\n---\n%s\n", strings.Join(parts, " | "))
	}

	return nil
}

func buildDoneEvent(elapsed time.Duration, usage core.UsageSummary) map[string]interface{} {
	event := map[string]interface{}{
		"type":        "done",
		"duration_ms": elapsed.Milliseconds(),
	}
	if usage.TotalTokens != nil {
		event["tokens"] = *usage.TotalTokens
	}
	if usage.CostUSD != nil {
		event["cost_usd"] = *usage.CostUSD
	}
	return event
}

func RunPrintModeStream(ctx context.Context, query string, w io.Writer) error {
	rt := core.NewRuntime()
	defer rt.Close()

	events, err := rt.Invoke(ctx, query)
	if err != nil {
		return err
	}
	for ev := range events {
		if ev.Type == "TextFinal" {
			fmt.Fprintln(w, ev.Message)
			return nil
		}
		if ev.Type == "Error" {
			return fmt.Errorf("%s", ev.Message)
		}
	}
	return nil
}
