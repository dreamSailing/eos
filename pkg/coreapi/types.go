package coreapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dreamSailing/eos/pkg/protocol"
	"github.com/dreamSailing/eos/pkg/sandbox"
)

var ErrUnsupported = errors.New("coreapi unsupported operation")

type Engine interface {
	State() StateService
	Workspaces() WorkspaceService
	Sessions() SessionService
	MCP() MCPService
	LSP() LSPService
	Config() ConfigService
	Permissions() PermissionService
	Extensions() ExtensionService
	Context() ContextService
	Usage() UsageService
	Versions() VersionService
	Tasks() TaskService
	Modes() ModeService
	Models() ModelService
	RemoteWorkspaces() RemoteWorkspaceService
	Git() GitService
	Insights() InsightService
	Memory() MemoryService
	Roles() RoleService
	Turns() TurnService
	Approvals() ApprovalService
	Inquiries() InquiryService
	Agents() AgentService
	Tools() ToolExecutor
	ToolCatalog() ToolCatalogService
	ToolTelemetry() ToolTelemetryService
	Events() EventSubscriber
	Sandbox() SandboxService
	Diagnostics() DiagnosticsService
}

type StateService interface {
	Snapshot(context.Context, StateSnapshotRequest) (StateSnapshot, error)
}

type WorkspaceService interface {
	List(context.Context, WorkspaceListRequest) ([]Workspace, error)
	Default(context.Context) (string, error)
	Last(context.Context) (string, error)
	ResolveForeground(context.Context, ResolveForegroundWorkspaceRequest) (string, error)
	Remember(context.Context, RememberWorkspaceRequest) error
	Forget(context.Context, WorkspacePathRequest) error
	Add(context.Context, WorkspacePathRequest) error
	Remove(context.Context, WorkspacePathRequest) error
	Use(context.Context, WorkspacePathRequest) error
	SetForeground(context.Context, WorkspacePathRequest) error
	Trust(context.Context, WorkspacePathRequest) error
	ListWorktrees(context.Context) ([]Worktree, error)
	CreateWorktree(context.Context, CreateWorktreeRequest) (Worktree, error)
	RemoveWorktree(context.Context, RemoveWorktreeRequest) error
}

type SessionService interface {
	Create(context.Context, CreateSessionRequest) (Session, error)
	Resume(context.Context, ResumeSessionRequest) (Session, error)
	List(context.Context, ListSessionsRequest) ([]Session, error)
	Current(context.Context, CurrentSessionRequest) (Session, error)
	SetCurrent(context.Context, SetCurrentSessionRequest) error
	Delete(context.Context, DeleteSessionRequest) error
	Rename(context.Context, RenameSessionRequest) (Session, error)
	SetMeta(context.Context, SetSessionMetaRequest) (Session, error)
	LoadMessages(context.Context, LoadSessionMessagesRequest) ([]SessionMessage, error)
	SaveMessages(context.Context, SaveSessionMessagesRequest) (Session, error)
}

type MCPService interface {
	List(context.Context) ([]MCPServer, error)
	Upsert(context.Context, UpsertMCPRequest) error
	ImportJSON(context.Context, ImportMCPJSONRequest) error
	Delete(context.Context, MCPNameRequest) error
	SetEnabled(context.Context, SetMCPEnabledRequest) error
}

type LSPService interface {
	List(context.Context) ([]LSPServer, error)
	Detect(context.Context, LSPLanguageRequest) (string, error)
	Start(context.Context, LSPLanguageRequest) (string, error)
	Diagnostics(context.Context) ([]string, error)
	DiagnosticsSummary(context.Context) (LSPDiagnosticsSummary, error)
}

type ConfigService interface {
	GetRules(context.Context) (string, error)
	RulesSnapshot(context.Context) (RulesSnapshot, error)
	SaveRules(context.Context, SaveRulesRequest) error
	ResetRules(context.Context) error
	GetSettings(context.Context) (Settings, error)
	SaveSettings(context.Context, Settings) error
}

type PermissionService interface {
	Snapshot(context.Context) (PermissionSnapshot, error)
	PendingReview(context.Context) (PendingReview, error)
	ClearPendingReview(context.Context) error
	SetAccessMode(context.Context, SetModeRequest) error
	SetApprovalMode(context.Context, SetModeRequest) error
}

type ExtensionService interface {
	ListSkills(context.Context) ([]SkillInfo, error)
	ReloadSkills(context.Context) error
	SetSkillEnabled(context.Context, SetExtensionEnabledRequest) error
	InvokeSkill(context.Context, InvokeSkillRequest) (InvokeSkillResult, error)
	ListPlugins(context.Context) ([]PluginInfo, error)
	SetPluginEnabled(context.Context, SetExtensionEnabledRequest) error
	BrowserStatus(context.Context) (BrowserStatus, error)
}

type ContextService interface {
	Preview(context.Context) ([]string, error)
	Stats(context.Context) (ContextStats, error)
	WindowTokens(context.Context) (int, error)
	PinDocument(context.Context, PinDocumentRequest) error
	Compact(context.Context) (string, error)
	Clear(context.Context) error
	Export(context.Context, ExportContextRequest) error
}

type UsageService interface {
	Summary(context.Context) (UsageSummary, error)
	CostSummary(context.Context) (string, error)
	CostItems(context.Context) ([]CostItem, error)
}

