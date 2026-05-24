package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficeToolsMetadata(t *testing.T) {
	officeTools := GetOfficeTools()
	if len(officeTools) == 0 {
		t.Fatal("GetOfficeTools() returned empty, expected at least document_generate and document_convert")
	}

	expected := map[string]struct {
		readOnly           bool
		needsSandboxRunner bool
	}{
		ToolDocumentGenerate: {readOnly: false, needsSandboxRunner: true},
		ToolDocumentConvert:  {readOnly: false, needsSandboxRunner: true},
	}

	for _, tool := range officeTools {
		exp, ok := expected[tool.Name]
		if !ok {
			t.Errorf("unexpected Office tool: %s", tool.Name)
			continue
		}
		if tool.Category != "Office 文档" {
			t.Errorf("%s.Category = %q, want %q", tool.Name, tool.Category, "Office 文档")
		}
		if tool.ReadOnly != exp.readOnly {
			t.Errorf("%s.ReadOnly = %v, want %v", tool.Name, tool.ReadOnly, exp.readOnly)
		}
		if tool.NeedsSandboxRunner != exp.needsSandboxRunner {
			t.Errorf("%s.NeedsSandboxRunner = %v, want %v", tool.Name, tool.NeedsSandboxRunner, exp.needsSandboxRunner)
		}
		if tool.RiskLevel != RiskLevelMedium {
			t.Errorf("%s.RiskLevel = %v, want RiskLevelMedium", tool.Name, tool.RiskLevel)
		}
	}
}

func TestOfficeToolsHelperFunctions(t *testing.T) {
	if IsReadOnlyTool(ToolDocumentGenerate) {
		t.Error("document_generate should not be read-only")
	}
	if IsReadOnlyTool(ToolDocumentConvert) {
		t.Error("document_convert should not be read-only")
	}
	if !NeedsSandbox(ToolDocumentGenerate) {
		t.Error("document_generate should need sandbox runner")
	}
	if !NeedsSandbox(ToolDocumentConvert) {
		t.Error("document_convert should need sandbox runner")
	}
}

func TestOfficeToolsAreNotReadOnlyInSecurityPolicy(t *testing.T) {
	for _, toolName := range []string{ToolDocumentGenerate, ToolDocumentConvert} {
		call := ToolCall{Tool: toolName, Parameters: map[string]interface{}{}}
		if isReadOnlyToolCall(call) {
			t.Errorf("%s should not be classified as read-only by security policy", toolName)
		}
	}
}

func TestOfficeToolsClassifiedAsDangerous(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		parameters map[string]interface{}
	}{
		{
			name:       "document_generate docx",
			tool:       ToolDocumentGenerate,
			parameters: map[string]interface{}{"format": "docx", "path": "out.docx"},
		},
		{
			name:       "document_generate xlsx",
			tool:       ToolDocumentGenerate,
			parameters: map[string]interface{}{"format": "xlsx", "path": "out.xlsx"},
		},
		{
			name:       "document_convert",
			tool:       ToolDocumentConvert,
			parameters: map[string]interface{}{"source_path": "in.docx", "target_format": "pdf"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, dangerous := ClassifyToolDanger(ToolCall{
				Tool:       tt.tool,
				Parameters: tt.parameters,
			})
			if !dangerous {
				t.Errorf("ClassifyToolDanger(%s) dangerous=false, want true", tt.tool)
			}
		})
	}
}

func TestDocumentGenerateReadOnlyBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	m := NewManager()
	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "out/report.docx",
		"title":   "Test",
		"content": "body",
	})
	if r.Status != "error" {
		t.Fatalf("expected read-only error, got status=%q", r.Status)
	}
	if !strings.Contains(r.Error, "read-only") {
		t.Fatalf("expected read-only error message, got %q", r.Error)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "report.docx")); !os.IsNotExist(err) {
		t.Fatal("file was created despite read-only sandbox")
	}
}

