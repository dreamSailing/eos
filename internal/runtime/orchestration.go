package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"errors"
	"fmt"
	ai "github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/pkg/utils"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

type SafetyGate struct {
	Classify        func(call tools.ToolCall) (category string, level string, summary string, dangerous bool)
	Prompt          func(ctx context.Context, category string, summary string) string
	SessionAllowed  func(category string) bool
	AllowSession    func(category string)
	SetPendingDiff  func(diff string)
	ClearReviewText func()
}

type AIModel interface {
	Chat(ctx context.Context, messages []ai.Message) (string, error)
	ChatStream(ctx context.Context, messages []ai.Message, onDelta func(string), onReasoning func(string)) (string, error)
}

type EinoRuntime struct {
	runnable              compose.Runnable[map[string]any, *schema.Message]
	ctxm                  *session.ContextManager
	tools                 *tools.Manager
	model                 AIModel
	safety                SafetyGate
	allowedTools          map[string]bool
	toolTimeout           time.Duration
	roundTimeout          time.Duration
	recentToolCalls       map[string]int
	recentAssistantHashes map[string]int
	onDelta               func(string)
	onMeta                func(string)
	onPlanUpdate          func(string)
	onReasoning           func(string) // 思考内容回调
	dispatchTools         *DispatchTools
	tokenAnalyzer         *TokenAnalyzer             // Token 使用分析器
	loopDetector          *SlidingWindowLoopDetector // 滑动窗口循环检测器
	sessionStarted        bool
}

// NewEinoRuntime 创建 EinoRuntime（不带 MCP 工具）
func NewEinoRuntime(ctx context.Context, cm *session.ContextManager, tm *tools.Manager, c AIModel) (*EinoRuntime, error) {
	return NewEinoRuntimeWithMCP(ctx, cm, tm, c, nil)
}

// NewEinoRuntimeWithMCP 创建带有 MCP 工具支持的 EinoRuntime
func NewEinoRuntimeWithMCP(ctx context.Context, cm *session.ContextManager, tm *tools.Manager, c AIModel, mcpTools []tool.BaseTool) (*EinoRuntime, error) {
	// Initialize EinoRuntime first to setup proxy
	rt := &EinoRuntime{
		ctxm:                  cm,
		tools:                 tm,
		model:                 c,
		allowedTools:          nil,
		toolTimeout:           60 * time.Second,
		roundTimeout:          120 * time.Second,
		recentToolCalls:       map[string]int{},
		recentAssistantHashes: map[string]int{},
		tokenAnalyzer:         NewTokenAnalyzer("session"),
		loopDetector:          NewSlidingWindowLoopDetector(),
	}

	onMetaProxy := func(s string) {
		if rt.onMeta != nil {
			rt.onMeta(s)
		}
	}

	LogDebug("runtime.new_graph.start", nil)
	rg, err := newRuntimeGraph(ctx, tm, c, onMetaProxy, rt, mcpTools)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime graph: %w", err)
	}
	LogDebug("runtime.new_graph.success", nil)
	rt.runnable = rg

	rt.bindPlanUpdates(cm)
	return rt, nil
}

func (rt *EinoRuntime) bindPlanUpdates(cm *session.ContextManager) {
	if rt == nil || cm == nil {
		return
	}
	cm.SetOnPlanUpdate(func(plan string) {
		if rt.onPlanUpdate != nil {
			rt.onPlanUpdate(plan)
		}
		if rt.onMeta != nil {
			rt.onMeta(EventPlanReady)
		}
	})
}

func (rt *EinoRuntime) WithSafety(h SafetyGate) *EinoRuntime {
	rt.safety = h
	tools.SafetyGatePrompt = h.Prompt
	tools.SafetyGateSessionAllowed = h.SessionAllowed
	tools.SafetyGateAllowSession = h.AllowSession
	tools.SafetyGateClassify = h.Classify
	tools.SetPendingDiff = h.SetPendingDiff
	tools.ClearReviewText = h.ClearReviewText

	tools.ObservationConsumer = func(res tools.ToolResult) {
		if res.Tool == "read_file" && res.Status == "success" {
			if v, ok := res.Data["content"].(string); ok {
				rt.ctxm.AddToolFull(v)
			}
		}
		rt.ctxm.AddToolObservation(res)
	}
	return rt
}

