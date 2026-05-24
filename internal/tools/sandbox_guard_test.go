package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dreamSailing/eos/pkg/sandbox"
)

func TestSandboxPolicyForToolContextWorkspaceWriteFallsBackToWorkingDir(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	policy := sandboxPolicyForToolContext(WithAccessMode(context.Background(), "workspace-write"))
	if policy.Mode != sandbox.ModeWorkspaceWrite {
		t.Fatalf("Mode=%q, want %q", policy.Mode, sandbox.ModeWorkspaceWrite)
	}
	if policy.WorkspaceRoot != dir {
		t.Fatalf("WorkspaceRoot=%q, want %q", policy.WorkspaceRoot, dir)
	}
}

func TestSandboxPolicyForToolContextIncludesTemporaryDirsAsWritableRoots(t *testing.T) {
	ctx := WithWorkspaceRoot(context.Background(), t.TempDir())
	ctx = WithAccessMode(ctx, "workspace-write")
	policy := sandboxPolicyForToolContext(ctx)

	tempDir := filepath.Clean(os.TempDir())
	for _, root := range policy.WritableRoots {
		if filepath.Clean(root) == tempDir {
			return
		}
	}
	t.Fatalf("WritableRoots=%v, want temp dir %q", policy.WritableRoots, tempDir)
}

func TestGitMutationSandboxResultSandboxIncludesRepoRootMetadata(t *testing.T) {
	repo := t.TempDir()
	workspace := filepath.Join(repo, "subdir")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", workspace, err)
	}

	ctx := WithWorkspaceRoot(context.Background(), workspace)
	ctx = WithAccessMode(ctx, "workspace-write")
	result, blocked := gitMutationSandboxResult(ctx, ToolGitPush, repo)
	if !blocked {
		t.Fatal("gitMutationSandboxResult() blocked = false, want true")
	}
	if result.Tool != ToolGitPush {
		t.Fatalf("Tool=%q, want %q", result.Tool, ToolGitPush)
	}
	if result.Status != "error" {
		t.Fatalf("Status=%q, want error", result.Status)
	}
	if got, _ := result.Data["repo_root"].(string); got != filepath.ToSlash(repo) {
		t.Fatalf("repo_root=%q, want %q", got, filepath.ToSlash(repo))
	}
}

func TestGitMutationSandboxResultSandboxAllowsRepoRootInsideWorkspace(t *testing.T) {
	repo := t.TempDir()
	ctx := WithWorkspaceRoot(context.Background(), repo)
	ctx = WithAccessMode(ctx, "workspace-write")

	if result, blocked := gitMutationSandboxResult(ctx, ToolGitCommit, repo); blocked {
		t.Fatalf("gitMutationSandboxResult() = %+v, want allowed", result)
	}
}
