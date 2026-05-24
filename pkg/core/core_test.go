package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/internal/config"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/pkg/protocol"
)

type coreTestPlugin struct {
	name string
	desc string
}

func (p *coreTestPlugin) Name() string { return p.name }

func (p *coreTestPlugin) Description() string { return p.desc }

func (p *coreTestPlugin) Execute(context.Context, map[string]any) (any, error) { return "ok", nil }

func TestToRuntimeMode(t *testing.T) {
	if got := toRuntimeMode("计划优先"); got != "plan" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("auto"); got != "auto" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("unknown"); got != "auto" {
		t.Fatalf("unexpected mode: %s", got)
	}
}

func TestFromRuntimeMode(t *testing.T) {
	if got := fromRuntimeMode("plan"); got != "plan" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("auto"); got != "auto" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("unknown"); got != "auto" {
		t.Fatalf("unexpected mode: %s", got)
	}
}

func TestRuntimePrepareStartupContextPinsWorkspaceDocs(t *testing.T) {
	workspace := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.WriteFile(filepath.Join(workspace, "EOS.md"), []byte("workspace eos rules"), 0o644); err != nil {
		t.Fatalf("WriteFile(EOS.md) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".eos"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.eos) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".eos", "Rules.md"), []byte("project rules"), 0o644); err != nil {
		t.Fatalf("WriteFile(project rules) error = %v", err)
	}

	rt := NewRuntime()
	defer rt.Close()
	rt.PrepareStartupContext(context.Background(), workspace)

	preview := strings.Join(rt.ContextPreview(), "\n")
	if !strings.Contains(preview, "workspace eos rules") {
		t.Fatalf("preview missing EOS.md content: %q", preview)
	}
	if !strings.Contains(preview, "project rules") {
		t.Fatalf("preview missing Rules.md content: %q", preview)
	}
}

func TestRuntimeStateChangesNotifySubscribers(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	events, unsubscribe := rt.SubscribeStateChanges(1)
	rt.SetExecutionMode("plan")

	select {
	case event := <-events:
		if event.Topic != StateTopicSettings {
			t.Fatalf("event.Topic=%q, want %q", event.Topic, StateTopicSettings)
		}
		if event.Source != "execution_mode" {
			t.Fatalf("event.Source=%q, want execution_mode", event.Source)
		}
		if event.At.IsZero() {
			t.Fatal("event.At should be set")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for state change event")
	}

	unsubscribe()
	if _, ok := <-events; ok {
		t.Fatal("expected subscription channel to close after unsubscribe")
	}
}

func TestRuntimeUsageSummaryEmpty(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	summary := rt.UsageSummary()
	if summary.Rounds != 0 {
		t.Fatalf("summary.Rounds=%d, want 0", summary.Rounds)
	}
	if summary.InputTokens != nil || summary.ReplyTokens != nil || summary.TotalTokens != nil || summary.CostUSD != nil {
		t.Fatalf("expected empty optional usage fields, got %#v", summary)
	}
	if got := rt.CostSummary(); got != "暂无模型调用" {
		t.Fatalf("CostSummary()=%q, want 暂无模型调用", got)
	}
}

func TestRuntimeRulesSnapshotAndScopedSave(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	if err := rt.SaveRulesScoped("project", "project rules"); err != nil {
		t.Fatalf("SaveRulesScoped(project) error = %v", err)
	}
	if err := rt.SaveRulesScoped("global", "global rules"); err != nil {
		t.Fatalf("SaveRulesScoped(global) error = %v", err)
	}
	snapshot := rt.RulesSnapshot()
	project, ok := findCoreRuleDocument(snapshot.Documents, "project")
	if !ok || !project.Exists || !strings.Contains(project.Content, "project rules") ||
		filepath.Clean(project.Path) != filepath.Join(workspace, ".eos", "Rules.md") {
		t.Fatalf("project rules doc=%+v ok=%v, want workspace project rules", project, ok)
	}
	global, ok := findCoreRuleDocument(snapshot.Documents, "global")
	home, _ := os.UserHomeDir()
	if !ok || !global.Exists || !strings.Contains(global.Content, "global rules") ||
		filepath.Clean(global.Path) != filepath.Join(home, ".eos", "Rules.md") {
		t.Fatalf("global rules doc=%+v ok=%v, want home global rules", global, ok)
	}
}

func TestRuntimeMemorySnapshotSaveAndRebuild(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	if err := rt.SaveMemory("project", "# Project\n\n- project memory"); err != nil {
		t.Fatalf("SaveMemory(project) error = %v", err)
	}
	if err := rt.SaveMemory("global", "# Global\n\n- global memory"); err != nil {
		t.Fatalf("SaveMemory(global) error = %v", err)
	}
	if err := rt.SaveMemory("session", "session memory"); err != nil {
		t.Fatalf("SaveMemory(session) error = %v", err)
	}
	if err := rt.RebuildMemoryIndex(); err != nil {
		t.Fatalf("RebuildMemoryIndex() error = %v", err)
	}

	snapshot := rt.MemorySnapshot()
	project, ok := findCoreMemoryDocument(snapshot.Documents, "project")
	if !ok || !project.Exists || !strings.Contains(project.Content, "project memory") ||
		filepath.Clean(project.Path) != filepath.Join(workspace, ".eos", "memory", "project.md") {
		t.Fatalf("project memory doc=%+v ok=%v, want workspace project memory", project, ok)
	}
	global, ok := findCoreMemoryDocument(snapshot.Documents, "global")
	if !ok || !global.Exists || !strings.Contains(global.Content, "global memory") {
		t.Fatalf("global memory doc=%+v ok=%v, want global memory", global, ok)
	}
	sessionDoc, ok := findCoreMemoryDocument(snapshot.Documents, "session")
	if !ok || !sessionDoc.Exists || !strings.Contains(sessionDoc.Content, "session memory") {
		t.Fatalf("session memory doc=%+v ok=%v, want session memory", sessionDoc, ok)
	}
	index, ok := findCoreMemoryDocument(snapshot.Documents, "index")
	if !ok || !index.Exists || !strings.Contains(index.Content, "project memory") {
		t.Fatalf("index memory doc=%+v ok=%v, want rebuilt index", index, ok)
	}
}

func TestRuntimeUsageSummaryTracksProviderUsageAndUnknownRounds(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	rt.core.AddTokenRecordWithModel(&schema.TokenUsage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		TotalTokens:      1200,
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: 250,
		},
	}, "deepseek-v4-pro")
	rt.core.AddTokenRecordWithModel(nil, "custom-no-usage")

	summary := rt.UsageSummary()
	if summary.Rounds != 2 {
		t.Fatalf("summary.Rounds=%d, want 2", summary.Rounds)
	}
	if summary.InputTokens == nil || *summary.InputTokens != 1000 {
		t.Fatalf("summary.InputTokens=%v, want 1000", summary.InputTokens)
	}
	if summary.ReplyTokens == nil || *summary.ReplyTokens != 200 {
		t.Fatalf("summary.ReplyTokens=%v, want 200", summary.ReplyTokens)
	}
	if summary.TotalTokens == nil || *summary.TotalTokens != 1200 {
		t.Fatalf("summary.TotalTokens=%v, want 1200", summary.TotalTokens)
	}
	if summary.CachedInputTokens == nil || *summary.CachedInputTokens != 250 {
		t.Fatalf("summary.CachedInputTokens=%v, want 250", summary.CachedInputTokens)
	}
	if summary.CostUSD == nil || *summary.CostUSD <= 0 {
		t.Fatalf("summary.CostUSD=%v, want > 0", summary.CostUSD)
	}
	if summary.UnknownUsageRounds != 1 || summary.UnknownCostRounds != 1 {
		t.Fatalf("unknown rounds got usage=%d cost=%d, want 1/1", summary.UnknownUsageRounds, summary.UnknownCostRounds)
	}

	items := rt.CostItems()
	if len(items) != 2 {
		t.Fatalf("len(CostItems())=%d, want 2", len(items))
	}
	var known, unknown *CostItem
	for i := range items {
		switch items[i].Model {
		case "deepseek-v4-pro":
			known = &items[i]
		case "custom-no-usage":
			unknown = &items[i]
		}
	}
	if known == nil || !known.UsageKnown || !known.CostKnown {
		t.Fatalf("known item missing usage/cost: %#v", known)
	}
	if known.InputTokens == nil || *known.InputTokens != 1000 || known.CachedInputTokens == nil || *known.CachedInputTokens != 250 {
		t.Fatalf("known item token fields not preserved: %#v", known)
	}
	if unknown == nil || unknown.UsageKnown || unknown.CostKnown || unknown.TotalTokens != nil || unknown.CostUSD != nil {
		t.Fatalf("unknown item should preserve nil usage/cost: %#v", unknown)
	}
	if got := rt.CostSummary(); !strings.Contains(got, "2 轮") || !strings.Contains(got, "usage 未知 1 轮") || !strings.Contains(got, "费用估算 $") || !strings.Contains(got, "费用未知 1 轮") {
		t.Fatalf("CostSummary()=%q, want readable token/cost summary", got)
	}
}

