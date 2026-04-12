package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/internal/config"
	pluginpkg "github.com/dreamSailing/vb-coding/internal/pkg/plugins"
	"github.com/dreamSailing/vb-coding/pkg/protocol"
)

type coreTestPlugin struct {
	name string
	desc string
}

func (p *coreTestPlugin) Name() string { return p.name }

func (p *coreTestPlugin) Description() string { return p.desc }

func (p *coreTestPlugin) Execute(context.Context, map[string]any) (any, error) { return "ok", nil }

func TestToRuntimeMode(t *testing.T) {
	if got := toRuntimeMode("手动确认"); got != "default" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("default"); got != "default" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("计划优先"); got != "plan" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("acceptEdits"); got != "acceptEdits" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("自动无人值守"); got != "auto" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("dontAsk"); got != "dontAsk" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("bypassPermissions"); got != "bypassPermissions" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := toRuntimeMode("unknown"); got != "default" {
		t.Fatalf("unexpected mode: %s", got)
	}
}

func TestFromRuntimeMode(t *testing.T) {
	if got := fromRuntimeMode("default"); got != "手动确认" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("acceptEdits"); got != "接受编辑" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("plan"); got != "计划优先" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("auto"); got != "自动无人值守" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("dontAsk"); got != "拒绝询问" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("bypassPermissions"); got != "绕过审批" {
		t.Fatalf("unexpected mode: %s", got)
	}
	if got := fromRuntimeMode("unknown"); got != "手动确认" {
		t.Fatalf("unexpected mode: %s", got)
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
	if len(items) != 1 {
		t.Fatalf("len(ListSkills())=%d, want 1", len(items))
	}
	if items[0].Name != "review" {
		t.Fatalf("skill name=%q, want review", items[0].Name)
	}
	if !items[0].Enabled {
		t.Fatal("skill should be enabled by default")
	}
	if items[0].Source != "project" {
		t.Fatalf("skill source=%q, want project", items[0].Source)
	}
	if items[0].BaseDir != skillDir {
		t.Fatalf("skill base dir=%q, want %q", items[0].BaseDir, skillDir)
	}
	if !items[0].UserInvocableDefined || !items[0].UserInvocable {
		t.Fatalf("user-invocable flags=%+v, want defined true", items[0])
	}
	if len(items[0].AllowedTools) != 2 || items[0].AllowedTools[0] != "read" || items[0].AllowedTools[1] != "grep" {
		t.Fatalf("allowed tools=%v, want [read grep]", items[0].AllowedTools)
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
	if len(items) != 1 || items[0].Enabled {
		t.Fatalf("ListSkills()=%+v, want disabled skill", items)
	}

	cfg, _ := config.Load()
	if !config.IsSkillDisabled(&cfg, "review") {
		t.Fatal("disabled skill should be persisted in config")
	}

	if err := rt.SetSkillEnabled("review", true); err != nil {
		t.Fatalf("SetSkillEnabled(true) error = %v", err)
	}
	items = rt.ListSkills()
	if len(items) != 1 || !items[0].Enabled {
		t.Fatalf("ListSkills()=%+v, want enabled skill", items)
	}
	cfg, _ = config.Load()
	if config.IsSkillDisabled(&cfg, "review") {
		t.Fatal("skill should be removed from disabled config after enabling")
	}
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
	if snap.ExecutionMode != "default" {
		t.Fatalf("ExecutionMode=%q, want default", snap.ExecutionMode)
	}
	if snap.HasPendingDiff {
		t.Fatal("HasPendingDiff should default to false")
	}
}
