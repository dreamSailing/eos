package runtime

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestSubAgentManager_RequestContextReuseAndClear(t *testing.T) {
	m := NewSubAgentManager()
	msgs := []*schema.Message{schema.UserMessage("hello")}

	c1 := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), msgs)
	if c1 == nil {
		t.Fatalf("expected context, got nil")
	}
	_ = m.AddMessage(c1.id, schema.AssistantMessage("a", nil))

	c2 := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), msgs)
	if c2 == nil {
		t.Fatalf("expected context, got nil")
	}
	if c1.id != c2.id {
		t.Fatalf("expected same id, got %q vs %q", c1.id, c2.id)
	}
	if got := len(m.GetMessages(c1.id)); got < 2 {
		t.Fatalf("expected messages persisted, got %d", got)
	}

	m.ClearRequest("rid")
	c3 := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), msgs)
	if c3 == nil {
		t.Fatalf("expected context, got nil")
	}
	if c3.id == c1.id {
		t.Fatalf("expected new id after clear, got %q", c3.id)
	}
}
