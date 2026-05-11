package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dreamSailing/eos/internal/tools"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/pkg/utils"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// ToolRiskLevel 工具风险等级
type ToolRiskLevel int

const (
	ToolRiskLow    ToolRiskLevel = iota // 只读工具，如读取文件、查看目录
	ToolRiskMedium                      // 写入工具，如创建/修改文件
	ToolRiskHigh                        // 危险工具，如删除、执行命令
)

// DispatchTask 表示调度任务的参数
type DispatchTask struct {
	Task string `json:"task" jsonschema:"required,description=任务目标与验收标准（避免指定具体命令行或逐条操作步骤）"`
}

// DispatchResult 表示子 Agent 调用的返回
type DispatchResult struct {
	AgentID string         `json:"agent_id,omitempty"`
	Task    string         `json:"task,omitempty"`   // 传递给子 Agent 的任务
	Status  string         `json:"status,omitempty"` // 生命周期状态
	Result  string         `json:"result,omitempty"` // 子 Agent 的返回内容（自然语言）
	Data    map[string]any `json:"data,omitempty"`
}

// DispatchTools 包装器，将子 Agent 包装为调度工具
type DispatchTools struct {
	plannerAgent         *react.Agent
	seniorDevAgent       *react.Agent
	testerAgent          *react.Agent
	verificationAgent    *react.Agent
	reviewerAgent        *react.Agent
	toolsManager         *tools.Manager
	currentCtx           context.Context
	currentMsgs          []*schema.Message
	onMeta               func(string)
	onReasoning          func(string) // 思考内容回调
	loopDetector         LoopDetector // 循环检测器 (供 Exec Agent 使用)
	mcpToolsInfo         string       // MCP 工具信息字符串，用于传递给子 Agent
	mcpTools             []tool.BaseTool
	subAgentMgr          *SubAgentManager // 子代理管理器
	registryID           string
	allowedToolsOverride map[string]bool
	hookMgr              *HookManager
	projectMu            sync.Mutex
	projectPrompt        string
	projectPromptAt      time.Time
	mu                   sync.RWMutex // 保护并发访问
}

// NewDispatchTools 创建调度工具包装器
func NewDispatchTools(
	ctx context.Context,
	planner, seniorDev, tester, verification, reviewer *react.Agent,
	tm *tools.Manager,
	onMeta func(string),
	detector LoopDetector,
	mcpToolsInfo string,
	mcpTools []tool.BaseTool,
) *DispatchTools {
	hm := NewHookManager(tm)
	_ = hm.LoadFromDefaultLocations(ctx)
	onMetaOrig := onMeta
	onMetaWrapped := func(line string) {
		if hm != nil {
			if msg, ok := strings.CutPrefix(line, "phase.note:"); ok {
				msg = strings.TrimSpace(msg)
				if !strings.HasPrefix(msg, "HOOK_NOTIFICATION:") && !strings.HasPrefix(msg, "HOOK_STATUS:") {
					dec, _ := hm.Notification(ctx, "phase_note", msg, "")
					if strings.TrimSpace(dec.AdditionalContext) != "" && onMetaOrig != nil {
						onMetaOrig("phase.note:HOOK_NOTIFICATION:" + strings.TrimSpace(dec.AdditionalContext))
					}
				}
			}
		}
		if onMetaOrig != nil {
			onMetaOrig(line)
		}
	}
	dt := &DispatchTools{
		plannerAgent:      planner,
		seniorDevAgent:    seniorDev,
		testerAgent:       tester,
		verificationAgent: verification,
		reviewerAgent:     reviewer,
		toolsManager:      tm,
		currentCtx:        ctx,
		onMeta:            onMetaWrapped,
		loopDetector:      detector,
		mcpToolsInfo:      mcpToolsInfo,
		mcpTools:          mcpTools,
		subAgentMgr:       NewSubAgentManager(),
		hookMgr:           hm,
	}
	dt.registryID = DefaultAgentRegistry().RegisterController(
		dt.subAgentMgr,
		func(id string, task string) error {
			_, err := dt.ResumeAgent(id, task)
			return err
		},
		func(id string) error {
			_, err := dt.CloseAgent(id)
			return err
		},
	)
	hm.SetOnMeta(onMetaWrapped)
	go startHooksWatcher(context.Background(), dt)
	return dt
}

// SetContext 设置当前的上下文和消息
func (dt *DispatchTools) SetContext(ctx context.Context, msgs []*schema.Message) {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.currentCtx = ctx
	dt.currentMsgs = msgs
}

func (dt *DispatchTools) SetAllowedToolsOverride(m map[string]bool) {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	if m == nil {
		dt.allowedToolsOverride = nil
		return
	}
	cp := make(map[string]bool, len(m))
	for k, v := range m {
		if v {
			cp[strings.ToLower(strings.TrimSpace(k))] = true
		}
	}
	dt.allowedToolsOverride = cp
}

