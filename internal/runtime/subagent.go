package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/dreamSailing/eos/internal/tools"
)

// SubAgentType 子代理类型
type SubAgentType int

const (
	SubAgentTypePlanner SubAgentType = iota
	SubAgentTypeSeniorDev
	SubAgentTypeTester
	SubAgentTypeReviewer
	SubAgentTypeExplore   // 只读探索代理
	SubAgentTypeSecurity  // 安全审计代理
	SubAgentTypeArchitect // 架构设计代理
)

func (t SubAgentType) String() string {
	switch t {
	case SubAgentTypePlanner:
		return "planner"
	case SubAgentTypeSeniorDev:
		return "senior-dev"
	case SubAgentTypeTester:
		return "tester"
	case SubAgentTypeReviewer:
		return "reviewer"
	case SubAgentTypeExplore:
		return "explore"
	case SubAgentTypeSecurity:
		return "security"
	case SubAgentTypeArchitect:
		return "architect"
	default:
		return "unknown"
	}
}

// Description 返回子代理类型的描述
func (t SubAgentType) Description() string {
	switch t {
	case SubAgentTypePlanner:
		return "规划复杂任务的执行步骤"
	case SubAgentTypeSeniorDev:
		return "编写和实现代码功能"
	case SubAgentTypeTester:
		return "生成和验证测试用例"
	case SubAgentTypeReviewer:
		return "审查代码质量和最佳实践"
	case SubAgentTypeExplore:
		return "探索和理解代码库结构"
	case SubAgentTypeSecurity:
		return "审计代码安全漏洞和风险"
	case SubAgentTypeArchitect:
		return "设计系统架构和技术方案"
	default:
		return "未知代理类型"
	}
}

// ContextStrategy 返回子代理的上下文策略
func (t SubAgentType) ContextStrategy() ContextStrategy {
	switch t {
	case SubAgentTypeExplore:
		return ContextStrategyIndependent // 只读探索，独立上下文
	case SubAgentTypeTester:
		return ContextStrategyIndependent // 测试生成，独立上下文
	case SubAgentTypeReviewer:
		return ContextStrategyHybrid // 代码审查，混合上下文
	case SubAgentTypeSecurity:
		return ContextStrategyHybrid // 安全审计，混合上下文
	case SubAgentTypeArchitect:
		return ContextStrategyHybrid // 架构设计，混合上下文
	default:
		return ContextStrategyShared // 其他类型使用共享上下文
	}
}

// AllowedTools 返回子代理允许的工具列表
func (t SubAgentType) AllowedTools() []string {
	switch t {
	case SubAgentTypeExplore:
		return []string{"glob", "grep", tools.ToolRead, tools.ToolSearch, tools.ToolTodoRead, tools.ToolProjectStructure}
	case SubAgentTypeReviewer:
		return []string{tools.ToolRead, "diff", "glob", "grep", tools.ToolSearch, tools.ToolTodoRead, tools.ToolProjectStructure}
	case SubAgentTypeTester:
		return []string{"read", "write", "glob", "grep"}
	case SubAgentTypeSecurity:
		return []string{"glob", "grep", tools.ToolRead, tools.ToolSearch, tools.ToolTodoRead, tools.ToolProjectStructure}
	case SubAgentTypeArchitect:
		return []string{"glob", "grep", tools.ToolRead, tools.ToolSearch, "diff", tools.ToolTodoRead, tools.ToolProjectStructure}
	default:
		return nil // 使用默认工具集
	}
}

// ContextStrategy 上下文策略类型
type ContextStrategy int

const (
	// ContextStrategyIndependent 独立上下文：不继承主上下文
	ContextStrategyIndependent ContextStrategy = iota
	// ContextStrategyShared 共享上下文：完全继承主上下文
	ContextStrategyShared
	// ContextStrategyHybrid 混合上下文：选择性继承主上下文
	ContextStrategyHybrid
)

