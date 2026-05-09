package bridge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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