func (rt *EinoRuntime) WithOnDelta(cb func(string)) *EinoRuntime {
	rt.onDelta = cb
	return rt
}

func (rt *EinoRuntime) WithOnPlanUpdate(cb func(string)) *EinoRuntime {
	rt.onPlanUpdate = cb
	return rt
}

func (rt *EinoRuntime) WithOnMeta(cb func(string)) *EinoRuntime {
	rt.onMeta = cb
	return rt
}

func (rt *EinoRuntime) WithOnReasoning(cb func(string)) *EinoRuntime {
	rt.onReasoning = cb
	return rt
}

func (rt *EinoRuntime) ModelName() string {
	if rt.model == nil {
		return ""
	}
	type named interface{ Name() string }
	if n, ok := rt.model.(named); ok {
		return n.Name()
	}
	return ""
}

func (rt *EinoRuntime) ModelBase() string {
	if rt.model == nil {
		return ""
	}
	type based interface{ Base() string }
	if b, ok := rt.model.(based); ok {
		return b.Base()
	}
	return ""
}

func (rt *EinoRuntime) ClearRequestContexts(requestID string) {
	if rt == nil || rt.dispatchTools == nil {
		return
	}
	rt.dispatchTools.ClearRequest(requestID)
}

func (rt *EinoRuntime) GraphInvoke(ctx context.Context, query string, executionMode string) (*schema.Message, error) {
	return rt.GraphInvokeWithImages(ctx, query, executionMode, nil)
}

func (rt *EinoRuntime) GraphInvokeWithImages(ctx context.Context, query string, executionMode string, imagePaths []string) (*schema.Message, error) {
	if rt.runnable == nil {
		return nil, fmt.Errorf("graph not initialized")
	}
	if !rt.sessionStarted && rt.dispatchTools != nil && rt.dispatchTools.hookMgr != nil {
		modelName := ""
		if rt.model != nil {
			type named interface{ Name() string }
			if n, ok := rt.model.(named); ok {
				modelName = n.Name()
			}
		}
		dec, _ := rt.dispatchTools.hookMgr.SessionStart(ctx, "startup", modelName, "")
		if strings.TrimSpace(dec.AdditionalContext) != "" && rt.ctxm != nil {
			rt.ctxm.AddEphemeral(strings.TrimSpace(dec.AdditionalContext))
		}
		rt.sessionStarted = true
	}
	if rt.dispatchTools != nil && rt.dispatchTools.hookMgr != nil && strings.TrimSpace(query) != "" {
		dec, _ := rt.dispatchTools.hookMgr.UserPromptSubmit(ctx, query)
		if strings.EqualFold(dec.Decision, "block") {
			reason := strings.TrimSpace(dec.Reason)
			if reason == "" {
				reason = "blocked by UserPromptSubmit hook"
			}
			return nil, errors.New(reason)
		}
		if strings.TrimSpace(dec.AdditionalContext) != "" && rt.ctxm != nil {
			rt.ctxm.AddEphemeral(strings.TrimSpace(dec.AdditionalContext))
		}
	}

	snippet := strings.TrimSpace(query)
	if len(snippet) > 120 {
		snippet = snippet[:120] + "…"
	}
	effPlan := strings.ToLower(strings.TrimSpace(executionMode))
	if effPlan == "" {
		effPlan = "auto"
	}
	LogDebug("runtime.graph.invoke",
		"query_len", len(strings.TrimSpace(query)),
		"execution_mode", effPlan,
		"query_snippet", snippet,
		"images_count", len(imagePaths))
	var extra []ai.Message
	if rt.ctxm != nil {
		if p := strings.TrimSpace(rt.ctxm.LastPlan()); p != "" {
			rt.ctxm.AddEphemeral("CURRENT_PLAN:\n" + p)
		}
	}
	if strings.TrimSpace(query) != "" || len(imagePaths) > 0 {
		extra = append(extra, ai.Message{Role: "user", Content: query, ImagePaths: imagePaths})
	}
	history := buildHistoryMessages(rt.ctxm, extra, tools.WorkspaceRootFromContext(ctx))
	in := map[string]any{
		"history":   history,
		"query":     query,
		"plan_pref": effPlan,
	}

	var out *schema.Message
	var err error
	maxStopRounds := 3
	stopActive := false
	for round := 0; round < maxStopRounds; round++ {
		out, err = rt.runnable.Invoke(ctx, in)
		if err != nil {
			LogError("runtime.graph_invoke.error",
				"query", query,
				"history_len", len(history),
				"error", err)
			return nil, err
		}

		if rt.dispatchTools == nil || rt.dispatchTools.hookMgr == nil {
			break
		}
		dec, _ := rt.dispatchTools.hookMgr.Stop(ctx, out.Content, stopActive)
		if strings.TrimSpace(dec.AdditionalContext) != "" {
			history = append(history, schema.SystemMessage(strings.TrimSpace(dec.AdditionalContext)))
		}
		if strings.EqualFold(dec.Decision, "block") {
			reason := strings.TrimSpace(dec.Reason)
			if reason == "" {
				reason = "Stop hook blocked stopping"
			}
			history = append(history, schema.AssistantMessage(strings.TrimSpace(out.Content), nil))
			history = append(history, schema.SystemMessage("STOP_HOOK: "+reason))
			in["history"] = history
			in["query"] = ""
			stopActive = true
			continue
		}
		break
	}

	LogDebug("runtime.graph_invoke.success",
		"query", query,
		"history_len", len(history),
		"response_length", len(out.Content))
	rt.emitAssistantDelta(out)
	return out, nil
}

