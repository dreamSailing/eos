package jsonrpc

const (
	MethodInitialize                 = "initialize"
	MethodShutdown                   = "shutdown"
	MethodWorkspaceList              = "workspace/list"
	MethodWorkspaceDefault           = "workspace/default"
	MethodWorkspaceLast              = "workspace/last"
	MethodWorkspaceResolve           = "workspace/resolve_foreground"
	MethodWorkspaceRemember          = "workspace/remember"
	MethodWorkspaceForget            = "workspace/forget"
	MethodWorkspaceAdd               = "workspace/add"
	MethodWorkspaceRemove            = "workspace/remove"
	MethodWorkspaceUse               = "workspace/use"
	MethodWorkspaceSetForeground     = "workspace/set_foreground"
	MethodWorkspaceTrust             = "workspace/trust"
	MethodWorkspaceWorktreeList      = "workspace/worktree/list"
	MethodWorkspaceWorktreeCreate    = "workspace/worktree/create"
	MethodWorkspaceWorktreeRemove    = "workspace/worktree/remove"
	MethodWorkspaceChanges           = "workspace/changes"
	MethodWorkspaceRollbackBuild     = "workspace/rollback/build"
	MethodWorkspaceRollbackApply     = "workspace/rollback/apply"
	MethodSessionCreate              = "session/create"
	MethodSessionResume              = "session/resume"
	MethodSessionList                = "session/list"
	MethodSessionCurrent             = "session/current"
	MethodSessionSetCurrent          = "session/set_current"
	MethodSessionDelete              = "session/delete"
	MethodSessionRename              = "session/rename"
	MethodSessionSetMeta             = "session/set_meta"
	MethodSessionMessagesLoad        = "session/messages/load"
	MethodSessionMessagesSave        = "session/messages/save"
	MethodMCPList                    = "mcp/list"
	MethodMCPUpsert                  = "mcp/upsert"
	MethodMCPImportJSON              = "mcp/import_json"
	MethodMCPDelete                  = "mcp/delete"
	MethodMCPSetEnabled              = "mcp/set_enabled"
	MethodLSPList                    = "lsp/list"
	MethodLSPDetect                  = "lsp/detect"
	MethodLSPStart                   = "lsp/start"
	MethodLSPDiagnostics             = "lsp/diagnostics"
	MethodLSPDiagnosticsSummary      = "lsp/diagnostics/summary"
	MethodConfigRulesGet             = "config/rules/get"
	MethodConfigRulesSnapshot        = "config/rules/snapshot"
	MethodConfigRulesSave            = "config/rules/save"
	MethodConfigRulesReset           = "config/rules/reset"
	MethodConfigSettingsGet          = "config/settings/get"
	MethodConfigSettingsSave         = "config/settings/save"
	MethodPermissionSnapshot         = "permission/snapshot"
	MethodPermissionPendingReview    = "permission/pending_review"
	MethodPermissionClearReview      = "permission/clear_pending_review"
	MethodPermissionAccessModeSet    = "permission/access_mode/set"
	MethodPermissionApprovalModeSet  = "permission/approval_mode/set"
	MethodExtensionsSkillsList       = "extensions/skills/list"
	MethodExtensionsSkillsReload     = "extensions/skills/reload"
	MethodExtensionsSkillSetEnabled  = "extensions/skill/set_enabled"
	MethodExtensionsSkillInvoke      = "extensions/skill/invoke"
	MethodExtensionsPluginsList      = "extensions/plugins/list"
	MethodExtensionsPluginSetEnabled = "extensions/plugin/set_enabled"
	MethodBrowserStatus              = "browser/status"
	MethodContextPreview             = "context/preview"
	MethodContextStats               = "context/stats"
	MethodContextWindow              = "context/window"
	MethodContextPin                 = "context/pin"
	MethodContextCompact             = "context/compact"
	MethodContextClear               = "context/clear"
	MethodContextExport              = "context/export"
	MethodUsageSummary               = "usage/summary"
	MethodUsageCostSummary           = "usage/cost_summary"
	MethodUsageCostItems             = "usage/cost_items"
	MethodVersionsList               = "versions/list"
	MethodVersionsRollback           = "versions/rollback"
	MethodVersionsDelete             = "versions/delete"
	MethodVersionsDeleteFile         = "versions/delete_file"
	MethodVersionsClear              = "versions/clear"
	MethodTaskList                   = "task/list"
	MethodTaskTodos                  = "task/todos"
	MethodTaskTail                   = "task/tail"
	MethodTaskKill                   = "task/kill"
	MethodTaskCleanup                = "task/cleanup"
	MethodRuntimeModesGet            = "runtime/modes/get"
	MethodRuntimeExecutionModeSet    = "runtime/execution_mode/set"
	MethodRuntimeSandboxModeSet      = "runtime/sandbox_mode/set"
	MethodRuntimeReasoningLevelSet   = "runtime/reasoning_level/set"
	MethodModelList                  = "model/list"
	MethodModelCatalog               = "model/catalog"
	MethodModelUpsert                = "model/upsert"
	MethodModelSave                  = "model/save"
	MethodModelDelete                = "model/delete"
	MethodModelActivate              = "model/activate"
	MethodModelSyncEnv               = "model/sync_env"
	MethodModelReload                = "model/reload"
	MethodModelContext               = "model/context"
	MethodModelWorkspaceSet          = "model/workspace/set"
	MethodModelWorkspaceClear        = "model/workspace/clear"
	MethodModelSessionSet            = "model/session/set"
	MethodModelSessionClear          = "model/session/clear"
	MethodRemoteWorkspaceList        = "remote_workspace/list"
	MethodRemoteWorkspaceOpen        = "remote_workspace/open"
	MethodRemoteWorkspaceForget      = "remote_workspace/forget"
	MethodRemoteWorkspaceClearCache  = "remote_workspace/clear_cache"
	MethodRemoteRepoCurrent          = "remote_repo/current"
	MethodGitStatus                  = "git/status"
	MethodGitDiff                    = "git/diff"
	MethodGitBranches                = "git/branches"
	MethodGitLog                     = "git/log"
	MethodGitShow                    = "git/show"
	MethodInsightPredictNextUser     = "insight/predict_next_user_message"
	MethodInsightPlanSnapshot        = "insight/plan_snapshot"
	MethodInsightMemorySnapshot      = "insight/memory_snapshot"
	MethodMemorySnapshot             = "memory/snapshot"
	MethodMemorySave                 = "memory/save"
	MethodMemoryRebuildIndex         = "memory/rebuild_index"
	MethodMemoryRecordAdd            = "memory/record/add"
	MethodMemoryRecordList           = "memory/record/list"
	MethodMemoryRecordSearch         = "memory/record/search"
	MethodMemoryRecordDelete         = "memory/record/delete"
	MethodRoleList                   = "role/list"
	MethodRoleResolve                = "role/resolve"
	MethodAgentSpawn                 = "agent/spawn"
	MethodAgentInput                 = "agent/input"
	MethodAgentWait                  = "agent/wait"
	MethodAgentRun                   = "agent/run"
	MethodAgentToolExecute           = "agent/tool/execute"
	MethodAgentList                  = "agent/list"
	MethodAgentClose                 = "agent/close"
	MethodEventSubscribe             = "event/subscribe"
	MethodEventUnsubscribe           = "event/unsubscribe"
	MethodTurnStart                  = "turn/start"
	MethodTurnInterrupt              = "turn/interrupt"
	MethodToolCatalog                = "tool/catalog"
	MethodToolExecute                = "tool/execute"
	MethodToolTraces                 = "tool/traces"
	MethodToolStats                  = "tool/stats"
	MethodApprovalRespond            = "approval/respond"
	MethodInquiryRespond             = "inquiry/respond"
	MethodStateSnapshot              = "state/snapshot"
	MethodSandboxPolicy              = "sandbox/policy"
	MethodSandboxSetPolicy           = "sandbox/set_policy"
	MethodSandboxBackend             = "sandbox/backend_status"
	MethodConfigReload               = "config/reload"
	MethodAgentControl               = "agent/control"
	MethodOrchestratorStart          = "orchestrator/start"
	MethodOrchestratorCancel         = "orchestrator/cancel"
	MethodDiagnosticsStartup         = "diagnostics/startup"
)

