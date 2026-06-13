//go:build legacy

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/memory"
	sharedruntime "github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/store"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/toolapi/impl"
	"github.com/dreamSailing/eos/internal/tools"
	gitops "github.com/dreamSailing/eos/internal/tools/git"
	"github.com/dreamSailing/eos/pkg/agentcore"
	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/protocol"
	"github.com/dreamSailing/eos/pkg/sandbox"
)

type legacyEngine struct {
	rt *Runtime
}

func NewLegacyEngine(rt *Runtime) coreapi.Engine {
	return &legacyEngine{rt: rt}
}

func (e *legacyEngine) State() coreapi.StateService {
	return legacyStateService{rt: e.rt}
}

func (e *legacyEngine) Workspaces() coreapi.WorkspaceService {
	return legacyWorkspaceService{rt: e.rt}
}

func (e *legacyEngine) Sessions() coreapi.SessionService {
	return legacySessionService{rt: e.rt}
}

func (e *legacyEngine) MCP() coreapi.MCPService {
	return legacyMCPService{rt: e.rt}
}

func (e *legacyEngine) LSP() coreapi.LSPService {
	return legacyLSPService{rt: e.rt}
}

func (e *legacyEngine) Config() coreapi.ConfigService {
	return legacyConfigService{rt: e.rt}
}

func (e *legacyEngine) Permissions() coreapi.PermissionService {
	return legacyPermissionService{rt: e.rt}
}

func (e *legacyEngine) Extensions() coreapi.ExtensionService {
	return legacyExtensionService{rt: e.rt}
}

func (e *legacyEngine) Context() coreapi.ContextService {
	return legacyContextService{rt: e.rt}
}

func (e *legacyEngine) Usage() coreapi.UsageService {
	return legacyUsageService{rt: e.rt}
}

func (e *legacyEngine) Versions() coreapi.VersionService {
	return legacyVersionService{rt: e.rt}
}

func (e *legacyEngine) Tasks() coreapi.TaskService {
	return legacyTaskService{rt: e.rt}
}

func (e *legacyEngine) Modes() coreapi.ModeService {
	return legacyModeService{rt: e.rt}
}

func (e *legacyEngine) Models() coreapi.ModelService {
	return legacyModelService{rt: e.rt}
}

func (e *legacyEngine) RemoteWorkspaces() coreapi.RemoteWorkspaceService {
	return legacyRemoteWorkspaceService{rt: e.rt}
}

func (e *legacyEngine) Git() coreapi.GitService {
	return legacyGitService{rt: e.rt}
}

func (e *legacyEngine) Insights() coreapi.InsightService {
	return legacyInsightService{rt: e.rt}
}

func (e *legacyEngine) Memory() coreapi.MemoryService {
	return legacyMemoryService{rt: e.rt}
}

func (e *legacyEngine) Roles() coreapi.RoleService {
	return legacyRoleService{rt: e.rt}
}

func (e *legacyEngine) Turns() coreapi.TurnService {
	return legacyTurnService{rt: e.rt}
}

func (e *legacyEngine) Approvals() coreapi.ApprovalService {
	return legacyApprovalService{rt: e.rt}
}

func (e *legacyEngine) Inquiries() coreapi.InquiryService {
	return legacyInquiryService{rt: e.rt}
}

func (e *legacyEngine) Agents() coreapi.AgentService {
	return legacyAgentService{rt: e.rt}
}

func (e *legacyEngine) Tools() coreapi.ToolExecutor {
	return legacyToolExecutor{rt: e.rt}
}

func (e *legacyEngine) ToolTelemetry() coreapi.ToolTelemetryService {
	return legacyToolTelemetryService{rt: e.rt}
}

func (e *legacyEngine) ToolCatalog() coreapi.ToolCatalogService {
	return legacyToolCatalogService{rt: e.rt}
}

func (e *legacyEngine) Events() coreapi.EventSubscriber {
	return legacyEventBus{rt: e.rt}
}

func (e *legacyEngine) Sandbox() coreapi.SandboxService {
	return legacySandboxService{rt: e.rt}
}

func (e *legacyEngine) Diagnostics() coreapi.DiagnosticsService {
	return legacyDiagnosticsService{rt: e.rt}
}

type legacyStateService struct {
	rt *Runtime
}

func (s legacyStateService) Snapshot(context.Context) (coreapi.StateSnapshot, error) {
	if s.rt == nil {
		return coreapi.StateSnapshot{}, unsupported("state/snapshot")
	}
	snapshot := mapRuntimeSnapshot(s.rt.RuntimeSnapshot())
	snapshot.Agents = legacyAgentsSnapshot(s.rt)
	return snapshot, nil
}

type legacyWorkspaceService struct {
	rt *Runtime
}

func (s legacyWorkspaceService) List(context.Context) ([]coreapi.Workspace, error) {
	if s.rt == nil {
		return nil, unsupported("workspace/list")
	}
	return mapWorkspaces(s.rt.ListWorkspaces()), nil
}

func (s legacyWorkspaceService) Default(context.Context) (string, error) {
	if s.rt == nil {
		return "", unsupported("workspace/default")
	}
	return strings.TrimSpace(s.rt.DefaultWorkspacePath()), nil
}

func (s legacyWorkspaceService) Last(context.Context) (string, error) {
	if s.rt == nil {
		return "", unsupported("workspace/last")
	}
	return strings.TrimSpace(s.rt.LastWorkspace()), nil
}

func (s legacyWorkspaceService) ResolveForeground(_ context.Context, req coreapi.ResolveForegroundWorkspaceRequest) (string, error) {
	if s.rt == nil {
		return "", unsupported("workspace/resolve_foreground")
	}
	return s.rt.ResolveForegroundWorkspace(req.Preferred)
}

func (s legacyWorkspaceService) Remember(_ context.Context, req coreapi.RememberWorkspaceRequest) error {
	if s.rt == nil {
		return unsupported("workspace/remember")
	}
	return s.rt.RememberWorkspace(req.Path, req.Foreground)
}

func (s legacyWorkspaceService) Forget(_ context.Context, req coreapi.WorkspacePathRequest) error {
	if s.rt == nil {
		return unsupported("workspace/forget")
	}
	return s.rt.ForgetWorkspace(req.Path)
}

func (s legacyWorkspaceService) Add(_ context.Context, req coreapi.WorkspacePathRequest) error {
	if s.rt == nil {
		return unsupported("workspace/add")
	}
	return s.rt.AddWorkspace(req.Path)
}

func (s legacyWorkspaceService) Remove(_ context.Context, req coreapi.WorkspacePathRequest) error {
	if s.rt == nil {
		return unsupported("workspace/remove")
	}
	return s.rt.RemoveWorkspace(req.Path)
}

func (s legacyWorkspaceService) Use(_ context.Context, req coreapi.WorkspacePathRequest) error {
	if s.rt == nil {
		return unsupported("workspace/use")
	}
	return s.rt.UseWorkspace(req.Path)
}

func (s legacyWorkspaceService) SetForeground(_ context.Context, req coreapi.WorkspacePathRequest) error {
	if s.rt == nil {
		return unsupported("workspace/set_foreground")
	}
	return s.rt.SetForegroundWorkspace(req.Path)
}

func (s legacyWorkspaceService) Trust(_ context.Context, req coreapi.WorkspacePathRequest) error {
	if s.rt == nil {
		return unsupported("workspace/trust")
	}
	return s.rt.TrustWorkspace(req.Path)
}

func (s legacyWorkspaceService) ListWorktrees(context.Context) ([]coreapi.Worktree, error) {
	if s.rt == nil {
		return nil, unsupported("workspace/worktree/list")
	}
	return mapWorktrees(s.rt.ListWorktrees()), nil
}

func (s legacyWorkspaceService) CreateWorktree(_ context.Context, req coreapi.CreateWorktreeRequest) (coreapi.Worktree, error) {
	if s.rt == nil {
		return coreapi.Worktree{}, unsupported("workspace/worktree/create")
	}
	item, err := s.rt.CreateWorktree(req.Name)
	if err != nil {
		return coreapi.Worktree{}, err
	}
	return mapWorktree(item), nil
}

func (s legacyWorkspaceService) RemoveWorktree(_ context.Context, req coreapi.RemoveWorktreeRequest) error {
	if s.rt == nil {
		return unsupported("workspace/worktree/remove")
	}
	return s.rt.RemoveWorktree(req.Path, req.Force)
}

type legacySessionService struct {
	rt *Runtime
}

func (s legacySessionService) Create(_ context.Context, req coreapi.CreateSessionRequest) (coreapi.Session, error) {
	if s.rt == nil {
		return coreapi.Session{}, unsupported("session/create")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = strings.TrimSpace(s.rt.RuntimeSnapshot().ForegroundWorkspace)
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = metadataText(req.Metadata, "title")
	}
	meta, err := s.rt.CreateWorkspaceSession(workspaceRoot, title, mapCoreAPISessionMessages(req.Messages))
	if err != nil {
		return coreapi.Session{}, err
	}
	return mapSessionMeta(meta, workspaceRoot), nil
}