func (rt *EinoRuntime) emitAssistantDelta(out *schema.Message) {
	if out == nil {
		return
	}
	text := strings.TrimSpace(out.Content)
	if text == "" {
		return
	}
	// 主 Agent 的最终结果不需要通过 emitAssistantDelta 发送 meta 事件
	// 因为它会由 UI 层的 assistant_final 事件统一处理
	// 这里只保留 onDelta 用于可能的流式回调（如果有的话）
	if rt.onDelta != nil {
		rt.onDelta(text)
	}
}

// newRuntimeGraph 创建简化的运行时图，使用调度工具而非分支
func newRuntimeGraph(ctx context.Context, tm *tools.Manager, mdl AIModel, onMeta func(string), rt *EinoRuntime, mcpTools []tool.BaseTool) (compose.Runnable[map[string]any, *schema.Message], error) {
	if rt == nil {
		return nil, fmt.Errorf("runtime is nil")
	}
	if tm == nil {
		return nil, fmt.Errorf("tools manager is nil")
	}
	if _, ok := mdl.(ToolCallingProvider); !ok {
		return nil, fmt.Errorf("model does not implement ToolCallingProvider")
	}

	// 预加载执行阶段所需的模板
	LogDebug("runtime.new_agents.create_template", nil)
	agentTpl, err := NewAgentChatTemplate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent chat template: %w", err)
	}

	// 预生成 MCP 工具信息字符串
	mcpToolsInfo := ""
	if len(mcpTools) > 0 {
		mcpToolsInfo = "**可用的 MCP 工具**：\n以下是来自已配置 MCP 服务器的工具，工具名称是第一列（非服务器名称），可以直接调用：\n"
		for _, t := range mcpTools {
			info, _ := t.Info(ctx)
			if info != nil {
				mcpToolsInfo += fmt.Sprintf("- 工具名: `%s` - 描述: %s\n", info.Name, info.Desc)
			}
		}
		mcpToolsInfo += "\n注意：调用工具时使用工具名（如上所示的 `工具名` 列），不是 MCP 服务器的名称。"
	}

	// 创建调度工具包装器 (先创建，此时 Agent 还是 nil)
	LogDebug("runtime.new_agents.create_dispatch_tools", nil)
	dispatchTools := NewDispatchTools(ctx, nil, nil, nil, nil, tm, onMeta, rt.loopDetector, mcpToolsInfo, mcpTools)
	rt.dispatchTools = dispatchTools
	if rt.ctxm != nil && dispatchTools != nil && dispatchTools.hookMgr != nil {
		rt.ctxm.SetOnPreCompact(func(trigger string, customInstructions string) {
			dec, _ := dispatchTools.hookMgr.PreCompact(context.Background(), trigger, customInstructions)
			if strings.TrimSpace(dec.AdditionalContext) != "" {
				rt.ctxm.AddEphemeral(strings.TrimSpace(dec.AdditionalContext))
			}
		})
	}

	// 创建所有 Agent，并将调度工具和 MCP 工具传入
	LogDebug("runtime.new_agents.create_agents_start", nil)
	execAgent, dispatchAgent, planAgent, reviewAgent, testAgent, err := newRuntimeAgentsWithDispatchTools(ctx, tm, mdl, dispatchTools, mcpTools, onMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime agents: %w", err)
	}
	LogDebug("runtime.new_agents.create_agents_success", nil)

	// 更新调度工具中的 Agent 引用
	dispatchTools.plannerAgent = planAgent
	dispatchTools.seniorDevAgent = execAgent
	dispatchTools.testerAgent = testAgent
	dispatchTools.reviewerAgent = reviewAgent

	g := compose.NewGraph[map[string]any, *schema.Message]()
	const (
		nodePlanPreference = "PlanPreferenceLambda"
		nodeAgentTemplate  = "AgentChatTemplate"
		nodeDispatch       = "DispatchLambda"
		nodeConverter      = "LambdaConverter"
	)

	// PlanPreference 节点
	planPref := compose.InvokableLambda(func(ctx context.Context, in map[string]any) (map[string]any, error) {
		pref, _ := in["plan_pref"].(string)
		prefNorm := strings.ToLower(strings.TrimSpace(pref))
		if prefNorm == "" || prefNorm == "auto" {
			return in, nil
		}
		history, _ := in["history"].([]*schema.Message)
		if history == nil {
			history = []*schema.Message{}
		}
		text := ""
		switch prefNorm {
		case "plan":
			text = "PLAN_PREFERENCE: prefer_plan_first\nPLAN_MODE: force"
		case "on":
			text = "PLAN_PREFERENCE: prefer_plan_first"
		case "off":
			text = "PLAN_PREFERENCE: prefer_no_plan"
		}
		if strings.TrimSpace(text) != "" {
			history = append(history, schema.SystemMessage(text))
			in["history"] = history
		}
		return in, nil
	})
	_ = g.AddLambdaNode(nodePlanPreference, planPref)
	_ = g.AddChatTemplateNode(nodeAgentTemplate, agentTpl)

	// Dispatch 节点 - 直接调用调度 Agent，不需要分支
	dispatch := compose.InvokableLambda(func(ctx context.Context, in []*schema.Message) ([]*schema.Message, error) {
		dispatchTools.SetContext(ctx, in)

		if hasPlanModeForce(in) {
			q := strings.TrimSpace(extractLastUserText(in))
			if q == "" {
				q = "（空）"
			}
			planTask := "为以下需求先制定可执行实施计划（包含受影响文件线索与验证方案），然后再进入实现执行。\n需求: " + q
			planRes := dispatchTools.InvokePlanner(DispatchTask{Task: planTask})
			devTask := "按需求实现并验证。\n需求: " + q
			if strings.TrimSpace(planRes.Result) != "" {
				devTask += "\n\n计划摘要: " + strings.TrimSpace(planRes.Result)
			}
			return dispatchTools.InvokeSeniorDevDirect(devTask)
		}

		if shouldBypassArchitect(in) {
			task := buildSeniorDevTaskHint(in)
			return dispatchTools.InvokeSeniorDevDirect(task)
		}

		return invokeDispatchAgentWithTools(ctx, in, dispatchAgent, onMeta, rt.onReasoning, rt, mcpTools)
	})
	_ = g.AddLambdaNode(nodeDispatch, dispatch)

	// Converter 节点
	lambda := compose.InvokableLambda(converterNode)
	_ = g.AddLambdaNode(nodeConverter, lambda)

	// 简化的连接：START -> PlanPreference -> Template -> Dispatch -> END
	_ = g.AddEdge(compose.START, nodePlanPreference)
	_ = g.AddEdge(nodePlanPreference, nodeAgentTemplate)
	_ = g.AddEdge(nodeAgentTemplate, nodeDispatch)
	_ = g.AddEdge(nodeDispatch, nodeConverter)
	_ = g.AddEdge(nodeConverter, compose.END)

	// 编译图
	r, err := g.Compile(ctx, compose.WithGraphName("simplified_runtime_graph"))
	if err != nil {
		return nil, fmt.Errorf("failed to compile runtime graph: %w", err)
	}
	return r, nil
}

