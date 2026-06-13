//go:build legacy

// adapter 包是 TUI 与运行时之间的兼容层。
//
// 依赖边界（import boundary）:
//   - 首选路径: pkg/coreapi DTO + pkg/protocol/jsonrpc JSON-RPC 客户端
//   - 允许: pkg/core (sharedcore.Runtime + coreapi.Engine legacy fallback)
//   - 允许: internal/bridge — 仅用于 legacy event 归一化 (runtime_events.go)
//     和 legacy session 类型 (PersistedSessionMeta / SessionTranscriptMessage)，
//     不应新增对 bridge 的直接业务调用
//   - 允许: internal/config, internal/pkg/settings, internal/pkg/workspace,
//     internal/session — TUI 层必需的配置/会话类型
//   - 禁止: internal/tools, internal/tools/git — 已迁移至 coreapi.Engine
//   - 测试允许: internal/tools/fileops — 版本测试的数据准备工具
package adapter

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/pkg/workspace"
	"github.com/dreamSailing/eos/internal/session"
	sharedcore "github.com/dreamSailing/eos/pkg/core"
	"github.com/dreamSailing/eos/pkg/coreapi"
	coreapijsonrpc "github.com/dreamSailing/eos/pkg/coreapi/jsonrpc"
	"github.com/dreamSailing/eos/pkg/protocol"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

// RuntimeAdapter �?RuntimeCore �?Bubble Tea UI 之间的适配�?
type RuntimeAdapter struct {
	runtime *sharedcore.Runtime
	core    *bridge.RuntimeCore
	engine  coreapi.Engine
	rpc     *protocoljsonrpc.InProcessClient

	useLegacyEvents bool

	eventsOnce      sync.Once
	pumpsOnce       sync.Once
	eventsCh        chan RuntimeEvent
	notificationCh  chan RuntimeEvent
	subscribersMu   sync.Mutex
	subscribers     map[int]chan RuntimeEvent
	nextSubscriber  int
	activeTurnMu    sync.Mutex
	activeTurn      coreapi.TurnRef
	activeTurnAlive bool
}

// NewRuntimeAdapter 创建新的 RuntimeAdapter 实例
func NewRuntimeAdapter(core *bridge.RuntimeCore) *RuntimeAdapter {
	rt := sharedcore.NewRuntimeFromLegacyCore(core)
	return &RuntimeAdapter{
		runtime:         rt,
		core:            core,
		engine:          sharedcore.NewLegacyEngine(rt),
		useLegacyEvents: true,
		notificationCh:  make(chan RuntimeEvent, 128),
		subscribers:     map[int]chan RuntimeEvent{},
	}
}

func NewRuntimeAdapterFromRuntime(runtime *sharedcore.Runtime) *RuntimeAdapter {
	return newRuntimeAdapter(runtime, false)
}

func newRuntimeAdapter(runtime *sharedcore.Runtime, useLegacyEvents bool) *RuntimeAdapter {
	a := &RuntimeAdapter{
		runtime:         runtime,
		useLegacyEvents: useLegacyEvents,
		notificationCh:  make(chan RuntimeEvent, 128),
		subscribers:     map[int]chan RuntimeEvent{},
	}
	if runtime != nil {
		a.core = runtime.LegacyCore()
		a.engine = sharedcore.NewLegacyEngine(runtime)
		client, err := runtime.JSONRPCClient(coreapijsonrpc.Options{
			ServerName:      "eos-tui-core",
			ProtocolVersion: "v1",
			Notifier: coreapijsonrpc.NotifierFunc(func(ctx context.Context, notification protocoljsonrpc.Notification) error {
				return a.handleNotification(ctx, notification)
			}),
		})
		if err == nil {
			a.rpc = client
		}
	}
	return a
}

// Events 返回运行时事件通道
func (a *RuntimeAdapter) Events() <-chan RuntimeEvent {
	a.eventsOnce.Do(func() {
		a.eventsCh = make(chan RuntimeEvent, 128)
		uiEvents, unsubscribe := a.subscribeEvents(128)
		go func() {
			defer unsubscribe()
			defer close(a.eventsCh)
			for event := range uiEvents {
				a.eventsCh <- event
			}
		}()
		a.startEventPumps()
	})
	return a.eventsCh
}

// Invoke 调用 RuntimeCore �?GraphInvokePlanWithImages 方法
func (a *RuntimeAdapter) Invoke(ctx context.Context, query, executionMode string, imagePaths []string) (string, error) {
	if a != nil && a.rpc != nil {
		return a.invokeJSONRPC(ctx, query, executionMode, imagePaths)
	}
	msg, err := a.core.GraphInvokePlanWithImages(ctx, query, executionMode, imagePaths)
	if err != nil {
		return "", err
	}
	if msg == nil {
		return "", nil
	}
	return msg.Content, nil
}

func (a *RuntimeAdapter) ExecuteBash(ctx context.Context, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("command required")
	}
	if a != nil && a.rpc != nil {
		args, _ := json.Marshal(map[string]any{"command": command})
		sessionID, _ := a.CurrentSessionID(ctx)
		req := coreapi.ToolRequest{
			SessionID: sessionID,
			RequestID: fmt.Sprintf("tui_bash_%d", time.Now().UnixNano()),
			Name:      "bash",
			Args:      args,
		}
		var result coreapi.ToolResult
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodToolExecute, req, &result); err == nil {
			output := toolResultOutputText(result)
			if output == "" {
				output = strings.TrimRight(result.Display, "\n")
			}
			if strings.TrimSpace(result.Error) != "" || strings.EqualFold(result.Status, "error") {
				if strings.TrimSpace(output) == "" {
					output = strings.TrimSpace(result.Error)
				}
				return output, errors.New(firstNonEmptyString(result.Error, output, "bash failed"))
			}
			return output, nil
		}
	}
	if a == nil || a.core == nil {
		return "", errors.New("runtime core is not available")
	}
	return a.core.ExecuteBash(ctx, command)
}

func (a *RuntimeAdapter) ToolTraces(ctx context.Context) ([]coreapi.ToolTrace, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.ToolTrace
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodToolTraces, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.engine != nil {
		return a.engine.ToolTelemetry().Traces(ctx)
	}
	return nil, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) ToolStats(ctx context.Context) ([]coreapi.ToolStat, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.ToolStat
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodToolStats, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.engine != nil {
		return a.engine.ToolTelemetry().Stats(ctx)
	}
	return nil, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) CancelForegroundRequest() bool {
	if a == nil {
		return false
	}
	if a.rpc != nil {
		if ref, ok := a.currentActiveTurn(); ok && strings.TrimSpace(ref.TurnID) != "" {
			var out map[string]any
			if err := a.rpc.Call(context.Background(), protocoljsonrpc.MethodTurnInterrupt, ref, &out); err == nil {
				a.clearActiveTurn(ref.TurnID)
				return true
			}
		}
	}
	if a.core == nil {
		return false
	}
	return a.core.CancelForegroundRequest()
}

// GetContext 获取会话上下文管理器
// TODO(codex-fallback-contract): 返回 live *session.ContextManager 引用，无法通过 JSON-RPC 迁移。
// 需要等 ContextManager 自身暴露为 JSON-RPC 服务后才可收缩此路径。
func (a *RuntimeAdapter) GetContext() *session.ContextManager {
	return a.core.GetContext()
}

