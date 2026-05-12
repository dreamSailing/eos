package codectx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecallIntent_UsesNaturalLanguageAliases(t *testing.T) {
	dir := t.TempDir()
	writeIntentFile(t, dir, "internal/runtime/runtime_auth.go", "package runtime\n\nfunc LoginSession() {}\nfunc ValidateToken() {}\n")
	writeIntentFile(t, dir, "internal/runtime/context_env.go", "package runtime\n\nfunc LoadContextEnv() {}\n")

	e := NewEngine(dir)
	if err := e.BuildIndex(); err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	result := e.RecallIntent("登录鉴权那块有问题，帮我排查一下", RecallOptions{Limit: 3})
	if len(result.Terms) == 0 {
		t.Fatalf("expected expanded recall terms")
	}
	if len(result.Candidates) == 0 {
		t.Fatalf("expected recall candidates")
	}
	if got := result.Candidates[0].Path; got != "internal/runtime/runtime_auth.go" {
		t.Fatalf("top candidate = %q, want runtime_auth.go", got)
	}
}

func TestRecallIntent_FusesEvidenceIntoRanking(t *testing.T) {
	dir := t.TempDir()
	writeIntentFile(t, dir, "internal/runtime/loop_impl.go", "package runtime\n\nfunc DetectLoop() {}\n")
	writeIntentFile(t, dir, "internal/runtime/loop_impl_test.go", "package runtime\n\nfunc TestDetectLoop() {}\n")

	e := NewEngine(dir)
	if err := e.BuildIndex(); err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	result := e.RecallIntent("请检查 loop_impl 相关逻辑", RecallOptions{
		Limit: 3,
		Evidence: []RecallEvidence{
			{Path: "internal/runtime/loop_impl.go", Source: RecallSourceRecentChanges, Weight: 0.82, Reason: "最近改动"},
			{Path: "internal/runtime/loop_impl.go", Source: RecallSourceRecentFocus, Weight: 0.74, Reason: "最近焦点"},
		},
	})
	if len(result.Candidates) < 2 {
		t.Fatalf("expected at least 2 candidates, got %+v", result.Candidates)
	}
	if got := result.Candidates[0].Path; got != "internal/runtime/loop_impl.go" {
		t.Fatalf("top candidate = %q, want loop_impl.go", got)
	}
	if result.Candidates[0].Score <= result.Candidates[1].Score {
		t.Fatalf("expected fused evidence to raise top score: %+v", result.Candidates[:2])
	}
	if len(result.Candidates[0].Sources) < 2 {
		t.Fatalf("expected multiple sources on top candidate, got %+v", result.Candidates[0].Sources)
	}
}

func writeIntentFile(t *testing.T, root string, relPath string, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", fullPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", fullPath, err)
	}
}