func (s ContextStrategy) String() string {
	switch s {
	case ContextStrategyIndependent:
		return "independent"
	case ContextStrategyShared:
		return "shared"
	case ContextStrategyHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

type AgentStatus string

const (
	AgentStatusPending   AgentStatus = "pending"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusFailed    AgentStatus = "failed"
	AgentStatusCancelled AgentStatus = "cancelled"
)

func (s AgentStatus) terminal() bool {
	switch s {
	case AgentStatusCompleted, AgentStatusFailed, AgentStatusCancelled:
		return true
	default:
		return false
	}
}

type AgentSnapshot struct {
	ID           string
	Name         string
	Task         string
	Status       AgentStatus
	Error        string
	Result       string
	StartedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  time.Time
	Messages     int
	Strategy     string
	AllowedTools []string
	CanResume    bool
	CanClose     bool
	CanSendInput bool
}

// SubAgentContext 子代理隔离上下文
type SubAgentContext struct {
	id           string            // 唯一 ID
	agentType    SubAgentType      // 代理类型
	parentCtx    context.Context   // 父上下文
	messages     []*schema.Message // 独立消息历史
	strategy     ContextStrategy   // 上下文策略
	allowedTools []string          // 允许的工具列表
	task         string            // 最近一次执行的任务
	status       AgentStatus       // 当前生命周期状态
	errorMsg     string            // 最近一次错误
	createdAt    time.Time         // 创建时间
	updatedAt    time.Time         // 更新时间
	completedAt  time.Time         // 最近一次完成时间
	runCount     int               // 已执行次数
	completed    bool              // 是否完成
	cancel       context.CancelFunc
	doneCh       chan struct{}
	cancelReq    bool
	result       string       // 最终结果
	mu           sync.RWMutex // 保护并发访问
}

func (c *SubAgentContext) ID() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

// SubAgentResult 子代理执行结果
type SubAgentResult struct {
	ID        string        // 子代理 ID
	Type      SubAgentType  // 代理类型
	Task      string        // 执行的任务
	Status    AgentStatus   // 完成状态
	Result    string        // 结果摘要
	Messages  int           // 产生的消息数量
	Duration  time.Duration // 执行时长
	Success   bool          // 是否成功
	Error     string        // 错误信息
	Timestamp time.Time     // 完成时间
}

// SubAgentManager 子代理管理器
type SubAgentManager struct {
	agents       map[string]*SubAgentContext
	requestIndex map[string]string
	history      []SubAgentResult
	mu           sync.RWMutex
	nextID       int
	maxHistory   int // 最大历史记录数
}

// NewSubAgentManager 创建子代理管理器
func NewSubAgentManager() *SubAgentManager {
	return &SubAgentManager{
		agents:       make(map[string]*SubAgentContext),
		requestIndex: make(map[string]string),
		history:      make([]SubAgentResult, 0, 100),
		maxHistory:   100,
	}
}

func (m *SubAgentManager) GetOrCreateRequestContext(
	requestID string,
	agentType SubAgentType,
	parentCtx context.Context,
	initialMsgs []*schema.Message,
) *SubAgentContext {
	if strings.TrimSpace(requestID) == "" {
		return m.CreateContext(agentType, parentCtx, initialMsgs)
	}

	key := strings.TrimSpace(requestID) + "|" + agentType.String()

	m.mu.Lock()
	defer m.mu.Unlock()

	if id, ok := m.requestIndex[key]; ok {
		if ctx, ok := m.agents[id]; ok {
			return ctx
		}
		delete(m.requestIndex, key)
	}

	m.nextID++
	id := fmt.Sprintf("subagent_%s_%d", agentType.String(), m.nextID)

	// 根据上下文策略过滤消息，避免子 Agent 上下文膨胀
	msgs := filterMessagesForSubAgent(initialMsgs, agentType.ContextStrategy())

	ctx := &SubAgentContext{
		id:           id,
		agentType:    agentType,
		parentCtx:    parentCtx,
		messages:     msgs,
		strategy:     agentType.ContextStrategy(),
		allowedTools: agentType.AllowedTools(),
		status:       AgentStatusPending,
		createdAt:    time.Now(),
		updatedAt:    time.Now(),
		doneCh:       make(chan struct{}),
	}

	m.agents[id] = ctx
	m.requestIndex[key] = id
	slog.Debug("subagent.request_context.create",
		"request_id", strings.TrimSpace(requestID),
		"id", id,
		"type", agentType.String(),
		"strategy", ctx.strategy.String(),
		"original_messages", len(initialMsgs),
		"filtered_messages", len(msgs),
	)

	return ctx
}

// filterMessagesForSubAgent 根据上下文策略过滤传给子 Agent 的消息
// 去掉老旧的 Conversation summary、保留最近的对话消息
func filterMessagesForSubAgent(msgs []*schema.Message, strategy ContextStrategy) []*schema.Message {
	switch strategy {
	case ContextStrategyIndependent:
		// 独立上下文: 不继承任何消息
		return make([]*schema.Message, 0)

	case ContextStrategyHybrid:
		// 混合上下文: 最近 8 条消息
		maxMsgs := 8
		if len(msgs) > maxMsgs {
			start := len(msgs) - maxMsgs
			filtered := make([]*schema.Message, maxMsgs)
			copy(filtered, msgs[start:])
			return filtered
		}
		filtered := make([]*schema.Message, len(msgs))
		copy(filtered, msgs)
		return filtered

	default:
		// 共享上下文: 去掉旧的压缩摘要，只保留有效消息
		// 过滤掉 "Conversation summary" 系统消息（已被压缩的历史）
		// 保留最近 12 条 user/assistant 消息 + 有效的 system 消息
		var systemMsgs []*schema.Message
		var convMsgs []*schema.Message
		for _, m := range msgs {
			if m == nil {
				continue
			}
			if m.Role == schema.System {
				// 跳过旧的压缩摘要
				if strings.HasPrefix(m.Content, "Conversation summary") {
					continue
				}
				systemMsgs = append(systemMsgs, m)
			} else {
				convMsgs = append(convMsgs, m)
			}
		}
		maxConv := 12
		if len(convMsgs) > maxConv {
			convMsgs = convMsgs[len(convMsgs)-maxConv:]
		}
		result := make([]*schema.Message, 0, len(systemMsgs)+len(convMsgs))
		result = append(result, systemMsgs...)
		result = append(result, convMsgs...)
		return result
	}
}

func (m *SubAgentManager) ClearRequest(requestID string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return
	}

	prefix := requestID + "|"
	toRemove := make([]struct {
		key string
		id  string
	}, 0)

	m.mu.RLock()
	for k, id := range m.requestIndex {
		if strings.HasPrefix(k, prefix) {
			toRemove = append(toRemove, struct {
				key string
				id  string
			}{key: k, id: id})
		}
	}
	m.mu.RUnlock()

	for _, item := range toRemove {
		_ = m.RequestCancel(item.id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range toRemove {
		delete(m.agents, item.id)
		delete(m.requestIndex, item.key)
	}
}

// CreateContext 创建子代理隔离上下文
func (m *SubAgentManager) CreateContext(agentType SubAgentType, parentCtx context.Context, initialMsgs []*schema.Message) *SubAgentContext {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := fmt.Sprintf("subagent_%s_%d", agentType.String(), m.nextID)

	// 复制初始消息，避免修改原始消息
	msgs := make([]*schema.Message, len(initialMsgs))
	copy(msgs, initialMsgs)

	ctx := &SubAgentContext{
		id:           id,
		agentType:    agentType,
		parentCtx:    parentCtx,
		messages:     msgs,
		strategy:     agentType.ContextStrategy(),
		allowedTools: agentType.AllowedTools(),
		status:       AgentStatusPending,
		createdAt:    time.Now(),
		updatedAt:    time.Now(),
		doneCh:       make(chan struct{}),
	}

	m.agents[id] = ctx
	slog.Debug("subagent.create",
		"id", id,
		"type", agentType.String(),
		"strategy", ctx.strategy.String(),
		"initial_messages", len(msgs),
	)

	return ctx
}

// CreateContextWithStrategy 使用指定策略创建子代理上下文
func (m *SubAgentManager) CreateContextWithStrategy(
	agentType SubAgentType,
	parentCtx context.Context,
	initialMsgs []*schema.Message,
	strategy ContextStrategy,
	allowedTools []string,
) *SubAgentContext {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := fmt.Sprintf("subagent_%s_%d", agentType.String(), m.nextID)

	// 根据策略处理初始消息
	var msgs []*schema.Message
	switch strategy {
	case ContextStrategyIndependent:
		// 独立上下文：不包含主上下文消息
		msgs = make([]*schema.Message, 0)
	case ContextStrategyShared:
		// 共享上下文：包含所有初始消息
		msgs = make([]*schema.Message, len(initialMsgs))
		copy(msgs, initialMsgs)
	case ContextStrategyHybrid:
		// 混合上下文：选择性包含消息（最近 N 条）
		maxMsgs := 8
		if len(initialMsgs) > maxMsgs {
			start := len(initialMsgs) - maxMsgs
			msgs = make([]*schema.Message, maxMsgs)
			copy(msgs, initialMsgs[start:])
		} else {
			msgs = make([]*schema.Message, len(initialMsgs))
			copy(msgs, initialMsgs)
		}
	default:
		msgs = make([]*schema.Message, len(initialMsgs))
		copy(msgs, initialMsgs)
	}

	ctx := &SubAgentContext{
		id:           id,
		agentType:    agentType,
		parentCtx:    parentCtx,
		messages:     msgs,
		strategy:     strategy,
		allowedTools: allowedTools,
		status:       AgentStatusPending,
		createdAt:    time.Now(),
		updatedAt:    time.Now(),
		doneCh:       make(chan struct{}),
	}

	m.agents[id] = ctx
	slog.Debug("subagent.create_with_strategy",
		"id", id,
		"type", agentType.String(),
		"strategy", strategy.String(),
		"filtered_messages", len(msgs),
		"original_messages", len(initialMsgs),
	)

	return ctx
}

// GetContext 获取子代理上下文
func (m *SubAgentManager) GetContext(id string) (*SubAgentContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ctx, ok := m.agents[id]
	return ctx, ok
}

// AddMessage 添加消息到子代理上下文
func (m *SubAgentManager) AddMessage(id string, msg *schema.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, ok := m.agents[id]
	if !ok {
		return fmt.Errorf("subagent context not found: %s", id)
	}

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	ctx.messages = append(ctx.messages, msg)
	ctx.updatedAt = time.Now()
	return nil
}

// GetMessages 获取子代理的消息
func (m *SubAgentManager) GetMessages(id string) []*schema.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctx, ok := m.agents[id]
	if !ok {
		return nil
	}

	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	msgs := make([]*schema.Message, len(ctx.messages))
	copy(msgs, ctx.messages)
	return msgs
}

func closeDoneCh(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
		return
	default:
		close(ch)
	}
}

func snapshotFromContext(ctx *SubAgentContext) AgentSnapshot {
	if ctx == nil {
		return AgentSnapshot{}
	}
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return AgentSnapshot{
		ID:           ctx.id,
		Name:         ctx.agentType.String(),
		Task:         ctx.task,
		Status:       ctx.status,
		Error:        ctx.errorMsg,
		Result:       ctx.result,
		StartedAt:    ctx.createdAt,
		UpdatedAt:    ctx.updatedAt,
		CompletedAt:  ctx.completedAt,
		Messages:     len(ctx.messages),
		Strategy:     ctx.strategy.String(),
		AllowedTools: append([]string(nil), ctx.allowedTools...),
		CanResume:    ctx.status.terminal(),
		CanClose:     true,
		CanSendInput: ctx.status != AgentStatusRunning,
	}
}

func snapshotFromHistory(res SubAgentResult) AgentSnapshot {
	return AgentSnapshot{
		ID:           res.ID,
		Name:         res.Type.String(),
		Task:         res.Task,
		Status:       res.Status,
		Error:        res.Error,
		Result:       res.Result,
		StartedAt:    res.Timestamp.Add(-res.Duration),
		UpdatedAt:    res.Timestamp,
		CompletedAt:  res.Timestamp,
		Messages:     res.Messages,
		Strategy:     res.Type.ContextStrategy().String(),
		AllowedTools: append([]string(nil), res.Type.AllowedTools()...),
		CanResume:    false,
		CanClose:     false,
		CanSendInput: false,
	}
}

func appendHistoryBounded(history []SubAgentResult, maxHistory int, res SubAgentResult) []SubAgentResult {
	history = append(history, res)
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}
	return history
}