func (dt *DispatchTools) GetAllowedToolsOverride() map[string]bool {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	if dt.allowedToolsOverride == nil {
		return nil
	}
	cp := make(map[string]bool, len(dt.allowedToolsOverride))
	for k, v := range dt.allowedToolsOverride {
		if v {
			cp[k] = true
		}
	}
	return cp
}

func (dt *DispatchTools) currentContextSnapshot() (context.Context, []*schema.Message) {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	ctx := dt.currentCtx
	if ctx == nil {
		ctx = context.Background()
	}
	msgs := make([]*schema.Message, len(dt.currentMsgs))
	copy(msgs, dt.currentMsgs)
	return ctx, msgs
}

func parseContextStrategy(raw string, fallback ContextStrategy) ContextStrategy {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "independent":
		return ContextStrategyIndependent
	case "shared":
		return ContextStrategyShared
	case "hybrid":
		return ContextStrategyHybrid
	default:
		return fallback
	}
}

func (dt *DispatchTools) resolveAgentRuntime(agentType SubAgentType) (string, *react.Agent) {
	switch agentType {
	case SubAgentTypePlanner:
		return "planner", dt.plannerAgent
	case SubAgentTypeTester:
		return "tester", dt.testerAgent
	case SubAgentTypeVerification:
		return "verification", dt.verificationAgent
	case SubAgentTypeReviewer:
		return "reviewer", dt.reviewerAgent
	default:
		return "senior-dev", dt.seniorDevAgent
	}
}

func dispatchResultFromSnapshot(snapshot AgentSnapshot, result string) DispatchResult {
	return DispatchResult{
		AgentID: snapshot.ID,
		Task:    snapshot.Task,
		Status:  string(snapshot.Status),
		Result:  strings.TrimSpace(result),
		Data: map[string]any{
			"agent_name":       snapshot.Name,
			"started_at":       snapshot.StartedAt,
			"updated_at":       snapshot.UpdatedAt,
			"completed_at":     snapshot.CompletedAt,
			"messages":         snapshot.Messages,
			"context_strategy": snapshot.Strategy,
			"allowed_tools":    append([]string(nil), snapshot.AllowedTools...),
			"can_resume":       snapshot.CanResume,
			"can_close":        snapshot.CanClose,
			"can_send_input":   snapshot.CanSendInput,
			"error":            snapshot.Error,
		},
	}
}

func (dt *DispatchTools) startManagedAgentRun(parentCtx context.Context, subCtx *SubAgentContext, role string, agent *react.Agent, task string) error {
	if dt == nil || dt.subAgentMgr == nil {
		return fmt.Errorf("dispatch tools not initialized")
	}
	if subCtx == nil {
		return fmt.Errorf("subagent context is nil")
	}
	if agent == nil {
		return fmt.Errorf("agent is nil")
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	runCtx, cancel := context.WithCancel(parentCtx)
	subCtx.mu.RLock()
	allowedTools := append([]string(nil), subCtx.allowedTools...)
	subCtx.mu.RUnlock()
	if allowed := buildAllowedToolsMap(allowedTools); allowed != nil {
		runCtx = tools.WithAllowedTools(runCtx, allowed)
	}

	if err := dt.subAgentMgr.MarkRunning(subCtx.id, task, cancel); err != nil {
		cancel()
		return err
	}
	dt.emitAgentEvent(EventAgentStarted, subCtx, task, task, nil)

	go func(baseCtx context.Context) {
		_, err := invokeRoleAgentWithSubContext(runCtx, nil, role, agent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo, subCtx, dt.subAgentMgr, dt.hookMgr)

		subCtx.mu.RLock()
		cancelRequested := subCtx.cancelReq
		currentTask := strings.TrimSpace(subCtx.task)
		subCtx.mu.RUnlock()

		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(runCtx.Err(), context.Canceled) || cancelRequested {
				subRes, cancelErr := dt.subAgentMgr.Cancel(subCtx.id, "cancelled")
				if cancelErr == nil {
					dt.emitAgentEvent(EventAgentCancelled, subCtx, subRes.Task, "cancelled", map[string]any{
						"duration_ms": subRes.Duration.Milliseconds(),
					})
					if dt.hookMgr != nil {
						_, _ = dt.hookMgr.TeammateIdle(baseCtx, subRes.Type.String(), false, "cancelled", subRes.Duration.Milliseconds())
					}
				}
				return
			}

			subRes := dt.subAgentMgr.Complete(subCtx.id, currentTask, false, err.Error())
			dt.emitAgentEvent(EventAgentFailed, subCtx, subRes.Task, err.Error(), map[string]any{
				"error":       err.Error(),
				"duration_ms": subRes.Duration.Milliseconds(),
			})
			if dt.hookMgr != nil {
				_, _ = dt.hookMgr.TeammateIdle(baseCtx, subRes.Type.String(), false, err.Error(), subRes.Duration.Milliseconds())
			}
			return
		}

		if errors.Is(runCtx.Err(), context.Canceled) || cancelRequested {
			subRes, cancelErr := dt.subAgentMgr.Cancel(subCtx.id, "cancelled")
			if cancelErr == nil {
				dt.emitAgentEvent(EventAgentCancelled, subCtx, subRes.Task, "cancelled", map[string]any{
					"duration_ms": subRes.Duration.Milliseconds(),
				})
				if dt.hookMgr != nil {
					_, _ = dt.hookMgr.TeammateIdle(baseCtx, subRes.Type.String(), false, "cancelled", subRes.Duration.Milliseconds())
				}
			}
			return
		}

		subRes := dt.subAgentMgr.Complete(subCtx.id, currentTask, true, "")
		dt.emitAgentEvent(EventAgentCompleted, subCtx, subRes.Task, dt.agentOutput(subCtx, subRes.Result), map[string]any{
			"duration_ms": subRes.Duration.Milliseconds(),
		})
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(baseCtx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
		}
	}(parentCtx)

	return nil
}

