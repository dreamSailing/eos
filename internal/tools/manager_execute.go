package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"

	"github.com/google/uuid"
)

// maxParallel 并行执行的最大并发数
const maxParallel = 5

// ExecuteStructured executes tool calls and returns structured results.
// 只读工具（RiskLevelLow）连续出现时自动并行执行，写入/危险工具保持串行。
func (m *Manager) ExecuteStructured(ctx context.Context, toolCalls []ToolCall) []ToolResult {
	results := make([]ToolResult, len(toolCalls))
	now := time.Now().Unix()
	i := 0

	for idx := range toolCalls {
		if toolCalls[idx].ID == "" {
			toolCalls[idx].ID = uuid.New().String()[:8]
		}
	}

	for i < len(toolCalls) {
		select {
		case <-ctx.Done():
			for j := i; j < len(toolCalls); j++ {
				results[j] = ToolResult{ID: toolCalls[j].ID, Type: "tool_result", Tool: toolCalls[j].Tool, Status: "error", Error: "Canceled", Ts: now}
			}
			return results
		default:
		}

		// 收集从 i 开始的连续只读工具批次
		batchEnd := i
		for batchEnd < len(toolCalls) {
			risk := GetToolRiskLevel(toolCalls[batchEnd].Tool)
			if risk != RiskLevelLow {
				break
			}
			batchEnd++
		}

		batchSize := batchEnd - i
		if batchSize > 1 {
			// 并行执行连续只读工具
			m.executeParallelBatch(ctx, toolCalls[i:batchEnd], results[i:batchEnd], now)
			i = batchEnd
		} else {
			// 串行执行单个工具（包括写入/危险工具和单个只读工具）
			results[i] = m.executeSingleWithCache(ctx, toolCalls[i], now)
			i++
		}
	}

	return results
}

// executeParallelBatch 并行执行一批只读工具调用
func (m *Manager) executeParallelBatch(ctx context.Context, calls []ToolCall, results []ToolResult, now int64) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallel)

	for idx, call := range calls {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(i int, c ToolCall) {
			defer wg.Done()
			defer func() { <-sem }() // 释放信号量

			results[i] = m.executeSingleWithCache(ctx, c, now)
		}(idx, call)
	}

	wg.Wait()
}

// executeSingleWithCache 执行单个工具调用，支持缓存
func (m *Manager) executeSingleWithCache(ctx context.Context, call ToolCall, now int64) ToolResult {
	select {
	case <-ctx.Done():
		return ToolResult{ID: call.ID, Type: "tool_result", Tool: call.Tool, Status: "error", Error: "Canceled", Ts: now}
	default:
	}

	// 检查缓存（仅对可缓存的工具）
	if m.cache != nil && IsCacheable(call.Tool, call.Parameters) {
		if cached, hit := m.cache.Get(call.Tool, call.Parameters); hit {
			cached.Display = "[cached] " + cached.Display
			cached.ID = call.ID
			return cached
		}
	}

	handler, ok := m.structured[call.Tool]
	if !ok {
		slog.Error("tool.unknown", "component", utils.ComponentTool, "tool", call.Tool)
		return ToolResult{
			ID:     call.ID,
			Type:   "tool_result",
			Tool:   call.Tool,
			Status: "error",
			Error:  fmt.Sprintf("Unknown tool: %s", call.Tool),
			Ts:     now,
		}
	}

	if allowed := AllowedToolsFromContext(ctx); allowed != nil && !allowed[strings.ToLower(call.Tool)] {
		return ToolResult{ID: call.ID, Type: "tool_result", Tool: call.Tool, Status: "error", Error: "permission denied: tool not allowed", Display: "Error: permission denied: tool not allowed", Ts: now}
	}

	// Fix 4: Ask-tool-approval for tools that require explicit user confirmation
	if m.AskToolApproval != nil && GetToolRiskLevel(call.Tool) >= RiskLevelHigh {
		if !m.AskToolApproval(call.Tool) {
			return ToolResult{ID: call.ID, Type: "tool_result", Tool: call.Tool, Status: "error", Error: "tool execution denied by user", Display: "Error: tool execution denied by user", Ts: now}
		}
	}

	// Pre-tool hook: allows input modification and execution veto
	params := call.Parameters
	if m.hookRunner != nil {
		proceed, modified, err := m.hookRunner.PreToolUse(ctx, call.Tool, call.Parameters)
		if err != nil {
			slog.Debug("tools.pre_hook.error", "component", utils.ComponentTool, "tool", call.Tool, "error", err.Error())
		}
		if !proceed {
			return ToolResult{ID: call.ID, Type: "tool_result", Tool: call.Tool, Status: "error", Error: "tool execution blocked by pre-hook", Display: "Error: tool execution blocked by pre-hook", Ts: now}
		}
		if modified != nil {
			params = modified
		}
	}

	r := handler(ctx, params)
	r.ID = call.ID
	if r.Ts == 0 {
		r.Ts = now
	}
	if strings.TrimSpace(r.Display) == "" {
		r.Display = summarizeDisplay(r)
	}
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	if _, exists := r.Data["params"]; !exists {
		r.Data["params"] = call.Parameters
	}
	r = m.limitToolOutputSize(r)

	// Enforce aggregate tool result budget per turn
	if m.resultBudget != nil && r.Status == "success" {
		if content, ok := r.Data["content"].(string); ok {
			replaced, truncated := m.resultBudget.CheckAndEnforce(call.ID, content)
			if truncated {
				r.Data["content"] = replaced
				r.Data["budget_truncated"] = true
			}
		} else if text, ok := r.Data["text"].(string); ok {
			replaced, truncated := m.resultBudget.CheckAndEnforce(call.ID, text)
			if truncated {
				r.Data["text"] = replaced
				r.Data["budget_truncated"] = true
			}
		}
	}

	// Post-tool hook: allows result processing (logging, notifications, etc.)
	if m.hookRunner != nil {
		if err := m.hookRunner.PostToolUse(ctx, call.Tool, params, r.Data); err != nil {
			slog.Debug("tools.post_hook.error", "component", utils.ComponentTool, "tool", call.Tool, "error", err.Error())
		}
	}

	// Reactive compaction: if tool output exceeds threshold, flag for context compression
	if r.Status == "success" && m.isReactiveCompactNeeded(r) {
		slog.Info("tools.reactive_compact.triggered", "component", utils.ComponentTool, "tool", call.Tool)
		r.Data["reactive_compact_suggested"] = true
		if m.OnReactiveCompact != nil {
			go m.OnReactiveCompact()
		}
	}

	// 写入缓存（仅对可缓存的成功结果）
	if m.cache != nil && r.Status == "success" && IsCacheable(call.Tool, call.Parameters) {
		m.cache.Put(call.Tool, call.Parameters, r)
	}

	// 写入操作时失效相关缓存
	if m.cache != nil {
		risk := GetToolRiskLevel(call.Tool)
		if risk >= RiskLevelMedium {
			path := extractPathFromParams(call.Parameters)
			m.cache.Invalidate(path)
		}
	}

	return r
}

