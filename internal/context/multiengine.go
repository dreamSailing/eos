package codectx

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"os"
	"sync"
)

type MultiEngine struct {
	mu      sync.RWMutex
	engines map[string]*Engine
	active  string
}

func NewMultiEngine() *MultiEngine {
	return &MultiEngine{engines: map[string]*Engine{}}
}

func (m *MultiEngine) AddRoot(path string) *Engine {
	if path == "" {
		wd, _ := os.Getwd()
		path = wd
	}
	m.mu.RLock()
	if e := m.engines[path]; e != nil {
		m.mu.RUnlock()
		return e
	}
	m.mu.RUnlock()

	e := NewEngine(path)
	_ = e.BuildIndex()

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing := m.engines[path]; existing != nil {
		return existing
	}
	m.engines[path] = e
	if m.active == "" {
		m.active = path
	}
	return e
}

func (m *MultiEngine) RemoveRoot(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.engines, path)
	if m.active == path {
		m.active = ""
		// pick any remaining as active
		for p := range m.engines {
			m.active = p
			break
		}
	}
}

func (m *MultiEngine) SetActive(path string) *Engine {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.engines[path]; e != nil {
		m.active = path
		return e
	}
	return nil
}

func (m *MultiEngine) Active() *Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.active == "" {
		return nil
	}
	return m.engines[m.active]
}

func (m *MultiEngine) Roots() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for p := range m.engines {
		out = append(out, p)
	}
	return out
}

// Start background indexing/watch for all engines
func (m *MultiEngine) StartBackground(ctx context.Context) {
	for _, e := range m.snapshotEngines() {
		if err := e.StartWatch(ctx); err != nil {
			e.StartPoll(ctx, 30*1000000000)
		}
	}
}

func (m *MultiEngine) snapshotEngines() []*Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Engine, 0, len(m.engines))
	for _, e := range m.engines {
		out = append(out, e)
	}
	return out
}
