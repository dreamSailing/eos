package session

import (
	"strings"
	"testing"
	"github.com/dreamSailing/vb-coding/internal/ai"
)

func TestContextManager_BuildTrimsToBudget(t *testing.T) {
	cm := NewContextManager()
	cm.SetMaxChars(400)
	cm.AddPinned(aiMessage("system", "pinned"))

	for i := 0; i < 12; i++ {
		cm.AddUser(strings.Repeat("中文", 120))
		cm.AddAssistant(strings.Repeat("abcd", 120))
	}

	msgs := cm.Build()
	got := cm.EstimateMessageTokens(msgs)
	if got > 100 {
		t.Fatalf("expected tokens <= 100, got %d", got)
	}
}

func TestContextManager_AutoCompactUsesTokens(t *testing.T) {
	cm := NewContextManager()
	cm.SetMaxChars(600)

	for i := 0; i < 10; i++ {
		cm.AddUser(strings.Repeat("中文", 80))
		cm.AddAssistant(strings.Repeat("abcd", 160))
	}

	_ = cm.Build()

	stats := cm.GetCompressionStats()
	if stats.TotalCompressions == 0 {
		t.Fatalf("expected compressions > 0, got %#v", stats)
	}
}

func aiMessage(role, content string) ai.Message {
	return ai.Message{Role: role, Content: content}
}
