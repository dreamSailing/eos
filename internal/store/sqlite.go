package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

type SQLiteStore struct {
	name   string
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

func NewSQLiteStore(name, dbPath string) (*SQLiteStore, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("store: name must not be empty")
	}
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("store: dbPath must not be empty")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("store: create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite: %w", err)
	}

	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA busy_timeout=5000;
		CREATE TABLE IF NOT EXISTS store (
			path       TEXT PRIMARY KEY,
			data       BLOB NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: init schema: %w", err)
	}

	return &SQLiteStore{name: name, db: db, dbPath: dbPath}, nil
}

func (s *SQLiteStore) Name() string { return s.name }
func (s *SQLiteStore) Root() string { return s.dbPath }

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) ReadFile(relPath string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	err := s.db.QueryRow("SELECT data FROM store WHERE path = ?", relPath).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("store: file not found: %s", relPath)
	}
	if err != nil {
		return nil, fmt.Errorf("store: read %s: %w", relPath, err)
	}
	return data, nil
}

func (s *SQLiteStore) WriteFile(relPath string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	_, err := s.db.Exec(`
		INSERT INTO store (path, data, created_at, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at
	`, relPath, data, now, now)
	if err != nil {
		return fmt.Errorf("store: write %s: %w", relPath, err)
	}
	return nil
}

func (s *SQLiteStore) WriteFileAtomic(relPath string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin tx for %s: %w", relPath, err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	_, err = tx.Exec(`
		INSERT INTO store (path, data, created_at, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at
	`, relPath, data, now, now)
	if err != nil {
		return fmt.Errorf("store: write %s: %w", relPath, err)
	}
	return tx.Commit()
}

func (s *SQLiteStore) Remove(relPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec("DELETE FROM store WHERE path = ?", relPath)
	if err != nil {
		return fmt.Errorf("store: remove %s: %w", relPath, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("store: file not found: %s", relPath)
	}
	return nil
}

func (s *SQLiteStore) Exists(relPath string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM store WHERE path = ?", relPath).Scan(&count)
	return count > 0
}

func (s *SQLiteStore) ListFiles(suffix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT path FROM store")
	if err != nil {
		return nil, fmt.Errorf("store: list files: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("store: scan path: %w", err)
		}
		base := filepath.Base(p)
		if suffix == "" || strings.HasSuffix(base, suffix) {
			out = append(out, base)
		}
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ReadJSON(relPath string, dest any) error {
	data, err := s.ReadFile(relPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (s *SQLiteStore) WriteJSON(relPath string, src any) error {
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	return s.WriteFile(relPath, data)
}

func (s *SQLiteStore) WriteJSONAtomic(relPath string, src any) error {
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	return s.WriteFileAtomic(relPath, data)
}