func TestDocumentGenerateWorkspaceWriteBlocksOutside(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	outside := filepath.Join(dir, "outside")
	for _, d := range []string{workspace, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", d, err)
		}
	}

	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")

	m := NewManager()
	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "../outside/report.docx",
		"title":   "Test",
		"content": "body",
	})
	if r.Status != "error" {
		t.Fatalf("expected sandbox error for outside path, got status=%q", r.Status)
	}
}

func TestDocumentGenerateWorkspaceWriteAllowsInside(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")

	m := NewManager()
	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "inner/report.docx",
		"title":   "Test",
		"content": "body",
	})
	if r.Status != "success" {
		t.Fatalf("expected success, got %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "inner", "report.docx")); err != nil {
		t.Fatalf("generated file missing: %v", err)
	}
}

func TestDocumentConvertReadOnlyBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.docx")
	if err := os.WriteFile(srcPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	m := NewManager()
	r := m.documentConvertStructured(ctx, map[string]interface{}{
		"source_path":      "source.docx",
		"target_format":    "pdf",
		"destination_path": "output.pdf",
	})
	if r.Status != "error" {
		t.Fatalf("expected read-only error, got status=%q", r.Status)
	}
	if !strings.Contains(r.Error, "read-only") {
		t.Fatalf("expected read-only error message, got %q", r.Error)
	}
}

func TestDocumentConvertWorkspaceWriteBlocksOutside(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	outside := filepath.Join(dir, "outside")
	for _, d := range []string{workspace, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", d, err)
		}
	}
	srcPath := filepath.Join(workspace, "source.docx")
	if err := os.WriteFile(srcPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")

	m := NewManager()
	r := m.documentConvertStructured(ctx, map[string]interface{}{
		"source_path":      "source.docx",
		"target_format":    "pdf",
		"destination_path": "../outside/output.pdf",
	})
	if r.Status != "error" {
		t.Fatalf("expected sandbox error for outside path, got status=%q", r.Status)
	}
}

func TestDocumentConvertWorkspaceWriteAllowsInside(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")

	m := NewManager()
	gen := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "source.docx",
		"title":   "Test",
		"content": "body",
	})
	if gen.Status != "success" {
		t.Fatalf("generate failed: %+v", gen)
	}

	r := m.documentConvertStructured(ctx, map[string]interface{}{
		"source_path":      "source.docx",
		"target_format":    "pdf",
		"destination_path": "output.pdf",
	})
	if r.Status != "success" {
		t.Fatalf("expected success, got %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "output.pdf")); err != nil {
		t.Fatalf("converted file missing: %v", err)
	}
}

func TestDocumentGenerateXLSXReadOnlyBlocks(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	m := NewManager()
	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format": "xlsx",
		"path":   "data.xlsx",
		"structured_content": map[string]interface{}{
			"sheets": []map[string]interface{}{
				{"name": "Sheet1", "rows": [][]string{{"A", "B"}, {"1", "2"}}},
			},
		},
	})
	if r.Status != "error" {
		t.Fatalf("expected read-only error for xlsx, got status=%q", r.Status)
	}
	if !strings.Contains(r.Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", r.Error)
	}
}

func TestDocumentGeneratePDFReadOnlyBlocks(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	m := NewManager()
	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "pdf",
		"path":    "report.pdf",
		"title":   "Test PDF",
		"content": "content",
	})
	if r.Status != "error" {
		t.Fatalf("expected read-only error for pdf, got status=%q", r.Status)
	}
	if !strings.Contains(r.Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", r.Error)
	}
}

func TestOfficeToolsCannotBypassAccessModeCheck(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	for _, toolName := range []string{ToolDocumentGenerate, ToolDocumentConvert} {
		allowed, reason := AccessModeAllowsToolCall(ctx, ToolCall{
			Tool:       toolName,
			Parameters: map[string]interface{}{"format": "docx", "path": "test.docx"},
		})
		if allowed {
			t.Errorf("AccessModeAllowsToolCall(%s) in read-only mode should be blocked", toolName)
		}
		if reason == "" {
			t.Errorf("AccessModeAllowsToolCall(%s) should return a reason when blocked", toolName)
		}
	}
}
