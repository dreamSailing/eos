//go:build legacy && !without_lsp
// +build legacy,!without_lsp

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"testing"

	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/lsp"
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

func TestRuntimeCore_LSPStatus_UsesActiveRoot(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()

	mgr := codectx.NewMultiEngine()
	mgr.AddRoot(first)
	mgr.AddRoot(second)
	mgr.SetActive(first)

	rc := &RuntimeCore{workspaceMgr: mgr}
	if status := rc.LSPStatus(); status.Workspace != first {
		t.Fatalf("expected first workspace %q, got %q", first, status.Workspace)
	}

	if rc.SetActiveWorkspaceRoot(second) == nil {
		t.Fatalf("expected second workspace to become active")
	}
	if status := rc.LSPStatus(); status.Workspace != second {
		t.Fatalf("expected second workspace %q, got %q", second, status.Workspace)
	}
}
