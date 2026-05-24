package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/pkg/agentcore"
	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/protocol"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

type stubModelRunner struct {
	lastReq agentcore.ModelRequest
	resp    agentcore.ModelResponse
	err     error
}

func (m *stubModelRunner) RunModel(_ context.Context, req agentcore.ModelRequest) (agentcore.ModelResponse, error) {
	m.lastReq = req
	return m.resp, m.err
}

type stubToolRunner struct {
	lastCall agentcore.ToolCall
	output   agentcore.ToolOutput
	err      error
}

func (r *stubToolRunner) RunTool(_ context.Context, call agentcore.ToolCall) (agentcore.ToolOutput, error) {
	r.lastCall = call
	return r.output, r.err
}

type stubEventSink struct {
	events []agentcore.AgentEvent
}

func (s *stubEventSink) PublishAgentEvent(_ context.Context, event agentcore.AgentEvent) error {
	s.events = append(s.events, event)
	return nil
}

type agentcoreAgentService struct {
	registry *agentcore.Registry
	mailbox  *agentcore.Mailbox
	runner   *agentcore.Runner
}

func newAgentcoreAgentService(modelRunner agentcore.ModelRunner, toolRunner agentcore.ToolRunner) *agentcoreAgentService {
	roles := agentcore.NewDefaultRoleRegistry()
	registry := agentcore.NewRegistry(roles)
	mailbox := agentcore.NewMailbox()
	var runner *agentcore.Runner
	if modelRunner != nil || toolRunner != nil {
		runner = agentcore.NewRunner(registry, mailbox,
			agentcore.WithModelRunner(modelRunner),
			agentcore.WithToolRunner(toolRunner),
			agentcore.WithAgentEventSink(&stubEventSink{}),
		)
	}
	return &agentcoreAgentService{registry: registry, mailbox: mailbox, runner: runner}
}

func (s *agentcoreAgentService) Spawn(_ context.Context, req coreapi.SpawnAgentRequest) (coreapi.Agent, error) {
	roleID := strings.TrimSpace(req.RoleID)
	if roleID == "" {
		roleID = "senior-dev"
	}
	parentID := strings.TrimSpace(req.ParentAgentID)
	var (
		agent agentcore.Agent
		err   error
	)
	if parentID == "" {
		agent, err = s.registry.RegisterRootWithTask(roleID, req.Task)
	} else {
		agent, err = s.registry.Spawn(parentID, roleID, req.Task)
	}
	if err != nil {
		return coreapi.Agent{}, err
	}
	return mapAgentcoreAgent(agent), nil
}

func (s *agentcoreAgentService) SendInput(_ context.Context, req coreapi.AgentInput) error {
	agentID := strings.TrimSpace(req.AgentID)
	if _, ok := s.registry.Get(agentID); !ok {
		return errors.New("agent not found: " + agentID)
	}
	return s.mailbox.Send(agentcore.MailboxMessage{
		FromAgentID: "user",
		ToAgentID:   agentID,
		Body:        req.Input,
	})
}

func (s *agentcoreAgentService) Wait(_ context.Context, ref coreapi.AgentRef) (coreapi.Agent, error) {
	agent, ok := s.registry.Get(ref.AgentID)
	if !ok {
		return coreapi.Agent{}, errors.New("agent not found: " + ref.AgentID)
	}
	return mapAgentcoreAgent(agent), nil
}

func (s *agentcoreAgentService) Run(ctx context.Context, req coreapi.RunAgentRequest) (coreapi.AgentRunResult, error) {
	if s.runner == nil {
		return coreapi.AgentRunResult{}, agentcore.ErrNoModelRunner
	}
	result, err := s.runner.RunOnce(ctx, req.AgentID, req.Options)
	return mapAgentcoreRunResult(result), err
}

func (s *agentcoreAgentService) RunTool(ctx context.Context, req coreapi.AgentToolRequest) (coreapi.AgentToolResult, error) {
	if s.runner == nil {
		return coreapi.AgentToolResult{}, agentcore.ErrNoToolRunner
	}
	output, err := s.runner.RunTool(ctx, req.AgentID, req.Name, req.Args)
	return mapAgentcoreToolOutput(output), err
}

func (s *agentcoreAgentService) List(_ context.Context, _ coreapi.ListAgentsRequest) ([]coreapi.Agent, error) {
	items := s.registry.List()
	out := make([]coreapi.Agent, 0, len(items))
	for _, item := range items {
		out = append(out, mapAgentcoreAgent(item))
	}
	return out, nil
}

