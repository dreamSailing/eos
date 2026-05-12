package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManager_ExecuteStructuredPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(path1, []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(path2, []byte("b"), 0644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	m := NewManager()
	res := m.ExecuteStructured(context.Background(), []ToolCall{
		{Tool: ToolRead, Parameters: map[string]interface{}{"path": "a.txt"}},
		{Tool: ToolRead, Parameters: map[string]interface{}{"path": "b.txt"}},
	})
	if len(res) != 2 {
		t.Fatalf("unexpected result count: %d", len(res))
	}
	if res[0].Status != "success" || res[1].Status != "success" {
		t.Fatalf("unexpected statuses: %#v", res)
	}
	p0, _ := res[0].Data["path"].(string)
	p1, _ := res[1].Data["path"].(string)
	if filepath.ToSlash(p0) != "a.txt" || filepath.ToSlash(p1) != "b.txt" {
		t.Fatalf("unexpected order: %q %q", p0, p1)
	}
}

func TestManager_ExecuteStructuredCacheAddsMarker(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path1, []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}

	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(old) })

	m := NewManager()
	res1 := m.ExecuteStructured(context.Background(), []ToolCall{
		{Tool: ToolRead, Parameters: map[string]interface{}{"path": "a.txt"}},
	})
	if len(res1) != 1 || res1[0].Status != "success" {
		t.Fatalf("unexpected first result: %#v", res1)
	}
	res2 := m.ExecuteStructured(context.Background(), []ToolCall{
		{Tool: ToolRead, Parameters: map[string]interface{}{"path": "a.txt"}},
	})
	if len(res2) != 1 || res2[0].Status != "success" {
		t.Fatalf("unexpected second result: %#v", res2)
	}
	if len(res2[0].Display) < 8 || res2[0].Display[:8] != "[cached]" {
		t.Fatalf("expected cached marker, display=%q", res2[0].Display)
	}
}

func TestManager_ExecuteStructuredAllowsSafeMetaToolsInReadOnlyContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAllowedTools(ctx, map[string]bool{
		ToolRead:                              true,
		ToolSearch:                            true,
		ToolTodoRead:                          true,
		strings.ToLower(ToolProjectStructure): true,
	})

	res := m.ExecuteStructured(ctx, []ToolCall{
		{Tool: ToolTodoRead, Parameters: map[string]interface{}{}},
		{Tool: ToolProjectStructure, Parameters: map[string]interface{}{"path": "."}},
	})
	if len(res) != 2 {
		t.Fatalf("unexpected result count: %d", len(res))
	}
	for _, r := range res {
		if r.Status != "success" {
			t.Fatalf("expected %s to succeed, got status=%s error=%q display=%q", r.Tool, r.Status, r.Error, r.Display)
		}
		if strings.Contains(r.Error, "permission denied") || strings.Contains(r.Display, "工具未授权") {
			t.Fatalf("expected %s not to be blocked, got error=%q display=%q", r.Tool, r.Error, r.Display)
		}
	}
}

func TestManager_ExecuteBashDirect_UsesWorkspaceRootFromContext(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available on PATH: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".workspace-root-sentinel"), []byte("ok"), 0644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	m := NewManager()
	out, err := m.ExecuteBashDirect(WithWorkspaceRoot(context.Background(), dir), "test -f .workspace-root-sentinel && printf found")
	if err != nil {
		t.Fatalf("ExecuteBashDirect error: %v", err)
	}
	if strings.TrimSpace(out) != "found" {
		t.Fatalf("expected bash command to run in workspace root, got %q", out)
	}
}

func TestManager_ExecuteStructured_ReadOnlyAccessModeBlocksMutatingTool(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")
	res := m.ExecuteStructured(ctx, []ToolCall{
		{Tool: ToolEdit, Parameters: map[string]interface{}{"file": "a.txt", "old": "", "new": "x"}},
	})
	if len(res) != 1 {
		t.Fatalf("unexpected result count: %d", len(res))
	}
	if res[0].Status != "error" {
		t.Fatalf("expected error, got %+v", res[0])
	}
	if !strings.Contains(res[0].Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", res[0].Error)
	}
}
