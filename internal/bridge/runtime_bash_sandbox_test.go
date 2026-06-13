//go:build legacy

package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
)

func TestRuntimeCoreExecuteBashHonorsReadOnlyAccessMode(t *testing.T) {
	rc := NewRuntimeCore(session.NewContextManager(), tools.NewManager(), nil)
	defer rc.Shutdown()
	rc.SetAccessMode("read-only")

	if _, err := rc.ExecuteBash(context.Background(), "echo hi"); err == nil {
		t.Fatal("ExecuteBash(read-only) error = nil, want sandbox policy error")
	} else if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("ExecuteBash(read-only) error = %v, want read-only policy error", err)
	}
}

func TestRuntimeCoreExecuteBashBlocksWorkspaceWriteOutsideTarget(t *testing.T) {
	rc := NewRuntimeCore(session.NewContextManager(), tools.NewManager(), nil)
	defer rc.Shutdown()

	workspaceRoot := t.TempDir()
	if rc.AddWorkspaceRoot(workspaceRoot) == nil {
		t.Fatalf("AddWorkspaceRoot(%q) returned nil", workspaceRoot)
	}
	if rc.SetActiveWorkspaceRoot(workspaceRoot) == nil {
		t.Fatalf("SetActiveWorkspaceRoot(%q) returned nil", workspaceRoot)
	}

	outsidePath := filepath.Join(filepath.Dir(workspaceRoot), "outside.txt")
	cmd := "echo hi > " + filepath.ToSlash(outsidePath)
	if _, err := rc.ExecuteBash(context.Background(), cmd); err == nil {
		t.Fatal("ExecuteBash(outside write) error = nil, want sandbox policy error")
	} else if !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("ExecuteBash(outside write) error = %v, want outside workspace policy error", err)
	}

	if _, err := os.Stat(outsidePath); !os.IsNotExist(err) {
		t.Fatalf("outside write created %q, stat err = %v", outsidePath, err)
	}
}

func TestRuntimeCoreEnterWorktreeBlocksRepoRootOutsideWorkspace(t *testing.T) {
	rc := NewRuntimeCore(session.NewContextManager(), tools.NewManager(), nil)
	defer rc.Shutdown()

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	workspaceRoot := filepath.Join(repo, "subdir")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}
	if rc.AddWorkspaceRoot(workspaceRoot) == nil {
		t.Fatalf("AddWorkspaceRoot(%q) returned nil", workspaceRoot)
	}
	if rc.SetActiveWorkspaceRoot(workspaceRoot) == nil {
		t.Fatalf("SetActiveWorkspaceRoot(%q) returned nil", workspaceRoot)
	}

	if _, err := rc.EnterWorktree(context.Background(), "blocked"); err == nil {
		t.Fatal("EnterWorktree() error = nil, want sandbox policy error")
	} else if !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("EnterWorktree() error = %v, want outside workspace policy error", err)
	}
}
