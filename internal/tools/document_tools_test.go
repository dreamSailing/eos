package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentGenerateStructuredDOCX(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	m := NewManager()
	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "out/sample.docx",
		"title":   "测试标题",
		"content": "第一段\n\n第二段",
	})
	if r.Status != "success" {
		t.Fatalf("generate failed: %+v", r)
	}
	target := filepath.Join(dir, "out", "sample.docx")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("generated file missing: %v", err)
	}
}

func TestDocumentConvertStructuredFallback(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	m := NewManager()

	gen := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "source.docx",
		"title":   "转换测试",
		"content": "表格前的文本",
	})
	if gen.Status != "success" {
		t.Fatalf("generate failed: %+v", gen)
	}

	r := m.documentConvertStructured(ctx, map[string]interface{}{
		"source_path":      "source.docx",
		"target_format":    "pdf",
		"destination_path": "converted.pdf",
		"fidelity":         "content",
	})
	if r.Status != "success" {
		t.Fatalf("convert failed: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "converted.pdf")); err != nil {
		t.Fatalf("converted file missing: %v", err)
	}
}

func TestReadStructuredDOCX(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	m := NewManager()
	gen := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "readme.docx",
		"title":   "读取测试",
		"content": "正文内容",
	})
	if gen.Status != "success" {
		t.Fatalf("generate failed: %+v", gen)
	}
	r := m.readStructured(ctx, map[string]interface{}{"path": "readme.docx"})
	if r.Status != "success" {
		t.Fatalf("read failed: %+v", r)
	}
	if got, _ := r.Data["format"].(string); got != "docx" {
		t.Fatalf("format = %q, want docx", got)
	}
}
