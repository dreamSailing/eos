package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/protocol"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
	"github.com/dreamSailing/eos/pkg/sandbox"
)

type fakeEngine struct {
	state       coreapi.StateService
	workspaces  coreapi.WorkspaceService
	sessions    coreapi.SessionService
	mcp         coreapi.MCPService
	lsp         coreapi.LSPService
	config      coreapi.ConfigService
	permissions coreapi.PermissionService
	extensions  coreapi.ExtensionService
	context     coreapi.ContextService
	usage       coreapi.UsageService
	versions    coreapi.VersionService
	tasks       coreapi.TaskService
	modes       coreapi.ModeService
	models      coreapi.ModelService
	remote      coreapi.RemoteWorkspaceService
	git         coreapi.GitService
	insights    coreapi.InsightService
	memory      coreapi.MemoryService
	roles       coreapi.RoleService
	turns       coreapi.TurnService
	approvals   coreapi.ApprovalService
	inquiries   coreapi.InquiryService
	agents      coreapi.AgentService
	tools       coreapi.ToolExecutor
	toolCatalog coreapi.ToolCatalogService
	telemetry   coreapi.ToolTelemetryService
	events      coreapi.EventSubscriber
	sandbox     coreapi.SandboxService
	diagnostics coreapi.DiagnosticsService
}

func (e fakeEngine) State() coreapi.StateService            { return e.state }
func (e fakeEngine) Workspaces() coreapi.WorkspaceService   { return e.workspaces }
func (e fakeEngine) Sessions() coreapi.SessionService       { return e.sessions }
func (e fakeEngine) MCP() coreapi.MCPService                { return e.mcp }
func (e fakeEngine) LSP() coreapi.LSPService                { return e.lsp }
func (e fakeEngine) Config() coreapi.ConfigService          { return e.config }
func (e fakeEngine) Permissions() coreapi.PermissionService { return e.permissions }
func (e fakeEngine) Extensions() coreapi.ExtensionService   { return e.extensions }
func (e fakeEngine) Context() coreapi.ContextService        { return e.context }
func (e fakeEngine) Usage() coreapi.UsageService            { return e.usage }
func (e fakeEngine) Versions() coreapi.VersionService       { return e.versions }
func (e fakeEngine) Tasks() coreapi.TaskService             { return e.tasks }
func (e fakeEngine) Modes() coreapi.ModeService             { return e.modes }
func (e fakeEngine) Models() coreapi.ModelService           { return e.models }
func (e fakeEngine) RemoteWorkspaces() coreapi.RemoteWorkspaceService {
	return e.remote
}
func (e fakeEngine) Git() coreapi.GitService            { return e.git }
func (e fakeEngine) Insights() coreapi.InsightService   { return e.insights }
func (e fakeEngine) Memory() coreapi.MemoryService      { return e.memory }
func (e fakeEngine) Roles() coreapi.RoleService         { return e.roles }
func (e fakeEngine) Turns() coreapi.TurnService         { return e.turns }
func (e fakeEngine) Approvals() coreapi.ApprovalService { return e.approvals }
func (e fakeEngine) Inquiries() coreapi.InquiryService  { return e.inquiries }
func (e fakeEngine) Agents() coreapi.AgentService       { return e.agents }
func (e fakeEngine) Tools() coreapi.ToolExecutor        { return e.tools }
func (e fakeEngine) ToolCatalog() coreapi.ToolCatalogService { return e.toolCatalog }
func (e fakeEngine) ToolTelemetry() coreapi.ToolTelemetryService {
	return e.telemetry
}
func (e fakeEngine) Events() coreapi.EventSubscriber { return e.events }
func (e fakeEngine) Sandbox() coreapi.SandboxService { return e.sandbox }
func (e fakeEngine) Diagnostics() coreapi.DiagnosticsService { return e.diagnostics }

type fakeStateService struct {
	snapshot coreapi.StateSnapshot
	err      error
}

func (s fakeStateService) Snapshot(context.Context, coreapi.StateSnapshotRequest) (coreapi.StateSnapshot, error) {
	return s.snapshot, s.err
}

type fakeWorkspaceService struct {
	items           []coreapi.Workspace
	worktrees       []coreapi.Worktree
	createdWorktree coreapi.Worktree
	defaultPath     string
	lastPath        string
	resolvedPath    string
	err             error
	defaultErr      error
	lastErr         error
	resolveErr      error
	rememberErr     error
	pathErr         error
	worktreeErr     error
	remembered      coreapi.RememberWorkspaceRequest
	forgot          coreapi.WorkspacePathRequest
	added           coreapi.WorkspacePathRequest
	removed         coreapi.WorkspacePathRequest
	used            coreapi.WorkspacePathRequest
	setForeground   coreapi.WorkspacePathRequest
	trusted         coreapi.WorkspacePathRequest
	resolved        coreapi.ResolveForegroundWorkspaceRequest
	createWorktree  coreapi.CreateWorktreeRequest
	removeWorktree  coreapi.RemoveWorktreeRequest
}

func (s *fakeWorkspaceService) List(context.Context, coreapi.WorkspaceListRequest) ([]coreapi.Workspace, error) {
	return s.items, s.err
}
func (s *fakeWorkspaceService) Default(context.Context) (string, error) {
	return s.defaultPath, s.defaultErr
}
func (s *fakeWorkspaceService) Last(context.Context) (string, error) {
	return s.lastPath, s.lastErr
}
func (s *fakeWorkspaceService) ResolveForeground(_ context.Context, req coreapi.ResolveForegroundWorkspaceRequest) (string, error) {
	s.resolved = req
	return s.resolvedPath, s.resolveErr
}
func (s *fakeWorkspaceService) Remember(_ context.Context, req coreapi.RememberWorkspaceRequest) error {
	s.remembered = req
	return s.rememberErr
}
func (s *fakeWorkspaceService) Forget(_ context.Context, req coreapi.WorkspacePathRequest) error {
	s.forgot = req
	return s.pathErr
}
func (s *fakeWorkspaceService) Add(_ context.Context, req coreapi.WorkspacePathRequest) error {
	s.added = req
	return s.pathErr
}
func (s *fakeWorkspaceService) Remove(_ context.Context, req coreapi.WorkspacePathRequest) error {
	s.removed = req
	return s.pathErr
}
func (s *fakeWorkspaceService) Use(_ context.Context, req coreapi.WorkspacePathRequest) error {
	s.used = req
	return s.pathErr
}
func (s *fakeWorkspaceService) SetForeground(_ context.Context, req coreapi.WorkspacePathRequest) error {
	s.setForeground = req
	return s.pathErr
}
func (s *fakeWorkspaceService) Trust(_ context.Context, req coreapi.WorkspacePathRequest) error {
	s.trusted = req
	return s.pathErr
}
func (s *fakeWorkspaceService) ListWorktrees(context.Context) ([]coreapi.Worktree, error) {
	return s.worktrees, s.worktreeErr
}
func (s *fakeWorkspaceService) CreateWorktree(_ context.Context, req coreapi.CreateWorktreeRequest) (coreapi.Worktree, error) {
	s.createWorktree = req
	return s.createdWorktree, s.worktreeErr
}
func (s *fakeWorkspaceService) RemoveWorktree(_ context.Context, req coreapi.RemoveWorktreeRequest) error {
	s.removeWorktree = req
	return s.worktreeErr
}

type fakeSessionService struct {
	items         []coreapi.Session
	err           error
	create        coreapi.Session
	createErr     error
	resume        coreapi.Session
	resumeErr     error
	current       coreapi.Session
	currentErr    error
	rename        coreapi.Session
	renameErr     error
	loadMessages  []coreapi.SessionMessage
	loadErr       error
	saveMessages  coreapi.Session
	saveErr       error
	setCurrentErr error
	deleteErr     error
	seen          coreapi.ListSessionsRequest
	created       coreapi.CreateSessionRequest
	resumed       coreapi.ResumeSessionRequest
	currentReq    coreapi.CurrentSessionRequest
	setCurrent    coreapi.SetCurrentSessionRequest
	deleted       coreapi.DeleteSessionRequest
	renamed       coreapi.RenameSessionRequest
	loaded        coreapi.LoadSessionMessagesRequest
	saved         coreapi.SaveSessionMessagesRequest
}

func (s *fakeSessionService) Create(_ context.Context, req coreapi.CreateSessionRequest) (coreapi.Session, error) {
	s.created = req
	return s.create, s.createErr
}

func (s *fakeSessionService) Resume(_ context.Context, req coreapi.ResumeSessionRequest) (coreapi.Session, error) {
	s.resumed = req
	return s.resume, s.resumeErr
}

func (s *fakeSessionService) List(_ context.Context, req coreapi.ListSessionsRequest) ([]coreapi.Session, error) {
	s.seen = req
	return s.items, s.err
}

func (s *fakeSessionService) Current(_ context.Context, req coreapi.CurrentSessionRequest) (coreapi.Session, error) {
	s.currentReq = req
	return s.current, s.currentErr
}

func (s *fakeSessionService) SetCurrent(_ context.Context, req coreapi.SetCurrentSessionRequest) error {
	s.setCurrent = req
	return s.setCurrentErr
}

func (s *fakeSessionService) Delete(_ context.Context, req coreapi.DeleteSessionRequest) error {
	s.deleted = req
	return s.deleteErr
}

func (s *fakeSessionService) Rename(_ context.Context, req coreapi.RenameSessionRequest) (coreapi.Session, error) {
	s.renamed = req
	return s.rename, s.renameErr
}

func (s *fakeSessionService) SetMeta(_ context.Context, req coreapi.SetSessionMetaRequest) (coreapi.Session, error) {
	s.renamed = coreapi.RenameSessionRequest{SessionID: req.SessionID, Title: req.Key}
	return s.rename, s.renameErr
}

func (s *fakeSessionService) LoadMessages(_ context.Context, req coreapi.LoadSessionMessagesRequest) ([]coreapi.SessionMessage, error) {
	s.loaded = req
	return s.loadMessages, s.loadErr
}

func (s *fakeSessionService) SaveMessages(_ context.Context, req coreapi.SaveSessionMessagesRequest) (coreapi.Session, error) {
	s.saved = req
	return s.saveMessages, s.saveErr
}

type fakeMCPService struct {
	items      []coreapi.MCPServer
	err        error
	upsertErr  error
	importErr  error
	deleteErr  error
	enabledErr error
	upserted   coreapi.UpsertMCPRequest
	imported   coreapi.ImportMCPJSONRequest
	deleted    coreapi.MCPNameRequest
	enabled    coreapi.SetMCPEnabledRequest
}

func (s *fakeMCPService) List(context.Context) ([]coreapi.MCPServer, error) {
	return s.items, s.err
}
func (s *fakeMCPService) Upsert(_ context.Context, req coreapi.UpsertMCPRequest) error {
	s.upserted = req
	return s.upsertErr
}
func (s *fakeMCPService) ImportJSON(_ context.Context, req coreapi.ImportMCPJSONRequest) error {
	s.imported = req
	return s.importErr
}
func (s *fakeMCPService) Delete(_ context.Context, req coreapi.MCPNameRequest) error {
	s.deleted = req
	return s.deleteErr
}
func (s *fakeMCPService) SetEnabled(_ context.Context, req coreapi.SetMCPEnabledRequest) error {
	s.enabled = req
	return s.enabledErr
}

type fakeLSPService struct {
	items            []coreapi.LSPServer
	diagnostics      []string
	diagnosticsSum   coreapi.LSPDiagnosticsSummary
	detectMsg        string
	startMsg         string
	err              error
	detected         coreapi.LSPLanguageRequest
	started          coreapi.LSPLanguageRequest
	summaryCalled    bool
}

func (s *fakeLSPService) List(context.Context) ([]coreapi.LSPServer, error) {
	return s.items, s.err
}
func (s *fakeLSPService) Detect(_ context.Context, req coreapi.LSPLanguageRequest) (string, error) {
	s.detected = req
	return s.detectMsg, s.err
}
func (s *fakeLSPService) Start(_ context.Context, req coreapi.LSPLanguageRequest) (string, error) {
	s.started = req
	return s.startMsg, s.err
}
func (s *fakeLSPService) Diagnostics(context.Context) ([]string, error) {
	return s.diagnostics, s.err
}
func (s *fakeLSPService) DiagnosticsSummary(context.Context) (coreapi.LSPDiagnosticsSummary, error) {
	s.summaryCalled = true
	return s.diagnosticsSum, s.err
}

type fakeConfigService struct {
	rules         string
	rulesSnapshot coreapi.RulesSnapshot
	settings      coreapi.Settings
	err           error
	savedRules    coreapi.SaveRulesRequest
	savedSettings coreapi.Settings
	resetCalled   bool
}

func (s *fakeConfigService) GetRules(context.Context) (string, error) {
	return s.rules, s.err
}
func (s *fakeConfigService) RulesSnapshot(context.Context) (coreapi.RulesSnapshot, error) {
	return s.rulesSnapshot, s.err
}
func (s *fakeConfigService) SaveRules(_ context.Context, req coreapi.SaveRulesRequest) error {
	s.savedRules = req
	return s.err
}
func (s *fakeConfigService) ResetRules(context.Context) error {
	s.resetCalled = true
	return s.err
}
func (s *fakeConfigService) GetSettings(context.Context) (coreapi.Settings, error) {
	return s.settings, s.err
}
func (s *fakeConfigService) SaveSettings(_ context.Context, settings coreapi.Settings) error {
	s.savedSettings = settings
	return s.err
}

type fakePermissionService struct {
	snapshot     coreapi.PermissionSnapshot
	review       coreapi.PendingReview
	err          error
	clearCalled  bool
	accessMode   string
	approvalMode string
}

func (s *fakePermissionService) Snapshot(context.Context) (coreapi.PermissionSnapshot, error) {
	return s.snapshot, s.err
}
func (s *fakePermissionService) PendingReview(context.Context) (coreapi.PendingReview, error) {
	return s.review, s.err
}
func (s *fakePermissionService) ClearPendingReview(context.Context) error {
	s.clearCalled = true
	return s.err
}
func (s *fakePermissionService) SetAccessMode(_ context.Context, req coreapi.SetModeRequest) error {
	s.accessMode = req.Mode
	return s.err
}
func (s *fakePermissionService) SetApprovalMode(_ context.Context, req coreapi.SetModeRequest) error {
	s.approvalMode = req.Mode
	return s.err
}