func (dt *DispatchTools) SpawnAgent(agentName string, task string, forkContext bool, strategyRaw string, allowedTools []string) (DispatchResult, error) {
	task = strings.TrimSpace(task)
	if task == "" {
		return DispatchResult{}, fmt.Errorf("task required")
	}

	agentType, _, _ := resolveForkAgent(dt, agentName)
	role, agent := dt.resolveAgentRuntime(agentType)
	if agent == nil {
		return DispatchResult{}, fmt.Errorf("fork agent not available: %s", strings.TrimSpace(agentName))
	}

	parentCtx, baseMsgs := dt.currentContextSnapshot()
	initialMsgs := make([]*schema.Message, 0, len(baseMsgs)+1)
	if forkContext {
		initialMsgs = append(initialMsgs, baseMsgs...)
	}
	initialMsgs = append(initialMsgs, schema.UserMessage(task))

	strategy := parseContextStrategy(strategyRaw, agentType.ContextStrategy())
	normalizedTools := normalizeAllowedTools(allowedTools)
	if len(normalizedTools) == 0 {
		normalizedTools = agentType.AllowedTools()
	}

	subCtx := dt.subAgentMgr.CreateContextWithStrategy(agentType, parentCtx, initialMsgs, strategy, normalizedTools)
	if err := dt.startManagedAgentRun(parentCtx, subCtx, role, agent, task); err != nil {
		return DispatchResult{}, err
	}

	snapshot := snapshotFromContext(subCtx)
	return dispatchResultFromSnapshot(snapshot, "agent started"), nil
}

func (dt *DispatchTools) SendInput(agentID string, input string) (DispatchResult, error) {
	agentID = strings.TrimSpace(agentID)
	input = strings.TrimSpace(input)
	if agentID == "" || input == "" {
		return DispatchResult{}, fmt.Errorf("agent_id and input required")
	}
	if err := dt.subAgentMgr.AddMessage(agentID, schema.UserMessage(input)); err != nil {
		return DispatchResult{}, err
	}
	subCtx, ok := dt.subAgentMgr.GetContext(agentID)
	if !ok {
		return DispatchResult{}, fmt.Errorf("subagent context not found: %s", agentID)
	}
	snapshot := snapshotFromContext(subCtx)
	result := dispatchResultFromSnapshot(snapshot, "input queued")
	result.Data["queued"] = true
	return result, nil
}

func (dt *DispatchTools) WaitAgent(agentID string, timeout time.Duration) (DispatchResult, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return DispatchResult{}, fmt.Errorf("agent_id required")
	}

	waitCtx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		waitCtx, cancel = context.WithTimeout(waitCtx, timeout)
		defer cancel()
	}

	snapshot, err := dt.subAgentMgr.Wait(waitCtx, agentID)
	result := dispatchResultFromSnapshot(snapshot, snapshot.Result)
	if errors.Is(err, context.DeadlineExceeded) {
		result.Data["timed_out"] = true
		return result, nil
	}
	return result, err
}

