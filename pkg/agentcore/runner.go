package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrAgentNotFound   = errors.New("agent not found")
	ErrNoModelRunner   = errors.New("agent model runner is not configured")
	ErrNoToolRunner    = errors.New("agent tool runner is not configured")
	ErrNoAgentRegistry = errors.New("agent registry is not configured")
	ErrToolNotAllowed  = errors.New("agent tool is not allowed")
)

type AgentMessage struct {
	FromAgentID string    `json:"from_agent_id,omitempty"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type ModelRequest struct {
	Agent           Agent           `json:"agent"`
	Role            Role            `json:"role"`
	Task            string          `json:"task,omitempty"`
	Messages        []AgentMessage  `json:"messages,omitempty"`
	AllowedTools    []string        `json:"allowed_tools,omitempty"`
	ContextStrategy ContextStrategy `json:"context_strategy,omitempty"`
	Model           string          `json:"model,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	Options         json.RawMessage `json:"options,omitempty"`
}

type ModelResponse struct {
	Text   string      `json:"text,omitempty"`
	Status AgentStatus `json:"status,omitempty"`
}

type ModelRunner interface {
	RunModel(context.Context, ModelRequest) (ModelResponse, error)
}

type ToolCall struct {
	Agent Agent           `json:"agent"`
	Name  string          `json:"name"`
	Args  json.RawMessage `json:"args,omitempty"`
}

type ToolOutput struct {
	Name    string          `json:"name"`
	Display string          `json:"display,omitempty"`
	Output  json.RawMessage `json:"output,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type ToolRunner interface {
	RunTool(context.Context, ToolCall) (ToolOutput, error)
}

type AgentEvent struct {
	Agent   Agent       `json:"agent"`
	Status  AgentStatus `json:"status"`
	Message string      `json:"message,omitempty"`
	Output  string      `json:"output,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type AgentEventSink interface {
	PublishAgentEvent(context.Context, AgentEvent) error
}

type AgentRunResult struct {
	Agent    Agent          `json:"agent"`
	Role     Role           `json:"role"`
	Messages []AgentMessage `json:"messages,omitempty"`
	Output   string         `json:"output,omitempty"`
}

type Runner struct {
	registry    *Registry
	mailbox     *Mailbox
	modelRunner ModelRunner
	toolRunner  ToolRunner
	eventSink   AgentEventSink
}

type RunnerOption func(*Runner)