type VersionService interface {
	List(context.Context) ([]VersionItem, error)
	Rollback(context.Context, VersionIDRequest) error
	Delete(context.Context, VersionIDRequest) error
	DeleteFile(context.Context, VersionFileRequest) (int, error)
	Clear(context.Context) (int, error)
}

type TaskService interface {
	List(context.Context) ([]TaskSnapshot, error)
	Todos(context.Context) ([]TodoItem, error)
	Tail(context.Context, TaskIDRequest) ([]string, error)
	Kill(context.Context, TaskIDRequest) error
	Cleanup(context.Context) (int, error)
}

type ModeService interface {
	Snapshot(context.Context) (ModeSnapshot, error)
	SetExecutionMode(context.Context, SetModeRequest) error
	SetSandboxMode(context.Context, SetModeRequest) error
	SetReasoningLevel(context.Context, SetModeRequest) error
}

type ModelService interface {
	List(context.Context) ([]ModelConfig, error)
	Catalog(context.Context) (ModelCatalogState, error)
	Upsert(context.Context, UpsertModelRequest) error
	Save(context.Context, ModelSaveRequest) error
	Delete(context.Context, ModelNameRequest) error
	Activate(context.Context, ModelNameRequest) error
	SyncEnv(context.Context) error
	Context(context.Context, ModelContextRequest) (ModelContextSnapshot, error)
	SetWorkspace(context.Context, SetWorkspaceModelRequest) error
	ClearWorkspace(context.Context, ClearWorkspaceModelRequest) error
	SetSession(context.Context, SetSessionModelRequest) error
	ClearSession(context.Context, ClearSessionModelRequest) error
}

type RemoteWorkspaceService interface {
	List(context.Context) ([]RemoteWorkspace, error)
	Open(context.Context, RemoteWorkspaceRef) (RemoteWorkspace, error)
	Forget(context.Context, RemoteWorkspaceRef) error
	ClearCache(context.Context, RemoteWorkspaceRef) error
	CurrentRepo(context.Context) (RemoteRepoState, bool, error)
}

type GitService interface {
	Status(context.Context, GitStatusRequest) ([]GitChange, error)
	Diff(context.Context, GitDiffRequest) (GitTextResult, error)
	Branches(context.Context, GitBranchesRequest) (GitBranchesResult, error)
	Log(context.Context, GitLogRequest) (GitLogResult, error)
	Show(context.Context, GitShowRequest) (GitShowResult, error)
}

type InsightService interface {
	PredictNextUserMessage(context.Context, PredictNextUserMessageRequest) (string, error)
	PlanSnapshot(context.Context) (PlanSnapshot, error)
	MemorySnapshot(context.Context) (MemorySnapshot, error)
}

type MemoryService interface {
	Snapshot(context.Context) (MemorySnapshot, error)
	Save(context.Context, SaveMemoryRequest) error
	RebuildIndex(context.Context) error
	RecordAdd(context.Context, AddMemoryRecordRequest) (MemoryRecord, error)
	RecordList(context.Context, ListMemoryRecordsRequest) ([]MemoryRecord, error)
	RecordSearch(context.Context, SearchMemoryRecordsRequest) ([]MemoryRecord, error)
	RecordDelete(context.Context, DeleteMemoryRecordRequest) error
}

type RoleService interface {
	List(context.Context) ([]RoleConfig, error)
	Resolve(context.Context, RoleRef) (RoleConfig, error)
}

type TurnService interface {
	Start(context.Context, StartTurnRequest) (Turn, error)
	Interrupt(context.Context, TurnRef) error
}

type ApprovalService interface {
	Respond(context.Context, ApprovalResponse) error
}

type InquiryService interface {
	Respond(context.Context, InquiryResponse) error
}

type AgentService interface {
	Spawn(context.Context, SpawnAgentRequest) (Agent, error)
	SendInput(context.Context, AgentInput) error
	Wait(context.Context, AgentRef) (Agent, error)
	Run(context.Context, RunAgentRequest) (AgentRunResult, error)
	RunTool(context.Context, AgentToolRequest) (AgentToolResult, error)
	List(context.Context, ListAgentsRequest) ([]Agent, error)
	Close(context.Context, AgentRef) error
}

type ToolExecutor interface {
	Execute(context.Context, ToolRequest) (ToolResult, error)
}

type ToolCatalogService interface {
	List(context.Context, ListToolCatalogRequest) ([]ToolDefinition, error)
}

type ToolTelemetryService interface {
	Traces(context.Context) ([]ToolTrace, error)
	Stats(context.Context) ([]ToolStat, error)
}

// EventSubscriber 订阅运行时事件流。
// 生产 Engine（包括 Rust sidecar RemoteEngine）只需暴露此接口。
type EventSubscriber interface {
	Subscribe(context.Context, EventFilter) (<-chan protocol.Envelope, error)
}

// EventPublisher 向运行时事件总线发布事件。
// 仅 legacy/in-process 引擎实现此接口；生产 RemoteEngine 不提供 Publish 能力。
type EventPublisher interface {
	Publish(context.Context, protocol.Envelope) error
}

// EventBus 是 EventSubscriber + EventPublisher 的联合接口，
// 仅供 legacy/in-process 引擎与测试 fake 使用。
// 新代码应优先依赖 EventSubscriber。
type EventBus interface {
	EventSubscriber
	EventPublisher
}

type SandboxService interface {
	Policy(context.Context, SessionRef) (sandbox.Policy, error)
	SetPolicy(context.Context, SessionRef, sandbox.Policy) error
	BackendStatus(context.Context) sandbox.BackendStatus
}