func (s legacySessionService) Resume(_ context.Context, req coreapi.ResumeSessionRequest) (coreapi.Session, error) {
	if s.rt == nil {
		return coreapi.Session{}, unsupported("session/resume")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return coreapi.Session{}, fmt.Errorf("session/resume: session id required")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	if workspaceRoot != "" {
		if err := s.rt.ResumeWorkspaceSession(workspaceRoot, sessionID); err != nil {
			return coreapi.Session{}, err
		}
		return s.findSession(sessionID, workspaceRoot)
	}
	if err := s.rt.ResumeSession(sessionID); err != nil {
		return coreapi.Session{}, err
	}
	if resolved, err := s.rt.ResolveSessionWorkspace(sessionID); err == nil {
		workspaceRoot = resolved
	}
	return s.findSession(sessionID, workspaceRoot)
}

func (s legacySessionService) List(_ context.Context, req coreapi.ListSessionsRequest) ([]coreapi.Session, error) {
	if s.rt == nil {
		return nil, unsupported("session/list")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	if workspaceRoot != "" {
		metas, err := s.rt.ListWorkspaceSessions(workspaceRoot)
		if err != nil {
			return nil, err
		}
		return mapSessionMetas(metas, workspaceRoot), nil
	}
	snapshot := s.rt.RuntimeSnapshot()
	return mapSessionMetas(s.rt.ListSessions(), snapshot.ForegroundWorkspace), nil
}

func (s legacySessionService) Current(_ context.Context, req coreapi.CurrentSessionRequest) (coreapi.Session, error) {
	if s.rt == nil {
		return coreapi.Session{}, unsupported("session/current")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	var (
		sessionID string
		err       error
	)
	if workspaceRoot != "" {
		sessionID, err = s.rt.GetWorkspaceCurrentSession(workspaceRoot)
	} else {
		sessionID, err = s.rt.CurrentSessionID()
		if sessionID != "" {
			if resolved, resolveErr := s.rt.ResolveSessionWorkspace(sessionID); resolveErr == nil {
				workspaceRoot = resolved
			}
		}
	}
	if err != nil {
		return coreapi.Session{}, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return coreapi.Session{WorkspaceRoot: strings.TrimSpace(workspaceRoot)}, nil
	}
	return s.findSession(sessionID, workspaceRoot)
}

func (s legacySessionService) SetCurrent(_ context.Context, req coreapi.SetCurrentSessionRequest) error {
	if s.rt == nil {
		return unsupported("session/set_current")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	sessionID := strings.TrimSpace(req.SessionID)
	if workspaceRoot != "" {
		return s.rt.SetWorkspaceCurrentSession(workspaceRoot, sessionID)
	}
	return s.rt.SetCurrentSession(sessionID)
}

func (s legacySessionService) Delete(_ context.Context, req coreapi.DeleteSessionRequest) error {
	if s.rt == nil {
		return unsupported("session/delete")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return fmt.Errorf("session/delete: session id required")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	if workspaceRoot == "" {
		if resolved, err := s.rt.ResolveSessionWorkspace(sessionID); err == nil {
			workspaceRoot = resolved
		}
	}
	if workspaceRoot != "" {
		return s.rt.DeleteWorkspaceSession(workspaceRoot, sessionID)
	}
	return s.rt.DeleteSession(sessionID)
}

func (s legacySessionService) Rename(_ context.Context, req coreapi.RenameSessionRequest) (coreapi.Session, error) {
	if s.rt == nil {
		return coreapi.Session{}, unsupported("session/rename")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return coreapi.Session{}, fmt.Errorf("session/rename: session id required")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	if workspaceRoot == "" {
		if resolved, err := s.rt.ResolveSessionWorkspace(sessionID); err == nil {
			workspaceRoot = resolved
		}
	}
	if workspaceRoot != "" {
		if err := s.rt.UpdateWorkspaceSessionTitle(workspaceRoot, sessionID, req.Title); err != nil {
			return coreapi.Session{}, err
		}
		return s.findSession(sessionID, workspaceRoot)
	}
	if err := s.rt.UpdateSessionTitle(sessionID, req.Title); err != nil {
		return coreapi.Session{}, err
	}
	return s.findSession(sessionID, workspaceRoot)
}

func (s legacySessionService) LoadMessages(_ context.Context, req coreapi.LoadSessionMessagesRequest) ([]coreapi.SessionMessage, error) {
	if s.rt == nil {
		return nil, unsupported("session/messages/load")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session/messages/load: session id required")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	var (
		items []SessionMessage
		err   error
	)
	if workspaceRoot != "" {
		items, err = s.rt.LoadWorkspaceSessionMessages(workspaceRoot, sessionID)
	} else if resolved, resolveErr := s.rt.ResolveSessionWorkspace(sessionID); resolveErr == nil {
		workspaceRoot = resolved
		items, err = s.rt.LoadWorkspaceSessionMessages(workspaceRoot, sessionID)
	} else {
		items, err = s.rt.LoadSessionMessages(sessionID)
	}
	if err != nil {
		return nil, err
	}
	return mapSessionMessages(items), nil
}

func (s legacySessionService) SaveMessages(_ context.Context, req coreapi.SaveSessionMessagesRequest) (coreapi.Session, error) {
	if s.rt == nil {
		return coreapi.Session{}, unsupported("session/messages/save")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	sessionID := strings.TrimSpace(req.SessionID)
	var (
		savedID string
		err     error
	)
	if workspaceRoot != "" {
		savedID, err = s.rt.SaveWorkspaceSessionMessages(workspaceRoot, sessionID, mapCoreAPISessionMessages(req.Messages))
	} else {
		savedID, err = s.rt.SaveSessionMessages(sessionID, mapCoreAPISessionMessages(req.Messages))
	}
	if err != nil {
		return coreapi.Session{}, err
	}
	if workspaceRoot == "" {
		if resolved, resolveErr := s.rt.ResolveSessionWorkspace(savedID); resolveErr == nil {
			workspaceRoot = resolved
		} else {
			workspaceRoot = strings.TrimSpace(s.rt.RuntimeSnapshot().ForegroundWorkspace)
		}
	}
	return s.findSession(savedID, workspaceRoot)
}

type legacyMCPService struct {
	rt *Runtime
}

func (s legacyMCPService) List(context.Context) ([]coreapi.MCPServer, error) {
	if s.rt == nil {
		return nil, unsupported("mcp/list")
	}
	return mapMCPServers(s.rt.ListMCP()), nil
}

func (s legacyMCPService) Upsert(_ context.Context, req coreapi.UpsertMCPRequest) error {
	if s.rt == nil {
		return unsupported("mcp/upsert")
	}
	return s.rt.UpsertMCPServer(MCPServer{
		Name:                 req.Name,
		Type:                 req.Type,
		Target:               req.Target,
		Command:              req.Command,
		Args:                 append([]string(nil), req.Args...),
		Envs:                 cloneStringMap(req.Envs),
		BaseURL:              req.BaseURL,
		Enabled:              req.Enabled,
		Auth:                 mcpAuthFromCoreAPI(req.Auth),
		ApprovalMode:         req.ApprovalMode,
		ToolApprovalOverride: cloneStringMap(req.ToolApprovalOverride),
	})
}

func (s legacyMCPService) ImportJSON(_ context.Context, req coreapi.ImportMCPJSONRequest) error {
	if s.rt == nil {
		return unsupported("mcp/import_json")
	}
	return s.rt.ImportMCPJSON(req.Raw)
}

func (s legacyMCPService) Delete(_ context.Context, req coreapi.MCPNameRequest) error {
	if s.rt == nil {
		return unsupported("mcp/delete")
	}
	return s.rt.DeleteMCP(req.Name)
}

func (s legacyMCPService) SetEnabled(_ context.Context, req coreapi.SetMCPEnabledRequest) error {
	if s.rt == nil {
		return unsupported("mcp/set_enabled")
	}
	return s.rt.SetMCPEnabled(req.Name, req.Enabled)
}

type legacyLSPService struct {
	rt *Runtime
}

func (s legacyLSPService) List(context.Context) ([]coreapi.LSPServer, error) {
	if s.rt == nil {
		return nil, unsupported("lsp/list")
	}
	return mapLSPServers(s.rt.ListLSP()), nil
}

func (s legacyLSPService) Detect(_ context.Context, req coreapi.LSPLanguageRequest) (string, error) {
	if s.rt == nil {
		return "", unsupported("lsp/detect")
	}
	return s.rt.DetectLSP(req.Language), nil
}

func (s legacyLSPService) Start(_ context.Context, req coreapi.LSPLanguageRequest) (string, error) {
	if s.rt == nil {
		return "", unsupported("lsp/start")
	}
	return s.rt.StartLSP(req.Language), nil
}

func (s legacyLSPService) Diagnostics(context.Context) ([]string, error) {
	if s.rt == nil {
		return nil, unsupported("lsp/diagnostics")
	}
	return append([]string(nil), s.rt.LSPDiagnostics()...), nil
}

func (s legacyLSPService) DiagnosticsSummary(context.Context) (coreapi.LSPDiagnosticsSummary, error) {
	if s.rt == nil {
		return coreapi.LSPDiagnosticsSummary{}, unsupported("lsp/diagnostics/summary")
	}
	return s.rt.LSPDiagnosticsSummary(), nil
}

type legacyConfigService struct {
	rt *Runtime
}

func (s legacyConfigService) GetRules(context.Context) (string, error) {
	if s.rt == nil {
		return "", unsupported("config/rules/get")
	}
	return s.rt.GetRules(), nil
}

func (s legacyConfigService) RulesSnapshot(context.Context) (coreapi.RulesSnapshot, error) {
	if s.rt == nil {
		return coreapi.RulesSnapshot{}, unsupported("config/rules/snapshot")
	}
	return mapRulesSnapshot(s.rt.RulesSnapshot()), nil
}

func (s legacyConfigService) SaveRules(_ context.Context, req coreapi.SaveRulesRequest) error {
	if s.rt == nil {
		return unsupported("config/rules/save")
	}
	return s.rt.SaveRulesScoped(req.Scope, req.Content)
}

func (s legacyConfigService) ResetRules(context.Context) error {
	if s.rt == nil {
		return unsupported("config/rules/reset")
	}
	return s.rt.ResetRules()
}

func (s legacyConfigService) GetSettings(context.Context) (coreapi.Settings, error) {
	if s.rt == nil {
		return coreapi.Settings{}, unsupported("config/settings/get")
	}
	return mapSettings(s.rt.GetSettings()), nil
}

func (s legacyConfigService) SaveSettings(_ context.Context, settings coreapi.Settings) error {
	if s.rt == nil {
		return unsupported("config/settings/save")
	}
	return s.rt.SaveSettings(Settings{
		PlanPromptStyle:      settings.PlanPromptStyle,
		PlanBubbleColor:      settings.PlanBubbleColor,
		AutoContext:          cloneBoolPtr(settings.AutoContext),
		DesktopNotifications: cloneBoolPtr(settings.DesktopNotifications),
		MaxInjectKB:          settings.MaxInjectKB,
		WatchMode:            settings.WatchMode,
		WatchDebounceMs:      settings.WatchDebounceMs,
		PollIntervalSec:      settings.PollIntervalSec,
		Language:             settings.Language,
		Theme:                settings.Theme,
		Trusted:              cloneBoolPtr(settings.Trusted),
		MaxTurnTokens:        settings.MaxTurnTokens,
		MaxSessionTokens:     settings.MaxSessionTokens,
		MidRiskConfirm:       settings.MidRiskConfirm,
	})
}

type legacyPermissionService struct {
	rt *Runtime
}

func (s legacyPermissionService) Snapshot(context.Context) (coreapi.PermissionSnapshot, error) {
	if s.rt == nil {
		return coreapi.PermissionSnapshot{}, unsupported("permission/snapshot")
	}
	return mapPermissionSnapshot(s.rt.PermissionSnapshot()), nil
}

func (s legacyPermissionService) PendingReview(context.Context) (coreapi.PendingReview, error) {
	if s.rt == nil {
		return coreapi.PendingReview{}, unsupported("permission/pending_review")
	}
	return mapPendingReview(s.rt.PendingReview()), nil
}

func (s legacyPermissionService) ClearPendingReview(context.Context) error {
	if s.rt == nil {
		return unsupported("permission/clear_pending_review")
	}
	s.rt.ClearPendingReview()
	return nil
}

func (s legacyPermissionService) SetAccessMode(_ context.Context, req coreapi.SetModeRequest) error {
	if s.rt == nil || s.rt.core == nil {
		return unsupported("permission/access_mode/set")
	}
	s.rt.core.SetAccessMode(req.Mode)
	s.rt.notifyStateChanged(StateTopicSettings, "permission.access_mode")
	return nil
}

func (s legacyPermissionService) SetApprovalMode(_ context.Context, req coreapi.SetModeRequest) error {
	if s.rt == nil || s.rt.core == nil {
		return unsupported("permission/approval_mode/set")
	}
	s.rt.core.SetApprovalMode(req.Mode)
	s.rt.notifyStateChanged(StateTopicSettings, "permission.approval_mode")
	return nil
}

type legacyExtensionService struct {
	rt *Runtime
}

func (s legacyExtensionService) ListSkills(context.Context) ([]coreapi.SkillInfo, error) {
	if s.rt == nil {
		return nil, unsupported("extensions/skills/list")
	}
	return mapSkillInfos(s.rt.ListSkills()), nil
}

func (s legacyExtensionService) ReloadSkills(context.Context) error {
	if s.rt == nil {
		return unsupported("extensions/skills/reload")
	}
	return s.rt.ReloadSkills()
}

func (s legacyExtensionService) SetSkillEnabled(_ context.Context, req coreapi.SetExtensionEnabledRequest) error {
	if s.rt == nil {
		return unsupported("extensions/skill/set_enabled")
	}
	return s.rt.SetSkillEnabled(req.Name, req.Enabled)
}

func (s legacyExtensionService) InvokeSkill(_ context.Context, req coreapi.InvokeSkillRequest) (coreapi.InvokeSkillResult, error) {
	if s.rt == nil {
		return coreapi.InvokeSkillResult{}, unsupported("extensions/skill/invoke")
	}
	invoked, err := s.rt.InvokeSkill(req.Name, req.Arguments)
	return coreapi.InvokeSkillResult{Name: strings.TrimSpace(req.Name), Invoked: invoked}, err
}

func (s legacyExtensionService) ListPlugins(context.Context) ([]coreapi.PluginInfo, error) {
	if s.rt == nil {
		return nil, unsupported("extensions/plugins/list")
	}
	return mapPluginInfos(s.rt.ListPlugins()), nil
}

func (s legacyExtensionService) SetPluginEnabled(_ context.Context, req coreapi.SetExtensionEnabledRequest) error {
	if s.rt == nil {
		return unsupported("extensions/plugin/set_enabled")
	}
	return s.rt.SetPluginEnabled(req.Name, req.Enabled)
}

func (s legacyExtensionService) BrowserStatus(context.Context) (coreapi.BrowserStatus, error) {
	if s.rt == nil {
		return coreapi.BrowserStatus{}, unsupported("browser/status")
	}
	return mapBrowserStatus(s.rt.BrowserStatus()), nil
}

type legacyContextService struct {
	rt *Runtime
}

func (s legacyContextService) Preview(context.Context) ([]string, error) {
	if s.rt == nil {
		return nil, unsupported("context/preview")
	}
	return append([]string(nil), s.rt.ContextPreview()...), nil
}

func (s legacyContextService) Stats(context.Context) (coreapi.ContextStats, error) {
	if s.rt == nil {
		return coreapi.ContextStats{}, unsupported("context/stats")
	}
	messages, tokens := s.rt.ContextStats()
	return coreapi.ContextStats{MessageCount: messages, Estimated: tokens}, nil
}

func (s legacyContextService) WindowTokens(context.Context) (int, error) {
	if s.rt == nil {
		return 0, unsupported("context/window")
	}
	return s.rt.ContextWindowTokens(), nil
}

func (s legacyContextService) PinDocument(_ context.Context, req coreapi.PinDocumentRequest) error {
	if s.rt == nil {
		return unsupported("context/pin")
	}
	return s.rt.PinContextDocument(req.ID, req.Content, req.TokenBudget)
}

func (s legacyContextService) Compact(context.Context) (string, error) {
	if s.rt == nil {
		return "", unsupported("context/compact")
	}
	return strings.TrimSpace(s.rt.CompactContext()), nil
}

func (s legacyContextService) Clear(context.Context) error {
	if s.rt == nil {
		return unsupported("context/clear")
	}
	s.rt.ClearContext()
	return nil
}

func (s legacyContextService) Export(_ context.Context, req coreapi.ExportContextRequest) error {
	if s.rt == nil {
		return unsupported("context/export")
	}
	return s.rt.ExportContext(req.Path)
}

type legacyUsageService struct {
	rt *Runtime
}

func (s legacyUsageService) Summary(context.Context) (coreapi.UsageSummary, error) {
	if s.rt == nil {
		return coreapi.UsageSummary{}, unsupported("usage/summary")
	}
	return mapUsageSummary(s.rt.UsageSummary()), nil
}

func (s legacyUsageService) CostSummary(context.Context) (string, error) {
	if s.rt == nil {
		return "", unsupported("usage/cost_summary")
	}
	return strings.TrimSpace(s.rt.CostSummary()), nil
}

func (s legacyUsageService) CostItems(context.Context) ([]coreapi.CostItem, error) {
	if s.rt == nil {
		return nil, unsupported("usage/cost_items")
	}
	return mapCostItems(s.rt.CostItems()), nil
}

type legacyVersionService struct {
	rt *Runtime
}

func (s legacyVersionService) List(context.Context) ([]coreapi.VersionItem, error) {
	if s.rt == nil {
		return nil, unsupported("versions/list")
	}
	return mapVersionItems(s.rt.ListVersions()), nil
}

func (s legacyVersionService) Rollback(_ context.Context, req coreapi.VersionIDRequest) error {
	if s.rt == nil {
		return unsupported("versions/rollback")
	}
	return s.rt.RollbackVersion(req.ID)
}

func (s legacyVersionService) Delete(_ context.Context, req coreapi.VersionIDRequest) error {
	if s.rt == nil {
		return unsupported("versions/delete")
	}
	return s.rt.DeleteVersion(req.ID)
}

func (s legacyVersionService) DeleteFile(_ context.Context, req coreapi.VersionFileRequest) (int, error) {
	if s.rt == nil {
		return 0, unsupported("versions/delete_file")
	}
	return s.rt.DeleteFileVersions(req.File), nil
}

func (s legacyVersionService) Clear(context.Context) (int, error) {
	if s.rt == nil {
		return 0, unsupported("versions/clear")
	}
	return s.rt.ClearVersions(), nil
}

type legacyTaskService struct {
	rt *Runtime
}

func (s legacyTaskService) List(context.Context) ([]coreapi.TaskSnapshot, error) {
	if s.rt == nil {
		return nil, unsupported("task/list")
	}
	return mapTaskSnapshots(s.rt.ListTasks()), nil
}

func (s legacyTaskService) Todos(context.Context) ([]coreapi.TodoItem, error) {
	if s.rt == nil {
		return nil, unsupported("task/todos")
	}
	return mapTodoItems(s.rt.ListTodos()), nil
}

func (s legacyTaskService) Tail(_ context.Context, req coreapi.TaskIDRequest) ([]string, error) {
	if s.rt == nil {
		return nil, unsupported("task/tail")
	}
	return s.rt.TailTask(req.TaskID)
}

func (s legacyTaskService) Kill(_ context.Context, req coreapi.TaskIDRequest) error {
	if s.rt == nil {
		return unsupported("task/kill")
	}
	return s.rt.KillTask(req.TaskID)
}

func (s legacyTaskService) Cleanup(context.Context) (int, error) {
	if s.rt == nil {
		return 0, unsupported("task/cleanup")
	}
	return s.rt.CleanupTasks(), nil
}

type legacyModeService struct {
	rt *Runtime
}

func (s legacyModeService) Snapshot(context.Context) (coreapi.ModeSnapshot, error) {
	if s.rt == nil {
		return coreapi.ModeSnapshot{}, unsupported("runtime/modes/get")
	}
	return coreapi.ModeSnapshot{
		ExecutionMode:  strings.TrimSpace(s.rt.ExecutionMode()),
		SandboxMode:    strings.TrimSpace(s.rt.SandboxMode()),
		ReasoningLevel: strings.TrimSpace(s.rt.ReasoningLevel()),
	}, nil
}

func (s legacyModeService) SetExecutionMode(_ context.Context, req coreapi.SetModeRequest) error {
	if s.rt == nil {
		return unsupported("runtime/execution_mode/set")
	}
	s.rt.SetExecutionMode(req.Mode)
	return nil
}

func (s legacyModeService) SetSandboxMode(_ context.Context, req coreapi.SetModeRequest) error {
	if s.rt == nil {
		return unsupported("runtime/sandbox_mode/set")
	}
	s.rt.SetSandboxMode(req.Mode)
	return nil
}

func (s legacyModeService) SetReasoningLevel(_ context.Context, req coreapi.SetModeRequest) error {
	if s.rt == nil {
		return unsupported("runtime/reasoning_level/set")
	}
	return s.rt.SetReasoningLevel(req.Mode)
}

type legacyModelService struct {
	rt *Runtime
}

func (s legacyModelService) List(context.Context) ([]coreapi.ModelConfig, error) {
	if s.rt == nil {
		return nil, unsupported("model/list")
	}
	return mapModelDescriptors(s.rt.ListModelDescriptors()), nil
}

func (s legacyModelService) Catalog(context.Context) (coreapi.ModelCatalogState, error) {
	if s.rt == nil {
		return coreapi.ModelCatalogState{}, unsupported("model/catalog")
	}
	return mapModelCatalog(s.rt.ModelCatalog()), nil
}

func (s legacyModelService) Upsert(_ context.Context, req coreapi.UpsertModelRequest) error {
	if s.rt == nil {
		return unsupported("model/upsert")
	}
	return s.rt.UpsertModel(req.Name, req.APIBase, req.APIKey, req.Model)
}

func (s legacyModelService) Save(_ context.Context, req coreapi.ModelSaveRequest) error {
	if s.rt == nil {
		return unsupported("model/save")
	}
	return s.rt.SaveModel(ModelSaveRequest{
		OriginalName: req.OriginalName,
		Mode:         ModelEditKind(req.Mode),
		ProviderID:   req.ProviderID,
		PresetID:     req.PresetID,
		Name:         req.Name,
		APIKey:       req.APIKey,
		APIBase:      req.APIBase,
		Model:        req.Model,
	})
}

func (s legacyModelService) Delete(_ context.Context, req coreapi.ModelNameRequest) error {
	if s.rt == nil {
		return unsupported("model/delete")
	}
	return s.rt.DeleteModel(req.Name)
}

func (s legacyModelService) Activate(_ context.Context, req coreapi.ModelNameRequest) error {
	if s.rt == nil {
		return unsupported("model/activate")
	}
	return s.rt.ActivateModel(req.Name)
}

func (s legacyModelService) SyncEnv(context.Context) error {
	if s.rt == nil {
		return unsupported("model/sync_env")
	}
	return s.rt.SyncEnvModel()
}

func (s legacyModelService) Context(context.Context, coreapi.ModelContextRequest) (coreapi.ModelContextSnapshot, error) {
	if s.rt == nil {
		return coreapi.ModelContextSnapshot{}, unsupported("model/context")
	}
	for _, desc := range s.rt.ListModelDescriptors() {
		if desc.IsActive {
			return coreapi.ModelContextSnapshot{
				GlobalDefaultName: strings.TrimSpace(desc.Name),
				ResolvedModelName: strings.TrimSpace(desc.Name),
				ResolvedScope:     "global",
			}, nil
		}
	}
	return coreapi.ModelContextSnapshot{}, nil
}

func (s legacyModelService) SetWorkspace(_ context.Context, req coreapi.SetWorkspaceModelRequest) error {
	if s.rt == nil {
		return unsupported("model/workspace/set")
	}
	return s.rt.ActivateModel(req.ModelName)
}

func (s legacyModelService) ClearWorkspace(context.Context, coreapi.ClearWorkspaceModelRequest) error {
	if s.rt == nil {
		return unsupported("model/workspace/clear")
	}
	return nil
}

func (s legacyModelService) SetSession(_ context.Context, req coreapi.SetSessionModelRequest) error {
	if s.rt == nil {
		return unsupported("model/session/set")
	}
	return s.rt.ActivateModel(req.ModelName)
}

func (s legacyModelService) ClearSession(context.Context, coreapi.ClearSessionModelRequest) error {
	if s.rt == nil {
		return unsupported("model/session/clear")
	}
	return nil
}

type legacyRemoteWorkspaceService struct {
	rt *Runtime
}

func (s legacyRemoteWorkspaceService) List(context.Context) ([]coreapi.RemoteWorkspace, error) {
	if s.rt == nil {
		return nil, unsupported("remote_workspace/list")
	}
	return mapRemoteWorkspaces(s.rt.ListRemoteWorkspaces()), nil
}

func (s legacyRemoteWorkspaceService) Open(_ context.Context, req coreapi.RemoteWorkspaceRef) (coreapi.RemoteWorkspace, error) {
	if s.rt == nil {
		return coreapi.RemoteWorkspace{}, unsupported("remote_workspace/open")
	}
	item, err := s.rt.OpenRemoteWorkspace(req.IDOrPath)
	if err != nil {
		return coreapi.RemoteWorkspace{}, err
	}
	return mapRemoteWorkspace(item), nil
}

func (s legacyRemoteWorkspaceService) Forget(_ context.Context, req coreapi.RemoteWorkspaceRef) error {
	if s.rt == nil {
		return unsupported("remote_workspace/forget")
	}
	return s.rt.ForgetRemoteWorkspace(req.IDOrPath)
}

func (s legacyRemoteWorkspaceService) ClearCache(_ context.Context, req coreapi.RemoteWorkspaceRef) error {
	if s.rt == nil {
		return unsupported("remote_workspace/clear_cache")
	}
	return s.rt.ClearRemoteWorkspaceCache(req.IDOrPath)
}

func (s legacyRemoteWorkspaceService) CurrentRepo(context.Context) (coreapi.RemoteRepoState, bool, error) {
	if s.rt == nil {
		return coreapi.RemoteRepoState{}, false, unsupported("remote_repo/current")
	}
	state, ok := s.rt.CurrentRemoteRepo()
	if !ok {
		return coreapi.RemoteRepoState{}, false, nil
	}
	return mapRemoteRepoState(state), true, nil
}

type legacyGitService struct {
	rt *Runtime
}

func (s legacyGitService) Status(_ context.Context, req coreapi.GitStatusRequest) ([]coreapi.GitChange, error) {
	ops, err := s.ops(req.WorkspaceRoot, "git/status")
	if err != nil {
		return nil, err
	}
	changes, err := ops.Status()
	if err != nil {
		return nil, err
	}
	out := make([]coreapi.GitChange, 0, len(changes))
	for _, change := range changes {
		out = append(out, coreapi.GitChange{
			Path:  strings.TrimSpace(change.Path),
			State: strings.TrimSpace(change.State),
		})
	}
	return out, nil
}

func (s legacyGitService) Diff(_ context.Context, req coreapi.GitDiffRequest) (coreapi.GitTextResult, error) {
	ops, err := s.ops(req.WorkspaceRoot, "git/diff")
	if err != nil {
		return coreapi.GitTextResult{}, err
	}
	text, err := ops.Diff(req.Path)
	if err != nil {
		return coreapi.GitTextResult{}, err
	}
	return coreapi.GitTextResult{Text: text}, nil
}

func (s legacyGitService) Branches(_ context.Context, req coreapi.GitBranchesRequest) (coreapi.GitBranchesResult, error) {
	ops, err := s.ops(req.WorkspaceRoot, "git/branches")
	if err != nil {
		return coreapi.GitBranchesResult{}, err
	}
	branches, current, err := ops.BranchList()
	if err != nil {
		return coreapi.GitBranchesResult{}, err
	}
	return coreapi.GitBranchesResult{Current: strings.TrimSpace(current), Branches: branches}, nil
}

func (s legacyGitService) Log(_ context.Context, req coreapi.GitLogRequest) (coreapi.GitLogResult, error) {
	ops, err := s.ops(req.WorkspaceRoot, "git/log")
	if err != nil {
		return coreapi.GitLogResult{}, err
	}
	out, err := ops.Log(req.Limit, req.Oneline, req.Graph, req.All, req.Path)
	if err != nil {
		return coreapi.GitLogResult{}, err
	}
	if out == nil {
		return coreapi.GitLogResult{}, nil
	}
	entries := make([]coreapi.GitLogEntry, 0, len(out.Entries))
	for _, entry := range out.Entries {
		entries = append(entries, coreapi.GitLogEntry{
			Hash:    strings.TrimSpace(entry.Hash),
			Message: strings.TrimSpace(entry.Message),
		})
	}
	return coreapi.GitLogResult{
		Branch:  strings.TrimSpace(out.Branch),
		Entries: entries,
		Text:    out.Text,
	}, nil
}

func (s legacyGitService) Show(_ context.Context, req coreapi.GitShowRequest) (coreapi.GitShowResult, error) {
	ops, err := s.ops(req.WorkspaceRoot, "git/show")
	if err != nil {
		return coreapi.GitShowResult{}, err
	}
	out, err := ops.Show(req.Revision, req.Path)
	if err != nil {
		return coreapi.GitShowResult{}, err
	}
	if out == nil {
		return coreapi.GitShowResult{}, nil
	}
	return coreapi.GitShowResult{
		Branch:   strings.TrimSpace(out.Branch),
		Revision: strings.TrimSpace(out.Revision),
		Text:     out.Text,
	}, nil
}

func (s legacyGitService) ops(workspaceRoot, op string) (*gitops.Ops, error) {
	if s.rt == nil {
		return nil, unsupported(op)
	}
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		root = strings.TrimSpace(s.rt.RuntimeSnapshot().ForegroundWorkspace)
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	return gitops.NewOpsWithRoot(root), nil
}

type legacyInsightService struct {
	rt *Runtime
}

func (s legacyInsightService) PredictNextUserMessage(ctx context.Context, req coreapi.PredictNextUserMessageRequest) (string, error) {
	if s.rt == nil {
		return "", unsupported("insight/predict_next_user_message")
	}
	return s.rt.PredictNextUserMessage(ctx, req.Draft)
}

func (s legacyInsightService) PlanSnapshot(context.Context) (coreapi.PlanSnapshot, error) {
	if s.rt == nil {
		return coreapi.PlanSnapshot{}, unsupported("insight/plan_snapshot")
	}
	return mapPlanSnapshot(s.rt.PlanSnapshot()), nil
}

func (s legacyInsightService) MemorySnapshot(context.Context) (coreapi.MemorySnapshot, error) {
	if s.rt == nil {
		return coreapi.MemorySnapshot{}, unsupported("insight/memory_snapshot")
	}
	return mapMemorySnapshot(s.rt.MemorySnapshot()), nil
}

type legacyMemoryService struct {
	rt *Runtime
}

func (s legacyMemoryService) Snapshot(context.Context) (coreapi.MemorySnapshot, error) {
	if s.rt == nil {
		return coreapi.MemorySnapshot{}, unsupported("memory/snapshot")
	}
	return mapMemorySnapshot(s.rt.MemorySnapshot()), nil
}

func (s legacyMemoryService) Save(_ context.Context, req coreapi.SaveMemoryRequest) error {
	if s.rt == nil {
		return unsupported("memory/save")
	}
	return s.rt.SaveMemory(req.Scope, req.Content)
}

func (s legacyMemoryService) RebuildIndex(context.Context) error {
	if s.rt == nil {
		return unsupported("memory/rebuild_index")
	}
	return s.rt.RebuildMemoryIndex()
}

func (s legacyMemoryService) recordSvc() *memory.Service {
	root := s.rt.workingRoot()
	dir := filepath.Join(root, ".eos", "memory-records")
	fs, err := store.NewReadWriteStore(store.FactoryOption{
		Name: "memory-records",
		Root: dir,
	})
	if err != nil {
		panic(fmt.Sprintf("store: init memory-records backend: %v", err))
	}
	return memory.NewService(fs)
}

func (s legacyMemoryService) RecordAdd(ctx context.Context, req coreapi.AddMemoryRecordRequest) (coreapi.MemoryRecord, error) {
	if s.rt == nil {
		return coreapi.MemoryRecord{}, unsupported("memory/record/add")
	}
	rec := &memory.MemoryRecord{
		Scope:         req.Scope,
		Kind:          req.Kind,
		Content:       req.Content,
		Tags:          req.Tags,
		Source:        req.Source,
		WorkspaceRoot: req.WorkspaceRoot,
		SessionID:     req.SessionID,
	}
	out, err := s.recordSvc().Add(ctx, rec)
	if err != nil {
		return coreapi.MemoryRecord{}, err
	}
	return mapMemoryRecord(out), nil
}

func (s legacyMemoryService) RecordList(ctx context.Context, req coreapi.ListMemoryRecordsRequest) ([]coreapi.MemoryRecord, error) {
	if s.rt == nil {
		return nil, unsupported("memory/record/list")
	}
	records, err := s.recordSvc().List(ctx, memory.Filter{
		Scope: req.Scope,
		Kind:  req.Kind,
		Tags:  req.Tags,
	})
	if err != nil {
		return nil, err
	}
	return mapMemoryRecords(records), nil
}

func (s legacyMemoryService) RecordSearch(ctx context.Context, req coreapi.SearchMemoryRecordsRequest) ([]coreapi.MemoryRecord, error) {
	if s.rt == nil {
		return nil, unsupported("memory/record/search")
	}
	records, err := s.recordSvc().Search(ctx, memory.SearchQuery{
		Keywords: req.Keywords,
		Tags:     req.Tags,
		Scope:    req.Scope,
		Kind:     req.Kind,
	})
	if err != nil {
		return nil, err
	}
	return mapMemoryRecords(records), nil
}

func (s legacyMemoryService) RecordDelete(ctx context.Context, req coreapi.DeleteMemoryRecordRequest) error {
	if s.rt == nil {
		return unsupported("memory/record/delete")
	}
	return s.recordSvc().Delete(ctx, req.ID)
}

func mapMemoryRecord(rec *memory.MemoryRecord) coreapi.MemoryRecord {
	return coreapi.MemoryRecord{
		ID:            rec.ID,
		Scope:         rec.Scope,
		WorkspaceRoot: rec.WorkspaceRoot,
		SessionID:     rec.SessionID,
		Kind:          rec.Kind,
		Content:       rec.Content,
		Tags:          rec.Tags,
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
		Source:        rec.Source,
	}
}

func mapMemoryRecords(records []*memory.MemoryRecord) []coreapi.MemoryRecord {
	out := make([]coreapi.MemoryRecord, 0, len(records))
	for _, r := range records {
		out = append(out, mapMemoryRecord(r))
	}
	return out
}

type legacyRoleService struct {
	rt *Runtime
}

func (s legacyRoleService) List(context.Context) ([]coreapi.RoleConfig, error) {
	registry, err := loadLegacyRoleRegistry(s.rt)
	if err != nil {
		return nil, err
	}
	return mapRoles(registry.List()), nil
}

func (s legacyRoleService) Resolve(_ context.Context, ref coreapi.RoleRef) (coreapi.RoleConfig, error) {
	registry, err := loadLegacyRoleRegistry(s.rt)
	if err != nil {
		return coreapi.RoleConfig{}, err
	}
	role, ok := registry.Resolve(ref.ID)
	if !ok {
		return coreapi.RoleConfig{}, fmt.Errorf("role not found: %s", strings.TrimSpace(ref.ID))
	}
	return mapRole(role), nil
}

func (s legacySessionService) findSession(sessionID, workspaceRoot string) (coreapi.Session, error) {
	sessionID = strings.TrimSpace(sessionID)
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot != "" {
		metas, err := s.rt.ListWorkspaceSessions(workspaceRoot)
		if err != nil {
			return coreapi.Session{}, err
		}
		for _, meta := range metas {
			if strings.TrimSpace(meta.ID) == sessionID {
				return mapSessionMeta(meta, workspaceRoot), nil
			}
		}
	}
	for _, meta := range s.rt.ListSessions() {
		if strings.TrimSpace(meta.ID) == sessionID {
			return mapSessionMeta(meta, workspaceRoot), nil
		}
	}
	return coreapi.Session{}, fmt.Errorf("session %q not found", sessionID)
}

type legacyTurnService struct {
	rt *Runtime
}

func (s legacyTurnService) Start(ctx context.Context, req coreapi.StartTurnRequest) (coreapi.Turn, error) {
	if s.rt == nil {
		return coreapi.Turn{}, unsupported("turn/start")
	}
	if err := ctx.Err(); err != nil {
		return coreapi.Turn{}, err
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID != "" {
		if err := s.rt.ResumeSession(sessionID); err != nil {
			return coreapi.Turn{}, err
		}
	} else if currentID, err := s.rt.CurrentSessionID(); err == nil {
		sessionID = strings.TrimSpace(currentID)
	}
	if sessionID == "" {
		created, err := legacySessionService{s.rt}.Create(ctx, coreapi.CreateSessionRequest{})
		if err != nil {
			return coreapi.Turn{}, fmt.Errorf("turn/start: failed to create session: %w", err)
		}
		sessionID = strings.TrimSpace(created.ID)
	}
	turnID := strings.TrimSpace(req.TurnID)
	if turnID == "" {
		turnID = newCoreRequestID("turn")
	}
	runCtx, cancel := context.WithCancel(context.Background())
	if err := s.rt.registerActiveTurn(turnID, cancel); err != nil {
		cancel()
		return coreapi.Turn{}, err
	}
	now := time.Now()
	turn := coreapi.Turn{
		ID:        turnID,
		SessionID: sessionID,
		Status:    "running",
		StartedAt: now,
		UpdatedAt: now,
	}
	go func() {
		defer s.rt.finishActiveTurn(turnID)
		events, err := s.invokeProtocolTurn(runCtx, req, turnID)
		if err != nil {
			s.rt.publishProtocolEvent(newCoreRequestEvent(protocol.EventTypeRequestFailed, sessionID, sessionID, turnID, map[string]any{
				"error":   err.Error(),
				"input":   strings.TrimSpace(req.Input),
				"summary": err.Error(),
			}))
			return
		}
		for range events {
		}
	}()
	return turn, nil
}

func (s legacyTurnService) invokeProtocolTurn(ctx context.Context, req coreapi.StartTurnRequest, turnID string) (<-chan protocol.Envelope, error) {
	attachments := mapStartTurnAttachments(req.Attachments)
	imagePaths := append([]string(nil), req.ImagePaths...)
	if len(attachments) == 0 {
		return s.rt.invokeProtocolWithImages(ctx, req.Input, imagePaths, turnID)
	}
	effectiveInput, attachmentImagePaths := inputWithAttachments(req.Input, attachments)
	imagePaths = append(attachmentImagePaths, imagePaths...)
	return s.rt.invokeProtocolWithImages(ctx, effectiveInput, imagePaths, turnID)
}

func (s legacyTurnService) Interrupt(_ context.Context, ref coreapi.TurnRef) error {
	if s.rt == nil {
		return unsupported("turn/interrupt")
	}
	turnID := strings.TrimSpace(ref.TurnID)
	if turnID == "" {
		return fmt.Errorf("turn/interrupt: turn id required")
	}
	if !s.rt.cancelActiveTurn(turnID) {
		return fmt.Errorf("turn %q not running", turnID)
	}
	return nil
}

func mapStartTurnAttachments(items []coreapi.Attachment) []Attachment {
	out := make([]Attachment, 0, len(items))
	for _, item := range items {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			continue
		}
		out = append(out, Attachment{
			Name: strings.TrimSpace(item.Name),
			Path: path,
			MIME: strings.TrimSpace(item.MIME),
			Kind: strings.TrimSpace(item.Kind),
		})
	}
	return out
}

type legacyApprovalService struct {
	rt *Runtime
}

func (s legacyApprovalService) Respond(_ context.Context, resp coreapi.ApprovalResponse) error {
	if s.rt == nil {
		return unsupported("approval/respond")
	}
	approvalID := strings.TrimSpace(resp.ApprovalID)
	if approvalID == "" {
		return fmt.Errorf("approval/respond: approval id required")
	}
	approve, err := approvalDecisionAllows(resp.Decision)
	if err != nil {
		return err
	}
	s.rt.ResolveConfirmation(approvalID, approve)
	return nil
}

type legacyInquiryService struct {
	rt *Runtime
}

func (s legacyInquiryService) Respond(_ context.Context, resp coreapi.InquiryResponse) error {
	if s.rt == nil {
		return unsupported("inquiry/respond")
	}
	inquiryID := strings.TrimSpace(resp.InquiryID)
	if inquiryID == "" {
		return fmt.Errorf("inquiry/respond: inquiry id required")
	}
	s.rt.ResolveInquiry(inquiryID, strings.TrimSpace(resp.Option), strings.TrimSpace(resp.Text))
	return nil
}

type legacyAgentService struct {
	rt *Runtime
}

type legacyAgentState struct {
	registry *agentcore.Registry
	mailbox  *agentcore.Mailbox
	runner   *agentcore.Runner
}

var legacyAgentStates sync.Map

func (s legacyAgentService) Spawn(_ context.Context, req coreapi.SpawnAgentRequest) (coreapi.Agent, error) {
	state, err := legacyAgentStateFor(s.rt)
	if err != nil {
		return coreapi.Agent{}, err
	}
	roleID := strings.TrimSpace(req.RoleID)
	if roleID == "" {
		roleID = "senior-dev"
	}
	parentID := strings.TrimSpace(req.ParentAgentID)
	var agent agentcore.Agent
	if parentID == "" {
		agent, err = state.registry.RegisterRootWithTask(roleID, req.Task)
	} else {
		agent, err = state.registry.Spawn(parentID, roleID, req.Task)
	}
	if err != nil {
		return coreapi.Agent{}, err
	}
	mapped := mapAgent(agent)
	publishLegacyAgentEvent(s.rt, protocol.EventTypeAgentStarted, mapped, "spawn", "Agent started")
	return mapped, nil
}

func (s legacyAgentService) SendInput(_ context.Context, req coreapi.AgentInput) error {
	state, err := legacyAgentStateFor(s.rt)
	if err != nil {
		return err
	}
	agentID := strings.TrimSpace(req.AgentID)
	if _, ok := state.registry.Get(agentID); !ok {
		if sharedruntime.DefaultAgentRegistry().SendInput(agentID, req.Input) {
			if snap, ok := sharedruntime.DefaultAgentRegistry().Snapshot(agentID); ok {
				publishLegacyAgentEvent(s.rt, protocol.EventTypeAgentProgress, mapRuntimeAgentSnapshot(snap), "input", "Agent input received")
			}
			return nil
		}
		return fmt.Errorf("agent not found: %s", agentID)
	}
	if err := state.mailbox.Send(agentcore.MailboxMessage{
		FromAgentID: "user",
		ToAgentID:   agentID,
		Body:        req.Input,
	}); err != nil {
		return err
	}
	agent, _ := state.registry.Get(agentID)
	publishLegacyAgentEvent(s.rt, protocol.EventTypeAgentProgress, mapAgent(agent), "input", "Agent input received")
	return nil
}

func (s legacyAgentService) Wait(ctx context.Context, ref coreapi.AgentRef) (coreapi.Agent, error) {
	state, err := legacyAgentStateFor(s.rt)
	if err != nil {
		return coreapi.Agent{}, err
	}
	agentID := strings.TrimSpace(ref.AgentID)
	agent, ok := state.registry.Get(agentID)
	if !ok {
		if snap, found, waitErr := sharedruntime.DefaultAgentRegistry().Wait(ctx, agentID); found {
			return mapRuntimeAgentSnapshot(snap), waitErr
		}
		return coreapi.Agent{}, fmt.Errorf("agent not found: %s", agentID)
	}
	return mapAgent(agent), nil
}

func (s legacyAgentService) Run(ctx context.Context, req coreapi.RunAgentRequest) (coreapi.AgentRunResult, error) {
	state, err := legacyAgentStateFor(s.rt)
	if err != nil {
		return coreapi.AgentRunResult{}, err
	}
	if _, ok := state.registry.Get(strings.TrimSpace(req.AgentID)); !ok {
		if snap, found := sharedruntime.DefaultAgentRegistry().Snapshot(req.AgentID); found {
			if snap.Status != sharedruntime.AgentStatusRunning {
				if !sharedruntime.DefaultAgentRegistry().Resume(req.AgentID, "") {
					return coreapi.AgentRunResult{Agent: mapRuntimeAgentSnapshot(snap), Output: strings.TrimSpace(snap.Result)}, fmt.Errorf("agent not resumable: %s", strings.TrimSpace(req.AgentID))
				}
			}
			snap, _, err = sharedruntime.DefaultAgentRegistry().Wait(ctx, req.AgentID)
			result := coreapi.AgentRunResult{
				Agent:  mapRuntimeAgentSnapshot(snap),
				Output: strings.TrimSpace(snap.Result),
			}
			return result, err
		}
	}
	runner := newLegacyAgentRunner(s.rt, state, req.SessionID)
	result, err := runner.RunOnce(ctx, req.AgentID, req.Options)
	return mapAgentRunResult(result), err
}

func (s legacyAgentService) RunTool(ctx context.Context, req coreapi.AgentToolRequest) (coreapi.AgentToolResult, error) {
	state, err := legacyAgentStateFor(s.rt)
	if err != nil {
		return coreapi.AgentToolResult{}, err
	}
	runner := newLegacyAgentRunnerWithTurn(s.rt, state, req.SessionID, req.TurnID)
	result, err := runner.RunTool(ctx, req.AgentID, req.Name, req.Args)
	return mapAgentToolResult(result), err
}

func (s legacyAgentService) List(_ context.Context, _ coreapi.ListAgentsRequest) ([]coreapi.Agent, error) {
	state, err := legacyAgentStateFor(s.rt)
	if err != nil {
		return nil, err
	}
	_ = state
	return legacyAgentsSnapshot(s.rt), nil
}

func (s legacyAgentService) Close(_ context.Context, ref coreapi.AgentRef) error {
	state, err := legacyAgentStateFor(s.rt)
	if err != nil {
		return err
	}
	agentID := strings.TrimSpace(ref.AgentID)
	agent, err := state.registry.UpdateStatus(agentID, agentcore.AgentCancelled)
	if err == nil {
		publishLegacyAgentEvent(s.rt, protocol.EventTypeAgentCancelled, mapAgent(agent), "close", "Agent cancelled")
		return nil
	}
	if sharedruntime.DefaultAgentRegistry().Close(agentID) {
		return nil
	}
	return err
}

func legacyAgentStateFor(rt *Runtime) (*legacyAgentState, error) {
	if rt == nil {
		return nil, unsupported("agent")
	}
	if cached, ok := legacyAgentStates.Load(rt); ok {
		return cached.(*legacyAgentState), nil
	}
	roles, err := loadLegacyRoleRegistry(rt)
	if err != nil {
		return nil, err
	}
	state := &legacyAgentState{
		registry: agentcore.NewRegistry(roles),
		mailbox:  agentcore.NewMailbox(),
	}
	state.runner = newLegacyAgentRunner(rt, state, "")
	actual, _ := legacyAgentStates.LoadOrStore(rt, state)
	return actual.(*legacyAgentState), nil
}

func legacyAgentsSnapshot(rt *Runtime) []coreapi.Agent {
	seen := map[string]struct{}{}
	out := make([]coreapi.Agent, 0)
	if rt == nil {
		return mapRuntimeAgentSnapshots(sharedruntime.DefaultAgentRegistry().ListSnapshots(), seen)
	}
	cached, ok := legacyAgentStates.Load(rt)
	if ok {
		if state, ok := cached.(*legacyAgentState); ok && state != nil && state.registry != nil {
			for _, agent := range mapAgents(state.registry.List()) {
				if _, exists := seen[agent.ID]; exists {
					continue
				}
				seen[agent.ID] = struct{}{}
				out = append(out, agent)
			}
		}
	}
	out = append(out, mapRuntimeAgentSnapshots(sharedruntime.DefaultAgentRegistry().ListSnapshots(), seen)...)
	return out
}

func mapRuntimeAgentSnapshots(items []sharedruntime.AgentSnapshot, seen map[string]struct{}) []coreapi.Agent {
	out := make([]coreapi.Agent, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if seen != nil {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
		}
		out = append(out, mapRuntimeAgentSnapshot(item))
	}
	return out
}

func mapRuntimeAgentSnapshot(item sharedruntime.AgentSnapshot) coreapi.Agent {
	return coreapi.Agent{
		ID:        strings.TrimSpace(item.ID),
		RoleID:    fallbackText(item.RoleID, item.Name),
		Task:      strings.TrimSpace(item.Task),
		Status:    strings.TrimSpace(string(item.Status)),
		CreatedAt: item.StartedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func newLegacyAgentRunner(rt *Runtime, state *legacyAgentState, sessionID string) *agentcore.Runner {
	return newLegacyAgentRunnerWithTurn(rt, state, sessionID, "")
}

func newLegacyAgentRunnerWithTurn(rt *Runtime, state *legacyAgentState, sessionID string, turnID string) *agentcore.Runner {
	if state == nil {
		return agentcore.NewRunner(nil, nil)
	}
	return agentcore.NewRunner(
		state.registry,
		state.mailbox,
		agentcore.WithModelRunner(coreapi.NewAgentTurnModelRunner(legacyTurnService{rt: rt}, legacyEventBus{rt: rt}).WithSession(sessionID)),
		agentcore.WithToolRunner(coreapi.NewAgentToolRunner(legacyToolExecutor{rt: rt}).WithSession(sessionID, turnID)),
		agentcore.WithAgentEventSink(legacyAgentEventSink{rt: rt}),
	)
}

type legacyAgentEventSink struct {
	rt *Runtime
}

func (s legacyAgentEventSink) PublishAgentEvent(_ context.Context, event agentcore.AgentEvent) error {
	publishLegacyAgentCoreEvent(s.rt, event)
	return nil
}

type legacyToolExecutor struct {
	rt *Runtime
}

func (e legacyToolExecutor) Execute(ctx context.Context, req coreapi.ToolRequest) (coreapi.ToolResult, error) {
	if e.rt == nil {
		return coreapi.ToolResult{}, unsupported("tool/execute")
	}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	switch name {
	case "bash":
		return e.executeBash(ctx, req)
	default:
		if name == "" {
			name = "unknown"
		}
		return coreapi.ToolResult{}, unsupported("tool/execute:" + name)
	}
}

func (e legacyToolExecutor) executeBash(ctx context.Context, req coreapi.ToolRequest) (coreapi.ToolResult, error) {
	command, err := toolCommandArg(req.Args)
	if err != nil {
		return coreapi.ToolResult{}, err
	}
	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		requestID = strings.TrimSpace(req.TurnID)
	}
	if requestID == "" {
		requestID = newCoreRequestID("tool")
	}
	policy, err := legacySandboxService{rt: e.rt}.Policy(ctx, coreapi.SessionRef{SessionID: strings.TrimSpace(req.SessionID)})
	if err != nil {
		return coreapi.ToolResult{}, err
	}
	preflight := sandbox.NewGuardedRunner(nil).Run([]string{"bash", "-lc", command}, policy)
	if preflight.Err != nil {
		output, _ := json.Marshal(map[string]any{
			"stderr":  preflight.Stderr,
			"status":  "error",
			"backend": preflight.Backend,
		})
		return coreapi.ToolResult{
			Name:      "bash",
			RequestID: requestID,
			Status:    "error",
			Display:   preflight.Stderr,
			Output:    output,
			Error:     preflight.Err.Error(),
		}, nil
	}
	startedAt := time.Now()
	events, err := e.rt.runBashProtocol(ctx, command, requestID)
	if err != nil {
		return coreapi.ToolResult{}, err
	}
	var display string
	var execErr string
	for ev := range events {
		switch ev.EventType {
		case protocol.EventTypeTextFinal:
			if text := protocolEventPayloadString(ev.Payload, "text"); text != "" {
				display = text
			}
		case protocol.EventTypeRequestFailed:
			execErr = firstNonEmptyProtocolString(ev.Payload, "error", "summary", "message")
		}
	}
	status := "success"
	if execErr != "" {
		status = "error"
	}
	output, _ := json.Marshal(map[string]any{
		"stdout": display,
		"status": status,
	})
	result := coreapi.ToolResult{
		Name:      "bash",
		RequestID: requestID,
		Status:    status,
		Display:   display,
		Output:    output,
		Error:     execErr,
		Duration:  time.Since(startedAt),
	}
	return result, nil
}

func toolCommandArg(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", fmt.Errorf("tool/execute:bash: command required")
	}
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return "", fmt.Errorf("tool/execute:bash: invalid args: %w", err)
	}
	for _, key := range []string{"command", "input", "cmd"} {
		if value, ok := params[key].(string); ok {
			command := strings.TrimSpace(value)
			if command != "" {
				return command, nil
			}
		}
	}
	return "", fmt.Errorf("tool/execute:bash: command required")
}

type legacyToolTelemetryService struct {
	rt *Runtime
}

func (s legacyToolTelemetryService) Traces(context.Context) ([]coreapi.ToolTrace, error) {
	if s.rt == nil || s.rt.core == nil || s.rt.core.GetTools() == nil {
		return nil, unsupported("tool/traces")
	}
	traces := s.rt.core.GetTools().GetToolTraces()
	out := make([]coreapi.ToolTrace, 0, len(traces))
	for _, trace := range traces {
		out = append(out, coreapi.ToolTrace{
			ID:         strings.TrimSpace(trace.ID),
			Tool:       strings.TrimSpace(trace.Tool),
			StartTime:  trace.StartTime,
			EndTime:    trace.EndTime,
			Duration:   trace.Duration,
			Success:    trace.Success,
			Cached:     trace.Cached,
			RetryCount: trace.RetryCount,
			ParentID:   strings.TrimSpace(trace.ParentID),
		})
	}
	return out, nil
}

func (s legacyToolTelemetryService) Stats(context.Context) ([]coreapi.ToolStat, error) {
	if s.rt == nil || s.rt.core == nil || s.rt.core.GetTools() == nil {
		return nil, unsupported("tool/stats")
	}
	stats := s.rt.core.GetTools().GetToolStats()
	out := make([]coreapi.ToolStat, 0, len(stats))
	for name, stat := range stats {
		if stat == nil {
			continue
		}
		out = append(out, coreapi.ToolStat{
			Tool:          strings.TrimSpace(name),
			TotalCalls:    stat.TotalCalls,
			SuccessCalls:  stat.SuccessCalls,
			FailureCalls:  stat.FailureCalls,
			CachedCalls:   stat.CachedCalls,
			RetriedCalls:  stat.RetriedCalls,
			TotalDuration: stat.TotalDuration,
			AvgDuration:   stat.AvgDuration,
		})
	}
	return out, nil
}

type legacyToolCatalogService struct {
	rt *Runtime
}

func (s legacyToolCatalogService) List(ctx context.Context, req coreapi.ListToolCatalogRequest) ([]coreapi.ToolDefinition, error) {
	if s.rt == nil {
		return nil, unsupported("tool/catalog")
	}
	workspaceRoot := strings.TrimSpace(req.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = strings.TrimSpace(s.rt.RuntimeSnapshot().ForegroundWorkspace)
	}
	if workspaceRoot != "" {
		ctx = tools.WithWorkspaceRoot(ctx, workspaceRoot)
	}
	catalogSvc := impl.NewServices().Catalog()
	defs, err := catalogSvc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]coreapi.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		out = append(out, mapToolDefinition(d))
	}
	return out, nil
}

func mapToolDefinition(d toolapi.ToolDefinition) coreapi.ToolDefinition {
	params := make(map[string]coreapi.ToolParameterInfo, len(d.Params))
	for k, v := range d.Params {
		params[k] = coreapi.ToolParameterInfo{
			Type:     strings.TrimSpace(v.Type),
			Required: v.Required,
			Desc:     strings.TrimSpace(v.Desc),
		}
	}
	examples := make([]coreapi.ToolExample, 0, len(d.Examples))
	for _, ex := range d.Examples {
		examples = append(examples, coreapi.ToolExample{
			Description: strings.TrimSpace(ex.Description),
			Input:       ex.Input,
		})
	}
	return coreapi.ToolDefinition{
		Name:               strings.TrimSpace(d.Name),
		Description:        strings.TrimSpace(d.Description),
		RiskLevel:          string(d.RiskLevel),
		Params:             params,
		Examples:           examples,
		Source:             string(d.Source),
		Category:           strings.TrimSpace(d.Category),
		VisibleIn:          append([]string(nil), d.VisibleIn...),
		ReadOnly:           d.ReadOnly,
		Invocable:          d.Invocable,
		RequiresFullAccess: d.RequiresFullAccess,
		Tags:               append([]string(nil), d.Tags...),
		Metadata:           d.Metadata,
	}
}

func firstNonEmptyProtocolString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := protocolEventPayloadString(payload, key); value != "" {
			return value
		}
	}
	return ""
}

type legacyEventBus struct {
	rt *Runtime
}

func (b legacyEventBus) Subscribe(ctx context.Context, filter coreapi.EventFilter) (<-chan protocol.Envelope, error) {
	if b.rt == nil {
		return nil, unsupported("event/subscribe")
	}
	events, unsubscribe := b.rt.subscribeProtocolEvents(filter, 64)
	out := make(chan protocol.Envelope, 64)
	go func() {
		defer close(out)
		defer unsubscribe()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- ev:
				}
			}
		}
	}()
	return out, nil
}

