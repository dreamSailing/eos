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
	"sort"
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
	homeDir string
}

// NewMemoryIndex creates a new memory index manager
func NewMemoryIndex(rootDir string) *MemoryIndex {
	homeDir, _ := os.UserHomeDir()
	return &MemoryIndex{rootDir: rootDir, homeDir: homeDir}
}

// indexPath returns the full path to the MEMORY.md index file
func (idx *MemoryIndex) indexPath() string {
	return ProjectMemoryIndexPath(idx.rootDir)
}

// AddEntry refreshes the workspace index after a memory update.
func (idx *MemoryIndex) AddEntry(entry MemoryEntry) error {
	return idx.RebuildIndexFromDisk()
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

	if err := ensureDir(idx.indexPath()); err != nil {
		return err
	}
	return os.WriteFile(idx.indexPath(), []byte(content), 0644)
}

// RebuildIndexFromDisk regenerates the workspace index from the global and project memory files.
func (idx *MemoryIndex) RebuildIndexFromDisk() error {
	entries, err := ScanMemoryFiles(idx.rootDir)
	if err != nil {
		return err
	}
	return idx.RebuildIndex(entries)
}

// formatIndexEntry formats a memory entry for the index
func formatIndexEntry(e MemoryEntry) string {
	preview := e.Content
	if len(preview) > 100 {
		preview = preview[:97] + "..."
	}
	section := strings.TrimSpace(e.Section)
	if section != "" {
		return fmt.Sprintf("- [%s] %s (%s, %s)", e.ID, preview, e.File, section)
	}
	return fmt.Sprintf("- [%s] %s (%s)", e.ID, preview, e.File)
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

// ScanMemoryFiles scans the workspace and home memory files and returns their entries.
func ScanMemoryFiles(rootDir string) ([]MemoryEntry, error) {
	var entries []MemoryEntry

	type fileInfo struct {
		file    string
		label   string
		memType MemoryType
	}

	homeDir, _ := os.UserHomeDir()
	files := make([]fileInfo, 0, 2)
	if strings.TrimSpace(homeDir) != "" {
		files = append(files, fileInfo{
			file:    GlobalMemoryPath(),
			label:   filepath.ToSlash(GlobalMemoryDocID),
			memType: MemoryTypeGlobal,
		})
	}
	files = append(files, fileInfo{
		file:    ProjectMemoryPath(rootDir),
		label:   filepath.ToSlash(ProjectMemoryDocID),
		memType: MemoryTypeProject,
	})

	now := timeNow()
	for _, fi := range files {
		if strings.TrimSpace(fi.file) == "" {
			continue
		}
		fileEntries, err := scanSingleMemoryFile(fi.file, fi.label, fi.memType, now)
		if err != nil {
			continue
		}
		entries = append(entries, fileEntries...)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type == entries[j].Type {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Type < entries[j].Type
	})
	return entries, nil
}

func scanSingleMemoryFile(path string, label string, memType MemoryType, now time.Time) ([]MemoryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	entries := make([]MemoryEntry, 0)
	lineNum := 0
	currentSection := ""
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "# "):
			continue
		case strings.HasPrefix(trimmed, "## "):
			currentSection = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			continue
		case strings.HasPrefix(trimmed, "<!--"):
			continue
		}

		trimmed = strings.TrimPrefix(trimmed, "- ")
		entry := MemoryEntry{
			Type:      memType,
			Content:   NormalizeContent(trimmed),
			File:      label,
			Section:   NormalizeSection(currentSection, memType),
			CreatedAt: now,
			UpdatedAt: now,
		}
		entry.ID = fmt.Sprintf("%s-L%d", entry.ensureFingerprint(), lineNum)
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// timeNow returns the current time (extracted for testability)
var timeNow = func() time.Time {
	return time.Now()
}