type StartupDiagnosticsResult struct {
	BinaryPath      string `json:"binary_path"`
	ManifestVersion string `json:"manifest_version"`
	ProtocolVersion string `json:"protocol_version"`
	StoreDir        string `json:"store_dir"`
	SandboxBackend  string `json:"sandbox_backend"`
	MigrationMarker string `json:"migration_marker"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
}

type DiagnosticsService interface {
	Startup(context.Context) (StartupDiagnosticsResult, error)
}

type SessionRef struct {
	SessionID string `json:"session_id"`
}

type WorkspacePathRequest struct {
	Path string `json:"path"`
}

type RememberWorkspaceRequest struct {
	Path       string `json:"path"`
	Foreground bool   `json:"foreground,omitempty"`
}

type ResolveForegroundWorkspaceRequest struct {
	Preferred string `json:"preferred,omitempty"`
}

type CreateWorktreeRequest struct {
	Name string `json:"name"`
}

type RemoveWorktreeRequest struct {
	Path  string `json:"path"`
	Force bool   `json:"force,omitempty"`
}

type UpsertMCPRequest struct {
	Name                 string            `json:"name"`
	Type                 string            `json:"type,omitempty"`
	Target               string            `json:"target,omitempty"`
	Command              string            `json:"command,omitempty"`
	Args                 []string          `json:"args,omitempty"`
	Envs                 map[string]string `json:"envs,omitempty"`
	BaseURL              string            `json:"base_url,omitempty"`
	Enabled              bool              `json:"enabled"`
	Auth                 *MCPAuth          `json:"auth,omitempty"`
	ApprovalMode         string            `json:"approval_mode,omitempty"`
	ToolApprovalOverride map[string]string `json:"tool_approval_override,omitempty"`
}

type ImportMCPJSONRequest struct {
	Raw string `json:"raw"`
}

type MCPNameRequest struct {
	Name string `json:"name"`
}

type SetMCPEnabledRequest struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type LSPLanguageRequest struct {
	Language string `json:"language,omitempty"`
}

type SaveRulesRequest struct {
	Scope   string `json:"scope,omitempty"`
	Content string `json:"content"`
}

type SetExtensionEnabledRequest struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type ExportContextRequest struct {
	Path string `json:"path"`
}

type VersionIDRequest struct {
	ID string `json:"id"`
}

type VersionFileRequest struct {
	File string `json:"file"`
}

type TaskIDRequest struct {
	TaskID string `json:"task_id"`
}

type SetModeRequest struct {
	Mode string `json:"mode"`
}

type UpsertModelRequest struct {
	Name    string `json:"name"`
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key,omitempty"`
	Model   string `json:"model"`
}

type ModelSaveRequest struct {
	OriginalName            string `json:"original_name,omitempty"`
	Mode                    string `json:"mode"`
	ProviderID              string `json:"provider_id,omitempty"`
	PresetID                string `json:"preset_id,omitempty"`
	Name                    string `json:"name"`
	APIKey                  string `json:"api_key,omitempty"`
	APIBase                 string `json:"api_base,omitempty"`
	Model                   string `json:"model,omitempty"`
	// 自定义模型能力开关。指针类型，nil = 由 core 用默认值（推理+工具开、视觉关）。
	// preset 模式下忽略，能力从 preset 继承。
	SupportsReasoningEffort *bool `json:"supports_reasoning_effort,omitempty"`
	SupportsVision          *bool `json:"supports_vision,omitempty"`
	SupportsTools           *bool `json:"supports_tools,omitempty"`
}

type ModelNameRequest struct {
	Name string `json:"name"`
}

type ModelContextRequest struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
}

type ModelContextSnapshot struct {
	WorkspaceRoot      string `json:"workspace_root,omitempty"`
	SessionID          string `json:"session_id,omitempty"`
	GlobalDefaultName  string `json:"global_default_name,omitempty"`
	WorkspaceModelName string `json:"workspace_model_name,omitempty"`
	SessionModelName   string `json:"session_model_name,omitempty"`
	ResolvedModelName  string `json:"resolved_model_name,omitempty"`
	ResolvedScope      string `json:"resolved_scope,omitempty"`
}

type SetWorkspaceModelRequest struct {
	WorkspaceRoot string `json:"workspace_root"`
	ModelName     string `json:"model_name"`
}

type ClearWorkspaceModelRequest struct {
	WorkspaceRoot string `json:"workspace_root"`
}

type SetSessionModelRequest struct {
	SessionID string `json:"session_id"`
	ModelName string `json:"model_name"`
}

type ClearSessionModelRequest struct {
	SessionID string `json:"session_id"`
}

type RemoteWorkspaceRef struct {
	IDOrPath string `json:"id_or_path"`
}

type PredictNextUserMessageRequest struct {
	Draft string `json:"draft,omitempty"`
}

type SaveMemoryRequest struct {
	Scope   string `json:"scope,omitempty"`
	Content string `json:"content"`
}

type MemoryRecord struct {
	ID            string    `json:"id"`
	Scope         string    `json:"scope"`
	WorkspaceRoot string    `json:"workspace_root,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
	Kind          string    `json:"kind,omitempty"`
	Content       string    `json:"content"`
	Tags          []string  `json:"tags,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Source        string    `json:"source,omitempty"`
}