// invokeDispatchAgentWithTools 调用带有调度工具的调度 Agent
// rt 用于获取模型名称，mcpTools 用于生成工具能力描述
func invokeDispatchAgentWithTools(ctx context.Context, in []*schema.Message, dispatchAgent *react.Agent, onMeta func(string), onReasoning func(string), rt *EinoRuntime, mcpTools []tool.BaseTool) ([]*schema.Message, error) {
	if dispatchAgent == nil {
		return in, nil
	}

	if onMeta != nil {
		onMeta("architect_start")
	}

	slog.Debug("runtime.dispatch_agent.invoke.start", "input_messages_count", len(in))

	// 构建系统提示：替换占位符并注入上下文信息
	systemPrompt, history := normalizeDispatchHistory(systemPromptWithContext(ctx, rt, mcpTools, in), in)
	msgs := append([]*schema.Message{schema.SystemMessage(systemPrompt)}, history...)

	slog.Debug("runtime.dispatch_agent.invoke.generating", "system_prompt_len", len(systemPrompt))

	out, err := dispatchAgent.Generate(ctx, msgs)
	if err != nil {
		slog.Error("runtime.dispatch_agent.generate.error", "error", err)
		return nil, wrapMaxStepError(err)
	}

	// 先处理推理内容（如果有）
	if onReasoning != nil && out.ReasoningContent != "" {
		onReasoning(out.ReasoningContent)
		slog.Debug("runtime.dispatch_agent.reasoning", "length", len(out.ReasoningContent))
	}

	slog.Debug("runtime.dispatch_agent.output", "content", out.Content, "content_len", len(out.Content))

	return append(in, out), nil
}