func (m *SubAgentManager) MarkRunning(id string, task string, cancel context.CancelFunc) error {
	m.mu.RLock()
	ctx, ok := m.agents[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("subagent context not found: %s", id)
	}

	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.status == AgentStatusRunning {
		return fmt.Errorf("subagent already running: %s", id)
	}
	ctx.task = strings.TrimSpace(task)
	ctx.status = AgentStatusRunning
	ctx.errorMsg = ""
	ctx.completed = false
	ctx.completedAt = time.Time{}
	ctx.updatedAt = time.Now()
	ctx.cancel = cancel
	ctx.cancelReq = false
	ctx.runCount++
	ctx.doneCh = make(chan struct{})
	return nil
}

func (m *SubAgentManager) RequestCancel(id string) error {
	m.mu.RLock()
	ctx, ok := m.agents[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("subagent context not found: %s", id)
	}

	var cancel context.CancelFunc
	ctx.mu.Lock()
	if ctx.status != AgentStatusRunning {
		ctx.mu.Unlock()
		return nil
	}
	ctx.cancelReq = true
	ctx.updatedAt = time.Now()
	cancel = ctx.cancel
	ctx.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}

func (m *SubAgentManager) Wait(ctx context.Context, id string) (AgentSnapshot, error) {
	m.mu.RLock()
	subCtx, ok := m.agents[id]
	m.mu.RUnlock()
	if !ok {
		return AgentSnapshot{}, fmt.Errorf("subagent context not found: %s", id)
	}

	subCtx.mu.RLock()
	doneCh := subCtx.doneCh
	status := subCtx.status
	subCtx.mu.RUnlock()

	if status.terminal() {
		return snapshotFromContext(subCtx), nil
	}

	select {
	case <-ctx.Done():
		return snapshotFromContext(subCtx), ctx.Err()
	case <-doneCh:
		return snapshotFromContext(subCtx), nil
	}
}

