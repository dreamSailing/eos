package memory

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryType represents the category of a memory entry
type MemoryType string

const (
	// MemoryTypeGlobal stores cross-project user preferences and profile.
	MemoryTypeGlobal MemoryType = "global"
	// MemoryTypeProject holds project-specific knowledge
	MemoryTypeProject MemoryType = "project"
)

// MemoryEntry represents a typed memory record
type MemoryEntry struct {
	ID          string     `json:"id"`
	Type        MemoryType `json:"type"`
	Content     string     `json:"content"`
	File        string     `json:"file"`
	Section     string     `json:"section,omitempty"`
	Source      string     `json:"source,omitempty"`
	Fingerprint string     `json:"fingerprint,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at,omitempty"`
}

// Validate checks if a MemoryEntry is valid
func (e *MemoryEntry) Validate() bool {
	if e == nil {
		return false
	}
	e.Content = strings.TrimSpace(e.Content)
	if e.Content == "" {
		return false
	}
	if e.ID == "" {
		e.ID = e.ensureFingerprint()
	}
	switch e.Type {
	case MemoryTypeGlobal, MemoryTypeProject:
		return true
	default:
		return false
	}
}

// DefaultFile returns the default file path for a memory type
func (t MemoryType) DefaultPath(rootDir string) string {
	switch t {
	case MemoryTypeProject:
		return ProjectMemoryPath(rootDir)
	case MemoryTypeGlobal:
		return GlobalMemoryPath()
	default:
		return ProjectMemoryPath(rootDir)
	}
}

// ParseMemoryType parses a string into a MemoryType
func ParseMemoryType(s string) MemoryType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "global", "user", "user_profile", "feedback", "reference":
		return MemoryTypeGlobal
	case "project":
		return MemoryTypeProject
	default:
		return MemoryTypeGlobal
	}
}

// AllMemoryTypes returns all valid memory types
func AllMemoryTypes() []MemoryType {
	return []MemoryType{MemoryTypeGlobal, MemoryTypeProject}
}

const (
	GlobalMemoryDocID  = "~/.eos/memory/user.md"
	ProjectMemoryDocID = ".eos/memory/project.md"
	ProjectIndexDocID  = ".eos/memory/MEMORY.md"
)

func GlobalMemoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".eos", "memory", "user.md")
	}
	return filepath.Join(home, ".eos", "memory", "user.md")
}

func ProjectMemoryPath(rootDir string) string {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return filepath.Join(".eos", "memory", "project.md")
	}
	return filepath.Join(rootDir, ".eos", "memory", "project.md")
}

func ProjectMemoryIndexPath(rootDir string) string {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return filepath.Join(".eos", "memory", IndexFile)
	}
	return filepath.Join(rootDir, ".eos", "memory", IndexFile)
}

func NormalizeSection(section string, memType MemoryType) string {
	section = strings.TrimSpace(section)
	if section != "" {
		return section
	}
	switch memType {
	case MemoryTypeProject:
		return "项目记忆"
	default:
		return "用户偏好"
	}
}

func NormalizeContent(content string) string {
	content = strings.TrimSpace(content)
	content = strings.Join(strings.Fields(content), " ")
	return strings.TrimSpace(content)
}

func (e *MemoryEntry) ensureFingerprint() string {
	if strings.TrimSpace(e.Fingerprint) != "" {
		return strings.TrimSpace(e.Fingerprint)
	}
	sum := sha1.Sum([]byte(string(e.Type) + "\n" + NormalizeSection(e.Section, e.Type) + "\n" + NormalizeContent(e.Content)))
	e.Fingerprint = hex.EncodeToString(sum[:8])
	return e.Fingerprint
}

func defaultTemplate(memType MemoryType) string {
	switch memType {
	case MemoryTypeProject:
		return "# 项目记忆\n\n## 项目约定\n\n## 任务结论\n\n## 排障经验\n"
	default:
		return "# 全局用户记忆\n\n## 用户偏好\n\n## 沟通风格\n\n## 工具与流程偏好\n"
	}
}