type fakeExtensionService struct {
	skills        []coreapi.SkillInfo
	plugins       []coreapi.PluginInfo
	browser       coreapi.BrowserStatus
	invokeResult  coreapi.InvokeSkillResult
	invokedSkill  coreapi.InvokeSkillRequest
	err           error
	reloadCalled  bool
	skillEnabled  coreapi.SetExtensionEnabledRequest
	pluginEnabled coreapi.SetExtensionEnabledRequest
}

func (s *fakeExtensionService) ListSkills(context.Context) ([]coreapi.SkillInfo, error) {
	return s.skills, s.err
}
func (s *fakeExtensionService) ReloadSkills(context.Context) error {
	s.reloadCalled = true
	return s.err
}
func (s *fakeExtensionService) SetSkillEnabled(_ context.Context, req coreapi.SetExtensionEnabledRequest) error {
	s.skillEnabled = req
	return s.err
}
func (s *fakeExtensionService) InvokeSkill(_ context.Context, req coreapi.InvokeSkillRequest) (coreapi.InvokeSkillResult, error) {
	s.invokedSkill = req
	return s.invokeResult, s.err
}
func (s *fakeExtensionService) ListPlugins(context.Context) ([]coreapi.PluginInfo, error) {
	return s.plugins, s.err
}
func (s *fakeExtensionService) SetPluginEnabled(_ context.Context, req coreapi.SetExtensionEnabledRequest) error {
	s.pluginEnabled = req
	return s.err
}
func (s *fakeExtensionService) BrowserStatus(context.Context) (coreapi.BrowserStatus, error) {
	return s.browser, s.err
}

type fakeContextService struct {
	preview      []string
	stats        coreapi.ContextStats
	window       int
	pinned       coreapi.PinDocumentRequest
	compactMsg   string
	err          error
	clearCalled  bool
	exportedPath string
}

func (s *fakeContextService) Preview(context.Context) ([]string, error) {
	return s.preview, s.err
}
func (s *fakeContextService) Stats(context.Context) (coreapi.ContextStats, error) {
	return s.stats, s.err
}
func (s *fakeContextService) WindowTokens(context.Context) (int, error) {
	return s.window, s.err
}
func (s *fakeContextService) PinDocument(_ context.Context, req coreapi.PinDocumentRequest) error {
	s.pinned = req
	return s.err
}
func (s *fakeContextService) Compact(context.Context) (string, error) {
	return s.compactMsg, s.err
}
func (s *fakeContextService) Clear(context.Context) error {
	s.clearCalled = true
	return s.err
}
func (s *fakeContextService) Export(_ context.Context, req coreapi.ExportContextRequest) error {
	s.exportedPath = req.Path
	return s.err
}

type fakeUsageService struct {
	summary     coreapi.UsageSummary
	costSummary string
	items       []coreapi.CostItem
	err         error
}

func (s *fakeUsageService) Summary(context.Context) (coreapi.UsageSummary, error) {
	return s.summary, s.err
}
func (s *fakeUsageService) CostSummary(context.Context) (string, error) {
	return s.costSummary, s.err
}
func (s *fakeUsageService) CostItems(context.Context) ([]coreapi.CostItem, error) {
	return s.items, s.err
}

type fakeVersionService struct {
	items           []coreapi.VersionItem
	err             error
	rolledBack      coreapi.VersionIDRequest
	deleted         coreapi.VersionIDRequest
	deletedFile     coreapi.VersionFileRequest
	deleteFileCount int
	clearCount      int
	clearCalled     bool
}

func (s *fakeVersionService) List(context.Context) ([]coreapi.VersionItem, error) {
	return s.items, s.err
}
func (s *fakeVersionService) Rollback(_ context.Context, req coreapi.VersionIDRequest) error {
	s.rolledBack = req
	return s.err
}
func (s *fakeVersionService) Delete(_ context.Context, req coreapi.VersionIDRequest) error {
	s.deleted = req
	return s.err
}
func (s *fakeVersionService) DeleteFile(_ context.Context, req coreapi.VersionFileRequest) (int, error) {
	s.deletedFile = req
	return s.deleteFileCount, s.err
}
func (s *fakeVersionService) Clear(context.Context) (int, error) {
	s.clearCalled = true
	return s.clearCount, s.err
}

type fakeTaskService struct {
	items         []coreapi.TaskSnapshot
	todos         []coreapi.TodoItem
	lines         []string
	err           error
	tailed        coreapi.TaskIDRequest
	killed        coreapi.TaskIDRequest
	cleanupCount  int
	cleanupCalled bool
}

func (s *fakeTaskService) List(context.Context) ([]coreapi.TaskSnapshot, error) {
	return s.items, s.err
}
func (s *fakeTaskService) Todos(context.Context) ([]coreapi.TodoItem, error) {
	return s.todos, s.err
}
func (s *fakeTaskService) Tail(_ context.Context, req coreapi.TaskIDRequest) ([]string, error) {
	s.tailed = req
	return s.lines, s.err
}
func (s *fakeTaskService) Kill(_ context.Context, req coreapi.TaskIDRequest) error {
	s.killed = req
	return s.err
}
func (s *fakeTaskService) Cleanup(context.Context) (int, error) {
	s.cleanupCalled = true
	return s.cleanupCount, s.err
}

type fakeModeService struct {
	snapshot  coreapi.ModeSnapshot
	err       error
	exec      coreapi.SetModeRequest
	sandbox   coreapi.SetModeRequest
	reasoning coreapi.SetModeRequest
}

func (s *fakeModeService) Snapshot(context.Context) (coreapi.ModeSnapshot, error) {
	return s.snapshot, s.err
}
func (s *fakeModeService) SetExecutionMode(_ context.Context, req coreapi.SetModeRequest) error {
	s.exec = req
	return s.err
}
func (s *fakeModeService) SetSandboxMode(_ context.Context, req coreapi.SetModeRequest) error {
	s.sandbox = req
	return s.err
}
func (s *fakeModeService) SetReasoningLevel(_ context.Context, req coreapi.SetModeRequest) error {
	s.reasoning = req
	return s.err
}

type fakeModelService struct {
	items     []coreapi.ModelConfig
	catalog   coreapi.ModelCatalogState
	err       error
	upserted  coreapi.UpsertModelRequest
	saved     coreapi.ModelSaveRequest
	deleted   coreapi.ModelNameRequest
	activated coreapi.ModelNameRequest
	synced    bool
}

func (s *fakeModelService) List(context.Context) ([]coreapi.ModelConfig, error) {
	return s.items, s.err
}
func (s *fakeModelService) Catalog(context.Context) (coreapi.ModelCatalogState, error) {
	return s.catalog, s.err
}
func (s *fakeModelService) Upsert(_ context.Context, req coreapi.UpsertModelRequest) error {
	s.upserted = req
	return s.err
}
func (s *fakeModelService) Save(_ context.Context, req coreapi.ModelSaveRequest) error {
	s.saved = req
	return s.err
}
func (s *fakeModelService) Delete(_ context.Context, req coreapi.ModelNameRequest) error {
	s.deleted = req
	return s.err
}
func (s *fakeModelService) Activate(_ context.Context, req coreapi.ModelNameRequest) error {
	s.activated = req
	return s.err
}
func (s *fakeModelService) SyncEnv(context.Context) error {
	s.synced = true
	return s.err
}
func (s *fakeModelService) Context(context.Context, coreapi.ModelContextRequest) (coreapi.ModelContextSnapshot, error) {
	return coreapi.ModelContextSnapshot{}, s.err
}
func (s *fakeModelService) SetWorkspace(context.Context, coreapi.SetWorkspaceModelRequest) error {
	return s.err
}
func (s *fakeModelService) ClearWorkspace(context.Context, coreapi.ClearWorkspaceModelRequest) error {
	return s.err
}
func (s *fakeModelService) SetSession(context.Context, coreapi.SetSessionModelRequest) error {
	return s.err
}
func (s *fakeModelService) ClearSession(context.Context, coreapi.ClearSessionModelRequest) error {
	return s.err
}

type fakeRemoteWorkspaceService struct {
	items      []coreapi.RemoteWorkspace
	current    coreapi.RemoteRepoState
	currentOK  bool
	err        error
	opened     coreapi.RemoteWorkspaceRef
	forgot     coreapi.RemoteWorkspaceRef
	cleared    coreapi.RemoteWorkspaceRef
	openResult coreapi.RemoteWorkspace
}

func (s *fakeRemoteWorkspaceService) List(context.Context) ([]coreapi.RemoteWorkspace, error) {
	return s.items, s.err
}
func (s *fakeRemoteWorkspaceService) Open(_ context.Context, req coreapi.RemoteWorkspaceRef) (coreapi.RemoteWorkspace, error) {
	s.opened = req
	if strings.TrimSpace(s.openResult.ID) != "" {
		return s.openResult, s.err
	}
	return coreapi.RemoteWorkspace{ID: req.IDOrPath, Active: true, Exists: true}, s.err
}
func (s *fakeRemoteWorkspaceService) Forget(_ context.Context, req coreapi.RemoteWorkspaceRef) error {
	s.forgot = req
	return s.err
}
func (s *fakeRemoteWorkspaceService) ClearCache(_ context.Context, req coreapi.RemoteWorkspaceRef) error {
	s.cleared = req
	return s.err
}
func (s *fakeRemoteWorkspaceService) CurrentRepo(context.Context) (coreapi.RemoteRepoState, bool, error) {
	return s.current, s.currentOK, s.err
}

type fakeGitService struct {
	status      []coreapi.GitChange
	diff        coreapi.GitTextResult
	branches    coreapi.GitBranchesResult
	log         coreapi.GitLogResult
	show        coreapi.GitShowResult
	err         error
	statusReq   coreapi.GitStatusRequest
	diffReq     coreapi.GitDiffRequest
	branchesReq coreapi.GitBranchesRequest
	logReq      coreapi.GitLogRequest
	showReq     coreapi.GitShowRequest
}

func (s *fakeGitService) Status(_ context.Context, req coreapi.GitStatusRequest) ([]coreapi.GitChange, error) {
	s.statusReq = req
	return s.status, s.err
}
func (s *fakeGitService) Diff(_ context.Context, req coreapi.GitDiffRequest) (coreapi.GitTextResult, error) {
	s.diffReq = req
	return s.diff, s.err
}
func (s *fakeGitService) Branches(_ context.Context, req coreapi.GitBranchesRequest) (coreapi.GitBranchesResult, error) {
	s.branchesReq = req
	return s.branches, s.err
}
func (s *fakeGitService) Log(_ context.Context, req coreapi.GitLogRequest) (coreapi.GitLogResult, error) {
	s.logReq = req
	return s.log, s.err
}
func (s *fakeGitService) Show(_ context.Context, req coreapi.GitShowRequest) (coreapi.GitShowResult, error) {
	s.showReq = req
	return s.show, s.err
}

type fakeInsightService struct {
	prediction string
	plan       coreapi.PlanSnapshot
	memory     coreapi.MemorySnapshot
	err        error
	predicted  coreapi.PredictNextUserMessageRequest
}

func (s *fakeInsightService) PredictNextUserMessage(_ context.Context, req coreapi.PredictNextUserMessageRequest) (string, error) {
	s.predicted = req
	return s.prediction, s.err
}
func (s *fakeInsightService) PlanSnapshot(context.Context) (coreapi.PlanSnapshot, error) {
	return s.plan, s.err
}
func (s *fakeInsightService) MemorySnapshot(context.Context) (coreapi.MemorySnapshot, error) {
	return s.memory, s.err
}

type fakeMemoryService struct {
	snapshot      coreapi.MemorySnapshot
	saved         coreapi.SaveMemoryRequest
	rebuildCalled bool
	err           error
}

func (s *fakeMemoryService) Snapshot(context.Context) (coreapi.MemorySnapshot, error) {
	return s.snapshot, s.err
}
func (s *fakeMemoryService) Save(_ context.Context, req coreapi.SaveMemoryRequest) error {
	s.saved = req
	return s.err
}
func (s *fakeMemoryService) RebuildIndex(context.Context) error {
	s.rebuildCalled = true
	return s.err
}
func (s *fakeMemoryService) RecordAdd(_ context.Context, req coreapi.AddMemoryRecordRequest) (coreapi.MemoryRecord, error) {
	return coreapi.MemoryRecord{
		ID:      "fake-id",
		Scope:   req.Scope,
		Kind:    req.Kind,
		Content: req.Content,
	}, s.err
}
func (s *fakeMemoryService) RecordList(context.Context, coreapi.ListMemoryRecordsRequest) ([]coreapi.MemoryRecord, error) {
	return nil, s.err
}
func (s *fakeMemoryService) RecordSearch(context.Context, coreapi.SearchMemoryRecordsRequest) ([]coreapi.MemoryRecord, error) {
	return nil, s.err
}
func (s *fakeMemoryService) RecordDelete(context.Context, coreapi.DeleteMemoryRecordRequest) error {
	return s.err
}

type fakeRoleService struct {
	items    []coreapi.RoleConfig
	resolved coreapi.RoleConfig
	err      error
	seen     coreapi.RoleRef
}

func (s *fakeRoleService) List(context.Context) ([]coreapi.RoleConfig, error) {
	return s.items, s.err
}
func (s *fakeRoleService) Resolve(_ context.Context, ref coreapi.RoleRef) (coreapi.RoleConfig, error) {
	s.seen = ref
	if strings.TrimSpace(s.resolved.ID) != "" {
		return s.resolved, s.err
	}
	for _, item := range s.items {
		if strings.EqualFold(strings.TrimSpace(item.ID), strings.TrimSpace(ref.ID)) {
			return item, s.err
		}
	}
	return coreapi.RoleConfig{}, s.err
}

