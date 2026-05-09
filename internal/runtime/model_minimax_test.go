package runtime

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestNormalizeMiniMaxMessages_MergesConsecutiveSystems(t *testing.T) {
	in := []*schema.Message{
		schema.SystemMessage("A"),
		schema.SystemMessage("B"),
		schema.UserMessage("hello"),
	}

	got := normalizeMiniMaxMessages(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Role != schema.System || got[0].Content != "A\n\nB" {
		t.Fatalf("unexpected merged system message: role=%s content=%q", got[0].Role, got[0].Content)
	}
}

func TestNormalizeMiniMaxMessages_MergesConsecutiveUsers(t *testing.T) {
	in := []*schema.Message{
		schema.UserMessage("first"),
		schema.UserMessage("second"),
		schema.AssistantMessage("done", nil),
	}

	got := normalizeMiniMaxMessages(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	if got[0].Role != schema.User || got[0].Content != "first\n\nsecond" {
		t.Fatalf("unexpected merged user message: role=%s content=%q", got[0].Role, got[0].Content)
	}
}

func TestNormalizeMiniMaxMessages_DoesNotMergeToolCalls(t *testing.T) {
	in := []*schema.Message{
		{
			Role:      schema.Assistant,
			Content:   "",
			ToolCalls: []schema.ToolCall{{ID: "call_1", Type: "function", Function: schema.FunctionCall{Name: "read"}}},
		},
		{
			Role:    schema.Assistant,
			Content: "plain text",
		},
	}

	got := normalizeMiniMaxMessages(in)
	if len(got) != 2 {
		t.Fatalf("expected tool call messages to stay separate, got %d", len(got))
	}
}

func TestShouldNormalizeMiniMaxMessages(t *testing.T) {
	if !shouldNormalizeMiniMaxMessages("", "https://api.minimaxi.com/v1", "MiniMax-M2.7") {
		t.Fatal("expected minimax base to enable normalization")
	}
	if shouldNormalizeMiniMaxMessages("", "https://api.openai.com/v1", "gpt-4o") {
		t.Fatal("did not expect openai config to enable normalization")
	}
}
