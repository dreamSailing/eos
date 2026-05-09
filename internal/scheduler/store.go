package scheduler

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) Load() ([]Schedule, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil, nil
	}
	body, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data StoreData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data.Schedules, nil
}

func (s *Store) Save(items []Schedule) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(StoreData{Schedules: items}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