func (m *SubAgentManager) Remove(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.agents[id]; !ok {
		return false
	}
	delete(m.agents, id)
	for key, agentID := range m.requestIndex {
		if agentID == id {
			delete(m.requestIndex, key)
		}
	}
	return true
}

func (m *SubAgentManager) ListSnapshots() []AgentSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snaps := make([]AgentSnapshot, 0, len(m.agents)+len(m.history))
	seen := make(map[string]struct{}, len(m.agents))
	for _, ctx := range m.agents {
		snap := snapshotFromContext(ctx)
		snaps = append(snaps, snap)
		seen[snap.ID] = struct{}{}
	}
	for i := len(m.history) - 1; i >= 0; i-- {
		res := m.history[i]
		if _, ok := seen[res.ID]; ok {
			continue
		}
		snaps = append(snaps, snapshotFromHistory(res))
		seen[res.ID] = struct{}{}
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].UpdatedAt.After(snaps[j].UpdatedAt)
	})
	return snaps
}

// Complete 标记子代理完成，并生成结果摘要
func (m *SubAgentManager) Complete(id string, task string, success bool, errorMsg string) SubAgentResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, ok := m.agents[id]
	if !ok {
		return SubAgentResult{
			ID:        id,
			Success:   false,
			Error:     "context not found",
			Timestamp: time.Now(),
		}
	}

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if ctx.status == AgentStatusCancelled {
		return SubAgentResult{
			ID:        id,
			Type:      ctx.agentType,
			Task:      ctx.task,
			Status:    AgentStatusCancelled,
			Result:    ctx.result,
			Messages:  len(ctx.messages),
			Duration:  ctx.completedAt.Sub(ctx.createdAt),
			Success:   false,
			Error:     ctx.errorMsg,
			Timestamp: ctx.completedAt,
		}
	}

	ctx.completed = true
	ctx.task = strings.TrimSpace(task)
	ctx.updatedAt = time.Now()
	ctx.completedAt = ctx.updatedAt
	ctx.errorMsg = strings.TrimSpace(errorMsg)
	ctx.cancel = nil
	ctx.cancelReq = false

	// 生成结果摘要
	result := m.generateSummary(ctx)
	if !success {
		result = fmt.Sprintf("执行失败: %s", errorMsg)
	}
	ctx.result = strings.TrimSpace(result)
	if success {
		ctx.status = AgentStatusCompleted
	} else {
		ctx.status = AgentStatusFailed
	}

	duration := ctx.completedAt.Sub(ctx.createdAt)

	res := SubAgentResult{
		ID:        id,
		Type:      ctx.agentType,
		Task:      ctx.task,
		Status:    ctx.status,
		Result:    result,
		Messages:  len(ctx.messages),
		Duration:  duration,
		Success:   success,
		Error:     ctx.errorMsg,
		Timestamp: ctx.completedAt,
	}

	// 添加到历史记录
	m.history = appendHistoryBounded(m.history, m.maxHistory, res)
	closeDoneCh(ctx.doneCh)

	slog.Debug("subagent.complete",
		"id", id,
		"type", ctx.agentType.String(),
		"success", success,
		"duration", duration.String(),
		"messages", len(ctx.messages),
	)

	return res
}

