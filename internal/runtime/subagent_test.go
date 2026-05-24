package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/pkg/agentcore"
)

type fakeSubAgentModelRunner struct {
	req  agentcore.ModelRequest
	resp agentcore.ModelResponse
	err  error
}

func (r *fakeSubAgentModelRunner) RunModel(_ context.Context, req agentcore.ModelRequest) (agentcore.ModelResponse, error) {
	r.req = req
	return r.resp, r.err
}

func TestSubAgentManager_RequestContextReuseAndClear(t *testing.T) {
	m := NewSubAgentManager()
	msgs := []*schema.Message{schema.UserMessage("hello")}

	c1 := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), msgs)
	if c1 == nil {
		t.Fatalf("expected context, got nil")
		return
	}
	c1ID := c1.id
	_ = m.AddMessage(c1ID, schema.AssistantMessage("a", nil))

	c2 := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), msgs)
	if c2 == nil {
		t.Fatalf("expected context, got nil")
		return
	}
	c2ID := c2.id
	if c1ID != c2ID {
		t.Fatalf("expected same id, got %q vs %q", c1ID, c2ID)
	}
	if got := len(m.GetMessages(c1ID)); got < 2 {
		t.Fatalf("expected messages persisted, got %d", got)
	}

	m.ClearRequest("rid")
	c3 := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), msgs)
	if c3 == nil {
		t.Fatalf("expected context, got nil")
		return
	}
	c3ID := c3.id
	if c3ID == c1ID {
		t.Fatalf("expected new id after clear, got %q", c3ID)
	}
}

func TestSubAgentManager_ClearRequestCancelsRunningAgents(t *testing.T) {
	m := NewSubAgentManager()
	sub := m.GetOrCreateRequestContext("rid", SubAgentTypeSeniorDev, context.Background(), []*schema.Message{schema.UserMessage("hello")})
	if sub == nil {
		t.Fatalf("expected context, got nil")
	}
	cancelled := false
	if err := m.MarkRunning(sub.id, "task", func() { cancelled = true }); err != nil {
		t.Fatalf("MarkRunning error: %v", err)
	}

	m.ClearRequest("rid")

	if !cancelled {
		t.Fatalf("expected ClearRequest to cancel running agent")
	}
}

func TestSubAgentManager_RequestContextWithStrategyUsesProvidedPolicy(t *testing.T) {
	m := NewSubAgentManager()
	msgs := []*schema.Message{schema.UserMessage("hello")}

	sub := m.GetOrCreateRequestContextWithStrategy(
		"rid",
		SubAgentTypeTester,
		context.Background(),
		msgs,
		ContextStrategyIndependent,
		[]string{"read"},
	)
	if sub == nil {
		t.Fatalf("expected context, got nil")
	}
	if got := sub.strategy; got != ContextStrategyIndependent {
		t.Fatalf("strategy = %s, want independent", got)
	}
	if got := m.GetMessages(sub.id); len(got) != 0 {
		t.Fatalf("independent strategy should filter initial messages, got %d", len(got))
	}
	if len(sub.allowedTools) != 1 || sub.allowedTools[0] != "read" {
		t.Fatalf("allowed tools = %#v, want [read]", sub.allowedTools)
	}
}

func TestSubAgentManagerMirrorsLifecycleToAgentCoreRegistry(t *testing.T) {
	m := NewSubAgentManager()
	sub := m.CreateContextWithStrategy(
		SubAgentTypeVerification,
		context.Background(),
		[]*schema.Message{schema.UserMessage("verify")},
		ContextStrategyIndependent,
		[]string{"read"},
	)
	if sub == nil {
		t.Fatalf("expected context, got nil")
	}
	registry := m.AgentCoreRegistry()
	if registry == nil {
		t.Fatalf("expected agentcore registry")
	}
	agent, ok := registry.Get(sub.id)
	if !ok {
		t.Fatalf("agentcore registry missing %q", sub.id)
	}
	if agent.ID != sub.id || agent.RoleID != "verification" || agent.Status != "pending" {
		t.Fatalf("agent=%+v, want mirrored pending verification agent", agent)
	}

	if err := m.MarkRunning(sub.id, "verify task", nil); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	agent, ok = registry.Get(sub.id)
	if !ok {
		t.Fatalf("agentcore registry missing %q after running", sub.id)
	}
	if agent.Task != "verify task" || agent.Status != "running" {
		t.Fatalf("agent=%+v, want running verify task", agent)
	}

	res := m.Complete(sub.id, "verify task", true, "")
	if !res.Success {
		t.Fatalf("Complete() success=false: %+v", res)
	}
	agent, ok = registry.Get(sub.id)
	if !ok {
		t.Fatalf("agentcore registry missing %q after complete", sub.id)
	}
	if agent.Status != "completed" || agent.Task != "verify task" {
		t.Fatalf("agent=%+v, want completed verify task", agent)
	}

	if !m.Remove(sub.id) {
		t.Fatalf("Remove() = false, want true")
	}
	if _, ok := registry.Get(sub.id); ok {
		t.Fatalf("agentcore registry should remove %q", sub.id)
	}
}