func (b legacyEventBus) Publish(_ context.Context, ev protocol.Envelope) error {
	if b.rt == nil {
		return unsupported("event/publish")
	}
	b.rt.publishProtocolEvent(ev)
	return nil
}

type legacySandboxService struct {
	rt *Runtime
}

func (s legacySandboxService) Policy(context.Context, coreapi.SessionRef) (sandbox.Policy, error) {
	if s.rt == nil {
		return sandbox.Policy{}, unsupported("sandbox/policy")
	}
	workspaceRoot := strings.TrimSpace(s.rt.RuntimeSnapshot().ForegroundWorkspace)
	accessMode := ""
	if s.rt.core != nil {
		accessMode = s.rt.core.AccessMode()
	}
	return sandbox.Policy{
		Mode:          sandboxModeFromAccessMode(accessMode),
		WorkspaceRoot: workspaceRoot,
		Network:       sandbox.NetworkDeny,
	}.Normalized(), nil
}

func (s legacySandboxService) SetPolicy(_ context.Context, _ coreapi.SessionRef, policy sandbox.Policy) error {
	if s.rt == nil {
		return unsupported("sandbox/set_policy")
	}
	mode := sandbox.NormalizeMode(string(policy.Mode))
	if s.rt.core != nil {
		s.rt.core.SetAccessMode(accessModeFromSandboxMode(mode))
	}
	s.rt.notifyStateChanged(StateTopicSettings, "sandbox_policy")
	return nil
}

