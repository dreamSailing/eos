package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "testing"

func TestRecordToolResultCountsErrors(t *testing.T) {
	rc := &RuntimeCore{}
	rc.StartRequest("rid-1")
	rc.RecordToolCall("rid-1", "grep")
	rc.RecordToolResult("rid-1", "grep", false)
	rc.EndRequest("rid-1", "m")
	if len(rc.reqHistory) != 1 {
		t.Fatalf("expected 1 req history, got %d", len(rc.reqHistory))
	}
	h := rc.reqHistory[0]
	if h.ToolCalls["grep"] != 1 {
		t.Fatalf("expected tool call 1, got %d", h.ToolCalls["grep"])
	}
	if h.ToolCallsError["grep"] != 1 {
		t.Fatalf("expected tool error 1, got %d", h.ToolCallsError["grep"])
	}
}