// TODO(codex-fallback-contract): 返回 live *settings.Manager 引用，无法通过 JSON-RPC 迁移。
// Settings 的读写已通过 Settings()/SaveSettings() 迁移，此方法仅供少数需要直接 Manager 的调用方使用。
func (a *RuntimeAdapter) GetSettings() *settings.Manager {
	return a.core.GetSettingsManager()
}

func (a *RuntimeAdapter) Settings(ctx context.Context) (settings.Settings, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.Settings
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodConfigSettingsGet, nil, &out); err == nil {
			return settingsFromCoreAPI(out), nil
		}
	}
	if a != nil && a.runtime != nil {
		return settingsFromCoreAPI(coreAPISettingsFromRuntime(a.runtime.GetSettings())), nil
	}
	if a == nil || a.core == nil {
		return settings.Settings{}, errors.New("runtime core is not available")
	}
	return a.core.GetSettings(), nil
}

func (a *RuntimeAdapter) SaveSettings(ctx context.Context, s settings.Settings) error {
	req := coreAPISettingsFromInternal(s)
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodConfigSettingsSave, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.SaveSettings(sharedcore.Settings{
			PlanPromptStyle:      req.PlanPromptStyle,
			PlanBubbleColor:      req.PlanBubbleColor,
			AutoContext:          cloneAdapterBoolPtr(req.AutoContext),
			DesktopNotifications: cloneAdapterBoolPtr(req.DesktopNotifications),
			MaxInjectKB:          req.MaxInjectKB,
			WatchMode:            req.WatchMode,
			WatchDebounceMs:      req.WatchDebounceMs,
			PollIntervalSec:      req.PollIntervalSec,
			Language:             req.Language,
			Theme:                req.Theme,
			Trusted:              cloneAdapterBoolPtr(req.Trusted),
			MaxTurnTokens:        req.MaxTurnTokens,
			MaxSessionTokens:     req.MaxSessionTokens,
			MidRiskConfirm:       req.MidRiskConfirm,
		})
	}
	return errors.New("runtime core is not available")
}

// GetWorkspace 获取工作区管理器
// TODO(codex-fallback-contract): 返回 live *workspace.Manager 引用，无法通过 JSON-RPC 迁移。
// Workspace CRUD 已通过 Workspaces()/AddWorkspace() 等迁移，此方法仅供少数需要直接 Manager 的调用方使用。
func (a *RuntimeAdapter) GetWorkspace() *workspace.Manager {
	return a.core.GetWorkspace()
}

// GetModelInfo 获取当前模型信息
func (a *RuntimeAdapter) GetModelInfo() (modelName, modelBase string) {
	if a != nil && a.runtime != nil {
		for _, desc := range a.runtime.ListModelDescriptors() {
			if desc.IsActive {
				return strings.TrimSpace(desc.Model), strings.TrimSpace(desc.APIBase)
			}
		}
	}
	if a == nil || a.core == nil {
		return "", ""
	}
	return a.core.ModelName(), a.core.ModelBase()
}

// ResolveAPIConfig 解析 API 配置
func (a *RuntimeAdapter) ResolveAPIConfig() (base, provider, model, key string) {
	if a != nil && a.runtime != nil {
		for _, desc := range a.runtime.ListModelDescriptors() {
			if desc.IsActive {
				return resolveAPIConfigFromDescriptor(desc)
			}
		}
	}
	if a == nil || a.core == nil {
		return "", "", "", ""
	}
	return a.core.ResolveAPIConfig()
}

// SetActiveModel 设置活动模型
func (a *RuntimeAdapter) SetActiveModel(name string) bool {
	return a.ActivateModel(context.Background(), name) == nil
}

func (a *RuntimeAdapter) Models(ctx context.Context) ([]coreapi.ModelConfig, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.ModelConfig
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodModelList, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPIModelsFromRuntime(a.runtime.ListModelDescriptors()), nil
	}
	if a == nil || a.core == nil {
		return nil, errors.New("runtime core is not available")
	}
	cfg, _ := a.core.LoadFullModelConfig()
	out := make([]coreapi.ModelConfig, 0, len(cfg.Models))
	for _, item := range cfg.Models {
		out = append(out, coreapi.ModelConfig{
			Name:                    strings.TrimSpace(item.Name),
			APIBase:                 strings.TrimSpace(item.APIBase),
			Model:                   strings.TrimSpace(item.Model),
			Source:                  strings.TrimSpace(item.Source),
			Active:                  strings.EqualFold(strings.TrimSpace(cfg.Active), strings.TrimSpace(item.Name)),
			SupportsReasoningEffort: item.SupportsReasoningEffort,
			CanEdit:                 true,
			CanDelete:               !strings.EqualFold(strings.TrimSpace(cfg.Active), strings.TrimSpace(item.Name)),
		})
	}
	return out, nil
}

func (a *RuntimeAdapter) ModelEntries(ctx context.Context) ([]config.ModelEntry, string, error) {
	items, err := a.Models(ctx)
	if err != nil {
		return nil, "", err
	}
	entries := make([]config.ModelEntry, 0, len(items))
	active := ""
	for _, item := range items {
		entries = append(entries, config.ModelEntry{
			Name:                    strings.TrimSpace(item.Name),
			APIBase:                 strings.TrimSpace(item.APIBase),
			APIKey:                  strings.TrimSpace(item.APIKeyMasked),
			Model:                   strings.TrimSpace(item.Model),
			Source:                  strings.TrimSpace(item.Source),
			SupportsReasoningEffort: item.SupportsReasoningEffort,
		})
		if item.Active {
			active = strings.TrimSpace(item.Name)
		}
	}
	return entries, active, nil
}

