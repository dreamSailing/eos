package tools

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
