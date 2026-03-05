package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Manager 负责配置的加载与保存
type Manager struct {
	path string
}

func NewManager(path string) *Manager {
	return &Manager{path: path}
}

func (m *Manager) SetPath(path string) {
	m.path = path
}

// Load 加载配置
func (m *Manager) Load() (*Settings, error) {
	if m.path == "" {
		return nil, os.ErrNotExist
	}
	bs, err := os.ReadFile(m.path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(bs, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Save 保存配置
func (m *Manager) Save(s *Settings) error {
	if m.path == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return err
	}
	bs, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, bs, 0644)
}