func (a *RuntimeAdapter) ActivateModel(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("model name required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodModelActivate, coreapi.ModelNameRequest{Name: name}, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.ActivateModel(name)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) DeleteModel(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("model name required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodModelDelete, coreapi.ModelNameRequest{Name: name}, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.DeleteModel(name)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) UpsertModelEntry(ctx context.Context, entry config.ModelEntry) error {
	req := coreapi.UpsertModelRequest{
		Name:    strings.TrimSpace(entry.Name),
		APIBase: strings.TrimSpace(entry.APIBase),
		APIKey:  strings.TrimSpace(entry.APIKey),
		Model:   strings.TrimSpace(entry.Model),
	}
	if req.Name == "" || req.APIBase == "" || req.Model == "" {
		return errors.New("name, api base, model required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodModelUpsert, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.UpsertModel(req.Name, req.APIBase, req.APIKey, req.Model)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) SyncEnvModel(ctx context.Context) error {
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodModelSyncEnv, nil, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.SyncEnvModel()
	}
	return errors.New("runtime is not available")
}

// GetCore 获取 RuntimeCore 实例（用于高级操作）
// TODO(codex-fallback-contract): 逃逸舱口，允许调用方直接访问 bridge.RuntimeCore。
// 仅在 adapter 方法无法覆盖的极少数场景使用（如测试中的 token 注入）。
// 长期目标：所有 RuntimeCore 操作都应有对应的 adapter 方法，逐步消除对此的依赖。
func (a *RuntimeAdapter) GetCore() *bridge.RuntimeCore {
	return a.core
}

func (a *RuntimeAdapter) StartContextEngine(ctx context.Context, workspacePath string) error {
	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		return errors.New("workspace path required")
	}
	if a != nil && a.runtime != nil {
		if err := a.runtime.SetForegroundWorkspace(workspacePath); err != nil {
			return err
		}
	}
	if a == nil || a.core == nil {
		return errors.New("runtime core is not available")
	}
	a.core.StartContextEngine(workspacePath)
	return nil
}

// Reload 重新加载运行时
// TODO(codex-fallback-contract): pkg/protocol/jsonrpc 尚未定义 runtime/reload 方法，
// 暂时保留 a.core.Reload() 直连。待 protocol 层添加对应方法后迁移。
func (a *RuntimeAdapter) Reload() error {
	return a.core.Reload()
}

func (a *RuntimeAdapter) StateSnapshot(ctx context.Context) (coreapi.StateSnapshot, error) {
	if a == nil || a.rpc == nil {
		return coreapi.StateSnapshot{}, errors.New("jsonrpc client is not available")
	}
	var out coreapi.StateSnapshot
	if err := a.rpc.Call(ctx, protocoljsonrpc.MethodStateSnapshot, nil, &out); err != nil {
		return coreapi.StateSnapshot{}, err
	}
	return out, nil
}

func (a *RuntimeAdapter) Workspaces(ctx context.Context) ([]coreapi.Workspace, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.Workspace
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodWorkspaceList, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		items := a.runtime.ListWorkspaces()
		out := make([]coreapi.Workspace, 0, len(items))
		for _, item := range items {
			out = append(out, coreapi.Workspace{
				Path:    strings.TrimSpace(item.Path),
				Trusted: item.Trusted,
				Active:  item.Active,
			})
		}
		return out, nil
	}
	return nil, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) ActiveWorkspace(ctx context.Context) string {
	if a != nil && a.rpc != nil {
		if snapshot, err := a.StateSnapshot(ctx); err == nil && strings.TrimSpace(snapshot.ForegroundWorkspace) != "" {
			return strings.TrimSpace(snapshot.ForegroundWorkspace)
		}
	}
	if a != nil && a.runtime != nil {
		if snapshot := a.runtime.RuntimeSnapshot(); strings.TrimSpace(snapshot.ForegroundWorkspace) != "" {
			return strings.TrimSpace(snapshot.ForegroundWorkspace)
		}
	}
	return ""
}

func (a *RuntimeAdapter) AddWorkspace(ctx context.Context, path string) error {
	req := coreapi.WorkspacePathRequest{Path: strings.TrimSpace(path)}
	if req.Path == "" {
		return errors.New("workspace path required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodWorkspaceAdd, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.AddWorkspace(req.Path)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) RemoveWorkspace(ctx context.Context, path string) error {
	req := coreapi.WorkspacePathRequest{Path: strings.TrimSpace(path)}
	if req.Path == "" {
		return errors.New("workspace path required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodWorkspaceRemove, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.RemoveWorkspace(req.Path)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) UseWorkspace(ctx context.Context, path string) error {
	req := coreapi.WorkspacePathRequest{Path: strings.TrimSpace(path)}
	if req.Path == "" {
		return errors.New("workspace path required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodWorkspaceUse, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.UseWorkspace(req.Path)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) TrustWorkspace(ctx context.Context, path string) error {
	req := coreapi.WorkspacePathRequest{Path: strings.TrimSpace(path)}
	if req.Path == "" {
		return errors.New("workspace path required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodWorkspaceTrust, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.TrustWorkspace(req.Path)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) PermissionSnapshot(ctx context.Context) (coreapi.PermissionSnapshot, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.PermissionSnapshot
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodPermissionSnapshot, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return permissionSnapshotFromRuntime(a.runtime.PermissionSnapshot()), nil
	}
	return coreapi.PermissionSnapshot{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) ModeSnapshot(ctx context.Context) (coreapi.ModeSnapshot, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.ModeSnapshot
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodRuntimeModesGet, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreapi.ModeSnapshot{
			ExecutionMode:  strings.TrimSpace(a.runtime.ExecutionMode()),
			SandboxMode:    strings.TrimSpace(a.runtime.SandboxMode()),
			ReasoningLevel: strings.TrimSpace(a.runtime.ReasoningLevel()),
		}, nil
	}
	return coreapi.ModeSnapshot{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) SetExecutionMode(ctx context.Context, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return nil
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodRuntimeExecutionModeSet, coreapi.SetModeRequest{Mode: mode}, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		a.runtime.SetExecutionMode(mode)
		return nil
	}
	if a != nil && a.core != nil {
		a.core.SetExecutionMode(mode)
		return nil
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) SetAccessMode(ctx context.Context, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return nil
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodPermissionAccessModeSet, coreapi.SetModeRequest{Mode: mode}, nil); err == nil {
			return nil
		}
	}
	if a == nil || a.core == nil {
		return errors.New("runtime core is not available")
	}
	a.core.SetAccessMode(mode)
	return nil
}

func (a *RuntimeAdapter) SetApprovalMode(ctx context.Context, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return nil
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodPermissionApprovalModeSet, coreapi.SetModeRequest{Mode: mode}, nil); err == nil {
			return nil
		}
	}
	if a == nil || a.core == nil {
		return errors.New("runtime core is not available")
	}
	a.core.SetApprovalMode(mode)
	return nil
}

func (a *RuntimeAdapter) PendingReview(ctx context.Context) (coreapi.PendingReview, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.PendingReview
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodPermissionPendingReview, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPIPendingReviewFromRuntime(a.runtime.PendingReview()), nil
	}
	return coreapi.PendingReview{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) Skills(ctx context.Context) ([]coreapi.SkillInfo, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.SkillInfo
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodExtensionsSkillsList, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPISkillsFromRuntime(a.runtime.ListSkills()), nil
	}
	return nil, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) ReloadSkills(ctx context.Context) error {
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodExtensionsSkillsReload, nil, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.ReloadSkills()
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) InvokeSkill(ctx context.Context, name, arguments string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, nil
	}
	if a != nil && a.rpc != nil {
		var out coreapi.InvokeSkillResult
		req := coreapi.InvokeSkillRequest{Name: name, Arguments: strings.TrimSpace(arguments)}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodExtensionsSkillInvoke, req, &out); err == nil {
			return out.Invoked, nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.InvokeSkill(name, arguments)
	}
	return false, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) Plugins(ctx context.Context) ([]coreapi.PluginInfo, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.PluginInfo
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodExtensionsPluginsList, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPIPluginsFromRuntime(a.runtime.ListPlugins()), nil
	}
	return nil, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) Tasks(ctx context.Context) ([]coreapi.TaskSnapshot, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.TaskSnapshot
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodTaskList, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPITaskSnapshotsFromRuntime(a.runtime.ListTasks()), nil
	}
	return nil, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) KillTask(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("task id required")
	}
	if a != nil && a.rpc != nil {
		req := coreapi.TaskIDRequest{TaskID: taskID}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodTaskKill, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.KillTask(taskID)
	}
	return errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) TailTask(ctx context.Context, taskID string) ([]string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("task id required")
	}
	if a != nil && a.rpc != nil {
		var out []string
		req := coreapi.TaskIDRequest{TaskID: taskID}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodTaskTail, req, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.TailTask(taskID)
	}
	return nil, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) CleanupTasks(ctx context.Context) (int, error) {
	if a != nil && a.rpc != nil {
		var out struct {
			Count int `json:"count"`
		}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodTaskCleanup, nil, &out); err == nil {
			return out.Count, nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.CleanupTasks(), nil
	}
	return 0, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) Todos(ctx context.Context) ([]coreapi.TodoItem, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.TodoItem
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodTaskTodos, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.engine != nil {
		return a.engine.Tasks().Todos(ctx)
	}
	return nil, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) Agents(ctx context.Context) ([]coreapi.Agent, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.Agent
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodAgentList, nil, &out); err == nil {
			return out, nil
		}
	}
	return nil, nil
}