func (m *SubAgentManager) Cancel(id string, reason string) (SubAgentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, ok := m.agents[id]
	if !ok {
		return SubAgentResult{}, fmt.Errorf("subagent context not found: %s", id)
	}

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if ctx.status == AgentStatusCancelled {
		return SubAgentResult{
			ID:        id,
			Type:      ctx.agentType,
			Task:      ctx.task,
			Status:    AgentStatusCancelled,
			Result:    ctx.result,
			Messages:  len(ctx.messages),
			Duration:  ctx.completedAt.Sub(ctx.createdAt),
			Success:   false,
			Error:     ctx.errorMsg,
			Timestamp: ctx.completedAt,
		}, nil
	}

	ctx.completed = true
	ctx.status = AgentStatusCancelled
	ctx.errorMsg = strings.TrimSpace(reason)
	ctx.updatedAt = time.Now()
	ctx.completedAt = ctx.updatedAt
	ctx.cancel = nil
	ctx.cancelReq = false
	if strings.TrimSpace(ctx.result) == "" {
		ctx.result = "执行已取消"
	}

	res := SubAgentResult{
		ID:        id,
		Type:      ctx.agentType,
		Task:      ctx.task,
		Status:    AgentStatusCancelled,
		Result:    ctx.result,
		Messages:  len(ctx.messages),
		Duration:  ctx.completedAt.Sub(ctx.createdAt),
		Success:   false,
		Error:     ctx.errorMsg,
		Timestamp: ctx.completedAt,
	}

	m.history = appendHistoryBounded(m.history, m.maxHistory, res)
	closeDoneCh(ctx.doneCh)
	return res, nil
}