func (dt *DispatchTools) ResumeAgent(agentID string, task string) (DispatchResult, error) {
	agentID = strings.TrimSpace(agentID)
	task = strings.TrimSpace(task)
	if agentID == "" {
		return DispatchResult{}, fmt.Errorf("agent_id required")
	}

	subCtx, ok := dt.subAgentMgr.Resume(agentID)
	if !ok {
		return DispatchResult{}, fmt.Errorf("subagent not resumable: %s", agentID)
	}
	subCtx.mu.RLock()
	currentTask := strings.TrimSpace(subCtx.task)
	agentType := subCtx.agentType
	parentCtx := subCtx.parentCtx
	status := subCtx.status
	subCtx.mu.RUnlock()
	if status == AgentStatusRunning {
		return DispatchResult{}, fmt.Errorf("subagent already running: %s", agentID)
	}
	if task != "" {
		if err := dt.subAgentMgr.AddMessage(agentID, schema.UserMessage(task)); err != nil {
			return DispatchResult{}, err
		}
		currentTask = task
	}
	if currentTask == "" {
		currentTask = "继续处理当前任务"
	}

	role, agent := dt.resolveAgentRuntime(agentType)
	if err := dt.startManagedAgentRun(parentCtx, subCtx, role, agent, currentTask); err != nil {
		return DispatchResult{}, err
	}
	snapshot := snapshotFromContext(subCtx)
	return dispatchResultFromSnapshot(snapshot, "agent resumed"), nil
}

func (dt *DispatchTools) CloseAgent(agentID string) (DispatchResult, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return DispatchResult{}, fmt.Errorf("agent_id required")
	}
	subCtx, ok := dt.subAgentMgr.GetContext(agentID)
	if !ok {
		return DispatchResult{}, fmt.Errorf("subagent context not found: %s", agentID)
	}
	snapshot := snapshotFromContext(subCtx)
	if snapshot.Status == AgentStatusRunning {
		if err := dt.subAgentMgr.RequestCancel(agentID); err != nil {
			return DispatchResult{}, err
		}
		result := dispatchResultFromSnapshot(snapshot, "cancel requested")
		result.Data["cancel_requested"] = true
		return result, nil
	}
	dt.subAgentMgr.Remove(agentID)
	result := dispatchResultFromSnapshot(snapshot, "agent closed")
	result.Data["closed"] = true
	return result, nil
}

// InvokePlanner 调用规划师
func (dt *DispatchTools) InvokePlanner(task DispatchTask) DispatchResult {
	return dt.invokeStandardAgent(task, "planner", SubAgentTypePlanner, dt.plannerAgent, dt.mcpToolsInfo, "planner agent not initialized")
}

func (dt *DispatchTools) invokeStandardAgent(task DispatchTask, role string, subType SubAgentType, agent *react.Agent, toolInfo string, missingMsg string) DispatchResult {
	dt.mu.RLock()
	ctx := dt.currentCtx
	msgs := dt.currentMsgs
	dt.mu.RUnlock()

	if agent == nil {
		return DispatchResult{
			Task:   task.Task,
			Result: missingMsg,
		}
	}

	// 创建子代理隔离上下文
	reqID := tools.TraceIDFromContext(ctx)
	subCtx := dt.subAgentMgr.GetOrCreateRequestContext(reqID, subType, ctx, msgs)
	_ = dt.subAgentMgr.MarkRunning(subCtx.id, task.Task, nil)
	dt.emitAgentEvent(EventAgentStarted, subCtx, task.Task, task.Task, nil)

	roleCtx := tools.WithRole(ctx, role)
	_, err := invokeRoleAgentWithSubContext(roleCtx, msgs, role, agent, dt.onMeta, dt.onReasoning, toolInfo, subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		dt.subAgentMgr.Complete(subCtx.id, task.Task, false, err.Error())
		dt.emitAgentEvent(EventAgentFailed, subCtx, task.Task, err.Error(), map[string]any{"error": err.Error()})
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(ctx, subCtx.agentType.String(), false, err.Error(), time.Since(subCtx.createdAt).Milliseconds())
		}
		return DispatchResult{
			Task:   task.Task,
			Result: err.Error(),
		}
	}

	subRes := dt.subAgentMgr.Complete(subCtx.id, task.Task, true, "")
	dt.emitAgentEvent(EventAgentCompleted, subCtx, task.Task, dt.agentOutput(subCtx, subRes.Result), map[string]any{"duration_ms": subRes.Duration.Milliseconds()})
	if dt.hookMgr != nil {
		_, _ = dt.hookMgr.TeammateIdle(ctx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
	}

	return DispatchResult{
		Task:   task.Task,
		Result: subRes.Result,
	}
}

