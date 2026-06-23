package confirm

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"testing"

	"github.com/dreamSailing/eos/internal/ui/styles"

	tea "github.com/charmbracelet/bubbletea"
)

func testStyles() *styles.Styles {
	return styles.NewStyles(styles.GetTheme("dark"))
}

func TestActionPopupEscEmitsCancel(t *testing.T) {
	p := NewActionPopup(testStyles(), "zh", ActionRequest{
		Actions: []ActionItem{{Kind: "copy", Label: "复制"}},
		Payload: "hello",
		Index:   3,
	})
	updated, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated != p {
		t.Fatalf("Update should return same popup instance")
	}
	msg := cmd()
	res, ok := msg.(ActionResultMsg)
	if !ok {
		t.Fatalf("expected ActionResultMsg, got %T", msg)
	}
	if res.Kind != "cancel" {
		t.Fatalf("Kind=%q, want cancel", res.Kind)
	}
	if res.Index != 3 {
		t.Fatalf("Index=%d, want 3", res.Index)
	}
}

func TestActionPopupEnterEmitsSelectedAction(t *testing.T) {
	p := NewActionPopup(testStyles(), "zh", ActionRequest{
		Actions: []ActionItem{
			{Kind: "copy", Label: "复制"},
			{Kind: "download", Label: "下载"},
		},
		Payload: "plan body",
		Index:   1,
	})

	// 选中第二项后回车
	if _, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown}); true {
	}
	updated, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	msg := cmd()
	res, ok := msg.(ActionResultMsg)
	if !ok {
		t.Fatalf("expected ActionResultMsg, got %T", msg)
	}
	if res.Kind != "download" {
		t.Fatalf("Kind=%q, want download", res.Kind)
	}
	if res.Payload != "plan body" {
		t.Fatalf("Payload=%q, want plan body", res.Payload)
	}
}

func TestActionPopupQuickSelect(t *testing.T) {
	p := NewActionPopup(testStyles(), "zh", ActionRequest{
		Actions: []ActionItem{
			{Kind: "copy", Label: "复制"},
			{Kind: "download", Label: "下载"},
		},
		Payload: "x",
		Index:   0,
	})

	// 按 "2" 快选第二项
	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	updated, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	msg := cmd()
	res, ok := msg.(ActionResultMsg)
	if !ok {
		t.Fatalf("expected ActionResultMsg, got %T", msg)
	}
	if res.Kind != "download" {
		t.Fatalf("Kind=%q, want download after quick-select 2", res.Kind)
	}
}