type AddMemoryRecordRequest struct {
	Scope         string   `json:"scope"`
	Kind          string   `json:"kind,omitempty"`
	Content       string   `json:"content"`
	Tags          []string `json:"tags,omitempty"`
	Source        string   `json:"source,omitempty"`
	WorkspaceRoot string   `json:"workspace_root,omitempty"`
	SessionID     string   `json:"session_id,omitempty"`
}

type ListMemoryRecordsRequest struct {
	Scope string   `json:"scope,omitempty"`
	Kind  string   `json:"kind,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

type SearchMemoryRecordsRequest struct {
	Keywords []string `json:"keywords,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Scope    string   `json:"scope,omitempty"`
	Kind     string   `json:"kind,omitempty"`
}

type DeleteMemoryRecordRequest struct {
	ID string `json:"id"`
}

type RoleRef struct {
	ID string `json:"id"`
}

type SandboxPolicyRequest struct {
	SessionID string `json:"session_id,omitempty"`
}

type SetSandboxPolicyRequest struct {
	SessionID string         `json:"session_id,omitempty"`
	Policy    sandbox.Policy `json:"policy"`
}

type TurnRef struct {
	SessionID string `json:"session_id"`
	TurnID    string `json:"turn_id"`
}

type AgentRef struct {
	AgentID string `json:"agent_id"`
}

type CreateSessionRequest struct {
	WorkspaceRoot string           `json:"workspace_root,omitempty"`
	Title         string           `json:"title,omitempty"`
	Messages      []SessionMessage `json:"messages,omitempty"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
}

type ResumeSessionRequest struct {
	SessionID     string `json:"session_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type ListSessionsRequest struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	// Source, when non-empty, restricts results to sessions whose metadata.source
	// equals this value. Sessions without a source tag are "unknown" and never
	// match a concrete source. Empty = return all sessions.
	Source string `json:"source,omitempty"`
	// IncludeArchived, when true, includes archived sessions. Default false.
	IncludeArchived bool `json:"include_archived,omitempty"`
}

// StateSnapshotRequest parameters for state/snapshot.
type StateSnapshotRequest struct {
	// Source, when non-empty, restricts the snapshot's sessions and the
	// workspaces derived from them to the given client source. Empty = full
	// snapshot (all clients).
	Source string `json:"source,omitempty"`
	// IncludeArchived, when true, includes archived sessions. Default false.
	IncludeArchived bool `json:"include_archived,omitempty"`
}

// WorkspaceListRequest parameters for workspace/list.
type WorkspaceListRequest struct {
	// Source, when non-empty, restricts workspaces to those that have at least
	// one session of this source. Empty = all workspaces.
	Source string `json:"source,omitempty"`
}

type CurrentSessionRequest struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type SetCurrentSessionRequest struct {
	SessionID     string `json:"session_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type DeleteSessionRequest struct {
	SessionID     string `json:"session_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type RenameSessionRequest struct {
	SessionID     string `json:"session_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	Title         string `json:"title"`
}

// SetSessionMetaRequest updates (or deletes, when Value is nil) a single
// metadata entry on a session. Used for soft-state flags like "archived".
type SetSessionMetaRequest struct {
	SessionID     string          `json:"session_id"`
	WorkspaceRoot string          `json:"workspace_root,omitempty"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value,omitempty"`
}

type LoadSessionMessagesRequest struct {
	SessionID     string `json:"session_id"`
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type SaveSessionMessagesRequest struct {
	SessionID     string           `json:"session_id,omitempty"`
	WorkspaceRoot string           `json:"workspace_root,omitempty"`
	Messages      []SessionMessage `json:"messages,omitempty"`
}

type StartTurnRequest struct {
	SessionID   string          `json:"session_id"`
	TurnID      string          `json:"turn_id,omitempty"`
	Input       string          `json:"input"`
	ImagePaths  []string        `json:"image_paths,omitempty"`
	Attachments []Attachment    `json:"attachments,omitempty"`
	Options     json.RawMessage `json:"options,omitempty"`
}

type Attachment struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path"`
	MIME string `json:"mime,omitempty"`
	Kind string `json:"kind,omitempty"`
}

// ApprovalDecision is the typed wire approval decision (camelCase on the wire),
// mirroring eos-core's ApprovalDecision / Codex's CommandExecutionApprovalDecision
// common variants. Command-only amendment variants are omitted: eos has no
// execpolicy/network-amendment approval flows, so they would be dead code.
type ApprovalDecision string

const (
	// ApprovalAccept approves the operation once.
	ApprovalAccept ApprovalDecision = "accept"
	// ApprovalAcceptForSession approves the operation and records a
	// session-scoped approval so matching operations run without re-prompting.
	ApprovalAcceptForSession ApprovalDecision = "acceptForSession"
	// ApprovalDecline denies the operation; the agent continues the turn.
	ApprovalDecline ApprovalDecision = "decline"
	// ApprovalCancel denies the operation and interrupts the turn.
	ApprovalCancel ApprovalDecision = "cancel"
)

type ApprovalResponse struct {
	ApprovalID string            `json:"approval_id"`
	Decision   ApprovalDecision  `json:"decision"`
	Reason     string            `json:"reason,omitempty"`
	Metadata   map[string]any    `json:"metadata,omitempty"`
}

type InquiryResponse struct {
	InquiryID string `json:"inquiry_id"`
	Option    string `json:"option,omitempty"`
	Text      string `json:"text,omitempty"`
}

