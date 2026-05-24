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

func TestManager_ExecuteStructuredBashUsesSandboxRunnerForRelativeOutsideWrite(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	base := filepath.Join(wd, ".sandbox-test-manager-bash")
	dir := filepath.Join(base, "workspace")
	outside := filepath.Join(base, "outside.txt")
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")

	res := m.ExecuteStructured(ctx, []ToolCall{
		{Tool: ToolBash, Parameters: map[string]interface{}{"command": "echo hi > ../outside.txt"}},
	})
	if len(res) != 1 {
		t.Fatalf("unexpected result count: %d", len(res))
	}
	if res[0].Status != "error" {
		t.Fatalf("expected sandbox error, got %+v", res[0])
	}
	if !strings.Contains(res[0].Error, "outside workspace") {
		t.Fatalf("expected outside workspace error, got %q", res[0].Error)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("outside write created %q, stat err = %v", outside, err)
	}
}

func TestManager_DirectFSWriteHonorsSandboxWriteGuard(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "blocked.txt")
	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")

	res := m.fsWrite(ctx, map[string]any{
		"path":    "blocked.txt",
		"content": "nope",
	})
	if res.Status != "error" {
		t.Fatalf("expected read-only sandbox error, got %+v", res)
	}
	if !strings.Contains(res.Error, "read-only") {
		t.Fatalf("expected read-only error, got %q", res.Error)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("direct fs write created %q, stat err = %v", target, err)
	}
}

func TestManager_GitMutationBlocksRepoRootOutsideWorkspace(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	workspace := filepath.Join(repo, "subdir")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")
	res := m.gitCommitStructured(ctx, map[string]interface{}{"message": "blocked"})
	if res.Status != "error" {
		t.Fatalf("expected git mutation sandbox error, got %+v", res)
	}
	if !strings.Contains(res.Error, "outside workspace") {
		t.Fatalf("expected outside workspace error, got %q", res.Error)
	}
}

func TestManager_GitPushSandboxBlocksRepoRootOutsideWorkspace(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	workspace := filepath.Join(repo, "subdir")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")
	res := m.gitPushStructured(ctx, map[string]interface{}{"remote": "origin", "branch": "main"})
	if res.Status != "error" {
		t.Fatalf("expected git push sandbox error, got %+v", res)
	}
	if !strings.Contains(res.Error, "outside workspace") {
		t.Fatalf("expected outside workspace error, got %q", res.Error)
	}
}

func TestManager_EnterWorktreeBlocksRepoRootOutsideWorkspace(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	workspace := filepath.Join(repo, "subdir")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}

	m := NewManager()
	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")
	res := m.enterWorktreeStructured(ctx, map[string]interface{}{"name": "blocked"})
	if res.Status != "error" {
		t.Fatalf("expected worktree sandbox error, got %+v", res)
	}
	if !strings.Contains(res.Error, "outside workspace") {
		t.Fatalf("expected outside workspace error, got %q", res.Error)
	}
}