func TestFilterTrustedWorkspaces(t *testing.T) {
	target := filepath.Join("C:", "Users", "tester", "demo")
	trusted := []string{
		target,
		filepath.Join("C:", "Users", "tester", "keep"),
	}
	filtered, changed := filterTrustedWorkspaces(trusted, filepath.Join("C:", "Users", "tester", "demo", "."))
	if !changed {
		t.Fatal("expected target workspace to be removed from trusted list")
	}
	want := []string{filepath.Join("C:", "Users", "tester", "keep")}
	if !reflect.DeepEqual(filtered, want) {
		t.Fatalf("filtered=%v, want %v", filtered, want)
	}
}

func TestFilterTrustedWorkspacesNoMatch(t *testing.T) {
	trusted := []string{filepath.Join("C:", "Users", "tester", "keep")}
	filtered, changed := filterTrustedWorkspaces(trusted, filepath.Join("C:", "Users", "tester", "demo"))
	if changed {
		t.Fatal("expected no trusted workspace removal")
	}
	if !reflect.DeepEqual(filtered, trusted) {
		t.Fatalf("filtered=%v, want %v", filtered, trusted)
	}
}

func TestLegacyEventToProtocolMapsTextFinal(t *testing.T) {
	ts := time.Date(2026, 4, 2, 21, 0, 0, 0, time.UTC)
	ev := legacyEventToProtocol(Event{
		Type:      "TextFinal",
		RequestID: "req_01",
		Message:   "done",
	}, "sess_01", "thread_01", ts)

	if ev.EventType != protocol.EventTypeTextFinal {
		t.Fatalf("event_type=%q, want %q", ev.EventType, protocol.EventTypeTextFinal)
	}
	if ev.SessionID != "sess_01" {
		t.Fatalf("session_id=%q, want sess_01", ev.SessionID)
	}
	if ev.ThreadID != "thread_01" {
		t.Fatalf("thread_id=%q, want thread_01", ev.ThreadID)
	}
	if ev.RequestID != "req_01" {
		t.Fatalf("request_id=%q, want req_01", ev.RequestID)
	}
	if got := ev.Payload["text"]; got != "done" {
		t.Fatalf("payload[text]=%v, want done", got)
	}
	if !ev.Timestamp.Equal(ts) {
		t.Fatalf("timestamp=%s, want %s", ev.Timestamp, ts)
	}
}