type SpawnAgentRequest struct {
	ParentAgentID   string          `json:"parent_agent_id,omitempty"`
	RoleID          string          `json:"role_id,omitempty"`
	Task            string          `json:"task"`
	ForkContextMode string          `json:"fork_context_mode,omitempty"`
	Options         json.RawMessage `json:"options,omitempty"`
}

type AgentInput struct {
	AgentID string `json:"agent_id"`
	Input   string `json:"input"`
}

type RunAgentRequest struct {
	AgentID   string          `json:"agent_id"`
	SessionID string          `json:"session_id,omitempty"`
	Options   json.RawMessage `json:"options,omitempty"`
}

type AgentToolRequest struct {
	AgentID   string          `json:"agent_id"`
	SessionID string          `json:"session_id,omitempty"`
	TurnID    string          `json:"turn_id,omitempty"`
	Name      string          `json:"name"`
	Args      json.RawMessage `json:"args,omitempty"`
}

type ListAgentsRequest struct {
	SessionID string `json:"session_id,omitempty"`
}

type ToolRequest struct {
	SessionID string          `json:"session_id,omitempty"`
	TurnID    string          `json:"turn_id,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	AgentID   string          `json:"agent_id,omitempty"`
	Name      string          `json:"name"`
	Args      json.RawMessage `json:"args,omitempty"`
}

type ToolResult struct {
	Name      string          `json:"name"`
	RequestID string          `json:"request_id,omitempty"`
	Status    string          `json:"status,omitempty"`
	Display   string          `json:"display,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
	Duration  time.Duration   `json:"duration,omitempty"`
}

type ToolTrace struct {
	ID         string        `json:"id,omitempty"`
	Tool       string        `json:"tool,omitempty"`
	StartTime  time.Time     `json:"start_time,omitempty"`
	EndTime    time.Time     `json:"end_time,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`
	Success    bool          `json:"success"`
	Cached     bool          `json:"cached"`
	RetryCount int           `json:"retry_count,omitempty"`
	ParentID   string        `json:"parent_id,omitempty"`
}

type ToolStat struct {
	Tool          string        `json:"tool,omitempty"`
	TotalCalls    int           `json:"total_calls"`
	SuccessCalls  int           `json:"success_calls"`
	FailureCalls  int           `json:"failure_calls"`
	CachedCalls   int           `json:"cached_calls"`
	RetriedCalls  int           `json:"retried_calls"`
	TotalDuration time.Duration `json:"total_duration,omitempty"`
	AvgDuration   time.Duration `json:"avg_duration,omitempty"`
}

type ListToolCatalogRequest struct {
	WorkspaceRoot string   `json:"workspace_root,omitempty"`
	IncludeTools  []string `json:"include_tools,omitempty"`
	AllowedTools  []string `json:"allowed_tools,omitempty"`
}

type ToolDefinition struct {
	Name               string                       `json:"name"`
	Description        string                       `json:"description,omitempty"`
	RiskLevel          string                       `json:"risk_level,omitempty"`
	Params             map[string]ToolParameterInfo `json:"params,omitempty"`
	Examples           []ToolExample                `json:"examples,omitempty"`
	Source             string                       `json:"source,omitempty"`
	Category           string                       `json:"category,omitempty"`
	VisibleIn          []string                     `json:"visible_in,omitempty"`
	ReadOnly           bool                         `json:"read_only"`
	Invocable          bool                         `json:"invocable"`
	RequiresFullAccess bool                         `json:"requires_full_access"`
	Tags               []string                     `json:"tags,omitempty"`
	Metadata           map[string]any               `json:"metadata,omitempty"`
}

type ToolParameterInfo struct {
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required"`
	Desc     string `json:"desc,omitempty"`
}

type ToolExample struct {
	Description string         `json:"description,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
}

type EventFilter struct {
	EventTypes []string `json:"event_types,omitempty"`
	SessionID  string   `json:"session_id,omitempty"`
	TurnID     string   `json:"turn_id,omitempty"`
	AgentID    string   `json:"agent_id,omitempty"`
}

type EventSubscribeRequest struct {
	EventTypes []string        `json:"event_types,omitempty"`
	SessionID  string          `json:"session_id,omitempty"`
	TurnID     string          `json:"turn_id,omitempty"`
	AgentID    string          `json:"agent_id,omitempty"`
	Filter     json.RawMessage `json:"filter,omitempty"`
}

type EventSubscription struct {
	ID             string   `json:"id,omitempty"`
	SubscriptionID string   `json:"subscription_id,omitempty"`
	EventTypes     []string `json:"event_types,omitempty"`
	Active         bool     `json:"active"`
	CreatedAt      string   `json:"created_at,omitempty"`
}

type EventUnsubscribeRequest struct {
	ID string `json:"id"`
}

type StateSnapshot struct {
	ForegroundWorkspace string              `json:"foreground_workspace,omitempty"`
	Workspaces          []WorkspaceSnapshot `json:"workspaces,omitempty"`
	Sessions            []SessionSnapshot   `json:"sessions,omitempty"`
	CurrentSession      *SessionSnapshot    `json:"current_session,omitempty"`
	Messages            []SessionMessage    `json:"messages,omitempty"`
	Tasks               []TaskSnapshot      `json:"tasks,omitempty"`
	Agents              []Agent             `json:"agents,omitempty"`
}

type Workspace struct {
	Path    string `json:"path"`
	Trusted bool   `json:"trusted"`
	Active  bool   `json:"active"`
}

type Worktree struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Branch    string `json:"branch,omitempty"`
	Head      string `json:"head,omitempty"`
	Active    bool   `json:"active"`
	Removable bool   `json:"removable"`
}

type MCPServer struct {
	Name                 string            `json:"name"`
	Type                 string            `json:"type"`
	Target               string            `json:"target,omitempty"`
	Command              string            `json:"command,omitempty"`
	Args                 []string          `json:"args,omitempty"`
	Envs                 map[string]string `json:"envs,omitempty"`
	BaseURL              string            `json:"base_url,omitempty"`
	Enabled              bool              `json:"enabled"`
	Auth                 *MCPAuth          `json:"auth,omitempty"`
	ApprovalMode         string            `json:"approval_mode,omitempty"`
	ToolApprovalOverride map[string]string `json:"tool_approval_override,omitempty"`
}

type MCPAuth struct {
	Type       string            `json:"type"`
	Token      string            `json:"token,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	HeadersEnv map[string]string `json:"headers_env,omitempty"`
}

