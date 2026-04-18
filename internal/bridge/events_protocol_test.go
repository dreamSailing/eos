package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "testing"

func TestBridgePromptEventInquiry(t *testing.T) {
	ev := bridgePromptEvent(PromptRequest{
		ID:        "inq-1",
		Kind:      "inquiry",
		Question:  "请选择执行模式",
		Options:   []string{"auto", "manual"},
		AllowText: true,
	})

	if ev.Type != "inquiry.required" {
		t.Fatalf("Type=%q, want inquiry.required", ev.Type)
	}
	if got, _ := ev.Data["inquiry_id"].(string); got != "inq-1" {
		t.Fatalf("inquiry_id=%q, want inq-1", got)
	}
	if got, _ := ev.Data["question"].(string); got != "请选择执行模式" {
		t.Fatalf("question=%q, want prompt question", got)
	}
}

func TestBridgeToolResultEvent(t *testing.T) {
	ev := bridgeToolResultEvent("tool-1", "bash", "success", "done", "", map[string]any{"lines": 3})

	if ev.Type != "tool.result" {
		t.Fatalf("Type=%q, want tool.result", ev.Type)
	}
	if got, _ := ev.Data["tool_name"].(string); got != "bash" {
		t.Fatalf("tool_name=%q, want bash", got)
	}
	if got, _ := ev.Data["display"].(string); got != "done" {
		t.Fatalf("display=%q, want done", got)
	}
}