func systemPromptWithContext(ctx context.Context, rt *EinoRuntime, mcpTools []tool.BaseTool, history []*schema.Message) string {
	return buildDispatchSystemPrompt(ctx, rt, mcpTools, history)
}

func normalizeDispatchHistory(systemPrompt string, history []*schema.Message) (string, []*schema.Message) {
	if len(history) == 0 {
		return systemPrompt, nil
	}
	normalized := make([]*schema.Message, 0, len(history))
	var foldedSystem []string
	for _, msg := range history {
		if msg == nil {
			continue
		}
		if msg.Role == schema.System {
			if content := strings.TrimSpace(msg.Content); content != "" {
				foldedSystem = append(foldedSystem, content)
			}
			continue
		}
		normalized = append(normalized, msg)
	}
	if len(foldedSystem) == 0 {
		return systemPrompt, normalized
	}
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(systemPrompt))
	sb.WriteString("\n\n## 前置上下文\n")
	for _, block := range foldedSystem {
		sb.WriteString(block)
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String()), normalized
}

// buildDispatchSystemPrompt 构建调度 Agent 的系统提示词
// 动态替换占位符：{model_name}, {available_tools}
func buildDispatchSystemPrompt(ctx context.Context, rt *EinoRuntime, mcpTools []tool.BaseTool, history []*schema.Message) string {
	prompt := RoleArchitectPrompt

	// 替换模型名称
	modelName := "未知模型"
	if rt != nil {
		if name := rt.ModelName(); name != "" {
			modelName = name
		}
	}
	prompt = strings.Replace(prompt, "{model_name}", modelName, -1)

	// 调度 Agent 只能看到自己真实可调用的调度工具，避免调用执行层工具。
	toolsDesc := GetDispatchToolsDescription()
	prompt = strings.Replace(prompt, "{available_tools}", toolsDesc, -1)

	// 替换工作目录
	envInfo := envInfoForContext(ctx)
	prompt = strings.Replace(prompt, "{cwd}", envInfo.CWD, -1)

	prompt += "\n\n" + utils.FormatEnvInfo(envInfo)
	prompt += "\n" + BuildDispatchPromptAdditions(envInfo.CWD)
	prompt += "\n" + buildIntentPromptAdditions(history)

	// Inject system reminders from prompt_system.go
	prompt += "\n\n" + getSystemRemindersSection()

	return prompt
}

