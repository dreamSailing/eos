package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/schema"
)

type AgentRegistry struct {
	mu      sync.RWMutex
	entries map[string]agentRegistryEntry
	nextID  int
}

type agentRegistryEntry struct {
	manager *SubAgentManager
	resume  func(id string, task string) error
	close   func(id string) error
}

var (
	defaultAgentRegistryOnce sync.Once
	defaultAgentRegistry     *AgentRegistry
)

func DefaultAgentRegistry() *AgentRegistry {
	defaultAgentRegistryOnce.Do(func() {
		defaultAgentRegistry = &AgentRegistry{
			entries: map[string]agentRegistryEntry{},
		}
	})
	return defaultAgentRegistry
}

func (r *AgentRegistry) RegisterManager(mgr *SubAgentManager) string {
	return r.RegisterController(mgr, nil, nil)
}

func (r *AgentRegistry) RegisterController(mgr *SubAgentManager, resume func(id string, task string) error, close func(id string) error) string {
	if r == nil || mgr == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	id := fmt.Sprintf("agent_registry_%d", r.nextID)
	r.entries[id] = agentRegistryEntry{
		manager: mgr,
		resume:  resume,
		close:   close,
	}
	return id
}

func (r *AgentRegistry) UnregisterManager(id string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
}

func (r *AgentRegistry) ListSnapshots() []AgentSnapshot {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	entries := make([]agentRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	snaps := make([]AgentSnapshot, 0)
	for _, entry := range entries {
		snaps = append(snaps, entry.manager.ListSnapshots()...)
	}
	return snaps
}

func (r *AgentRegistry) Snapshot(id string) (AgentSnapshot, bool) {
	if r == nil {
		return AgentSnapshot{}, false
	}
	r.mu.RLock()
	entries := make([]agentRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	for _, entry := range entries {
		if ctx, ok := entry.manager.GetContext(id); ok {
			return snapshotFromContext(ctx), true
		}
	}
	return AgentSnapshot{}, false
}

func (r *AgentRegistry) SendInput(id string, input string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	entries := make([]agentRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	for _, entry := range entries {
		if _, ok := entry.manager.GetContext(id); !ok {
			continue
		}
		return entry.manager.AddMessage(id, schema.UserMessage(input)) == nil
	}
	return false
}

func (r *AgentRegistry) Wait(ctx context.Context, id string) (AgentSnapshot, bool, error) {
	if r == nil {
		return AgentSnapshot{}, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.RLock()
	entries := make([]agentRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	for _, entry := range entries {
		if _, ok := entry.manager.GetContext(id); !ok {
			continue
		}
		snap, err := entry.manager.Wait(ctx, id)
		return snap, true, err
	}
	return AgentSnapshot{}, false, nil
}

func (r *AgentRegistry) RequestCancel(id string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	entries := make([]agentRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	for _, entry := range entries {
		if _, ok := entry.manager.GetContext(id); ok {
			return entry.manager.RequestCancel(id) == nil
		}
	}
	return false
}

func (r *AgentRegistry) Remove(id string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	entries := make([]agentRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	for _, entry := range entries {
		if entry.manager.Remove(id) {
			return true
		}
	}
	return false
}

func (r *AgentRegistry) Resume(id string, task string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	entries := make([]agentRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	for _, entry := range entries {
		if _, ok := entry.manager.GetContext(id); !ok {
			continue
		}
		if entry.resume == nil {
			return false
		}
		return entry.resume(id, task) == nil
	}
	return false
}

func (r *AgentRegistry) Close(id string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	entries := make([]agentRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	for _, entry := range entries {
		if _, ok := entry.manager.GetContext(id); !ok {
			continue
		}
		if entry.close != nil {
			return entry.close(id) == nil
		}
		return entry.manager.Remove(id)
	}
	return false
}