func TestLegacyEventToProtocolMapsApproval(t *testing.T) {
	ev := legacyEventToProtocol(Event{
		Type:      "ConfirmRequired",
		RequestID: "approval_01",
		Message:   "继续执行危险操作？",
		Data: map[string]any{
			"risk_level": "high",
			"options":    []string{"allow_once", "deny"},
		},
	}, "sess_01", "thread_01", time.Unix(0, 0))

	if ev.EventType != protocol.EventTypeApprovalReq {
		t.Fatalf("event_type=%q, want %q", ev.EventType, protocol.EventTypeApprovalReq)
	}
	if got := ev.Payload["approval_id"]; got != "approval_01" {
		t.Fatalf("payload[approval_id]=%v, want approval_01", got)
	}
	if got := ev.Payload["message"]; got != "继续执行危险操作？" {
		t.Fatalf("payload[message]=%v, want question", got)
	}
	if got := ev.Payload["risk_level"]; got != "high" {
		t.Fatalf("payload[risk_level]=%v, want high", got)
	}
}

func TestLegacyEventToProtocolMapsInquiry(t *testing.T) {
	ev := legacyEventToProtocol(Event{
		Type:      "Inquiry",
		RequestID: "inq_01",
		Message:   "选择一个工作区",
		Data: map[string]any{
			"options": []string{"A", "B"},
		},
	}, "", "", time.Unix(0, 0))

	if ev.EventType != protocol.EventTypeInquiryReq {
		t.Fatalf("event_type=%q, want %q", ev.EventType, protocol.EventTypeInquiryReq)
	}
	if got := ev.Payload["inquiry_id"]; got != "inq_01" {
		t.Fatalf("payload[inquiry_id]=%v, want inq_01", got)
	}
	if got := ev.Payload["question"]; got != "选择一个工作区" {
		t.Fatalf("payload[question]=%v, want message", got)
	}
}

func TestBridgeEventToProtocolMapsApprovalRequired(t *testing.T) {
	ev, ok := bridgeEventToProtocol(bridge.Event{
		Type: "approval.required",
		RID:  "req-1",
		Data: map[string]any{
			"approval_id": "req-1",
			"message":     "需要确认",
		},
	}, "session-a", "thread-a", "", time.Unix(1710000000, 0))
	if !ok {
		t.Fatalf("bridgeEventToProtocol should map approval.required")
	}
	if ev.EventType != protocol.EventTypeApprovalReq {
		t.Fatalf("EventType=%q, want %q", ev.EventType, protocol.EventTypeApprovalReq)
	}
	if ev.RequestID != "req-1" {
		t.Fatalf("RequestID=%q, want req-1", ev.RequestID)
	}
	if got := ev.Payload["message"]; got != "需要确认" {
		t.Fatalf("payload message=%v, want 需要确认", got)
	}
}

func TestBridgeEventToProtocolUsesFallbackRequestID(t *testing.T) {
	ev, ok := bridgeEventToProtocol(bridge.Event{
		Type: "text.delta",
		Data: map[string]any{
			"text": "hello",
		},
	}, "session-a", "thread-a", "req-fallback", time.Unix(1710000001, 0))
	if !ok {
		t.Fatalf("bridgeEventToProtocol should map text.delta")
	}
	if ev.RequestID != "req-fallback" {
		t.Fatalf("RequestID=%q, want req-fallback", ev.RequestID)
	}
	if ev.CorrelationID != "req-fallback" {
		t.Fatalf("CorrelationID=%q, want req-fallback", ev.CorrelationID)
	}
}

func TestMapBridgeEventSupportsProtocolTextDelta(t *testing.T) {
	ev, ok := mapBridgeEvent(bridge.Event{
		Type: "text.delta",
		RID:  "req-2",
		Data: map[string]any{
			"text": "hello",
		},
	})
	if !ok {
		t.Fatalf("mapBridgeEvent should map text.delta")
	}
	if ev.Type != "TextDelta" {
		t.Fatalf("Type=%q, want TextDelta", ev.Type)
	}
	if ev.Message != "hello" {
		t.Fatalf("Message=%q, want hello", ev.Message)
	}
}

