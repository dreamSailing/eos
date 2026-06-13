//go:build legacy

package adapter

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/tools/fileops"
	sharedcore "github.com/dreamSailing/eos/pkg/core"
	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/protocol"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

func TestRuntimeAdapterSessionMethodsUseJSONRPC(t *testing.T) {
	workspace := t.TempDir()
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	if err := rt.RememberWorkspace(workspace, true); err != nil {
		t.Fatalf("RememberWorkspace() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}

	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()
	id, err := adapter.SaveSessionMessages(ctx, "", []coreapi.SessionMessage{
		{Role: "user", Type: "user", Content: "hello", Time: time.Now()},
		{Role: "assistant", Type: "assistant", Content: "world", Time: time.Now()},
	})
	if err != nil {
		t.Fatalf("SaveSessionMessages() error = %v", err)
	}
	if id == "" {
		t.Fatal("SaveSessionMessages() id is empty")
	}

	current, err := adapter.CurrentSessionID(ctx)
	if err != nil {
		t.Fatalf("CurrentSessionID() error = %v", err)
	}
	if current != id {
		t.Fatalf("CurrentSessionID()=%q, want %q", current, id)
	}

	sessions, err := adapter.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != id {
		t.Fatalf("sessions=%+v, want saved session %q", sessions, id)
	}

	loaded, err := adapter.LoadSessionMessages(ctx, id)
	if err != nil {
		t.Fatalf("LoadSessionMessages() error = %v", err)
	}
	if len(loaded) != 2 || loaded[0].Content != "hello" || loaded[1].Content != "world" {
		t.Fatalf("loaded=%+v, want saved messages", loaded)
	}

	if err := adapter.RenameSession(ctx, id, "renamed session"); err != nil {
		t.Fatalf("RenameSession() error = %v", err)
	}
	sessions, err = adapter.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() after rename error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].Title != "renamed session" {
		t.Fatalf("sessions=%+v, want renamed session", sessions)
	}

	exportPath := filepath.Join(workspace, "export.md")
	if err := adapter.ExportSessionMarkdown(ctx, id, exportPath); err != nil {
		t.Fatalf("ExportSessionMarkdown() error = %v", err)
	}
	exported, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("ReadFile(export) error = %v", err)
	}
	if !strings.Contains(string(exported), "hello") || !strings.Contains(string(exported), "world") {
		t.Fatalf("exported=%q, want transcript", string(exported))
	}

	if err := adapter.ResumeSession(ctx, id); err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}
}

