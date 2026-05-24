package memory

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// MemoryRecord represents a structured long-term memory entry stored via the
// Store abstraction. Unlike the Markdown-based MemoryEntry, a MemoryRecord is
// an atomic JSON document keyed by ID.
type MemoryRecord struct {
	ID            string    `json:"id"`
	Scope         string    `json:"scope"`
	WorkspaceRoot string    `json:"workspace_root"`
	SessionID     string    `json:"session_id"`
	Kind          string    `json:"kind"`
	Content       string    `json:"content"`
	Tags          []string  `json:"tags,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Source        string    `json:"source"`
}

// Filter constrains List results by optional fields.
type Filter struct {
	Scope string
	Kind  string
	Tags  []string
}

// SearchQuery constrains Search results. Keywords are matched against Content
// (case-insensitive substring). Tags, Scope and Kind are exact filters.
type SearchQuery struct {
	Keywords []string
	Tags     []string
	Scope    string
	Kind     string
}

// ContentHash computes a dedup fingerprint from scope, kind, and content.
func ContentHash(scope, kind, content string) string {
	h := sha256.New()
	h.Write([]byte(scope))
	h.Write([]byte{0})
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