func (legacySandboxService) BackendStatus(context.Context) sandbox.BackendStatus {
	return sandbox.DetectBackend()
}

type legacyDiagnosticsService struct {
	rt *Runtime
}

func (s legacyDiagnosticsService) Startup(_ context.Context) (coreapi.StartupDiagnosticsResult, error) {
	result := coreapi.StartupDiagnosticsResult{}
	if s.rt == nil {
		return result, unsupported("diagnostics/startup")
	}
	if binary, err := os.Executable(); err == nil {
		result.BinaryPath = binary
	}
	result.ManifestVersion = "0.1.0"
	result.ProtocolVersion = "v1"
	if storeDir := strings.TrimSpace(os.Getenv("EOS_STORE_DIR")); storeDir != "" {
		result.StoreDir = storeDir
	} else if coreDir := strings.TrimSpace(os.Getenv("EOS_CORE_STORE_DIR")); coreDir != "" {
		result.StoreDir = coreDir
	}
	result.SandboxBackend = string(sandbox.DetectBackend().Backend)
	if result.StoreDir != "" {
		marker := filepath.Join(result.StoreDir, ".migration_complete")
		if _, err := os.Stat(marker); err == nil {
			result.MigrationMarker = "complete"
		} else {
			result.MigrationMarker = "pending"
		}
	}
	result.OS = runtime.GOOS
	result.Arch = runtime.GOARCH
	return result, nil
}