func TestRuntimeAdapterPublishesJSONRPCNotificationsToEvents(t *testing.T) {
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	adapter := NewRuntimeAdapterFromRuntime(rt)
	events := adapter.Events()

	envelope := protocol.NewEvent(protocol.EventTypeTextDelta, protocol.EventOptions{
		RequestID: "turn-1",
		Payload:   protocol.TextPayloadMap(protocol.TextPayload{Text: "hello"}),
	})
	adapter.handleNotification(context.Background(), mustEventNotification(t, envelope))

	select {
	case event := <-events:
		if event.Type != string(protocol.EventTypeTextDelta) {
			t.Fatalf("event.Type=%q, want text.delta", event.Type)
		}
		if event.RID != "turn-1" {
			t.Fatalf("event.RID=%q, want turn-1", event.RID)
		}
		if got, _ := event.Data["text"].(string); got != "hello" {
			t.Fatalf("event text=%q, want hello", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for JSON-RPC notification event")
	}
}

func TestRuntimeAdapterWorkspaceMethodsUseJSONRPC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	workspace := t.TempDir()
	removedWorkspace := t.TempDir()
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	if err := adapter.AddWorkspace(ctx, workspace); err != nil {
		t.Fatalf("AddWorkspace() error = %v", err)
	}
	if err := adapter.UseWorkspace(ctx, workspace); err != nil {
		t.Fatalf("UseWorkspace() error = %v", err)
	}
	if got := adapter.ActiveWorkspace(ctx); got != workspace {
		t.Fatalf("ActiveWorkspace()=%q, want %q", got, workspace)
	}
	items, err := adapter.Workspaces(ctx)
	if err != nil {
		t.Fatalf("Workspaces() error = %v", err)
	}
	if !hasWorkspace(items, workspace, true) {
		t.Fatalf("workspaces=%+v, want active %q", items, workspace)
	}
	if err := adapter.AddWorkspace(ctx, removedWorkspace); err != nil {
		t.Fatalf("AddWorkspace(removed) error = %v", err)
	}
	if err := adapter.RemoveWorkspace(ctx, removedWorkspace); err != nil {
		t.Fatalf("RemoveWorkspace() error = %v", err)
	}
	items, err = adapter.Workspaces(ctx)
	if err != nil {
		t.Fatalf("Workspaces(after remove) error = %v", err)
	}
	if hasWorkspacePath(items, removedWorkspace) {
		t.Fatalf("workspaces=%+v, did not expect removed workspace %q", items, removedWorkspace)
	}
}

func TestRuntimeAdapterGitMethodsUseJSONRPC(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	workspace := t.TempDir()
	runAdapterGitCommand(t, workspace, "init")
	runAdapterGitCommand(t, workspace, "config", "user.email", "test@example.com")
	runAdapterGitCommand(t, workspace, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	runAdapterGitCommand(t, workspace, "add", "a.txt")
	runAdapterGitCommand(t, workspace, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}

	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	if err := rt.RememberWorkspace(workspace, true); err != nil {
		t.Fatalf("RememberWorkspace() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	changes, err := adapter.GitStatus(ctx)
	if err != nil {
		t.Fatalf("GitStatus() error = %v", err)
	}
	if !hasAdapterGitChange(changes, "a.txt", "modified") {
		t.Fatalf("changes=%+v, want a.txt", changes)
	}
	diff, err := adapter.GitDiff(ctx, "a.txt")
	if err != nil {
		t.Fatalf("GitDiff() error = %v", err)
	}
	if !strings.Contains(diff, "+world") {
		t.Fatalf("diff=%q, want added world line", diff)
	}
	branches, err := adapter.GitBranches(ctx)
	if err != nil {
		t.Fatalf("GitBranches() error = %v", err)
	}
	if branches.Current == "" || len(branches.Branches) == 0 {
		t.Fatalf("branches=%+v, want current branch", branches)
	}
	log, err := adapter.GitLog(ctx, coreapi.GitLogRequest{Limit: 5, Oneline: true})
	if err != nil {
		t.Fatalf("GitLog() error = %v", err)
	}
	if !strings.Contains(log.Text, "init") {
		t.Fatalf("log=%q, want init commit", log.Text)
	}
	show, err := adapter.GitShow(ctx, coreapi.GitShowRequest{Revision: "HEAD", Path: "a.txt"})
	if err != nil {
		t.Fatalf("GitShow() error = %v", err)
	}
	if show.Revision != "HEAD" || !strings.Contains(show.Text, "a.txt") {
		t.Fatalf("show=%+v, want HEAD a.txt", show)
	}
}

func runAdapterGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func hasAdapterGitChange(changes []coreapi.GitChange, path, state string) bool {
	for _, change := range changes {
		if change.Path == path && change.State == state {
			return true
		}
	}
	return false
}

func TestRuntimeAdapterPermissionMethodsUseJSONRPC(t *testing.T) {
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	if err := adapter.SetAccessMode(ctx, "read-only"); err != nil {
		t.Fatalf("SetAccessMode() error = %v", err)
	}
	if err := adapter.SetApprovalMode(ctx, "never"); err != nil {
		t.Fatalf("SetApprovalMode() error = %v", err)
	}
	snapshot, err := adapter.PermissionSnapshot(ctx)
	if err != nil {
		t.Fatalf("PermissionSnapshot() error = %v", err)
	}
	if snapshot.AccessMode != "read-only" || snapshot.ApprovalMode != "never" {
		t.Fatalf("PermissionSnapshot()=%+v, want read-only/never", snapshot)
	}
	modeSnapshot, err := adapter.ModeSnapshot(ctx)
	if err != nil {
		t.Fatalf("ModeSnapshot() error = %v", err)
	}
	if modeSnapshot.ExecutionMode == "" {
		t.Fatalf("ModeSnapshot()=%+v, want execution mode", modeSnapshot)
	}
	review, err := adapter.PendingReview(ctx)
	if err != nil {
		t.Fatalf("PendingReview() error = %v", err)
	}
	if review.HasDiff || strings.TrimSpace(review.Diff) != "" {
		t.Fatalf("PendingReview()=%+v, want no pending diff", review)
	}
}

func TestRuntimeAdapterExtensionMethodsUseJSONRPC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	if _, err := adapter.Skills(ctx); err != nil {
		t.Fatalf("Skills() error = %v", err)
	}
	if err := adapter.ReloadSkills(ctx); err != nil {
		t.Fatalf("ReloadSkills() error = %v", err)
	}
	if invoked, err := adapter.InvokeSkill(ctx, "missing-skill", ""); err != nil {
		t.Fatalf("InvokeSkill() error = %v", err)
	} else if invoked {
		t.Fatal("InvokeSkill(missing-skill) invoked=true, want false")
	}
	if _, err := adapter.Plugins(ctx); err != nil {
		t.Fatalf("Plugins() error = %v", err)
	}
	if status, err := adapter.BrowserStatus(ctx); err != nil {
		t.Fatalf("BrowserStatus() error = %v", err)
	} else if strings.TrimSpace(status.ServerName) == "" {
		t.Fatalf("BrowserStatus()=%+v, want default server name", status)
	}
	if remote, ok, err := adapter.CurrentRemoteRepo(ctx); err != nil {
		t.Fatalf("CurrentRemoteRepo() error = %v", err)
	} else if ok {
		t.Fatalf("CurrentRemoteRepo()=%+v ok=true, want no active remote", remote)
	}
	if _, err := adapter.Tasks(ctx); err != nil {
		t.Fatalf("Tasks() error = %v", err)
	}
	if _, err := adapter.Todos(ctx); err != nil {
		t.Fatalf("Todos() error = %v", err)
	}
	if _, err := adapter.Agents(ctx); err != nil {
		t.Fatalf("Agents() error = %v", err)
	}
}

func TestRuntimeAdapterMCPMethodsUseJSONRPC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	entry := config.MCPEntry{
		Name:    "docs",
		Type:    config.MCPTypeStdio,
		Command: "node",
		Args:    []string{"server.js"},
		Envs:    map[string]string{"MODE": "test"},
		Enabled: true,
		Auth: &config.MCPAuth{
			Type:       "bearer",
			Token:      "secret",
			HeadersEnv: map[string]string{"X-Token": "TOKEN_ENV"},
		},
		ApprovalMode:         "on-request",
		ToolApprovalOverride: map[string]string{"search": "never"},
	}
	if err := adapter.AddMCPEntries(ctx, []config.MCPEntry{entry}); err != nil {
		t.Fatalf("AddMCPEntries() error = %v", err)
	}
	items, err := adapter.MCPServers(ctx)
	if err != nil {
		t.Fatalf("MCPServers() error = %v", err)
	}
	got, ok := findMCPEntry(items, "docs")
	if !ok {
		t.Fatalf("MCPServers()=%+v, want docs", items)
	}
	if got.Command != "node" || len(got.Args) != 1 || got.Args[0] != "server.js" || got.Envs["MODE"] != "test" ||
		got.Auth == nil || got.Auth.HeadersEnv["X-Token"] != "TOKEN_ENV" || got.ToolApprovalOverride["search"] != "never" {
		t.Fatalf("MCP entry=%+v, want full editable fields", got)
	}

	got.Args = []string{"updated.js"}
	got.Enabled = false
	if err := adapter.UpsertMCPEntry(ctx, got); err != nil {
		t.Fatalf("UpsertMCPEntry() error = %v", err)
	}
	if err := adapter.SetMCPEnabled(ctx, "docs", true); err != nil {
		t.Fatalf("SetMCPEnabled() error = %v", err)
	}
	items, err = adapter.MCPServers(ctx)
	if err != nil {
		t.Fatalf("MCPServers(after update) error = %v", err)
	}
	got, ok = findMCPEntry(items, "docs")
	if !ok || !got.Enabled || len(got.Args) != 1 || got.Args[0] != "updated.js" {
		t.Fatalf("updated MCP entry=%+v ok=%v, want enabled updated.js", got, ok)
	}

	if err := adapter.DeleteMCPServer(ctx, "docs"); err != nil {
		t.Fatalf("DeleteMCPServer() error = %v", err)
	}
	items, err = adapter.MCPServers(ctx)
	if err != nil {
		t.Fatalf("MCPServers(after delete) error = %v", err)
	}
	if _, ok := findMCPEntry(items, "docs"); ok {
		t.Fatalf("MCPServers()=%+v, did not expect docs", items)
	}
}

func TestRuntimeAdapterContextUsageAndVersionMethodsUseJSONRPC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	workspace := t.TempDir()
	target := filepath.Join(workspace, "dir", "file.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(target, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	if err := rt.RememberWorkspace(workspace, true); err != nil {
		t.Fatalf("RememberWorkspace() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	adapter.GetContext().AddUser("hello from context")
	preview, err := adapter.ContextPreview(ctx)
	if err != nil {
		t.Fatalf("ContextPreview() error = %v", err)
	}
	if !containsText(preview, "hello from context") {
		t.Fatalf("ContextPreview()=%+v, want added message", preview)
	}
	stats, err := adapter.ContextStats(ctx)
	if err != nil {
		t.Fatalf("ContextStats() error = %v", err)
	}
	if stats.MessageCount == 0 || stats.Estimated == 0 {
		t.Fatalf("ContextStats()=%+v, want non-zero stats", stats)
	}
	tokens, _, err := adapter.CurrentContextUsage(ctx)
	if err != nil {
		t.Fatalf("CurrentContextUsage() error = %v", err)
	}
	if tokens == 0 {
		t.Fatal("CurrentContextUsage() tokens = 0, want non-zero")
	}
	window, err := adapter.ContextWindowTokens(ctx)
	if err != nil {
		t.Fatalf("ContextWindowTokens() error = %v", err)
	}
	if window == 0 {
		t.Fatal("ContextWindowTokens() = 0, want non-zero")
	}
	if err := adapter.PinContextDocument(ctx, "test.md", "pinned content", 1000); err != nil {
		t.Fatalf("PinContextDocument() error = %v", err)
	}
	if message, err := adapter.CompactContext(ctx); err != nil {
		t.Fatalf("CompactContext() error = %v", err)
	} else if strings.TrimSpace(message) == "" {
		t.Fatal("CompactContext() message is empty")
	}
	exportPath := filepath.Join(workspace, ".eos", "context-export.md")
	if err := adapter.ExportContext(ctx, exportPath); err != nil {
		t.Fatalf("ExportContext() error = %v", err)
	}
	if _, err := os.Stat(exportPath); err != nil {
		t.Fatalf("ExportContext() did not write file: %v", err)
	}
	if err := adapter.ClearContext(ctx); err != nil {
		t.Fatalf("ClearContext() error = %v", err)
	}

	adapter.GetCore().AddTokenRecord(10, 5, 15)
	summary, err := adapter.UsageSummary(ctx)
	if err != nil {
		t.Fatalf("UsageSummary() error = %v", err)
	}
	if summary.Rounds != 1 || summary.TotalTokens == nil || *summary.TotalTokens != 15 {
		t.Fatalf("UsageSummary()=%+v, want one round with total 15", summary)
	}
	costItems, err := adapter.CostItems(ctx)
	if err != nil {
		t.Fatalf("CostItems() error = %v", err)
	}
	if len(costItems) != 1 || costItems[0].TotalTokens == nil || *costItems[0].TotalTokens != 15 {
		t.Fatalf("CostItems()=%+v, want one item with total 15", costItems)
	}

	ops := fileops.NewFileOperations()
	ops.SetRoot(workspace)
	version, err := ops.SaveVersion(target, "old\n")
	if err != nil {
		t.Fatalf("SaveVersion() error = %v", err)
	}
	_, err = ops.SaveVersion(target, "older\n")
	if err != nil {
		t.Fatalf("SaveVersion(second) error = %v", err)
	}
	versions, err := adapter.Versions(ctx)
	if err != nil {
		t.Fatalf("Versions() error = %v", err)
	}
	if !hasVersion(versions, version.ID, "dir/file.txt") {
		t.Fatalf("Versions()=%+v, want saved version %q", versions, version.ID)
	}
	if err := adapter.DeleteVersion(ctx, version.ID); err != nil {
		t.Fatalf("DeleteVersion() error = %v", err)
	}
	versions, err = adapter.Versions(ctx)
	if err != nil {
		t.Fatalf("Versions(after delete) error = %v", err)
	}
	if hasVersion(versions, version.ID, "dir/file.txt") {
		t.Fatalf("Versions(after delete)=%+v, did not expect %q", versions, version.ID)
	}
}

func TestRuntimeAdapterRulesMethodsUseJSONRPC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	workspace := t.TempDir()
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	if err := rt.RememberWorkspace(workspace, true); err != nil {
		t.Fatalf("RememberWorkspace() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	if err := adapter.SaveRules(ctx, "project", "project rule"); err != nil {
		t.Fatalf("SaveRules(project) error = %v", err)
	}
	if err := adapter.SaveRules(ctx, "global", "global rule"); err != nil {
		t.Fatalf("SaveRules(global) error = %v", err)
	}
	snapshot, err := adapter.RulesSnapshot(ctx)
	if err != nil {
		t.Fatalf("RulesSnapshot() error = %v", err)
	}
	project, ok := findRuleDoc(snapshot.Documents, "project")
	if !ok || !project.Exists || !strings.Contains(project.Content, "project rule") ||
		filepath.Clean(project.Path) != filepath.Join(workspace, ".eos", "Rules.md") {
		t.Fatalf("project rules doc=%+v ok=%v, want workspace project rule", project, ok)
	}
	global, ok := findRuleDoc(snapshot.Documents, "global")
	if !ok || !global.Exists || !strings.Contains(global.Content, "global rule") ||
		filepath.Clean(global.Path) != filepath.Join(home, ".eos", "Rules.md") {
		t.Fatalf("global rules doc=%+v ok=%v, want home global rule", global, ok)
	}
}

func TestRuntimeAdapterMemoryMethodsUseJSONRPC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	workspace := t.TempDir()
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	if err := rt.RememberWorkspace(workspace, true); err != nil {
		t.Fatalf("RememberWorkspace() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	if err := adapter.SaveMemory(ctx, "project", "# Project\n\n- project memory"); err != nil {
		t.Fatalf("SaveMemory(project) error = %v", err)
	}
	if err := adapter.SaveMemory(ctx, "global", "# Global\n\n- global memory"); err != nil {
		t.Fatalf("SaveMemory(global) error = %v", err)
	}
	if err := adapter.SaveMemory(ctx, "session", "session memory"); err != nil {
		t.Fatalf("SaveMemory(session) error = %v", err)
	}
	if err := adapter.RebuildMemoryIndex(ctx); err != nil {
		t.Fatalf("RebuildMemoryIndex() error = %v", err)
	}
	snapshot, err := adapter.MemorySnapshot(ctx)
	if err != nil {
		t.Fatalf("MemorySnapshot() error = %v", err)
	}
	project, ok := findMemoryDoc(snapshot.Documents, "project")
	if !ok || !project.Exists || !strings.Contains(project.Content, "project memory") {
		t.Fatalf("project memory doc=%+v ok=%v, want project memory", project, ok)
	}
	sessionDoc, ok := findMemoryDoc(snapshot.Documents, "session")
	if !ok || !sessionDoc.Exists || !strings.Contains(sessionDoc.Content, "session memory") {
		t.Fatalf("session memory doc=%+v ok=%v, want session memory", sessionDoc, ok)
	}
	index, ok := findMemoryDoc(snapshot.Documents, "index")
	if !ok || !index.Exists || !strings.Contains(index.Content, "project memory") {
		t.Fatalf("index memory doc=%+v ok=%v, want rebuilt index", index, ok)
	}
}

func TestRuntimeAdapterSettingsMethodsUseJSONRPC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	workspace := t.TempDir()
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	if err := rt.RememberWorkspace(workspace, true); err != nil {
		t.Fatalf("RememberWorkspace() error = %v", err)
	}
	if err := rt.SetForegroundWorkspace(workspace); err != nil {
		t.Fatalf("SetForegroundWorkspace() error = %v", err)
	}
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	disabled := false
	input := settings.Settings{
		Language:             "en",
		Theme:                "light",
		PlanPromptStyle:      "detailed",
		AutoContext:          false,
		DesktopNotifications: &disabled,
		MaxInjectKB:          24,
		WatchDebounceMs:      250,
		PollIntervalSec:      3,
	}
	if err := adapter.SaveSettings(ctx, input); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}
	got, err := adapter.Settings(ctx)
	if err != nil {
		t.Fatalf("Settings() error = %v", err)
	}
	if got.Language != "en" || got.Theme != "light" || got.PlanPromptStyle != "detailed" ||
		got.AutoContext || got.DesktopNotifications == nil || *got.DesktopNotifications ||
		got.MaxInjectKB != 24 || got.WatchDebounceMs != 250 || got.PollIntervalSec != 3 {
		t.Fatalf("Settings()=%+v, want saved settings with AutoContext=false", got)
	}
}

func TestRuntimeAdapterModelMethodsUseJSONRPC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	mainModel := config.ModelEntry{Name: "main", APIBase: "https://example.com", APIKey: "sk-main", Model: "main-model"}
	if err := adapter.UpsertModelEntry(ctx, mainModel); err != nil {
		t.Fatalf("UpsertModelEntry(main) error = %v", err)
	}
	if err := adapter.ActivateModel(ctx, "main"); err != nil {
		t.Fatalf("ActivateModel(main) error = %v", err)
	}
	entries, active, err := adapter.ModelEntries(ctx)
	if err != nil {
		t.Fatalf("ModelEntries() error = %v", err)
	}
	if active != "main" || !hasModelEntry(entries, "main", "user") {
		t.Fatalf("entries=%+v active=%q, want active user model main", entries, active)
	}

	secondModel := config.ModelEntry{Name: "second", APIBase: "https://example.org", APIKey: "sk-second", Model: "second-model"}
	if err := adapter.UpsertModelEntry(ctx, secondModel); err != nil {
		t.Fatalf("UpsertModelEntry(second) error = %v", err)
	}
	if err := adapter.DeleteModel(ctx, "second"); err != nil {
		t.Fatalf("DeleteModel(second) error = %v", err)
	}
	entries, _, err = adapter.ModelEntries(ctx)
	if err != nil {
		t.Fatalf("ModelEntries(after delete) error = %v", err)
	}
	if hasModelEntry(entries, "second", "") {
		t.Fatalf("entries(after delete)=%+v, did not expect second", entries)
	}

	t.Setenv("EOS_API_BASE", "https://env.example.com")
	t.Setenv("EOS_API_KEY", "sk-env")
	t.Setenv("EOS_MODEL", "env-model")
	if err := adapter.SyncEnvModel(ctx); err != nil {
		t.Fatalf("SyncEnvModel() error = %v", err)
	}
	entries, active, err = adapter.ModelEntries(ctx)
	if err != nil {
		t.Fatalf("ModelEntries(after sync env) error = %v", err)
	}
	if active != "env" || !hasModelEntry(entries, "env", "env") {
		t.Fatalf("entries(after sync env)=%+v active=%q, want active env model", entries, active)
	}
}