func (a *RuntimeAdapter) BrowserStatus(ctx context.Context) (coreapi.BrowserStatus, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.BrowserStatus
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodBrowserStatus, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPIBrowserStatusFromRuntime(a.runtime.BrowserStatus()), nil
	}
	return coreapi.BrowserStatus{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) CurrentRemoteRepo(ctx context.Context) (coreapi.RemoteRepoState, bool, error) {
	if a != nil && a.rpc != nil {
		var out struct {
			OK    bool                    `json:"ok"`
			State coreapi.RemoteRepoState `json:"state"`
		}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodRemoteRepoCurrent, nil, &out); err == nil {
			return out.State, out.OK, nil
		}
	}
	if a != nil && a.runtime != nil {
		state, ok := a.runtime.CurrentRemoteRepo()
		return coreAPIRemoteRepoFromRuntime(state), ok, nil
	}
	return coreapi.RemoteRepoState{}, false, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) GitStatus(ctx context.Context) ([]coreapi.GitChange, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.GitChange
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodGitStatus, coreapi.GitStatusRequest{}, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.engine != nil {
		return a.engine.Git().Status(ctx, coreapi.GitStatusRequest{})
	}
	return nil, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) GitDiff(ctx context.Context, path string) (string, error) {
	req := coreapi.GitDiffRequest{Path: strings.TrimSpace(path)}
	if a != nil && a.rpc != nil {
		var out coreapi.GitTextResult
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodGitDiff, req, &out); err == nil {
			return out.Text, nil
		}
	}
	if a != nil && a.engine != nil {
		out, err := a.engine.Git().Diff(ctx, req)
		return out.Text, err
	}
	return "", errors.New("runtime is not available")
}

func (a *RuntimeAdapter) GitBranches(ctx context.Context) (coreapi.GitBranchesResult, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.GitBranchesResult
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodGitBranches, coreapi.GitBranchesRequest{}, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.engine != nil {
		return a.engine.Git().Branches(ctx, coreapi.GitBranchesRequest{})
	}
	return coreapi.GitBranchesResult{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) GitLog(ctx context.Context, req coreapi.GitLogRequest) (coreapi.GitLogResult, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.GitLogResult
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodGitLog, req, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.engine != nil {
		return a.engine.Git().Log(ctx, req)
	}
	return coreapi.GitLogResult{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) GitShow(ctx context.Context, req coreapi.GitShowRequest) (coreapi.GitShowResult, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.GitShowResult
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodGitShow, req, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.engine != nil {
		return a.engine.Git().Show(ctx, req)
	}
	return coreapi.GitShowResult{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) MCPServers(ctx context.Context) ([]config.MCPEntry, error) {
	if a != nil && a.rpc != nil {
		var items []coreapi.MCPServer
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodMCPList, nil, &items); err == nil {
			return mcpEntriesFromCoreAPI(items), nil
		}
	}
	if a != nil && a.runtime != nil {
		return mcpEntriesFromRuntime(a.runtime.ListMCP()), nil
	}
	return nil, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) SetMCPEnabled(ctx context.Context, name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("mcp server name required")
	}
	if a != nil && a.rpc != nil {
		req := coreapi.SetMCPEnabledRequest{Name: name, Enabled: enabled}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodMCPSetEnabled, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.SetMCPEnabled(name, enabled)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) DeleteMCPServer(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("mcp server name required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodMCPDelete, coreapi.MCPNameRequest{Name: name}, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.DeleteMCP(name)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) ImportMCPJSON(ctx context.Context, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("mcp config is empty")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodMCPImportJSON, coreapi.ImportMCPJSONRequest{Raw: raw}, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.ImportMCPJSON(raw)
	}
	return errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) AddMCPEntries(ctx context.Context, entries []config.MCPEntry) error {
	if len(entries) == 0 {
		return errors.New("empty config")
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return a.ImportMCPJSON(ctx, string(raw))
}

func (a *RuntimeAdapter) UpsertMCPEntry(ctx context.Context, entry config.MCPEntry) error {
	req := coreAPIUpsertMCPRequest(entry)
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("mcp server name required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodMCPUpsert, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.UpsertMCPServer(runtimeMCPServerFromConfig(entry))
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) LSPServers(ctx context.Context) ([]coreapi.LSPServer, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.LSPServer
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodLSPList, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPILSPServersFromRuntime(a.runtime.ListLSP()), nil
	}
	return nil, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) LSPDiagnostics(ctx context.Context) ([]string, error) {
	if a != nil && a.rpc != nil {
		var out []string
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodLSPDiagnostics, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return append([]string(nil), a.runtime.LSPDiagnostics()...), nil
	}
	return nil, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) LSPDiagnosticsSummary(ctx context.Context) (coreapi.LSPDiagnosticsSummary, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.LSPDiagnosticsSummary
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodLSPDiagnosticsSummary, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.LSPDiagnosticsSummary(), nil
	}
	return coreapi.LSPDiagnosticsSummary{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) LSPDiagnosticsMarkdown(ctx context.Context) string {
	lines, err := a.LSPDiagnostics(ctx)
	if err != nil || len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func (a *RuntimeAdapter) RulesSnapshot(ctx context.Context) (coreapi.RulesSnapshot, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.RulesSnapshot
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodConfigRulesSnapshot, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPIRulesSnapshotFromRuntime(a.runtime.RulesSnapshot()), nil
	}
	return coreapi.RulesSnapshot{}, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) SaveRules(ctx context.Context, scope, content string) error {
	req := coreapi.SaveRulesRequest{
		Scope:   strings.ToLower(strings.TrimSpace(scope)),
		Content: content,
	}
	if req.Scope == "" {
		req.Scope = "project"
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodConfigRulesSave, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.SaveRulesScoped(req.Scope, req.Content)
	}
	return errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) MemorySnapshot(ctx context.Context) (coreapi.MemorySnapshot, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.MemorySnapshot
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodMemorySnapshot, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPIMemorySnapshotFromRuntime(a.runtime.MemorySnapshot()), nil
	}
	return coreapi.MemorySnapshot{}, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) SaveMemory(ctx context.Context, scope, content string) error {
	req := coreapi.SaveMemoryRequest{
		Scope:   strings.ToLower(strings.TrimSpace(scope)),
		Content: content,
	}
	if req.Scope == "" {
		req.Scope = "project"
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodMemorySave, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.SaveMemory(req.Scope, req.Content)
	}
	return errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) RebuildMemoryIndex(ctx context.Context) error {
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodMemoryRebuildIndex, nil, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.RebuildMemoryIndex()
	}
	return errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) PredictNextUserMessage(ctx context.Context, draft string) (string, error) {
	if a != nil && a.rpc != nil {
		var out struct {
			Message string `json:"message"`
		}
		req := coreapi.PredictNextUserMessageRequest{Draft: draft}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodInsightPredictNextUser, req, &out); err == nil {
			return strings.TrimSpace(out.Message), nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.PredictNextUserMessage(ctx, draft)
	}
	return "", errors.New("runtime is not available")
}

func (a *RuntimeAdapter) CurrentContextUsage(ctx context.Context) (int, float64, error) {
	if a != nil && a.rpc != nil {
		stats, err := a.ContextStats(ctx)
		if err == nil {
			window, _ := a.ContextWindowTokens(ctx)
			ratio := 0.0
			if window > 0 {
				ratio = float64(stats.Estimated) / float64(window)
			}
			return stats.Estimated, ratio, nil
		}
	}
	if a != nil && a.runtime != nil {
		_, estimated := a.runtime.ContextStats()
		window := a.runtime.ContextWindowTokens()
		ratio := 0.0
		if window > 0 {
			ratio = float64(estimated) / float64(window)
		}
		return estimated, ratio, nil
	}
	return 0, 0, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) ContextPreview(ctx context.Context) ([]string, error) {
	if a != nil && a.rpc != nil {
		var out []string
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodContextPreview, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return append([]string(nil), a.runtime.ContextPreview()...), nil
	}
	return nil, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) ContextStats(ctx context.Context) (coreapi.ContextStats, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.ContextStats
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodContextStats, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		messages, estimated := a.runtime.ContextStats()
		return coreapi.ContextStats{MessageCount: messages, Estimated: estimated}, nil
	}
	return coreapi.ContextStats{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) ContextWindowTokens(ctx context.Context) (int, error) {
	if a != nil && a.rpc != nil {
		var out struct {
			Tokens int `json:"tokens"`
		}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodContextWindow, nil, &out); err == nil {
			return out.Tokens, nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.ContextWindowTokens(), nil
	}
	return 0, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) PinContextDocument(ctx context.Context, id, content string, tokenBudget int) error {
	req := coreapi.PinDocumentRequest{
		ID:          strings.TrimSpace(id),
		Content:     content,
		TokenBudget: tokenBudget,
	}
	if req.ID == "" {
		return errors.New("document id required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodContextPin, req, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.PinContextDocument(req.ID, req.Content, req.TokenBudget)
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) CompactContext(ctx context.Context) (string, error) {
	if a != nil && a.rpc != nil {
		var out struct {
			Message string `json:"message"`
		}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodContextCompact, nil, &out); err == nil {
			return strings.TrimSpace(out.Message), nil
		}
	}
	if a != nil && a.runtime != nil {
		return strings.TrimSpace(a.runtime.CompactContext()), nil
	}
	return "", errors.New("runtime is not available")
}

func (a *RuntimeAdapter) ClearContext(ctx context.Context) error {
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodContextClear, nil, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		a.runtime.ClearContext()
		return nil
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) ExportContext(ctx context.Context, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodContextExport, coreapi.ExportContextRequest{Path: path}, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.ExportContext(path)
	}
	return errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) UsageSummary(ctx context.Context) (coreapi.UsageSummary, error) {
	if a != nil && a.rpc != nil {
		var out coreapi.UsageSummary
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodUsageSummary, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPIUsageSummaryFromRuntime(a.runtime.UsageSummary()), nil
	}
	return coreapi.UsageSummary{}, errors.New("runtime is not available")
}

func (a *RuntimeAdapter) CostItems(ctx context.Context) ([]coreapi.CostItem, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.CostItem
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodUsageCostItems, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPICostItemsFromRuntime(a.runtime.CostItems()), nil
	}
	return nil, errors.New("runtime is not available")
}

