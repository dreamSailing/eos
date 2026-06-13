package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/protocol"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

type Notifier interface {
	Notify(context.Context, protocoljsonrpc.Notification) error
}

type NotifierFunc func(context.Context, protocoljsonrpc.Notification) error

func (f NotifierFunc) Notify(ctx context.Context, notification protocoljsonrpc.Notification) error {
	return f(ctx, notification)
}

type Options struct {
	ServerName      string         `json:"server_name,omitempty"`
	ProtocolVersion string         `json:"protocol_version,omitempty"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	Notifier        Notifier       `json:"-"`
}

type InitializeResult struct {
	ServerName      string         `json:"server_name"`
	ProtocolVersion string         `json:"protocol_version"`
	Methods         []string       `json:"methods"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
}

func Register(router *protocoljsonrpc.Router, engine coreapi.Engine, opts Options) error {
	if router == nil {
		return errors.New("jsonrpc router is nil")
	}
	if engine == nil {
		return errors.New("coreapi engine is nil")
	}
	opts = normalizeOptions(opts)
	eventSubscriptions := newEventSubscriptions()

	if err := router.Register(protocoljsonrpc.MethodInitialize, func(context.Context, protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		return InitializeResult{
			ServerName:      opts.ServerName,
			ProtocolVersion: opts.ProtocolVersion,
			Methods: []string{
				protocoljsonrpc.MethodInitialize,
				protocoljsonrpc.MethodStateSnapshot,
				protocoljsonrpc.MethodWorkspaceList,
				protocoljsonrpc.MethodWorkspaceDefault,
				protocoljsonrpc.MethodWorkspaceLast,
				protocoljsonrpc.MethodWorkspaceResolve,
				protocoljsonrpc.MethodWorkspaceRemember,
				protocoljsonrpc.MethodWorkspaceForget,
				protocoljsonrpc.MethodWorkspaceAdd,
				protocoljsonrpc.MethodWorkspaceRemove,
				protocoljsonrpc.MethodWorkspaceUse,
				protocoljsonrpc.MethodWorkspaceSetForeground,
				protocoljsonrpc.MethodWorkspaceTrust,
				protocoljsonrpc.MethodWorkspaceWorktreeList,
				protocoljsonrpc.MethodWorkspaceWorktreeCreate,
				protocoljsonrpc.MethodWorkspaceWorktreeRemove,
				protocoljsonrpc.MethodSessionCreate,
				protocoljsonrpc.MethodSessionResume,
				protocoljsonrpc.MethodSessionList,
				protocoljsonrpc.MethodSessionCurrent,
				protocoljsonrpc.MethodSessionSetCurrent,
				protocoljsonrpc.MethodSessionDelete,
				protocoljsonrpc.MethodSessionRename,
				protocoljsonrpc.MethodSessionMessagesLoad,
				protocoljsonrpc.MethodSessionMessagesSave,
				protocoljsonrpc.MethodMCPList,
				protocoljsonrpc.MethodMCPUpsert,
				protocoljsonrpc.MethodMCPImportJSON,
				protocoljsonrpc.MethodMCPDelete,
				protocoljsonrpc.MethodMCPSetEnabled,
				protocoljsonrpc.MethodLSPList,
				protocoljsonrpc.MethodLSPDetect,
				protocoljsonrpc.MethodLSPStart,
				protocoljsonrpc.MethodLSPDiagnostics,
				protocoljsonrpc.MethodConfigRulesGet,
				protocoljsonrpc.MethodConfigRulesSnapshot,
				protocoljsonrpc.MethodConfigRulesSave,
				protocoljsonrpc.MethodConfigRulesReset,
				protocoljsonrpc.MethodConfigSettingsGet,
				protocoljsonrpc.MethodConfigSettingsSave,
				protocoljsonrpc.MethodPermissionSnapshot,
				protocoljsonrpc.MethodPermissionPendingReview,
				protocoljsonrpc.MethodPermissionClearReview,
				protocoljsonrpc.MethodPermissionAccessModeSet,
				protocoljsonrpc.MethodPermissionApprovalModeSet,
				protocoljsonrpc.MethodExtensionsSkillsList,
				protocoljsonrpc.MethodExtensionsSkillsReload,
				protocoljsonrpc.MethodExtensionsSkillSetEnabled,
				protocoljsonrpc.MethodExtensionsSkillInvoke,
				protocoljsonrpc.MethodExtensionsPluginsList,
				protocoljsonrpc.MethodExtensionsPluginSetEnabled,
				protocoljsonrpc.MethodBrowserStatus,
				protocoljsonrpc.MethodContextPreview,
				protocoljsonrpc.MethodContextStats,
				protocoljsonrpc.MethodContextWindow,
				protocoljsonrpc.MethodContextPin,
				protocoljsonrpc.MethodContextCompact,
				protocoljsonrpc.MethodContextClear,
				protocoljsonrpc.MethodContextExport,
				protocoljsonrpc.MethodUsageSummary,
				protocoljsonrpc.MethodUsageCostSummary,
				protocoljsonrpc.MethodUsageCostItems,
				protocoljsonrpc.MethodVersionsList,
				protocoljsonrpc.MethodVersionsRollback,
				protocoljsonrpc.MethodVersionsDelete,
				protocoljsonrpc.MethodVersionsDeleteFile,
				protocoljsonrpc.MethodVersionsClear,
				protocoljsonrpc.MethodTaskList,
				protocoljsonrpc.MethodTaskTodos,
				protocoljsonrpc.MethodTaskTail,
				protocoljsonrpc.MethodTaskKill,
				protocoljsonrpc.MethodTaskCleanup,
				protocoljsonrpc.MethodRuntimeModesGet,
				protocoljsonrpc.MethodRuntimeExecutionModeSet,
				protocoljsonrpc.MethodRuntimeSandboxModeSet,
				protocoljsonrpc.MethodRuntimeReasoningLevelSet,
				protocoljsonrpc.MethodModelList,
				protocoljsonrpc.MethodModelCatalog,
				protocoljsonrpc.MethodModelUpsert,
				protocoljsonrpc.MethodModelSave,
				protocoljsonrpc.MethodModelDelete,
				protocoljsonrpc.MethodModelActivate,
				protocoljsonrpc.MethodModelSyncEnv,
				protocoljsonrpc.MethodModelContext,
				protocoljsonrpc.MethodModelWorkspaceSet,
				protocoljsonrpc.MethodModelWorkspaceClear,
				protocoljsonrpc.MethodModelSessionSet,
				protocoljsonrpc.MethodModelSessionClear,
				protocoljsonrpc.MethodRemoteWorkspaceList,
				protocoljsonrpc.MethodRemoteWorkspaceOpen,
				protocoljsonrpc.MethodRemoteWorkspaceForget,
				protocoljsonrpc.MethodRemoteWorkspaceClearCache,
				protocoljsonrpc.MethodRemoteRepoCurrent,
				protocoljsonrpc.MethodGitStatus,
				protocoljsonrpc.MethodGitDiff,
				protocoljsonrpc.MethodGitBranches,
				protocoljsonrpc.MethodGitLog,
				protocoljsonrpc.MethodGitShow,
				protocoljsonrpc.MethodInsightPredictNextUser,
				protocoljsonrpc.MethodInsightPlanSnapshot,
				protocoljsonrpc.MethodInsightMemorySnapshot,
				protocoljsonrpc.MethodMemorySnapshot,
				protocoljsonrpc.MethodMemorySave,
				protocoljsonrpc.MethodMemoryRebuildIndex,
				protocoljsonrpc.MethodMemoryRecordAdd,
				protocoljsonrpc.MethodMemoryRecordList,
				protocoljsonrpc.MethodMemoryRecordSearch,
				protocoljsonrpc.MethodMemoryRecordDelete,
				protocoljsonrpc.MethodRoleList,
				protocoljsonrpc.MethodRoleResolve,
				protocoljsonrpc.MethodAgentSpawn,
				protocoljsonrpc.MethodAgentInput,
				protocoljsonrpc.MethodAgentWait,
				protocoljsonrpc.MethodAgentRun,
				protocoljsonrpc.MethodAgentToolExecute,
				protocoljsonrpc.MethodAgentList,
				protocoljsonrpc.MethodAgentClose,
				protocoljsonrpc.MethodEventSubscribe,
				protocoljsonrpc.MethodEventUnsubscribe,
				protocoljsonrpc.MethodApprovalRespond,
				protocoljsonrpc.MethodTurnStart,
				protocoljsonrpc.MethodTurnInterrupt,
				protocoljsonrpc.MethodToolCatalog,
				protocoljsonrpc.MethodToolExecute,
				protocoljsonrpc.MethodToolTraces,
				protocoljsonrpc.MethodToolStats,
				protocoljsonrpc.MethodInquiryRespond,
				protocoljsonrpc.MethodSandboxPolicy,
				protocoljsonrpc.MethodSandboxSetPolicy,
				protocoljsonrpc.MethodSandboxBackend,
				protocoljsonrpc.MethodDiagnosticsStartup,
			},
			Capabilities: cloneMap(opts.Capabilities),
		}, nil
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodStateSnapshot, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if state := engine.State(); state != nil {
			snapshot, err := state.Snapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodWorkspaceList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if workspaces := engine.Workspaces(); workspaces != nil {
			items, err := workspaces.List(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodWorkspaceDefault, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if workspaces := engine.Workspaces(); workspaces != nil {
			path, err := workspaces.Default(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"path": path}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodWorkspaceLast, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if workspaces := engine.Workspaces(); workspaces != nil {
			path, err := workspaces.Last(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"path": path}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodWorkspaceResolve, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ResolveForegroundWorkspaceRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if workspaces := engine.Workspaces(); workspaces != nil {
			path, err := workspaces.ResolveForeground(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"path": path}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodWorkspaceRemember, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.RememberWorkspaceRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if workspaces := engine.Workspaces(); workspaces != nil {
			if err := workspaces.Remember(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := registerWorkspacePathMethod(router, protocoljsonrpc.MethodWorkspaceForget, func(ctx context.Context, req coreapi.WorkspacePathRequest) error {
		if workspaces := engine.Workspaces(); workspaces != nil {
			return workspaces.Forget(ctx, req)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}
	if err := registerWorkspacePathMethod(router, protocoljsonrpc.MethodWorkspaceAdd, func(ctx context.Context, req coreapi.WorkspacePathRequest) error {
		if workspaces := engine.Workspaces(); workspaces != nil {
			return workspaces.Add(ctx, req)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}
	if err := registerWorkspacePathMethod(router, protocoljsonrpc.MethodWorkspaceRemove, func(ctx context.Context, req coreapi.WorkspacePathRequest) error {
		if workspaces := engine.Workspaces(); workspaces != nil {
			return workspaces.Remove(ctx, req)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}
	if err := registerWorkspacePathMethod(router, protocoljsonrpc.MethodWorkspaceUse, func(ctx context.Context, req coreapi.WorkspacePathRequest) error {
		if workspaces := engine.Workspaces(); workspaces != nil {
			return workspaces.Use(ctx, req)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}
	if err := registerWorkspacePathMethod(router, protocoljsonrpc.MethodWorkspaceSetForeground, func(ctx context.Context, req coreapi.WorkspacePathRequest) error {
		if workspaces := engine.Workspaces(); workspaces != nil {
			return workspaces.SetForeground(ctx, req)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}
	if err := registerWorkspacePathMethod(router, protocoljsonrpc.MethodWorkspaceTrust, func(ctx context.Context, req coreapi.WorkspacePathRequest) error {
		if workspaces := engine.Workspaces(); workspaces != nil {
			return workspaces.Trust(ctx, req)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodWorkspaceWorktreeList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if workspaces := engine.Workspaces(); workspaces != nil {
			items, err := workspaces.ListWorktrees(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodWorkspaceWorktreeCreate, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.CreateWorktreeRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if workspaces := engine.Workspaces(); workspaces != nil {
			item, err := workspaces.CreateWorktree(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return item, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodWorkspaceWorktreeRemove, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.RemoveWorktreeRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if workspaces := engine.Workspaces(); workspaces != nil {
			if err := workspaces.RemoveWorktree(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionCreate, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.CreateSessionRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			item, err := sessions.Create(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return item, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionResume, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ResumeSessionRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			item, err := sessions.Resume(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return item, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionCurrent, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.CurrentSessionRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			item, err := sessions.Current(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return item, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionSetCurrent, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetCurrentSessionRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			if err := sessions.SetCurrent(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionDelete, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.DeleteSessionRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			if err := sessions.Delete(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionRename, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.RenameSessionRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			item, err := sessions.Rename(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return item, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionMessagesLoad, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.LoadSessionMessagesRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			items, err := sessions.LoadMessages(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionMessagesSave, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SaveSessionMessagesRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			item, err := sessions.SaveMessages(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return item, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMCPList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if mcp := engine.MCP(); mcp != nil {
			items, err := mcp.List(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMCPUpsert, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.UpsertMCPRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if mcp := engine.MCP(); mcp != nil {
			if err := mcp.Upsert(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMCPImportJSON, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ImportMCPJSONRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if mcp := engine.MCP(); mcp != nil {
			if err := mcp.ImportJSON(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMCPDelete, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.MCPNameRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if mcp := engine.MCP(); mcp != nil {
			if err := mcp.Delete(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMCPSetEnabled, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetMCPEnabledRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if mcp := engine.MCP(); mcp != nil {
			if err := mcp.SetEnabled(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodLSPList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if lsp := engine.LSP(); lsp != nil {
			items, err := lsp.List(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodLSPDetect, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.LSPLanguageRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if lsp := engine.LSP(); lsp != nil {
			message, err := lsp.Detect(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"message": message}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodLSPStart, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.LSPLanguageRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if lsp := engine.LSP(); lsp != nil {
			message, err := lsp.Start(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"message": message}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodLSPDiagnostics, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if lsp := engine.LSP(); lsp != nil {
			items, err := lsp.Diagnostics(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodLSPDiagnosticsSummary, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if lsp := engine.LSP(); lsp != nil {
			summary, err := lsp.DiagnosticsSummary(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return summary, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodConfigRulesGet, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if config := engine.Config(); config != nil {
			content, err := config.GetRules(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"content": content}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodConfigRulesSnapshot, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if config := engine.Config(); config != nil {
			snapshot, err := config.RulesSnapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodConfigRulesSave, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SaveRulesRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if config := engine.Config(); config != nil {
			if err := config.SaveRules(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodConfigRulesReset, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if config := engine.Config(); config != nil {
			if err := config.ResetRules(ctx); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodConfigSettingsGet, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if config := engine.Config(); config != nil {
			settings, err := config.GetSettings(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return settings, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodConfigSettingsSave, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.Settings
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if config := engine.Config(); config != nil {
			if err := config.SaveSettings(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodPermissionSnapshot, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if permissions := engine.Permissions(); permissions != nil {
			snapshot, err := permissions.Snapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodPermissionPendingReview, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if permissions := engine.Permissions(); permissions != nil {
			review, err := permissions.PendingReview(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return review, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodPermissionClearReview, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if permissions := engine.Permissions(); permissions != nil {
			if err := permissions.ClearPendingReview(ctx); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := registerPermissionModeSetter(router, protocoljsonrpc.MethodPermissionAccessModeSet, func(ctx context.Context, permissions coreapi.PermissionService, params coreapi.SetModeRequest) error {
		setter, ok := permissions.(interface {
			SetAccessMode(context.Context, coreapi.SetModeRequest) error
		})
		if !ok {
			return coreapi.ErrUnsupported
		}
		return setter.SetAccessMode(ctx, params)
	}, engine); err != nil {
		return err
	}

	if err := registerPermissionModeSetter(router, protocoljsonrpc.MethodPermissionApprovalModeSet, func(ctx context.Context, permissions coreapi.PermissionService, params coreapi.SetModeRequest) error {
		setter, ok := permissions.(interface {
			SetApprovalMode(context.Context, coreapi.SetModeRequest) error
		})
		if !ok {
			return coreapi.ErrUnsupported
		}
		return setter.SetApprovalMode(ctx, params)
	}, engine); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodExtensionsSkillsList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if extensions := engine.Extensions(); extensions != nil {
			items, err := extensions.ListSkills(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodExtensionsSkillsReload, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if extensions := engine.Extensions(); extensions != nil {
			if err := extensions.ReloadSkills(ctx); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodExtensionsSkillSetEnabled, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetExtensionEnabledRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if extensions := engine.Extensions(); extensions != nil {
			if err := extensions.SetSkillEnabled(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodExtensionsSkillInvoke, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.InvokeSkillRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if extensions := engine.Extensions(); extensions != nil {
			result, err := extensions.InvokeSkill(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return result, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodExtensionsPluginsList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if extensions := engine.Extensions(); extensions != nil {
			items, err := extensions.ListPlugins(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodExtensionsPluginSetEnabled, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetExtensionEnabledRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if extensions := engine.Extensions(); extensions != nil {
			if err := extensions.SetPluginEnabled(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodBrowserStatus, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if extensions := engine.Extensions(); extensions != nil {
			status, err := extensions.BrowserStatus(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return status, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodContextPreview, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if contextSvc := engine.Context(); contextSvc != nil {
			lines, err := contextSvc.Preview(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return lines, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodContextStats, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if contextSvc := engine.Context(); contextSvc != nil {
			stats, err := contextSvc.Stats(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return stats, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodContextWindow, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if contextSvc := engine.Context(); contextSvc != nil {
			tokens, err := contextSvc.WindowTokens(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"tokens": tokens}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodContextPin, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.PinDocumentRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if contextSvc := engine.Context(); contextSvc != nil {
			if err := contextSvc.PinDocument(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodContextCompact, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if contextSvc := engine.Context(); contextSvc != nil {
			message, err := contextSvc.Compact(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"message": message}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodContextClear, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if contextSvc := engine.Context(); contextSvc != nil {
			if err := contextSvc.Clear(ctx); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodContextExport, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ExportContextRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if contextSvc := engine.Context(); contextSvc != nil {
			if err := contextSvc.Export(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodUsageSummary, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if usage := engine.Usage(); usage != nil {
			summary, err := usage.Summary(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return summary, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodUsageCostSummary, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if usage := engine.Usage(); usage != nil {
			summary, err := usage.CostSummary(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"summary": summary}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodUsageCostItems, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if usage := engine.Usage(); usage != nil {
			items, err := usage.CostItems(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodVersionsList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if versions := engine.Versions(); versions != nil {
			items, err := versions.List(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodVersionsRollback, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.VersionIDRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if versions := engine.Versions(); versions != nil {
			if err := versions.Rollback(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodVersionsDelete, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.VersionIDRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if versions := engine.Versions(); versions != nil {
			if err := versions.Delete(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodVersionsDeleteFile, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.VersionFileRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if versions := engine.Versions(); versions != nil {
			count, err := versions.DeleteFile(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"count": count}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodVersionsClear, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if versions := engine.Versions(); versions != nil {
			count, err := versions.Clear(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"count": count}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodTaskList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if tasks := engine.Tasks(); tasks != nil {
			items, err := tasks.List(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodTaskTodos, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if tasks := engine.Tasks(); tasks != nil {
			items, err := tasks.Todos(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodTaskTail, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.TaskIDRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if tasks := engine.Tasks(); tasks != nil {
			lines, err := tasks.Tail(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return lines, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodTaskKill, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.TaskIDRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if tasks := engine.Tasks(); tasks != nil {
			if err := tasks.Kill(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodTaskCleanup, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if tasks := engine.Tasks(); tasks != nil {
			count, err := tasks.Cleanup(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"count": count}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodRuntimeModesGet, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if modes := engine.Modes(); modes != nil {
			snapshot, err := modes.Snapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := registerModeSetter(router, protocoljsonrpc.MethodRuntimeExecutionModeSet, func(ctx context.Context, params coreapi.SetModeRequest) error {
		if modes := engine.Modes(); modes != nil {
			return modes.SetExecutionMode(ctx, params)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}

	if err := registerModeSetter(router, protocoljsonrpc.MethodRuntimeSandboxModeSet, func(ctx context.Context, params coreapi.SetModeRequest) error {
		if modes := engine.Modes(); modes != nil {
			return modes.SetSandboxMode(ctx, params)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}

	if err := registerModeSetter(router, protocoljsonrpc.MethodRuntimeReasoningLevelSet, func(ctx context.Context, params coreapi.SetModeRequest) error {
		if modes := engine.Modes(); modes != nil {
			return modes.SetReasoningLevel(ctx, params)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if models := engine.Models(); models != nil {
			items, err := models.List(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelCatalog, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if models := engine.Models(); models != nil {
			catalog, err := models.Catalog(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return catalog, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelUpsert, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.UpsertModelRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if models := engine.Models(); models != nil {
			if err := models.Upsert(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelSave, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ModelSaveRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if models := engine.Models(); models != nil {
			if err := models.Save(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := registerModelNameMethod(router, protocoljsonrpc.MethodModelDelete, func(ctx context.Context, params coreapi.ModelNameRequest) error {
		if models := engine.Models(); models != nil {
			return models.Delete(ctx, params)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}

	if err := registerModelNameMethod(router, protocoljsonrpc.MethodModelActivate, func(ctx context.Context, params coreapi.ModelNameRequest) error {
		if models := engine.Models(); models != nil {
			return models.Activate(ctx, params)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelSyncEnv, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if models := engine.Models(); models != nil {
			if err := models.SyncEnv(ctx); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelContext, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ModelContextRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if models := engine.Models(); models != nil {
			snapshot, err := models.Context(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelWorkspaceSet, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetWorkspaceModelRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if models := engine.Models(); models != nil {
			if err := models.SetWorkspace(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelWorkspaceClear, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ClearWorkspaceModelRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if models := engine.Models(); models != nil {
			if err := models.ClearWorkspace(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelSessionSet, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetSessionModelRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if models := engine.Models(); models != nil {
			if err := models.SetSession(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodModelSessionClear, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ClearSessionModelRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if models := engine.Models(); models != nil {
			if err := models.ClearSession(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodRemoteWorkspaceList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if remote := engine.RemoteWorkspaces(); remote != nil {
			items, err := remote.List(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodRemoteWorkspaceOpen, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.RemoteWorkspaceRef
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if remote := engine.RemoteWorkspaces(); remote != nil {
			item, err := remote.Open(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return item, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := registerRemoteWorkspaceRefMethod(router, protocoljsonrpc.MethodRemoteWorkspaceForget, func(ctx context.Context, params coreapi.RemoteWorkspaceRef) error {
		if remote := engine.RemoteWorkspaces(); remote != nil {
			return remote.Forget(ctx, params)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}

	if err := registerRemoteWorkspaceRefMethod(router, protocoljsonrpc.MethodRemoteWorkspaceClearCache, func(ctx context.Context, params coreapi.RemoteWorkspaceRef) error {
		if remote := engine.RemoteWorkspaces(); remote != nil {
			return remote.ClearCache(ctx, params)
		}
		return coreapi.ErrUnsupported
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodRemoteRepoCurrent, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if remote := engine.RemoteWorkspaces(); remote != nil {
			state, ok, err := remote.CurrentRepo(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": ok, "state": state}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodGitStatus, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.GitStatusRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if git := engine.Git(); git != nil {
			changes, err := git.Status(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return changes, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodGitDiff, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.GitDiffRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if git := engine.Git(); git != nil {
			out, err := git.Diff(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return out, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodGitBranches, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.GitBranchesRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if git := engine.Git(); git != nil {
			out, err := git.Branches(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return out, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodGitLog, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.GitLogRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if git := engine.Git(); git != nil {
			out, err := git.Log(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return out, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodGitShow, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.GitShowRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if git := engine.Git(); git != nil {
			out, err := git.Show(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return out, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodInsightPredictNextUser, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.PredictNextUserMessageRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if insights := engine.Insights(); insights != nil {
			message, err := insights.PredictNextUserMessage(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"message": message}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodInsightPlanSnapshot, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if insights := engine.Insights(); insights != nil {
			snapshot, err := insights.PlanSnapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodInsightMemorySnapshot, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if insights := engine.Insights(); insights != nil {
			snapshot, err := insights.MemorySnapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		if memory := engine.Memory(); memory != nil {
			snapshot, err := memory.Snapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMemorySnapshot, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if memory := engine.Memory(); memory != nil {
			snapshot, err := memory.Snapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		if insights := engine.Insights(); insights != nil {
			snapshot, err := insights.MemorySnapshot(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return snapshot, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMemorySave, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SaveMemoryRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if memory := engine.Memory(); memory != nil {
			if err := memory.Save(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMemoryRebuildIndex, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if memory := engine.Memory(); memory != nil {
			if err := memory.RebuildIndex(ctx); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMemoryRecordAdd, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.AddMemoryRecordRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if memory := engine.Memory(); memory != nil {
			rec, err := memory.RecordAdd(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return rec, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMemoryRecordList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ListMemoryRecordsRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if memory := engine.Memory(); memory != nil {
			items, err := memory.RecordList(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMemoryRecordSearch, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SearchMemoryRecordsRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if memory := engine.Memory(); memory != nil {
			items, err := memory.RecordSearch(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodMemoryRecordDelete, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.DeleteMemoryRecordRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if memory := engine.Memory(); memory != nil {
			if err := memory.RecordDelete(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodRoleList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if roles := engine.Roles(); roles != nil {
			items, err := roles.List(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodRoleResolve, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.RoleRef
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if roles := engine.Roles(); roles != nil {
			role, err := roles.Resolve(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return role, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodAgentSpawn, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SpawnAgentRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if agents := engine.Agents(); agents != nil {
			agent, err := agents.Spawn(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			notifyAgentEvent(ctx, opts.Notifier, protocol.EventTypeAgentStarted, agent, "spawn", "Agent started")
			return agent, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodAgentInput, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.AgentInput
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if agents := engine.Agents(); agents != nil {
			if err := agents.SendInput(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			notifyAgentEvent(ctx, opts.Notifier, protocol.EventTypeAgentProgress, coreapi.Agent{ID: strings.TrimSpace(params.AgentID)}, "input", "Agent input received")
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodAgentWait, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.AgentRef
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if agents := engine.Agents(); agents != nil {
			agent, err := agents.Wait(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return agent, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodAgentRun, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.RunAgentRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		params.AgentID = strings.TrimSpace(params.AgentID)
		params.SessionID = strings.TrimSpace(params.SessionID)
		if agents := engine.Agents(); agents != nil {
			stopForwarder := startEventForwarder(ctx, engine, opts.Notifier, coreapi.EventFilter{
				SessionID: params.SessionID,
				AgentID:   params.AgentID,
			})
			result, err := agents.Run(ctx, params)
			if err != nil {
				if stopForwarder != nil {
					stopForwarder()
				}
				return nil, errorFromErr(err)
			}
			return result, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodAgentToolExecute, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.AgentToolRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		params.AgentID = strings.TrimSpace(params.AgentID)
		params.SessionID = strings.TrimSpace(params.SessionID)
		params.TurnID = strings.TrimSpace(params.TurnID)
		params.Name = strings.TrimSpace(params.Name)
		if agents := engine.Agents(); agents != nil {
			stopForwarder := startEventForwarder(ctx, engine, opts.Notifier, coreapi.EventFilter{
				SessionID: params.SessionID,
				TurnID:    params.TurnID,
				AgentID:   params.AgentID,
			})
			result, err := agents.RunTool(ctx, params)
			if err != nil {
				if stopForwarder != nil {
					stopForwarder()
				}
				return nil, errorFromErr(err)
			}
			return result, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodAgentList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ListAgentsRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if agents := engine.Agents(); agents != nil {
			items, err := agents.List(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodAgentClose, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.AgentRef
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if agents := engine.Agents(); agents != nil {
			if err := agents.Close(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			notifyAgentEvent(ctx, opts.Notifier, protocol.EventTypeAgentCancelled, coreapi.Agent{ID: strings.TrimSpace(params.AgentID), Status: "cancelled"}, "close", "Agent cancelled")
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodEventSubscribe, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.EventSubscribeRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if opts.Notifier == nil {
			return nil, errorFromErr(errors.New("jsonrpc notifier is not configured"))
		}
		events := engine.Events()
		if events == nil {
			return nil, errorFromErr(coreapi.ErrUnsupported)
		}
		subCtx, cancel := context.WithCancel(ctx)
		filter := coreapi.EventFilter{
			SessionID: strings.TrimSpace(params.SessionID),
			TurnID:    strings.TrimSpace(params.TurnID),
			AgentID:   strings.TrimSpace(params.AgentID),
		}
		ch, err := events.Subscribe(subCtx, filter)
		if err != nil {
			cancel()
			return nil, errorFromErr(err)
		}
		id := eventSubscriptions.add(cancel)
		go eventSubscriptions.forward(subCtx, id, ch, opts.Notifier)
		return coreapi.EventSubscription{ID: id}, nil
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodEventUnsubscribe, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.EventUnsubscribeRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		id := strings.TrimSpace(params.ID)
		if id == "" {
			return nil, invalidParamsError("event subscription id is required")
		}
		cancelled := eventSubscriptions.cancel(id)
		return map[string]any{"ok": cancelled}, nil
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodTurnStart, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.StartTurnRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if strings.TrimSpace(params.TurnID) == "" {
			params.TurnID = newTurnID()
		}
		if turns := engine.Turns(); turns != nil {
			stopForwarder := startEventForwarder(ctx, engine, opts.Notifier, coreapi.EventFilter{
				SessionID: strings.TrimSpace(params.SessionID),
				TurnID:    strings.TrimSpace(params.TurnID),
			})
			turn, err := turns.Start(ctx, params)
			if err != nil {
				if stopForwarder != nil {
					stopForwarder()
				}
				return nil, errorFromErr(err)
			}
			return turn, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodTurnInterrupt, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.TurnRef
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if turns := engine.Turns(); turns != nil {
			if err := turns.Interrupt(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodToolCatalog, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ListToolCatalogRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if catalog := engine.ToolCatalog(); catalog != nil {
			items, err := catalog.List(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodToolExecute, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ToolRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		params.Name = strings.TrimSpace(params.Name)
		params.SessionID = strings.TrimSpace(params.SessionID)
		params.TurnID = strings.TrimSpace(params.TurnID)
		params.RequestID = strings.TrimSpace(params.RequestID)
		params.AgentID = strings.TrimSpace(params.AgentID)
		if params.RequestID == "" {
			params.RequestID = params.TurnID
		}
		if params.RequestID == "" {
			params.RequestID = newToolID()
		}
		if params.TurnID == "" {
			params.TurnID = params.RequestID
		}
		if tools := engine.Tools(); tools != nil {
			stopForwarder := startEventForwarder(ctx, engine, opts.Notifier, coreapi.EventFilter{
				SessionID: params.SessionID,
				TurnID:    params.RequestID,
			})
			result, err := tools.Execute(ctx, params)
			if err != nil {
				if stopForwarder != nil {
					stopForwarder()
				}
				return nil, errorFromErr(err)
			}
			return result, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodApprovalRespond, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ApprovalResponse
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if approvals := engine.Approvals(); approvals != nil {
			if err := approvals.Respond(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodInquiryRespond, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.InquiryResponse
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if inquiries := engine.Inquiries(); inquiries != nil {
			if err := inquiries.Respond(ctx, params); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSessionList, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ListSessionsRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if sessions := engine.Sessions(); sessions != nil {
			items, err := sessions.List(ctx, params)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return items, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodToolTraces, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if telemetry := engine.ToolTelemetry(); telemetry != nil {
			traces, err := telemetry.Traces(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return traces, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodToolStats, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if telemetry := engine.ToolTelemetry(); telemetry != nil {
			stats, err := telemetry.Stats(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return stats, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSandboxPolicy, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SandboxPolicyRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if service := engine.Sandbox(); service != nil {
			policy, err := service.Policy(ctx, coreapi.SessionRef{SessionID: strings.TrimSpace(params.SessionID)})
			if err != nil {
				return nil, errorFromErr(err)
			}
			return policy, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSandboxSetPolicy, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetSandboxPolicyRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if service := engine.Sandbox(); service != nil {
			if err := service.SetPolicy(ctx, coreapi.SessionRef{SessionID: strings.TrimSpace(params.SessionID)}, params.Policy); err != nil {
				return nil, errorFromErr(err)
			}
			return map[string]any{"ok": true}, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodSandboxBackend, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if service := engine.Sandbox(); service != nil {
			return service.BackendStatus(ctx), nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	if err := router.Register(protocoljsonrpc.MethodDiagnosticsStartup, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		if service := engine.Diagnostics(); service != nil {
			result, err := service.Startup(ctx)
			if err != nil {
				return nil, errorFromErr(err)
			}
			return result, nil
		}
		return nil, errorFromErr(coreapi.ErrUnsupported)
	}); err != nil {
		return err
	}

	return nil
}

func newTurnID() string {
	return "turn_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

func newToolID() string {
	return "tool_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

func normalizeOptions(opts Options) Options {
	opts.ServerName = strings.TrimSpace(opts.ServerName)
	if opts.ServerName == "" {
		opts.ServerName = "eos-core"
	}
	opts.ProtocolVersion = strings.TrimSpace(opts.ProtocolVersion)
	if opts.ProtocolVersion == "" {
		opts.ProtocolVersion = "v1"
	}
	opts.Capabilities = cloneMap(opts.Capabilities)
	if opts.Capabilities == nil {
		opts.Capabilities = map[string]any{
			"state_snapshot":     true,
			"workspace":          true,
			"workspace_worktree": true,
			"mcp":                true,
			"lsp":                true,
			"config":             true,
			"permissions":        true,
			"permission_modes":   true,
			"extensions":         true,
			"context":            true,
			"usage":              true,
			"versions":           true,
			"tasks":              true,
			"runtime_modes":      true,
			"models":             true,
			"remote_workspaces":  true,
			"insights":           true,
			"roles":              true,
			"agents":             true,
			"events":             true,
			"session_create":     true,
			"session_resume":     true,
			"session_list":       true,
			"session_current":    true,
			"session_messages":   true,
			"session_mutation":   true,
			"approval_respond":   true,
			"inquiry_respond":    true,
			"tool_execute":       true,
			"sandbox_policy":     true,
			"sandbox_backend":    true,
			"turn_boundary":      true,
			"turn_start":         true,
			"turn_interrupt":     true,
			"readonly":           false,
		}
	}
	return opts
}

func registerWorkspacePathMethod(router *protocoljsonrpc.Router, method string, fn func(context.Context, coreapi.WorkspacePathRequest) error) error {
	return router.Register(method, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.WorkspacePathRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if err := fn(ctx, params); err != nil {
			return nil, errorFromErr(err)
		}
		return map[string]any{"ok": true}, nil
	})
}

func registerModeSetter(router *protocoljsonrpc.Router, method string, fn func(context.Context, coreapi.SetModeRequest) error) error {
	return router.Register(method, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetModeRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if err := fn(ctx, params); err != nil {
			return nil, errorFromErr(err)
		}
		return map[string]any{"ok": true}, nil
	})
}

func registerPermissionModeSetter(router *protocoljsonrpc.Router, method string, fn func(context.Context, coreapi.PermissionService, coreapi.SetModeRequest) error, engine coreapi.Engine) error {
	return router.Register(method, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.SetModeRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		permissions := engine.Permissions()
		if permissions == nil {
			return nil, errorFromErr(coreapi.ErrUnsupported)
		}
		if err := fn(ctx, permissions, params); err != nil {
			return nil, errorFromErr(err)
		}
		return map[string]any{"ok": true}, nil
	})
}

func registerModelNameMethod(router *protocoljsonrpc.Router, method string, fn func(context.Context, coreapi.ModelNameRequest) error) error {
	return router.Register(method, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.ModelNameRequest
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if err := fn(ctx, params); err != nil {
			return nil, errorFromErr(err)
		}
		return map[string]any{"ok": true}, nil
	})
}

func registerRemoteWorkspaceRefMethod(router *protocoljsonrpc.Router, method string, fn func(context.Context, coreapi.RemoteWorkspaceRef) error) error {
	return router.Register(method, func(ctx context.Context, req protocoljsonrpc.Request) (any, *protocoljsonrpc.Error) {
		var params coreapi.RemoteWorkspaceRef
		if rpcErr := decodeParams(req.Params, &params); rpcErr != nil {
			return nil, rpcErr
		}
		if err := fn(ctx, params); err != nil {
			return nil, errorFromErr(err)
		}
		return map[string]any{"ok": true}, nil
	})
}

type eventSubscriptionSet struct {
	mu      sync.Mutex
	next    int64
	cancels map[string]context.CancelFunc
}

func newEventSubscriptions() *eventSubscriptionSet {
	return &eventSubscriptionSet{cancels: map[string]context.CancelFunc{}}
}

func (s *eventSubscriptionSet) add(cancel context.CancelFunc) string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	id := "eventsub_" + strconv.FormatInt(s.next, 10)
	s.cancels[id] = cancel
	return id
}

func (s *eventSubscriptionSet) cancel(id string) bool {
	if s == nil {
		return false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	s.mu.Lock()
	cancel := s.cancels[id]
	if cancel != nil {
		delete(s.cancels, id)
	}
	s.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (s *eventSubscriptionSet) forward(ctx context.Context, id string, ch <-chan protocol.Envelope, notifier Notifier) {
	defer s.cancel(id)
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-ch:
			if !ok {
				return
			}
			notification, err := eventNotification(envelope)
			if err != nil {
				slog.Debug("coreapi.jsonrpc.event_notification_failed", "subscription_id", id, "error", err)
				continue
			}
			if err := notifier.Notify(ctx, notification); err != nil {
				slog.Debug("coreapi.jsonrpc.event_notify_failed", "subscription_id", id, "error", err)
				return
			}
		}
	}
}

func startEventForwarder(ctx context.Context, engine coreapi.Engine, notifier Notifier, filter coreapi.EventFilter) context.CancelFunc {
	if engine == nil || notifier == nil {
		return nil
	}
	events := engine.Events()
	if events == nil {
		return nil
	}
	forwardCtx, cancel := context.WithCancel(ctx)
	ch, err := events.Subscribe(forwardCtx, filter)
	if err != nil {
		cancel()
		slog.Debug("coreapi.jsonrpc.event_subscribe_failed", "error", err)
		return nil
	}
	go func() {
		for {
			select {
			case <-forwardCtx.Done():
				return
			case envelope, ok := <-ch:
				if !ok {
					return
				}
				notification, err := eventNotification(envelope)
				if err != nil {
					slog.Debug("coreapi.jsonrpc.event_notification_failed", "error", err)
					continue
				}
				if err := notifier.Notify(forwardCtx, notification); err != nil {
					slog.Debug("coreapi.jsonrpc.event_notify_failed", "error", err)
					return
				}
				if isTerminalEvent(envelope) {
					return
				}
			}
		}
	}()
	return cancel
}

func isTerminalEvent(envelope protocol.Envelope) bool {
	switch envelope.EventType {
	case protocol.EventTypeRequestDone, protocol.EventTypeRequestFailed, protocol.EventTypeAgentDone, protocol.EventTypeAgentFailed, protocol.EventTypeAgentCancelled:
		return true
	default:
		return false
	}
}

func notifyAgentEvent(ctx context.Context, notifier Notifier, eventType protocol.EventType, agent coreapi.Agent, action string, message string) {
	if notifier == nil || strings.TrimSpace(agent.ID) == "" {
		return
	}
	notification, err := eventNotification(protocol.NewEvent(eventType, protocol.EventOptions{
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
	if err != nil {
		slog.Debug("coreapi.jsonrpc.agent_event_notification_build_failed", "error", err)
		return
	}
	if err := notifier.Notify(ctx, notification); err != nil {
		slog.Debug("coreapi.jsonrpc.agent_event_notify_failed", "error", err)
	}
}

func eventNotification(envelope protocol.Envelope) (protocoljsonrpc.Notification, error) {
	return protocoljsonrpc.NewNotification(protocoljsonrpc.NotificationEvent, envelope)
}

func decodeParams(raw json.RawMessage, out any) *protocoljsonrpc.Error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return invalidParamsError(err.Error())
	}
	return nil
}

func invalidParamsError(reason string) *protocoljsonrpc.Error {
	return &protocoljsonrpc.Error{
		Code:    protocoljsonrpc.CodeInvalidParams,
		Message: "invalid params",
		Data:    json.RawMessage([]byte(`{"reason":` + quoteJSONString(reason) + `}`)),
	}
}

func errorFromErr(err error) *protocoljsonrpc.Error {
	if err == nil {
		return nil
	}
	code := protocoljsonrpc.CodeInternalError
	message := err.Error()
	if errors.Is(err, coreapi.ErrUnsupported) {
		message = coreapi.ErrUnsupported.Error()
	}
	return &protocoljsonrpc.Error{Code: code, Message: message}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func quoteJSONString(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(data)
}
