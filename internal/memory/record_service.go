package memory

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/store"
)

var (
	ErrDuplicateMemory = errors.New("memory record already exists")
	ErrNotFound        = errors.New("memory record not found")
)

// Service provides CRUD operations for MemoryRecord backed by a ReadWriteStore.
// All methods are safe for concurrent use.
type Service struct {
	fs store.ReadWriteStore
	mu sync.Mutex
}

// NewService creates a Service whose records live under fs.
func NewService(fs store.ReadWriteStore) *Service {
	return &Service{fs: fs}
}

// Add persists a new MemoryRecord. If a record with the same scope, kind and
// content already exists the existing record is returned without duplication.
func (s *Service) Add(ctx context.Context, rec *MemoryRecord) (*MemoryRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("record must not be nil")
	}
	rec.Content = strings.TrimSpace(rec.Content)
	if rec.Content == "" {
		return nil, fmt.Errorf("content must not be empty")
	}
	rec.Scope = strings.TrimSpace(rec.Scope)
	if rec.Scope == "" {
		return nil, fmt.Errorf("scope must not be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hash := ContentHash(rec.Scope, rec.Kind, rec.Content)

	existing, err := s.loadAll()
	if err != nil {
		return nil, fmt.Errorf("load records: %w", err)
	}
	for _, e := range existing {
		if ContentHash(e.Scope, e.Kind, e.Content) == hash {
			return e, nil
		}
	}

	now := time.Now()
	if rec.ID == "" {
		rec.ID = newRecordID()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now

	if err := s.fs.WriteJSONAtomic(rec.ID+".json", rec); err != nil {
		return nil, fmt.Errorf("write record: %w", err)
	}
	return rec, nil
}

// List returns all records matching the filter. Zero-value fields in f are
// ignored.
func (s *Service) List(ctx context.Context, f Filter) ([]*MemoryRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	return filterRecords(all, f), nil
}

// Search returns records whose content contains every keyword (case-insensitive)
// and that match the optional tag/scope/kind filters.
func (s *Service) Search(ctx context.Context, q SearchQuery) ([]*MemoryRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	return searchRecords(all, q), nil
}

// Delete removes the record with the given id.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	path := strings.TrimSpace(id) + ".json"
	if !s.fs.Exists(path) {
		return ErrNotFound
	}
	return s.fs.Remove(path)
}

func (s *Service) loadAll() ([]*MemoryRecord, error) {
	files, err := s.fs.ListFiles(".json")
	if err != nil {
		return nil, err
	}
	var records []*MemoryRecord
	for _, f := range files {
		var rec MemoryRecord
		if err := s.fs.ReadJSON(f, &rec); err != nil {
			continue
		}
		records = append(records, &rec)
	}
	return records, nil
}

func newRecordID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func filterRecords(records []*MemoryRecord, f Filter) []*MemoryRecord {
	var out []*MemoryRecord
	for _, r := range records {
		if f.Scope != "" && r.Scope != f.Scope {
			continue
		}
		if f.Kind != "" && r.Kind != f.Kind {
			continue
		}
		if len(f.Tags) > 0 && !hasAllTags(r.Tags, f.Tags) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func searchRecords(records []*MemoryRecord, q SearchQuery) []*MemoryRecord {
	var out []*MemoryRecord
	for _, r := range records {
		if q.Scope != "" && r.Scope != q.Scope {
			continue
		}
		if q.Kind != "" && r.Kind != q.Kind {
			continue
		}
		if len(q.Tags) > 0 && !hasAllTags(r.Tags, q.Tags) {
			continue
		}
		if len(q.Keywords) > 0 && !containsAllKeywords(r.Content, q.Keywords) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func hasAllTags(recordTags, required []string) bool {
	set := make(map[string]struct{}, len(recordTags))
	for _, t := range recordTags {
		set[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}
	for _, t := range required {
		if _, ok := set[strings.ToLower(strings.TrimSpace(t))]; !ok {
			return false
		}
	}
	return true
}

func containsAllKeywords(content string, keywords []string) bool {
	lower := strings.ToLower(content)
	for _, kw := range keywords {
		if !strings.Contains(lower, strings.ToLower(strings.TrimSpace(kw))) {
			return false
		}
	}
	return true
}