func sandboxModeFromAccessMode(mode string) sandbox.Mode {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "read-only", "readonly", "read_only":
		return sandbox.ModeReadOnly
	case "danger-full-access", "danger_full_access", "full_access", "full-access":
		return sandbox.ModeDangerFullAccess
	default:
		return sandbox.ModeWorkspaceWrite
	}
}

func accessModeFromSandboxMode(mode sandbox.Mode) string {
	switch sandbox.NormalizeMode(string(mode)) {
	case sandbox.ModeReadOnly:
		return "read-only"
	case sandbox.ModeDangerFullAccess:
		return "danger-full-access"
	default:
		return "workspace-write"
	}
}

func mapRuntimeSnapshot(snapshot RuntimeSnapshot) coreapi.StateSnapshot {
	out := coreapi.StateSnapshot{
		ForegroundWorkspace: strings.TrimSpace(snapshot.ForegroundWorkspace),
		Workspaces:          make([]coreapi.WorkspaceSnapshot, 0, len(snapshot.Workspaces)),
		Sessions:            make([]coreapi.SessionSnapshot, 0, len(snapshot.Sessions)),
		Messages:            make([]coreapi.SessionMessage, 0, len(snapshot.Messages)),
		Tasks:               make([]coreapi.TaskSnapshot, 0, len(snapshot.Tasks)),
	}
	for _, workspace := range snapshot.Workspaces {
		out.Workspaces = append(out.Workspaces, coreapi.WorkspaceSnapshot{
			Path:             workspace.Path,
			Name:             workspace.Name,
			Trusted:          workspace.Trusted,
			Active:           workspace.Active,
			SessionCount:     workspace.SessionCount,
			CurrentSessionID: workspace.CurrentSessionID,
		})
	}
	for _, session := range snapshot.Sessions {
		mapped := mapSessionSnapshot(session)
		out.Sessions = append(out.Sessions, mapped)
		if snapshot.CurrentSession != nil && session.ID == snapshot.CurrentSession.ID {
			current := mapped
			out.CurrentSession = &current
		}
	}
	if snapshot.CurrentSession != nil && out.CurrentSession == nil {
		current := mapSessionSnapshot(*snapshot.CurrentSession)
		out.CurrentSession = &current
	}
	for _, msg := range snapshot.Messages {
		out.Messages = append(out.Messages, mapSessionMessage(msg))
	}
	for _, task := range snapshot.Tasks {
		out.Tasks = append(out.Tasks, coreapi.TaskSnapshot{
			ID:        task.ID,
			Status:    task.Status,
			StartedAt: task.StartedAt,
			Label:     task.Label,
			CanKill:   task.CanKill,
			Workspace: task.Workspace,
		})
	}
	return out
}

