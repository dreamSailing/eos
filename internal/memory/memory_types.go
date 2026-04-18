package memory

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"time"
)

// MemoryType represents the category of a memory entry
type MemoryType string

const (
	// MemoryTypeUser holds user preferences and instructions
	MemoryTypeUser MemoryType = "user"
	// MemoryTypeFeedback holds feedback/corrections from interactions
	MemoryTypeFeedback MemoryType = "feedback"
	// MemoryTypeProject holds project-specific knowledge
	MemoryTypeProject MemoryType = "project"
	// MemoryTypeReference holds reference material
	MemoryTypeReference MemoryType = "reference"
)

// MemoryEntry represents a typed memory record
type MemoryEntry struct {
	ID        string    `json:"id"`
	Type      MemoryType `json:"type"`
	Content   string    `json:"content"`
	File      string    `json:"file"`
	Section   string    `json:"section,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
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
	switch e.Type {
	case MemoryTypeUser, MemoryTypeFeedback, MemoryTypeProject, MemoryTypeReference:
		return true
	default:
		return false
	}
}

// DefaultFile returns the default file path for a memory type
func (t MemoryType) DefaultFile() string {
	switch t {
	case MemoryTypeUser:
		return "EOS.md"
	case MemoryTypeProject:
		return ".eos/Rules.md"
	case MemoryTypeFeedback:
		return "EOS.md"
	case MemoryTypeReference:
		return "EOS.md"
	default:
		return "EOS.md"
	}
}

// ParseMemoryType parses a string into a MemoryType
func ParseMemoryType(s string) MemoryType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "user":
		return MemoryTypeUser
	case "feedback":
		return MemoryTypeFeedback
	case "project":
		return MemoryTypeProject
	case "reference":
		return MemoryTypeReference
	default:
		return MemoryTypeUser
	}
}

// AllMemoryTypes returns all valid memory types
func AllMemoryTypes() []MemoryType {
	return []MemoryType{MemoryTypeUser, MemoryTypeFeedback, MemoryTypeProject, MemoryTypeReference}
}
