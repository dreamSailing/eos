package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"github.com/dreamSailing/vb-coding/internal/tools"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"

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
	Task   string `json:"task"`   // 传递给子 Agent 的任务
	Result string `json:"result"` // 子 Agent 的返回内容（自然语言）
}

// DispatchTools 包装器，将子 Agent 包装为调度工具
type DispatchTools struct {
	plannerAgent   *react.Agent
	seniorDevAgent *react.Agent
	testerAgent    *react.Agent
	reviewerAgent  *react.Agent
	toolsManager   *tools.Manager
	currentCtx     context.Context
	currentMsgs    []*schema.Message
	onMeta         func(string)
	onReasoning    func(string)     // 思考内容回调
	loopDetector   LoopDetector     // 循环检测器 (供 Exec Agent 使用)
	mcpToolsInfo   string           // MCP 工具信息字符串，用于传递给子 Agent
	mcpTools       []tool.BaseTool
	subAgentMgr    *SubAgentManager // 子代理管理器
	allowedToolsOverride map[string]bool
	hookMgr       *HookManager
	projectMu      sync.Mutex
	projectPrompt  string
	projectPromptAt time.Time
	mu             sync.RWMutex     // 保护并发访问
}

// NewDispatchTools 创建调度工具包装器
func NewDispatchTools(
	ctx context.Context,
	planner, seniorDev, tester, reviewer *react.Agent,
	tm *tools.Manager,
	onMeta func(string),
	detector LoopDetector,
	mcpToolsInfo string,
	mcpTools []tool.BaseTool,
) *DispatchTools {
	hm := NewHookManager(tm)
	_ = hm.LoadFromDefaultLocations()
	onMetaOrig := onMeta
	onMetaWrapped := func(line string) {
		if hm != nil {
			if strings.HasPrefix(line, "phase.note:") {
				msg := strings.TrimPrefix(line, "phase.note:")
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
		plannerAgent:   planner,
		seniorDevAgent: seniorDev,
		testerAgent:    tester,
		reviewerAgent:  reviewer,
		toolsManager:   tm,
		currentCtx:     ctx,
		onMeta:         onMetaWrapped,
		loopDetector:   detector,
		mcpToolsInfo:   mcpToolsInfo,
		mcpTools:       mcpTools,
		subAgentMgr:    NewSubAgentManager(),
		hookMgr:        hm,
	}
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

// InvokePlanner 调用规划师
func (dt *DispatchTools) InvokePlanner(task DispatchTask) DispatchResult {
	dt.mu.RLock()
	ctx := dt.currentCtx
	msgs := dt.currentMsgs
	dt.mu.RUnlock()

	// 发送任务分配事件，在UI上显示Agent调用信息
	if dt.onMeta != nil {
		dt.onMeta(EventAgentTask + ":planner " + task.Task)
	}

	if dt.plannerAgent == nil {
		return DispatchResult{
			Task:   task.Task,
			Result: "planner agent not initialized",
		}
	}

	// 创建子代理隔离上下文
	reqID := tools.TraceIDFromContext(ctx)
	subCtx := dt.subAgentMgr.GetOrCreateRequestContext(reqID, SubAgentTypePlanner, ctx, msgs)

	// 使用 Planner 角色调用
	roleCtx := tools.WithRole(ctx, "planner")
	_, err := invokeRoleAgentWithSubContext(roleCtx, msgs, "planner", dt.plannerAgent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo, subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		// 标记失败并记录结果
		dt.subAgentMgr.Complete(subCtx.id, task.Task, false, err.Error())
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(ctx, subCtx.agentType.String(), false, err.Error(), time.Since(subCtx.createdAt).Milliseconds())
		}
		return DispatchResult{
			Task:   task.Task,
			Result: err.Error(),
		}
	}

	// 标记完成并记录结果摘要
	subRes := dt.subAgentMgr.Complete(subCtx.id, task.Task, true, "")
	if dt.hookMgr != nil {
		_, _ = dt.hookMgr.TeammateIdle(ctx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
	}

	return DispatchResult{
		Task:   task.Task,
		Result: subRes.Result, // 使用摘要而非完整输出
	}
}

// InvokeSeniorDev 调用高级开发工程师
func (dt *DispatchTools) InvokeSeniorDev(task DispatchTask) DispatchResult {
	dt.mu.RLock()
	ctx := dt.currentCtx
	msgs := dt.currentMsgs
	dt.mu.RUnlock()

	// 发送任务分配事件，在UI上显示Agent调用信息
	if dt.onMeta != nil {
		dt.onMeta(EventAgentTask + ":senior-dev " + task.Task)
	}

	slog.Debug("runtime.dispatch_tools.invoke_senior_dev.start", "task", task.Task, "task_empty", task.Task == "")

	if dt.seniorDevAgent == nil {
		return DispatchResult{
			Task:   task.Task,
			Result: "senior-dev agent not initialized",
		}
	}

	slog.Debug("runtime.dispatch_tools.invoke_senior_dev.calling", "task", task.Task, "context_messages", len(msgs))

	// 创建子代理隔离上下文
	reqID := tools.TraceIDFromContext(ctx)
	subCtx := dt.subAgentMgr.GetOrCreateRequestContext(reqID, SubAgentTypeSeniorDev, ctx, msgs)

	// 使用 SeniorDev 角色调用，确保具有 bash 等工具的权限
	roleCtx := tools.WithRole(ctx, "senior-dev")
	_, err := invokeRoleAgentWithSubContext(roleCtx, msgs, "senior-dev", dt.seniorDevAgent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo+dt.getProjectStructurePrompt(roleCtx), subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		slog.Error("runtime.dispatch_tools.invoke_senior_dev.error", "error", err)
		dt.subAgentMgr.Complete(subCtx.id, task.Task, false, err.Error())
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(ctx, subCtx.agentType.String(), false, err.Error(), time.Since(subCtx.createdAt).Milliseconds())
		}
		return DispatchResult{
			Task:   task.Task,
			Result: err.Error(),
		}
	}

	// 标记完成并记录结果摘要
	subRes := dt.subAgentMgr.Complete(subCtx.id, task.Task, true, "")
	if dt.hookMgr != nil {
		_, _ = dt.hookMgr.TeammateIdle(ctx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
	}

	return DispatchResult{
		Task:   task.Task,
		Result: subRes.Result, // 使用摘要而非完整输出
	}
}

func (dt *DispatchTools) InvokeSeniorDevDirect(task string) ([]*schema.Message, error) {
	dt.mu.RLock()
	ctx := dt.currentCtx
	msgs := dt.currentMsgs
	dt.mu.RUnlock()

	if dt.onMeta != nil {
		dt.onMeta(EventAgentTask + ":senior-dev " + task)
	}

	if dt.seniorDevAgent == nil {
		return nil, fmt.Errorf("senior-dev agent not initialized")
	}

	reqID := tools.TraceIDFromContext(ctx)
	subCtx := dt.subAgentMgr.GetOrCreateRequestContext(reqID, SubAgentTypeSeniorDev, ctx, msgs)
	if strings.TrimSpace(task) != "" {
		_ = dt.subAgentMgr.AddMessage(subCtx.id, schema.UserMessage("任务补充: "+strings.TrimSpace(task)))
	}

	roleCtx := tools.WithRole(ctx, "senior-dev")
	outMsgs, err := invokeRoleAgentWithSubContext(roleCtx, msgs, "senior-dev", dt.seniorDevAgent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo+dt.getProjectStructurePrompt(roleCtx), subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		dt.subAgentMgr.Complete(subCtx.id, task, false, err.Error())
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(ctx, subCtx.agentType.String(), false, err.Error(), time.Since(subCtx.createdAt).Milliseconds())
		}
		return nil, err
	}

	subRes := dt.subAgentMgr.Complete(subCtx.id, task, true, "")
	if dt.hookMgr != nil {
		_, _ = dt.hookMgr.TeammateIdle(ctx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
	}
	return outMsgs, nil
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
		{Tool: tools.ToolProjectStructure, Parameters: map[string]interface{}{"path": "."}},
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
	dt.mu.RLock()
	ctx := dt.currentCtx
	msgs := dt.currentMsgs
	dt.mu.RUnlock()

	if dt.onMeta != nil {
		dt.onMeta(EventAgentTask + ":tester " + task.Task)
	}
	if dt.testerAgent == nil {
		return DispatchResult{
			Task:   task.Task,
			Result: "tester agent not initialized",
		}
	}

	// 创建子代理隔离上下文
	reqID := tools.TraceIDFromContext(ctx)
	subCtx := dt.subAgentMgr.GetOrCreateRequestContext(reqID, SubAgentTypeTester, ctx, msgs)

	// 使用 Tester 角色调用
	roleCtx := tools.WithRole(ctx, "tester")
	_, err := invokeRoleAgentWithSubContext(roleCtx, msgs, "tester", dt.testerAgent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo, subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		dt.subAgentMgr.Complete(subCtx.id, task.Task, false, err.Error())
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(ctx, subCtx.agentType.String(), false, err.Error(), time.Since(subCtx.createdAt).Milliseconds())
		}
		return DispatchResult{
			Task:   task.Task,
			Result: err.Error(),
		}
	}

	// 标记完成并记录结果摘要
	subRes := dt.subAgentMgr.Complete(subCtx.id, task.Task, true, "")
	if dt.hookMgr != nil {
		_, _ = dt.hookMgr.TeammateIdle(ctx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
	}

	return DispatchResult{
		Task:   task.Task,
		Result: subRes.Result,
	}
}

// InvokeReviewer 调用审核者
func (dt *DispatchTools) InvokeReviewer(task DispatchTask) DispatchResult {
	dt.mu.RLock()
	ctx := dt.currentCtx
	msgs := dt.currentMsgs
	dt.mu.RUnlock()

	if dt.onMeta != nil {
		dt.onMeta(EventAgentTask + ":reviewer " + task.Task)
	}
	if dt.reviewerAgent == nil {
		return DispatchResult{
			Task:   task.Task,
			Result: "reviewer agent not initialized",
		}
	}

	// 创建子代理隔离上下文
	reqID := tools.TraceIDFromContext(ctx)
	subCtx := dt.subAgentMgr.GetOrCreateRequestContext(reqID, SubAgentTypeReviewer, ctx, msgs)

	// 使用 Reviewer 角色调用
	roleCtx := tools.WithRole(ctx, "reviewer")
	_, err := invokeRoleAgentWithSubContext(roleCtx, msgs, "reviewer", dt.reviewerAgent, dt.onMeta, dt.onReasoning, dt.mcpToolsInfo, subCtx, dt.subAgentMgr, dt.hookMgr)
	if err != nil {
		dt.subAgentMgr.Complete(subCtx.id, task.Task, false, err.Error())
		if dt.hookMgr != nil {
			_, _ = dt.hookMgr.TeammateIdle(ctx, subCtx.agentType.String(), false, err.Error(), time.Since(subCtx.createdAt).Milliseconds())
		}
		return DispatchResult{
			Task:   task.Task,
			Result: err.Error(),
		}
	}

	// 标记完成并记录结果摘要
	subRes := dt.subAgentMgr.Complete(subCtx.id, task.Task, true, "")
	if dt.hookMgr != nil {
		_, _ = dt.hookMgr.TeammateIdle(ctx, subRes.Type.String(), true, "", subRes.Duration.Milliseconds())
	}

	return DispatchResult{
		Task:   task.Task,
		Result: subRes.Result,
	}
}

// GetDispatchToolsInfo 返回调度工具的信息，用于注册
func GetDispatchToolsInfo() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "invoke_planner",
			"description": "调用规划师来制定详细的实施计划",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
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
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
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
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "任务目标与验收标准（避免指定具体命令行或逐条操作步骤）",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			"name":        "invoke_reviewer",
			"description": "调用审核者来评估代码质量和任务完成度",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task": map[string]interface{}{
						"type":        "string",
						"description": "任务目标与验收标准（避免指定具体命令行或逐条操作步骤）",
					},
				},
				"required": []string{"task"},
			},
		},
	}
}