// InvokeSeniorDev 调用高级开发工程师
func (dt *DispatchTools) InvokeSeniorDev(task DispatchTask) DispatchResult {
	slog.Debug("runtime.dispatch_tools.invoke_senior_dev.start", "task", task.Task, "task_empty", task.Task == "")

	taskText := strings.TrimSpace(task.Task)
	if dt.seniorDevAgent == nil {
		return DispatchResult{
			Task:   taskText,
			Result: "senior-dev agent not initialized",
		}
	}

	dt.mu.RLock()
	msgs := dt.currentMsgs
	dt.mu.RUnlock()
	slog.Debug("runtime.dispatch_tools.invoke_senior_dev.calling", "task", taskText, "context_messages", len(msgs))

	outMsgs, err := dt.InvokeSeniorDevDirect(taskText)
	if err != nil {
		slog.Error("runtime.dispatch_tools.invoke_senior_dev.error", "error", err)
		return DispatchResult{
			Task:   taskText,
			Result: err.Error(),
		}
	}

	implementationResult := strings.TrimSpace(extractLastAssistantText(outMsgs))
	if !shouldAutoVerifySeniorDevTask(taskText) {
		return DispatchResult{
			Task:   taskText,
			Result: implementationResult,
		}
	}

	verifyTask := buildAutomaticVerificationTask(taskText, implementationResult)
	verifyMsgs, verifyErr := dt.InvokeVerificationDirect(verifyTask, outMsgs)
	if verifyErr != nil {
		slog.Error("runtime.dispatch_tools.invoke_senior_dev.auto_verification.error", "error", verifyErr)
		return DispatchResult{
			Task:   taskText,
			Result: combineImplementationAndVerificationResult(implementationResult, "自动验收未完成: "+verifyErr.Error()),
		}
	}

	verificationResult := strings.TrimSpace(extractLastAssistantText(verifyMsgs))
	return DispatchResult{
		Task:   taskText,
		Result: combineImplementationAndVerificationResult(implementationResult, verificationResult),
	}
}

func (dt *DispatchTools) InvokeSeniorDevDirect(task string) ([]*schema.Message, error) {
	dt.mu.RLock()
	ctx := dt.currentCtx
	msgs := dt.currentMsgs
	dt.mu.RUnlock()

	if dt.seniorDevAgent == nil {
		return nil, fmt.Errorf("senior-dev agent not initialized")
	}

	reqID := tools.TraceIDFromContext(ctx)
	subCtx := dt.subAgentMgr.GetOrCreateRequestContext(reqID, SubAgentTypeSeniorDev, ctx, msgs)
	_ = dt.subAgentMgr.MarkRunning(subCtx.id, task, nil)
	dt.emitAgentEvent(EventAgentStarted, subCtx, task, task, nil)
	if strings.TrimSpace(task) != "" {
		_ = dt.subAgentMgr.AddMessage(subCtx.id, schema.UserMessage("任务补充: "+strings.TrimSpace(task)))
	}

	roleCtx := tools.WithRole(ctx, "senior-dev")
	outMsgs, err := invokeRoleAgentWithSubContext(roleCtx, msgs, "senior-dev", dt.seniorDevAgent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo+dt.getProjectStructurePrompt(roleCtx), subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		dt.subAgentMgr.Complete(subCtx.id, task, false, err.Error())
		dt.emitAgentEvent(EventAgentFailed, subCtx, task, err.Error(), map[string]any{"error": err.Error()})
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(ctx, subCtx.agentType.String(), false, err.Error(), time.Since(subCtx.createdAt).Milliseconds())
		}
		return nil, err
	}

	subRes := dt.subAgentMgr.Complete(subCtx.id, task, true, "")
	dt.emitAgentEvent(EventAgentCompleted, subCtx, task, dt.agentOutput(subCtx, subRes.Result), map[string]any{"duration_ms": subRes.Duration.Milliseconds()})
	if dt.hookMgr != nil {
		_, _ = dt.hookMgr.TeammateIdle(ctx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
	}
	return outMsgs, nil
}

func (dt *DispatchTools) InvokeVerificationDirect(task string, baseMsgs []*schema.Message) ([]*schema.Message, error) {
	dt.mu.RLock()
	ctx := dt.currentCtx
	dt.mu.RUnlock()

	if ctx == nil {
		ctx = context.Background()
	}
	if dt.verificationAgent == nil {
		return nil, fmt.Errorf("verification agent not initialized")
	}

	reqID := tools.TraceIDFromContext(ctx)
	subCtx := dt.subAgentMgr.GetOrCreateRequestContext(reqID, SubAgentTypeVerification, ctx, baseMsgs)
	_ = dt.subAgentMgr.MarkRunning(subCtx.id, task, nil)
	dt.emitAgentEvent(EventAgentStarted, subCtx, task, task, nil)
	if strings.TrimSpace(task) != "" {
		_ = dt.subAgentMgr.AddMessage(subCtx.id, schema.UserMessage("验证目标: "+strings.TrimSpace(task)))
	}

	roleCtx := tools.WithRole(ctx, "verification")
	outMsgs, err := invokeRoleAgentWithSubContext(roleCtx, baseMsgs, "verification", dt.verificationAgent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo, subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		dt.subAgentMgr.Complete(subCtx.id, task, false, err.Error())
		dt.emitAgentEvent(EventAgentFailed, subCtx, task, err.Error(), map[string]any{"error": err.Error()})
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(ctx, subCtx.agentType.String(), false, err.Error(), time.Since(subCtx.createdAt).Milliseconds())
		}
		return nil, err
	}

	subRes := dt.subAgentMgr.Complete(subCtx.id, task, true, "")
	dt.emitAgentEvent(EventAgentCompleted, subCtx, task, dt.agentOutput(subCtx, subRes.Result), map[string]any{"duration_ms": subRes.Duration.Milliseconds()})
	if dt.hookMgr != nil {
		_, _ = dt.hookMgr.TeammateIdle(ctx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
	}
	return outMsgs, nil
}

