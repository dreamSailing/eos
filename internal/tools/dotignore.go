package tools

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DotIgnore manages .vbignore pattern loading and caching
type DotIgnore struct {
	mu       sync.RWMutex
	patterns []string
	mtime    time.Time
	root     string
}

// NewDotIgnore creates a new DotIgnore instance
func NewDotIgnore(root string) *DotIgnore {
	return &DotIgnore{root: root}
}

// LoadIgnorePatterns loads and caches .vbignore patterns from the project root
func LoadIgnorePatterns(root string) []string {
	di := NewDotIgnore(root)
	return di.Load()
}

// Load reads .vbignore and returns patterns
func (di *DotIgnore) Load() []string {
	ignorePath := filepath.Join(di.root, ".vbignore")

	info, err := os.Stat(ignorePath)
	if err != nil {
		return nil
	}

	di.mu.RLock()
	if di.patterns != nil && !info.ModTime().After(di.mtime) {
		patterns := make([]string, len(di.patterns))
		copy(patterns, di.patterns)
		di.mu.RUnlock()
		return patterns
	}
	di.mu.RUnlock()

	f, err := os.Open(ignorePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	di.mu.Lock()
	di.patterns = patterns
	di.mtime = info.ModTime()
	di.mu.Unlock()

	return patterns
}

// Match checks if a path matches any .vbignore pattern
func (di *DotIgnore) Match(path string) bool {
	patterns := di.Load()
	if len(patterns) == 0 {
		return false
	}

	relPath := path
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(di.root, path)
		if err == nil {
			relPath = rel
		}
	}
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		// Negation pattern (like .gitignore)
		if strings.HasPrefix(pattern, "!") {
			negPattern := pattern[1:]
			if matchIgnorePattern(negPattern, relPath) {
				return false
			}
			continue
		}

		if matchIgnorePattern(pattern, relPath) {
			return true
		}
	}

	return false
}

func matchIgnorePattern(pattern, path string) bool {
	// Directory pattern (trailing /)
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Match using filepath.Match for glob support
	matched, _ := filepath.Match(pattern, filepath.Base(path))
	if matched {
		return true
	}

	// Try matching the full path
	matched, _ = filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Check if path is under a directory pattern
	if strings.Contains(pattern, "/") {
		matched, _ = filepath.Match(pattern, path)
		if matched {
			return true
		}
	}

	// Check prefix match for directory patterns
	if strings.HasPrefix(path, pattern) {
		return true
	}

	return false
}