func (s *agentcoreAgentService) Close(_ context.Context, ref coreapi.AgentRef) error {
	_, err := s.registry.UpdateStatus(ref.AgentID, agentcore.AgentCancelled)
	return err
}

type agentcoreRoleService struct {
	registry *agentcore.RoleRegistry
}

func newAgentcoreRoleService() *agentcoreRoleService {
	return &agentcoreRoleService{registry: agentcore.NewDefaultRoleRegistry()}
}

func (s *agentcoreRoleService) List(context.Context) ([]coreapi.RoleConfig, error) {
	items := s.registry.List()
	out := make([]coreapi.RoleConfig, 0, len(items))
	for _, item := range items {
		out = append(out, mapAgentcoreRole(item))
	}
	return out, nil
}

func (s *agentcoreRoleService) Resolve(_ context.Context, ref coreapi.RoleRef) (coreapi.RoleConfig, error) {
	role, ok := s.registry.Resolve(ref.ID)
	if !ok {
		return coreapi.RoleConfig{}, errors.New("role not found: " + ref.ID)
	}
	return mapAgentcoreRole(role), nil
}

func mapAgentcoreAgent(agent agentcore.Agent) coreapi.Agent {
	return coreapi.Agent{
		ID:            agent.ID,
		ParentAgentID: agent.ParentID,
		RoleID:        agent.RoleID,
		Task:          agent.Task,
		Status:        string(agent.Status),
		CreatedAt:     agent.CreatedAt,
		UpdatedAt:     agent.UpdatedAt,
	}
}

func mapAgentcoreRunResult(result agentcore.AgentRunResult) coreapi.AgentRunResult {
	msgs := make([]coreapi.AgentMailboxMessage, 0, len(result.Messages))
	for _, m := range result.Messages {
		msgs = append(msgs, coreapi.AgentMailboxMessage{
			FromAgentID: m.FromAgentID,
			Body:        m.Body,
			CreatedAt:   m.CreatedAt,
		})
	}
	return coreapi.AgentRunResult{
		Agent:    mapAgentcoreAgent(result.Agent),
		Role:     mapAgentcoreRole(result.Role),
		Messages: msgs,
		Output:   result.Output,
	}
}

func mapAgentcoreToolOutput(output agentcore.ToolOutput) coreapi.AgentToolResult {
	return coreapi.AgentToolResult{
		Name:    output.Name,
		Display: output.Display,
		Output:  output.Output,
		Error:   output.Error,
	}
}

func mapAgentcoreRole(role agentcore.Role) coreapi.RoleConfig {
	return coreapi.RoleConfig{
		ID:              role.ID,
		Description:     role.Description,
		SystemPrompt:    role.SystemPrompt,
		PromptFile:      role.PromptFile,
		AllowedTools:    append([]string(nil), role.AllowedTools...),
		ContextStrategy: string(role.ContextStrategy),
		Model:           role.Model,
		ReasoningEffort: role.ReasoningEffort,
		LegacyAliases:   append([]string(nil), role.LegacyAliases...),
	}
}

func TestE2ERoleListReturnsBuiltinRoles(t *testing.T) {
	roles := newAgentcoreRoleService()
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		roles:    roles,
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var items []coreapi.RoleConfig
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRoleList, nil, &items); err != nil {
		t.Fatalf("Call(role/list) error = %v", err)
	}
	if len(items) == 0 {
		t.Fatal("role/list returned no roles")
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.ID] = true
	}
	for _, want := range []string{"planner", "senior-dev", "tester", "architect"} {
		if !seen[want] {
			t.Errorf("role/list missing %q; got ids: %v", want, keysOfMap(seen))
		}
	}
}

func TestE2ERoleResolveLegacyAlias(t *testing.T) {
	roles := newAgentcoreRoleService()
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		roles:    roles,
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	for _, alias := range []string{"senior_dev", "developer"} {
		var role coreapi.RoleConfig
		if err := client.Call(context.Background(), protocoljsonrpc.MethodRoleResolve, coreapi.RoleRef{ID: alias}, &role); err != nil {
			t.Fatalf("Call(role/resolve, %q) error = %v", alias, err)
		}
		if role.ID != "senior-dev" {
			t.Errorf("role/resolve(%q) = %q, want senior-dev", alias, role.ID)
		}
	}

	var explorer coreapi.RoleConfig
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRoleResolve, coreapi.RoleRef{ID: "explorer"}, &explorer); err != nil {
		t.Fatalf("Call(role/resolve, explorer) error = %v", err)
	}
	if explorer.ID != "explore" {
		t.Errorf("role/resolve(explorer) = %q, want explore", explorer.ID)
	}
}