func shouldAutoVerifySeniorDevTask(task string) bool {
	normalized := strings.ToLower(strings.TrimSpace(task))
	if normalized == "" {
		return false
	}

	positiveKeywords := []string{
		"修复", "修正", "fix", "bug",
		"实现", "implement",
		"添加", "增加", "新增", "add",
		"删除", "移除", "remove",
		"修改", "调整", "优化", "update", "改为",
		"重构", "迁移", "refactor", "migrate",
		"支持", "处理",
		"warning", "lint", "报错", "错误", "异常", "编译失败",
		"验证", "验收", "测试",
	}
	negativeKeywords := []string{
		"分析", "建议", "解释", "说明", "梳理", "评估",
		"review", "审查", "对比", "总结", "介绍", "概述",
	}

	hasPositive := false
	for _, keyword := range positiveKeywords {
		if strings.Contains(normalized, keyword) {
			hasPositive = true
			break
		}
	}
	if hasPositive {
		return true
	}
	for _, keyword := range negativeKeywords {
		if strings.Contains(normalized, keyword) {
			return false
		}
	}
	return false
}

func combineImplementationAndVerificationResult(implementationResult string, verificationResult string) string {
	var sections []string
	if text := strings.TrimSpace(implementationResult); text != "" {
		sections = append(sections, "IMPLEMENTATION_RESULT:\n"+text)
	}
	if text := strings.TrimSpace(verificationResult); text != "" {
		sections = append(sections, "VERIFICATION_RESULT:\n"+text)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func (dt *DispatchTools) ClearRequest(requestID string) {
	if dt == nil || dt.subAgentMgr == nil {
		return
	}
	dt.subAgentMgr.ClearRequest(requestID)
}

func (dt *DispatchTools) getProjectStructurePrompt(ctx context.Context) string {
	dt.projectMu.Lock()
	defer dt.projectMu.Unlock()

	ttl := 10 * time.Minute
	if !dt.projectPromptAt.IsZero() && time.Since(dt.projectPromptAt) < ttl {
		return dt.projectPrompt
	}
	dt.projectPromptAt = time.Now()
	dt.projectPrompt = ""
	if dt.toolsManager == nil {
		return ""
	}
	res := dt.toolsManager.ExecuteStructured(ctx, []tools.ToolCall{
		{Tool: tools.ToolProjectStructure, Parameters: map[string]any{"path": "."}},
	})
	if len(res) != 1 {
		return ""
	}
	if strings.TrimSpace(res[0].Status) != "success" {
		return ""
	}
	raw, _ := res[0].Data["structure"].(string)
	raw = limitText(raw, 140, 9000)
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	dt.projectPrompt = "\n\n**项目结构（缓存）**：\n```text\n" + raw + "\n```"
	return dt.projectPrompt
}

func limitText(s string, maxLines int, maxChars int) string {
	if maxChars > 0 && len(s) > maxChars {
		s = s[:maxChars]
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if maxLines <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

// InvokeTester 调用测试工程师
func (dt *DispatchTools) InvokeTester(task DispatchTask) DispatchResult {
	return dt.invokeStandardAgent(task, "tester", SubAgentTypeTester, dt.testerAgent, dt.mcpToolsInfo, "tester agent not initialized")
}

// InvokeVerification 调用独立验证代理
func (dt *DispatchTools) InvokeVerification(task DispatchTask) DispatchResult {
	return dt.invokeStandardAgent(task, "verification", SubAgentTypeVerification, dt.verificationAgent, dt.mcpToolsInfo, "verification agent not initialized")
}

// InvokeReviewer 调用审核者
func (dt *DispatchTools) InvokeReviewer(task DispatchTask) DispatchResult {
	return dt.invokeStandardAgent(task, "reviewer", SubAgentTypeReviewer, dt.reviewerAgent, dt.mcpToolsInfo, "reviewer agent not initialized")
}

// GetDispatchToolsInfo 返回调度工具的信息，用于注册
func GetDispatchToolsInfo() []map[string]any {
	return []map[string]any{
		{
			"name":        "invoke_planner",
			"description": "调用规划师来制定详细的实施计划",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task": map[string]any{
						"type":        "string",
						"description": "任务目标与验收标准（避免指定具体命令行或逐条操作步骤）",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			"name":        "invoke_senior_dev",
			"description": "调用高级开发工程师来执行开发任务",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task": map[string]any{
						"type":        "string",
						"description": "任务目标与验收标准（避免指定具体命令行或逐条操作步骤）",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			"name":        "invoke_tester",
			"description": "调用测试工程师来执行测试任务",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task": map[string]any{
						"type":        "string",
						"description": "任务目标与验收标准（避免指定具体命令行或逐条操作步骤）",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			"name":        "invoke_verification",
			"description": "调用独立验证代理来做对抗式验收与风险核查",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task": map[string]any{
						"type":        "string",
						"description": "验证目标、关键路径、证据要求与验收标准",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			"name":        "invoke_reviewer",
			"description": "调用审核者来评估代码质量和任务完成度",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task": map[string]any{
						"type":        "string",
						"description": "任务目标与验收标准（避免指定具体命令行或逐条操作步骤）",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			"name":        "spawn_agent",
			"description": "创建并异步启动一个子代理任务，返回 agent_id。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent": map[string]any{
						"type":        "string",
						"description": "代理类型: planner, senior-dev, tester, reviewer, explorer",
					},
					"task": map[string]any{
						"type":        "string",
						"description": "任务目标与验收标准",
					},
					"fork_context": map[string]any{
						"type":        "boolean",
						"description": "是否继承当前上下文，默认 true",
					},
					"context_strategy": map[string]any{
						"type":        "string",
						"description": "可选: independent, shared, hybrid",
					},
					"allowed_tools": map[string]any{
						"type":        "array",
						"description": "可选：覆盖该子代理允许使用的工具列表",
					},
				},
				"required": []string{"agent", "task"},
			},
		},
		{
			"name":        "send_input",
			"description": "向已创建的子代理上下文追加输入，用于后续 resume。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"description": "子代理 ID",
					},
					"input": map[string]any{
						"type":        "string",
						"description": "要追加给子代理的新输入",
					},
				},
				"required": []string{"agent_id", "input"},
			},
		},
		{
			"name":        "wait_agent",
			"description": "等待子代理完成，支持超时返回当前状态。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"description": "子代理 ID",
					},
					"timeout_ms": map[string]any{
						"type":        "integer",
						"description": "可选：最长等待毫秒数",
					},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			"name":        "resume_agent",
			"description": "继续执行已完成或已暂停的子代理上下文。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"description": "子代理 ID",
					},
					"task": map[string]any{
						"type":        "string",
						"description": "可选：继续执行前追加的新任务说明",
					},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			"name":        "close_agent",
			"description": "关闭子代理；若仍在运行则先请求取消。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{
						"type":        "string",
						"description": "子代理 ID",
					},
				},
				"required": []string{"agent_id"},
			},
		},
	}
}