// generateSummary 生成子代理执行摘要
func (m *SubAgentManager) generateSummary(ctx *SubAgentContext) string {
	if len(ctx.messages) == 0 {
		return "无输出内容"
	}

	var sb strings.Builder

	// 提取工具调用记录（去重）
	toolCallSeen := map[string]struct{}{}
	var toolCalls []string
	var finalResult string
	for _, msg := range ctx.messages {
		if msg == nil {
			continue
		}
		if msg.Role == schema.Assistant {
			finalResult = msg.Content
			for _, tc := range msg.ToolCalls {
				name := tc.Function.Name
				if _, ok := toolCallSeen[name]; !ok {
					toolCallSeen[name] = struct{}{}
					toolCalls = append(toolCalls, name)
				}
			}
		}
	}

	// 生成结构化摘要
	if len(toolCalls) > 0 {
		sb.WriteString("执行操作: " + strings.Join(toolCalls, ", ") + "\n")
	}
	if len(finalResult) > 800 {
		finalResult = finalResult[:800] + "..."
	}
	if finalResult != "" {
		sb.WriteString("结果: " + finalResult)
	}

	result := sb.String()
	if result == "" {
		return fmt.Sprintf("产生 %d 条消息", len(ctx.messages))
	}
	return result
}

// GetHistory 获取子代理执行历史
func (m *SubAgentManager) GetHistory() []SubAgentResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]SubAgentResult, len(m.history))
	copy(results, m.history)
	return results
}

