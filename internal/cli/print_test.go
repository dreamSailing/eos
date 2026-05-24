package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/core"
)

func TestPrintResult_JSONOutput(t *testing.T) {
	input := 200
	reply := 80
	total := 280
	cost := 0.005

	result := PrintResult{
		Content:     "print output",
		Model:       "gpt-4",
		InputTokens: &input,
		ReplyTokens: &reply,
		TotalTokens: &total,
		DurationMs:  5678,
		CostUSD:     &cost,
	}

	bs, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(bs, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["content"] != "print output" {
		t.Fatalf("unexpected content: %v", parsed["content"])
	}
	if parsed["model"] != "gpt-4" {
		t.Fatalf("unexpected model: %v", parsed["model"])
	}
	if parsed["duration_ms"] != float64(5678) {
		t.Fatalf("unexpected duration_ms: %v", parsed["duration_ms"])
	}
	if parsed["input_tokens"] != float64(200) {
		t.Fatalf("unexpected input_tokens: %v", parsed["input_tokens"])
	}
	if parsed["total_tokens"] != float64(280) {
		t.Fatalf("unexpected total_tokens: %v", parsed["total_tokens"])
	}
	if parsed["cost_usd"] != 0.005 {
		t.Fatalf("unexpected cost_usd: %v", parsed["cost_usd"])
	}
}

func TestPrintResult_OmitEmptyFields(t *testing.T) {
	result := PrintResult{
		DurationMs: 100,
	}

	bs, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(bs, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	emptyFields := []string{"input_tokens", "reply_tokens", "total_tokens", "cost_usd"}
	for _, f := range emptyFields {
		if _, ok := parsed[f]; ok {
			t.Fatalf("field %s should be omitted when nil", f)
		}
	}
}

func TestBuildDoneEvent_WithTokensAndCost(t *testing.T) {
	total := 500
	cost := 0.01
	usage := core.UsageSummary{
		TotalTokens: &total,
		CostUSD:     &cost,
	}

	evt := buildDoneEvent(2*time.Second, usage)

	if evt["type"] != "done" {
		t.Fatalf("unexpected type: %v", evt["type"])
	}
	if evt["duration_ms"] != int64(2000) {
		t.Fatalf("unexpected duration_ms: %v", evt["duration_ms"])
	}
	if evt["tokens"] != 500 {
		t.Fatalf("unexpected tokens: %v", evt["tokens"])
	}
	if evt["cost_usd"] != 0.01 {
		t.Fatalf("unexpected cost_usd: %v", evt["cost_usd"])
	}
}

func TestBuildDoneEvent_NilTokensAndCost(t *testing.T) {
	usage := core.UsageSummary{}

	evt := buildDoneEvent(500*time.Millisecond, usage)

	if evt["type"] != "done" {
		t.Fatalf("unexpected type: %v", evt["type"])
	}
	if evt["duration_ms"] != int64(500) {
		t.Fatalf("unexpected duration_ms: %v", evt["duration_ms"])
	}
	if _, ok := evt["tokens"]; ok {
		t.Fatal("tokens should be omitted when nil")
	}
	if _, ok := evt["cost_usd"]; ok {
		t.Fatal("cost_usd should be omitted when nil")
	}
}

func TestPrintOptions_DefaultOutputFormat(t *testing.T) {
	opts := PrintOptions{}
	if opts.OutputFormat != "" {
		t.Fatalf("expected empty default output format, got %q", opts.OutputFormat)
	}
}
