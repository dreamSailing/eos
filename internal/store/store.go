package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Store interface {
	Name() string
	Root() string
}

type ReadWriteStore interface {
	Store
	ReadFile(relPath string) ([]byte, error)
	WriteFile(relPath string, data []byte) error
	WriteFileAtomic(relPath string, data []byte) error
	Remove(relPath string) error
	Exists(relPath string) bool
	ListFiles(suffix string) ([]string, error)
	ReadJSON(relPath string, dest any) error
	WriteJSON(relPath string, src any) error
	WriteJSONAtomic(relPath string, src any) error
}

type FileStore struct {
	name string
	root string
}

func NewFileStore(name, root string) *FileStore {
	return &FileStore{
		name: strings.TrimSpace(name),
		root: strings.TrimSpace(root),
	}
}

func (fs *FileStore) Name() string { return fs.name }
func (fs *FileStore) Root() string { return fs.root }

func (fs *FileStore) ReadFile(relPath string) ([]byte, error) {
	return os.ReadFile(fs.AbsPath(relPath))
}

func (fs *FileStore) WriteFile(relPath string, data []byte) error {
	abs := fs.AbsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

func (fs *FileStore) WriteFileAtomic(relPath string, data []byte) error {
	abs := fs.AbsPath(relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	tmp := abs + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, abs)
}

func (fs *FileStore) Remove(relPath string) error {
	return os.Remove(fs.AbsPath(relPath))
}

func (fs *FileStore) Exists(relPath string) bool {
	_, err := os.Stat(fs.AbsPath(relPath))
	return err == nil
}

func (fs *FileStore) MkdirAll(relPath string) error {
	return os.MkdirAll(fs.AbsPath(relPath), 0o755)
}

func (fs *FileStore) AbsPath(relPath string) string {
	return filepath.Join(fs.root, filepath.FromSlash(relPath))
}

func (fs *FileStore) ListFiles(suffix string) ([]string, error) {
	entries, err := os.ReadDir(fs.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if suffix == "" || strings.HasSuffix(e.Name(), suffix) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func (fs *FileStore) ReadJSON(relPath string, dest any) error {
	data, err := fs.ReadFile(relPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (fs *FileStore) WriteJSON(relPath string, src any) error {
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	return fs.WriteFile(relPath, data)
}

func (fs *FileStore) WriteJSONAtomic(relPath string, src any) error {
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	return fs.WriteFileAtomic(relPath, data)
}
