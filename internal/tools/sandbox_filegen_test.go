package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/ai"
)

func TestDocumentGenerateSandboxReadOnlyBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "out/report.docx",
		"title":   "测试",
		"content": "正文",
	})
	if r.Status != "error" {
		t.Fatalf("expected read-only sandbox error, got %+v", r)
	}
	if !strings.Contains(r.Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", r.Error)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "report.docx")); !os.IsNotExist(err) {
		t.Fatalf("file was created despite read-only sandbox")
	}
}

func TestDocumentGenerateSandboxWorkspaceWriteBlocksOutside(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	workspace := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")

	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "../outside/report.docx",
		"title":   "测试",
		"content": "正文",
	})
	if r.Status != "error" {
		t.Fatalf("expected sandbox error for outside path, got %+v", r)
	}
	if !strings.Contains(r.Error, "outside") && !strings.Contains(r.Display, "超出") {
		t.Fatalf("expected outside workspace error, got error=%q display=%q", r.Error, r.Display)
	}
}

func TestDocumentGenerateSandboxWorkspaceWriteAllowsInside(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")

	r := m.documentGenerateStructured(ctx, map[string]interface{}{
		"format":  "docx",
		"path":    "inner/report.docx",
		"title":   "测试",
		"content": "正文",
	})
	if r.Status != "success" {
		t.Fatalf("expected success for inside workspace, got %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "inner", "report.docx")); err != nil {
		t.Fatalf("generated file missing: %v", err)
	}
}

func TestDocumentConvertSandboxReadOnlyBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.docx")
	if err := os.WriteFile(srcPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	r := m.documentConvertStructured(ctx, map[string]interface{}{
		"source_path":      "source.docx",
		"target_format":    "pdf",
		"destination_path": "output.pdf",
	})
	if r.Status != "error" {
		t.Fatalf("expected read-only sandbox error, got %+v", r)
	}
	if !strings.Contains(r.Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", r.Error)
	}
}

func TestDocumentConvertSandboxWorkspaceWriteBlocksOutside(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	srcPath := filepath.Join(workspace, "source.docx")
	if err := os.WriteFile(srcPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")

	r := m.documentConvertStructured(ctx, map[string]interface{}{
		"source_path":      "source.docx",
		"target_format":    "pdf",
		"destination_path": "../outside/output.pdf",
	})
	if r.Status != "error" {
		t.Fatalf("expected sandbox error for outside path, got %+v", r)
	}
	if !strings.Contains(r.Error, "outside") && !strings.Contains(r.Display, "超出") {
		t.Fatalf("expected outside workspace error, got error=%q display=%q", r.Error, r.Display)
	}
}

func TestNotebookEditSandboxReadOnlyBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	notebook := map[string]interface{}{
		"cells": []map[string]interface{}{
			{"id": "cell-1", "cell_type": "code", "source": []string{"print('hello')"}},
		},
		"metadata":      map[string]interface{}{},
		"nbformat":      4,
		"nbformat_minor": 5,
	}
	data, err := json.Marshal(notebook)
	if err != nil {
		t.Fatalf("marshal notebook: %v", err)
	}
	if err := os.WriteFile(nbPath, data, 0644); err != nil {
		t.Fatalf("write notebook: %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	r := m.notebookEditStructured(ctx, map[string]interface{}{
		"path":      "test.ipynb",
		"edit_mode": "replace",
		"cell_id":   "cell-1",
		"source":    "print('world')",
	})
	if r.Status != "error" {
		t.Fatalf("expected read-only sandbox error, got %+v", r)
	}
	if !strings.Contains(r.Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", r.Error)
	}

	raw, _ := os.ReadFile(nbPath)
	if strings.Contains(string(raw), "world") {
		t.Fatalf("notebook was modified despite read-only sandbox")
	}
}

func TestNotebookEditSandboxWorkspaceWriteBlocksOutside(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	nbPath := filepath.Join(outside, "test.ipynb")
	notebook := map[string]interface{}{
		"cells": []map[string]interface{}{
			{"id": "cell-1", "cell_type": "code", "source": []string{"print('hello')"}},
		},
		"metadata":      map[string]interface{}{},
		"nbformat":      4,
		"nbformat_minor": 5,
	}
	data, _ := json.Marshal(notebook)
	if err := os.WriteFile(nbPath, data, 0644); err != nil {
		t.Fatalf("write notebook: %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")

	r := m.notebookEditStructured(ctx, map[string]interface{}{
		"path":      "../outside/test.ipynb",
		"edit_mode": "replace",
		"cell_id":   "cell-1",
		"source":    "print('world')",
	})
	if r.Status != "error" {
		t.Fatalf("expected sandbox error for outside path, got %+v", r)
	}
	if !strings.Contains(r.Error, "outside") && !strings.Contains(r.Display, "超出") {
		t.Fatalf("expected outside workspace error, got error=%q display=%q", r.Error, r.Display)
	}
}

func TestSaveGeneratedMediaSetSandboxReadOnlyBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	items := []ai.GeneratedMedia{
		{Bytes: []byte("fake-image-data"), MIMEType: "image/png"},
	}
	_, err := saveGeneratedMediaSet(ctx, "outputs/test.png", items, "image", ".png")
	if err == nil {
		t.Fatal("expected read-only sandbox error, got nil")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %q", err.Error())
	}
	if _, statErr := os.Stat(filepath.Join(dir, "outputs", "test.png")); !os.IsNotExist(statErr) {
		t.Fatal("file was created despite read-only sandbox")
	}
}

func TestSaveGeneratedMediaSetSandboxWorkspaceWriteBlocksOutside(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")

	items := []ai.GeneratedMedia{
		{Bytes: []byte("fake-image-data"), MIMEType: "image/png"},
	}
	_, err := saveGeneratedMediaSet(ctx, "../outside/test.png", items, "image", ".png")
	if err == nil {
		t.Fatal("expected sandbox error for outside path, got nil")
	}
	if !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected outside workspace error, got %q", err.Error())
	}
}

