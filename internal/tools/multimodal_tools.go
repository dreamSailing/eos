package tools

import (
	"context"
	"fmt"
	"strings"
)

func (m *Manager) imageGenerateStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	prompt := strings.TrimSpace(asString(params["prompt"]))
	if prompt == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolImageGenerate,
			Status:  "error",
			Error:   "prompt is required",
			Display: "错误：prompt 参数为必填项",
		}
	}
	return multimodalNotImplementedResult(ToolImageGenerate)
}

func (m *Manager) videoGenerateStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	prompt := strings.TrimSpace(asString(params["prompt"]))
	if prompt == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolVideoGenerate,
			Status:  "error",
			Error:   "prompt is required",
			Display: "错误：prompt 参数为必填项",
		}
	}
	return multimodalNotImplementedResult(ToolVideoGenerate)
}

func (m *Manager) speechSynthesizeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	text := strings.TrimSpace(asString(params["text"]))
	if text == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolSpeechSynthesize,
			Status:  "error",
			Error:   "text is required",
			Display: "错误：text 参数为必填项",
		}
	}
	return multimodalNotImplementedResult(ToolSpeechSynthesize)
}

func multimodalNotImplementedResult(toolName string) ToolResult {
	errMsg := fmt.Sprintf("%s is not implemented yet", toolName)
	return ToolResult{
		Type:    "tool_result",
		Tool:    toolName,
		Status:  "error",
		Error:   errMsg,
		Display: fmt.Sprintf("错误：工具 %s 暂未实现", toolName),
	}
}