// GetToolRiskLevel 获取工具的风险等级
func GetToolRiskLevel(toolName string) ToolRiskLevel {
	// 调度工具包装的子agent调用，免检
	if toolName == "invoke_planner" || toolName == "invoke_senior_dev" ||
		toolName == "invoke_tester" || toolName == "invoke_verification" || toolName == "invoke_reviewer" ||
		toolName == "spawn_agent" || toolName == "send_input" ||
		toolName == "wait_agent" || toolName == "resume_agent" ||
		toolName == "close_agent" {
		return ToolRiskLow
	}

	// 使用集中定义的风险等级
	risk := tools.GetToolRiskLevel(toolName)
	switch risk {
	case tools.RiskLevelLow:
		return ToolRiskLow
	case tools.RiskLevelMedium:
		return ToolRiskMedium
	case tools.RiskLevelHigh:
		return ToolRiskHigh
	default:
		return ToolRiskHigh
	}
}

// invokeRoleAgentWithSubContext 调用角色 Agent，使用子代理隔离上下文
func invokeRoleAgentWithSubContext(
	ctx context.Context,
	msgs []*schema.Message,
	role string,
	agent *react.Agent,
	_ func(string),
	onReasoning func(string),
	mcpToolsInfo string,
	subCtx *SubAgentContext,
	mgr *SubAgentManager,
	hookMgr *HookManager,
) ([]*schema.Message, error) {
	if agent == nil {
		return nil, fmt.Errorf("agent is nil")
	}

	// 构建系统提示词
	systemPrompt := buildRoleSystemPrompt(ctx, role, mcpToolsInfo)

	// 使用子代理的隔离上下文
	subCtx.mu.RLock()
	subMsgs := subCtx.messages
	subCtx.mu.RUnlock()

	// 添加系统提示词到子代理消息
	agentMsgs := make([]*schema.Message, 0, len(subMsgs)+1)
	agentMsgs = append(agentMsgs, schema.SystemMessage(systemPrompt))
	agentMsgs = append(agentMsgs, subMsgs...)

	slog.Debug("runtime.subagent.invoke", "role", role, "id", subCtx.id, "messages", len(agentMsgs))

	maxRounds := 3
	stopActive := false
	var out *schema.Message
	var err error
	for range maxRounds {
		out, err = agent.Generate(ctx, agentMsgs)
		if err != nil {
			return nil, wrapMaxStepError(err)
		}

		if hookMgr == nil {
			break
		}
		dec, _ := hookMgr.SubagentStop(ctx, role, out.Content, stopActive)
		if strings.TrimSpace(dec.AdditionalContext) != "" {
			_ = mgr.AddMessage(subCtx.id, schema.SystemMessage(strings.TrimSpace(dec.AdditionalContext)))
			agentMsgs = append(agentMsgs, schema.SystemMessage(strings.TrimSpace(dec.AdditionalContext)))
		}
		if strings.EqualFold(dec.Decision, "block") {
			reason := strings.TrimSpace(dec.Reason)
			if reason == "" {
				reason = "SubagentStop hook blocked stopping"
			}
			_ = mgr.AddMessage(subCtx.id, out)
			agentMsgs = append(agentMsgs, out)
			agentMsgs = append(agentMsgs, schema.SystemMessage("SUBAGENT_STOP_HOOK: "+reason))
			stopActive = true
			continue
		}
		break
	}

	// 处理推理内容
	if onReasoning != nil && out.ReasoningContent != "" {
		onReasoning(out.ReasoningContent)
	}

	// 将输出消息添加到子代理上下文
	_ = mgr.AddMessage(subCtx.id, out)
	subCtx.mu.Lock()
	subCtx.result = strings.TrimSpace(out.Content)
	subCtx.mu.Unlock()

	return append(msgs, out), nil
}