type LSPServer struct {
	Language string `json:"language"`
	Status   string `json:"status"`
	Command  string `json:"command,omitempty"`
}

type LSPDiagnosticItem struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
	Code     string `json:"code,omitempty"`
}

type LSPDiagnosticsSummary struct {
	Files    int                 `json:"files"`
	Errors   int                 `json:"errors"`
	Warnings int                 `json:"warnings"`
	Infos    int                 `json:"infos"`
	Items    []LSPDiagnosticItem `json:"items,omitempty"`
}

type Settings struct {
	PlanPromptStyle      string `json:"plan_prompt_style,omitempty"`
	PlanBubbleColor      string `json:"plan_bubble,omitempty"`
	AutoContext          *bool  `json:"auto_context,omitempty"`
	DesktopNotifications *bool  `json:"desktop_notifications,omitempty"`
	MaxInjectKB          int    `json:"max_inject_kb,omitempty"`
	WatchMode            string `json:"watch_mode,omitempty"`
	WatchDebounceMs      int    `json:"watch_debounce_ms,omitempty"`
	PollIntervalSec      int    `json:"poll_interval_sec,omitempty"`
	Language             string `json:"language,omitempty"`
	Theme                string `json:"theme,omitempty"`
	Trusted              *bool  `json:"trusted,omitempty"`
	MaxTurnTokens        int    `json:"max_turn_tokens,omitempty"`
	MaxSessionTokens     int    `json:"max_session_tokens,omitempty"`
	MidRiskConfirm       bool   `json:"mid_risk_confirm,omitempty"`
}

type RuleDocument struct {
	Scope     string    `json:"scope,omitempty"`
	Path      string    `json:"path,omitempty"`
	Content   string    `json:"content,omitempty"`
	Exists    bool      `json:"exists"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type RulesSnapshot struct {
	ActiveRoot string         `json:"active_root,omitempty"`
	Documents  []RuleDocument `json:"documents,omitempty"`
}

type PermissionSnapshot struct {
	ExecutionMode           string   `json:"execution_mode,omitempty"`
	AccessMode              string   `json:"access_mode,omitempty"`
	ApprovalMode            string   `json:"approval_mode,omitempty"`
	SandboxMode             string   `json:"sandbox_mode,omitempty"`
	AllowAll                bool     `json:"allow_all"`
	AllowedCategories       []string `json:"allowed_categories,omitempty"`
	HasPendingDiff          bool     `json:"has_pending_diff"`
	PendingDiffPath         string   `json:"pending_diff_path,omitempty"`
	LastAuthorization       string   `json:"last_authorization,omitempty"`
	LastAuthorizationAt     string   `json:"last_authorization_at,omitempty"`
	LastAuthorizationKind   string   `json:"last_authorization_kind,omitempty"`
	LastAuthorizationNote   string   `json:"last_authorization_note,omitempty"`
	LastAuthorizationTarget string   `json:"last_authorization_target,omitempty"`
}

type PendingReview struct {
	Path    string `json:"path,omitempty"`
	Diff    string `json:"diff,omitempty"`
	HasDiff bool   `json:"has_diff"`
}

type SkillInfo struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description,omitempty"`
	Source                 string   `json:"source,omitempty"`
	ArgumentHint           string   `json:"argument_hint,omitempty"`
	Location               string   `json:"location,omitempty"`
	BaseDir                string   `json:"base_dir,omitempty"`
	AllowedTools           []string `json:"allowed_tools,omitempty"`
	Enabled                bool     `json:"enabled"`
	Active                 bool     `json:"active"`
	DisableModelInvocation bool     `json:"disable_model_invocation"`
	UserInvocable          bool     `json:"user_invocable"`
	UserInvocableDefined   bool     `json:"user_invocable_defined"`
}

type InvokeSkillRequest struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type InvokeSkillResult struct {
	Name    string `json:"name,omitempty"`
	Invoked bool   `json:"invoked"`
}

type PluginInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Command     string `json:"command,omitempty"`
	Enabled     bool   `json:"enabled"`
}

type BrowserStatus struct {
	ServerName  string `json:"server_name,omitempty"`
	Configured  bool   `json:"configured"`
	Enabled     bool   `json:"enabled"`
	Loaded      bool   `json:"loaded"`
	Tools       int    `json:"tools,omitempty"`
	LastError   string `json:"last_error,omitempty"`
	Command     string `json:"command,omitempty"`
	InstallHint string `json:"install_hint,omitempty"`
}

