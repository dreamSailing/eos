//go:build legacy

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sharedruntime "github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/pkg/agentcore"
	"github.com/dreamSailing/eos/pkg/coreapi"
	coreapijsonrpc "github.com/dreamSailing/eos/pkg/coreapi/jsonrpc"
	"github.com/dreamSailing/eos/pkg/protocol"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
	"github.com/dreamSailing/eos/pkg/sandbox"
)

func TestLegacyEngineListsCurrentWorkspaceSessions(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "thread-a", []SessionMessage{{Role: "assistant", Type: "text", Content: "hello"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	engine := NewLegacyEngine(rt)
	items, err := engine.Sessions().List(context.Background(), coreapi.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("Sessions().List() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != meta.ID {
		t.Fatalf("sessions=%+v, want %q", items, meta.ID)
	}
	if filepath.Clean(items[0].WorkspaceRoot) != filepath.Clean(workspace) {
		t.Fatalf("WorkspaceRoot=%q, want %q", items[0].WorkspaceRoot, workspace)
	}
}

func TestLegacyEngineGitServiceUsesForegroundWorkspace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	runGitTestCommand(t, workspace, "init")
	runGitTestCommand(t, workspace, "config", "user.email", "test@example.com")
	runGitTestCommand(t, workspace, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	runGitTestCommand(t, workspace, "add", "a.txt")
	runGitTestCommand(t, workspace, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	gitSvc := NewLegacyEngine(rt).Git()
	changes, err := gitSvc.Status(context.Background(), coreapi.GitStatusRequest{})
	if err != nil {
		t.Fatalf("Git().Status() error = %v", err)
	}
	if !hasGitChange(changes, "a.txt", "modified") {
		t.Fatalf("changes=%+v, want modified a.txt", changes)
	}
	diff, err := gitSvc.Diff(context.Background(), coreapi.GitDiffRequest{Path: "a.txt"})
	if err != nil {
		t.Fatalf("Git().Diff() error = %v", err)
	}
	if !strings.Contains(diff.Text, "+world") {
		t.Fatalf("diff=%q, want added world line", diff.Text)
	}
	branches, err := gitSvc.Branches(context.Background(), coreapi.GitBranchesRequest{})
	if err != nil {
		t.Fatalf("Git().Branches() error = %v", err)
	}
	if branches.Current == "" || len(branches.Branches) == 0 {
		t.Fatalf("branches=%+v, want current branch", branches)
	}
	log, err := gitSvc.Log(context.Background(), coreapi.GitLogRequest{Limit: 5, Oneline: true})
	if err != nil {
		t.Fatalf("Git().Log() error = %v", err)
	}
	if !strings.Contains(log.Text, "init") {
		t.Fatalf("log=%q, want init commit", log.Text)
	}
	show, err := gitSvc.Show(context.Background(), coreapi.GitShowRequest{Revision: "HEAD", Path: "a.txt"})
	if err != nil {
		t.Fatalf("Git().Show() error = %v", err)
	}
	if show.Revision != "HEAD" || !strings.Contains(show.Text, "a.txt") {
		t.Fatalf("show=%+v, want HEAD a.txt", show)
	}
}

func runGitTestCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func hasGitChange(changes []coreapi.GitChange, path, state string) bool {
	for _, change := range changes {
		if change.Path == path && change.State == state {
			return true
		}
	}
	return false
}

func TestLegacyEngineListsRequestedWorkspaceSessions(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspaceA := t.TempDir()
	workspaceB := t.TempDir()
	metaA, err := rt.CreateWorkspaceSession(workspaceA, "thread-a", []SessionMessage{{Role: "assistant", Type: "text", Content: "a"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(A) error = %v", err)
	}
	metaB, err := rt.CreateWorkspaceSession(workspaceB, "thread-b", []SessionMessage{{Role: "assistant", Type: "text", Content: "b"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(B) error = %v", err)
	}

	engine := NewLegacyEngine(rt)
	items, err := engine.Sessions().List(context.Background(), coreapi.ListSessionsRequest{WorkspaceRoot: workspaceB})
	if err != nil {
		t.Fatalf("Sessions().List(workspaceB) error = %v", err)
	}
	if len(items) != 1 || items[0].ID != metaB.ID {
		t.Fatalf("workspaceB sessions=%+v, want %q and not %q", items, metaB.ID, metaA.ID)
	}
	if filepath.Clean(items[0].WorkspaceRoot) != filepath.Clean(workspaceB) {
		t.Fatalf("WorkspaceRoot=%q, want %q", items[0].WorkspaceRoot, workspaceB)
	}
}

func TestLegacyEngineStateSnapshotMapsRuntimeSnapshot(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "thread-a", []SessionMessage{
		{Role: "user", Type: "text", Content: "hello"},
		{Role: "assistant", Type: "text", Content: "world"},
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	snapshot, err := NewLegacyEngine(rt).State().Snapshot(context.Background())
	if err != nil {
		t.Fatalf("State().Snapshot() error = %v", err)
	}
	if filepath.Clean(snapshot.ForegroundWorkspace) != filepath.Clean(workspace) {
		t.Fatalf("ForegroundWorkspace=%q, want %q", snapshot.ForegroundWorkspace, workspace)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != meta.ID {
		t.Fatalf("CurrentSession=%+v, want %q", snapshot.CurrentSession, meta.ID)
	}
	if len(snapshot.Messages) != 2 {
		t.Fatalf("len(Messages)=%d, want 2", len(snapshot.Messages))
	}
	if len(snapshot.Workspaces) == 0 || len(snapshot.Sessions) == 0 {
		t.Fatalf("snapshot missing workspaces/sessions: %+v", snapshot)
	}
}

func TestLegacyEngineStateSnapshotIncludesRuntimeAgentRegistry(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	mgr := sharedruntime.NewSubAgentManager()
	registryID := sharedruntime.DefaultAgentRegistry().RegisterManager(mgr)
	defer sharedruntime.DefaultAgentRegistry().UnregisterManager(registryID)
	sub := mgr.CreateContext(sharedruntime.SubAgentTypeReviewer, context.Background(), nil)
	if sub == nil {
		t.Fatalf("CreateContext() = nil")
	}
	if err := mgr.MarkRunning(sub.ID(), "review live agents", nil); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}

	snapshot, err := NewLegacyEngine(rt).State().Snapshot(context.Background())
	if err != nil {
		t.Fatalf("State().Snapshot() error = %v", err)
	}
	var got *coreapi.Agent
	for i := range snapshot.Agents {
		if snapshot.Agents[i].ID == sub.ID() {
			got = &snapshot.Agents[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("State().Snapshot().Agents=%+v, want runtime subagent %q", snapshot.Agents, sub.ID())
	}
	if got.RoleID != "reviewer" || got.Task != "review live agents" || got.Status != string(agentcore.AgentRunning) {
		t.Fatalf("runtime agent snapshot=%+v, want reviewer running task", *got)
	}
}

func TestLegacyEngineAgentServiceControlsRuntimeRegistryAgents(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	engine := NewLegacyEngine(rt)
	ctx := context.Background()

	mgr := sharedruntime.NewSubAgentManager()
	registryID := sharedruntime.DefaultAgentRegistry().RegisterManager(mgr)
	defer sharedruntime.DefaultAgentRegistry().UnregisterManager(registryID)
	sub := mgr.CreateContext(sharedruntime.SubAgentTypeReviewer, ctx, nil)
	if sub == nil {
		t.Fatalf("CreateContext() = nil")
	}

	if err := engine.Agents().SendInput(ctx, coreapi.AgentInput{AgentID: sub.ID(), Input: "queued input"}); err != nil {
		t.Fatalf("Agents().SendInput(runtime agent) error = %v", err)
	}
	msgs := mgr.GetMessages(sub.ID())
	if len(msgs) != 1 || msgs[0].Content != "queued input" {
		t.Fatalf("runtime messages=%+v, want queued input", msgs)
	}

	items, err := engine.Agents().List(ctx, coreapi.ListAgentsRequest{})
	if err != nil {
		t.Fatalf("Agents().List() error = %v", err)
	}
	if _, ok := agentByID(items, sub.ID()); !ok {
		t.Fatalf("Agents().List()=%+v, want runtime agent %q", items, sub.ID())
	}

	mgr.Complete(sub.ID(), "runtime complete", true, "")
	waited, err := engine.Agents().Wait(ctx, coreapi.AgentRef{AgentID: sub.ID()})
	if err != nil {
		t.Fatalf("Agents().Wait(runtime agent) error = %v", err)
	}
	if waited.ID != sub.ID() || waited.Status != string(agentcore.AgentCompleted) || waited.Task != "runtime complete" {
		t.Fatalf("Agents().Wait(runtime agent)=%+v, want completed runtime agent", waited)
	}

	closeSub := mgr.CreateContext(sharedruntime.SubAgentTypeTester, ctx, nil)
	if err := engine.Agents().Close(ctx, coreapi.AgentRef{AgentID: closeSub.ID()}); err != nil {
		t.Fatalf("Agents().Close(runtime agent) error = %v", err)
	}
	if _, ok := mgr.GetContext(closeSub.ID()); ok {
		t.Fatalf("Agents().Close(runtime agent) should remove %q", closeSub.ID())
	}
}

func TestLegacyEngineManagesWorkspaces(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspaceA := t.TempDir()
	workspaceB := t.TempDir()

	workspaces := NewLegacyEngine(rt).Workspaces()
	if path, err := workspaces.Default(context.Background()); err != nil {
		t.Fatalf("Workspaces().Default() error = %v", err)
	} else if strings.TrimSpace(path) == "" {
		t.Fatal("default workspace path is empty")
	}
	if err := workspaces.Remember(context.Background(), coreapi.RememberWorkspaceRequest{Path: workspaceA, Foreground: true}); err != nil {
		t.Fatalf("Workspaces().Remember() error = %v", err)
	}
	if last, err := workspaces.Last(context.Background()); err != nil {
		t.Fatalf("Workspaces().Last() error = %v", err)
	} else if filepath.Clean(last) != filepath.Clean(workspaceA) {
		t.Fatalf("last workspace=%q, want %q", last, workspaceA)
	}
	if resolved, err := workspaces.ResolveForeground(context.Background(), coreapi.ResolveForegroundWorkspaceRequest{}); err != nil {
		t.Fatalf("Workspaces().ResolveForeground() error = %v", err)
	} else if filepath.Clean(resolved) != filepath.Clean(workspaceA) {
		t.Fatalf("resolved workspace=%q, want %q", resolved, workspaceA)
	}
	if err := workspaces.Use(context.Background(), coreapi.WorkspacePathRequest{Path: workspaceA}); err != nil {
		t.Fatalf("Workspaces().Use() error = %v", err)
	}
	if err := workspaces.Trust(context.Background(), coreapi.WorkspacePathRequest{Path: workspaceA}); err != nil {
		t.Fatalf("Workspaces().Trust() error = %v", err)
	}
	items, err := workspaces.List(context.Background())
	if err != nil {
		t.Fatalf("Workspaces().List() error = %v", err)
	}
	var foundA bool
	for _, item := range items {
		if filepath.Clean(item.Path) == filepath.Clean(workspaceA) {
			foundA = true
			if !item.Active || !item.Trusted {
				t.Fatalf("workspaceA=%+v, want active and trusted", item)
			}
		}
	}
	if !foundA {
		t.Fatalf("workspaceA not listed: %+v", items)
	}

	if err := workspaces.Add(context.Background(), coreapi.WorkspacePathRequest{Path: workspaceB}); err != nil {
		t.Fatalf("Workspaces().Add(B) error = %v", err)
	}
	if err := workspaces.Remove(context.Background(), coreapi.WorkspacePathRequest{Path: workspaceB}); err != nil {
		t.Fatalf("Workspaces().Remove(B) error = %v", err)
	}
	if err := workspaces.Forget(context.Background(), coreapi.WorkspacePathRequest{Path: workspaceA}); err != nil {
		t.Fatalf("Workspaces().Forget(A) error = %v", err)
	}
}

func TestLegacyEngineCreatesAndResumesSessions(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()

	engine := NewLegacyEngine(rt)
	created, err := engine.Sessions().Create(context.Background(), coreapi.CreateSessionRequest{
		WorkspaceRoot: workspace,
		Title:         "thread-created",
		Messages:      []coreapi.SessionMessage{{Role: "user", Type: "text", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Sessions().Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("created.ID is empty")
	}
	if filepath.Clean(created.WorkspaceRoot) != filepath.Clean(workspace) {
		t.Fatalf("created.WorkspaceRoot=%q, want %q", created.WorkspaceRoot, workspace)
	}
	if got := created.Metadata["title"]; got != "thread-created" {
		t.Fatalf("created.Metadata[title]=%v, want thread-created", got)
	}

	other, err := rt.CreateWorkspaceSession(workspace, "thread-other", nil)
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(other) error = %v", err)
	}
	if other.ID == created.ID {
		t.Fatal("expected distinct sessions")
	}
	resumed, err := engine.Sessions().Resume(context.Background(), coreapi.ResumeSessionRequest{
		WorkspaceRoot: workspace,
		SessionID:     created.ID,
	})
	if err != nil {
		t.Fatalf("Sessions().Resume() error = %v", err)
	}
	if resumed.ID != created.ID {
		t.Fatalf("resumed.ID=%q, want %q", resumed.ID, created.ID)
	}
	currentID, err := rt.GetWorkspaceCurrentSession(workspace)
	if err != nil {
		t.Fatalf("GetWorkspaceCurrentSession() error = %v", err)
	}
	if currentID != created.ID {
		t.Fatalf("currentID=%q, want resumed %q", currentID, created.ID)
	}
}

func TestLegacyEngineManagesCurrentRenameAndDeleteSessions(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	metaA, err := rt.CreateWorkspaceSession(workspace, "thread-a", []SessionMessage{{Role: "assistant", Type: "text", Content: "a"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(A) error = %v", err)
	}
	metaB, err := rt.CreateWorkspaceSession(workspace, "thread-b", []SessionMessage{{Role: "assistant", Type: "text", Content: "b"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(B) error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	sessions := NewLegacyEngine(rt).Sessions()
	if err := sessions.SetCurrent(context.Background(), coreapi.SetCurrentSessionRequest{
		WorkspaceRoot: workspace,
		SessionID:     metaA.ID,
	}); err != nil {
		t.Fatalf("Sessions().SetCurrent() error = %v", err)
	}
	current, err := sessions.Current(context.Background(), coreapi.CurrentSessionRequest{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("Sessions().Current() error = %v", err)
	}
	if current.ID != metaA.ID {
		t.Fatalf("current.ID=%q, want %q", current.ID, metaA.ID)
	}

	renamed, err := sessions.Rename(context.Background(), coreapi.RenameSessionRequest{
		SessionID: metaA.ID,
		Title:     "thread-renamed",
	})
	if err != nil {
		t.Fatalf("Sessions().Rename() error = %v", err)
	}
	if got := renamed.Metadata["title"]; got != "thread-renamed" {
		t.Fatalf("renamed.Metadata[title]=%v, want thread-renamed", got)
	}

	saved, err := sessions.SaveMessages(context.Background(), coreapi.SaveSessionMessagesRequest{
		WorkspaceRoot: workspace,
		SessionID:     metaA.ID,
		Messages: []coreapi.SessionMessage{
			{Role: "user", Type: "text", Content: "updated hello"},
			{Role: "assistant", Type: "text", Content: "updated world"},
		},
	})
	if err != nil {
		t.Fatalf("Sessions().SaveMessages() error = %v", err)
	}
	if saved.ID != metaA.ID {
		t.Fatalf("saved.ID=%q, want %q", saved.ID, metaA.ID)
	}
	messages, err := sessions.LoadMessages(context.Background(), coreapi.LoadSessionMessagesRequest{
		WorkspaceRoot: workspace,
		SessionID:     metaA.ID,
	})
	if err != nil {
		t.Fatalf("Sessions().LoadMessages() error = %v", err)
	}
	if len(messages) != 2 || messages[0].Content != "updated hello" || messages[1].Content != "updated world" {
		t.Fatalf("messages=%+v, want updated transcript", messages)
	}

	if err := sessions.Delete(context.Background(), coreapi.DeleteSessionRequest{SessionID: metaA.ID}); err != nil {
		t.Fatalf("Sessions().Delete() error = %v", err)
	}
	items, err := sessions.List(context.Background(), coreapi.ListSessionsRequest{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("Sessions().List() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != metaB.ID {
		t.Fatalf("remaining sessions=%+v, want only %q", items, metaB.ID)
	}
}

func TestLegacyEngineRespondsToApproval(t *testing.T) {
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()

	if err := engine.Approvals().Respond(context.Background(), coreapi.ApprovalResponse{
		ApprovalID: "approval-1",
		Decision:   "allow_once",
	}); err != nil {
		t.Fatalf("Approvals().Respond(allow_once) error = %v", err)
	}
	if err := engine.Approvals().Respond(context.Background(), coreapi.ApprovalResponse{
		ApprovalID: "approval-1",
		Decision:   "deny",
	}); err != nil {
		t.Fatalf("Approvals().Respond(deny) error = %v", err)
	}
	if err := engine.Approvals().Respond(context.Background(), coreapi.ApprovalResponse{
		ApprovalID: "approval-1",
		Decision:   "maybe",
	}); err == nil {
		t.Fatal("Approvals().Respond(maybe) error = nil, want error")
	}
	if err := engine.Approvals().Respond(context.Background(), coreapi.ApprovalResponse{
		Decision: "allow_once",
	}); err == nil {
		t.Fatal("Approvals().Respond(empty id) error = nil, want error")
	}
}

func TestLegacyEngineRespondsToInquiry(t *testing.T) {
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()

	if err := engine.Inquiries().Respond(context.Background(), coreapi.InquiryResponse{
		InquiryID: "inq-1",
		Option:    "manual",
		Text:      "use manual mode",
	}); err != nil {
		t.Fatalf("Inquiries().Respond() error = %v", err)
	}
	if err := engine.Inquiries().Respond(context.Background(), coreapi.InquiryResponse{
		Option: "manual",
	}); err == nil {
		t.Fatal("Inquiries().Respond(empty id) error = nil, want error")
	}
}

func TestLegacyEventBusPublishSubscribeFilters(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng := NewLegacyEngine(rt)
	sub := eng.Events()
	events, err := sub.Subscribe(ctx, coreapi.EventFilter{
		SessionID: "sess-a",
		TurnID:    "turn-a",
	})
	if err != nil {
		t.Fatalf("Events().Subscribe() error = %v", err)
	}
	pub := legacyEventBus{rt: rt}
	ignored := protocol.NewEvent(protocol.EventTypeTextDelta, protocol.EventOptions{
		SessionID: "sess-b",
		RequestID: "turn-a",
		Payload:   protocol.TextPayloadMap(protocol.TextPayload{Text: "ignore"}),
	})
	if err := pub.Publish(context.Background(), ignored); err != nil {
		t.Fatalf("Publish(ignored) error = %v", err)
	}
	select {
	case got := <-events:
		t.Fatalf("received filtered event unexpectedly: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}

	want := protocol.NewEvent(protocol.EventTypeTextDelta, protocol.EventOptions{
		EventID:   "evt-a",
		SessionID: "sess-a",
		RequestID: "turn-a",
		Payload:   protocol.TextPayloadMap(protocol.TextPayload{Text: "hello"}),
	})
	if err := pub.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish(want) error = %v", err)
	}
	select {
	case got := <-events:
		if got.EventID != "evt-a" || got.SessionID != "sess-a" || got.RequestID != "turn-a" {
			t.Fatalf("got event=%+v, want evt-a/sess-a/turn-a", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for published event")
	}
}

func TestLegacyEngineUnsupportedMethods(t *testing.T) {
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()

	if _, err := engine.Tools().Execute(context.Background(), coreapi.ToolRequest{Name: "unknown"}); !errors.Is(err, coreapi.ErrUnsupported) {
		t.Fatalf("Tools().Execute() error = %v, want ErrUnsupported", err)
	}
}

func TestLegacyEngineToolCatalogListsBuiltinTools(t *testing.T) {
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()

	workspace := t.TempDir()
	defs, err := engine.ToolCatalog().List(context.Background(), coreapi.ListToolCatalogRequest{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("ToolCatalog().List() error = %v", err)
	}
	if len(defs) == 0 {
		t.Fatal("ToolCatalog().List() returned empty, want builtin tools")
	}

	seen := map[string]coreapi.ToolDefinition{}
	for _, d := range defs {
		seen[d.Name] = d
	}

	bashDef, ok := seen["bash"]
	if !ok {
		t.Fatal("expected bash tool in catalog")
	}
	if bashDef.Source != "builtin" {
		t.Fatalf("bash.Source=%q, want builtin", bashDef.Source)
	}
	if bashDef.RiskLevel == "" {
		t.Fatal("bash.RiskLevel is empty")
	}
	if bashDef.Category == "" {
		t.Fatal("bash.Category is empty")
	}

	readDef, ok := seen["read"]
	if !ok {
		t.Fatal("expected read tool in catalog")
	}
	if readDef.Source != "builtin" {
		t.Fatalf("read.Source=%q, want builtin", readDef.Source)
	}
	if !readDef.ReadOnly {
		t.Fatal("read.ReadOnly=false, want true")
	}
	if !readDef.Invocable {
		t.Fatal("read.Invocable=false, want true")
	}
}

func TestLegacyEngineToolCatalogMapsParamsAndMetadata(t *testing.T) {
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()

	workspace := t.TempDir()
	defs, err := engine.ToolCatalog().List(context.Background(), coreapi.ListToolCatalogRequest{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("ToolCatalog().List() error = %v", err)
	}

	var bashDef coreapi.ToolDefinition
	for _, d := range defs {
		if d.Name == "bash" {
			bashDef = d
			break
		}
	}
	if bashDef.Name == "" {
		t.Fatal("bash tool not found")
	}
	if bashDef.Params == nil {
		t.Fatal("bash.Params is nil")
	}
	commandParam, ok := bashDef.Params["command"]
	if !ok {
		t.Fatal("bash.Params missing 'command'")
	}
	if commandParam.Type != "string" {
		t.Fatalf("bash.Params[command].Type=%q, want string", commandParam.Type)
	}
	if !commandParam.Required {
		t.Fatal("bash.Params[command].Required=false, want true")
	}
	if bashDef.Tags == nil {
		t.Fatal("bash.Tags is nil")
	}
}

func TestLegacyEngineToolCatalogUsesWorkspaceRoot(t *testing.T) {
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()

	workspace := t.TempDir()
	defs1, err := engine.ToolCatalog().List(context.Background(), coreapi.ListToolCatalogRequest{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	defs2, err := engine.ToolCatalog().List(context.Background(), coreapi.ListToolCatalogRequest{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("second List() error = %v", err)
	}
	if len(defs1) != len(defs2) {
		t.Fatalf("len(defs1)=%d, len(defs2)=%d, want same count", len(defs1), len(defs2))
	}
}

func TestLegacyEngineToolCatalogNilRuntime(t *testing.T) {
	svc := legacyToolCatalogService{rt: nil}
	_, err := svc.List(context.Background(), coreapi.ListToolCatalogRequest{})
	if !errors.Is(err, coreapi.ErrUnsupported) {
		t.Fatalf("List() error = %v, want ErrUnsupported", err)
	}
}

func TestLegacyEngineAgentServiceTracksState(t *testing.T) {
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()

	ctx := context.Background()
	eventCtx, cancelEvents := context.WithCancel(ctx)
	defer cancelEvents()
	events, err := engine.Events().Subscribe(eventCtx, coreapi.EventFilter{})
	if err != nil {
		t.Fatalf("Events().Subscribe() error = %v", err)
	}

	root, err := engine.Agents().Spawn(ctx, coreapi.SpawnAgentRequest{
		RoleID: "senior_dev",
		Task:   "root task",
	})
	if err != nil {
		t.Fatalf("Agents().Spawn(root) error = %v", err)
	}
	if root.RoleID != "senior-dev" || root.Task != "root task" || root.ParentAgentID != "" {
		t.Fatalf("root=%+v, want senior-dev root task with no parent", root)
	}
	spawnEvent := receiveProtocolEvent(t, events)
	if spawnEvent.EventType != protocol.EventTypeAgentStarted || spawnEvent.Payload["agent_id"] != root.ID {
		t.Fatalf("spawnEvent=%+v, want agent.started for %s", spawnEvent, root.ID)
	}

	child, err := engine.Agents().Spawn(ctx, coreapi.SpawnAgentRequest{
		ParentAgentID: root.ID,
		RoleID:        "review",
		Task:          "inspect changes",
	})
	if err != nil {
		t.Fatalf("Agents().Spawn(child) error = %v", err)
	}
	if child.ParentAgentID != root.ID || child.RoleID != "reviewer" {
		t.Fatalf("child=%+v, want parent %s and reviewer role", child, root.ID)
	}
	_ = receiveProtocolEvent(t, events)
	if err := engine.Agents().SendInput(ctx, coreapi.AgentInput{AgentID: child.ID, Input: "hello"}); err != nil {
		t.Fatalf("Agents().SendInput() error = %v", err)
	}
	inputEvent := receiveProtocolEvent(t, events)
	if inputEvent.EventType != protocol.EventTypeAgentProgress || inputEvent.Payload["agent_id"] != child.ID {
		t.Fatalf("inputEvent=%+v, want agent.progress for %s", inputEvent, child.ID)
	}
	items, err := engine.Agents().List(ctx, coreapi.ListAgentsRequest{})
	if err != nil {
		t.Fatalf("Agents().List() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("Agents().List() len=%d, want 2", len(items))
	}
	snapshot, err := engine.State().Snapshot(ctx)
	if err != nil {
		t.Fatalf("State().Snapshot() error = %v", err)
	}
	if len(snapshot.Agents) != 2 {
		t.Fatalf("snapshot.Agents=%+v, want root and child", snapshot.Agents)
	}
	agentsByID := make(map[string]coreapi.Agent, len(snapshot.Agents))
	for _, agent := range snapshot.Agents {
		agentsByID[agent.ID] = agent
	}
	if agentsByID[root.ID].ID != root.ID || agentsByID[child.ID].ParentAgentID != root.ID {
		t.Fatalf("snapshot.Agents=%+v, want root and child", snapshot.Agents)
	}
	got, err := engine.Agents().Wait(ctx, coreapi.AgentRef{AgentID: child.ID})
	if err != nil {
		t.Fatalf("Agents().Wait() error = %v", err)
	}
	if got.ID != child.ID || got.Status != string(agentcore.AgentPending) {
		t.Fatalf("Agents().Wait()=%+v, want pending child", got)
	}
	if err := engine.Agents().Close(ctx, coreapi.AgentRef{AgentID: child.ID}); err != nil {
		t.Fatalf("Agents().Close() error = %v", err)
	}
	closeEvent := receiveProtocolEvent(t, events)
	if closeEvent.EventType != protocol.EventTypeAgentCancelled || closeEvent.Payload["agent_id"] != child.ID {
		t.Fatalf("closeEvent=%+v, want agent.cancelled for %s", closeEvent, child.ID)
	}
	got, err = engine.Agents().Wait(ctx, coreapi.AgentRef{AgentID: child.ID})
	if err != nil {
		t.Fatalf("Agents().Wait(after close) error = %v", err)
	}
	if got.Status != string(agentcore.AgentCancelled) {
		t.Fatalf("status=%q, want cancelled", got.Status)
	}
}

func receiveProtocolEvent(t *testing.T, events <-chan protocol.Envelope) protocol.Envelope {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for protocol event")
		return protocol.Envelope{}
	}
}

func TestLegacyEngineValidatesBashToolExecute(t *testing.T) {
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()

	if _, err := engine.Tools().Execute(context.Background(), coreapi.ToolRequest{
		Name:      "bash",
		RequestID: "tool-test",
		Args:      json.RawMessage(`{}`),
	}); err == nil {
		t.Fatal("Tools().Execute(bash empty command) error = nil, want error")
	}
	if _, err := engine.Tools().Execute(context.Background(), coreapi.ToolRequest{
		Name: "bash",
		Args: json.RawMessage(`"not an object"`),
	}); err == nil {
		t.Fatal("Tools().Execute(bash invalid args) error = nil, want error")
	}
}

func TestLegacyEngineSandboxPolicyMapsAccessMode(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	engine := NewLegacyEngine(rt)
	if err := engine.Sandbox().SetPolicy(context.Background(), coreapi.SessionRef{}, sandbox.Policy{Mode: sandbox.ModeReadOnly}); err != nil {
		t.Fatalf("Sandbox().SetPolicy(read-only) error = %v", err)
	}
	policy, err := engine.Sandbox().Policy(context.Background(), coreapi.SessionRef{})
	if err != nil {
		t.Fatalf("Sandbox().Policy() error = %v", err)
	}
	if policy.Mode != sandbox.ModeReadOnly {
		t.Fatalf("policy.Mode=%q, want read-only", policy.Mode)
	}
	if filepath.Clean(policy.WorkspaceRoot) != filepath.Clean(workspace) {
		t.Fatalf("policy.WorkspaceRoot=%q, want %q", policy.WorkspaceRoot, workspace)
	}

	if err := engine.Sandbox().SetPolicy(context.Background(), coreapi.SessionRef{}, sandbox.Policy{Mode: sandbox.ModeDangerFullAccess}); err != nil {
		t.Fatalf("Sandbox().SetPolicy(danger-full-access) error = %v", err)
	}
	policy, err = engine.Sandbox().Policy(context.Background(), coreapi.SessionRef{})
	if err != nil {
		t.Fatalf("Sandbox().Policy() danger error = %v", err)
	}
	if policy.Mode != sandbox.ModeDangerFullAccess {
		t.Fatalf("policy.Mode=%q, want danger-full-access", policy.Mode)
	}
}

func TestLegacyEngineBashToolHonorsReadOnlySandbox(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	engine := NewLegacyEngine(NewRuntime())
	defer engine.(*legacyEngine).rt.Close()
	if err := engine.Sandbox().SetPolicy(context.Background(), coreapi.SessionRef{}, sandbox.Policy{Mode: sandbox.ModeReadOnly}); err != nil {
		t.Fatalf("Sandbox().SetPolicy(read-only) error = %v", err)
	}

	result, err := engine.Tools().Execute(context.Background(), coreapi.ToolRequest{
		Name:      "bash",
		RequestID: "tool-readonly",
		Args:      json.RawMessage(`{"command":"echo hi"}`),
	})
	if err != nil {
		t.Fatalf("Tools().Execute(read-only bash) transport error = %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("result.Status=%q, want error", result.Status)
	}
	if !strings.Contains(result.Error, "read-only") {
		t.Fatalf("result.Error=%q, want read-only policy error", result.Error)
	}
}

func TestLegacyEngineBashToolBlocksWorkspaceWriteOutsideTarget(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}
	outside := filepath.Join(filepath.Dir(workspace), "outside.txt")

	result, err := NewLegacyEngine(rt).Tools().Execute(context.Background(), coreapi.ToolRequest{
		Name:      "bash",
		RequestID: "tool-outside",
		Args:      json.RawMessage(`{"command":"echo hi > ` + filepath.ToSlash(outside) + `"}`),
	})
	if err != nil {
		t.Fatalf("Tools().Execute(outside bash) transport error = %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("result.Status=%q, want error", result.Status)
	}
	if !strings.Contains(result.Error, "outside workspace") {
		t.Fatalf("result.Error=%q, want outside workspace policy error", result.Error)
	}
	if _, statErr := os.Stat(outside); !os.IsNotExist(statErr) {
		t.Fatalf("outside target should not be written, stat error = %v", statErr)
	}
}

func TestRuntimeJSONRPCClientReadsStateSnapshot(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "thread-rpc", []SessionMessage{{Role: "assistant", Type: "text", Content: "hello rpc"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}
	var snapshot coreapi.StateSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodStateSnapshot, nil, &snapshot); err != nil {
		t.Fatalf("Call(state/snapshot) error = %v", err)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != meta.ID {
		t.Fatalf("CurrentSession=%+v, want %q", snapshot.CurrentSession, meta.ID)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Content != "hello rpc" {
		t.Fatalf("Messages=%+v, want persisted message", snapshot.Messages)
	}
}

func TestRuntimeServeJSONRPCStreamHandlesInitialize(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	req, err := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req-1"), protocoljsonrpc.MethodInitialize, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	var input bytes.Buffer
	if err := protocoljsonrpc.NewStream(nil, &input).WriteMessage(req); err != nil {
		t.Fatalf("WriteMessage(input) error = %v", err)
	}
	var output bytes.Buffer
	if err := rt.ServeJSONRPCStream(context.Background(), &input, &output); err != nil {
		t.Fatalf("ServeJSONRPCStream() error = %v", err)
	}
	decoded, err := protocoljsonrpc.NewStream(&output, nil).ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage(output) error = %v", err)
	}
	if decoded.Kind != protocoljsonrpc.KindResponse || decoded.Response == nil {
		t.Fatalf("decoded=%+v, want response", decoded)
	}
	var result coreapijsonrpc.InitializeResult
	if err := json.Unmarshal(decoded.Response.Result, &result); err != nil {
		t.Fatalf("Unmarshal(result) error = %v", err)
	}
	if result.ServerName != "eos-core" || !containsString(result.Methods, protocoljsonrpc.MethodStateSnapshot) {
		t.Fatalf("result=%+v, want eos-core with state/snapshot", result)
	}
}

func TestRuntimeJSONRPCClientManagesWorkspaces(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceRemember, coreapi.RememberWorkspaceRequest{
		Path:       workspace,
		Foreground: true,
	}, nil); err != nil {
		t.Fatalf("Call(workspace/remember) error = %v", err)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceSetForeground, coreapi.WorkspacePathRequest{Path: workspace}, nil); err != nil {
		t.Fatalf("Call(workspace/set_foreground) error = %v", err)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceTrust, coreapi.WorkspacePathRequest{Path: workspace}, nil); err != nil {
		t.Fatalf("Call(workspace/trust) error = %v", err)
	}
	var items []coreapi.Workspace
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceList, nil, &items); err != nil {
		t.Fatalf("Call(workspace/list) error = %v", err)
	}
	var found bool
	for _, item := range items {
		if filepath.Clean(item.Path) == filepath.Clean(workspace) {
			found = true
			if !item.Active || !item.Trusted {
				t.Fatalf("workspace=%+v, want active and trusted", item)
			}
		}
	}
	if !found {
		t.Fatalf("workspace not listed: %+v", items)
	}
	var pathOut struct {
		Path string `json:"path"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceLast, nil, &pathOut); err != nil {
		t.Fatalf("Call(workspace/last) error = %v", err)
	}
	if filepath.Clean(pathOut.Path) != filepath.Clean(workspace) {
		t.Fatalf("last path=%q, want %q", pathOut.Path, workspace)
	}
}

func TestRuntimeJSONRPCClientListsSessions(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()
	meta, err := rt.CreateWorkspaceSession(workspace, "thread-rpc-list", []SessionMessage{{Role: "assistant", Type: "text", Content: "hello"}})
	if err != nil {
		t.Fatalf("CreateWorkspaceSession() error = %v", err)
	}

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}
	var sessions []coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionList, coreapi.ListSessionsRequest{WorkspaceRoot: workspace}, &sessions); err != nil {
		t.Fatalf("Call(session/list) error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != meta.ID {
		t.Fatalf("sessions=%+v, want %q", sessions, meta.ID)
	}
}

func TestRuntimeJSONRPCClientCreatesAndResumesSession(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()
	workspace := t.TempDir()

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}
	var created coreapi.Session
	createReq := coreapi.CreateSessionRequest{
		WorkspaceRoot: workspace,
		Title:         "rpc-created",
		Messages:      []coreapi.SessionMessage{{Role: "user", Type: "text", Content: "hello rpc create"}},
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionCreate, createReq, &created); err != nil {
		t.Fatalf("Call(session/create) error = %v", err)
	}
	if created.ID == "" || created.Metadata["title"] != "rpc-created" {
		t.Fatalf("created=%+v, want rpc-created title", created)
	}

	other, err := rt.CreateWorkspaceSession(workspace, "rpc-other", nil)
	if err != nil {
		t.Fatalf("CreateWorkspaceSession(other) error = %v", err)
	}
	if other.ID == created.ID {
		t.Fatal("expected distinct other session")
	}
	var resumed coreapi.Session
	resumeReq := coreapi.ResumeSessionRequest{WorkspaceRoot: workspace, SessionID: created.ID}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionResume, resumeReq, &resumed); err != nil {
		t.Fatalf("Call(session/resume) error = %v", err)
	}
	if resumed.ID != created.ID {
		t.Fatalf("resumed.ID=%q, want %q", resumed.ID, created.ID)
	}
	currentID, err := rt.GetWorkspaceCurrentSession(workspace)
	if err != nil {
		t.Fatalf("GetWorkspaceCurrentSession() error = %v", err)
	}
	if currentID != created.ID {
		t.Fatalf("currentID=%q, want %q", currentID, created.ID)
	}

	var current coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionCurrent, coreapi.CurrentSessionRequest{WorkspaceRoot: workspace}, &current); err != nil {
		t.Fatalf("Call(session/current) error = %v", err)
	}
	if current.ID != created.ID {
		t.Fatalf("current.ID=%q, want %q", current.ID, created.ID)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionSetCurrent, coreapi.SetCurrentSessionRequest{
		WorkspaceRoot: workspace,
		SessionID:     other.ID,
	}, nil); err != nil {
		t.Fatalf("Call(session/set_current) error = %v", err)
	}
	if currentID, err := rt.GetWorkspaceCurrentSession(workspace); err != nil {
		t.Fatalf("GetWorkspaceCurrentSession(after set) error = %v", err)
	} else if currentID != other.ID {
		t.Fatalf("currentID(after set)=%q, want %q", currentID, other.ID)
	}

	var renamed coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionRename, coreapi.RenameSessionRequest{
		WorkspaceRoot: workspace,
		SessionID:     created.ID,
		Title:         "rpc-renamed",
	}, &renamed); err != nil {
		t.Fatalf("Call(session/rename) error = %v", err)
	}
	if renamed.Metadata["title"] != "rpc-renamed" {
		t.Fatalf("renamed=%+v, want rpc-renamed title", renamed)
	}
	var saved coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionMessagesSave, coreapi.SaveSessionMessagesRequest{
		WorkspaceRoot: workspace,
		SessionID:     created.ID,
		Messages: []coreapi.SessionMessage{
			{Role: "user", Type: "text", Content: "saved rpc hello"},
			{Role: "assistant", Type: "text", Content: "saved rpc world"},
		},
	}, &saved); err != nil {
		t.Fatalf("Call(session/messages/save) error = %v", err)
	}
	if saved.ID != created.ID {
		t.Fatalf("saved.ID=%q, want %q", saved.ID, created.ID)
	}
	var messages []coreapi.SessionMessage
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionMessagesLoad, coreapi.LoadSessionMessagesRequest{
		WorkspaceRoot: workspace,
		SessionID:     created.ID,
	}, &messages); err != nil {
		t.Fatalf("Call(session/messages/load) error = %v", err)
	}
	if len(messages) != 2 || messages[0].Content != "saved rpc hello" || messages[1].Content != "saved rpc world" {
		t.Fatalf("messages=%+v, want saved rpc transcript", messages)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionDelete, coreapi.DeleteSessionRequest{
		WorkspaceRoot: workspace,
		SessionID:     created.ID,
	}, nil); err != nil {
		t.Fatalf("Call(session/delete) error = %v", err)
	}
	sessions := rt.ListSessions()
	for _, session := range sessions {
		if session.ID == created.ID {
			t.Fatalf("deleted session still listed: %+v", sessions)
		}
	}
}

func TestRuntimeJSONRPCClientRespondsToApproval(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodApprovalRespond, coreapi.ApprovalResponse{
		ApprovalID: "approval-rpc",
		Decision:   "allow_once",
	}, nil); err != nil {
		t.Fatalf("Call(approval/respond) error = %v", err)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodApprovalRespond, coreapi.ApprovalResponse{
		ApprovalID: "approval-rpc",
		Decision:   "maybe",
	}, nil); err == nil {
		t.Fatal("Call(approval/respond invalid decision) error = nil, want error")
	}
}

func TestRuntimeJSONRPCClientRespondsToInquiry(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInquiryRespond, coreapi.InquiryResponse{
		InquiryID: "inq-rpc",
		Option:    "auto",
		Text:      "continue",
	}, nil); err != nil {
		t.Fatalf("Call(inquiry/respond) error = %v", err)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInquiryRespond, coreapi.InquiryResponse{
		Option: "auto",
	}, nil); err == nil {
		t.Fatal("Call(inquiry/respond empty id) error = nil, want error")
	}
}

func TestRuntimeJSONRPCClientTurnMethodsAreRegistered(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()
	defer rt.Close()

	client, err := rt.JSONRPCClient()
	if err != nil {
		t.Fatalf("JSONRPCClient() error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	var turn coreapi.Turn
	if err := client.Call(canceled, protocoljsonrpc.MethodTurnStart, coreapi.StartTurnRequest{
		SessionID: "sess-1",
		Input:     "hello",
	}, &turn); err == nil {
		t.Fatal("Call(turn/start canceled) error = nil, want context canceled")
	} else if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("Call(turn/start canceled) error = %v, want context canceled", err)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodTurnInterrupt, coreapi.TurnRef{
		SessionID: "sess-1",
		TurnID:    "turn-1",
	}, nil); err == nil {
		t.Fatal("Call(turn/interrupt unknown) error = nil, want not running")
	} else if !strings.Contains(err.Error(), "not running") {
		t.Fatalf("Call(turn/interrupt unknown) error = %v, want not running", err)
	}
}

func agentByID(items []coreapi.Agent, id string) (coreapi.Agent, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return coreapi.Agent{}, false
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
