package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dreamSailing/eos/internal/ui/styles"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSIForTest(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestRenderStatusBarShowsExecutionMode(t *testing.T) {
	s := styles.NewStyles(styles.GetTheme("dark"))
	model := New(100, 30, s, "zh")
	model.SetContextVisible(true)
	model.SetExecutionMode("plan")

	out := stripANSIForTest(model.renderStatusBar())
	if !strings.Contains(out, "执行:plan") {
		t.Fatalf("expected status bar to include execution mode, got %q", out)
	}
}

func TestRightKeyAcceptsPredictionOnlyWhenInputEmpty(t *testing.T) {
	s := styles.NewStyles(styles.GetTheme("dark"))
	model := New(100, 30, s, "zh")
	model.SetPrediction("请继续展开这个方案")

	handled, _ := model.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	if !handled {
		t.Fatalf("expected right key to accept prediction")
	}
	if got := model.GetInputValue(); got != "请继续展开这个方案" {
		t.Fatalf("input=%q, want accepted prediction", got)
	}
	if model.HasPrediction() {
		t.Fatalf("expected prediction to clear after accept")
	}
}

func TestRightKeyDoesNotOverrideExistingInput(t *testing.T) {
	s := styles.NewStyles(styles.GetTheme("dark"))
	model := New(100, 30, s, "zh")
	model.SetInputValue("已有输入")
	model.SetPrediction("请继续展开这个方案")

	handled, _ := model.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	if handled {
		t.Fatalf("expected right key to fall through when input already exists")
	}
	if got := model.GetInputValue(); got != "已有输入" {
		t.Fatalf("input=%q, want original input", got)
	}
}