func TestBridgeEventToProtocolMapsAgentStarted(t *testing.T) {
	ev, ok := bridgeEventToProtocol(bridge.Event{
		Type: "agent.started",
		RID:  "agent-1",
		Data: map[string]any{
			"agent_id":   "agent-1",
			"agent_name": "planner",
			"task":       "设计实施计划",
			"message":    "设计实施计划",
		},
	}, "session-a", "thread-a", "req-1", time.Unix(1710000002, 0))
	if !ok {
		t.Fatalf("bridgeEventToProtocol should map agent.started")
	}
	if ev.EventType != protocol.EventTypeAgentStarted {
		t.Fatalf("EventType=%q, want %q", ev.EventType, protocol.EventTypeAgentStarted)
	}
	if got := ev.Payload["agent_name"]; got != "planner" {
		t.Fatalf("payload[agent_name]=%v, want planner", got)
	}
	if got := ev.Payload["task"]; got != "设计实施计划" {
		t.Fatalf("payload[task]=%v, want task", got)
	}
}

func TestBridgeEventToProtocolMapsAgentFailed(t *testing.T) {
	ev, ok := bridgeEventToProtocol(bridge.Event{
		Type: "agent.failed",
		RID:  "agent-2",
		Data: map[string]any{
			"agent_id":   "agent-2",
			"agent_name": "reviewer",
			"error":      "subagent crashed",
		},
	}, "session-a", "thread-a", "req-2", time.Unix(1710000003, 0))
	if !ok {
		t.Fatalf("bridgeEventToProtocol should map agent.failed")
	}
	if ev.EventType != protocol.EventTypeAgentFailed {
		t.Fatalf("EventType=%q, want %q", ev.EventType, protocol.EventTypeAgentFailed)
	}
	if got := ev.Payload["error"]; got != "subagent crashed" {
		t.Fatalf("payload[error]=%v, want subagent crashed", got)
	}
}

func TestBridgeEventToProtocolMapsAgentCancelled(t *testing.T) {
	ev, ok := bridgeEventToProtocol(bridge.Event{
		Type: "agent.cancelled",
		RID:  "agent-3",
		Data: map[string]any{
			"agent_id":   "agent-3",
			"agent_name": "tester",
			"reason":     "cancelled",
		},
	}, "session-a", "thread-a", "req-3", time.Unix(1710000004, 0))
	if !ok {
		t.Fatalf("bridgeEventToProtocol should map agent.cancelled")
	}
	if ev.EventType != protocol.EventTypeAgentCancelled {
		t.Fatalf("EventType=%q, want %q", ev.EventType, protocol.EventTypeAgentCancelled)
	}
	if got := ev.Payload["reason"]; got != "cancelled" {
		t.Fatalf("payload[reason]=%v, want cancelled", got)
	}
}

func TestRuntimeListsSkills(t *testing.T) {
	rt := NewRuntime()
	root := t.TempDir()
	skillDir := filepath.Join(root, "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	skillDoc := strings.Join([]string{
		"---",
		"name: review",
		"description: Review the current workspace",
		"argument-hint: target path",
		"allowed-tools: read,grep",
		"user-invocable: true",
		"---",
		"Review instructions",
	}, "\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillDoc), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader := rt.core.GetSkillsLoader()
	loader.SetSkillsDirs([]string{root})
	if err := loader.Scan(); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	items := rt.ListSkills()
	item, ok := findCoreSkillInfo(items, "review")
	if !ok {
		t.Fatalf("ListSkills()=%+v, want review skill", items)
	}
	if item.Name != "review" {
		t.Fatalf("skill name=%q, want review", item.Name)
	}
	if !item.Enabled {
		t.Fatal("skill should be enabled by default")
	}
	if item.Source != "project" {
		t.Fatalf("skill source=%q, want project", item.Source)
	}
	if item.BaseDir != skillDir {
		t.Fatalf("skill base dir=%q, want %q", item.BaseDir, skillDir)
	}
	if !item.UserInvocableDefined || !item.UserInvocable {
		t.Fatalf("user-invocable flags=%+v, want defined true", item)
	}
	if len(item.AllowedTools) != 2 || item.AllowedTools[0] != "read" || item.AllowedTools[1] != "grep" {
		t.Fatalf("allowed tools=%v, want [read grep]", item.AllowedTools)
	}
}

func TestRuntimeSetSkillEnabledPersistsState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	rt := NewRuntime()
	root := t.TempDir()
	skillDir := filepath.Join(root, "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	skillDoc := strings.Join([]string{
		"---",
		"name: review",
		"description: Review the current workspace",
		"---",
		"Review instructions",
	}, "\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillDoc), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loader := rt.core.GetSkillsLoader()
	loader.SetSkillsDirs([]string{root})
	if err := loader.Scan(); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if err := rt.SetSkillEnabled("review", false); err != nil {
		t.Fatalf("SetSkillEnabled(false) error = %v", err)
	}
	items := rt.ListSkills()
	item, ok := findCoreSkillInfo(items, "review")
	if !ok || item.Enabled {
		t.Fatalf("ListSkills()=%+v, want disabled review skill", items)
	}

	cfg, _ := config.Load()
	if !config.IsSkillDisabled(&cfg, "review") {
		t.Fatal("disabled skill should be persisted in config")
	}

	if err := rt.SetSkillEnabled("review", true); err != nil {
		t.Fatalf("SetSkillEnabled(true) error = %v", err)
	}
	items = rt.ListSkills()
	item, ok = findCoreSkillInfo(items, "review")
	if !ok || !item.Enabled {
		t.Fatalf("ListSkills()=%+v, want enabled review skill", items)
	}
	cfg, _ = config.Load()
	if config.IsSkillDisabled(&cfg, "review") {
		t.Fatal("skill should be removed from disabled config after enabling")
	}
}

