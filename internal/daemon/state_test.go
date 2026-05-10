package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := State{
		PID:        1234,
		StartedAt:  time.Now().UTC().Truncate(time.Second),
		ListenAddr: "127.0.0.1:8765",
		Workspace:  "/tmp/workspace",
		WebBaseURL: "http://127.0.0.1:8765",
	}
	if err := SaveState(path, want); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got.PID != want.PID || got.ListenAddr != want.ListenAddr || got.Workspace != want.Workspace || got.WebBaseURL != want.WebBaseURL {
		t.Fatalf("LoadState() = %+v, want %+v", got, want)
	}
	if err := RemoveState(path); err != nil {
		t.Fatalf("RemoveState() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected state file removed, err=%v", err)
	}
}

func TestDefaultLogFileUsesConfiguredGlobalLogDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	logDir := filepath.Join(home, "custom-logs")
	configPath := filepath.Join(home, ".eos.json")
	body, err := json.Marshal(map[string]string{"log_dir": logDir})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, body, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	want := filepath.Join(logDir, "daemon.log")
	if got := DefaultLogFile(); got != want {
		t.Fatalf("DefaultLogFile()=%q, want %q", got, want)
	}
}
