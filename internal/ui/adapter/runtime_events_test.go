package adapter

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"testing"

	"github.com/dreamSailing/eos/internal/bridge"
)

func TestNormalizeRuntimeEventMapsPromptRequestToApprovalRequired(t *testing.T) {
	ev := normalizeRuntimeEvent(bridge.Event{
		Type:    "prompt.request",
		RID:     "req-1",
		Content: "请确认执行",
		Data: map[string]any{
			"kind":    "permission",
			"summary": "请确认执行",
		},
	})

	if ev.Type != "approval.required" {
		t.Fatalf("Type=%q, want approval.required", ev.Type)
	}
	if got, _ := ev.Data["approval_id"].(string); got != "req-1" {
		t.Fatalf("approval_id=%q, want req-1", got)
	}
	if got, _ := ev.Data["message"].(string); got != "请确认执行" {
		t.Fatalf("message=%q, want 请确认执行", got)
	}
}

func TestNormalizeRuntimeEventMapsInquiryPrompt(t *testing.T) {
	ev := normalizeRuntimeEvent(bridge.Event{
		Type:    "prompt.request",
		RID:     "inq-1",
		Content: "请选择模式",
		Data: map[string]any{
			"kind":    "inquiry",
			"options": []string{"auto", "plan"},
		},
	})

	if ev.Type != "inquiry.required" {
		t.Fatalf("Type=%q, want inquiry.required", ev.Type)
	}
	if got, _ := ev.Data["inquiry_id"].(string); got != "inq-1" {
		t.Fatalf("inquiry_id=%q, want inq-1", got)
	}
	if got, _ := ev.Data["question"].(string); got != "请选择模式" {
		t.Fatalf("question=%q, want 请选择模式", got)
	}
}

func TestNormalizeRuntimeEventMapsDeltaToProtocolText(t *testing.T) {
	ev := normalizeRuntimeEvent(bridge.Event{
		Type:    "delta",
		RID:     "req-2",
		Content: "hello",
	})

	if ev.Type != "text.delta" {
		t.Fatalf("Type=%q, want text.delta", ev.Type)
	}
	if got, _ := ev.Data["text"].(string); got != "hello" {
		t.Fatalf("text=%q, want hello", got)
	}
}