type ContextStats struct {
	MessageCount int `json:"message_count"`
	Estimated    int `json:"estimated"`
}

type PinDocumentRequest struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	TokenBudget int    `json:"token_budget,omitempty"`
}

type UsageSummary struct {
	Rounds             int      `json:"rounds"`
	InputTokens        *int     `json:"input_tokens,omitempty"`
	ReplyTokens        *int     `json:"reply_tokens,omitempty"`
	CachedInputTokens  *int     `json:"cached_input_tokens,omitempty"`
	TotalTokens        *int     `json:"total_tokens,omitempty"`
	CostUSD            *float64 `json:"cost_usd,omitempty"`
	UnknownUsageRounds int      `json:"unknown_usage_rounds"`
	UnknownCostRounds  int      `json:"unknown_cost_rounds"`
}

type CostItem struct {
	Time              time.Time `json:"time,omitempty"`
	Model             string    `json:"model,omitempty"`
	InputTokens       *int      `json:"input_tokens,omitempty"`
	ReplyTokens       *int      `json:"reply_tokens,omitempty"`
	CachedInputTokens *int      `json:"cached_input_tokens,omitempty"`
	TotalTokens       *int      `json:"total_tokens,omitempty"`
	CostUSD           *float64  `json:"cost_usd,omitempty"`
	UsageKnown        bool      `json:"usage_known"`
	CostKnown         bool      `json:"cost_known"`
}

type VersionItem struct {
	ID        string    `json:"id"`
	File      string    `json:"file,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Summary   string    `json:"summary,omitempty"`
}

type ModeSnapshot struct {
	ExecutionMode  string `json:"execution_mode,omitempty"`
	SandboxMode    string `json:"sandbox_mode,omitempty"`
	ReasoningLevel string `json:"reasoning_level,omitempty"`
}

type ModelConfig struct {
	Name                    string `json:"name"`
	APIBase                 string `json:"api_base,omitempty"`
	APIKeyMasked            string `json:"api_key_masked,omitempty"`
	Model                   string `json:"model,omitempty"`
	Source                  string `json:"source,omitempty"`
	Active                  bool   `json:"active"`
	SupportsReasoningEffort bool   `json:"supports_reasoning_effort"`
	SupportsVision          bool   `json:"supports_vision"`
	SupportsTools           bool   `json:"supports_tools"`
	ProviderID              string `json:"provider_id,omitempty"`
	Format                  string `json:"format,omitempty"`
	PresetID                string `json:"preset_id,omitempty"`
	EditKind                string `json:"edit_kind,omitempty"`
	CanEdit                 bool   `json:"can_edit"`
	CanDelete               bool   `json:"can_delete"`
}

// ProviderEndpoint 服务商一个接入端点 = (plan, format) 组合。
type ProviderEndpoint struct {
	Plan    string `json:"plan,omitempty"`
	Format  string `json:"format,omitempty"`
	APIBase string `json:"api_base,omitempty"`
}

type ModelProviderOption struct {
	ID            string             `json:"id"`
	Name          string             `json:"name,omitempty"`
	Website       string             `json:"website,omitempty"`
	APIKeyEnv     string             `json:"api_key_env,omitempty"`
	Endpoints     []ProviderEndpoint `json:"endpoints,omitempty"`
	DefaultModels []string           `json:"default_models,omitempty"`
}

type ModelPresetOption struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name,omitempty"`
	ProviderID              string   `json:"provider_id,omitempty"`
	ModelName               string   `json:"model_name,omitempty"`
	Plan                    string   `json:"plan,omitempty"`
	Format                  string   `json:"format,omitempty"`
	ContextWindow           int      `json:"context_window,omitempty"`
	Tags                    []string `json:"tags,omitempty"`
	Description             string   `json:"description,omitempty"`
	SupportsReasoningEffort bool     `json:"supports_reasoning_effort"`
	SupportsVision          bool     `json:"supports_vision"`
	SupportsImageGeneration bool     `json:"supports_image_generation"`
	SupportsVideoGeneration bool     `json:"supports_video_generation"`
	SupportsSpeechSynthesis bool     `json:"supports_speech_synthesis"`
	SupportsTools           bool     `json:"supports_tools"`
}

type ModelCatalogState struct {
	Providers           []ModelProviderOption `json:"providers,omitempty"`
	Presets             []ModelPresetOption   `json:"presets,omitempty"`
	AllowCustomProvider bool                  `json:"allow_custom_provider"`
	AllowCustomModel    bool                  `json:"allow_custom_model"`
}

