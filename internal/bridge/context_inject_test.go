//go:build legacy

package bridge

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/session"
)

func TestRecentVersionedFilesListIgnoresMetadataAndUsesModTime(t *testing.T) {
	root := t.TempDir()
	olderDir := filepath.Join(root, "dir", "older.txt")
	newerDir := filepath.Join(root, "dir", "newer.txt")
	if err := os.MkdirAll(olderDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(olderDir) error = %v", err)
	}
	if err := os.MkdirAll(newerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(newerDir) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "_checkpoints"), 0o755); err != nil {
		t.Fatalf("MkdirAll(_checkpoints) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "_index.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(_index.jsonl) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "meta.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(meta.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "_checkpoints", "trace.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(checkpoint) error = %v", err)
	}

	older := filepath.Join(olderDir, "20260503-120000.000000001.content")
	newer := filepath.Join(newerDir, "20260503-120000.000000002-1.content")
	if err := os.WriteFile(older, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(older) error = %v", err)
	}
	if err := os.WriteFile(newer, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(newer) error = %v", err)
	}

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes(older) error = %v", err)
	}
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatalf("Chtimes(newer) error = %v", err)
	}

	got := recentVersionedFilesList(root, 4)
	want := []string{"dir/newer.txt", "dir/older.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recentVersionedFilesList() = %#v, want %#v", got, want)
	}
}

func TestComputeInjectBudgetBytesUsesRemainingPromptBudget(t *testing.T) {
	cm := session.NewContextManager()
	cm.SetMaxChars(400)
	for i := 0; i < 6; i++ {
		cm.AddUser(strings.Repeat("budget ", 40))
		cm.AddAssistant(strings.Repeat("reply ", 40))
	}

	rc := &RuntimeCore{cm: cm}
	if got := computeInjectBudgetBytes(rc, 64); got != 0 {
		t.Fatalf("computeInjectBudgetBytes() = %d, want 0 when prompt budget is exhausted", got)
	}
}

func TestBuildInjectedContextPackageSummarizesTrimmedAndOmittedFiles(t *testing.T) {
	root := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(root)
	t.Cleanup(func() { _ = os.Chdir(old) })

	fileA := filepath.Join(root, "internal", "bridge", "runtime_invoke.go")
	fileB := filepath.Join(root, "internal", "session", "context_build.go")
	if err := os.MkdirAll(filepath.Dir(fileA), 0o755); err != nil {
		t.Fatalf("MkdirAll(fileA) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(fileB), 0o755); err != nil {
		t.Fatalf("MkdirAll(fileB) error = %v", err)
	}
	if err := os.WriteFile(fileA, []byte(strings.Repeat("func invoke() { helperCall() }\n", 160)), 0o644); err != nil {
		t.Fatalf("WriteFile(fileA) error = %v", err)
	}
	if err := os.WriteFile(fileB, []byte(strings.Repeat("func build() { contextBudget() }\n", 160)), 0o644); err != nil {
		t.Fatalf("WriteFile(fileB) error = %v", err)
	}

	rc := &RuntimeCore{cm: session.NewContextManager()}
	pkg := rc.buildInjectedContextPackage("trim runtime invoke budget", []codectx.Suggestion{
		{Path: "internal/bridge/runtime_invoke.go", Symbols: []string{"invoke"}},
		{Path: "internal/session/context_build.go", Symbols: []string{"build"}},
	}, 1400)

	if pkg.usedBytes > 1400 {
		t.Fatalf("package usedBytes = %d, want <= 1400", pkg.usedBytes)
	}
	if len(pkg.entries) == 0 {
		t.Fatalf("expected at least one injected entry")
	}
	if pkg.trimmedFiles == 0 && pkg.omittedFiles == 0 {
		t.Fatalf("expected trimmed or omitted files, got %#v", pkg)
	}
	if !strings.Contains(pkg.summary, "AutoContext package:") {
		t.Fatalf("expected summary, got %q", pkg.summary)
	}
}