// limitToolOutputSize limits the output size of tool results
func (m *Manager) limitToolOutputSize(r ToolResult) ToolResult {
	if r.Data == nil {
		return r
	}

	if content, ok := r.Data["content"].(string); ok && len(content) > ToolOutputMaxSize {
		slog.Debug("tools.limit_output_size",
			"component", utils.ComponentTool,
			"tool", r.Tool,
			"original_size", len(content),
			"max_size", ToolOutputMaxSize,
		)
		r.Data["content"] = TruncateOutput(content, ToolOutputMaxSize)
		r.Data["truncated"] = true
		r.Data["original_bytes"] = len(content)
	}

	if text, ok := r.Data["text"].(string); ok && len(text) > ToolOutputMaxSize {
		slog.Debug("tools.limit_output_size",
			"component", utils.ComponentTool,
			"tool", r.Tool,
			"original_size", len(text),
			"max_size", ToolOutputMaxSize,
		)
		r.Data["text"] = TruncateOutput(text, ToolOutputMaxSize)
		r.Data["truncated"] = true
	}

	if len(r.Display) > 500 {
		r.Display = r.Display[:500] + "..."
	}

	if len(r.Error) > 1000 {
		r.Error = r.Error[:1000] + "..."
	}

	return r
}

// execStructured executes a single tool call
func (m *Manager) execStructured(ctx context.Context, call ToolCall) ToolResult {
	handler, ok := m.structured[call.Tool]
	if !ok {
		slog.Error("tool.unknown", "component", utils.ComponentTool, "tool", call.Tool)
		return ToolResult{Type: "tool_result", Tool: call.Tool, Status: "error", Error: fmt.Sprintf("Unknown tool: %s", call.Tool)}
	}

	if allowed := AllowedToolsFromContext(ctx); allowed != nil && !allowed[strings.ToLower(call.Tool)] {
		r := ToolResult{Type: "tool_result", Tool: call.Tool, Status: "error", Error: "permission denied: tool not allowed", Display: "Error: permission denied: tool not allowed"}
		return r
	}

	return handler(ctx, call.Parameters)
}

// normalizePathPlaceholder removes @ prefix from paths
func normalizePathPlaceholder(p string) string {
	if strings.HasPrefix(p, "@") {
		np := strings.TrimPrefix(p, "@")
		slog.Debug("tools.normalize_path_placeholder", "component", utils.ComponentTool, "raw", p, "normalized", np)
		return np
	}
	return p
}

// reactiveCompactThresholdKB is the threshold in KB above which tool output triggers reactive compaction
const reactiveCompactThresholdKB = 50

// isReactiveCompactNeeded checks if the tool result size exceeds the reactive compaction threshold
func (m *Manager) isReactiveCompactNeeded(r ToolResult) bool {
	if r.Data == nil {
		return false
	}
	totalSize := 0
	for _, v := range r.Data {
		if s, ok := v.(string); ok {
			totalSize += len(s)
		}
	}
	threshold := reactiveCompactThresholdKB * 1024
	return totalSize > threshold
}