// GetToolRiskLevel 获取工具的风险等级
func GetToolRiskLevel(toolName string) ToolRiskLevel {
	// 调度工具包装的子agent调用，免检
	if toolName == "invoke_planner" || toolName == "invoke_senior_dev" ||
		toolName == "invoke_tester" || toolName == "invoke_reviewer" {
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
	onMeta func(string),
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
	systemPrompt := buildRoleSystemPrompt(role, mcpToolsInfo)

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
	for round := 0; round < maxRounds; round++ {
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

	// 发送 agent.final 事件，使子 Agent 输出显示为白色圆点
	if onMeta != nil && out.Content != "" {
		onMeta(EventAgentFinal + ":" + out.Content)
	}

	// 将输出消息添加到子代理上下文
	mgr.AddMessage(subCtx.id, out)

	return append(msgs, out), nil
}

// buildRoleSystemPrompt 构建角色特定的系统提示词
func buildRoleSystemPrompt(role, mcpToolsInfo string) string {
	var prompt string
	switch role {
	case "planner":
		prompt = PlanPrompt
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
	case "reviewer":
		prompt = RoleReviewerPrompt
	default:
		prompt = RoleDefaultPrompt
	}

	// 注入环境信息
	envInfo := utils.GetEnvInfo()
	prompt += "\n\n" + utils.FormatEnvInfo(envInfo)

	return prompt
}