// TODO(codex-fallback-contract): pkg/protocol/jsonrpc 尚未定义 usage/clear_history 方法，
// 暂时保留 a.core.ClearTokenHistory() 直连。待 protocol 层添加对应方法后迁移。
func (a *RuntimeAdapter) ClearTokenHistory() {
	if a != nil && a.core != nil {
		a.core.ClearTokenHistory()
	}
}

func (a *RuntimeAdapter) Versions(ctx context.Context) ([]coreapi.VersionItem, error) {
	if a != nil && a.rpc != nil {
		var out []coreapi.VersionItem
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodVersionsList, nil, &out); err == nil {
			return out, nil
		}
	}
	if a != nil && a.runtime != nil {
		return coreAPIVersionItemsFromRuntime(a.runtime.ListVersions()), nil
	}
	return nil, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) RollbackVersion(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("version id required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodVersionsRollback, coreapi.VersionIDRequest{ID: id}, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.RollbackVersion(id)
	}
	return errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) DeleteVersion(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("version id required")
	}
	if a != nil && a.rpc != nil {
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodVersionsDelete, coreapi.VersionIDRequest{ID: id}, nil); err == nil {
			return nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.DeleteVersion(id)
	}
	return errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) DeleteFileVersions(ctx context.Context, file string) (int, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return 0, errors.New("file required")
	}
	if a != nil && a.rpc != nil {
		var out struct {
			Count int `json:"count"`
		}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodVersionsDeleteFile, coreapi.VersionFileRequest{File: file}, &out); err == nil {
			return out.Count, nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.DeleteFileVersions(file), nil
	}
	return 0, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) ClearVersions(ctx context.Context) (int, error) {
	if a != nil && a.rpc != nil {
		var out struct {
			Count int `json:"count"`
		}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodVersionsClear, nil, &out); err == nil {
			return out.Count, nil
		}
	}
	if a != nil && a.runtime != nil {
		return a.runtime.ClearVersions(), nil
	}
	return 0, errors.New("runtime core is not available")
}

func (a *RuntimeAdapter) ListSessions(ctx context.Context) ([]bridge.PersistedSessionMeta, error) {
	if a != nil && a.rpc != nil {
		var sessions []coreapi.Session
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodSessionList, coreapi.ListSessionsRequest{}, &sessions); err != nil {
			return nil, err
		}
		return persistedSessionMetasFromCoreAPI(sessions), nil
	}
	if a == nil || a.core == nil {
		return nil, errors.New("runtime core is not available")
	}
	return a.core.ListSessions()
}

func (a *RuntimeAdapter) CurrentSessionID(ctx context.Context) (string, error) {
	if a != nil && a.rpc != nil {
		var session coreapi.Session
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodSessionCurrent, coreapi.CurrentSessionRequest{}, &session); err != nil {
			return "", err
		}
		return strings.TrimSpace(session.ID), nil
	}
	if a != nil && a.runtime != nil {
		sid, err := a.runtime.CurrentSessionID()
		return strings.TrimSpace(sid), err
	}
	return "", errors.New("runtime is not available")
}