type fakeAgentService struct {
	spawned coreapi.SpawnAgentRequest
	spawn   coreapi.Agent
	input   coreapi.AgentInput
	waited  coreapi.AgentRef
	wait    coreapi.Agent
	runReq  coreapi.RunAgentRequest
	run     coreapi.AgentRunResult
	toolReq coreapi.AgentToolRequest
	tool    coreapi.AgentToolResult
	listReq coreapi.ListAgentsRequest
	items   []coreapi.Agent
	closed  coreapi.AgentRef
	err     error
}

func (s *fakeAgentService) Spawn(_ context.Context, req coreapi.SpawnAgentRequest) (coreapi.Agent, error) {
	s.spawned = req
	return s.spawn, s.err
}
func (s *fakeAgentService) SendInput(_ context.Context, input coreapi.AgentInput) error {
	s.input = input
	return s.err
}
func (s *fakeAgentService) Wait(_ context.Context, ref coreapi.AgentRef) (coreapi.Agent, error) {
	s.waited = ref
	return s.wait, s.err
}
func (s *fakeAgentService) Run(_ context.Context, req coreapi.RunAgentRequest) (coreapi.AgentRunResult, error) {
	s.runReq = req
	return s.run, s.err
}
func (s *fakeAgentService) RunTool(_ context.Context, req coreapi.AgentToolRequest) (coreapi.AgentToolResult, error) {
	s.toolReq = req
	return s.tool, s.err
}
func (s *fakeAgentService) List(_ context.Context, req coreapi.ListAgentsRequest) ([]coreapi.Agent, error) {
	s.listReq = req
	return s.items, s.err
}
func (s *fakeAgentService) Close(_ context.Context, ref coreapi.AgentRef) error {
	s.closed = ref
	return s.err
}

type fakeApprovalService struct {
	seen coreapi.ApprovalResponse
	err  error
}

func (s *fakeApprovalService) Respond(_ context.Context, resp coreapi.ApprovalResponse) error {
	s.seen = resp
	return s.err
}

type fakeInquiryService struct {
	seen coreapi.InquiryResponse
	err  error
}

func (s *fakeInquiryService) Respond(_ context.Context, resp coreapi.InquiryResponse) error {
	s.seen = resp
	return s.err
}

type fakeTurnService struct {
	started      coreapi.StartTurnRequest
	start        coreapi.Turn
	startErr     error
	interrupted  coreapi.TurnRef
	interruptErr error
}

func (s *fakeTurnService) Start(_ context.Context, req coreapi.StartTurnRequest) (coreapi.Turn, error) {
	s.started = req
	return s.start, s.startErr
}

func (s *fakeTurnService) Interrupt(_ context.Context, ref coreapi.TurnRef) error {
	s.interrupted = ref
	return s.interruptErr
}

type fakeToolExecutor struct {
	seen   coreapi.ToolRequest
	result coreapi.ToolResult
	err    error
}

func (e *fakeToolExecutor) Execute(_ context.Context, req coreapi.ToolRequest) (coreapi.ToolResult, error) {
	e.seen = req
	return e.result, e.err
}

type fakeToolTelemetryService struct {
	traces []coreapi.ToolTrace
	stats  []coreapi.ToolStat
	err    error
}

func (s *fakeToolTelemetryService) Traces(context.Context) ([]coreapi.ToolTrace, error) {
	return s.traces, s.err
}
func (s *fakeToolTelemetryService) Stats(context.Context) ([]coreapi.ToolStat, error) {
	return s.stats, s.err
}

type fakeToolCatalogService struct {
	definitions []coreapi.ToolDefinition
	err         error
}

func (s *fakeToolCatalogService) List(_ context.Context, _ coreapi.ListToolCatalogRequest) ([]coreapi.ToolDefinition, error) {
	return s.definitions, s.err
}

type fakeSandboxService struct {
	policy    sandbox.Policy
	policyErr error
	setErr    error
	status    sandbox.BackendStatus
	seenRef   coreapi.SessionRef
	setRef    coreapi.SessionRef
	setPolicy sandbox.Policy
}

func (s *fakeSandboxService) Policy(_ context.Context, ref coreapi.SessionRef) (sandbox.Policy, error) {
	s.seenRef = ref
	return s.policy, s.policyErr
}
func (s *fakeSandboxService) SetPolicy(_ context.Context, ref coreapi.SessionRef, policy sandbox.Policy) error {
	s.setRef = ref
	s.setPolicy = policy
	return s.setErr
}
func (s *fakeSandboxService) BackendStatus(context.Context) sandbox.BackendStatus {
	return s.status
}

type fakeEvents struct {
	ch   chan protocol.Envelope
	seen coreapi.EventFilter
	err  error
	ctx  context.Context
}

func (e *fakeEvents) Subscribe(ctx context.Context, filter coreapi.EventFilter) (<-chan protocol.Envelope, error) {
	e.ctx = ctx
	e.seen = filter
	return e.ch, e.err
}

type captureNotifier struct {
	ch chan protocoljsonrpc.Notification
}

func (n captureNotifier) Notify(_ context.Context, notification protocoljsonrpc.Notification) error {
	n.ch <- notification
	return nil
}

func TestRegisterInitialize(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	engine := fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}}
	if err := Register(router, engine, Options{ServerName: "test-core"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var result InitializeResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInitialize, nil, &result); err != nil {
		t.Fatalf("Call(initialize) error = %v", err)
	}
	if result.ServerName != "test-core" {
		t.Fatalf("ServerName=%q, want test-core", result.ServerName)
	}
	if !contains(result.Methods, protocoljsonrpc.MethodStateSnapshot) ||
		!contains(result.Methods, protocoljsonrpc.MethodWorkspaceList) ||
		!contains(result.Methods, protocoljsonrpc.MethodWorkspaceRemember) ||
		!contains(result.Methods, protocoljsonrpc.MethodWorkspaceSetForeground) ||
		!contains(result.Methods, protocoljsonrpc.MethodWorkspaceWorktreeList) ||
		!contains(result.Methods, protocoljsonrpc.MethodWorkspaceWorktreeCreate) ||
		!contains(result.Methods, protocoljsonrpc.MethodWorkspaceWorktreeRemove) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionCreate) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionResume) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionList) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionCurrent) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionSetCurrent) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionDelete) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionRename) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionMessagesLoad) ||
		!contains(result.Methods, protocoljsonrpc.MethodSessionMessagesSave) ||
		!contains(result.Methods, protocoljsonrpc.MethodMCPList) ||
		!contains(result.Methods, protocoljsonrpc.MethodMCPUpsert) ||
		!contains(result.Methods, protocoljsonrpc.MethodMCPImportJSON) ||
		!contains(result.Methods, protocoljsonrpc.MethodMCPDelete) ||
		!contains(result.Methods, protocoljsonrpc.MethodMCPSetEnabled) ||
		!contains(result.Methods, protocoljsonrpc.MethodLSPList) ||
		!contains(result.Methods, protocoljsonrpc.MethodLSPDetect) ||
		!contains(result.Methods, protocoljsonrpc.MethodLSPStart) ||
		!contains(result.Methods, protocoljsonrpc.MethodLSPDiagnostics) ||
		!contains(result.Methods, protocoljsonrpc.MethodConfigRulesGet) ||
		!contains(result.Methods, protocoljsonrpc.MethodConfigSettingsGet) ||
		!contains(result.Methods, protocoljsonrpc.MethodPermissionSnapshot) ||
		!contains(result.Methods, protocoljsonrpc.MethodPermissionAccessModeSet) ||
		!contains(result.Methods, protocoljsonrpc.MethodPermissionApprovalModeSet) ||
		!contains(result.Methods, protocoljsonrpc.MethodExtensionsSkillsList) ||
		!contains(result.Methods, protocoljsonrpc.MethodExtensionsPluginsList) ||
		!contains(result.Methods, protocoljsonrpc.MethodGitStatus) ||
		!contains(result.Methods, protocoljsonrpc.MethodGitDiff) ||
		!contains(result.Methods, protocoljsonrpc.MethodGitBranches) ||
		!contains(result.Methods, protocoljsonrpc.MethodGitLog) ||
		!contains(result.Methods, protocoljsonrpc.MethodGitShow) ||
		!contains(result.Methods, protocoljsonrpc.MethodAgentSpawn) ||
		!contains(result.Methods, protocoljsonrpc.MethodAgentInput) ||
		!contains(result.Methods, protocoljsonrpc.MethodAgentWait) ||
		!contains(result.Methods, protocoljsonrpc.MethodAgentRun) ||
		!contains(result.Methods, protocoljsonrpc.MethodAgentToolExecute) ||
		!contains(result.Methods, protocoljsonrpc.MethodAgentList) ||
		!contains(result.Methods, protocoljsonrpc.MethodAgentClose) ||
		!contains(result.Methods, protocoljsonrpc.MethodEventSubscribe) ||
		!contains(result.Methods, protocoljsonrpc.MethodEventUnsubscribe) ||
		!contains(result.Methods, protocoljsonrpc.MethodApprovalRespond) ||
		!contains(result.Methods, protocoljsonrpc.MethodInquiryRespond) ||
		!contains(result.Methods, protocoljsonrpc.MethodTurnStart) ||
		!contains(result.Methods, protocoljsonrpc.MethodTurnInterrupt) ||
		!contains(result.Methods, protocoljsonrpc.MethodToolCatalog) ||
		!contains(result.Methods, protocoljsonrpc.MethodToolExecute) ||
		!contains(result.Methods, protocoljsonrpc.MethodSandboxPolicy) ||
		!contains(result.Methods, protocoljsonrpc.MethodSandboxSetPolicy) ||
		!contains(result.Methods, protocoljsonrpc.MethodSandboxBackend) {
		t.Fatalf("methods=%v, want state/session/agent/approval/turn/tool/sandbox methods", result.Methods)
	}
}

func TestStateSnapshotOverJSONRPC(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	engine := fakeEngine{
		state: fakeStateService{snapshot: coreapi.StateSnapshot{
			ForegroundWorkspace: "C:/work/demo",
			CurrentSession:      &coreapi.SessionSnapshot{ID: "sess_1", Active: true},
		}},
		sessions: &fakeSessionService{},
	}
	if err := Register(router, engine, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var snapshot coreapi.StateSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodStateSnapshot, nil, &snapshot); err != nil {
		t.Fatalf("Call(state/snapshot) error = %v", err)
	}
	if snapshot.ForegroundWorkspace != "C:/work/demo" {
		t.Fatalf("ForegroundWorkspace=%q, want C:/work/demo", snapshot.ForegroundWorkspace)
	}
	if snapshot.CurrentSession == nil || snapshot.CurrentSession.ID != "sess_1" {
		t.Fatalf("CurrentSession=%+v, want sess_1", snapshot.CurrentSession)
	}
}