const (
	NotificationEvent        = "event"
	NotificationInitialized  = "initialized"
	NotificationStateChanged = "state/changed"
)

func AllCoreMethods() []string {
	return []string{
		MethodInitialize,
		MethodShutdown,
		MethodWorkspaceList,
		MethodWorkspaceDefault,
		MethodWorkspaceLast,
		MethodWorkspaceResolve,
		MethodWorkspaceRemember,
		MethodWorkspaceForget,
		MethodWorkspaceAdd,
		MethodWorkspaceRemove,
		MethodWorkspaceUse,
		MethodWorkspaceSetForeground,
		MethodWorkspaceTrust,
		MethodWorkspaceWorktreeList,
		MethodWorkspaceWorktreeCreate,
		MethodWorkspaceWorktreeRemove,
		MethodSessionCreate,
		MethodSessionResume,
		MethodSessionList,
		MethodSessionCurrent,
		MethodSessionSetCurrent,
		MethodSessionDelete,
		MethodSessionRename,
		MethodSessionSetMeta,
		MethodSessionMessagesLoad,
		MethodSessionMessagesSave,
		MethodMCPList,
		MethodMCPUpsert,
		MethodMCPImportJSON,
		MethodMCPDelete,
		MethodMCPSetEnabled,
		MethodLSPList,
		MethodLSPDetect,
		MethodLSPStart,
		MethodLSPDiagnostics,
		MethodLSPDiagnosticsSummary,
		MethodConfigRulesGet,
		MethodConfigRulesSnapshot,
		MethodConfigRulesSave,
		MethodConfigRulesReset,
		MethodConfigSettingsGet,
		MethodConfigSettingsSave,
		MethodPermissionSnapshot,
		MethodPermissionPendingReview,
		MethodPermissionClearReview,
		MethodPermissionAccessModeSet,
		MethodPermissionApprovalModeSet,
		MethodExtensionsSkillsList,
		MethodExtensionsSkillsReload,
		MethodExtensionsSkillSetEnabled,
		MethodExtensionsSkillInvoke,
		MethodExtensionsPluginsList,
		MethodExtensionsPluginSetEnabled,
		MethodBrowserStatus,
		MethodContextPreview,
		MethodContextStats,
		MethodContextWindow,
		MethodContextPin,
		MethodContextCompact,
		MethodContextClear,
		MethodContextExport,
		MethodUsageSummary,
		MethodUsageCostSummary,
		MethodUsageCostItems,
		MethodVersionsList,
		MethodVersionsRollback,
		MethodVersionsDelete,
		MethodVersionsDeleteFile,
		MethodVersionsClear,
		MethodTaskList,
		MethodTaskTodos,
		MethodTaskTail,
		MethodTaskKill,
		MethodTaskCleanup,
		MethodRuntimeModesGet,
		MethodRuntimeExecutionModeSet,
		MethodRuntimeSandboxModeSet,
		MethodRuntimeReasoningLevelSet,
		MethodModelList,
		MethodModelCatalog,
		MethodModelUpsert,
		MethodModelSave,
		MethodModelDelete,
		MethodModelActivate,
		MethodModelSyncEnv,
		MethodModelReload,
		MethodModelContext,
		MethodModelWorkspaceSet,
		MethodModelWorkspaceClear,
		MethodModelSessionSet,
		MethodModelSessionClear,
		MethodRemoteWorkspaceList,
		MethodRemoteWorkspaceOpen,
		MethodRemoteWorkspaceForget,
		MethodRemoteWorkspaceClearCache,
		MethodRemoteRepoCurrent,
		MethodGitStatus,
		MethodGitDiff,
		MethodGitBranches,
		MethodGitLog,
		MethodGitShow,
		MethodInsightPredictNextUser,
		MethodInsightPlanSnapshot,
		MethodInsightMemorySnapshot,
		MethodMemorySnapshot,
		MethodMemorySave,
		MethodMemoryRebuildIndex,
		MethodMemoryRecordAdd,
		MethodMemoryRecordList,
		MethodMemoryRecordSearch,
		MethodMemoryRecordDelete,
		MethodRoleList,
		MethodRoleResolve,
		MethodAgentSpawn,
		MethodAgentInput,
		MethodAgentWait,
		MethodAgentRun,
		MethodAgentToolExecute,
		MethodAgentList,
		MethodAgentClose,
		MethodEventSubscribe,
		MethodEventUnsubscribe,
		MethodTurnStart,
		MethodTurnInterrupt,
		MethodToolCatalog,
		MethodToolExecute,
		MethodToolTraces,
		MethodToolStats,
		MethodApprovalRespond,
		MethodInquiryRespond,
		MethodStateSnapshot,
		MethodSandboxPolicy,
		MethodSandboxSetPolicy,
		MethodSandboxBackend,
		MethodConfigReload,
		MethodAgentControl,
		MethodOrchestratorStart,
		MethodOrchestratorCancel,
		MethodDiagnosticsStartup,
	}
}