func (a *RuntimeAdapter) SaveSessionMessages(ctx context.Context, id string, messages []coreapi.SessionMessage) (string, error) {
	if a != nil && a.rpc != nil {
		var session coreapi.Session
		req := coreapi.SaveSessionMessagesRequest{
			SessionID: strings.TrimSpace(id),
			Messages:  messages,
		}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodSessionMessagesSave, req, &session); err != nil {
			return "", err
		}
		return strings.TrimSpace(session.ID), nil
	}
	if a == nil || a.core == nil {
		return "", errors.New("runtime core is not available")
	}
	return a.core.SaveSessionMessages(ctx, id, bridgeSessionMessagesFromCoreAPI(messages))
}

func (a *RuntimeAdapter) LoadSessionMessages(ctx context.Context, id string) ([]bridge.SessionTranscriptMessage, error) {
	if a != nil && a.rpc != nil {
		var messages []coreapi.SessionMessage
		req := coreapi.LoadSessionMessagesRequest{SessionID: strings.TrimSpace(id)}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodSessionMessagesLoad, req, &messages); err != nil {
			return nil, err
		}
		return bridgeSessionMessagesFromCoreAPI(messages), nil
	}
	if a == nil || a.core == nil {
		return nil, errors.New("runtime core is not available")
	}
	return a.core.LoadSessionMessages(id)
}

func (a *RuntimeAdapter) RenameSession(ctx context.Context, id, title string) error {
	id = strings.TrimSpace(id)
	title = strings.TrimSpace(title)
	if id == "" {
		return errors.New("session id required")
	}
	if a != nil && a.rpc != nil {
		var session coreapi.Session
		req := coreapi.RenameSessionRequest{SessionID: id, Title: title}
		if err := a.rpc.Call(ctx, protocoljsonrpc.MethodSessionRename, req, &session); err == nil {
			return nil
		}
	}
	if a == nil || a.core == nil {
		return errors.New("runtime core is not available")
	}
	return a.core.UpdateSessionTitle(id, title)
}

func (a *RuntimeAdapter) ResumeSession(ctx context.Context, id string) error {
	if a != nil && a.rpc != nil {
		var session coreapi.Session
		req := coreapi.ResumeSessionRequest{SessionID: strings.TrimSpace(id)}
		return a.rpc.Call(ctx, protocoljsonrpc.MethodSessionResume, req, &session)
	}
	if a == nil || a.core == nil {
		return errors.New("runtime core is not available")
	}
	return a.core.ResumeSession(ctx, id)
}

func (a *RuntimeAdapter) SessionsDir(ctx context.Context) string {
	root := ""
	if a != nil {
		root = strings.TrimSpace(a.ActiveWorkspace(ctx))
	}
	if root == "" {
		root, _ = filepath.Abs(".")
	}
	return filepath.Join(root, ".eos", "sessions")
}

func (a *RuntimeAdapter) ExportSessionMarkdown(ctx context.Context, id, outputPath string) error {
	id = strings.TrimSpace(id)
	outputPath = strings.TrimSpace(outputPath)
	if id == "" {
		return errors.New("session id required")
	}
	if outputPath == "" {
		return errors.New("output path required")
	}
	bridgeMessages, err := a.LoadSessionMessages(ctx, id)
	if err != nil {
		return err
	}
	converted := make([]coreapi.SessionMessage, 0, len(bridgeMessages))
	for _, m := range bridgeMessages {
		converted = append(converted, coreapi.SessionMessage{
			Role:       m.Role,
			Type:       m.Type,
			Content:    m.Content,
			Time:       time.Unix(m.Timestamp, 0),
			ImagePaths: append([]string(nil), m.ImagePaths...),
			Metadata:   cloneMap(m.Metadata),
		})
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(renderSessionMarkdown(id, converted)), 0o644)
}

func (a *RuntimeAdapter) RespondPrompt(ctx context.Context, id, kind string, response PromptResponse) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if a != nil && a.rpc != nil {
		if strings.EqualFold(strings.TrimSpace(kind), "inquiry") {
			req := coreapi.InquiryResponse{
				InquiryID: id,
				Option:    strings.TrimSpace(response.Option),
				Text:      strings.TrimSpace(response.Text),
			}
			if err := a.rpc.Call(ctx, protocoljsonrpc.MethodInquiryRespond, req, nil); err == nil {
				return nil
			}
		} else {
			decision := strings.TrimSpace(response.Decision)
			if decision == "" {
				decision = strings.TrimSpace(response.Option)
			}
			req := coreapi.ApprovalResponse{
				ApprovalID: id,
				Decision:   decision,
				Message:    strings.TrimSpace(response.Text),
			}
			if err := a.rpc.Call(ctx, protocoljsonrpc.MethodApprovalRespond, req, nil); err == nil {
				return nil
			}
		}
	}
	return errors.New("runtime is not available")
}

func (a *RuntimeAdapter) invokeJSONRPC(ctx context.Context, query, executionMode string, imagePaths []string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	a.startEventPumps()
	if err := a.SetExecutionMode(ctx, executionMode); err != nil {
		return "", err
	}
	sessionID, _ := a.CurrentSessionID(ctx)
	turnID := fmt.Sprintf("tui_turn_%d", time.Now().UnixNano())
	events, unsubscribe := a.subscribeEvents(128)
	defer unsubscribe()
	a.setActiveTurn(coreapi.TurnRef{SessionID: sessionID, TurnID: turnID})
	defer a.clearActiveTurn(turnID)

	req := coreapi.StartTurnRequest{
		SessionID:  sessionID,
		TurnID:     turnID,
		Input:      query,
		ImagePaths: append([]string(nil), imagePaths...),
	}
	var turn coreapi.Turn
	if err := a.rpc.Call(ctx, protocoljsonrpc.MethodTurnStart, req, &turn); err != nil {
		return "", err
	}
	if strings.TrimSpace(turn.ID) != "" {
		turnID = strings.TrimSpace(turn.ID)
		a.setActiveTurn(coreapi.TurnRef{SessionID: strings.TrimSpace(turn.SessionID), TurnID: turnID})
	}

	var final string
	for {
		select {
		case <-ctx.Done():
			_, _ = a.interruptTurn(context.Background(), coreapi.TurnRef{SessionID: sessionID, TurnID: turnID})
			return "", ctx.Err()
		case event, ok := <-events:
			if !ok {
				return final, nil
			}
			if rid := strings.TrimSpace(event.RID); rid != "" && rid != turnID {
				continue
			}
			switch event.Type {
			case string(protocol.EventTypeTextFinal), "final":
				final = firstEventText(event.Data, event.Content, "text", "message")
			case string(protocol.EventTypeRequestDone):
				return final, nil
			case string(protocol.EventTypeRequestFailed), "error":
				msg := firstEventText(event.Data, event.Content, "error", "summary", "message", "text")
				if msg == "" {
					msg = "request failed"
				}
				return final, errors.New(msg)
			}
		}
	}
}

