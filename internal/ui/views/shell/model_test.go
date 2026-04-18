package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"regexp"
	"strings"
	"testing"

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