func hasWorkspace(items []coreapi.Workspace, path string, active bool) bool {
	for _, item := range items {
		if item.Path == path && item.Active == active {
			return true
		}
	}
	return false
}

func hasWorkspacePath(items []coreapi.Workspace, path string) bool {
	for _, item := range items {
		if item.Path == path {
			return true
		}
	}
	return false
}

func findMCPEntry(items []config.MCPEntry, name string) (config.MCPEntry, bool) {
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return config.MCPEntry{}, false
}

func containsText(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}

func hasVersion(items []coreapi.VersionItem, id, file string) bool {
	for _, item := range items {
		if item.ID == id && filepath.ToSlash(item.File) == filepath.ToSlash(file) {
			return true
		}
	}
	return false
}

func hasModelEntry(items []config.ModelEntry, name, source string) bool {
	for _, item := range items {
		if item.Name != name {
			continue
		}
		return source == "" || item.Source == source
	}
	return false
}

func findRuleDoc(items []coreapi.RuleDocument, scope string) (coreapi.RuleDocument, bool) {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Scope), scope) {
			return item, true
		}
	}
	return coreapi.RuleDocument{}, false
}

func findMemoryDoc(items []coreapi.MemoryDocument, scope string) (coreapi.MemoryDocument, bool) {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Scope), scope) {
			return item, true
		}
	}
	return coreapi.MemoryDocument{}, false
}

