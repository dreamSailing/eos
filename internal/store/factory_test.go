package store

import (
	"path/filepath"
	"testing"
)

func TestResolveBackend_Override(t *testing.T) {
	if got := ResolveBackend("sqlite"); got != "sqlite" {
		t.Fatalf("ResolveBackend(\"sqlite\") = %q, want \"sqlite\"", got)
	}
	if got := ResolveBackend("SQLITE"); got != "sqlite" {
		t.Fatalf("ResolveBackend(\"SQLITE\") = %q, want \"sqlite\"", got)
	}
	if got := ResolveBackend(" file "); got != "file" {
		t.Fatalf("ResolveBackend(\" file \") = %q, want \"file\"", got)
	}
	if got := ResolveBackend(""); got != "" {
		t.Fatalf("ResolveBackend(\"\") = %q, want \"\"", got)
	}
}

func TestResolveBackend_EnvVar(t *testing.T) {
	t.Setenv(EnvStoreBackend, "sqlite")
	if got := ResolveBackend(""); got != "sqlite" {
		t.Fatalf("ResolveBackend with env = %q, want \"sqlite\"", got)
	}
}

func TestResolveBackend_OverrideTakesPrecedence(t *testing.T) {
	t.Setenv(EnvStoreBackend, "sqlite")
	if got := ResolveBackend("file"); got != "file" {
		t.Fatalf("ResolveBackend override = %q, want \"file\"", got)
	}
}

func TestNewReadWriteStore_DefaultFile(t *testing.T) {
	dir := t.TempDir()
	s, err := NewReadWriteStore(FactoryOption{
		Name: "test",
		Root: dir,
	})
	if err != nil {
		t.Fatalf("NewReadWriteStore default: %v", err)
	}
	if _, ok := s.(*FileStore); !ok {
		t.Fatalf("expected *FileStore, got %T", s)
	}
}

func TestNewReadWriteStore_ExplicitFile(t *testing.T) {
	dir := t.TempDir()
	s, err := NewReadWriteStore(FactoryOption{
		Name:    "test",
		Root:    dir,
		Backend: "file",
	})
	if err != nil {
		t.Fatalf("NewReadWriteStore file: %v", err)
	}
	if _, ok := s.(*FileStore); !ok {
		t.Fatalf("expected *FileStore, got %T", s)
	}
}

func TestNewReadWriteStore_SQLite(t *testing.T) {
	dir := t.TempDir()
	s, err := NewReadWriteStore(FactoryOption{
		Name:    "test",
		Root:    filepath.Join(dir, "data"),
		Backend: "sqlite",
	})
	if err != nil {
		t.Fatalf("NewReadWriteStore sqlite: %v", err)
	}
	defer s.(*SQLiteStore).Close()

	sqlite, ok := s.(*SQLiteStore)
	if !ok {
		t.Fatalf("expected *SQLiteStore, got %T", s)
	}

	if err := sqlite.WriteFile("key.txt", []byte("value")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := sqlite.ReadFile("key.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("content = %q, want \"value\"", string(got))
	}
}

func TestNewReadWriteStore_SQLiteViaEnv(t *testing.T) {
	t.Setenv(EnvStoreBackend, "sqlite")
	dir := t.TempDir()
	s, err := NewReadWriteStore(FactoryOption{
		Name: "test",
		Root: filepath.Join(dir, "data"),
	})
	if err != nil {
		t.Fatalf("NewReadWriteStore via env: %v", err)
	}
	defer s.(*SQLiteStore).Close()

	if _, ok := s.(*SQLiteStore); !ok {
		t.Fatalf("expected *SQLiteStore, got %T", s)
	}
}

func TestNewReadWriteStore_UnknownBackend(t *testing.T) {
	dir := t.TempDir()
	_, err := NewReadWriteStore(FactoryOption{
		Name:    "test",
		Root:    dir,
		Backend: "redis",
	})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestNewReadWriteStore_SQLiteInitError(t *testing.T) {
	_, err := NewReadWriteStore(FactoryOption{
		Name:    "",
		Root:    filepath.Join(t.TempDir(), "data"),
		Backend: "sqlite",
	})
	if err == nil {
		t.Fatal("expected error for sqlite init failure with empty name")
	}
}

func TestNewReadWriteStore_ImplementsInterface(t *testing.T) {
	dir := t.TempDir()
	s, err := NewReadWriteStore(FactoryOption{
		Name: "test",
		Root: dir,
	})
	if err != nil {
		t.Fatalf("NewReadWriteStore: %v", err)
	}
	var _ ReadWriteStore = s
}