func (a *RuntimeAdapter) interruptTurn(ctx context.Context, ref coreapi.TurnRef) (bool, error) {
	if a == nil || a.rpc == nil || strings.TrimSpace(ref.TurnID) == "" {
		return false, nil
	}
	var out map[string]any
	err := a.rpc.Call(ctx, protocoljsonrpc.MethodTurnInterrupt, ref, &out)
	return err == nil, err
}

func (a *RuntimeAdapter) handleNotification(ctx context.Context, notification protocoljsonrpc.Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	event, ok := bridgeEventFromNotification(notification)
	if !ok {
		return nil
	}
	select {
	case a.notificationCh <- event:
	default:
	}
	return nil
}

func (a *RuntimeAdapter) startEventPumps() {
	if a == nil {
		return
	}
	a.pumpsOnce.Do(func() {
		go func() {
			// bridge 事件归一化已被 protocol.Envelope 路径取代；
			// 保留 useLegacyEvents 标志位以保持 API 兼容，但已不依赖 bridge。
			_ = a.useLegacyEvents
			for {
				select {
				case event := <-a.notificationCh:
					a.publishEvent(event)
				}
			}
		}()
	})
}

func (a *RuntimeAdapter) subscribeEvents(buffer int) (<-chan RuntimeEvent, func()) {
	if buffer <= 0 {
		buffer = 64
	}
	ch := make(chan RuntimeEvent, buffer)
	a.subscribersMu.Lock()
	a.nextSubscriber++
	id := a.nextSubscriber
	a.subscribers[id] = ch
	a.subscribersMu.Unlock()
	a.startEventPumps()
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			a.subscribersMu.Lock()
			if cur, ok := a.subscribers[id]; ok {
				delete(a.subscribers, id)
				close(cur)
			}
			a.subscribersMu.Unlock()
		})
	}
	return ch, unsubscribe
}

