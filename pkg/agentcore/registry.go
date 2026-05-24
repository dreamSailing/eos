package agentcore

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type AgentStatus string

const (
	AgentPending   AgentStatus = "pending"
	AgentRunning   AgentStatus = "running"
	AgentCompleted AgentStatus = "completed"
	AgentFailed    AgentStatus = "failed"
	AgentCancelled AgentStatus = "cancelled"
)

type Agent struct {
	ID        string
	ParentID  string
	RoleID    string
	Task      string
	Status    AgentStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Registry struct {
	mu       sync.RWMutex
	next     int
	agents   map[string]Agent
	children map[string][]string
	roles    *RoleRegistry
}

func NewRegistry(roles *RoleRegistry) *Registry {
	if roles == nil {
		roles = NewDefaultRoleRegistry()
	}
	return &Registry{
		agents:   make(map[string]Agent),
		children: make(map[string][]string),
		roles:    roles,
	}
}

func (r *Registry) RegisterRoot(roleID string) (Agent, error) {
	return r.create("", roleID, "")
}

func (r *Registry) RegisterRootWithTask(roleID string, task string) (Agent, error) {
	return r.create("", roleID, task)
}

func (r *Registry) Spawn(parentID string, roleID string, task string) (Agent, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return Agent{}, errors.New("parent id is required")
	}
	r.mu.RLock()
	_, ok := r.agents[parentID]
	r.mu.RUnlock()
	if !ok {
		return Agent{}, fmt.Errorf("parent agent not found: %s", parentID)
	}
	return r.create(parentID, roleID, task)
}

func (r *Registry) Get(id string) (Agent, bool) {
	id = strings.TrimSpace(id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[id]
	return agent, ok
}

func (r *Registry) ResolveRole(id string) (Role, bool) {
	if r == nil || r.roles == nil {
		return Role{}, false
	}
	return r.roles.Resolve(id)
}

func (r *Registry) UpdateStatus(id string, status AgentStatus) (Agent, error) {
	id = strings.TrimSpace(id)
	r.mu.Lock()
	defer r.mu.Unlock()
	agent, ok := r.agents[id]
	if !ok {
		return Agent{}, fmt.Errorf("agent not found: %s", id)
	}
	agent.Status = status
	agent.UpdatedAt = time.Now()
	r.agents[id] = agent
	return agent, nil
}

func (r *Registry) List() []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		out = append(out, agent)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (r *Registry) Children(parentID string) []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := append([]string(nil), r.children[strings.TrimSpace(parentID)]...)
	out := make([]Agent, 0, len(ids))
	for _, id := range ids {
		if agent, ok := r.agents[id]; ok {
			out = append(out, agent)
		}
	}
	return out
}

func (r *Registry) create(parentID string, roleID string, task string) (Agent, error) {
	role, ok := r.roles.Resolve(roleID)
	if !ok {
		return Agent{}, fmt.Errorf("role not found: %s", roleID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	now := time.Now()
	agent := Agent{
		ID:        fmt.Sprintf("agent_%d", r.next),
		ParentID:  parentID,
		RoleID:    role.ID,
		Task:      strings.TrimSpace(task),
		Status:    AgentPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.agents[agent.ID] = agent
	if parentID != "" {
		r.children[parentID] = append(r.children[parentID], agent.ID)
	}
	return agent, nil
}

type MailboxMessage struct {
	FromAgentID string
	ToAgentID   string
	Body        string
	CreatedAt   time.Time
}

type Mailbox struct {
	mu       sync.Mutex
	messages map[string][]MailboxMessage
}

func NewMailbox() *Mailbox {
	return &Mailbox{messages: make(map[string][]MailboxMessage)}
}

func (m *Mailbox) Send(msg MailboxMessage) error {
	msg.ToAgentID = strings.TrimSpace(msg.ToAgentID)
	msg.FromAgentID = strings.TrimSpace(msg.FromAgentID)
	msg.Body = strings.TrimSpace(msg.Body)
	if msg.ToAgentID == "" {
		return errors.New("to agent id is required")
	}
	if msg.Body == "" {
		return errors.New("message body is required")
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[msg.ToAgentID] = append(m.messages[msg.ToAgentID], msg)
	return nil
}

func (m *Mailbox) Drain(agentID string) []MailboxMessage {
	agentID = strings.TrimSpace(agentID)
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]MailboxMessage(nil), m.messages[agentID]...)
	delete(m.messages, agentID)
	return out
}

func (m *Mailbox) List(agentID string) []MailboxMessage {
	agentID = strings.TrimSpace(agentID)
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MailboxMessage(nil), m.messages[agentID]...)
}

func (m *Mailbox) Clear(agentID string) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.messages, agentID)
}