func TestSubAgentManagerMirrorsUserInputToAgentCoreMailbox(t *testing.T) {
	m := NewSubAgentManager()
	sub := m.CreateContext(SubAgentTypeSeniorDev, context.Background(), nil)
	if sub == nil {
		t.Fatalf("expected context, got nil")
	}
	if m.AgentCoreRunner() == nil {
		t.Fatalf("expected agentcore runner shell")
	}
	box := m.AgentCoreMailbox()
	if box == nil {
		t.Fatalf("expected agentcore mailbox")
	}

	if err := m.AddMessage(sub.id, schema.UserMessage("continue work")); err != nil {
		t.Fatalf("AddMessage(user) error = %v", err)
	}
	if err := m.AddMessage(sub.id, schema.SystemMessage("extra context")); err != nil {
		t.Fatalf("AddMessage(system) error = %v", err)
	}
	if err := m.AddMessage(sub.id, schema.AssistantMessage("assistant output", nil)); err != nil {
		t.Fatalf("AddMessage(assistant) error = %v", err)
	}

	items := box.List(sub.id)
	if len(items) != 2 {
		t.Fatalf("mailbox items=%+v, want user and system messages", items)
	}
	if items[0].FromAgentID != "user" || items[0].Body != "continue work" {
		t.Fatalf("first mailbox item=%+v, want user continue work", items[0])
	}
	if items[1].FromAgentID != "system" || items[1].Body != "extra context" {
		t.Fatalf("second mailbox item=%+v, want system extra context", items[1])
	}

	if !m.Remove(sub.id) {
		t.Fatalf("Remove() = false, want true")
	}
	if got := box.List(sub.id); len(got) != 0 {
		t.Fatalf("mailbox after Remove=%+v, want none", got)
	}
}

func TestSubAgentManagerAgentCoreRunnerOwnsLifecycle(t *testing.T) {
	m := NewSubAgentManager()
	sub := m.CreateContextWithStrategy(SubAgentTypeSeniorDev, context.Background(), nil, ContextStrategyShared, nil)
	if sub == nil {
		t.Fatalf("expected context, got nil")
	}
	if err := m.BeginAgentCoreRun(sub.id, "build via core", nil); err != nil {
		t.Fatalf("BeginAgentCoreRun() error = %v", err)
	}
	if snap := snapshotFromContext(sub); snap.Status != AgentStatusRunning {
		t.Fatalf("legacy snapshot status=%q, want running", snap.Status)
	}
	coreBefore, ok := m.AgentCoreRegistry().Get(sub.id)
	if !ok {
		t.Fatalf("agentcore registry missing %q", sub.id)
	}
	if coreBefore.Task != "build via core" || coreBefore.Status != agentcore.AgentPending {
		t.Fatalf("core before run=%+v, want task updated with pending status", coreBefore)
	}

	model := &fakeSubAgentModelRunner{resp: agentcore.ModelResponse{Text: "runner done", Status: agentcore.AgentCompleted}}
	runner := m.NewAgentCoreRunner(agentcore.WithModelRunner(model))
	runResult, err := runner.RunOnce(context.Background(), sub.id, nil)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	res := m.FinishAgentCoreRun(sub.id, "build via core", runResult, nil)
	if !res.Success || res.Status != AgentStatusCompleted || res.Result != "runner done" {
		t.Fatalf("FinishAgentCoreRun()=%+v, want completed runner output", res)
	}
	if model.req.Agent.ID != sub.id || model.req.Task != "build via core" || model.req.Role.ID != "senior-dev" {
		t.Fatalf("model request=%+v, want visible legacy id/task/role", model.req)
	}
	coreAfter, _ := m.AgentCoreRegistry().Get(sub.id)
	if coreAfter.Status != agentcore.AgentCompleted || coreAfter.Task != "build via core" {
		t.Fatalf("core after run=%+v, want completed task", coreAfter)
	}
}

