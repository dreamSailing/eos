package store

import (
	"fmt"
	"os"
	"strings"
)

const (
	EnvStoreBackend = "EOS_STORE_BACKEND"

	BackendFile   = "file"
	BackendSQLite = "sqlite"
)

type FactoryOption struct {
	Name   string
	Root   string
	Backend string
}

func ResolveBackend(override string) string {
	b := strings.TrimSpace(override)
	if b != "" {
		return strings.ToLower(b)
	}
	return strings.ToLower(strings.TrimSpace(os.Getenv(EnvStoreBackend)))
}

func NewReadWriteStore(opt FactoryOption) (ReadWriteStore, error) {
	backend := ResolveBackend(opt.Backend)

	switch backend {
	case "", BackendFile:
		return NewFileStore(opt.Name, opt.Root), nil
	case BackendSQLite:
		s, err := NewSQLiteStore(opt.Name, opt.Root+".db")
		if err != nil {
			return nil, fmt.Errorf("store: sqlite backend init failed for %q: %w", opt.Name, err)
		}
		return s, nil
	default:
		return nil, fmt.Errorf("store: unknown backend %q (supported: file, sqlite)", backend)
	}
}
