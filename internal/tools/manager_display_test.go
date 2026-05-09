package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"testing"
)

func TestToolResult_FormatForUI_BashSuccess(t *testing.T) {
	r := ToolResult{
		Type:   ToolTypeTool,
		Tool:   "bash",
		Status: ToolStatusSuccess,
		Display: "line1\nline2\nline3",
		Data: map[string]interface{}{
			"command": "dir",
		},
	}
	out := r.FormatForUI()
	if !strings.Contains(out, "✓") || !strings.Contains(out, "Bash") {
		t.Fatalf("expected success marker and tool name, got %q", out)
	}
	if !strings.Contains(out, "→") {
		t.Fatalf("expected summary arrow, got %q", out)
	}
}

func TestToolResult_FormatForUI_ProjectStructure(t *testing.T) {
	r := ToolResult{
		Type:    ToolTypeTool,
		Tool:    ToolProjectStructure,
		Status:  ToolStatusSuccess,
		Display: "Project structure for .:\n├── a\n└── b\n",
		Data: map[string]interface{}{
			"path": ".",
		},
	}
	out := r.FormatForUI()
	if !strings.Contains(out, "✓") || !strings.Contains(out, "ProjectStructure") {
		t.Fatalf("expected success marker and tool name, got %q", out)
	}
	if !strings.Contains(out, "→") {
		t.Fatalf("expected summary arrow, got %q", out)
	}
}
