package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"log/slog"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// 统一的日志键名
const (
	LogKeyTool    = "tool"
	LogKeyStatus  = "status"
	LogKeyReplyID = "reply_id"
	LogKeyLength  = "length"
	LogKeyContent = "content"
	LogKeyError   = "error"
	LogKeyEvent   = "event"
	LogKeyRole    = "role"
	LogKeyTask    = "task"
	LogKeyCount   = "count"
)

// LogToolCall 记录工具调用
func LogToolCall(toolName string) {
	slog.Debug("runtime.tool.call", "component", utils.ComponentTool, LogKeyTool, toolName)
}

// LogToolResult 记录工具结果
func LogToolResult(toolName, status string, contentLength int) {
	slog.Debug("runtime.tool.result", "component", utils.ComponentTool,
		LogKeyTool, toolName,
		LogKeyStatus, status,
		LogKeyLength, contentLength,
	)
}

// LogAgentInvoke 记录代理调用
func LogAgentInvoke(role, task string) {
	slog.Debug("runtime.agent.invoke", "component", utils.ComponentAgent,
		LogKeyRole, role,
		LogKeyTask, task,
	)
}

// LogEmitEvent 记录事件发送
func LogEmitEvent(eventType, preview string) {
	slog.Debug("runtime.emit.event", "component", utils.ComponentSystem,
		LogKeyEvent, eventType,
		LogKeyContent, preview,
	)
}

// LogWarn 记录警告
func LogWarn(msg string, args ...any) {
	slog.Warn(msg, append([]any{"component", utils.ComponentSystem}, args...)...)
}

// LogError 记录错误
func LogError(msg string, args ...any) {
	slog.Error(msg, append([]any{"component", utils.ComponentSystem}, args...)...)
}

// LogDebug 记录调试信息
func LogDebug(msg string, args ...any) {
	slog.Debug(msg, append([]any{"component", utils.ComponentSystem}, args...)...)
}