func shouldBypassArchitect(msgs []*schema.Message) bool {
	// 显式要求计划模式时，强制走 Architect
	if hasPlanPreferenceFirst(msgs) {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(extractLastUserText(msgs)))
	if q == "" {
		return false
	}
	// 询问系统能力/子代理等“元问题”，由主 Agent 直接回答，避免分配给子 Agent
	if isLikelyMetaQuery(q) {
		return false
	}
	// 问候/闲聊由主 Agent 直接回复，避免触发子 Agent 调度
	if isLikelySmallTalk(q) {
		return false
	}
	// 纯问答直接由 Architect 回复，不需要旁路到 SeniorDev
	if isLikelyQuestionOnly(q) && !isLikelyCodeChange(q) {
		return false
	}
	// 只有涉及多模块重构/架构设计的真正复杂任务才需要 Architect 先规划
	if isHighComplexityTask(q) {
		return false
	}
	// 其余所有任务（代码修改、调试、添加功能、简单分析）直接给 SeniorDev
	return true
}

func isLikelyMetaQuery(q string) bool {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return false
	}
	keys := []string{
		"子agent", "子 agent", "subagent", "sub-agent", "sub agent",
		"子代理", "子代理人",
		"有哪些agent", "有哪些 agent", "有哪些代理", "有哪些子代理", "有哪些子 agent", "有哪些子agent",
		"有哪些工具", "有哪些能力", "能做什么", "可以做什么",
	}
	for _, k := range keys {
		if strings.Contains(q, k) {
			return true
		}
	}
	return false
}

func isLikelySmallTalk(q string) bool {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return false
	}
	// 超短输入（且不含代码修改意图）多为寒暄/确认在线
	if len([]rune(q)) <= 6 && !isLikelyCodeChange(q) {
		return true
	}
	keys := []string{
		"你好", "您好", "在吗", "在不在", "hi", "hello",
		"你是谁", "你是啥", "你是什么", "你能做什么", "能做什么",
		"有哪些子agent", "有哪些子 agent", "有哪些子代理", "子agent有哪些", "子代理有哪些",
		"help", "帮助", "说明", "介绍一下",
	}
	for _, k := range keys {
		if strings.Contains(q, k) {
			return true
		}
	}
	return false
}

func hasPlanPreferenceFirst(msgs []*schema.Message) bool {
	for _, m := range msgs {
		if m == nil || m.Role != schema.System {
			continue
		}
		if strings.Contains(strings.ToLower(m.Content), "plan_preference: prefer_plan_first") {
			return true
		}
	}
	return false
}

func hasPlanModeForce(msgs []*schema.Message) bool {
	for _, m := range msgs {
		if m == nil || m.Role != schema.System {
			continue
		}
		if strings.Contains(strings.ToLower(m.Content), "plan_mode: force") {
			return true
		}
	}
	return false
}

func extractLastUserText(msgs []*schema.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m == nil {
			continue
		}
		if m.Role == schema.User {
			return strings.TrimSpace(m.Content)
		}
	}
	return ""
}

func isLikelyQuestionOnly(q string) bool {
	if strings.Contains(q, "？") || strings.Contains(q, "?") {
		return true
	}
	if strings.HasPrefix(q, "什么") || strings.HasPrefix(q, "怎么") || strings.HasPrefix(q, "为何") || strings.HasPrefix(q, "为什么") {
		return true
	}
	if strings.HasPrefix(q, "有哪些") || strings.Contains(q, "有哪些") {
		return true
	}
	return false
}