func TestSubAgentManagerAgentCoreRunnerFailureMirrorsToLegacy(t *testing.T) {
	m := NewSubAgentManager()
	sub := m.CreateContext(SubAgentTypeReviewer, context.Background(), nil)
	if err := m.BeginAgentCoreRun(sub.id, "review via core", nil); err != nil {
		t.Fatalf("BeginAgentCoreRun() error = %v", err)
	}
	modelErr := errors.New("model failed")
	runner := m.NewAgentCoreRunner(agentcore.WithModelRunner(&fakeSubAgentModelRunner{err: modelErr}))
	runResult, err := runner.RunOnce(context.Background(), sub.id, nil)
	if !errors.Is(err, modelErr) {
		t.Fatalf("RunOnce() error = %v, want model failed", err)
	}
	res := m.FinishAgentCoreRun(sub.id, "review via core", runResult, err)
	if res.Success || res.Status != AgentStatusFailed || res.Error != modelErr.Error() {
		t.Fatalf("FinishAgentCoreRun()=%+v, want failed model error", res)
	}
	coreAfter, _ := m.AgentCoreRegistry().Get(sub.id)
	if coreAfter.Status != agentcore.AgentFailed {
		t.Fatalf("core status=%q, want failed", coreAfter.Status)
	}
}

func TestAgentRegistryControlsRegisteredManagers(t *testing.T) {
	registry := &AgentRegistry{entries: map[string]agentRegistryEntry{}}
	m := NewSubAgentManager()
	registryID := registry.RegisterManager(m)
	defer registry.UnregisterManager(registryID)
	sub := m.CreateContext(SubAgentTypeReviewer, context.Background(), nil)
	if sub == nil {
		t.Fatalf("CreateContext() = nil")
	}

	if snap, ok := registry.Snapshot(sub.id); !ok || snap.ID != sub.id || snap.RoleID != "reviewer" {
		t.Fatalf("Snapshot()=(%+v,%v), want reviewer %q", snap, ok, sub.id)
	}
	if !registry.SendInput(sub.id, "please continue") {
		t.Fatalf("SendInput() = false, want true")
	}
	msgs := m.GetMessages(sub.id)
	if len(msgs) != 1 || msgs[0].Content != "please continue" {
		t.Fatalf("messages=%+v, want queued user input", msgs)
	}

	m.Complete(sub.id, "review task", true, "")
	waited, ok, err := registry.Wait(context.Background(), sub.id)
	if err != nil || !ok {
		t.Fatalf("Wait()=(%+v,%v,%v), want completed snapshot", waited, ok, err)
	}
	if waited.Status != AgentStatusCompleted || waited.Task != "review task" {
		t.Fatalf("Wait()=%+v, want completed review task", waited)
	}

	sub2 := m.CreateContext(SubAgentTypeTester, context.Background(), nil)
	if !registry.Close(sub2.id) {
		t.Fatalf("Close() = false, want true")
	}
	if _, ok := m.GetContext(sub2.id); ok {
		t.Fatalf("Close() should remove context %q", sub2.id)
	}
}

func TestSubAgentAllowedTools_ReadOnlyAgentsIncludeSafeMetaTools(t *testing.T) {
	for _, agentType := range []SubAgentType{
		SubAgentTypeExplore,
		SubAgentTypeVerification,
		SubAgentTypeReviewer,
		SubAgentTypeSecurity,
		SubAgentTypeArchitect,
	} {
		allowed := buildAllowedToolsMap(agentType.AllowedTools())
		for _, want := range []string{
			tools.ToolRead,
			tools.ToolSearch,
			tools.ToolTodoRead,
			"projectstructure",
		} {
			if !allowed[want] {
				t.Fatalf("agent %s missing allowed tool %q in %#v", agentType, want, allowed)
			}
		}
	}
}
