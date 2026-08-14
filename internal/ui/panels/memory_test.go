package panels

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreamSailing/eos/internal/ui/styles"
)

func newTestMemoryPanel() *MemoryPanel {
	return NewMemoryPanel(styles.NewStyles(styles.DefaultDarkTheme()), "zh")
}

func TestMemoryPanelShowsSummaryAndHandbookTabs(t *testing.T) {
	panel := newTestMemoryPanel()
	panel.SetSize(80, 24)
	panel.SetData([]MemoryDoc{
		{Scope: "memory_summary.md", Path: "C:/home/.eos/memories/memory_summary.md", Content: "user prefers tabs", Exists: true},
		{Scope: "MEMORY.md", Path: "C:/home/.eos/memories/MEMORY.md", Content: "# handbook", Exists: true},
	})

	view := panel.View()
	if !strings.Contains(view, "memory_summary.md") || !strings.Contains(view, "MEMORY.md") {
		t.Fatalf("view missing doc tabs:\n%s", view)
	}
	if !strings.Contains(view, "user prefers tabs") {
		t.Fatalf("view missing summary content:\n%s", view)
	}

	updated, _ := panel.Update(tea.KeyMsg{Type: tea.KeyTab})
	view = updated.(*MemoryPanel).View()
	if !strings.Contains(view, "# handbook") {
		t.Fatalf("view missing handbook content after tab switch:\n%s", view)
	}
}

func TestMemoryPanelMissingDocShowsEmptyState(t *testing.T) {
	panel := newTestMemoryPanel()
	panel.SetSize(80, 24)
	panel.SetData(nil)

	view := panel.View()
	if !strings.Contains(view, "记忆尚未生成") {
		t.Fatalf("view missing empty-state hint:\n%s", view)
	}
}

func TestMemoryPanelComposeNoteEmitsSaveMsg(t *testing.T) {
	panel := newTestMemoryPanel()
	panel.SetSize(80, 24)

	updated, _ := panel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	panel = updated.(*MemoryPanel)
	if !panel.IsEditing() {
		t.Fatal("expected composing mode after 'a'")
	}

	panel.editor.SetValue("remember prefer tabs")
	updated, cmd := panel.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	panel = updated.(*MemoryPanel)
	if panel.IsEditing() {
		t.Fatal("expected composing mode to exit after ctrl+s")
	}
	if cmd == nil {
		t.Fatal("expected save cmd")
	}
	msg := cmd()
	save, ok := msg.(MemorySaveMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want MemorySaveMsg", msg)
	}
	if save.Content != "remember prefer tabs" {
		t.Fatalf("save content = %q", save.Content)
	}
}

func TestMemoryPanelRefreshKeyEmitsRefreshMsg(t *testing.T) {
	panel := newTestMemoryPanel()
	_, cmd := panel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh cmd")
	}
	if _, ok := cmd().(MemoryRefreshMsg); !ok {
		t.Fatalf("cmd() = %T, want MemoryRefreshMsg", cmd())
	}
}
