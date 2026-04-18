package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


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
		return
	}
	c1ID := c1.id
	_ = m.AddMessage(c1ID, schema.AssistantMessage("a", nil))

	c2 := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), msgs)
	if c2 == nil {
		t.Fatalf("expected context, got nil")
		return
	}
	c2ID := c2.id
	if c1ID != c2ID {
		t.Fatalf("expected same id, got %q vs %q", c1ID, c2ID)
	}
	if got := len(m.GetMessages(c1ID)); got < 2 {
		t.Fatalf("expected messages persisted, got %d", got)
	}

	m.ClearRequest("rid")
	c3 := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), msgs)
	if c3 == nil {
		t.Fatalf("expected context, got nil")
		return
	}
	c3ID := c3.id
	if c3ID == c1ID {
		t.Fatalf("expected new id after clear, got %q", c3ID)
	}
}
