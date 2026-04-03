package tools

import (
	"sync"
	"time"
)

type TodoItem struct {
	ID        string
	Content   string
	Status    string
	Priority  any
	UpdatedAt time.Time
}

type TodoStore struct {
	mu    sync.RWMutex
	items []TodoItem
}

var (
	defaultTodoStoreOnce sync.Once
	defaultTodoStore     *TodoStore
)

func DefaultTodoStore() *TodoStore {
	defaultTodoStoreOnce.Do(func() {
		defaultTodoStore = &TodoStore{}
	})
	return defaultTodoStore
}

func (s *TodoStore) Replace(items []TodoItem) []TodoItem {
	if s == nil {
		return nil
	}

	now := time.Now()
	cp := make([]TodoItem, 0, len(items))
	for _, item := range items {
		item.UpdatedAt = now
		cp = append(cp, item)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = cp
	out := make([]TodoItem, len(s.items))
	copy(out, s.items)
	return out
}

func (s *TodoStore) List() []TodoItem {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TodoItem, len(s.items))
	copy(out, s.items)
	return out
}