func NewRunner(registry *Registry, mailbox *Mailbox, opts ...RunnerOption) *Runner {
	if mailbox == nil {
		mailbox = NewMailbox()
	}
	r := &Runner{registry: registry, mailbox: mailbox}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

func WithModelRunner(modelRunner ModelRunner) RunnerOption {
	return func(r *Runner) {
		r.modelRunner = modelRunner
	}
}

func WithToolRunner(toolRunner ToolRunner) RunnerOption {
	return func(r *Runner) {
		r.toolRunner = toolRunner
	}
}

func WithAgentEventSink(eventSink AgentEventSink) RunnerOption {
	return func(r *Runner) {
		r.eventSink = eventSink
	}
}

func (r *Runner) RunOnce(ctx context.Context, agentID string, options json.RawMessage) (AgentRunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.registry == nil {
		return AgentRunResult{}, ErrNoAgentRegistry
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return AgentRunResult{}, ErrAgentNotFound
	}
	agent, ok := r.registry.Get(agentID)
	if !ok {
		return AgentRunResult{}, ErrAgentNotFound
	}
	role, ok := r.registry.ResolveRole(agent.RoleID)
	if !ok {
		return AgentRunResult{Agent: agent}, errors.New("agent role not found")
	}
	messages := mailboxMessages(r.mailbox.Drain(agent.ID))

	agent, err := r.registry.UpdateStatus(agent.ID, AgentRunning)
	if err != nil {
		return AgentRunResult{}, err
	}
	r.publish(ctx, AgentEvent{Agent: agent, Status: AgentRunning, Message: "Agent running"})

	req := ModelRequest{
		Agent:           agent,
		Role:            role,
		Task:            strings.TrimSpace(agent.Task),
		Messages:        messages,
		AllowedTools:    append([]string(nil), role.AllowedTools...),
		ContextStrategy: role.ContextStrategy,
		Model:           strings.TrimSpace(role.Model),
		ReasoningEffort: strings.TrimSpace(role.ReasoningEffort),
		Options:         append(json.RawMessage(nil), options...),
	}
	if r.modelRunner == nil {
		agent, _ = r.registry.UpdateStatus(agent.ID, AgentFailed)
		r.publish(ctx, AgentEvent{Agent: agent, Status: AgentFailed, Error: ErrNoModelRunner.Error()})
		return AgentRunResult{Agent: agent, Role: role, Messages: messages}, ErrNoModelRunner
	}

	resp, err := r.modelRunner.RunModel(ctx, req)
	if err != nil {
		status := AgentFailed
		if errors.Is(err, context.Canceled) {
			status = AgentCancelled
		}
		agent, _ = r.registry.UpdateStatus(agent.ID, status)
		r.publish(ctx, AgentEvent{Agent: agent, Status: status, Error: err.Error()})
		return AgentRunResult{Agent: agent, Role: role, Messages: messages}, err
	}

	status := resp.Status
	if status == "" {
		status = AgentCompleted
	}
	agent, err = r.registry.UpdateStatus(agent.ID, status)
	if err != nil {
		return AgentRunResult{}, err
	}
	r.publish(ctx, AgentEvent{
		Agent:   agent,
		Status:  status,
		Message: "Agent completed",
		Output:  strings.TrimSpace(resp.Text),
	})
	return AgentRunResult{
		Agent:    agent,
		Role:     role,
		Messages: messages,
		Output:   strings.TrimSpace(resp.Text),
	}, nil
}

func (r *Runner) ToolRunner() ToolRunner {
	if r == nil {
		return nil
	}
	return r.toolRunner
}

func (r *Runner) RunTool(ctx context.Context, agentID string, name string, args json.RawMessage) (ToolOutput, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.registry == nil {
		return ToolOutput{}, ErrNoAgentRegistry
	}
	if r.toolRunner == nil {
		return ToolOutput{}, ErrNoToolRunner
	}
	agentID = strings.TrimSpace(agentID)
	agent, ok := r.registry.Get(agentID)
	if !ok {
		return ToolOutput{}, ErrAgentNotFound
	}
	role, ok := r.registry.ResolveRole(agent.RoleID)
	if !ok {
		return ToolOutput{}, errors.New("agent role not found")
	}
	name = strings.TrimSpace(name)
	if !toolAllowed(role.AllowedTools, name) {
		return ToolOutput{Name: name, Error: ErrToolNotAllowed.Error()}, ErrToolNotAllowed
	}
	output, err := r.toolRunner.RunTool(ctx, ToolCall{
		Agent: agent,
		Name:  name,
		Args:  append(json.RawMessage(nil), args...),
	})
	if err != nil {
		r.publish(ctx, AgentEvent{Agent: agent, Status: agent.Status, Error: err.Error()})
		return output, err
	}
	r.publish(ctx, AgentEvent{Agent: agent, Status: agent.Status, Message: strings.TrimSpace(output.Display), Output: string(output.Output)})
	return output, nil
}

func (r *Runner) publish(ctx context.Context, event AgentEvent) {
	if r == nil || r.eventSink == nil {
		return
	}
	_ = r.eventSink.PublishAgentEvent(ctx, event)
}

func mailboxMessages(items []MailboxMessage) []AgentMessage {
	out := make([]AgentMessage, 0, len(items))
	for _, item := range items {
		body := strings.TrimSpace(item.Body)
		if body == "" {
			continue
		}
		out = append(out, AgentMessage{
			FromAgentID: strings.TrimSpace(item.FromAgentID),
			Body:        body,
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}

func toolAllowed(allowed []string, name string) bool {
	name = normalizeToolName(name)
	if name == "" {
		return false
	}
	aliases := toolNameAliases(name)
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		pattern := normalizeToolName(item)
		if pattern == "" {
			continue
		}
		for _, alias := range aliases {
			if toolPatternMatches(pattern, alias) {
				return true
			}
		}
	}
	return false
}

func toolPatternMatches(pattern string, name string) bool {
	if pattern == "*" || pattern == name {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		base := strings.TrimSuffix(pattern, "/*")
		return name == base || strings.HasPrefix(name, base+"/")
	}
	if strings.HasSuffix(pattern, ".*") {
		base := strings.TrimSuffix(pattern, ".*")
		return name == base || strings.HasPrefix(name, base+".")
	}
	if strings.HasSuffix(pattern, "/") || strings.HasSuffix(pattern, ".") {
		return strings.HasPrefix(name, pattern)
	}
	return false
}

func toolNameAliases(name string) []string {
	name = normalizeToolName(name)
	if name == "" {
		return nil
	}
	out := []string{name}
	for _, sep := range []string{"/", ".", ":"} {
		if idx := strings.LastIndex(name, sep); idx >= 0 && idx < len(name)-1 {
			out = append(out, name[idx+1:])
		}
	}
	if strings.HasPrefix(name, "fs/") || strings.HasPrefix(name, "fs.") {
		out = append(out, "fs")
	}
	return uniqueToolNames(out)
}

func uniqueToolNames(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = normalizeToolName(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeToolName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\\", "/")
	return name
}