func mapWorkspaces(items []Workspace) []coreapi.Workspace {
	out := make([]coreapi.Workspace, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.Workspace{
			Path:    strings.TrimSpace(item.Path),
			Trusted: item.Trusted,
			Active:  item.Active,
		})
	}
	return out
}

func mapWorktrees(items []Worktree) []coreapi.Worktree {
	out := make([]coreapi.Worktree, 0, len(items))
	for _, item := range items {
		out = append(out, mapWorktree(item))
	}
	return out
}

func mapWorktree(item Worktree) coreapi.Worktree {
	return coreapi.Worktree{
		Name:      strings.TrimSpace(item.Name),
		Path:      strings.TrimSpace(item.Path),
		Branch:    strings.TrimSpace(item.Branch),
		Head:      strings.TrimSpace(item.Head),
		Active:    item.Active,
		Removable: item.Removable,
	}
}

func mapMCPServers(items []MCPServer) []coreapi.MCPServer {
	out := make([]coreapi.MCPServer, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.MCPServer{
			Name:                 strings.TrimSpace(item.Name),
			Type:                 strings.TrimSpace(item.Type),
			Target:               strings.TrimSpace(item.Target),
			Command:              strings.TrimSpace(item.Command),
			Args:                 append([]string(nil), item.Args...),
			Envs:                 cloneStringMap(item.Envs),
			BaseURL:              strings.TrimSpace(item.BaseURL),
			Enabled:              item.Enabled,
			Auth:                 mcpAuthToCoreAPI(item.Auth),
			ApprovalMode:         strings.TrimSpace(item.ApprovalMode),
			ToolApprovalOverride: cloneStringMap(item.ToolApprovalOverride),
		})
	}
	return out
}

func mcpAuthToCoreAPI(auth *MCPAuth) *coreapi.MCPAuth {
	if auth == nil {
		return nil
	}
	return &coreapi.MCPAuth{
		Type:       strings.TrimSpace(auth.Type),
		Token:      auth.Token,
		Headers:    cloneStringMap(auth.Headers),
		HeadersEnv: cloneStringMap(auth.HeadersEnv),
	}
}

