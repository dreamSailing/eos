package scheduler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_SaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedules.json")
	store := NewStore(path)

	original := []Schedule{
		{ID: "s1", Name: "Morning Brief", Enabled: true, Cron: "0 8 * * *", Kind: TaskKindEOSCall},
		{ID: "s2", Name: "Backup", Enabled: false, Cron: "0 0 * * 0", Kind: TaskKindShell, Workspace: "/tmp"},
	}
	if err := store.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != len(original) {
		t.Fatalf("length mismatch: %d vs %d", len(loaded), len(original))
	}
	for i, s := range loaded {
		if s.ID != original[i].ID {
			t.Fatalf("[%d] ID mismatch: %q vs %q", i, s.ID, original[i].ID)
		}
		if s.Name != original[i].Name {
			t.Fatalf("[%d] Name mismatch: %q vs %q", i, s.Name, original[i].Name)
		}
		if s.Cron != original[i].Cron {
			t.Fatalf("[%d] Cron mismatch: %q vs %q", i, s.Cron, original[i].Cron)
		}
		if s.Enabled != original[i].Enabled {
			t.Fatalf("[%d] Enabled mismatch: %v vs %v", i, s.Enabled, original[i].Enabled)
		}
	}
}

func TestStore_Load_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")
	store := NewStore(path)

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil, got %v", items)
	}
}

func TestStore_Save_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "schedules.json")
	store := NewStore(path)

	if err := store.Save([]Schedule{{ID: "s1"}}); err != nil {
		t.Fatalf("Save should create dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}

func TestStore_Save_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedules.json")
	store := NewStore(path)

	if err := store.Save([]Schedule{{ID: "s1"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("tmp file should not exist after atomic save")
	}
}

func TestStore_Load_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedules.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)

	items, err := store.Load()
	if err != nil {
		t.Fatalf("Load on empty store: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestStore_Path(t *testing.T) {
	path := "/tmp/test/schedules.json"
	store := NewStore(path)
	if got := store.Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
}

func TestStore_NilReceiver(t *testing.T) {
	var store *Store
	if got := store.Path(); got != "" {
		t.Fatalf("nil Path() = %q, want empty", got)
	}
	items, err := store.Load()
	if err != nil {
		t.Fatalf("nil Load() error: %v", err)
	}
	if items != nil {
		t.Fatalf("nil Load() = %v, want nil", items)
	}
	if err := store.Save(nil); err != nil {
		t.Fatalf("nil Save() error: %v", err)
	}
}

func TestStore_NilPath(t *testing.T) {
	store := NewStore("")
	items, err := store.Load()
	if err != nil {
		t.Fatalf("empty path Load() error: %v", err)
	}
	if items != nil {
		t.Fatalf("empty path Load() = %v, want nil", items)
	}
}