func TestWorkspaceMethodsOverJSONRPC(t *testing.T) {
	workspaces := &fakeWorkspaceService{
		items:           []coreapi.Workspace{{Path: "C:/work/demo", Trusted: true, Active: true}},
		worktrees:       []coreapi.Worktree{{Name: "wt-a", Path: "C:/work/wt-a", Branch: "wt-a", Removable: true}},
		createdWorktree: coreapi.Worktree{Name: "wt-b", Path: "C:/work/wt-b", Branch: "wt-b", Removable: true},
		defaultPath:     "C:/work/default",
		lastPath:        "C:/work/last",
		resolvedPath:    "C:/work/resolved",
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, workspaces: workspaces, sessions: &fakeSessionService{}}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var listed []coreapi.Workspace
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceList, nil, &listed); err != nil {
		t.Fatalf("Call(workspace/list) error = %v", err)
	}
	if len(listed) != 1 || listed[0].Path != "C:/work/demo" {
		t.Fatalf("listed=%+v, want demo workspace", listed)
	}

	var pathOut struct {
		Path string `json:"path"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceDefault, nil, &pathOut); err != nil {
		t.Fatalf("Call(workspace/default) error = %v", err)
	}
	if pathOut.Path != "C:/work/default" {
		t.Fatalf("default path=%q", pathOut.Path)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceLast, nil, &pathOut); err != nil {
		t.Fatalf("Call(workspace/last) error = %v", err)
	}
	if pathOut.Path != "C:/work/last" {
		t.Fatalf("last path=%q", pathOut.Path)
	}
	resolveReq := coreapi.ResolveForegroundWorkspaceRequest{Preferred: "C:/preferred"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceResolve, resolveReq, &pathOut); err != nil {
		t.Fatalf("Call(workspace/resolve_foreground) error = %v", err)
	}
	if pathOut.Path != "C:/work/resolved" || workspaces.resolved != resolveReq {
		t.Fatalf("resolved path=%q req=%+v", pathOut.Path, workspaces.resolved)
	}

	rememberReq := coreapi.RememberWorkspaceRequest{Path: "C:/work/demo", Foreground: true}
	var ok map[string]bool
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceRemember, rememberReq, &ok); err != nil {
		t.Fatalf("Call(workspace/remember) error = %v", err)
	}
	if workspaces.remembered != rememberReq || !ok["ok"] {
		t.Fatalf("remembered=%+v ok=%v", workspaces.remembered, ok)
	}

	pathReq := coreapi.WorkspacePathRequest{Path: "C:/work/demo"}
	for _, method := range []string{
		protocoljsonrpc.MethodWorkspaceAdd,
		protocoljsonrpc.MethodWorkspaceRemove,
		protocoljsonrpc.MethodWorkspaceUse,
		protocoljsonrpc.MethodWorkspaceSetForeground,
		protocoljsonrpc.MethodWorkspaceTrust,
		protocoljsonrpc.MethodWorkspaceForget,
	} {
		ok = nil
		if err := client.Call(context.Background(), method, pathReq, &ok); err != nil {
			t.Fatalf("Call(%s) error = %v", method, err)
		}
		if !ok["ok"] {
			t.Fatalf("Call(%s) ok=%v, want true", method, ok)
		}
	}
	if workspaces.added != pathReq || workspaces.removed != pathReq || workspaces.used != pathReq ||
		workspaces.setForeground != pathReq || workspaces.trusted != pathReq || workspaces.forgot != pathReq {
		t.Fatalf("workspace path calls not captured: %+v", workspaces)
	}

	var worktreeList []coreapi.Worktree
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceWorktreeList, nil, &worktreeList); err != nil {
		t.Fatalf("Call(workspace/worktree/list) error = %v", err)
	}
	if len(worktreeList) != 1 || worktreeList[0].Name != "wt-a" {
		t.Fatalf("worktreeList=%+v, want wt-a", worktreeList)
	}
	createWorktreeReq := coreapi.CreateWorktreeRequest{Name: "wt-b"}
	var created coreapi.Worktree
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceWorktreeCreate, createWorktreeReq, &created); err != nil {
		t.Fatalf("Call(workspace/worktree/create) error = %v", err)
	}
	if workspaces.createWorktree != createWorktreeReq || created.Name != "wt-b" {
		t.Fatalf("createWorktree=%+v created=%+v, want wt-b", workspaces.createWorktree, created)
	}
	removeWorktreeReq := coreapi.RemoveWorktreeRequest{Path: "C:/work/wt-b", Force: true}
	ok = nil
	if err := client.Call(context.Background(), protocoljsonrpc.MethodWorkspaceWorktreeRemove, removeWorktreeReq, &ok); err != nil {
		t.Fatalf("Call(workspace/worktree/remove) error = %v", err)
	}
	if workspaces.removeWorktree != removeWorktreeReq || !ok["ok"] {
		t.Fatalf("removeWorktree=%+v ok=%v, want request/ok", workspaces.removeWorktree, ok)
	}
}

func TestSessionListOverJSONRPC(t *testing.T) {
	sessions := &fakeSessionService{items: []coreapi.Session{
		{ID: "sess_1", WorkspaceRoot: "C:/work/demo", UpdatedAt: time.Unix(1, 0)},
	}}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: sessions}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out []coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionList, coreapi.ListSessionsRequest{WorkspaceRoot: "C:/work/demo"}, &out); err != nil {
		t.Fatalf("Call(session/list) error = %v", err)
	}
	if sessions.seen.WorkspaceRoot != "C:/work/demo" {
		t.Fatalf("seen workspace=%q, want C:/work/demo", sessions.seen.WorkspaceRoot)
	}
	if len(out) != 1 || out[0].ID != "sess_1" {
		t.Fatalf("sessions=%+v, want sess_1", out)
	}
}

func TestSessionCreateOverJSONRPC(t *testing.T) {
	sessions := &fakeSessionService{create: coreapi.Session{
		ID:            "sess_new",
		WorkspaceRoot: "C:/work/demo",
		UpdatedAt:     time.Unix(2, 0),
		Metadata:      map[string]any{"title": "New thread"},
	}}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: sessions}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out coreapi.Session
	req := coreapi.CreateSessionRequest{
		WorkspaceRoot: "C:/work/demo",
		Title:         "New thread",
		Messages:      []coreapi.SessionMessage{{Role: "user", Type: "text", Content: "hello"}},
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionCreate, req, &out); err != nil {
		t.Fatalf("Call(session/create) error = %v", err)
	}
	if sessions.created.WorkspaceRoot != "C:/work/demo" || sessions.created.Title != "New thread" {
		t.Fatalf("created=%+v, want workspace/title", sessions.created)
	}
	if len(sessions.created.Messages) != 1 || sessions.created.Messages[0].Content != "hello" {
		t.Fatalf("created messages=%+v, want hello", sessions.created.Messages)
	}
	if out.ID != "sess_new" {
		t.Fatalf("out.ID=%q, want sess_new", out.ID)
	}
}

func TestSessionResumeOverJSONRPC(t *testing.T) {
	sessions := &fakeSessionService{resume: coreapi.Session{
		ID:            "sess_existing",
		WorkspaceRoot: "C:/work/demo",
		UpdatedAt:     time.Unix(3, 0),
	}}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: sessions}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out coreapi.Session
	req := coreapi.ResumeSessionRequest{WorkspaceRoot: "C:/work/demo", SessionID: "sess_existing"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionResume, req, &out); err != nil {
		t.Fatalf("Call(session/resume) error = %v", err)
	}
	if sessions.resumed != req {
		t.Fatalf("resumed=%+v, want %+v", sessions.resumed, req)
	}
	if out.ID != "sess_existing" {
		t.Fatalf("out.ID=%q, want sess_existing", out.ID)
	}
}

func TestSessionCurrentSetDeleteRenameOverJSONRPC(t *testing.T) {
	sessions := &fakeSessionService{
		current: coreapi.Session{ID: "sess_current", WorkspaceRoot: "C:/work/demo", UpdatedAt: time.Unix(4, 0)},
		rename:  coreapi.Session{ID: "sess_current", WorkspaceRoot: "C:/work/demo", Metadata: map[string]any{"title": "Renamed"}},
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: sessions}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var current coreapi.Session
	currentReq := coreapi.CurrentSessionRequest{WorkspaceRoot: "C:/work/demo"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionCurrent, currentReq, &current); err != nil {
		t.Fatalf("Call(session/current) error = %v", err)
	}
	if sessions.currentReq != currentReq || current.ID != "sess_current" {
		t.Fatalf("currentReq=%+v current=%+v, want sess_current", sessions.currentReq, current)
	}

	setReq := coreapi.SetCurrentSessionRequest{WorkspaceRoot: "C:/work/demo", SessionID: "sess_current"}
	var ok map[string]bool
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionSetCurrent, setReq, &ok); err != nil {
		t.Fatalf("Call(session/set_current) error = %v", err)
	}
	if sessions.setCurrent != setReq || !ok["ok"] {
		t.Fatalf("setCurrent=%+v ok=%v, want request/ok", sessions.setCurrent, ok)
	}

	renameReq := coreapi.RenameSessionRequest{WorkspaceRoot: "C:/work/demo", SessionID: "sess_current", Title: "Renamed"}
	var renamed coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionRename, renameReq, &renamed); err != nil {
		t.Fatalf("Call(session/rename) error = %v", err)
	}
	if sessions.renamed != renameReq || renamed.Metadata["title"] != "Renamed" {
		t.Fatalf("renamed request=%+v out=%+v, want Renamed", sessions.renamed, renamed)
	}

	deleteReq := coreapi.DeleteSessionRequest{WorkspaceRoot: "C:/work/demo", SessionID: "sess_current"}
	ok = nil
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionDelete, deleteReq, &ok); err != nil {
		t.Fatalf("Call(session/delete) error = %v", err)
	}
	if sessions.deleted != deleteReq || !ok["ok"] {
		t.Fatalf("deleted=%+v ok=%v, want request/ok", sessions.deleted, ok)
	}
}

func TestSessionMessagesLoadSaveOverJSONRPC(t *testing.T) {
	sessions := &fakeSessionService{
		loadMessages: []coreapi.SessionMessage{{Role: "user", Type: "text", Content: "hello"}},
		saveMessages: coreapi.Session{
			ID:            "sess_saved",
			WorkspaceRoot: "C:/work/demo",
			Metadata:      map[string]any{"rounds": 1},
		},
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: sessions}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	loadReq := coreapi.LoadSessionMessagesRequest{WorkspaceRoot: "C:/work/demo", SessionID: "sess_saved"}
	var messages []coreapi.SessionMessage
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionMessagesLoad, loadReq, &messages); err != nil {
		t.Fatalf("Call(session/messages/load) error = %v", err)
	}
	if sessions.loaded != loadReq || len(messages) != 1 || messages[0].Content != "hello" {
		t.Fatalf("loaded=%+v messages=%+v, want hello", sessions.loaded, messages)
	}

	saveReq := coreapi.SaveSessionMessagesRequest{
		WorkspaceRoot: "C:/work/demo",
		SessionID:     "sess_saved",
		Messages:      []coreapi.SessionMessage{{Role: "assistant", Type: "text", Content: "world"}},
	}
	var saved coreapi.Session
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSessionMessagesSave, saveReq, &saved); err != nil {
		t.Fatalf("Call(session/messages/save) error = %v", err)
	}
	if sessions.saved.WorkspaceRoot != saveReq.WorkspaceRoot || sessions.saved.SessionID != saveReq.SessionID {
		t.Fatalf("saved request=%+v, want workspace/session", sessions.saved)
	}
	if len(sessions.saved.Messages) != 1 || sessions.saved.Messages[0].Content != "world" {
		t.Fatalf("saved messages=%+v, want world", sessions.saved.Messages)
	}
	if saved.ID != "sess_saved" {
		t.Fatalf("saved.ID=%q, want sess_saved", saved.ID)
	}
}

func TestMCPMethodsOverJSONRPC(t *testing.T) {
	mcp := &fakeMCPService{items: []coreapi.MCPServer{
		{Name: "docs", Type: "stdio", Target: "node server.js", Enabled: true},
	}}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, mcp: mcp}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var listed []coreapi.MCPServer
	if err := client.Call(context.Background(), protocoljsonrpc.MethodMCPList, nil, &listed); err != nil {
		t.Fatalf("Call(mcp/list) error = %v", err)
	}
	if len(listed) != 1 || listed[0].Name != "docs" {
		t.Fatalf("listed=%+v, want docs", listed)
	}

	upsertReq := coreapi.UpsertMCPRequest{Name: "docs", Type: "stdio", Target: "node server.js", Enabled: true}
	var ok map[string]bool
	if err := client.Call(context.Background(), protocoljsonrpc.MethodMCPUpsert, upsertReq, &ok); err != nil {
		t.Fatalf("Call(mcp/upsert) error = %v", err)
	}
	if !reflect.DeepEqual(mcp.upserted, upsertReq) || !ok["ok"] {
		t.Fatalf("upserted=%+v ok=%v, want request/ok", mcp.upserted, ok)
	}

	importReq := coreapi.ImportMCPJSONRequest{Raw: `{"mcpServers":{"docs":{"command":"node server.js"}}}`}
	ok = nil
	if err := client.Call(context.Background(), protocoljsonrpc.MethodMCPImportJSON, importReq, &ok); err != nil {
		t.Fatalf("Call(mcp/import_json) error = %v", err)
	}
	if mcp.imported != importReq || !ok["ok"] {
		t.Fatalf("imported=%+v ok=%v, want request/ok", mcp.imported, ok)
	}

	deleteReq := coreapi.MCPNameRequest{Name: "docs"}
	ok = nil
	if err := client.Call(context.Background(), protocoljsonrpc.MethodMCPDelete, deleteReq, &ok); err != nil {
		t.Fatalf("Call(mcp/delete) error = %v", err)
	}
	if mcp.deleted != deleteReq || !ok["ok"] {
		t.Fatalf("deleted=%+v ok=%v, want request/ok", mcp.deleted, ok)
	}

	enableReq := coreapi.SetMCPEnabledRequest{Name: "docs", Enabled: false}
	ok = nil
	if err := client.Call(context.Background(), protocoljsonrpc.MethodMCPSetEnabled, enableReq, &ok); err != nil {
		t.Fatalf("Call(mcp/set_enabled) error = %v", err)
	}
	if mcp.enabled != enableReq || !ok["ok"] {
		t.Fatalf("enabled=%+v ok=%v, want request/ok", mcp.enabled, ok)
	}
}

func TestLSPMethodsOverJSONRPC(t *testing.T) {
	summary := coreapi.LSPDiagnosticsSummary{
		Files:    1,
		Errors:   0,
		Warnings: 1,
		Infos:    0,
		Items:    []coreapi.LSPDiagnosticItem{{File: "main.go", Line: 1, Severity: "Warning", Message: "unused var"}},
	}
	lsp := &fakeLSPService{
		items:          []coreapi.LSPServer{{Language: "go", Status: "running", Command: "gopls"}},
		detectMsg:      "go: gopls",
		startMsg:       "go already running",
		diagnostics:    []string{"main.go:1: warning"},
		diagnosticsSum: summary,
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, lsp: lsp}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var listed []coreapi.LSPServer
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPList, nil, &listed); err != nil {
		t.Fatalf("Call(lsp/list) error = %v", err)
	}
	if len(listed) != 1 || listed[0].Language != "go" {
		t.Fatalf("listed=%+v, want go", listed)
	}

	var msg struct {
		Message string `json:"message"`
	}
	detectReq := coreapi.LSPLanguageRequest{Language: "go"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPDetect, detectReq, &msg); err != nil {
		t.Fatalf("Call(lsp/detect) error = %v", err)
	}
	if lsp.detected != detectReq || msg.Message != "go: gopls" {
		t.Fatalf("detected=%+v msg=%q, want go: gopls", lsp.detected, msg.Message)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPStart, detectReq, &msg); err != nil {
		t.Fatalf("Call(lsp/start) error = %v", err)
	}
	if lsp.started != detectReq || msg.Message != "go already running" {
		t.Fatalf("started=%+v msg=%q, want running", lsp.started, msg.Message)
	}
	var diagnostics []string
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPDiagnostics, nil, &diagnostics); err != nil {
		t.Fatalf("Call(lsp/diagnostics) error = %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0] != "main.go:1: warning" {
		t.Fatalf("diagnostics=%+v, want warning", diagnostics)
	}

	var diagSummary coreapi.LSPDiagnosticsSummary
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPDiagnosticsSummary, nil, &diagSummary); err != nil {
		t.Fatalf("Call(lsp/diagnostics/summary) error = %v", err)
	}
	if !lsp.summaryCalled {
		t.Fatal("DiagnosticsSummary was not called")
	}
	if diagSummary.Files != 1 || diagSummary.Warnings != 1 {
		t.Fatalf("summary=%+v, want 1 file 1 warning", diagSummary)
	}
	if len(diagSummary.Items) != 1 || diagSummary.Items[0].Severity != "Warning" {
		t.Fatalf("items=%+v, want 1 Warning item", diagSummary.Items)
	}
}

func TestLSPMethodsOverJSONRPC_NilService(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var listed []coreapi.LSPServer
	err := client.Call(context.Background(), protocoljsonrpc.MethodLSPList, nil, &listed)
	if err == nil {
		t.Fatal("expected error for nil LSP service, got nil")
	}

	var diagSummary coreapi.LSPDiagnosticsSummary
	err = client.Call(context.Background(), protocoljsonrpc.MethodLSPDiagnosticsSummary, nil, &diagSummary)
	if err == nil {
		t.Fatal("expected error for nil LSP service on diagnostics/summary, got nil")
	}
}

func TestLSPMethodsOverJSONRPC_ServiceError(t *testing.T) {
	lsp := &fakeLSPService{err: errors.New("lsp crashed")}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, lsp: lsp}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var listed []coreapi.LSPServer
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPList, nil, &listed); err == nil {
		t.Fatal("expected error from lsp/list, got nil")
	}

	var msg struct {
		Message string `json:"message"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPDetect, coreapi.LSPLanguageRequest{Language: "go"}, &msg); err == nil {
		t.Fatal("expected error from lsp/detect, got nil")
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPStart, coreapi.LSPLanguageRequest{Language: "go"}, &msg); err == nil {
		t.Fatal("expected error from lsp/start, got nil")
	}

	var diagnostics []string
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPDiagnostics, nil, &diagnostics); err == nil {
		t.Fatal("expected error from lsp/diagnostics, got nil")
	}

	var diagSummary coreapi.LSPDiagnosticsSummary
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPDiagnosticsSummary, nil, &diagSummary); err == nil {
		t.Fatal("expected error from lsp/diagnostics/summary, got nil")
	}
}