func mcpAuthFromCoreAPI(auth *coreapi.MCPAuth) *MCPAuth {
	if auth == nil {
		return nil
	}
	return &MCPAuth{
		Type:       strings.TrimSpace(auth.Type),
		Token:      auth.Token,
		Headers:    cloneStringMap(auth.Headers),
		HeadersEnv: cloneStringMap(auth.HeadersEnv),
	}
}

func mapLSPServers(items []LSPServer) []coreapi.LSPServer {
	out := make([]coreapi.LSPServer, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.LSPServer{
			Language: strings.TrimSpace(item.Language),
			Status:   strings.TrimSpace(item.Status),
			Command:  strings.TrimSpace(item.Command),
		})
	}
	return out
}

func mapSettings(settings Settings) coreapi.Settings {
	return coreapi.Settings{
		PlanPromptStyle:      strings.TrimSpace(settings.PlanPromptStyle),
		PlanBubbleColor:      strings.TrimSpace(settings.PlanBubbleColor),
		AutoContext:          cloneBoolPtr(settings.AutoContext),
		DesktopNotifications: cloneBoolPtr(settings.DesktopNotifications),
		MaxInjectKB:          settings.MaxInjectKB,
		WatchMode:            strings.TrimSpace(settings.WatchMode),
		WatchDebounceMs:      settings.WatchDebounceMs,
		PollIntervalSec:      settings.PollIntervalSec,
		Language:             strings.TrimSpace(settings.Language),
		Theme:                strings.TrimSpace(settings.Theme),
		Trusted:              cloneBoolPtr(settings.Trusted),
		MaxTurnTokens:        settings.MaxTurnTokens,
		MaxSessionTokens:     settings.MaxSessionTokens,
		MidRiskConfirm:       settings.MidRiskConfirm,
	}
}

func mapRulesSnapshot(snapshot RulesSnapshot) coreapi.RulesSnapshot {
	out := coreapi.RulesSnapshot{
		ActiveRoot: strings.TrimSpace(snapshot.ActiveRoot),
		Documents:  make([]coreapi.RuleDocument, 0, len(snapshot.Documents)),
	}
	for _, doc := range snapshot.Documents {
		out.Documents = append(out.Documents, coreapi.RuleDocument{
			Scope:     strings.TrimSpace(doc.Scope),
			Path:      strings.TrimSpace(doc.Path),
			Content:   doc.Content,
			Exists:    doc.Exists,
			UpdatedAt: doc.UpdatedAt,
		})
	}
	return out
}

func mapPermissionSnapshot(snapshot PermissionSnapshot) coreapi.PermissionSnapshot {
	return coreapi.PermissionSnapshot{
		ExecutionMode:           strings.TrimSpace(snapshot.ExecutionMode),
		AccessMode:              strings.TrimSpace(snapshot.AccessMode),
		ApprovalMode:            strings.TrimSpace(snapshot.ApprovalMode),
		SandboxMode:             strings.TrimSpace(snapshot.SandboxMode),
		AllowAll:                snapshot.AllowAll,
		AllowedCategories:       append([]string(nil), snapshot.AllowedCategories...),
		HasPendingDiff:          snapshot.HasPendingDiff,
		PendingDiffPath:         strings.TrimSpace(snapshot.PendingDiffPath),
		LastAuthorization:       strings.TrimSpace(snapshot.LastAuthorization),
		LastAuthorizationAt:     strings.TrimSpace(snapshot.LastAuthorizationAt),
		LastAuthorizationKind:   strings.TrimSpace(snapshot.LastAuthorizationKind),
		LastAuthorizationNote:   strings.TrimSpace(snapshot.LastAuthorizationNote),
		LastAuthorizationTarget: strings.TrimSpace(snapshot.LastAuthorizationTarget),
	}
}

func mapPendingReview(review PendingReview) coreapi.PendingReview {
	return coreapi.PendingReview{
		Path:    strings.TrimSpace(review.Path),
		Diff:    strings.TrimSpace(review.Diff),
		HasDiff: review.HasDiff,
	}
}

func mapSkillInfos(items []SkillInfo) []coreapi.SkillInfo {
	out := make([]coreapi.SkillInfo, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.SkillInfo{
			Name:                   strings.TrimSpace(item.Name),
			Description:            strings.TrimSpace(item.Description),
			Source:                 strings.TrimSpace(item.Source),
			ArgumentHint:           strings.TrimSpace(item.ArgumentHint),
			Location:               strings.TrimSpace(item.Location),
			BaseDir:                strings.TrimSpace(item.BaseDir),
			AllowedTools:           append([]string(nil), item.AllowedTools...),
			Enabled:                item.Enabled,
			Active:                 item.Active,
			DisableModelInvocation: item.DisableModelInvocation,
			UserInvocable:          item.UserInvocable,
			UserInvocableDefined:   item.UserInvocableDefined,
		})
	}
	return out
}

func mapPluginInfos(items []PluginInfo) []coreapi.PluginInfo {
	out := make([]coreapi.PluginInfo, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.PluginInfo{
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
			Source:      strings.TrimSpace(item.Source),
			Command:     strings.TrimSpace(item.Command),
			Enabled:     item.Enabled,
		})
	}
	return out
}

func mapBrowserStatus(status BrowserStatus) coreapi.BrowserStatus {
	return coreapi.BrowserStatus{
		ServerName:  strings.TrimSpace(status.ServerName),
		Configured:  status.Configured,
		Enabled:     status.Enabled,
		Loaded:      status.Loaded,
		Tools:       status.Tools,
		LastError:   strings.TrimSpace(status.LastError),
		Command:     strings.TrimSpace(status.Command),
		InstallHint: strings.TrimSpace(status.InstallHint),
	}
}

func mapUsageSummary(summary UsageSummary) coreapi.UsageSummary {
	return coreapi.UsageSummary{
		Rounds:             summary.Rounds,
		InputTokens:        cloneIntPtr(summary.InputTokens),
		ReplyTokens:        cloneIntPtr(summary.ReplyTokens),
		CachedInputTokens:  cloneIntPtr(summary.CachedInputTokens),
		TotalTokens:        cloneIntPtr(summary.TotalTokens),
		CostUSD:            cloneFloatPtr(summary.CostUSD),
		UnknownUsageRounds: summary.UnknownUsageRounds,
		UnknownCostRounds:  summary.UnknownCostRounds,
	}
}

func mapCostItems(items []CostItem) []coreapi.CostItem {
	out := make([]coreapi.CostItem, 0, len(items))
	for _, item := range items {
		totalTokens := firstIntPtr(item.TotalTokens, item.Token)
		out = append(out, coreapi.CostItem{
			Time:              item.Time,
			Model:             strings.TrimSpace(item.Model),
			InputTokens:       firstIntPtr(item.InputTokens, item.Input),
			ReplyTokens:       firstIntPtr(item.ReplyTokens, item.Reply),
			CachedInputTokens: cloneIntPtr(item.CachedInputTokens),
			TotalTokens:       totalTokens,
			CostUSD:           cloneFloatPtr(item.CostUSD),
			UsageKnown:        item.UsageKnown || totalTokens != nil,
			CostKnown:         item.CostKnown || item.CostUSD != nil,
		})
	}
	return out
}

func mapVersionItems(items []VersionItem) []coreapi.VersionItem {
	out := make([]coreapi.VersionItem, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.VersionItem{
			ID:        strings.TrimSpace(item.ID),
			File:      strings.TrimSpace(item.File),
			CreatedAt: item.CreatedAt,
			Summary:   strings.TrimSpace(item.Summary),
		})
	}
	return out
}

func mapTaskSnapshots(items []BackgroundTask) []coreapi.TaskSnapshot {
	out := make([]coreapi.TaskSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.TaskSnapshot{
			ID:        strings.TrimSpace(item.ID),
			Kind:      "shell_task",
			Status:    strings.TrimSpace(item.Status),
			StartedAt: item.StartedAt,
			Label:     strings.TrimSpace(item.Label),
			Summary:   strings.TrimSpace(item.Label),
			CanKill:   item.CanKill,
			Workspace: strings.TrimSpace(item.Workspace),
		})
	}
	return out
}

