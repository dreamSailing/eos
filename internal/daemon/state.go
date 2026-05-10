package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
)

type State struct {
	PID              int       `json:"pid"`
	StartedAt        time.Time `json:"started_at"`
	ListenAddr       string    `json:"listen_addr"`
	Workspace        string    `json:"workspace"`
	SessionStorePath string    `json:"session_store_path,omitempty"`
	SchedulePath     string    `json:"schedule_path,omitempty"`
	MCPBasePath      string    `json:"mcp_base_path,omitempty"`
	MCPMessagePath   string    `json:"mcp_message_path,omitempty"`
	WebBaseURL       string    `json:"web_base_url,omitempty"`
	LogFile          string    `json:"log_file,omitempty"`
}

func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".eos", "daemon")
	}
	return filepath.Join(home, ".eos", "daemon")
}

func DefaultStateFile() string {
	return filepath.Join(DefaultDir(), "state.json")
}

func DefaultScheduleFile() string {
	return filepath.Join(DefaultDir(), "schedules.json")
}

func DefaultLogFile() string {
	return filepath.Join(config.ConfiguredLogDir(), "daemon.log")
}

func LoadState(path string) (State, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultStateFile()
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func SaveState(path string, state State) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultStateFile()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func RemoveState(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultStateFile()
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