// GetHistoryByType 获取指定类型的子代理历史
func (m *SubAgentManager) GetHistoryByType(agentType SubAgentType) []SubAgentResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []SubAgentResult
	for _, r := range m.history {
		if r.Type == agentType {
			results = append(results, r)
		}
	}
	return results
}

// Clear 清除所有子代理上下文
func (m *SubAgentManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.agents = make(map[string]*SubAgentContext)
	slog.Debug("subagent.clear", "remaining_agents", len(m.agents))
}

// Cleanup 清理已完成的子代理上下文
func (m *SubAgentManager) Cleanup(olderThan time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()
	for id, ctx := range m.agents {
		ctx.mu.RLock()
		if ctx.completed && now.Sub(ctx.updatedAt) > olderThan {
			delete(m.agents, id)
			count++
		}
		ctx.mu.RUnlock()
	}

	if count > 0 {
		slog.Debug("subagent.cleanup", "cleaned_count", count, "remaining", len(m.agents))
	}

	return count
}

// Resume 从历史记录中恢复子代理状态
func (m *SubAgentManager) Resume(id string) (*SubAgentContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找历史记录
	var historyResult *SubAgentResult
	for _, r := range m.history {
		if r.ID == id {
			historyResult = &r
			break
		}
	}

	if historyResult == nil {
		return nil, false
	}

	// 检查是否还有活跃的上下文
	ctx, ok := m.agents[id]
	if !ok {
		return nil, false
	}

	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	return ctx, true
}

// Stats 获取子代理统计信息
func (m *SubAgentManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeCount := 0
	completedCount := 0
	typeCounts := make(map[SubAgentType]int)
	totalDuration := time.Duration(0)

	for _, ctx := range m.agents {
		ctx.mu.RLock()
		if ctx.completed {
			completedCount++
		} else {
			activeCount++
		}
		typeCounts[ctx.agentType]++
		ctx.mu.RUnlock()
	}

	for _, r := range m.history {
		typeCounts[r.Type]++
		totalDuration += r.Duration
	}

	avgDuration := time.Duration(0)
	if len(m.history) > 0 {
		avgDuration = totalDuration / time.Duration(len(m.history))
	}

	return map[string]interface{}{
		"active_count":    activeCount,
		"completed_count": completedCount,
		"history_count":   len(m.history),
		"type_counts":     typeCounts,
		"total_duration":  totalDuration.String(),
		"avg_duration":    avgDuration.String(),
	}
}
