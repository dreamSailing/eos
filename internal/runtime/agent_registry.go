package runtime

import (
	"fmt"
	"sync"
)

type AgentRegistry struct {
	mu       sync.RWMutex
	managers map[string]*SubAgentManager
	nextID   int
}

var (
	defaultAgentRegistryOnce sync.Once
	defaultAgentRegistry     *AgentRegistry
)

func DefaultAgentRegistry() *AgentRegistry {
	defaultAgentRegistryOnce.Do(func() {
		defaultAgentRegistry = &AgentRegistry{
			managers: map[string]*SubAgentManager{},
		}
	})
	return defaultAgentRegistry
}

func (r *AgentRegistry) RegisterManager(mgr *SubAgentManager) string {
	if r == nil || mgr == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	id := fmt.Sprintf("agent_registry_%d", r.nextID)
	r.managers[id] = mgr
	return id
}

func (r *AgentRegistry) UnregisterManager(id string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.managers, id)
}

func (r *AgentRegistry) ListSnapshots() []AgentSnapshot {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	managers := make([]*SubAgentManager, 0, len(r.managers))
	for _, mgr := range r.managers {
		managers = append(managers, mgr)
	}
	r.mu.RUnlock()

	snaps := make([]AgentSnapshot, 0)
	for _, mgr := range managers {
		snaps = append(snaps, mgr.ListSnapshots()...)
	}
	return snaps
}

func (r *AgentRegistry) RequestCancel(id string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	managers := make([]*SubAgentManager, 0, len(r.managers))
	for _, mgr := range r.managers {
		managers = append(managers, mgr)
	}
	r.mu.RUnlock()

	for _, mgr := range managers {
		if _, ok := mgr.GetContext(id); ok {
			return mgr.RequestCancel(id) == nil
		}
	}
	return false
}

func (r *AgentRegistry) Remove(id string) bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	managers := make([]*SubAgentManager, 0, len(r.managers))
	for _, mgr := range r.managers {
		managers = append(managers, mgr)
	}
	r.mu.RUnlock()

	for _, mgr := range managers {
		if mgr.Remove(id) {
			return true
		}
	}
	return false
}
