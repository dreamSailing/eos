//go:build !without_lsp
// +build !without_lsp

package bridge

import (
	"strings"
	"testing"

	"github.com/dreamSailing/vb-coding/internal/lsp"
)

func TestFormatProblemsAndDiagnosticsMarkdown(t *testing.T) {
	diags := map[string][]lsp.Diagnostic{
		"file:///C:/repo/main.go": {
			{Severity: lsp.SeverityError, Message: "boom", Range: lsp.Range{Start: lsp.Position{Line: 2}}},
			{Severity: lsp.SeverityWarning, Message: "warn", Range: lsp.Range{Start: lsp.Position{Line: 4}}},
		},
	}
	out := formatProblemsAndDiagnosticsMarkdown(diags, "C:\\repo")
	if strings.TrimSpace(out) == "" {
		t.Fatalf("expected non-empty output")
	}
	if !strings.Contains(out, "main.go") {
		t.Fatalf("expected file basename in output, got: %s", out)
	}
	if !strings.Contains(out, "Line 3") {
		t.Fatalf("expected 1-based line numbers, got: %s", out)
	}
	if !strings.Contains(out, "**Summary**") {
		t.Fatalf("expected summary, got: %s", out)
	}
}

func TestURIToLocalPath_WindowsDrive(t *testing.T) {
	p := uriToLocalPath("file:///C:/repo/main.go")
	if strings.Contains(p, "file://") {
		t.Fatalf("expected local path, got: %s", p)
	}
	if !strings.Contains(strings.ToLower(p), "c:") {
		t.Fatalf("expected drive path, got: %s", p)
	}
}