func TestLSPMethodsOverJSONRPC_EmptyDiagnostics(t *testing.T) {
	lsp := &fakeLSPService{
		items:          []coreapi.LSPServer{},
		diagnostics:    nil,
		diagnosticsSum: coreapi.LSPDiagnosticsSummary{},
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, lsp: lsp}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var listed []coreapi.LSPServer
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPList, nil, &listed); err != nil {
		t.Fatalf("Call(lsp/list) error = %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("listed=%+v, want empty", listed)
	}

	var diagnostics []string
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPDiagnostics, nil, &diagnostics); err != nil {
		t.Fatalf("Call(lsp/diagnostics) error = %v", err)
	}
	if diagnostics != nil && len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v, want empty/nil", diagnostics)
	}

	var diagSummary coreapi.LSPDiagnosticsSummary
	if err := client.Call(context.Background(), protocoljsonrpc.MethodLSPDiagnosticsSummary, nil, &diagSummary); err != nil {
		t.Fatalf("Call(lsp/diagnostics/summary) error = %v", err)
	}
	if diagSummary.Files != 0 || diagSummary.Errors != 0 {
		t.Fatalf("summary=%+v, want zeroed", diagSummary)
	}
}

func TestConfigPermissionAndExtensionMethodsOverJSONRPC(t *testing.T) {
	config := &fakeConfigService{
		rules: "use tests",
		rulesSnapshot: coreapi.RulesSnapshot{
			ActiveRoot: "/tmp/work",
			Documents:  []coreapi.RuleDocument{{Scope: "project", Path: "/tmp/work/.eos/Rules.md", Content: "project rules", Exists: true}},
		},
		settings: coreapi.Settings{Language: "zh", Theme: "system"},
	}
	permissions := &fakePermissionService{
		snapshot: coreapi.PermissionSnapshot{ExecutionMode: "auto", SandboxMode: "workspace-write", HasPendingDiff: true, PendingDiffPath: "diff.patch"},
		review:   coreapi.PendingReview{Path: "diff.patch", Diff: "patch", HasDiff: true},
	}
	extensions := &fakeExtensionService{
		skills:       []coreapi.SkillInfo{{Name: "review", Enabled: true, Active: true}},
		plugins:      []coreapi.PluginInfo{{Name: "browser", Enabled: true}},
		browser:      coreapi.BrowserStatus{ServerName: "playwright", Configured: true, Enabled: true, Loaded: true, Tools: 3},
		invokeResult: coreapi.InvokeSkillResult{Name: "review", Invoked: true},
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, config: config, permissions: permissions, extensions: extensions}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var rules struct {
		Content string `json:"content"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodConfigRulesGet, nil, &rules); err != nil {
		t.Fatalf("Call(config/rules/get) error = %v", err)
	}
	if rules.Content != "use tests" {
		t.Fatalf("rules=%q, want use tests", rules.Content)
	}
	var rulesSnapshot coreapi.RulesSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodConfigRulesSnapshot, nil, &rulesSnapshot); err != nil {
		t.Fatalf("Call(config/rules/snapshot) error = %v", err)
	}
	if rulesSnapshot.ActiveRoot != "/tmp/work" || len(rulesSnapshot.Documents) != 1 || rulesSnapshot.Documents[0].Content != "project rules" {
		t.Fatalf("rulesSnapshot=%+v, want project rules", rulesSnapshot)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodConfigRulesSave, coreapi.SaveRulesRequest{Content: "new rules"}, nil); err != nil {
		t.Fatalf("Call(config/rules/save) error = %v", err)
	}
	if config.savedRules.Content != "new rules" {
		t.Fatalf("savedRules=%+v, want new rules", config.savedRules)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodConfigRulesReset, nil, nil); err != nil {
		t.Fatalf("Call(config/rules/reset) error = %v", err)
	}
	if !config.resetCalled {
		t.Fatal("resetCalled=false, want true")
	}
	var settings coreapi.Settings
	if err := client.Call(context.Background(), protocoljsonrpc.MethodConfigSettingsGet, nil, &settings); err != nil {
		t.Fatalf("Call(config/settings/get) error = %v", err)
	}
	if settings.Language != "zh" || settings.Theme != "system" {
		t.Fatalf("settings=%+v, want zh/system", settings)
	}
	saveSettings := coreapi.Settings{Language: "en", Theme: "dark"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodConfigSettingsSave, saveSettings, nil); err != nil {
		t.Fatalf("Call(config/settings/save) error = %v", err)
	}
	if config.savedSettings != saveSettings {
		t.Fatalf("savedSettings=%+v, want %+v", config.savedSettings, saveSettings)
	}

	var snapshot coreapi.PermissionSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodPermissionSnapshot, nil, &snapshot); err != nil {
		t.Fatalf("Call(permission/snapshot) error = %v", err)
	}
	if !snapshot.HasPendingDiff || snapshot.PendingDiffPath != "diff.patch" {
		t.Fatalf("snapshot=%+v, want pending diff", snapshot)
	}
	var review coreapi.PendingReview
	if err := client.Call(context.Background(), protocoljsonrpc.MethodPermissionPendingReview, nil, &review); err != nil {
		t.Fatalf("Call(permission/pending_review) error = %v", err)
	}
	if !review.HasDiff || review.Diff != "patch" {
		t.Fatalf("review=%+v, want patch", review)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodPermissionClearReview, nil, nil); err != nil {
		t.Fatalf("Call(permission/clear_pending_review) error = %v", err)
	}
	if !permissions.clearCalled {
		t.Fatal("clearCalled=false, want true")
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodPermissionAccessModeSet, coreapi.SetModeRequest{Mode: "read-only"}, nil); err != nil {
		t.Fatalf("Call(permission/access_mode/set) error = %v", err)
	}
	if permissions.accessMode != "read-only" {
		t.Fatalf("accessMode=%q, want read-only", permissions.accessMode)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodPermissionApprovalModeSet, coreapi.SetModeRequest{Mode: "never"}, nil); err != nil {
		t.Fatalf("Call(permission/approval_mode/set) error = %v", err)
	}
	if permissions.approvalMode != "never" {
		t.Fatalf("approvalMode=%q, want never", permissions.approvalMode)
	}

	var skills []coreapi.SkillInfo
	if err := client.Call(context.Background(), protocoljsonrpc.MethodExtensionsSkillsList, nil, &skills); err != nil {
		t.Fatalf("Call(extensions/skills/list) error = %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "review" {
		t.Fatalf("skills=%+v, want review", skills)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodExtensionsSkillsReload, nil, nil); err != nil {
		t.Fatalf("Call(extensions/skills/reload) error = %v", err)
	}
	if !extensions.reloadCalled {
		t.Fatal("reloadCalled=false, want true")
	}
	enableReq := coreapi.SetExtensionEnabledRequest{Name: "review", Enabled: false}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodExtensionsSkillSetEnabled, enableReq, nil); err != nil {
		t.Fatalf("Call(extensions/skill/set_enabled) error = %v", err)
	}
	if extensions.skillEnabled != enableReq {
		t.Fatalf("skillEnabled=%+v, want %+v", extensions.skillEnabled, enableReq)
	}
	var invoke coreapi.InvokeSkillResult
	invokeReq := coreapi.InvokeSkillRequest{Name: "review", Arguments: "diff"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodExtensionsSkillInvoke, invokeReq, &invoke); err != nil {
		t.Fatalf("Call(extensions/skill/invoke) error = %v", err)
	}
	if !invoke.Invoked || extensions.invokedSkill != invokeReq {
		t.Fatalf("invoke=%+v invokedSkill=%+v, want invoked review", invoke, extensions.invokedSkill)
	}
	var plugins []coreapi.PluginInfo
	if err := client.Call(context.Background(), protocoljsonrpc.MethodExtensionsPluginsList, nil, &plugins); err != nil {
		t.Fatalf("Call(extensions/plugins/list) error = %v", err)
	}
	if len(plugins) != 1 || plugins[0].Name != "browser" {
		t.Fatalf("plugins=%+v, want browser", plugins)
	}
	pluginReq := coreapi.SetExtensionEnabledRequest{Name: "browser", Enabled: false}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodExtensionsPluginSetEnabled, pluginReq, nil); err != nil {
		t.Fatalf("Call(extensions/plugin/set_enabled) error = %v", err)
	}
	if extensions.pluginEnabled != pluginReq {
		t.Fatalf("pluginEnabled=%+v, want %+v", extensions.pluginEnabled, pluginReq)
	}
	var browser coreapi.BrowserStatus
	if err := client.Call(context.Background(), protocoljsonrpc.MethodBrowserStatus, nil, &browser); err != nil {
		t.Fatalf("Call(browser/status) error = %v", err)
	}
	if browser.ServerName != "playwright" || !browser.Configured || !browser.Loaded || browser.Tools != 3 {
		t.Fatalf("browser=%+v, want loaded playwright", browser)
	}
}

func TestContextUsageAndVersionMethodsOverJSONRPC(t *testing.T) {
	inputTokens := 10
	costUSD := 0.25
	contextSvc := &fakeContextService{
		preview:    []string{"user: hello", "assistant: world"},
		stats:      coreapi.ContextStats{MessageCount: 2, Estimated: 42},
		window:     128000,
		compactMsg: "context compacted",
	}
	usage := &fakeUsageService{
		summary:     coreapi.UsageSummary{Rounds: 1, InputTokens: &inputTokens, CostUSD: &costUSD},
		costSummary: "1 轮 · 输入 10",
		items:       []coreapi.CostItem{{Model: "test-model", InputTokens: &inputTokens, CostUSD: &costUSD, UsageKnown: true, CostKnown: true}},
	}
	versions := &fakeVersionService{
		items:           []coreapi.VersionItem{{ID: "v1", File: "main.go", Summary: "size=10"}},
		deleteFileCount: 3,
		clearCount:      4,
	}
	tasks := &fakeTaskService{
		items:        []coreapi.TaskSnapshot{{ID: "task-1", Status: "running", CanKill: true}},
		todos:        []coreapi.TodoItem{{ID: "todo-1", Content: "ship it", Status: "pending"}},
		lines:        []string{"stdout: ok"},
		cleanupCount: 2,
	}
	modes := &fakeModeService{
		snapshot: coreapi.ModeSnapshot{ExecutionMode: "auto", SandboxMode: "workspace", ReasoningLevel: "high"},
	}
	models := &fakeModelService{
		items: []coreapi.ModelConfig{{Name: "main", APIKeyMasked: "****1234", Active: true}},
		catalog: coreapi.ModelCatalogState{
			Providers:           []coreapi.ModelProviderOption{{ID: "openai", Name: "OpenAI"}},
			Presets:             []coreapi.ModelPresetOption{{ID: "gpt", ProviderID: "openai", ModelName: "gpt-test"}},
			AllowCustomProvider: true,
			AllowCustomModel:    true,
		},
	}
	remote := &fakeRemoteWorkspaceService{
		items:     []coreapi.RemoteWorkspace{{ID: "github:https://example.com/repo.git", Platform: "github", Repo: "repo", Exists: true}},
		current:   coreapi.RemoteRepoState{Platform: "github", Repo: "repo", LocalPath: "/tmp/repo"},
		currentOK: true,
	}
	git := &fakeGitService{
		status:   []coreapi.GitChange{{Path: "main.go", State: "modified"}},
		diff:     coreapi.GitTextResult{Text: "diff --git a/main.go b/main.go"},
		branches: coreapi.GitBranchesResult{Current: "main", Branches: []string{"main", "feature"}},
		log:      coreapi.GitLogResult{Branch: "main", Entries: []coreapi.GitLogEntry{{Hash: "abc123", Message: "init"}}, Text: "abc123 init"},
		show:     coreapi.GitShowResult{Branch: "main", Revision: "HEAD", Text: "commit abc123"},
	}
	insights := &fakeInsightService{
		prediction: "next message",
		plan:       coreapi.PlanSnapshot{HasPlan: true, Content: "plan", WorkspaceCurrent: "step 1"},
		memory:     coreapi.MemorySnapshot{Documents: []coreapi.MemoryDocument{{Scope: "project", Path: "MEMORY.md", Exists: true, Summary: "remember this"}}},
	}
	memorySvc := &fakeMemoryService{
		snapshot: coreapi.MemorySnapshot{Documents: []coreapi.MemoryDocument{{Scope: "global", Path: "user.md", Exists: true, Content: "remember user", Summary: "remember user"}}},
	}
	roles := &fakeRoleService{
		items: []coreapi.RoleConfig{{ID: "planner", ContextStrategy: "shared"}, {ID: "senior-dev", LegacyAliases: []string{"senior_dev"}}},
		resolved: coreapi.RoleConfig{
			ID:              "senior-dev",
			Description:     "Implement production code changes.",
			ContextStrategy: "shared",
			LegacyAliases:   []string{"senior_dev"},
		},
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, context: contextSvc, usage: usage, versions: versions, tasks: tasks, modes: modes, models: models, remote: remote, git: git, insights: insights, memory: memorySvc, roles: roles}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var preview []string
	if err := client.Call(context.Background(), protocoljsonrpc.MethodContextPreview, nil, &preview); err != nil {
		t.Fatalf("Call(context/preview) error = %v", err)
	}
	if strings.Join(preview, "\n") != "user: hello\nassistant: world" {
		t.Fatalf("preview=%+v, want two lines", preview)
	}
	var stats coreapi.ContextStats
	if err := client.Call(context.Background(), protocoljsonrpc.MethodContextStats, nil, &stats); err != nil {
		t.Fatalf("Call(context/stats) error = %v", err)
	}
	if stats.MessageCount != 2 || stats.Estimated != 42 {
		t.Fatalf("stats=%+v, want 2/42", stats)
	}
	var window struct {
		Tokens int `json:"tokens"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodContextWindow, nil, &window); err != nil {
		t.Fatalf("Call(context/window) error = %v", err)
	}
	if window.Tokens != 128000 {
		t.Fatalf("window=%d, want 128000", window.Tokens)
	}
	pinReq := coreapi.PinDocumentRequest{ID: "EOS.md", Content: "rules", TokenBudget: 20000}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodContextPin, pinReq, nil); err != nil {
		t.Fatalf("Call(context/pin) error = %v", err)
	}
	if contextSvc.pinned != pinReq {
		t.Fatalf("pinned=%+v, want %+v", contextSvc.pinned, pinReq)
	}
	var compact struct {
		Message string `json:"message"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodContextCompact, nil, &compact); err != nil {
		t.Fatalf("Call(context/compact) error = %v", err)
	}
	if compact.Message != "context compacted" {
		t.Fatalf("compact=%q, want context compacted", compact.Message)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodContextClear, nil, nil); err != nil {
		t.Fatalf("Call(context/clear) error = %v", err)
	}
	if !contextSvc.clearCalled {
		t.Fatal("clearCalled=false, want true")
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodContextExport, coreapi.ExportContextRequest{Path: "ctx.md"}, nil); err != nil {
		t.Fatalf("Call(context/export) error = %v", err)
	}
	if contextSvc.exportedPath != "ctx.md" {
		t.Fatalf("exportedPath=%q, want ctx.md", contextSvc.exportedPath)
	}

	var summary coreapi.UsageSummary
	if err := client.Call(context.Background(), protocoljsonrpc.MethodUsageSummary, nil, &summary); err != nil {
		t.Fatalf("Call(usage/summary) error = %v", err)
	}
	if summary.Rounds != 1 || summary.InputTokens == nil || *summary.InputTokens != 10 {
		t.Fatalf("summary=%+v, want one round/input 10", summary)
	}
	var text struct {
		Summary string `json:"summary"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodUsageCostSummary, nil, &text); err != nil {
		t.Fatalf("Call(usage/cost_summary) error = %v", err)
	}
	if text.Summary != "1 轮 · 输入 10" {
		t.Fatalf("cost summary=%q, want text", text.Summary)
	}
	var costItems []coreapi.CostItem
	if err := client.Call(context.Background(), protocoljsonrpc.MethodUsageCostItems, nil, &costItems); err != nil {
		t.Fatalf("Call(usage/cost_items) error = %v", err)
	}
	if len(costItems) != 1 || costItems[0].Model != "test-model" {
		t.Fatalf("costItems=%+v, want test-model", costItems)
	}

	var versionItems []coreapi.VersionItem
	if err := client.Call(context.Background(), protocoljsonrpc.MethodVersionsList, nil, &versionItems); err != nil {
		t.Fatalf("Call(versions/list) error = %v", err)
	}
	if len(versionItems) != 1 || versionItems[0].ID != "v1" {
		t.Fatalf("versionItems=%+v, want v1", versionItems)
	}
	versionReq := coreapi.VersionIDRequest{ID: "v1"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodVersionsRollback, versionReq, nil); err != nil {
		t.Fatalf("Call(versions/rollback) error = %v", err)
	}
	if versions.rolledBack != versionReq {
		t.Fatalf("rolledBack=%+v, want %+v", versions.rolledBack, versionReq)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodVersionsDelete, versionReq, nil); err != nil {
		t.Fatalf("Call(versions/delete) error = %v", err)
	}
	if versions.deleted != versionReq {
		t.Fatalf("deleted=%+v, want %+v", versions.deleted, versionReq)
	}
	var count struct {
		Count int `json:"count"`
	}
	fileReq := coreapi.VersionFileRequest{File: "main.go"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodVersionsDeleteFile, fileReq, &count); err != nil {
		t.Fatalf("Call(versions/delete_file) error = %v", err)
	}
	if versions.deletedFile != fileReq || count.Count != 3 {
		t.Fatalf("deletedFile=%+v count=%d, want main.go/3", versions.deletedFile, count.Count)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodVersionsClear, nil, &count); err != nil {
		t.Fatalf("Call(versions/clear) error = %v", err)
	}
	if !versions.clearCalled || count.Count != 4 {
		t.Fatalf("clearCalled=%v count=%d, want true/4", versions.clearCalled, count.Count)
	}

	var taskItems []coreapi.TaskSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodTaskList, nil, &taskItems); err != nil {
		t.Fatalf("Call(task/list) error = %v", err)
	}
	if len(taskItems) != 1 || taskItems[0].ID != "task-1" {
		t.Fatalf("taskItems=%+v, want task-1", taskItems)
	}
	var todos []coreapi.TodoItem
	if err := client.Call(context.Background(), protocoljsonrpc.MethodTaskTodos, nil, &todos); err != nil {
		t.Fatalf("Call(task/todos) error = %v", err)
	}
	if len(todos) != 1 || todos[0].ID != "todo-1" || todos[0].Content != "ship it" {
		t.Fatalf("todos=%+v, want todo-1", todos)
	}
	taskReq := coreapi.TaskIDRequest{TaskID: "task-1"}
	var lines []string
	if err := client.Call(context.Background(), protocoljsonrpc.MethodTaskTail, taskReq, &lines); err != nil {
		t.Fatalf("Call(task/tail) error = %v", err)
	}
	if tasks.tailed != taskReq || len(lines) != 1 || lines[0] != "stdout: ok" {
		t.Fatalf("tailed=%+v lines=%+v, want stdout", tasks.tailed, lines)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodTaskKill, taskReq, nil); err != nil {
		t.Fatalf("Call(task/kill) error = %v", err)
	}
	if tasks.killed != taskReq {
		t.Fatalf("killed=%+v, want %+v", tasks.killed, taskReq)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodTaskCleanup, nil, &count); err != nil {
		t.Fatalf("Call(task/cleanup) error = %v", err)
	}
	if !tasks.cleanupCalled || count.Count != 2 {
		t.Fatalf("cleanupCalled=%v count=%d, want true/2", tasks.cleanupCalled, count.Count)
	}

	var modeSnapshot coreapi.ModeSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRuntimeModesGet, nil, &modeSnapshot); err != nil {
		t.Fatalf("Call(runtime/modes/get) error = %v", err)
	}
	if modeSnapshot.ExecutionMode != "auto" || modeSnapshot.SandboxMode != "workspace" || modeSnapshot.ReasoningLevel != "high" {
		t.Fatalf("modeSnapshot=%+v, want auto/workspace/high", modeSnapshot)
	}
	modeReq := coreapi.SetModeRequest{Mode: "manual"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRuntimeExecutionModeSet, modeReq, nil); err != nil {
		t.Fatalf("Call(runtime/execution_mode/set) error = %v", err)
	}
	if modes.exec != modeReq {
		t.Fatalf("exec=%+v, want %+v", modes.exec, modeReq)
	}
	sandboxReq := coreapi.SetModeRequest{Mode: "full_access"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRuntimeSandboxModeSet, sandboxReq, nil); err != nil {
		t.Fatalf("Call(runtime/sandbox_mode/set) error = %v", err)
	}
	if modes.sandbox != sandboxReq {
		t.Fatalf("sandbox=%+v, want %+v", modes.sandbox, sandboxReq)
	}
	reasoningReq := coreapi.SetModeRequest{Mode: "medium"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRuntimeReasoningLevelSet, reasoningReq, nil); err != nil {
		t.Fatalf("Call(runtime/reasoning_level/set) error = %v", err)
	}
	if modes.reasoning != reasoningReq {
		t.Fatalf("reasoning=%+v, want %+v", modes.reasoning, reasoningReq)
	}

	var modelItems []coreapi.ModelConfig
	if err := client.Call(context.Background(), protocoljsonrpc.MethodModelList, nil, &modelItems); err != nil {
		t.Fatalf("Call(model/list) error = %v", err)
	}
	if len(modelItems) != 1 || modelItems[0].Name != "main" || modelItems[0].APIKeyMasked != "****1234" {
		t.Fatalf("modelItems=%+v, want masked main", modelItems)
	}
	var catalog coreapi.ModelCatalogState
	if err := client.Call(context.Background(), protocoljsonrpc.MethodModelCatalog, nil, &catalog); err != nil {
		t.Fatalf("Call(model/catalog) error = %v", err)
	}
	if len(catalog.Providers) != 1 || catalog.Providers[0].ID != "openai" {
		t.Fatalf("catalog=%+v, want openai", catalog)
	}
	upsertReq := coreapi.UpsertModelRequest{Name: "main", APIBase: "https://example.com", APIKey: "sk-test", Model: "test-model"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodModelUpsert, upsertReq, nil); err != nil {
		t.Fatalf("Call(model/upsert) error = %v", err)
	}
	if models.upserted != upsertReq {
		t.Fatalf("upserted=%+v, want %+v", models.upserted, upsertReq)
	}
	saveReq := coreapi.ModelSaveRequest{Name: "main", Mode: "custom_provider", APIBase: "https://example.com", Model: "test-model"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodModelSave, saveReq, nil); err != nil {
		t.Fatalf("Call(model/save) error = %v", err)
	}
	if models.saved != saveReq {
		t.Fatalf("saved=%+v, want %+v", models.saved, saveReq)
	}
	nameReq := coreapi.ModelNameRequest{Name: "main"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodModelDelete, nameReq, nil); err != nil {
		t.Fatalf("Call(model/delete) error = %v", err)
	}
	if models.deleted != nameReq {
		t.Fatalf("deleted=%+v, want %+v", models.deleted, nameReq)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodModelActivate, nameReq, nil); err != nil {
		t.Fatalf("Call(model/activate) error = %v", err)
	}
	if models.activated != nameReq {
		t.Fatalf("activated=%+v, want %+v", models.activated, nameReq)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodModelSyncEnv, nil, nil); err != nil {
		t.Fatalf("Call(model/sync_env) error = %v", err)
	}
	if !models.synced {
		t.Fatal("model sync env was not called")
	}

	var remoteItems []coreapi.RemoteWorkspace
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRemoteWorkspaceList, nil, &remoteItems); err != nil {
		t.Fatalf("Call(remote_workspace/list) error = %v", err)
	}
	if len(remoteItems) != 1 || remoteItems[0].Repo != "repo" {
		t.Fatalf("remoteItems=%+v, want repo", remoteItems)
	}
	remoteReq := coreapi.RemoteWorkspaceRef{IDOrPath: "github:https://example.com/repo.git"}
	var opened coreapi.RemoteWorkspace
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRemoteWorkspaceOpen, remoteReq, &opened); err != nil {
		t.Fatalf("Call(remote_workspace/open) error = %v", err)
	}
	if remote.opened != remoteReq || !opened.Active {
		t.Fatalf("openedReq=%+v opened=%+v, want active", remote.opened, opened)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRemoteWorkspaceForget, remoteReq, nil); err != nil {
		t.Fatalf("Call(remote_workspace/forget) error = %v", err)
	}
	if remote.forgot != remoteReq {
		t.Fatalf("forgot=%+v, want %+v", remote.forgot, remoteReq)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRemoteWorkspaceClearCache, remoteReq, nil); err != nil {
		t.Fatalf("Call(remote_workspace/clear_cache) error = %v", err)
	}
	if remote.cleared != remoteReq {
		t.Fatalf("cleared=%+v, want %+v", remote.cleared, remoteReq)
	}
	var current struct {
		OK    bool                    `json:"ok"`
		State coreapi.RemoteRepoState `json:"state"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRemoteRepoCurrent, nil, &current); err != nil {
		t.Fatalf("Call(remote_repo/current) error = %v", err)
	}
	if !current.OK || current.State.Repo != "repo" {
		t.Fatalf("current=%+v, want repo", current)
	}

	statusReq := coreapi.GitStatusRequest{WorkspaceRoot: "/tmp/repo"}
	var gitChanges []coreapi.GitChange
	if err := client.Call(context.Background(), protocoljsonrpc.MethodGitStatus, statusReq, &gitChanges); err != nil {
		t.Fatalf("Call(git/status) error = %v", err)
	}
	if git.statusReq != statusReq || len(gitChanges) != 1 || gitChanges[0].Path != "main.go" {
		t.Fatalf("git status req=%+v changes=%+v, want main.go", git.statusReq, gitChanges)
	}
	diffReq := coreapi.GitDiffRequest{WorkspaceRoot: "/tmp/repo", Path: "main.go"}
	var gitDiff coreapi.GitTextResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodGitDiff, diffReq, &gitDiff); err != nil {
		t.Fatalf("Call(git/diff) error = %v", err)
	}
	if git.diffReq != diffReq || !strings.Contains(gitDiff.Text, "diff --git") {
		t.Fatalf("git diff req=%+v out=%+v, want diff text", git.diffReq, gitDiff)
	}
	branchesReq := coreapi.GitBranchesRequest{WorkspaceRoot: "/tmp/repo"}
	var gitBranches coreapi.GitBranchesResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodGitBranches, branchesReq, &gitBranches); err != nil {
		t.Fatalf("Call(git/branches) error = %v", err)
	}
	if git.branchesReq != branchesReq || gitBranches.Current != "main" || len(gitBranches.Branches) != 2 {
		t.Fatalf("git branches req=%+v out=%+v, want main branches", git.branchesReq, gitBranches)
	}
	logReq := coreapi.GitLogRequest{WorkspaceRoot: "/tmp/repo", Limit: 5, Oneline: true, Path: "main.go"}
	var gitLog coreapi.GitLogResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodGitLog, logReq, &gitLog); err != nil {
		t.Fatalf("Call(git/log) error = %v", err)
	}
	if git.logReq != logReq || len(gitLog.Entries) != 1 || gitLog.Entries[0].Hash != "abc123" {
		t.Fatalf("git log req=%+v out=%+v, want abc123", git.logReq, gitLog)
	}
	showReq := coreapi.GitShowRequest{WorkspaceRoot: "/tmp/repo", Revision: "HEAD", Path: "main.go"}
	var gitShow coreapi.GitShowResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodGitShow, showReq, &gitShow); err != nil {
		t.Fatalf("Call(git/show) error = %v", err)
	}
	if git.showReq != showReq || gitShow.Revision != "HEAD" || !strings.Contains(gitShow.Text, "commit") {
		t.Fatalf("git show req=%+v out=%+v, want commit text", git.showReq, gitShow)
	}

	predictReq := coreapi.PredictNextUserMessageRequest{Draft: "draft"}
	var prediction struct {
		Message string `json:"message"`
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInsightPredictNextUser, predictReq, &prediction); err != nil {
		t.Fatalf("Call(insight/predict_next_user_message) error = %v", err)
	}
	if insights.predicted != predictReq || prediction.Message != "next message" {
		t.Fatalf("predicted=%+v message=%q, want next message", insights.predicted, prediction.Message)
	}
	var plan coreapi.PlanSnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInsightPlanSnapshot, nil, &plan); err != nil {
		t.Fatalf("Call(insight/plan_snapshot) error = %v", err)
	}
	if !plan.HasPlan || plan.Content != "plan" {
		t.Fatalf("plan=%+v, want plan", plan)
	}
	var memory coreapi.MemorySnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInsightMemorySnapshot, nil, &memory); err != nil {
		t.Fatalf("Call(insight/memory_snapshot) error = %v", err)
	}
	if len(memory.Documents) != 1 || memory.Documents[0].Summary != "remember this" {
		t.Fatalf("memory=%+v, want memory document", memory)
	}
	var memorySnapshot coreapi.MemorySnapshot
	if err := client.Call(context.Background(), protocoljsonrpc.MethodMemorySnapshot, nil, &memorySnapshot); err != nil {
		t.Fatalf("Call(memory/snapshot) error = %v", err)
	}
	if len(memorySnapshot.Documents) != 1 || memorySnapshot.Documents[0].Content != "remember user" {
		t.Fatalf("memorySnapshot=%+v, want memory service document", memorySnapshot)
	}
	saveMemory := coreapi.SaveMemoryRequest{Scope: "project", Content: "save memory"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodMemorySave, saveMemory, nil); err != nil {
		t.Fatalf("Call(memory/save) error = %v", err)
	}
	if memorySvc.saved != saveMemory {
		t.Fatalf("saved memory=%+v, want %+v", memorySvc.saved, saveMemory)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodMemoryRebuildIndex, nil, nil); err != nil {
		t.Fatalf("Call(memory/rebuild_index) error = %v", err)
	}
	if !memorySvc.rebuildCalled {
		t.Fatal("memory.rebuildCalled=false, want true")
	}

	var roleItems []coreapi.RoleConfig
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRoleList, nil, &roleItems); err != nil {
		t.Fatalf("Call(role/list) error = %v", err)
	}
	if len(roleItems) != 2 || roleItems[1].ID != "senior-dev" {
		t.Fatalf("roleItems=%+v, want senior-dev", roleItems)
	}
	roleReq := coreapi.RoleRef{ID: "senior_dev"}
	var role coreapi.RoleConfig
	if err := client.Call(context.Background(), protocoljsonrpc.MethodRoleResolve, roleReq, &role); err != nil {
		t.Fatalf("Call(role/resolve) error = %v", err)
	}
	if roles.seen != roleReq || role.ID != "senior-dev" {
		t.Fatalf("seen=%+v role=%+v, want senior-dev", roles.seen, role)
	}
}

func TestAgentMethodsOverJSONRPC(t *testing.T) {
	agents := &fakeAgentService{
		spawn: coreapi.Agent{ID: "agent_1", RoleID: "senior-dev", Task: "build", Status: "pending"},
		wait:  coreapi.Agent{ID: "agent_1", RoleID: "senior-dev", Task: "build", Status: "pending"},
		run: coreapi.AgentRunResult{
			Agent:  coreapi.Agent{ID: "agent_1", RoleID: "senior-dev", Task: "build", Status: "completed"},
			Output: "done",
		},
		tool:  coreapi.AgentToolResult{Name: "read", Display: "ok", Output: json.RawMessage(`{"text":"hello"}`)},
		items: []coreapi.Agent{{ID: "agent_1", RoleID: "senior-dev", Status: "pending"}},
	}
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 3)}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, agents: agents}, Options{Notifier: notifier}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	spawnReq := coreapi.SpawnAgentRequest{RoleID: "senior_dev", Task: "build"}
	var spawned coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentSpawn, spawnReq, &spawned); err != nil {
		t.Fatalf("Call(agent/spawn) error = %v", err)
	}
	if agents.spawned.RoleID != spawnReq.RoleID || agents.spawned.Task != spawnReq.Task || spawned.ID != "agent_1" {
		t.Fatalf("spawnedReq=%+v spawned=%+v, want agent_1", agents.spawned, spawned)
	}
	assertAgentNotification(t, notifier.ch, protocol.EventTypeAgentStarted, "agent_1")
	input := coreapi.AgentInput{AgentID: "agent_1", Input: "continue"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentInput, input, nil); err != nil {
		t.Fatalf("Call(agent/input) error = %v", err)
	}
	if agents.input != input {
		t.Fatalf("input=%+v, want %+v", agents.input, input)
	}
	assertAgentNotification(t, notifier.ch, protocol.EventTypeAgentProgress, "agent_1")
	ref := coreapi.AgentRef{AgentID: "agent_1"}
	var waited coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentWait, ref, &waited); err != nil {
		t.Fatalf("Call(agent/wait) error = %v", err)
	}
	if agents.waited != ref || waited.ID != "agent_1" {
		t.Fatalf("waitedRef=%+v waited=%+v, want agent_1", agents.waited, waited)
	}
	runReq := coreapi.RunAgentRequest{AgentID: "agent_1", SessionID: "sess_1"}
	var run coreapi.AgentRunResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentRun, runReq, &run); err != nil {
		t.Fatalf("Call(agent/run) error = %v", err)
	}
	if agents.runReq.AgentID != runReq.AgentID || agents.runReq.SessionID != runReq.SessionID || run.Output != "done" {
		t.Fatalf("runReq=%+v run=%+v, want done", agents.runReq, run)
	}
	toolReq := coreapi.AgentToolRequest{AgentID: "agent_1", SessionID: "sess_1", TurnID: "turn_1", Name: "read", Args: json.RawMessage(`{"path":"README.md"}`)}
	var tool coreapi.AgentToolResult
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentToolExecute, toolReq, &tool); err != nil {
		t.Fatalf("Call(agent/tool/execute) error = %v", err)
	}
	if agents.toolReq.AgentID != toolReq.AgentID || agents.toolReq.TurnID != toolReq.TurnID || agents.toolReq.Name != "read" || tool.Display != "ok" {
		t.Fatalf("toolReq=%+v tool=%+v, want ok", agents.toolReq, tool)
	}
	listReq := coreapi.ListAgentsRequest{SessionID: "sess_1"}
	var items []coreapi.Agent
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentList, listReq, &items); err != nil {
		t.Fatalf("Call(agent/list) error = %v", err)
	}
	if agents.listReq != listReq || len(items) != 1 || items[0].ID != "agent_1" {
		t.Fatalf("listReq=%+v items=%+v, want agent_1", agents.listReq, items)
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodAgentClose, ref, nil); err != nil {
		t.Fatalf("Call(agent/close) error = %v", err)
	}
	if agents.closed != ref {
		t.Fatalf("closed=%+v, want %+v", agents.closed, ref)
	}
	assertAgentNotification(t, notifier.ch, protocol.EventTypeAgentCancelled, "agent_1")
}

func assertAgentNotification(t *testing.T, ch <-chan protocoljsonrpc.Notification, eventType protocol.EventType, agentID string) {
	t.Helper()
	select {
	case notification := <-ch:
		if notification.Method != protocoljsonrpc.NotificationEvent {
			t.Fatalf("notification.Method=%q, want %q", notification.Method, protocoljsonrpc.NotificationEvent)
		}
		var envelope protocol.Envelope
		if err := json.Unmarshal(notification.Params, &envelope); err != nil {
			t.Fatalf("Unmarshal(notification.Params) error = %v", err)
		}
		if envelope.EventType != eventType || envelope.Payload["agent_id"] != agentID {
			t.Fatalf("envelope=%+v, want %s for %s", envelope, eventType, agentID)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for %s notification", eventType)
	}
}

func TestEventSubscribeOverJSONRPCForwardsNotifications(t *testing.T) {
	events := &fakeEvents{ch: make(chan protocol.Envelope, 1)}
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 1)}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		events:   events,
	}, Options{Notifier: notifier}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	req := coreapi.EventSubscribeRequest{SessionID: "sess-1", TurnID: "turn-1", AgentID: "agent-1"}
	var subscription coreapi.EventSubscription
	if err := client.Call(context.Background(), protocoljsonrpc.MethodEventSubscribe, req, &subscription); err != nil {
		t.Fatalf("Call(event/subscribe) error = %v", err)
	}
	if subscription.ID == "" {
		t.Fatal("subscription.ID is empty")
	}
	if !reflect.DeepEqual(events.seen, coreapi.EventFilter{SessionID: "sess-1", TurnID: "turn-1", AgentID: "agent-1"}) {
		t.Fatalf("filter=%+v, want session/turn/agent filter", events.seen)
	}
	if events.ctx == nil {
		t.Fatal("subscription context is nil")
	}

	want := protocol.NewEvent(protocol.EventTypeItemDelta, protocol.EventOptions{
		EventID:   "evt-sub",
		SessionID: "sess-1",
		RequestID: "turn-1",
		Payload:   map[string]any{"text": "stream"},
	})
	events.ch <- want
	select {
	case notification := <-notifier.ch:
		if notification.Method != protocoljsonrpc.NotificationEvent {
			t.Fatalf("notification.Method=%q, want %q", notification.Method, protocoljsonrpc.NotificationEvent)
		}
		var got protocol.Envelope
		if err := json.Unmarshal(notification.Params, &got); err != nil {
			t.Fatalf("Unmarshal(notification.Params) error = %v", err)
		}
		if got.EventID != "evt-sub" || got.EventType != protocol.EventTypeItemDelta {
			t.Fatalf("event=%+v, want evt-sub text.delta", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting event notification")
	}

	var ok map[string]bool
	if err := client.Call(context.Background(), protocoljsonrpc.MethodEventUnsubscribe, coreapi.EventUnsubscribeRequest{ID: subscription.ID}, &ok); err != nil {
		t.Fatalf("Call(event/unsubscribe) error = %v", err)
	}
	if !ok["ok"] {
		t.Fatalf("ok=%v, want true", ok)
	}
	select {
	case <-events.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting subscription cancellation")
	}
}

func TestEventUnsubscribeRequiresID(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	req, err := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req_1"), protocoljsonrpc.MethodEventUnsubscribe, coreapi.EventUnsubscribeRequest{})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp := router.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("response error = nil, want invalid params")
	}
	if resp.Error.Code != protocoljsonrpc.CodeInvalidParams {
		t.Fatalf("error code=%d, want %d", resp.Error.Code, protocoljsonrpc.CodeInvalidParams)
	}
}

func TestApprovalRespondOverJSONRPC(t *testing.T) {
	approvals := &fakeApprovalService{}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, approvals: approvals}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out map[string]bool
	req := coreapi.ApprovalResponse{ApprovalID: "approval-1", Decision: coreapi.ApprovalAccept, Reason: "go"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodApprovalRespond, req, &out); err != nil {
		t.Fatalf("Call(approval/respond) error = %v", err)
	}
	if !reflect.DeepEqual(approvals.seen, req) {
		t.Fatalf("seen=%+v, want %+v", approvals.seen, req)
	}
	if !out["ok"] {
		t.Fatalf("out=%v, want ok=true", out)
	}
}

func TestInquiryRespondOverJSONRPC(t *testing.T) {
	inquiries := &fakeInquiryService{}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, inquiries: inquiries}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out map[string]bool
	req := coreapi.InquiryResponse{InquiryID: "inq-1", Option: "manual", Text: "use manual mode"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodInquiryRespond, req, &out); err != nil {
		t.Fatalf("Call(inquiry/respond) error = %v", err)
	}
	if inquiries.seen != req {
		t.Fatalf("seen=%+v, want %+v", inquiries.seen, req)
	}
	if !out["ok"] {
		t.Fatalf("out=%v, want ok=true", out)
	}
}

func TestTurnStartOverJSONRPCForwardsEvents(t *testing.T) {
	turns := &fakeTurnService{start: coreapi.Turn{
		ID:        "turn-1",
		SessionID: "sess-1",
		Status:    "running",
		StartedAt: time.Unix(4, 0),
		UpdatedAt: time.Unix(4, 0),
	}}
	events := &fakeEvents{ch: make(chan protocol.Envelope, 1)}
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 1)}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		turns:    turns,
		events:   events,
	}, Options{Notifier: notifier}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out coreapi.Turn
	req := coreapi.StartTurnRequest{
		SessionID:  "sess-1",
		TurnID:     "turn-1",
		Input:      "hello",
		ImagePaths: []string{"C:/tmp/a.png"},
		Attachments: []coreapi.Attachment{{
			Name: "notes.txt",
			Path: "C:/tmp/notes.txt",
			MIME: "text/plain",
			Kind: "file",
		}},
		Options: json.RawMessage(`{"reasoning":"low"}`),
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodTurnStart, req, &out); err != nil {
		t.Fatalf("Call(turn/start) error = %v", err)
	}
	if turns.started.SessionID != "sess-1" || turns.started.Input != "hello" || string(turns.started.Options) != `{"reasoning":"low"}` {
		t.Fatalf("started=%+v, want request passthrough", turns.started)
	}
	if len(turns.started.ImagePaths) != 1 || turns.started.ImagePaths[0] != "C:/tmp/a.png" {
		t.Fatalf("ImagePaths=%v, want image passthrough", turns.started.ImagePaths)
	}
	if len(turns.started.Attachments) != 1 || turns.started.Attachments[0].Path != "C:/tmp/notes.txt" {
		t.Fatalf("Attachments=%+v, want attachment passthrough", turns.started.Attachments)
	}
	if out.ID != "turn-1" || out.SessionID != "sess-1" {
		t.Fatalf("turn=%+v, want turn-1/sess-1", out)
	}
	if events.seen.SessionID != "sess-1" || events.seen.TurnID != "turn-1" {
		t.Fatalf("event filter=%+v, want sess-1/turn-1", events.seen)
	}

	want := protocol.NewEvent(protocol.EventTypeItemDelta, protocol.EventOptions{
		EventID:   "evt-1",
		SessionID: "sess-1",
		RequestID: "turn-1",
		Payload:   map[string]any{"text": "hi"},
	})
	events.ch <- want
	select {
	case notification := <-notifier.ch:
		if notification.Method != protocoljsonrpc.NotificationEvent {
			t.Fatalf("notification.Method=%q, want %q", notification.Method, protocoljsonrpc.NotificationEvent)
		}
		var got protocol.Envelope
		if err := json.Unmarshal(notification.Params, &got); err != nil {
			t.Fatalf("notification params unmarshal error = %v", err)
		}
		if got.EventID != "evt-1" || got.EventType != protocol.EventTypeItemDelta {
			t.Fatalf("notification envelope=%+v, want evt-1 text.delta", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting forwarded event notification")
	}
}

func TestTurnInterruptOverJSONRPC(t *testing.T) {
	turns := &fakeTurnService{}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, turns: turns}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out map[string]bool
	ref := coreapi.TurnRef{SessionID: "sess-1", TurnID: "turn-1"}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodTurnInterrupt, ref, &out); err != nil {
		t.Fatalf("Call(turn/interrupt) error = %v", err)
	}
	if turns.interrupted != ref {
		t.Fatalf("interrupted=%+v, want %+v", turns.interrupted, ref)
	}
	if !out["ok"] {
		t.Fatalf("out=%v, want ok=true", out)
	}
}

func TestToolExecuteOverJSONRPCForwardsEvents(t *testing.T) {
	tools := &fakeToolExecutor{result: coreapi.ToolResult{
		Name:      "bash",
		RequestID: "tool-1",
		Status:    "success",
		Display:   "ok",
		Output:    json.RawMessage(`{"stdout":"ok"}`),
	}}
	events := &fakeEvents{ch: make(chan protocol.Envelope, 1)}
	defer close(events.ch)
	notifier := captureNotifier{ch: make(chan protocoljsonrpc.Notification, 1)}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		tools:    tools,
		events:   events,
	}, Options{Notifier: notifier}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var out coreapi.ToolResult
	req := coreapi.ToolRequest{
		SessionID: "sess-1",
		RequestID: "tool-1",
		Name:      "bash",
		Args:      json.RawMessage(`{"command":"echo ok"}`),
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodToolExecute, req, &out); err != nil {
		t.Fatalf("Call(tool/execute) error = %v", err)
	}
	if tools.seen.RequestID != "tool-1" || tools.seen.TurnID != "tool-1" || tools.seen.Name != "bash" {
		t.Fatalf("seen=%+v, want request id mirrored to turn id", tools.seen)
	}
	if events.seen.SessionID != "sess-1" || events.seen.TurnID != "tool-1" {
		t.Fatalf("event filter=%+v, want sess-1/tool-1", events.seen)
	}
	if out.Name != "bash" || out.Status != "success" || out.Display != "ok" {
		t.Fatalf("out=%+v, want bash success", out)
	}

	want := protocol.NewEvent(protocol.EventTypeTextFinal, protocol.EventOptions{
		EventID:   "evt-tool",
		SessionID: "sess-1",
		RequestID: "tool-1",
		Payload:   map[string]any{"text": "ok"},
	})
	events.ch <- want
	select {
	case notification := <-notifier.ch:
		if notification.Method != protocoljsonrpc.NotificationEvent {
			t.Fatalf("notification.Method=%q, want %q", notification.Method, protocoljsonrpc.NotificationEvent)
		}
		var got protocol.Envelope
		if err := json.Unmarshal(notification.Params, &got); err != nil {
			t.Fatalf("notification params unmarshal error = %v", err)
		}
		if got.EventID != "evt-tool" || got.EventType != protocol.EventTypeTextFinal {
			t.Fatalf("notification envelope=%+v, want evt-tool text.final", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting forwarded tool event notification")
	}
}

func TestToolExecuteCopiesTurnIDToRequestID(t *testing.T) {
	tools := &fakeToolExecutor{result: coreapi.ToolResult{Name: "bash", Status: "success"}}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{},
		sessions: &fakeSessionService{},
		tools:    tools,
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	if err := client.Call(context.Background(), protocoljsonrpc.MethodToolExecute, coreapi.ToolRequest{
		TurnID: "legacy-turn",
		Name:   "bash",
	}, nil); err != nil {
		t.Fatalf("Call(tool/execute) error = %v", err)
	}
	if tools.seen.RequestID != "legacy-turn" || tools.seen.TurnID != "legacy-turn" {
		t.Fatalf("seen=%+v, want legacy turn as request id", tools.seen)
	}
}

func TestToolTelemetryOverJSONRPC(t *testing.T) {
	telemetry := &fakeToolTelemetryService{
		traces: []coreapi.ToolTrace{{ID: "trace-1", Tool: "bash", Success: true, Duration: 10 * time.Millisecond}},
		stats:  []coreapi.ToolStat{{Tool: "bash", TotalCalls: 2, SuccessCalls: 2, AvgDuration: 5 * time.Millisecond}},
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, telemetry: telemetry}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var traces []coreapi.ToolTrace
	if err := client.Call(context.Background(), protocoljsonrpc.MethodToolTraces, nil, &traces); err != nil {
		t.Fatalf("Call(tool/traces) error = %v", err)
	}
	if len(traces) != 1 || traces[0].ID != "trace-1" || traces[0].Tool != "bash" {
		t.Fatalf("traces=%+v, want bash trace", traces)
	}
	var stats []coreapi.ToolStat
	if err := client.Call(context.Background(), protocoljsonrpc.MethodToolStats, nil, &stats); err != nil {
		t.Fatalf("Call(tool/stats) error = %v", err)
	}
	if len(stats) != 1 || stats[0].Tool != "bash" || stats[0].TotalCalls != 2 {
		t.Fatalf("stats=%+v, want bash stats", stats)
	}
}

func TestToolCatalogOverJSONRPC(t *testing.T) {
	catalog := &fakeToolCatalogService{
		definitions: []coreapi.ToolDefinition{
			{
				Name:        "bash",
				Description: "Execute a shell command",
				RiskLevel:   "high",
				Source:      "builtin",
				Category:    "shell",
				ReadOnly:    false,
				Invocable:   true,
				Params: map[string]coreapi.ToolParameterInfo{
					"command": {Type: "string", Required: true, Desc: "The command to execute"},
				},
				Tags: []string{"shell", "high"},
			},
			{
				Name:        "read",
				Description: "Read file contents",
				RiskLevel:   "low",
				Source:      "builtin",
				Category:    "filesystem",
				ReadOnly:    true,
				Invocable:   true,
				Tags:        []string{"filesystem", "low"},
			},
		},
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, toolCatalog: catalog}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})

	var defs []coreapi.ToolDefinition
	if err := client.Call(context.Background(), protocoljsonrpc.MethodToolCatalog, coreapi.ListToolCatalogRequest{WorkspaceRoot: "C:/work/demo"}, &defs); err != nil {
		t.Fatalf("Call(tool/catalog) error = %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("len(defs)=%d, want 2", len(defs))
	}
	if defs[0].Name != "bash" || defs[0].RiskLevel != "high" || defs[0].Source != "builtin" {
		t.Fatalf("defs[0]=%+v, want bash builtin high", defs[0])
	}
	if defs[0].Params == nil || defs[0].Params["command"].Type != "string" {
		t.Fatalf("defs[0].Params=%+v, want command:string", defs[0].Params)
	}
	if defs[1].Name != "read" || defs[1].RiskLevel != "low" || !defs[1].ReadOnly {
		t.Fatalf("defs[1]=%+v, want read low readonly", defs[1])
	}
}

func TestToolCatalogOverJSONRPCUnsupported(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	req, err := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req_1"), protocoljsonrpc.MethodToolCatalog, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp := router.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("response error = nil, want internal error")
	}
	if resp.Error.Code != protocoljsonrpc.CodeInternalError {
		t.Fatalf("error code=%d, want %d", resp.Error.Code, protocoljsonrpc.CodeInternalError)
	}
	if !strings.Contains(resp.Error.Message, "unsupported") {
		t.Fatalf("error message=%q, want contains 'unsupported'", resp.Error.Message)
	}
}

func TestSandboxPolicyOverJSONRPC(t *testing.T) {
	svc := &fakeSandboxService{
		policy: sandbox.Policy{Mode: sandbox.ModeReadOnly, WorkspaceRoot: "C:/work/demo"},
		status: sandbox.BackendStatus{GOOS: "windows", Backend: "path-broker", Degraded: true, Reason: "visible"},
	}
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}, sandbox: svc}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	client := protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router})
	var policy sandbox.Policy
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSandboxPolicy, coreapi.SandboxPolicyRequest{SessionID: "sess-1"}, &policy); err != nil {
		t.Fatalf("Call(sandbox/policy) error = %v", err)
	}
	if svc.seenRef.SessionID != "sess-1" {
		t.Fatalf("seenRef=%+v, want sess-1", svc.seenRef)
	}
	if policy.Mode != sandbox.ModeReadOnly || policy.WorkspaceRoot != "C:/work/demo" {
		t.Fatalf("policy=%+v, want read-only C:/work/demo", policy)
	}

	var ok map[string]bool
	set := coreapi.SetSandboxPolicyRequest{
		SessionID: "sess-2",
		Policy:    sandbox.Policy{Mode: sandbox.ModeDangerFullAccess},
	}
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSandboxSetPolicy, set, &ok); err != nil {
		t.Fatalf("Call(sandbox/set_policy) error = %v", err)
	}
	if !ok["ok"] {
		t.Fatalf("ok=%v, want true", ok)
	}
	if svc.setRef.SessionID != "sess-2" || svc.setPolicy.Mode != sandbox.ModeDangerFullAccess {
		t.Fatalf("setRef=%+v setPolicy=%+v, want sess-2 danger", svc.setRef, svc.setPolicy)
	}

	var status sandbox.BackendStatus
	if err := client.Call(context.Background(), protocoljsonrpc.MethodSandboxBackend, nil, &status); err != nil {
		t.Fatalf("Call(sandbox/backend_status) error = %v", err)
	}
	if status.Backend != "path-broker" || !status.Degraded {
		t.Fatalf("status=%+v, want visible degraded path-broker", status)
	}
}

func TestInvalidParamsReturnInvalidParams(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{state: fakeStateService{}, sessions: &fakeSessionService{}}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	req := protocoljsonrpc.Request{
		ID:     protocoljsonrpc.StringID("req_1"),
		Method: protocoljsonrpc.MethodSessionList,
		Params: json.RawMessage(`"not an object"`),
	}
	resp := router.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("response error = nil, want invalid params")
	}
	if resp.Error.Code != protocoljsonrpc.CodeInvalidParams {
		t.Fatalf("error code=%d, want %d", resp.Error.Code, protocoljsonrpc.CodeInvalidParams)
	}
}

func TestEngineErrorsReturnInternalError(t *testing.T) {
	router := protocoljsonrpc.NewRouter()
	if err := Register(router, fakeEngine{
		state:    fakeStateService{err: errors.New("boom")},
		sessions: &fakeSessionService{},
	}, Options{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	req, err := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req_1"), protocoljsonrpc.MethodStateSnapshot, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	resp := router.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("response error = nil, want internal error")
	}
	if resp.Error.Code != protocoljsonrpc.CodeInternalError {
		t.Fatalf("error code=%d, want %d", resp.Error.Code, protocoljsonrpc.CodeInternalError)
	}
}

func TestWireShapeOmitsJSONRPCField(t *testing.T) {
	req, err := protocoljsonrpc.NewRequest(protocoljsonrpc.StringID("req_1"), protocoljsonrpc.MethodInitialize, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	data, err := protocoljsonrpc.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(data), "jsonrpc") {
		t.Fatalf("request should not contain jsonrpc field: %s", data)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

var _ coreapi.SandboxService = (*fakeSandboxService)(nil)
var _ coreapi.EventSubscriber = (*fakeEvents)(nil)
var _ coreapi.WorkspaceService = (*fakeWorkspaceService)(nil)
var _ coreapi.MCPService = (*fakeMCPService)(nil)
var _ coreapi.LSPService = (*fakeLSPService)(nil)
var _ coreapi.ConfigService = (*fakeConfigService)(nil)
var _ coreapi.PermissionService = (*fakePermissionService)(nil)
var _ coreapi.ExtensionService = (*fakeExtensionService)(nil)
var _ coreapi.ContextService = (*fakeContextService)(nil)
var _ coreapi.UsageService = (*fakeUsageService)(nil)
var _ coreapi.VersionService = (*fakeVersionService)(nil)
var _ coreapi.TaskService = (*fakeTaskService)(nil)
var _ coreapi.ModeService = (*fakeModeService)(nil)
var _ coreapi.ModelService = (*fakeModelService)(nil)
var _ coreapi.RemoteWorkspaceService = (*fakeRemoteWorkspaceService)(nil)
var _ coreapi.GitService = (*fakeGitService)(nil)
var _ coreapi.InsightService = (*fakeInsightService)(nil)
var _ coreapi.MemoryService = (*fakeMemoryService)(nil)
var _ coreapi.RoleService = (*fakeRoleService)(nil)
var _ coreapi.ToolTelemetryService = (*fakeToolTelemetryService)(nil)