// isHighComplexityTask 判断任务是否为真正需要 Architect 先规划的高复杂度任务
// 需要多个复杂性指标同时出现，或使用特定的架构级关键词
func isHighComplexityTask(q string) bool {
	complexIndicators := []string{
		"重构", "架构", "设计方案", "多模块", "大范围", "系统性",
		"迁移", "migration", "roadmap",
	}
	matchCount := 0
	for _, k := range complexIndicators {
		if strings.Contains(q, k) {
			matchCount++
		}
	}
	// 需要至少 2 个复杂性指标同时出现才认定为高复杂度
	return matchCount >= 2
}

func isLikelyCodeChange(q string) bool {
	change := []string{
		"修复", "修正", "fix", "bug",
		"改", "调整", "优化", "增加", "添加", "实现", "支持", "去掉", "删除",
		"warning", "lint", "报错", "错误", "异常",
	}
	for _, k := range change {
		if strings.Contains(q, k) {
			return true
		}
	}
	return false
}

func buildSeniorDevTaskHint(msgs []*schema.Message) string {
	q := strings.TrimSpace(extractLastUserText(msgs))
	if q == "" {
		return ""
	}
	ql := strings.ToLower(q)
	var hints []string

	// 硬编码的特定场景提示
	if strings.Contains(ql, "ctrl+c") || strings.Contains(ql, "ctrl + c") || strings.Contains(ql, "ctrl c") {
		hints = append(hints, "关键词：signal、Interrupt、os.Signal、ctrl")
	}
	if strings.Contains(ql, "warning") || strings.Contains(ql, "lint") {
		hints = append(hints, "关键词：golangci-lint、staticcheck、govet、switch")
	}

	// 自动从用户输入中提取可能的代码标识符作为搜索线索
	keywords := extractCodeKeywords(q)
	if len(keywords) > 0 {
		hints = append(hints, "搜索建议关键词："+strings.Join(keywords, "、"))
	}

	if len(hints) == 0 {
		return "目标：" + q
	}
	return "目标：" + q + "\n" + strings.Join(hints, "\n") + "\n完成标准：修改后行为符合预期且 go test ./... 通过。"
}

// extractCodeKeywords 从用户输入中提取可能的代码标识符
// 识别驼峰命名、蛇形命名、点分路径等代码相关词
func extractCodeKeywords(text string) []string {
	// 匹配驼峰命名 (e.g. BuildContext, isLikelyCodeChange)
	camelRe := regexp.MustCompile(`[A-Z][a-zA-Z]{4,}`)
	// 匹配蛇形/下划线命名 (e.g. tool_cache, max_step)
	snakeRe := regexp.MustCompile(`[a-z]+_[a-z_]+`)
	// 匹配点分路径 (e.g. config.Load, tools.Manager)
	dotRe := regexp.MustCompile(`[a-zA-Z]+\.[a-zA-Z]+`)

	seen := map[string]struct{}{}
	var out []string

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || len(s) < 4 {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	for _, m := range camelRe.FindAllString(text, 10) {
		add(m)
	}
	for _, m := range snakeRe.FindAllString(text, 10) {
		add(m)
	}
	for _, m := range dotRe.FindAllString(text, 10) {
		add(m)
	}

	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

// GetTokenAnalyzer 获取 Token 分析器
func (rt *EinoRuntime) GetTokenAnalyzer() *TokenAnalyzer {
	return rt.tokenAnalyzer
}

// GetTokenSummary 获取 Token 使用摘要
func (rt *EinoRuntime) GetTokenSummary() map[string]any {
	if rt.tokenAnalyzer == nil {
		return nil
	}
	return rt.tokenAnalyzer.GetSummary()
}

// RecordTokenMetrics 记录 Token 使用指标
func (rt *EinoRuntime) RecordTokenMetrics(stage, component string, inputTokens, outputTokens int64) {
	if rt.tokenAnalyzer == nil {
		return
	}
	rt.tokenAnalyzer.Record(TokenMetrics{
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		TotalTokens:   inputTokens + outputTokens,
		Component:     component,
		Stage:         stage,
		Timestamp:     time.Now(),
	})
}

// NextTokenRound 进入下一轮 Token 记录
func (rt *EinoRuntime) NextTokenRound() {
	if rt.tokenAnalyzer == nil {
		return
	}
	rt.tokenAnalyzer.NextRound()
}
