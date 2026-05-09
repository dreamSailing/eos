package tools

import (
	"context"
	"testing"
)

func TestWorkspaceRootFromContextUsesRemoteOverride(t *testing.T) {
	traceID := "trace-remote-override"
	ClearRemoteRepoContext(traceID)
	defer ClearRemoteRepoContext(traceID)

	ctx := WithWorkspaceRoot(context.Background(), "/workspace/local")
	ctx = WithTraceID(ctx, traceID)

	SetRemoteRepoContext(traceID, RemoteRepoContext{
		Platform:  "github",
		RepoURL:   "https://github.com/acme/demo.git",
		Owner:     "acme",
		Repo:      "demo",
		LocalPath: "/tmp/remote-demo",
	})

	if got := WorkspaceRootFromContext(ctx); got != "/tmp/remote-demo" {
		t.Fatalf("expected remote workspace root, got %q", got)
	}
}

func TestWorkspaceRootFromContextFallsBackToLocal(t *testing.T) {
	traceID := "trace-remote-fallback"
	ClearRemoteRepoContext(traceID)
	defer ClearRemoteRepoContext(traceID)

	ctx := WithWorkspaceRoot(context.Background(), "/workspace/local")
	ctx = WithTraceID(ctx, traceID)

	if got := WorkspaceRootFromContext(ctx); got != "/workspace/local" {
		t.Fatalf("expected local workspace root, got %q", got)
	}
}
