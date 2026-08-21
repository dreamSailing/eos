package shell

// selection_test.go — 内容区框选高亮（applySelectionHighlight）测试。

import (
	"strings"
	"testing"
)

func TestApplySelectionHighlight(t *testing.T) {
	view := "a\nb\nc"
	got := applySelectionHighlight(view, 0, 1, 1)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if !strings.Contains(lines[1], "\x1b[7m") {
		t.Fatalf("line 1 should be highlighted: %q", lines[1])
	}
	if strings.Contains(lines[0], "\x1b[7m") || strings.Contains(lines[2], "\x1b[7m") {
		t.Fatalf("lines 0/2 should not be highlighted: %q", got)
	}
}

func TestApplySelectionHighlight_Offset(t *testing.T) {
	view := "a\nb\nc"
	// yOffset=5：可见行对应物理行 5,6,7，高亮物理行 6 → 第二个可见行。
	got := applySelectionHighlight(view, 5, 6, 6)
	lines := strings.Split(got, "\n")
	if !strings.Contains(lines[1], "\x1b[7m") {
		t.Fatalf("line index 1 (phys 6) should be highlighted: %q", got)
	}
	if strings.Contains(lines[0], "\x1b[7m") {
		t.Fatalf("line index 0 (phys 5) should not be highlighted: %q", got)
	}
}

func TestApplySelectionHighlight_NoSelection(t *testing.T) {
	view := "a\nb"
	if got := applySelectionHighlight(view, 0, -1, -1); got != view {
		t.Fatalf("no-selection should return unchanged, got %q", got)
	}
}

func TestApplySelectionHighlight_Range(t *testing.T) {
	view := "1\n2\n3\n4"
	got := applySelectionHighlight(view, 0, 1, 2)
	lines := strings.Split(got, "\n")
	if strings.Contains(lines[0], "\x1b[7m") {
		t.Fatalf("line 0 should not be highlighted: %q", got)
	}
	if !strings.Contains(lines[1], "\x1b[7m") || !strings.Contains(lines[2], "\x1b[7m") {
		t.Fatalf("lines 1-2 should be highlighted: %q", got)
	}
	if strings.Contains(lines[3], "\x1b[7m") {
		t.Fatalf("line 3 should not be highlighted: %q", got)
	}
}
