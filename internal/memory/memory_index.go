package memory

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// IndexMaxLines is the maximum number of lines for the MEMORY.md index
	IndexMaxLines = 200
	// IndexFile is the name of the memory index file
	IndexFile = "MEMORY.md"
)

// MemoryIndex manages the MEMORY.md index file that summarizes all memory entries
type MemoryIndex struct {
	mu      sync.RWMutex
	rootDir string
}

// NewMemoryIndex creates a new memory index manager
func NewMemoryIndex(rootDir string) *MemoryIndex {
	return &MemoryIndex{rootDir: rootDir}
}

// indexPath returns the full path to the MEMORY.md index file
func (idx *MemoryIndex) indexPath() string {
	return filepath.Join(idx.rootDir, IndexFile)
}

// AddEntry adds an entry to the MEMORY.md index
func (idx *MemoryIndex) AddEntry(entry MemoryEntry) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	path := idx.indexPath()

	// Read existing content
	var lines []string
	existing, err := os.ReadFile(path)
	if err == nil {
		lines = strings.Split(string(existing), "\n")
	}

	// Build the new entry line
	entryLine := formatIndexEntry(entry)

	// Find the section for this type
	sectionHeader := "## " + string(entry.Type)
	sectionIdx := -1

	for i, l := range lines {
		if strings.TrimSpace(l) == sectionHeader {
			sectionIdx = i
			break
		}
	}

	// Insert the entry
	if sectionIdx >= 0 {
		// Insert after section header
		lines = append(lines[:sectionIdx+1], append([]string{entryLine}, lines[sectionIdx+1:]...)...)
	} else {
		// Add new section
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, sectionHeader, "", entryLine, "")
	}

	// Enforce line limit
	if len(lines) > IndexMaxLines {
		lines = truncateToLimit(lines, IndexMaxLines)
	}

	// Write back
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// RemoveEntry removes an entry from the index by ID
func (idx *MemoryIndex) RemoveEntry(id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	path := idx.indexPath()
	existing, err := os.ReadFile(path)
	if err != nil {
		return nil // no file, nothing to remove
	}

	lines := strings.Split(string(existing), "\n")
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		if !strings.Contains(l, "["+id+"]") {
			filtered = append(filtered, l)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(filtered, "\n")), 0644)
}

// ReadIndex reads the current MEMORY.md content
func (idx *MemoryIndex) ReadIndex() (string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	data, err := os.ReadFile(idx.indexPath())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RebuildIndex regenerates the entire index from memory files
func (idx *MemoryIndex) RebuildIndex(entries []MemoryEntry) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# Memory Index\n\n")

	// Group by type
	groups := make(map[MemoryType][]MemoryEntry)
	for _, e := range entries {
		groups[e.Type] = append(groups[e.Type], e)
	}

	for _, t := range AllMemoryTypes() {
		entries := groups[t]
		if len(entries) == 0 {
			continue
		}
		sb.WriteString("## " + string(t) + "\n\n")
		for _, e := range entries {
			sb.WriteString(formatIndexEntry(e))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	content := sb.String()
	lines := strings.Split(content, "\n")
	if len(lines) > IndexMaxLines {
		lines = truncateToLimit(lines, IndexMaxLines)
		content = strings.Join(lines, "\n")
	}

	return os.WriteFile(idx.indexPath(), []byte(content), 0644)
}

// formatIndexEntry formats a memory entry for the index
func formatIndexEntry(e MemoryEntry) string {
	preview := e.Content
	if len(preview) > 100 {
		preview = preview[:97] + "..."
	}
	return fmt.Sprintf("- [%s] %s (in %s)", e.ID, preview, e.File)
}

// truncateToLimit truncates lines to fit within the limit, keeping the header
func truncateToLimit(lines []string, limit int) []string {
	if len(lines) <= limit {
		return lines
	}

	// Keep first 10 lines (header area) and last (limit-12) lines
	head := 10
	tail := limit - head - 2 // 2 lines for truncation notice

	result := make([]string, 0, limit)
	result = append(result, lines[:head]...)
	result = append(result, fmt.Sprintf("...[%d lines truncated to fit %d line limit]...", len(lines)-limit, limit))
	result = append(result, lines[len(lines)-tail:]...)

	return result
}

// Ensure directory exists
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// ScanMemoryFiles scans the workspace for memory files and returns their entries
func ScanMemoryFiles(rootDir string) ([]MemoryEntry, error) {
	var entries []MemoryEntry

	type fileInfo struct {
		file    string
		memType MemoryType
	}

	files := []fileInfo{
		{"EOS.md", MemoryTypeUser},
		{".eos/Rules.md", MemoryTypeProject},
	}

	for _, fi := range files {
		path := filepath.Join(rootDir, fi.file)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			line := scanner.Text()
			lineNum++
			if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			entries = append(entries, MemoryEntry{
				ID:        fmt.Sprintf("%s-L%d", fi.file, lineNum),
				Type:      fi.memType,
				Content:   strings.TrimSpace(line),
				File:      fi.file,
				CreatedAt: timeNow(),
			})
		}
		f.Close()
	}

	return entries, nil
}

// timeNow returns the current time (extracted for testability)
var timeNow = func() time.Time {
	return time.Now()
}