func TestE2EAgentSpawnInputListClose(t *testing.T) {
	modelRunner := &stubModelRunner{
		resp: agentcore.ModelResponse{Text: "task complete", Status: agentcore.AgentCompleted},
	}
	agents := newAgentcoreAgentService(modelRunner, nil)
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 4)}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		agents:   agents,
	}, Options{Notifier: notifier}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var spawned coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentSpawn, coreapi.SpawnAgentRequest{
		RoleID: "senior_dev",
		Task:   "implement feature X",
	}, &spawned); err != nil {
		t.Fatalf("Call(agent/spawn) error = %v", err)
	}
	if spawned.ID == "" {
		t.Fatal("spawned.ID is empty")
	}
	if spawned.RoleID != "senior-dev" {
		t.Fatalf("spawned.RoleID=%q, want senior-dev", spawned.RoleID)
	}
	if spawned.Status != "pending" {
		t.Fatalf("spawned.Status=%q, want pending", spawned.Status)
	}
	assertAgentNotification(t, notifier.ch, protocol.EventTypeAgentStarted, spawned.ID)

	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentInput, coreapi.AgentInput{
		AgentID: spawned.ID,
		Input:   "please also add tests",
	}, nil); err != nil {
		t.Fatalf("Call(agent/input) error = %v", err)
	}
	assertAgentNotification(t, notifier.ch, protocol.EventTypeAgentProgress, spawned.ID)

	var items []coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentList, coreapi.ListAgentsRequest{}, &items); err != nil {
		t.Fatalf("Call(agent/list) error = %v", err)
	}
	if len(items) != 1 || items[0].ID != spawned.ID {
		t.Fatalf("agent/list=%+v, want 1 agent with ID %s", items, spawned.ID)
	}

	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentClose, coreapi.AgentRef{AgentID: spawned.ID}, nil); err != nil {
		t.Fatalf("Call(agent/close) error = %v", err)
	}
	assertAgentNotification(t, notifier.ch, protocol.EventTypeAgentCancelled, spawned.ID)

	agentAfter, ok := agents.registry.Get(spawned.ID)
	if !ok {
		t.Fatal("agent not found after close")
	}
	if agentAfter.Status != agentcore.AgentCancelled {
		t.Fatalf("agent status after close=%q, want cancelled", agentAfter.Status)
	}
}

func TestE2EMailboxInputFlowsToRunRequest(t *testing.T) {
	modelRunner := &stubModelRunner{
		resp: agentcore.ModelResponse{Text: "acknowledged", Status: agentcore.AgentCompleted},
	}
	agents := newAgentcoreAgentService(modelRunner, nil)
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		agents:   agents,
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var spawned coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentSpawn, coreapi.SpawnAgentRequest{
		RoleID: "senior-dev",
		Task:   "review the code",
	}, &spawned); err != nil {
		t.Fatalf("Call(agent/spawn) error = %v", err)
	}

	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentInput, coreapi.AgentInput{
		AgentID: spawned.ID,
		Input:   "focus on error handling",
	}, nil); err != nil {
		t.Fatalf("Call(agent/input) error = %v", err)
	}

	var runResult coreapi.AgentRunResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentRun, coreapi.RunAgentRequest{
		AgentID: spawned.ID,
	}, &runResult); err != nil {
		t.Fatalf("Call(agent/run) error = %v", err)
	}

	if runResult.Output != "acknowledged" {
		t.Fatalf("runResult.Output=%q, want acknowledged", runResult.Output)
	}
	if len(modelRunner.lastReq.Messages) == 0 {
		t.Fatal("modelRunner.lastReq.Messages is empty, want at least 1 mailbox message")
	}
	found := false
	for _, msg := range modelRunner.lastReq.Messages {
		if strings.Contains(msg.Body, "focus on error handling") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mailbox message not found in model request; messages=%+v", modelRunner.lastReq.Messages)
	}
}