func mapTodoItems(items []TodoItem) []coreapi.TodoItem {
	out := make([]coreapi.TodoItem, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.TodoItem{
			ID:        strings.TrimSpace(item.ID),
			Content:   strings.TrimSpace(item.Content),
			Status:    strings.TrimSpace(item.Status),
			Priority:  item.Priority,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return out
}

func mapModelDescriptors(items []ModelDescriptor) []coreapi.ModelConfig {
	out := make([]coreapi.ModelConfig, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.ModelConfig{
			Name:                    strings.TrimSpace(item.Name),
			APIBase:                 strings.TrimSpace(item.APIBase),
			APIKeyMasked:            maskAPIKey(item.APIKey),
			Model:                   strings.TrimSpace(item.Model),
			Source:                  strings.TrimSpace(item.Source),
			Active:                  item.IsActive,
			SupportsReasoningEffort: item.SupportsReasoningEffort,
			ProviderID:              strings.TrimSpace(item.ProviderID),
			APIType:                 strings.TrimSpace(item.APIType),
			PresetID:                strings.TrimSpace(item.PresetID),
			EditKind:                strings.TrimSpace(string(item.EditKind)),
			CanEdit:                 item.CanEdit,
			CanDelete:               item.CanDelete,
		})
	}
	return out
}

func mapModelCatalog(catalog ModelCatalogState) coreapi.ModelCatalogState {
	out := coreapi.ModelCatalogState{
		Providers:           make([]coreapi.ModelProviderOption, 0, len(catalog.Providers)),
		Presets:             make([]coreapi.ModelPresetOption, 0, len(catalog.Presets)),
		AllowCustomProvider: catalog.AllowCustomProvider,
		AllowCustomModel:    catalog.AllowCustomModel,
	}
	for _, provider := range catalog.Providers {
		out.Providers = append(out.Providers, coreapi.ModelProviderOption{
			ID:              strings.TrimSpace(provider.ID),
			Name:            strings.TrimSpace(provider.Name),
			Website:         strings.TrimSpace(provider.Website),
			APIKeyEnv:       strings.TrimSpace(provider.APIKeyEnv),
			DefaultAPIBase:  strings.TrimSpace(provider.DefaultAPIBase),
			CodePlanAPIBase: strings.TrimSpace(provider.CodePlanAPIBase),
			ClaudeAPIBase:   strings.TrimSpace(provider.ClaudeAPIBase),
			HasCodePlan:     provider.HasCodePlan,
			HasClaudeCode:   provider.HasClaudeCode,
			DefaultModels:   append([]string(nil), provider.DefaultModels...),
		})
	}
	for _, preset := range catalog.Presets {
		out.Presets = append(out.Presets, coreapi.ModelPresetOption{
			ID:                      strings.TrimSpace(preset.ID),
			Name:                    strings.TrimSpace(preset.Name),
			ProviderID:              strings.TrimSpace(preset.ProviderID),
			ModelName:               strings.TrimSpace(preset.ModelName),
			APIType:                 strings.TrimSpace(preset.APIType),
			ContextWindow:           preset.ContextWindow,
			Tags:                    append([]string(nil), preset.Tags...),
			Description:             strings.TrimSpace(preset.Description),
			SupportsReasoningEffort: preset.SupportsReasoningEffort,
		})
	}
	return out
}

func mapRemoteWorkspaces(items []RemoteWorkspace) []coreapi.RemoteWorkspace {
	out := make([]coreapi.RemoteWorkspace, 0, len(items))
	for _, item := range items {
		out = append(out, mapRemoteWorkspace(item))
	}
	return out
}

func mapRemoteWorkspace(item RemoteWorkspace) coreapi.RemoteWorkspace {
	return coreapi.RemoteWorkspace{
		ID:            strings.TrimSpace(item.ID),
		Kind:          strings.TrimSpace(item.Kind),
		Platform:      strings.TrimSpace(item.Platform),
		RepoURL:       strings.TrimSpace(item.RepoURL),
		Owner:         strings.TrimSpace(item.Owner),
		Repo:          strings.TrimSpace(item.Repo),
		DefaultBranch: strings.TrimSpace(item.DefaultBranch),
		Branch:        strings.TrimSpace(item.Branch),
		Account:       strings.TrimSpace(item.Account),
		LocalPath:     strings.TrimSpace(item.LocalPath),
		Active:        item.Active,
		Exists:        item.Exists,
		LastUsedAt:    item.LastUsedAt,
	}
}

func mapRemoteRepoState(state RemoteRepoState) coreapi.RemoteRepoState {
	return coreapi.RemoteRepoState{
		Mode:          strings.TrimSpace(state.Mode),
		Platform:      strings.TrimSpace(state.Platform),
		RepoURL:       strings.TrimSpace(state.RepoURL),
		Owner:         strings.TrimSpace(state.Owner),
		Repo:          strings.TrimSpace(state.Repo),
		DefaultBranch: strings.TrimSpace(state.DefaultBranch),
		WorkingBranch: strings.TrimSpace(state.WorkingBranch),
		LocalPath:     strings.TrimSpace(state.LocalPath),
		AccountLogin:  strings.TrimSpace(state.AccountLogin),
		AccountName:   strings.TrimSpace(state.AccountName),
		UpdatedAt:     state.UpdatedAt,
	}
}

func mapPlanSnapshot(snapshot PlanSnapshot) coreapi.PlanSnapshot {
	return coreapi.PlanSnapshot{
		HasPlan:          snapshot.HasPlan,
		Content:          strings.TrimSpace(snapshot.Content),
		WorkspaceCurrent: strings.TrimSpace(snapshot.WorkspaceCurrent),
		UserLatest:       strings.TrimSpace(snapshot.UserLatest),
		UserSnapshot:     strings.TrimSpace(snapshot.UserSnapshot),
		UpdatedAt:        snapshot.UpdatedAt,
	}
}

func mapMemorySnapshot(snapshot MemorySnapshot) coreapi.MemorySnapshot {
	out := coreapi.MemorySnapshot{Documents: make([]coreapi.MemoryDocument, 0, len(snapshot.Documents))}
	for _, doc := range snapshot.Documents {
		out.Documents = append(out.Documents, coreapi.MemoryDocument{
			Scope:     strings.TrimSpace(doc.Scope),
			Path:      strings.TrimSpace(doc.Path),
			Exists:    doc.Exists,
			Content:   doc.Content,
			Summary:   strings.TrimSpace(doc.Summary),
			UpdatedAt: doc.UpdatedAt,
		})
	}
	return out
}

func mapRoles(items []agentcore.Role) []coreapi.RoleConfig {
	out := make([]coreapi.RoleConfig, 0, len(items))
	for _, item := range items {
		out = append(out, mapRole(item))
	}
	return out
}

func mapAgents(items []agentcore.Agent) []coreapi.Agent {
	out := make([]coreapi.Agent, 0, len(items))
	for _, item := range items {
		out = append(out, mapAgent(item))
	}
	return out
}

func mapAgent(agent agentcore.Agent) coreapi.Agent {
	return coreapi.Agent{
		ID:            strings.TrimSpace(agent.ID),
		ParentAgentID: strings.TrimSpace(agent.ParentID),
		RoleID:        strings.TrimSpace(agent.RoleID),
		Task:          strings.TrimSpace(agent.Task),
		Status:        strings.TrimSpace(string(agent.Status)),
		CreatedAt:     agent.CreatedAt,
		UpdatedAt:     agent.UpdatedAt,
	}
}

func mapAgentRunResult(result agentcore.AgentRunResult) coreapi.AgentRunResult {
	return coreapi.AgentRunResult{
		Agent:    mapAgent(result.Agent),
		Role:     mapRole(result.Role),
		Messages: mapAgentMailboxMessages(result.Messages),
		Output:   strings.TrimSpace(result.Output),
	}
}

func mapAgentToolResult(result agentcore.ToolOutput) coreapi.AgentToolResult {
	return coreapi.AgentToolResult{
		Name:    strings.TrimSpace(result.Name),
		Display: strings.TrimSpace(result.Display),
		Output:  append(json.RawMessage(nil), result.Output...),
		Error:   strings.TrimSpace(result.Error),
	}
}

func mapAgentMailboxMessages(items []agentcore.AgentMessage) []coreapi.AgentMailboxMessage {
	out := make([]coreapi.AgentMailboxMessage, 0, len(items))
	for _, item := range items {
		body := strings.TrimSpace(item.Body)
		if body == "" {
			continue
		}
		out = append(out, coreapi.AgentMailboxMessage{
			FromAgentID: strings.TrimSpace(item.FromAgentID),
			Body:        body,
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}

func publishLegacyAgentEvent(rt *Runtime, eventType protocol.EventType, agent coreapi.Agent, action string, message string) {
	if rt == nil || strings.TrimSpace(agent.ID) == "" {
		return
	}
	rt.publishProtocolEvent(protocol.NewEvent(eventType, protocol.EventOptions{
		Source: protocol.SourceAgent,
		Payload: map[string]any{
			"agent_id":        strings.TrimSpace(agent.ID),
			"parent_agent_id": strings.TrimSpace(agent.ParentAgentID),
			"role_id":         strings.TrimSpace(agent.RoleID),
			"task":            strings.TrimSpace(agent.Task),
			"status":          strings.TrimSpace(agent.Status),
			"action":          strings.TrimSpace(action),
			"message":         strings.TrimSpace(message),
		},
	}))
}

func publishLegacyAgentCoreEvent(rt *Runtime, event agentcore.AgentEvent) {
	if rt == nil {
		return
	}
	eventType := protocol.EventTypeAgentProgress
	switch event.Status {
	case agentcore.AgentCompleted:
		eventType = protocol.EventTypeAgentDone
	case agentcore.AgentFailed:
		eventType = protocol.EventTypeAgentFailed
	case agentcore.AgentCancelled:
		eventType = protocol.EventTypeAgentCancelled
	}
	agent := mapAgent(event.Agent)
	message := strings.TrimSpace(event.Message)
	if message == "" {
		message = string(event.Status)
	}
	payload := map[string]any{
		"agent_id":        strings.TrimSpace(agent.ID),
		"parent_agent_id": strings.TrimSpace(agent.ParentAgentID),
		"role_id":         strings.TrimSpace(agent.RoleID),
		"task":            strings.TrimSpace(agent.Task),
		"status":          strings.TrimSpace(agent.Status),
		"message":         message,
	}
	if output := strings.TrimSpace(event.Output); output != "" {
		payload["output"] = output
	}
	if errText := strings.TrimSpace(event.Error); errText != "" {
		payload["error"] = errText
	}
	rt.publishProtocolEvent(protocol.NewEvent(eventType, protocol.EventOptions{
		Source:  protocol.SourceAgent,
		Payload: payload,
	}))
}

func mapRole(role agentcore.Role) coreapi.RoleConfig {
	return coreapi.RoleConfig{
		ID:              strings.TrimSpace(role.ID),
		Description:     strings.TrimSpace(role.Description),
		SystemPrompt:    strings.TrimSpace(role.SystemPrompt),
		PromptFile:      strings.TrimSpace(role.PromptFile),
		AllowedTools:    append([]string(nil), role.AllowedTools...),
		ContextStrategy: strings.TrimSpace(string(role.ContextStrategy)),
		Model:           strings.TrimSpace(role.Model),
		ReasoningEffort: strings.TrimSpace(role.ReasoningEffort),
		LegacyAliases:   append([]string(nil), role.LegacyAliases...),
	}
}

func loadLegacyRoleRegistry(rt *Runtime) (*agentcore.RoleRegistry, error) {
	return agentcore.LoadRoleRegistryWithPaths(agentcore.DefaultRoleConfigPaths(legacyRoleWorkspaceRoot(rt)))
}

func legacyRoleWorkspaceRoot(rt *Runtime) string {
	if rt == nil {
		return ""
	}
	return strings.TrimSpace(rt.workingRoot())
}

func maskAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}

func firstIntPtr(values ...*int) *int {
	for _, value := range values {
		if value != nil {
			return cloneIntPtr(value)
		}
	}
	return nil
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFloatPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func mapSessionSnapshot(session SessionSnapshot) coreapi.SessionSnapshot {
	return coreapi.SessionSnapshot{
		ID:             session.ID,
		WorkspacePath:  session.WorkspacePath,
		Title:          session.Title,
		Preview:        session.Preview,
		UpdatedAt:      session.UpdatedAt,
		Running:        session.Running,
		NeedsAttention: session.NeedsAttention,
		MessageCount:   session.MessageCount,
		PendingPrompts: session.PendingPrompts,
		Active:         session.Active,
	}
}

func mapSessionMessage(msg SessionMessage) coreapi.SessionMessage {
	return coreapi.SessionMessage{
		Role:       msg.Role,
		Type:       msg.Type,
		Content:    msg.Content,
		Time:       msg.Time,
		ImagePaths: append([]string(nil), msg.ImagePaths...),
		Metadata:   cloneSessionMessageMetadata(msg.Metadata),
	}
}

func mapSessionMessages(items []SessionMessage) []coreapi.SessionMessage {
	out := make([]coreapi.SessionMessage, 0, len(items))
	for _, item := range items {
		out = append(out, mapSessionMessage(item))
	}
	return out
}

func mapCoreAPISessionMessages(items []coreapi.SessionMessage) []SessionMessage {
	out := make([]SessionMessage, 0, len(items))
	for _, item := range items {
		out = append(out, SessionMessage{
			Role:       item.Role,
			Type:       item.Type,
			Content:    item.Content,
			Time:       item.Time,
			ImagePaths: append([]string(nil), item.ImagePaths...),
			Metadata:   cloneSessionMessageMetadata(item.Metadata),
		})
	}
	return out
}

func mapSessionMetas(items []SessionMeta, workspaceRoot string) []coreapi.Session {
	out := make([]coreapi.Session, 0, len(items))
	for _, item := range items {
		out = append(out, mapSessionMeta(item, workspaceRoot))
	}
	return out
}

func mapSessionMeta(item SessionMeta, workspaceRoot string) coreapi.Session {
	return coreapi.Session{
		ID:            strings.TrimSpace(item.ID),
		WorkspaceRoot: strings.TrimSpace(workspaceRoot),
		UpdatedAt:     item.SavedAt,
		CreatedAt:     item.SavedAt,
		Metadata: map[string]any{
			"model":   strings.TrimSpace(item.Model),
			"title":   strings.TrimSpace(item.Title),
			"summary": strings.TrimSpace(item.Summary),
			"preview": strings.TrimSpace(item.Preview),
			"rounds":  item.Rounds,
			"tokens":  item.Tokens,
		},
	}
}

func metadataText(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	switch value := metadata[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}

func unsupported(op string) error {
	return fmt.Errorf("%s: %w", op, coreapi.ErrUnsupported)
}

var _ coreapi.Engine = (*legacyEngine)(nil)
var _ coreapi.StateService = legacyStateService{}
var _ coreapi.WorkspaceService = legacyWorkspaceService{}
var _ coreapi.SessionService = legacySessionService{}
var _ coreapi.MCPService = legacyMCPService{}
var _ coreapi.LSPService = legacyLSPService{}
var _ coreapi.ConfigService = legacyConfigService{}
var _ coreapi.PermissionService = legacyPermissionService{}
var _ coreapi.ExtensionService = legacyExtensionService{}
var _ coreapi.ContextService = legacyContextService{}
var _ coreapi.UsageService = legacyUsageService{}
var _ coreapi.VersionService = legacyVersionService{}
var _ coreapi.TaskService = legacyTaskService{}
var _ coreapi.ModeService = legacyModeService{}
var _ coreapi.ModelService = legacyModelService{}
var _ coreapi.RemoteWorkspaceService = legacyRemoteWorkspaceService{}
var _ coreapi.GitService = legacyGitService{}
var _ coreapi.InsightService = legacyInsightService{}
var _ coreapi.RoleService = legacyRoleService{}
var _ coreapi.TurnService = legacyTurnService{}
var _ coreapi.ApprovalService = legacyApprovalService{}
var _ coreapi.InquiryService = legacyInquiryService{}
var _ coreapi.AgentService = legacyAgentService{}
var _ coreapi.ToolExecutor = legacyToolExecutor{}
var _ coreapi.ToolCatalogService = legacyToolCatalogService{}
var _ coreapi.ToolTelemetryService = legacyToolTelemetryService{}
var _ coreapi.EventBus = legacyEventBus{}
var _ coreapi.EventPublisher = legacyEventBus{}
var _ coreapi.SandboxService = legacySandboxService{}

func approvalDecisionAllows(decision string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(decision))
	switch normalized {
	case "allow", "allow_once", "allow-session", "allow_session", "approve", "approved", "accept", "accepted", "yes", "true":
		return true, nil
	case "deny", "reject", "rejected", "no", "false":
		return false, nil
	default:
		return false, fmt.Errorf("approval/respond: unsupported decision %q", decision)
	}
}