func (a *RuntimeAdapter) publishEvent(event RuntimeEvent) {
	a.subscribersMu.Lock()
	defer a.subscribersMu.Unlock()
	for _, ch := range a.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (a *RuntimeAdapter) setActiveTurn(ref coreapi.TurnRef) {
	a.activeTurnMu.Lock()
	a.activeTurn = ref
	a.activeTurnAlive = strings.TrimSpace(ref.TurnID) != ""
	a.activeTurnMu.Unlock()
}

func (a *RuntimeAdapter) currentActiveTurn() (coreapi.TurnRef, bool) {
	a.activeTurnMu.Lock()
	defer a.activeTurnMu.Unlock()
	return a.activeTurn, a.activeTurnAlive
}

func (a *RuntimeAdapter) clearActiveTurn(turnID string) {
	a.activeTurnMu.Lock()
	if strings.TrimSpace(turnID) == "" || strings.TrimSpace(a.activeTurn.TurnID) == strings.TrimSpace(turnID) {
		a.activeTurn = coreapi.TurnRef{}
		a.activeTurnAlive = false
	}
	a.activeTurnMu.Unlock()
}

func bridgeEventFromNotification(notification protocoljsonrpc.Notification) (RuntimeEvent, bool) {
	if notification.Method != protocoljsonrpc.NotificationEvent {
		return RuntimeEvent{}, false
	}
	var envelope protocol.Envelope
	if err := json.Unmarshal(notification.Params, &envelope); err != nil {
		return RuntimeEvent{}, false
	}
	return runtimeEventFromEnvelope(envelope), true
}

func bridgeSessionMessagesFromCoreAPI(items []coreapi.SessionMessage) []bridge.SessionTranscriptMessage {
	out := make([]bridge.SessionTranscriptMessage, 0, len(items))
	for _, item := range items {
		var ts int64
		if !item.Time.IsZero() {
			ts = item.Time.Unix()
		}
		out = append(out, bridge.SessionTranscriptMessage{
			Role:       strings.TrimSpace(item.Role),
			Type:       strings.TrimSpace(item.Type),
			Content:    item.Content,
			Timestamp:  ts,
			ImagePaths: append([]string(nil), item.ImagePaths...),
			Metadata:   cloneMap(item.Metadata),
		})
	}
	return out
}

func persistedSessionMetasFromCoreAPI(items []coreapi.Session) []bridge.PersistedSessionMeta {
	out := make([]bridge.PersistedSessionMeta, 0, len(items))
	for _, item := range items {
		meta := item.Metadata
		out = append(out, bridge.PersistedSessionMeta{
			ID:      strings.TrimSpace(item.ID),
			SavedAt: item.UpdatedAt.Unix(),
			Model:   mapString(meta, "model"),
			Summary: mapString(meta, "summary"),
			Preview: mapString(meta, "preview"),
			Title:   mapString(meta, "title"),
			Rounds:  mapInt(meta, "rounds"),
			Tokens:  mapInt(meta, "tokens"),
		})
	}
	return out
}

func permissionSnapshotFromRuntime(snapshot sharedcore.PermissionSnapshot) coreapi.PermissionSnapshot {
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

func permissionSnapshotFromBridge(snapshot bridge.PermissionSnapshot) coreapi.PermissionSnapshot {
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

func coreAPIPendingReviewFromRuntime(review sharedcore.PendingReview) coreapi.PendingReview {
	return coreapi.PendingReview{
		Path:    strings.TrimSpace(review.Path),
		Diff:    review.Diff,
		HasDiff: review.HasDiff,
	}
}

func coreAPISkillsFromRuntime(items []sharedcore.SkillInfo) []coreapi.SkillInfo {
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

func coreAPIPluginsFromRuntime(items []sharedcore.PluginInfo) []coreapi.PluginInfo {
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

func coreAPIBrowserStatusFromRuntime(status sharedcore.BrowserStatus) coreapi.BrowserStatus {
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

func coreAPITaskSnapshotsFromRuntime(items []sharedcore.BackgroundTask) []coreapi.TaskSnapshot {
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

func coreAPIRemoteRepoFromRuntime(state sharedcore.RemoteRepoState) coreapi.RemoteRepoState {
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

func coreAPIRulesSnapshotFromRuntime(snapshot sharedcore.RulesSnapshot) coreapi.RulesSnapshot {
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

func coreAPISettingsFromRuntime(s sharedcore.Settings) coreapi.Settings {
	return coreapi.Settings{
		PlanPromptStyle:      strings.TrimSpace(s.PlanPromptStyle),
		PlanBubbleColor:      strings.TrimSpace(s.PlanBubbleColor),
		AutoContext:          cloneAdapterBoolPtr(s.AutoContext),
		DesktopNotifications: cloneAdapterBoolPtr(s.DesktopNotifications),
		MaxInjectKB:          s.MaxInjectKB,
		WatchMode:            strings.TrimSpace(s.WatchMode),
		WatchDebounceMs:      s.WatchDebounceMs,
		PollIntervalSec:      s.PollIntervalSec,
		Language:             strings.TrimSpace(s.Language),
		Theme:                strings.TrimSpace(s.Theme),
		Trusted:              cloneAdapterBoolPtr(s.Trusted),
		MaxTurnTokens:        s.MaxTurnTokens,
		MaxSessionTokens:     s.MaxSessionTokens,
		MidRiskConfirm:       s.MidRiskConfirm,
	}
}

// coreAPISettingsFromInternal / settingsFromCoreAPI 已迁出至 core_client.go。
// 该测试文件仅保留 *_test.go 可见的辅助函数（adapterBoolPtr / cloneAdapterBoolPtr）。

func adapterBoolPtr(v bool) *bool {
	out := v
	return &out
}

func cloneAdapterBoolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func coreAPIMemorySnapshotFromRuntime(snapshot sharedcore.MemorySnapshot) coreapi.MemorySnapshot {
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

func mcpEntriesFromRuntime(items []sharedcore.MCPServer) []config.MCPEntry {
	out := make([]config.MCPEntry, 0, len(items))
	for _, item := range items {
		out = append(out, config.MCPEntry{
			Name:                 strings.TrimSpace(item.Name),
			Type:                 config.MCPClientType(strings.TrimSpace(item.Type)),
			Command:              strings.TrimSpace(item.Command),
			Args:                 append([]string(nil), item.Args...),
			Envs:                 cloneStringMap(item.Envs),
			BaseURL:              strings.TrimSpace(item.BaseURL),
			Enabled:              item.Enabled,
			Auth:                 configMCPAuthFromRuntime(item.Auth),
			ApprovalMode:         strings.TrimSpace(item.ApprovalMode),
			ToolApprovalOverride: cloneStringMap(item.ToolApprovalOverride),
		})
		if out[len(out)-1].Command == "" && strings.TrimSpace(item.Target) != "" {
			if out[len(out)-1].Type == config.MCPTypeSSE || out[len(out)-1].Type == config.MCPTypeStreamableHTTP {
				out[len(out)-1].BaseURL = strings.TrimSpace(item.Target)
			} else {
				out[len(out)-1].Command = strings.TrimSpace(item.Target)
			}
		}
	}
	return out
}

func runtimeMCPServerFromConfig(entry config.MCPEntry) sharedcore.MCPServer {
	target := strings.TrimSpace(entry.Command)
	if strings.TrimSpace(entry.BaseURL) != "" {
		target = strings.TrimSpace(entry.BaseURL)
	}
	return sharedcore.MCPServer{
		Name:                 strings.TrimSpace(entry.Name),
		Type:                 string(entry.Type),
		Target:               target,
		Command:              strings.TrimSpace(entry.Command),
		Args:                 append([]string(nil), entry.Args...),
		Envs:                 cloneStringMap(entry.Envs),
		BaseURL:              strings.TrimSpace(entry.BaseURL),
		Enabled:              entry.Enabled,
		Auth:                 runtimeMCPAuthFromConfig(entry.Auth),
		ApprovalMode:         strings.TrimSpace(entry.ApprovalMode),
		ToolApprovalOverride: cloneStringMap(entry.ToolApprovalOverride),
	}
}

func configMCPAuthFromRuntime(auth *sharedcore.MCPAuth) *config.MCPAuth {
	if auth == nil {
		return nil
	}
	return &config.MCPAuth{
		Type:       strings.TrimSpace(auth.Type),
		Token:      auth.Token,
		Headers:    cloneStringMap(auth.Headers),
		HeadersEnv: cloneStringMap(auth.HeadersEnv),
	}
}

func runtimeMCPAuthFromConfig(auth *config.MCPAuth) *sharedcore.MCPAuth {
	if auth == nil {
		return nil
	}
	return &sharedcore.MCPAuth{
		Type:       strings.TrimSpace(auth.Type),
		Token:      auth.Token,
		Headers:    cloneStringMap(auth.Headers),
		HeadersEnv: cloneStringMap(auth.HeadersEnv),
	}
}

func coreAPILSPServersFromRuntime(items []sharedcore.LSPServer) []coreapi.LSPServer {
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

func coreAPIUsageSummaryFromRuntime(summary sharedcore.UsageSummary) coreapi.UsageSummary {
	return coreapi.UsageSummary{
		Rounds:             summary.Rounds,
		InputTokens:        summary.InputTokens,
		ReplyTokens:        summary.ReplyTokens,
		CachedInputTokens:  summary.CachedInputTokens,
		TotalTokens:        summary.TotalTokens,
		CostUSD:            summary.CostUSD,
		UnknownUsageRounds: summary.UnknownUsageRounds,
		UnknownCostRounds:  summary.UnknownCostRounds,
	}
}

func coreAPICostItemsFromRuntime(items []sharedcore.CostItem) []coreapi.CostItem {
	out := make([]coreapi.CostItem, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.CostItem{
			Time:              item.Time,
			Model:             strings.TrimSpace(item.Model),
			InputTokens:       item.InputTokens,
			ReplyTokens:       item.ReplyTokens,
			CachedInputTokens: item.CachedInputTokens,
			TotalTokens:       item.TotalTokens,
			CostUSD:           item.CostUSD,
			UsageKnown:        item.UsageKnown,
			CostKnown:         item.CostKnown,
		})
	}
	return out
}

func coreAPIVersionItemsFromRuntime(items []sharedcore.VersionItem) []coreapi.VersionItem {
	out := make([]coreapi.VersionItem, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.VersionItem{
			ID:        strings.TrimSpace(item.ID),
			File:      filepath.ToSlash(strings.TrimSpace(item.File)),
			CreatedAt: item.CreatedAt,
			Summary:   strings.TrimSpace(item.Summary),
		})
	}
	return out
}

func coreAPIModelsFromRuntime(items []sharedcore.ModelDescriptor) []coreapi.ModelConfig {
	out := make([]coreapi.ModelConfig, 0, len(items))
	for _, item := range items {
		out = append(out, coreapi.ModelConfig{
			Name:                    strings.TrimSpace(item.Name),
			APIBase:                 strings.TrimSpace(item.APIBase),
			Model:                   strings.TrimSpace(item.Model),
			Source:                  strings.TrimSpace(item.Source),
			Active:                  item.IsActive,
			SupportsReasoningEffort: item.SupportsReasoningEffort,
			ProviderID:              strings.TrimSpace(item.ProviderID),
			APIType:                 strings.TrimSpace(item.APIType),
			PresetID:                strings.TrimSpace(item.PresetID),
			EditKind:                string(item.EditKind),
			CanEdit:                 item.CanEdit,
			CanDelete:               item.CanDelete,
		})
	}
	return out
}

// cloneStringMap 已迁出至 core_client.go。RuntimeAdapter 继续使用 adapter 包级实现。

func mapString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func mapInt(metadata map[string]any, key string) int {
	if len(metadata) == 0 {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		return 0
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func resolveAPIConfigFromDescriptor(desc sharedcore.ModelDescriptor) (string, string, string, string) {
	base := strings.TrimSpace(os.Getenv("EOS_API_BASE"))
	key := strings.TrimSpace(os.Getenv("EOS_API_KEY"))
	model := strings.TrimSpace(os.Getenv("EOS_MODEL"))
	if base == "" {
		base = strings.TrimSpace(desc.APIBase)
	}
	if key == "" {
		key = strings.TrimSpace(desc.APIKey)
	}
	if model == "" {
		model = strings.TrimSpace(desc.Model)
	}
	return base, strings.TrimSpace(desc.Source), model, key
}
