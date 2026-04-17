package session

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"github.com/dreamSailing/eos/internal/ai"
)

type Session struct {
	Name         string       `json:"name"`
	Conversation []ai.Message `json:"conversation"`
	ContextPaths []string     `json:"context_paths"`
	Tasks        []Task       `json:"tasks"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type Task struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type Manager struct {
	dir string
}

func NewManager(dir string) *Manager {
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("session.new_manager.mkdir.error",
			"dir", dir,
			"error", err)
	}
	return &Manager{dir: dir}
}

func (m *Manager) path(name string) string {
	return filepath.Join(m.dir, name+".json")
}

func (m *Manager) Save(s *Session) error {
	if s == nil {
		slog.Error("session.save.nil_session")
		return fmt.Errorf("session is nil")
	}
	s.UpdatedAt = time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = s.UpdatedAt
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		slog.Error("session.save.marshal.error",
			"session_name", s.Name,
			"conversation_length", len(s.Conversation),
			"tasks_count", len(s.Tasks),
			"error", err)
		return err
	}
	sessionPath := m.path(s.Name)
	if err := os.WriteFile(sessionPath, b, 0644); err != nil {
		slog.Error("session.save.write_file.error",
			"session_name", s.Name,
			"session_path", sessionPath,
			"data_size", len(b),
			"error", err)
		return err
	}
	slog.Debug("session.save.success",
		"session_name", s.Name,
		"session_path", sessionPath,
		"conversation_length", len(s.Conversation),
		"tasks_count", len(s.Tasks))
	return nil
}

func (m *Manager) Load(name string) (*Session, error) {
	if name == "" {
		slog.Error("session.load.empty_name")
		return nil, fmt.Errorf("empty session name")
	}
	sessionPath := m.path(name)
	b, err := os.ReadFile(sessionPath)
	if err != nil {
		slog.Error("session.load.read_file.error",
			"session_name", name,
			"session_path", sessionPath,
			"error", err)
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		slog.Error("session.load.unmarshal.error",
			"session_name", name,
			"session_path", sessionPath,
			"data_size", len(b),
			"error", err)
		return nil, err
	}
	slog.Debug("session.load.success",
		"session_name", name,
		"conversation_length", len(s.Conversation),
		"tasks_count", len(s.Tasks))
	return &s, nil
}

func (m *Manager) Exists(name string) bool {
	if name == "" {
		slog.Error("session.exists.empty_name")
		return false
	}
	sessionPath := m.path(name)
	_, err := os.Stat(sessionPath)
	if err != nil {
		slog.Debug("session.exists.not_found",
			"session_name", name,
			"session_path", sessionPath,
			"error", err)
		return false
	}
	return true
}

func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		slog.Error("session.list.readdir.error",
			"dir", m.dir,
			"error", err)
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name()[:len(e.Name())-5])
		}
	}
	return names, nil
}

func (m *Manager) Create(name string) (*Session, error) {
	if name == "" {
		slog.Error("session.create.empty_name")
		return nil, fmt.Errorf("empty session name")
	}
	s := &Session{Name: name, Conversation: []ai.Message{}, ContextPaths: []string{}, Tasks: []Task{}}
	slog.Debug("session.create.success",
		"session_name", name)
	return s, m.Save(s)
}
