package panels

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/ui/styles"
)

func TestSettingsPanelAlwaysShowsDefaultPlanPromptStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos", "settings.json")
	panel := NewSettingsPanel(styles.NewStyles(styles.DefaultDarkTheme()), settings.NewManager(path), "zh")

	panel.LoadSettings()

	for _, row := range panel.table.Rows() {
		if row[0] == "计划详略" {
			if row[1] != "concise" {
				t.Fatalf("计划详略 row=%q, want concise", row[1])
			}
			return
		}
	}
	t.Fatal("expected 计划详略 row")
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
		if row[0] == "计划详略" {
			if row[1] != "detailed" {
				t.Fatalf("计划详略 row=%q, want detailed", row[1])
			}
			return
		}
	}
	t.Fatal("expected 计划详略 row")
}

func TestSettingsPanelHidesDevelopmentOnlyRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".eos", "settings.json")
	panel := NewSettingsPanel(styles.NewStyles(styles.DefaultDarkTheme()), settings.NewManager(path), "zh")

	panel.LoadSettings()

	hiddenRows := map[string]bool{
		"NextMessagePrediction(Global)": true,
		"PlanBubbleColor":               true,
		"LogDir":                        true,
		"WatchDebounceMs":               true,
		"PollIntervalSec":               true,
	}
	for _, row := range panel.table.Rows() {
		if hiddenRows[row[0]] {
			t.Fatalf("development-only row %q should not be visible", row[0])
		}
	}
}
