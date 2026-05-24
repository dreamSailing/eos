package agentcore

import (
	"fmt"
	"strings"
	"time"
)

func (r *Registry) RegisterRootWithID(id string, roleID string, task string) (Agent, error) {
	return r.createWithID("", id, roleID, task)
}

func (r *Registry) SpawnWithID(parentID string, id string, roleID string, task string) (Agent, error) {
	if r == nil {
		return Agent{}, fmt.Errorf("agent registry is nil")
	}
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return Agent{}, fmt.Errorf("parent id is required")
	}
	r.mu.RLock()
	_, ok := r.agents[parentID]
	r.mu.RUnlock()
	if !ok {
		return Agent{}, fmt.Errorf("parent agent not found: %s", parentID)
	}
	return r.createWithID(parentID, id, roleID, task)
}

func (r *Registry) UpdateTaskStatus(id string, task string, status AgentStatus) (Agent, error) {
	return r.UpdateRoleTaskStatus(id, "", task, status)
}

func (r *Registry) UpdateRoleTask(id string, roleID string, task string) (Agent, error) {
	if r == nil {
		return Agent{}, fmt.Errorf("agent registry is nil")
	}
	id = strings.TrimSpace(id)
	roleID = strings.TrimSpace(roleID)
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[id]
	if !ok {
		return Agent{}, fmt.Errorf("agent not found: %s", id)
	}
	if roleID != "" {
		if r.roles == nil {
			return Agent{}, fmt.Errorf("role registry is nil")
		}
		role, ok := r.roles.Resolve(roleID)
		if !ok {
			return Agent{}, fmt.Errorf("role not found: %s", roleID)
		}
		agent.RoleID = role.ID
	}
	agent.Task = strings.TrimSpace(task)
	agent.UpdatedAt = time.Now()
	r.agents[id] = agent
	return agent, nil
}

func (r *Registry) UpdateRoleTaskStatus(id string, roleID string, task string, status AgentStatus) (Agent, error) {
	if r == nil {
		return Agent{}, fmt.Errorf("agent registry is nil")
	}
	id = strings.TrimSpace(id)
	roleID = strings.TrimSpace(roleID)
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[id]
	if !ok {
		return Agent{}, fmt.Errorf("agent not found: %s", id)
	}
	if roleID != "" {
		if r.roles == nil {
			return Agent{}, fmt.Errorf("role registry is nil")
		}
		role, ok := r.roles.Resolve(roleID)
		if !ok {
			return Agent{}, fmt.Errorf("role not found: %s", roleID)
		}
		agent.RoleID = role.ID
	}
	agent.Task = strings.TrimSpace(task)
	agent.Status = status
	agent.UpdatedAt = time.Now()
	r.agents[id] = agent
	return agent, nil
}

func (r *Registry) Remove(id string) bool {
	if r == nil {
		return false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.agents[id]; !ok {
		return false
	}
	delete(r.agents, id)
	delete(r.children, id)
	for parentID, childIDs := range r.children {
		next := childIDs[:0]
		for _, childID := range childIDs {
			if childID != id {
				next = append(next, childID)
			}
		}
		if len(next) == 0 {
			delete(r.children, parentID)
			continue
		}
		r.children[parentID] = append([]string(nil), next...)
	}
	return true
}

func (r *Registry) createWithID(parentID string, id string, roleID string, task string) (Agent, error) {
	if r == nil {
		return Agent{}, fmt.Errorf("agent registry is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Agent{}, fmt.Errorf("agent id is required")
	}
	if r.roles == nil {
		return Agent{}, fmt.Errorf("role registry is nil")
	}
	role, ok := r.roles.Resolve(roleID)
	if !ok {
		return Agent{}, fmt.Errorf("role not found: %s", roleID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.agents[id]; exists {
		return Agent{}, fmt.Errorf("agent already exists: %s", id)
	}
	now := time.Now()
	agent := Agent{
		ID:        id,
		ParentID:  strings.TrimSpace(parentID),
		RoleID:    role.ID,
		Task:      strings.TrimSpace(task),
		Status:    AgentPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.agents[agent.ID] = agent
	if agent.ParentID != "" {
		r.children[agent.ParentID] = append(r.children[agent.ParentID], agent.ID)
	}
	return agent, nil
}