func (dt *DispatchTools) emitAgentEvent(eventType string, subCtx *SubAgentContext, task string, message string, extra map[string]any) {
	if dt == nil || dt.onMeta == nil || subCtx == nil {
		return
	}
	payload := map[string]any{
		"agent_id":         strings.TrimSpace(subCtx.id),
		"agent_name":       subCtx.agentType.String(),
		"context_strategy": subCtx.strategy.String(),
		"task":             strings.TrimSpace(task),
	}
	if msg := strings.TrimSpace(message); msg != "" {
		payload["message"] = msg
	}
	if len(subCtx.allowedTools) > 0 {
		payload["allowed_tools"] = append([]string(nil), subCtx.allowedTools...)
	}
	maps.Copy(payload, extra)
	raw, err := json.Marshal(EventData{
		Type:    eventType,
		ID:      strings.TrimSpace(subCtx.id),
		Content: strings.TrimSpace(message),
		Data:    payload,
	})
	if err != nil {
		slog.Warn("runtime.dispatch_tools.emit_agent_event.marshal_failed", "event_type", eventType, "error", err)
		return
	}
	dt.onMeta(string(raw))
}

func (dt *DispatchTools) agentOutput(subCtx *SubAgentContext, fallback string) string {
	if subCtx == nil {
		return strings.TrimSpace(fallback)
	}
	subCtx.mu.RLock()
	defer subCtx.mu.RUnlock()
	if strings.TrimSpace(subCtx.result) != "" {
		return strings.TrimSpace(subCtx.result)
	}
	return strings.TrimSpace(fallback)
}

// buildRoleSystemPrompt 构建角色特定的系统提示词
func buildRoleSystemPrompt(ctx context.Context, role, mcpToolsInfo string) string {
	var prompt string
	switch role {
	case "planner":
		prompt = BuildPlanPromptForStyle(planPromptStyleFromContext(ctx))
		if mcpToolsInfo != "" {
			prompt += "\n\n" + mcpToolsInfo
		}
	case "senior-dev":
		prompt = RoleSeniorDevPrompt
		if mcpToolsInfo != "" {
			prompt += "\n\n" + mcpToolsInfo
		}
	case "tester":
		prompt = RoleTesterPrompt
	case "verification":
		prompt = RoleVerificationPrompt
	case "reviewer":
		prompt = RoleReviewerPrompt
	default:
		prompt = RoleDefaultPrompt
	}

	// 注入环境信息
	envInfo := envInfoForContext(ctx)
	prompt += "\n\n" + utils.FormatEnvInfo(envInfo)

	return prompt
}
