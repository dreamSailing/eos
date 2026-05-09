package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


const (
	ToolStatusPending = "pending"
	ToolStatusSuccess = "success"
	ToolStatusError   = "error"
	ToolStatusRunning = "running"

	ToolTypeTool  = "tool_result"
	ToolTypeAgent = "agent_result"
)

type ToolDisplayStyle struct {
	Prefix     string
	Color      string
	StatusIcon string
	StatusText string
}

var toolStyles = map[string]ToolDisplayStyle{
	ToolStatusPending: {
		Prefix:     "▸",
		Color:      "yellow",
		StatusIcon: "⏳",
		StatusText: "执行中",
	},
	ToolStatusSuccess: {
		Prefix:     "✓",
		Color:      "green",
		StatusIcon: "",
		StatusText: "",
	},
	ToolStatusError: {
		Prefix:     "✗",
		Color:      "red",
		StatusIcon: "",
		StatusText: "失败",
	},
	ToolStatusRunning: {
		Prefix:     "▸",
		Color:      "cyan",
		StatusIcon: "⚙",
		StatusText: "运行中",
	},
}

func GetToolStyle(status string) ToolDisplayStyle {
	if style, ok := toolStyles[status]; ok {
		return style
	}
	return toolStyles[ToolStatusPending]
}

// FormatToolPlaceholder 格式化工具步骤的占位符显示
// 只显示步骤名称，不显示动态状态，避免状态不同步问题
func FormatToolPlaceholder(toolName, toolRest, status string) []string {
	style := GetToolStyle(ToolStatusPending)
	mainLine := style.Prefix + " " + toolName + toolRest
	return []string{"[" + style.Color + "]" + mainLine + "[-]"}
}

func FormatToolPlaceholderSimple(toolName, toolRest string) []string {
	return FormatToolPlaceholder(toolName, toolRest, ToolStatusPending)
}

func FormatToolHeader(toolName, toolRest string) string {
	return toolName + toolRest
}

func FormatToolResult(toolName, toolRest, status, summary string) []string {
	style := GetToolStyle(status)

	mainLine := style.Prefix + " " + toolName + toolRest

	if summary != "" {
		mainLine += " - " + summary
	}

	return []string{"[" + style.Color + "]" + mainLine + "[-]"}
}

func TruncateDisplay(display string, maxLen int) string {
	if len(display) <= maxLen {
		return display
	}
	return display[:maxLen-3] + "…"
}

func FormatToolSummary(result ToolResult) string {
	if result.Status == ToolStatusError {
		if result.Error != "" {
			return TruncateDisplay(result.Error, 80)
		}
		return "失败"
	}

	if result.Display != "" {
		return TruncateDisplay(result.Display, 80)
	}

	if data := result.Data; data != nil {
		if stdout, ok := data["stdout"].(string); ok && stdout != "" {
			return TruncateDisplay(stdout, 80)
		}
	}

	return "完成"
}
