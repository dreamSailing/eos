package runtime

import (
	"encoding/json"
	"log/slog"

	"github.com/dreamSailing/eos/internal/tools"
)

// EventData 结构化的事件数据
type EventData struct {
	Type    string                 `json:"type"`
	ID      string                 `json:"id,omitempty"`
	Tool    string                 `json:"tool,omitempty"`
	Status  string                 `json:"status,omitempty"`
	Display string                 `json:"display,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Content string                 `json:"content,omitempty"`
}

// EmitToolCallJSON 发送工具调用事件（JSON格式）
func (e *EventEmitter) EmitToolCallJSON(toolID, toolName string, params map[string]any) {
	if e.onMeta == nil {
		return
	}

	event := EventData{
		Type: "tool.call",
		ID:   toolID,
		Tool: toolName,
		Data: params,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		slog.Error("runtime.emit_tool_call_json.marshal_error", "error", err)
		return
	}

	e.onMeta(string(jsonData))
}

// EmitToolResultJSON 发送工具结果事件（JSON格式）
func (e *EventEmitter) EmitToolResultJSON(result tools.ToolResult) {
	if e.onMeta == nil {
		return
	}

	// 子 Agent 工具不发送工具结果事件
	if result.Type == tools.ToolTypeAgent {
		slog.Debug("runtime.emit_tool_result_json.skip_agent", "tool", result.Tool)
		return
	}

	event := EventData{
		Type:    "tool.result",
		ID:      result.ID,
		Tool:    result.Tool,
		Status:  result.Status,
		Display: result.Display,
		Error:   result.Error,
		Data:    result.Data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		slog.Error("runtime.emit_tool_result_json.marshal_error", "error", err)
		return
	}

	e.onMeta(string(jsonData))
}
