package panels

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/ui/styles"
)

func TestSettingsPanelAlwaysShowsDefaultPlanPromptStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos", "settings.json")
	panel := NewSettingsPanel(styles.NewStyles(styles.DefaultDarkTheme()), settings.NewManager(path), "zh")

	panel.LoadSettings()

	for _, row := range panel.table.Rows() {
		if row[0] == "PlanPromptStyle" {
			if row[1] != "concise" {
				t.Fatalf("PlanPromptStyle row=%q, want concise", row[1])
			}
			return
		}
	}
	t.Fatal("expected PlanPromptStyle row")
}

func TestSettingsPanelShowsConfiguredPlanPromptStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"plan_prompt_style":"detailed"}`), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	panel := NewSettingsPanel(styles.NewStyles(styles.DefaultDarkTheme()), settings.NewManager(path), "zh")

	panel.LoadSettings()

	for _, row := range panel.table.Rows() {
		if row[0] == "PlanPromptStyle" {
			if row[1] != "detailed" {
				t.Fatalf("PlanPromptStyle row=%q, want detailed", row[1])
			}
			return
		}
	}
	t.Fatal("expected PlanPromptStyle row")
}

func TestSettingsPanelShowsGlobalPredictionToggle(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos", "settings.json")
	panel := NewSettingsPanel(styles.NewStyles(styles.DefaultDarkTheme()), settings.NewManager(path), "zh")

	panel.LoadSettings()

	for _, row := range panel.table.Rows() {
		if row[0] == "NextMessagePrediction(Global)" {
			if row[1] != "true" {
				t.Fatalf("NextMessagePrediction(Global) row=%q, want true", row[1])
			}
			return
		}
	}
	t.Fatal("expected NextMessagePrediction(Global) row")
}

func TestSettingsPanelSavesGlobalPredictionToggle(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos", "settings.json")
	panel := NewSettingsPanel(styles.NewStyles(styles.DefaultDarkTheme()), settings.NewManager(path), "zh")
	panel.LoadSettings()

	rows := panel.table.Rows()
	rowIdx := -1
	for i, row := range rows {
		if row[0] == "NextMessagePrediction(Global)" {
			rowIdx = i
			break
		}
	}
	if rowIdx < 0 {
		t.Fatal("expected NextMessagePrediction(Global) row")
	}
	panel.table.SetCursor(rowIdx)

	if _, cmd := panel.enterEditMode(); cmd == nil {
		t.Fatalf("expected blink cmd when entering edit mode")
	}
	panel.editInput.SetValue("false")
	updated, cmd := panel.handleEditMode(tea.KeyMsg{Type: tea.KeyEnter})
	typed, ok := updated.(*SettingsPanel)
	if !ok {
		t.Fatalf("updated panel type=%T, want *SettingsPanel", updated)
	}
	msg := cmd()
	save, ok := msg.(SettingsSaveMsg)
	if !ok {
		t.Fatalf("msg type=%T, want SettingsSaveMsg", msg)
	}
	if save.GlobalPredictionEnabled == nil || *save.GlobalPredictionEnabled {
		t.Fatalf("expected global prediction toggle to be false, got %#v", save.GlobalPredictionEnabled)
	}
	if typed.globalPredictionEnabled {
		t.Fatalf("panel state should update to false")
	}
}