func TestE2EAllowedToolsRestrictsAgentToolExecute(t *testing.T) {
	toolRunner := &stubToolRunner{
		output: agentcore.ToolOutput{Name: "bash", Display: "ok", Output: json.RawMessage(`{"status":"ok"}`)},
	}
	roles, err := agentcore.NewRoleRegistry([]agentcore.Role{
		{
			ID:           "restricted",
			SystemPrompt: "You are a restricted agent.",
			AllowedTools: []string{"fs/*", "read"},
		},
	})
	if err != nil {
		t.Fatalf("NewRoleRegistry() error = %v", err)
	}
	registry := agentcore.NewRegistry(roles)
	mailbox := agentcore.NewMailbox()
	runner := agentcore.NewRunner(registry, mailbox,
		agentcore.WithToolRunner(toolRunner),
		agentcore.WithAgentEventSink(&stubEventSink{}),
	)
	svc := &agentcoreAgentService{registry: registry, mailbox: mailbox, runner: runner}

	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		agents:   svc,
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var spawned coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentSpawn, coreapi.SpawnAgentRequest{
		RoleID: "restricted",
		Task:   "do work",
	}, &spawned); err != nil {
		t.Fatalf("Call(agent/spawn) error = %v", err)
	}

	var allowed coreapi.AgentToolResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentToolExecute, coreapi.AgentToolRequest{
		AgentID: spawned.ID,
		Name:    "fs/read",
		Args:    json.RawMessage(`{"path":"test.txt"}`),
	}, &allowed); err != nil {
		t.Fatalf("Call(agent/tool/execute fs/read) error = %v", err)
	}
	if allowed.Error != "" {
		t.Fatalf("allowed tool fs/read returned error=%q", allowed.Error)
	}

	var denied coreapi.AgentToolResult
	err = client.Call(context.Background(), protocoljsonrpc.MethodAgentToolExecute, coreapi.AgentToolRequest{
		AgentID: spawned.ID,
		Name:    "bash",
		Args:    json.RawMessage(`{"command":"ls"}`),
	}, &denied)
	if err == nil {
		t.Fatal("expected error for denied tool bash, got nil")
	}
	if !strings.Contains(err.Error(), "tool") && !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("unexpected error for denied tool: %v", err)
	}
}

func TestE2EAgentCloseStateObservable(t *testing.T) {
	modelRunner := &stubModelRunner{
		resp: agentcore.ModelResponse{Text: "done", Status: agentcore.AgentCompleted},
	}
	agents := newAgentcoreAgentService(modelRunner, nil)
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 2)}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		agents:   agents,
	}, Options{Notifier: notifier}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var spawned coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentSpawn, coreapi.SpawnAgentRequest{
		RoleID: "senior-dev",
		Task:   "build module",
	}, &spawned); err != nil {
		t.Fatalf("Call(agent/spawn) error = %v", err)
	}
	assertAgentNotification(t, notifier.ch, protocol.EventTypeAgentStarted, spawned.ID)

	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentClose, coreapi.AgentRef{AgentID: spawned.ID}, nil); err != nil {
		t.Fatalf("Call(agent/close) error = %v", err)
	}
	assertAgentNotification(t, notifier.ch, protocol.EventTypeAgentCancelled, spawned.ID)

	var waited coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentWait, coreapi.AgentRef{AgentID: spawned.ID}, &waited); err != nil {
		t.Fatalf("Call(agent/wait) error = %v", err)
	}
	if waited.Status != "cancelled" {
		t.Fatalf("waited.Status=%q after close, want cancelled", waited.Status)
	}

	var items []coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentList, coreapi.ListAgentsRequest{}, &items); err != nil {
		t.Fatalf("Call(agent/list) error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("agent/list returned %d items, want 1", len(items))
	}
	if items[0].Status != "cancelled" {
		t.Fatalf("agent/list[0].Status=%q, want cancelled", items[0].Status)
	}
}

func TestE2EResponseOmitsJSONRPCField(t *testing.T) {
	roles := newAgentcoreRoleService()
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		roles:    roles,
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	req, err := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("e2e_1"), protocoljsonrpc.MethodRoleList, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp := router.Handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("Handle error = %v", resp.Error)
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal(response) error = %v", err)
	}
	raw := string(data)
	if strings.Contains(raw, `"jsonrpc"`) {
		t.Fatalf("response contains jsonrpc field: %s", raw)
	}
}

func keysOfMap(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

var _ coreapi.AgentService = (*agentcoreAgentService)(nil)
var _ coreapi.RoleService = (*agentcoreRoleService)(nil)
var _ agentcore.ModelRunner = (*stubModelRunner)(nil)
var _ agentcore.ToolRunner = (*stubToolRunner)(nil)
var _ agentcore.AgentEventSink = (*stubEventSink)(nil)
