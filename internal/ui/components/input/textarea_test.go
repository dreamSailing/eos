package input

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "testing"

func TestHistoryDraftRestore(t *testing.T) {
	m := New()
	m.SetSize(40, 0)
	m.SetHistory([]string{"prev"})
	m.SetValue("draft")

	m.HistoryUp()
	if got := m.Value(); got != "prev" {
		t.Fatalf("expected prev, got %q", got)
	}

	m.HistoryDown()
	if got := m.Value(); got != "draft" {
		t.Fatalf("expected draft restored, got %q", got)
	}
}

func TestIsMultiLine(t *testing.T) {
	m := New()
	m.SetSize(10, 0)
	m.SetValue("a\nb\nc")
	if !m.IsMultiLine() {
		t.Fatalf("expected multiline for newline text")
	}

	m.Clear()
	m.SetSize(6, 0) // textarea width becomes 4
	m.SetValue("12345")
	if !m.IsMultiLine() {
		t.Fatalf("expected multiline for wrapped text")
	}
}

