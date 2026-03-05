package runtime

import (
	"log/slog"
	"strings"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

// EventEmitter 统一的事件发送器
// 封装观察者模式的事件发送逻辑，提供统一的日志和错误处理
type EventEmitter struct {
	onMeta func(string)
}

// NewEventEmitter 创建事件发送器
func NewEventEmitter(onMeta func(string)) *EventEmitter {
	return &EventEmitter{onMeta: onMeta}
}

// EmitToolCall 发送工具调用事件
func (e *EventEmitter) EmitToolCall(toolID, toolName string) {
	if e.onMeta == nil {
		return
	}
	name := strings.TrimSpace(toolName)
	if name == "" {
		LogWarn("runtime.emit_tool_call.empty_name")
		return
	}
	// 调试日志：检查 invoke_* 工具是否被发送
	if strings.HasPrefix(name, "invoke_") {
		slog.Debug("runtime.emit_tool_call.invoke_tool", "tool", name, "event", EventToolCall)
	}
	if toolID != "" {
		e.onMeta(EventToolCall + ":" + toolID + ":" + name)
	} else {
		e.onMeta(EventToolCall + ":" + name)
	}
	LogToolCall(name)
}

// EmitToolResult 发送工具结果事件
func (e *EventEmitter) EmitToolResult(result tools.ToolResult) {
	if e.onMeta == nil {
		slog.Warn("runtime.emit_tool_result.skipped_nil_meta", "tool", result.Tool)
		return
	}

	name := strings.TrimSpace(result.Tool)
	if name == "" {
		LogWarn("runtime.emit_tool_result.empty_name")
		return
	}

	status := strings.ToLower(strings.TrimSpace(result.Status))
	if status == "" {
		LogWarn("runtime.emit_tool_result.empty_status", LogKeyTool, name)
		return
	}

	// 子 Agent 工具（invoke_planner, invoke_senior_dev 等）不发送工具结果事件
	// 它们的执行过程通过 agent.task 事件显示任务分配，通过 assistant.final 显示结果
	if result.Type == tools.ToolTypeAgent {
		slog.Debug("runtime.emit_tool_result.skip_agent", "tool", name, "type", result.Type)
		return
	}

	// pending 状态使用 EmitToolCall
	if status == "pending" {
		toolStr := name
		if params, ok := result.Data["params"].(map[string]any); ok {
			if p, ok := params["path"].(string); ok {
				toolStr += " (" + p + ")"
			} else if p, ok := params["file"].(string); ok {
				toolStr += " (" + p + ")"
			} else if p, ok := params["source"].(string); ok {
				toolStr += " (" + p + ")"
			}
		}
		e.EmitToolCall(result.ID, toolStr)
		return
	}

	// 其他状态发送格式化的结果，包含 ID
	formatted := result.FormatForUI()
	if result.ID != "" {
		e.onMeta(EventToolResult + ":" + result.ID + ":" + formatted)
	} else {
		e.onMeta(EventToolResult + ":" + formatted)
	}
	LogToolResult(name, status, len(formatted))
}

// EmitAssistantDelta 发送助手增量输出事件
func (e *EventEmitter) EmitAssistantDelta(content string) {
	if e.onMeta == nil || content == "" {
		return
	}
	e.onMeta(EventAssistantDelta + ":" + content)
	preview := content
	if len(preview) > 50 {
		preview = preview[:50] + "..."
	}
	LogEmitEvent(EventAssistantDelta, preview)
}

// EmitPhaseNote 发送阶段备注事件
func (e *EventEmitter) EmitPhaseNote(note string) {
	if e.onMeta == nil || note == "" {
		return
	}
	e.onMeta(EventPhaseNote + ":" + note)
	LogDebug("runtime.emit.phase_note", LogKeyContent, note)
}

// EmitCrumb 发送面包屑事件（调试信息）
func (e *EventEmitter) EmitCrumb(msg string) {
	if e.onMeta == nil || msg == "" {
		return
	}
	e.onMeta(EventCrumb + ":" + msg)
}

// EmitPlanReady 发送计划就绪事件
func (e *EventEmitter) EmitPlanReady() {
	if e.onMeta == nil {
		return
	}
	e.onMeta(EventPlanReady)
	LogDebug("runtime.emit.plan_ready")
}

// HasCallback 检查是否有回调注册
func (e *EventEmitter) HasCallback() bool {
	return e.onMeta != nil
}
