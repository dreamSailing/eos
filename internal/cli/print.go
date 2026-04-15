package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

// PrintOptions holds options for headless print mode
type PrintOptions struct {
	Query        string
	OutputFormat string // "text", "json", "stream-json"
}

// PrintResult holds the result of a print mode execution
type PrintResult struct {
	Content     string  `json:"content"`
	Model       string  `json:"model"`
	InputTokens int     `json:"input_tokens"`
	ReplyTokens int     `json:"reply_tokens"`
	TotalTokens int     `json:"total_tokens"`
	DurationMs  int     `json:"duration_ms"`
	CostUSD     float64 `json:"cost_usd,omitempty"`
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
			{"type": "done", "duration_ms": elapsed.Milliseconds(), "tokens": stats.Total},
		}
		for _, evt := range events {
			bs, _ := json.Marshal(evt)
			fmt.Fprintln(os.Stdout, string(bs))
		}

	default: // text
		fmt.Fprintln(os.Stdout, content)
		// Print metadata to stderr
		fmt.Fprintf(os.Stderr, "\n---\nModel: %s | Tokens: %d | Duration: %v\n",
			modelName, stats.Total, elapsed.Round(time.Millisecond))
	}

	rc.Shutdown()
	return nil
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