type RemoteWorkspace struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind,omitempty"`
	Platform      string    `json:"platform,omitempty"`
	RepoURL       string    `json:"repo_url,omitempty"`
	Owner         string    `json:"owner,omitempty"`
	Repo          string    `json:"repo,omitempty"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	Branch        string    `json:"branch,omitempty"`
	Account       string    `json:"account,omitempty"`
	LocalPath     string    `json:"local_path,omitempty"`
	Active        bool      `json:"active"`
	Exists        bool      `json:"exists"`
	LastUsedAt    time.Time `json:"last_used_at,omitempty"`
}

type RemoteRepoState struct {
	Mode          string    `json:"mode,omitempty"`
	Platform      string    `json:"platform,omitempty"`
	RepoURL       string    `json:"repo_url,omitempty"`
	Owner         string    `json:"owner,omitempty"`
	Repo          string    `json:"repo,omitempty"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	WorkingBranch string    `json:"working_branch,omitempty"`
	LocalPath     string    `json:"local_path,omitempty"`
	AccountLogin  string    `json:"account_login,omitempty"`
	AccountName   string    `json:"account_name,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type GitStatusRequest struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type GitChange struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

type GitDiffRequest struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	Path          string `json:"path,omitempty"`
}

type GitTextResult struct {
	Text string `json:"text"`
}

type GitBranchesRequest struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
}

type GitBranchesResult struct {
	Current  string   `json:"current,omitempty"`
	Branches []string `json:"branches,omitempty"`
}

type GitLogRequest struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Oneline       bool   `json:"oneline,omitempty"`
	Graph         bool   `json:"graph,omitempty"`
	All           bool   `json:"all,omitempty"`
	Path          string `json:"path,omitempty"`
}

type GitLogEntry struct {
	Hash    string `json:"hash,omitempty"`
	Message string `json:"message,omitempty"`
}

type GitLogResult struct {
	Branch  string        `json:"branch,omitempty"`
	Entries []GitLogEntry `json:"entries,omitempty"`
	Text    string        `json:"text"`
}

type GitShowRequest struct {
	WorkspaceRoot string `json:"workspace_root,omitempty"`
	Revision      string `json:"revision,omitempty"`
	Path          string `json:"path,omitempty"`
}

type GitShowResult struct {
	Branch   string `json:"branch,omitempty"`
	Revision string `json:"revision,omitempty"`
	Text     string `json:"text"`
}

type PlanSnapshot struct {
	HasPlan          bool      `json:"has_plan"`
	Content          string    `json:"content,omitempty"`
	WorkspaceCurrent string    `json:"workspace_current,omitempty"`
	UserLatest       string    `json:"user_latest,omitempty"`
	UserSnapshot     string    `json:"user_snapshot,omitempty"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

type MemoryDocument struct {
	Scope     string    `json:"scope,omitempty"`
	Path      string    `json:"path,omitempty"`
	Exists    bool      `json:"exists"`
	Content   string    `json:"content,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type MemorySnapshot struct {
	Documents []MemoryDocument `json:"documents,omitempty"`
}

type RoleConfig struct {
	ID              string   `json:"id"`
	Description     string   `json:"description,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	PromptFile      string   `json:"prompt_file,omitempty"`
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	ContextStrategy string   `json:"context_strategy,omitempty"`
	Model           string   `json:"model,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
	LegacyAliases   []string `json:"legacy_aliases,omitempty"`
}

type WorkspaceSnapshot struct {
	Path             string `json:"path"`
	Name             string `json:"name,omitempty"`
	Trusted          bool   `json:"trusted"`
	Active           bool   `json:"active"`
	SessionCount     int    `json:"session_count"`
	CurrentSessionID string `json:"current_session_id,omitempty"`
}

type SessionSnapshot struct {
	ID             string    `json:"id"`
	WorkspacePath  string    `json:"workspace_path,omitempty"`
	Title          string    `json:"title,omitempty"`
	Preview        string    `json:"preview,omitempty"`
	// Source is the originating client (e.g. "cli", "gui"); empty when the
	// session predates source tagging. Mirrors Session.metadata["source"].
	Source         string    `json:"source,omitempty"`
	// Archived is true when the session is soft-hidden (metadata.archived).
	Archived       bool      `json:"archived,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
	Running        bool      `json:"running"`
	NeedsAttention bool      `json:"needs_attention"`
	MessageCount   int       `json:"message_count"`
	PendingPrompts int       `json:"pending_prompts"`
	Active         bool      `json:"active"`
}

type SessionMessage struct {
	Role       string         `json:"role"`
	Type       string         `json:"type,omitempty"`
	Content    string         `json:"content,omitempty"`
	Time       time.Time      `json:"time,omitempty"`
	ImagePaths []string       `json:"image_paths,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type TaskSnapshot struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind,omitempty"`
	Status    string         `json:"status"`
	StartedAt time.Time      `json:"started_at"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
	EndedAt   time.Time      `json:"ended_at,omitempty"`
	Label     string         `json:"label,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	CanKill   bool           `json:"can_kill"`
	CanResume bool           `json:"can_resume,omitempty"`
	CanClose  bool           `json:"can_close,omitempty"`
	Workspace string         `json:"workspace,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type TodoItem struct {
	ID        string    `json:"id,omitempty"`
	Content   string    `json:"content,omitempty"`
	Status    string    `json:"status,omitempty"`
	Priority  any       `json:"priority,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type Session struct {
	ID            string         `json:"id"`
	WorkspaceRoot string         `json:"workspace_root,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type Turn struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Agent struct {
	ID            string    `json:"id"`
	ParentAgentID string    `json:"parent_agent_id,omitempty"`
	RoleID        string    `json:"role_id"`
	Task          string    `json:"task,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AgentRunResult struct {
	Agent    Agent                 `json:"agent"`
	Role     RoleConfig            `json:"role,omitempty"`
	Messages []AgentMailboxMessage `json:"messages,omitempty"`
	Output   string                `json:"output,omitempty"`
}

type AgentToolResult struct {
	Name    string          `json:"name"`
	Display string          `json:"display,omitempty"`
	Output  json.RawMessage `json:"output,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type AgentMailboxMessage struct {
	FromAgentID string    `json:"from_agent_id,omitempty"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}