func TestSaveGeneratedMediaSetSandboxWorkspaceWriteAllowsInside(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")

	items := []ai.GeneratedMedia{
		{Bytes: []byte("fake-image-data"), MIMEType: "image/png"},
	}
	paths, err := saveGeneratedMediaSet(ctx, "outputs/test.png", items, "image", ".png")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if _, statErr := os.Stat(filepath.Join(dir, "outputs", "test.png")); statErr != nil {
		t.Fatalf("generated file missing: %v", statErr)
	}
}

func TestSaveGeneratedMediaSetSandboxReadOnlyBlocksAutoPath(t *testing.T) {
	dir := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	items := []ai.GeneratedMedia{
		{Bytes: []byte("fake-video-data"), MIMEType: "video/mp4"},
	}
	_, err := saveGeneratedMediaSet(ctx, "", items, "video", ".mp4")
	if err == nil {
		t.Fatal("expected read-only sandbox error for auto-generated path, got nil")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %q", err.Error())
	}
}

func TestBrowserScreenshotSandboxReadOnlyBlocksWrite(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	r := m.browserScreenshotStructured(ctx, map[string]interface{}{
		"path": "screenshots/page.png",
	})
	if r.Status != "error" {
		t.Fatalf("expected read-only sandbox error, got %+v", r)
	}
	if !strings.Contains(r.Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", r.Error)
	}
}

func TestBrowserScreenshotSandboxWorkspaceWriteBlocksOutside(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")

	r := m.browserScreenshotStructured(ctx, map[string]interface{}{
		"path": "../outside/screenshot.png",
	})
	if r.Status != "error" {
		t.Fatalf("expected sandbox error for outside path, got %+v", r)
	}
	if !strings.Contains(r.Error, "outside") && !strings.Contains(r.Display, "超出") {
		t.Fatalf("expected outside workspace error, got error=%q display=%q", r.Error, r.Display)
	}
}

func TestExecuteStructuredDocumentGenerateReadOnlySandbox(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	res := m.ExecuteStructured(ctx, []ToolCall{
		{Tool: ToolDocumentGenerate, Parameters: map[string]interface{}{
			"format":  "docx",
			"path":    "report.docx",
			"title":   "测试",
			"content": "正文",
		}},
	})
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Status != "error" {
		t.Fatalf("expected error, got %+v", res[0])
	}
	if !strings.Contains(res[0].Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", res[0].Error)
	}
}

func TestExecuteStructuredNotebookEditReadOnlySandbox(t *testing.T) {
	dir := t.TempDir()
	nbPath := filepath.Join(dir, "test.ipynb")
	notebook := map[string]interface{}{
		"cells": []map[string]interface{}{
			{"id": "c1", "cell_type": "code", "source": []string{"x=1"}},
		},
		"metadata":       map[string]interface{}{},
		"nbformat":       4,
		"nbformat_minor": 5,
	}
	data, _ := json.Marshal(notebook)
	if err := os.WriteFile(nbPath, data, 0644); err != nil {
		t.Fatalf("write notebook: %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	res := m.ExecuteStructured(ctx, []ToolCall{
		{Tool: ToolNotebookEdit, Parameters: map[string]interface{}{
			"path":      "test.ipynb",
			"edit_mode": "replace",
			"cell_id":   "c1",
			"source":    "x=2",
		}},
	})
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Status != "error" {
		t.Fatalf("expected error, got %+v", res[0])
	}
	if !strings.Contains(res[0].Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", res[0].Error)
	}
}

func TestExecuteStructuredImageGenerateReadOnlySandbox(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	res := m.ExecuteStructured(ctx, []ToolCall{
		{Tool: ToolImageGenerate, Parameters: map[string]interface{}{
			"prompt":      "test image",
			"output_path": "outputs/test.png",
		}},
	})
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Status != "error" {
		t.Fatalf("expected error, got %+v", res[0])
	}
}

func TestExecuteStructuredVideoGenerateReadOnlySandbox(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	res := m.ExecuteStructured(ctx, []ToolCall{
		{Tool: ToolVideoGenerate, Parameters: map[string]interface{}{
			"prompt":      "test video",
			"output_path": "outputs/test.mp4",
		}},
	})
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].Status != "error" {
		t.Fatalf("expected error, got %+v", res[0])
	}
}
