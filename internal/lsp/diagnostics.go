//go:build !without_lsp

package lsp

import (
	"sync"
	"time"
)

// DiagnosticStore 诊断信息存储
type DiagnosticStore struct {
	mu    sync.RWMutex
	items map[string]*FileDiagnostics // key: file URI
}

// FileDiagnostics 文件诊断信息
type FileDiagnostics struct {
	URI         DocumentURI
	Version     int
	Diagnostics []Diagnostic
	UpdatedAt   time.Time
}

// NewDiagnosticStore 创建存储
func NewDiagnosticStore() *DiagnosticStore {
	return &DiagnosticStore{
		items: make(map[string]*FileDiagnostics),
	}
}

// Set 设置诊断信息
func (s *DiagnosticStore) Set(uri DocumentURI, version int, diagnostics []Diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[string(uri)] = &FileDiagnostics{
		URI:         uri,
		Version:     version,
		Diagnostics: diagnostics,
		UpdatedAt:   time.Now(),
	}
}

// Get 获取诊断信息
func (s *DiagnosticStore) Get(uri DocumentURI) (*FileDiagnostics, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[string(uri)]
	return item, ok
}

// GetAll 获取所有诊断信息
func (s *DiagnosticStore) GetAll() map[string]*FileDiagnostics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*FileDiagnostics, len(s.items))
	for k, v := range s.items {
		result[k] = v
	}
	return result
}

// Clear 清除诊断信息
func (s *DiagnosticStore) Clear(uri DocumentURI) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, string(uri))
}

// ClearAll 清除所有
func (s *DiagnosticStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = make(map[string]*FileDiagnostics)
}

// GetErrors 获取错误数量
func (s *DiagnosticStore) GetErrors(uri DocumentURI) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if item, ok := s.items[string(uri)]; ok {
		count := 0
		for _, d := range item.Diagnostics {
			if d.Severity == SeverityError {
				count++
			}
		}
		return count
	}
	return 0
}

// GetWarnings 获取警告数量
func (s *DiagnosticStore) GetWarnings(uri DocumentURI) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if item, ok := s.items[string(uri)]; ok {
		count := 0
		for _, d := range item.Diagnostics {
			if d.Severity == SeverityWarning {
				count++
			}
		}
		return count
	}
	return 0
}
