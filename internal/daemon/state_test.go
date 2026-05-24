package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := State{
		PID:        12345,
		ListenAddr: "127.0.0.1:9999",
		Workspace:  "/tmp/workspace",
	}
	if err := SaveState(path, original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded.PID != original.PID {
		t.Fatalf("PID mismatch: %d vs %d", loaded.PID, original.PID)
	}
	if loaded.ListenAddr != original.ListenAddr {
		t.Fatalf("ListenAddr mismatch: %q vs %q", loaded.ListenAddr, original.ListenAddr)
	}
	if loaded.Workspace != original.Workspace {
		t.Fatalf("Workspace mismatch: %q vs %q", loaded.Workspace, original.Workspace)
	}
}

func TestSaveState_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "state.json")

	if err := SaveState(path, State{PID: 1}); err != nil {
		t.Fatalf("SaveState should create dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file should exist: %v", err)
	}
}

func TestSaveState_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := SaveState(path, State{PID: 100}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("tmp file should not exist after atomic save")
	}
}

func TestLoadState_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	_, err := LoadState(path)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRemoveState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := SaveState(path, State{PID: 1}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	if err := RemoveState(path); err != nil {
		t.Fatalf("RemoveState: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should be removed")
	}
}

func TestRemoveState_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	if err := RemoveState(path); err != nil {
		t.Fatalf("RemoveState on missing file should not error: %v", err)
	}
}

func TestSaveLoadState_AllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := State{
		PID:              42,
		ListenAddr:       "0.0.0.0:8080",
		Workspace:        "/home/user/project",
		SessionStorePath: "/home/user/.eos/sessions.json",
		SchedulePath:     "/home/user/.eos/schedules.json",
		MCPBasePath:      "/home/user/.eos/mcp",
		MCPMessagePath:   "/home/user/.eos/mcp/messages",
		WebBaseURL:       "http://localhost:3000",
		LogFile:          "/home/user/.eos/logs/daemon.log",
	}
	if err := SaveState(path, original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded != original {
		t.Fatalf("roundtrip mismatch:\n  got:  %+v\n  want: %+v", loaded, original)
	}
}
