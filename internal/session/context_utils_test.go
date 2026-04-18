package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"testing"
)

func TestContextManager_DrainAndClearToolContext(t *testing.T) {
	cm := NewContextManager()
	cm.AddToolObservation(map[string]any{"k": "v"})
	cm.AddToolSummary("tool summary 1")
	cm.AddToolFull("@a.txt\nhello")

	obs, sums, full := cm.DrainAndClearToolContext()
	if len(obs) == 0 || len(sums) == 0 || len(full) == 0 {
		t.Fatalf("expected non-empty drained content")
	}

	obs2, sums2, full2 := cm.DrainAndClearToolContext()
	if len(obs2) != 0 || len(sums2) != 0 || len(full2) != 0 {
		t.Fatalf("expected cleared after drain")
	}
}

func TestContextManager_AppendTaskSummaryPinned(t *testing.T) {
	cm := NewContextManager()
	cm.AppendTaskSummary("- t1\n  - 用户: a")
	cm.AppendTaskSummary("- t2\n  - 用户: b")

	msgs := cm.BuildPreview()
	var found bool
	for _, m := range msgs {
		if m.Role == "system" && strings.HasPrefix(m.Content, "TASK_SUMMARY_HISTORY:\n") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pinned task summary")
	}
}
