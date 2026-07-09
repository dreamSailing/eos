package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu      sync.Mutex
	rootDir string
	index   *MemoryIndex
}

type WriteResult struct {
	Entry     MemoryEntry
	Path      string
	Section   string
	Created   bool
	Deduped   bool
	IndexPath string
}

type Snapshot struct {
	GlobalPath     string
	GlobalContent  string
	GlobalExists   bool
	ProjectPath    string
	ProjectContent string
	ProjectExists  bool
	SessionPath    string
	SessionContent string
	SessionExists  bool
	IndexPath      string
	IndexContent   string
	IndexExists    bool
}

func NewStore(rootDir string) *Store {
	rootDir = strings.TrimSpace(rootDir)
	return &Store{
		rootDir: rootDir,
		index:   NewMemoryIndex(rootDir),
	}
}

func (s *Store) RootDir() string {
	return s.rootDir
}

func (s *Store) Upsert(entry MemoryEntry) (WriteResult, error) {
	entry.Type = ParseMemoryType(string(entry.Type))
	entry.Content = NormalizeContent(entry.Content)
	entry.Section = NormalizeSection(entry.Section, entry.Type)
	entry.Source = strings.TrimSpace(entry.Source)
	if !entry.Validate() {
		return WriteResult{}, fmt.Errorf("invalid memory entry")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.UpdatedAt = time.Now()
	entry.ensureFingerprint()

	path := entry.File
	if strings.TrimSpace(path) == "" {
		path = entry.Type.DefaultPath(s.rootDir)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ensureFileWithTemplate(path, entry.Type); err != nil {
		return WriteResult{}, err
	}

	existing, _ := os.ReadFile(path)
	existingText := string(existing)
	if containsNormalizedEntry(existingText, entry.Content) {
		_ = s.rebuildIndex()
		return WriteResult{
			Entry:     entry,
			Path:      path,
			Section:   entry.Section,
			Deduped:   true,
			IndexPath: ProjectMemoryIndexPath(s.rootDir),
		}, nil
	}

	updated := upsertSectionContent(existingText, entry.Section, entry.Content)
	if err := writeFileAtomic(path, []byte(updated), 0o644); err != nil {
		return WriteResult{}, err
	}
	if err := s.rebuildIndex(); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{
		Entry:     entry,
		Path:      path,
		Section:   entry.Section,
		Created:   true,
		IndexPath: ProjectMemoryIndexPath(s.rootDir),
	}, nil
}

func (s *Store) RebuildIndex() error {
	return s.rebuildIndex()
}

func (s *Store) rebuildIndex() error {
	if strings.TrimSpace(s.rootDir) == "" || s.index == nil {
		return nil
	}
	return s.index.RebuildIndexFromDisk()
}

func LoadSnapshot(rootDir string) Snapshot {
	snap := Snapshot{
		GlobalPath:  GlobalMemoryPath(),
		ProjectPath: ProjectMemoryPath(rootDir),
		SessionPath: filepath.Join(rootDir, ".eos", "session-memory", "session.md"),
		IndexPath:   ProjectMemoryIndexPath(rootDir),
	}
	snap.GlobalContent, snap.GlobalExists = readSnapshotFile(snap.GlobalPath)
	snap.ProjectContent, snap.ProjectExists = readSnapshotFile(snap.ProjectPath)
	snap.SessionContent, snap.SessionExists = readSnapshotFile(snap.SessionPath)
	snap.IndexContent, snap.IndexExists = readSnapshotFile(snap.IndexPath)
	return snap
}

func readSnapshotFile(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func EnsureWorkspaceMemory(rootDir string) error {
	store := NewStore(rootDir)
	if err := ensureFileWithTemplate(ProjectMemoryPath(rootDir), MemoryTypeProject); err != nil {
		return err
	}
	return store.rebuildIndex()
}

func ensureFileWithTemplate(path string, memType MemoryType) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("memory path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeFileAtomic(path, []byte(defaultTemplate(memType)), 0o644)
}

func containsNormalizedEntry(content string, candidate string) bool {
	candidate = NormalizeContent(candidate)
	if candidate == "" {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimPrefix(line, "- ")
		}
		if NormalizeContent(line) == candidate {
			return true
		}
	}
	return false
}

func upsertSectionContent(content string, section string, entry string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	sectionHeader := "## " + strings.TrimSpace(section)
	insertAt := -1
	sectionFound := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == sectionHeader {
			sectionFound = true
			insertAt = i + 1
			for insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) == "" {
				insertAt++
			}
			break
		}
	}
	if !sectionFound {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, sectionHeader, "", "- "+entry, "")
		return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	}

	lines = append(lines[:insertAt], append([]string{"- " + entry}, lines[insertAt:]...)...)
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
