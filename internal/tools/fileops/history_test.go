package fileops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setHistoryTestHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func TestSaveVersionWithExtraWritesToGlobalWorkspaceStore(t *testing.T) {
	setHistoryTestHome(t)
	workspace := t.TempDir()
	target := filepath.Join(workspace, "dir", "file.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(target, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	ops := NewFileOperations()
	ops.SetRoot(workspace)
	version, err := ops.SaveVersionWithExtra(target, "old\n", VersionExtra{TraceID: "trace-1", Tool: "test", Operation: "write"})
	if err != nil {
		t.Fatalf("SaveVersionWithExtra() error = %v", err)
	}

	versionPath := filepath.Join(VersionFilesRoot(workspace), "dir", "file.txt", version.ID+".content")
	if _, err := os.Stat(versionPath); err != nil {
		t.Fatalf("expected version content under global store, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(VersionWorkspaceRoot(workspace), "_index.jsonl")); err != nil {
		t.Fatalf("expected global version index, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(VersionWorkspaceRoot(workspace), "_checkpoints", "trace-1.json")); err != nil {
		t.Fatalf("expected checkpoint under global store, stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(LegacyVersionWorkspaceRoot(workspace), "dir", "file.txt", version.ID+".content")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy workspace store to stay untouched, stat error = %v", err)
	}
}

func TestWorkspaceVersionNamespaceIDStable(t *testing.T) {
	root := filepath.Join("C:\\", "Projects", "Demo")
	if got, want := workspaceVersionNamespaceID(root), workspaceVersionNamespaceID(root); got != want {
		t.Fatalf("workspaceVersionNamespaceID() unstable: got %q want %q", got, want)
	}
}

func TestLoadCheckpointUnderFallsBackToLegacyWorkspaceStore(t *testing.T) {
	setHistoryTestHome(t)
	workspace := t.TempDir()
	legacyDir := filepath.Join(LegacyVersionWorkspaceRoot(workspace), "_checkpoints")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(legacyDir) error = %v", err)
	}

	expected := Checkpoint{
		TraceID:   "trace-legacy",
		CreatedAt: "2026-05-03T00:00:00Z",
		Files:     map[string]string{"dir/file.txt": "v1"},
	}
	raw, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "trace-legacy.json"), raw, 0o644); err != nil {
		t.Fatalf("WriteFile(legacy checkpoint) error = %v", err)
	}

	checkpoint, err := LoadCheckpointUnder(workspace, "trace-legacy")
	if err != nil {
		t.Fatalf("LoadCheckpointUnder() error = %v", err)
	}
	if checkpoint.TraceID != expected.TraceID {
		t.Fatalf("checkpoint.TraceID = %q, want %q", checkpoint.TraceID, expected.TraceID)
	}
	if checkpoint.Files["dir/file.txt"] != "v1" {
		t.Fatalf("checkpoint.Files = %#v, want legacy content", checkpoint.Files)
	}
}