func mustEventNotification(t *testing.T, envelope protocol.Envelope) protocoljsonrpc.Notification {
	t.Helper()
	notification, err := protocoljsonrpc.NewNotification(protocoljsonrpc.NotificationEvent, envelope)
	if err != nil {
		t.Fatalf("NewNotification() error = %v", err)
	}
	return notification
}

func TestRuntimeAdapterLSPMethodsUseJSONRPC(t *testing.T) {
	rt := sharedcore.NewRuntime()
	t.Cleanup(rt.Close)
	adapter := NewRuntimeAdapterFromRuntime(rt)
	ctx := context.Background()

	servers, err := adapter.LSPServers(ctx)
	if err != nil {
		t.Fatalf("LSPServers() error = %v", err)
	}
	if servers == nil {
		servers = []coreapi.LSPServer{}
	}

	diagnostics, err := adapter.LSPDiagnostics(ctx)
	if err != nil {
		t.Fatalf("LSPDiagnostics() error = %v", err)
	}
	if diagnostics == nil {
		diagnostics = []string{}
	}

	summary, err := adapter.LSPDiagnosticsSummary(ctx)
	if err != nil {
		t.Fatalf("LSPDiagnosticsSummary() error = %v", err)
	}
	if summary.Files < 0 {
		t.Fatalf("LSPDiagnosticsSummary()=%+v, want non-negative files", summary)
	}

	md := adapter.LSPDiagnosticsMarkdown(ctx)
	_ = md
}