func findCoreSkillInfo(items []SkillInfo, name string) (SkillInfo, bool) {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(name)) {
			return item, true
		}
	}
	return SkillInfo{}, false
}

func TestRuntimeListWorkspaceSessionsSeparatesWorkspaces(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()

	workspaceA := t.TempDir()
	workspaceB := t.TempDir()

	metaA, err := rt.CreateWorkspaceSession(workspaceA, "thread-a", []SessionMessage{{Role: "assistant", Type: "text", Content: "workspace-a"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(workspaceA) error = %v", err)
	}
	metaB, err := rt.CreateWorkspaceSession(workspaceB, "thread-b", []SessionMessage{{Role: "assistant", Type: "text", Content: "workspace-b"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(workspaceB) error = %v", err)
	}

	sessionsA, err := rt.ListWorkspaceSessions(workspaceA)
	if err != nil {
		t.Fatalf("ListWorkspaceSessions(workspaceA) error = %v", err)
	}
	sessionsB, err := rt.ListWorkspaceSessions(workspaceB)
	if err != nil {
		t.Fatalf("ListWorkspaceSessions(workspaceB) error = %v", err)
	}
	if len(sessionsA) != 1 || sessionsA[0].ID != metaA.ID {
		t.Fatalf("workspaceA sessions=%+v, want only %q", sessionsA, metaA.ID)
	}
	if len(sessionsB) != 1 || sessionsB[0].ID != metaB.ID {
		t.Fatalf("workspaceB sessions=%+v, want only %q", sessionsB, metaB.ID)
	}

	snapshot := rt.RuntimeSnapshot()
	foundA := false
	foundB := false
	for _, workspace := range snapshot.Workspaces {
		switch {
		case filepath.Clean(workspace.Path) == filepath.Clean(workspaceA):
			foundA = true
			if workspace.SessionCount != 1 {
				t.Fatalf("workspaceA sessionCount=%d, want 1", workspace.SessionCount)
			}
		case filepath.Clean(workspace.Path) == filepath.Clean(workspaceB):
			foundB = true
			if workspace.SessionCount != 1 {
				t.Fatalf("workspaceB sessionCount=%d, want 1", workspace.SessionCount)
			}
		}
	}
	if !foundA || !foundB {
		t.Fatalf("runtime snapshot missing workspaces: foundA=%v foundB=%v", foundA, foundB)
	}
}

func TestRuntimeWorkspaceSessionOpsStayScoped(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()

	workspaceA := t.TempDir()
	workspaceB := t.TempDir()

	metaA, err := rt.CreateWorkspaceSession(workspaceA, "thread-a", []SessionMessage{{Role: "assistant", Type: "text", Content: "workspace-a"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(workspaceA) error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspaceA); err != nil {
		t.Fatalf("SetForegroundWorkspace(workspaceA) error = %v", err)
	}

	metaB, err := rt.CreateWorkspaceSession(workspaceB, "thread-b", []SessionMessage{{Role: "assistant", Type: "text", Content: "workspace-b"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(workspaceB) error = %v", err)
	}

	currentA, err := rt.GetWorkspaceCurrentSession(workspaceA)
	if err != nil {
		t.Fatalf("GetWorkspaceCurrentSession(workspaceA) error = %v", err)
	}
	currentB, err := rt.GetWorkspaceCurrentSession(workspaceB)
	if err != nil {
		t.Fatalf("GetWorkspaceCurrentSession(workspaceB) error = %v", err)
	}
	if currentA != metaA.ID {
		t.Fatalf("currentA=%q, want %q", currentA, metaA.ID)
	}
	if currentB != metaB.ID {
		t.Fatalf("currentB=%q, want %q", currentB, metaB.ID)
	}

	if err := rt.DeleteWorkspaceSession(workspaceA, metaA.ID); err != nil {
		t.Fatalf("DeleteWorkspaceSession(workspaceA) error = %v", err)
	}
	sessionsA, err := rt.ListWorkspaceSessions(workspaceA)
	if err != nil {
		t.Fatalf("ListWorkspaceSessions(workspaceA) error = %v", err)
	}
	sessionsB, err := rt.ListWorkspaceSessions(workspaceB)
	if err != nil {
		t.Fatalf("ListWorkspaceSessions(workspaceB) error = %v", err)
	}
	if len(sessionsA) != 0 {
		t.Fatalf("workspaceA sessions=%d, want 0 after delete", len(sessionsA))
	}
	if len(sessionsB) != 1 || sessionsB[0].ID != metaB.ID {
		t.Fatalf("workspaceB sessions=%+v, want only %q", sessionsB, metaB.ID)
	}
}

func TestRuntimeWorkspaceReadsDoNotChangeActiveWorkspace(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()

	workspaceA := t.TempDir()
	workspaceB := t.TempDir()

	metaA, err := rt.CreateWorkspaceSession(workspaceA, "thread-a", []SessionMessage{
		{Role: "user", Type: "text", Content: "hello a"},
		{Role: "assistant", Type: "text", Content: "reply a"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(workspaceA) error = %v", err)
	}
	metaB, err := rt.CreateWorkspaceSession(workspaceB, "thread-b", []SessionMessage{
		{Role: "user", Type: "text", Content: "hello b"},
		{Role: "assistant", Type: "text", Content: "reply b"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(workspaceB) error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspaceA); err != nil {
		t.Fatalf("SetForegroundWorkspace(workspaceA) error = %v", err)
	}
	before := normalizeWorkspacePath(rt.core.GetActiveRoot())
	if before != normalizeWorkspacePath(workspaceA) {
		t.Fatalf("active root before reads = %q, want %q", before, normalizeWorkspacePath(workspaceA))
	}

	if _, err := rt.ListWorkspaceSessions(workspaceB); err != nil {
		t.Fatalf("ListWorkspaceSessions(workspaceB) error = %v", err)
	}
	if got := normalizeWorkspacePath(rt.core.GetActiveRoot()); got != before {
		t.Fatalf("active root after ListWorkspaceSessions = %q, want %q", got, before)
	}

	if currentID, err := rt.GetWorkspaceCurrentSession(workspaceB); err != nil {
		t.Fatalf("GetWorkspaceCurrentSession(workspaceB) error = %v", err)
	} else if currentID != metaB.ID {
		t.Fatalf("GetWorkspaceCurrentSession(workspaceB)=%q, want %q", currentID, metaB.ID)
	}
	if got := normalizeWorkspacePath(rt.core.GetActiveRoot()); got != before {
		t.Fatalf("active root after GetWorkspaceCurrentSession = %q, want %q", got, before)
	}

	if messages, err := rt.LoadWorkspaceSessionMessages(workspaceB, metaB.ID); err != nil {
		t.Fatalf("LoadWorkspaceSessionMessages(workspaceB) error = %v", err)
	} else if len(messages) != 2 {
		t.Fatalf("len(LoadWorkspaceSessionMessages(workspaceB))=%d, want 2", len(messages))
	}
	if got := normalizeWorkspacePath(rt.core.GetActiveRoot()); got != before {
		t.Fatalf("active root after LoadWorkspaceSessionMessages = %q, want %q", got, before)
	}

	if resolved, err := rt.ResolveSessionWorkspace(metaB.ID); err != nil {
		t.Fatalf("ResolveSessionWorkspace(metaB) error = %v", err)
	} else if filepath.Clean(resolved) != filepath.Clean(workspaceB) {
		t.Fatalf("ResolveSessionWorkspace(metaB)=%q, want %q", resolved, workspaceB)
	}
	if got := normalizeWorkspacePath(rt.core.GetActiveRoot()); got != before {
		t.Fatalf("active root after ResolveSessionWorkspace = %q, want %q", got, before)
	}

	snapshot := rt.RuntimeSnapshot()
	if got := normalizeWorkspacePath(rt.core.GetActiveRoot()); got != before {
		t.Fatalf("active root after RuntimeSnapshot = %q, want %q", got, before)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != metaA.ID {
		t.Fatalf("snapshot.CurrentSession=%+v, want %q", snapshot.CurrentSession, metaA.ID)
	}
}

func TestRuntimeSnapshotLoadsOnlyForegroundMessages(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()

	workspaceA := t.TempDir()
	workspaceB := t.TempDir()
	messagesA := []SessionMessage{
		{Role: "user", Type: "text", Content: "hello a"},
		{Role: "assistant", Type: "text", Content: "reply a"},
	}
	messagesB := []SessionMessage{
		{Role: "user", Type: "text", Content: "hello b"},
		{Role: "assistant", Type: "text", Content: "reply b-1"},
		{Role: "assistant", Type: "text", Content: "reply b-2"},
	}

	metaA, err := rt.CreateWorkspaceSession(workspaceA, "thread-a", messagesA)
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(workspaceA) error = %v", err)
	}
	metaB, err := rt.CreateWorkspaceSession(workspaceB, "thread-b", messagesB)
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(workspaceB) error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspaceA); err != nil {
		t.Fatalf("SetForegroundWorkspace(workspaceA) error = %v", err)
	}

	snapshot := rt.RuntimeSnapshot()
	if got := filepath.Clean(snapshot.ForegroundWorkspace); got != filepath.Clean(workspaceA) {
		t.Fatalf("snapshot.ForegroundWorkspace=%q, want %q", got, workspaceA)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != metaA.ID {
		t.Fatalf("snapshot.CurrentSession=%+v, want %q", snapshot.CurrentSession, metaA.ID)
	}
	if len(snapshot.Messages) != len(messagesA) {
		t.Fatalf("len(snapshot.Messages)=%d, want %d", len(snapshot.Messages), len(messagesA))
	}

	var sessionA *SessionSnapshot
	var sessionB *SessionSnapshot
	for index := range snapshot.Sessions {
		item := &snapshot.Sessions[index]
		switch item.ID {
		case metaA.ID:
			sessionA = item
		case metaB.ID:
			sessionB = item
		}
	}
	if sessionA == nil || sessionB == nil {
		t.Fatalf("snapshot sessions missing foreground/background entries: %+v", snapshot.Sessions)
	}
	if !sessionA.Active {
		t.Fatal("foreground session should be active in snapshot")
	}
	if sessionA.MessageCount != len(messagesA) {
		t.Fatalf("foreground MessageCount=%d, want %d", sessionA.MessageCount, len(messagesA))
	}
	if sessionB.Active {
		t.Fatal("background session should not be active in snapshot")
	}
	if sessionB.MessageCount != metaB.Rounds {
		t.Fatalf("background MessageCount=%d, want metadata rounds %d", sessionB.MessageCount, metaB.Rounds)
	}
}

func TestRuntimeWorkspaceSessionMessagesPreserveMetadata(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	workspace := t.TempDir()
	metadata := map[string]any{
		"eos_gui": map[string]any{
			"state":          "failed",
			"updatedAt":      "2026-04-24T20:00:00+08:00",
			"runtimeSummary": "失败 · 最近一步：请求超时",
			"runtimeEvents": []any{
				map[string]any{"title": "请求超时", "status": "failed", "durationMs": int64(30000)},
			},
		},
	}

	meta, err := rt.CreateWorkspaceSession(workspace, "thread-runtime", []SessionMessage{
		{Role: "assistant", Type: "text", Content: "请求超时", Metadata: metadata},
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}
	loaded, err := rt.LoadWorkspaceSessionMessages(workspace, meta.ID)
	if err != nil {
		t.Fatalf("LoadWorkspaceSessionMessages() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("len(loaded)=%d, want 1", len(loaded))
	}
	gui, ok := loaded[0].Metadata["eos_gui"].(map[string]any)
	if !ok {
		t.Fatalf("metadata=%#v, want eos_gui map", loaded[0].Metadata)
	}
	if got := gui["runtimeSummary"]; got != "失败 · 最近一步：请求超时" {
		t.Fatalf("runtimeSummary=%#v, want persisted summary", got)
	}

	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}
	snapshot := rt.RuntimeSnapshot()
	if len(snapshot.Messages) != 1 {
		t.Fatalf("len(snapshot.Messages)=%d, want 1", len(snapshot.Messages))
	}
	snapshotGUI := snapshot.Messages[0].Metadata["eos_gui"].(map[string]any)
	if got := snapshotGUI["runtimeSummary"]; got != "失败 · 最近一步：请求超时" {
		t.Fatalf("snapshot runtimeSummary=%#v, want persisted summary", got)
	}

	snapshotGUI["runtimeSummary"] = "mutated"
	reloaded, err := rt.LoadWorkspaceSessionMessages(workspace, meta.ID)
	if err != nil {
		t.Fatalf("LoadWorkspaceSessionMessages() second error = %v", err)
	}
	reloadedGUI := reloaded[0].Metadata["eos_gui"].(map[string]any)
	if got := reloadedGUI["runtimeSummary"]; got != "失败 · 最近一步：请求超时" {
		t.Fatalf("metadata was aliased after snapshot, runtimeSummary=%#v", got)
	}
}

func configureCoreWorkspaceTestEnv(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func findCoreRuleDocument(items []RuleDocument, scope string) (RuleDocument, bool) {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Scope), scope) {
			return item, true
		}
	}
	return RuleDocument{}, false
}

func findCoreMemoryDocument(items []MemoryDocument, scope string) (MemoryDocument, bool) {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Scope), scope) {
			return item, true
		}
	}
	return MemoryDocument{}, false
}

func TestRuntimeListsPluginSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	rt := NewRuntime()
	workspace := t.TempDir()
	pluginSkillDir := filepath.Join(workspace, ".claude", "plugins", "formatter", "skills", "review")
	if err := os.MkdirAll(filepath.Join(workspace, ".claude", "plugins", "formatter", ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(pluginSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".claude", "plugins", "formatter", ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	skillDoc := strings.Join([]string{
		"---",
		"description: Review files from plugin",
		"user-invocable: true",
		"---",
		"Plugin review instructions",
	}, "\n")
	if err := os.WriteFile(filepath.Join(pluginSkillDir, "SKILL.md"), []byte(skillDoc), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	if err := rt.AddWorkspace(workspace); err != nil {
		t.Fatalf("AddWorkspace error: %v", err)
	}
	if err := rt.UseWorkspace(workspace); err != nil {
		t.Fatalf("UseWorkspace error: %v", err)
	}

	items := rt.ListSkills()
	if len(items) != 1 {
		t.Fatalf("len(ListSkills())=%d, want 1", len(items))
	}
	if items[0].Name != "formatter:review" {
		t.Fatalf("skill name=%q, want formatter:review", items[0].Name)
	}
	if items[0].Source != "plugin:formatter" {
		t.Fatalf("skill source=%q, want plugin:formatter", items[0].Source)
	}
	if items[0].Location != "project" {
		t.Fatalf("skill location=%q, want project", items[0].Location)
	}
}

func TestRuntimeListsPlugins(t *testing.T) {
	pluginpkg.DefaultRegistry().Reset()
	t.Cleanup(func() { pluginpkg.DefaultRegistry().Reset() })
	pluginpkg.DefaultRegistry().Register(&coreTestPlugin{name: "echo_plugin", desc: "Echo input"})

	rt := NewRuntime()
	items := rt.ListPlugins()
	if len(items) != 1 {
		t.Fatalf("len(ListPlugins())=%d, want 1", len(items))
	}
	if items[0].Name != "echo_plugin" {
		t.Fatalf("plugin name=%q, want echo_plugin", items[0].Name)
	}
	if items[0].Description != "Echo input" {
		t.Fatalf("plugin description=%q, want Echo input", items[0].Description)
	}
	if items[0].Source != "registry" {
		t.Fatalf("plugin source=%q, want registry", items[0].Source)
	}
	if !items[0].Enabled {
		t.Fatal("plugin should be enabled by default")
	}
}

func TestRuntimeSetPluginEnabledPersistsState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginpkg.DefaultRegistry().Reset()
	t.Cleanup(func() { pluginpkg.DefaultRegistry().Reset() })
	pluginpkg.DefaultRegistry().Register(&pluginpkg.ExternalToolPlugin{
		ToolName:        "git_summary",
		ToolDescription: "Summarize git status",
		Command:         "git",
		Args:            []string{"status", "--short"},
	})

	rt := NewRuntime()
	items := rt.ListPlugins()
	if len(items) != 1 {
		t.Fatalf("len(ListPlugins())=%d, want 1", len(items))
	}
	if items[0].Source != "external" {
		t.Fatalf("plugin source=%q, want external", items[0].Source)
	}
	if items[0].Command != "git status --short" {
		t.Fatalf("plugin command=%q, want %q", items[0].Command, "git status --short")
	}

	if err := rt.SetPluginEnabled("git_summary", false); err != nil {
		t.Fatalf("SetPluginEnabled(false) error = %v", err)
	}
	items = rt.ListPlugins()
	if len(items) != 1 || items[0].Enabled {
		t.Fatalf("ListPlugins()=%+v, want disabled plugin", items)
	}
	cfg, _ := config.Load()
	if enabled, ok := config.PluginEnabled(&cfg, "git_summary"); !ok || enabled {
		t.Fatalf("config.PluginEnabled()=(%v,%v), want (false,true)", enabled, ok)
	}

	if err := rt.SetPluginEnabled("git_summary", true); err != nil {
		t.Fatalf("SetPluginEnabled(true) error = %v", err)
	}
	items = rt.ListPlugins()
	if len(items) != 1 || !items[0].Enabled {
		t.Fatalf("ListPlugins()=%+v, want enabled plugin", items)
	}
	cfg, _ = config.Load()
	if _, ok := config.PluginEnabled(&cfg, "git_summary"); ok {
		t.Fatal("enabled plugin should not keep explicit disabled override in config")
	}
}

func TestRuntimeListsManifestPlugins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginpkg.DefaultRegistry().Reset()
	t.Cleanup(func() { pluginpkg.DefaultRegistry().Reset() })

	workspace := t.TempDir()
	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format project files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	rt := NewRuntime()
	if err := rt.AddWorkspace(workspace); err != nil {
		t.Fatalf("AddWorkspace error: %v", err)
	}
	if err := rt.UseWorkspace(workspace); err != nil {
		t.Fatalf("UseWorkspace error: %v", err)
	}

	items := rt.ListPlugins()
	if len(items) != 1 {
		t.Fatalf("len(ListPlugins())=%d, want 1", len(items))
	}
	if items[0].Name != "formatter" {
		t.Fatalf("plugin name=%q, want formatter", items[0].Name)
	}
	if items[0].Source != "directory:project" {
		t.Fatalf("plugin source=%q, want directory:project", items[0].Source)
	}
	if !items[0].Enabled {
		t.Fatal("manifest plugin should be enabled by default")
	}
}

func TestRuntimeSetManifestPluginEnabledPersistsState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginpkg.DefaultRegistry().Reset()
	t.Cleanup(func() { pluginpkg.DefaultRegistry().Reset() })

	workspace := t.TempDir()
	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format project files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	rt := NewRuntime()
	if err := rt.AddWorkspace(workspace); err != nil {
		t.Fatalf("AddWorkspace error: %v", err)
	}
	if err := rt.UseWorkspace(workspace); err != nil {
		t.Fatalf("UseWorkspace error: %v", err)
	}
	if err := rt.SetPluginEnabled("formatter", false); err != nil {
		t.Fatalf("SetPluginEnabled(false) error = %v", err)
	}

	items := rt.ListPlugins()
	if len(items) != 1 || items[0].Enabled {
		t.Fatalf("ListPlugins()=%+v, want disabled manifest plugin", items)
	}
	cfg, _ := config.Load()
	if enabled, ok := config.PluginEnabled(&cfg, "formatter"); !ok || enabled {
		t.Fatalf("config.PluginEnabled()=(%v,%v), want (false,true)", enabled, ok)
	}
}

func TestRuntimePermissionSnapshotDefaults(t *testing.T) {
	rt := NewRuntime()
	snap := rt.PermissionSnapshot()
	if snap.ExecutionMode != "auto" {
		t.Fatalf("ExecutionMode=%q, want auto", snap.ExecutionMode)
	}
	if snap.HasPendingDiff {
		t.Fatal("HasPendingDiff should default to false")
	}
}
