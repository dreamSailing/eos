package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
	"github.com/dreamSailing/vb-coding/internal/tools/bg"
)

func (m *Manager) bgTaskStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	action, _ := params["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: "action parameter is required", Display: "Error: action parameter is required"}
	}

	switch action {
	case "start":
		command, _ := params["command"].(string)
		if err := validateBashCommand(command); err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "Error: " + err.Error()}
		}
		wd, _ := params["working_dir"].(string)
		var env []string
		if raw, ok := params["env"].([]interface{}); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					env = append(env, s)
				}
			}
		}
		logCap := 2000
		if v, ok := params["log_cap"].(float64); ok && int(v) > 0 {
			logCap = int(v)
		}
		info, err := bg.Default().Start(command, &bg.StartOptions{WorkingDir: wd, Env: env, LogCap: logCap})
		if err != nil {
			slog.Error("bg_task.start.error", "component", utils.ComponentTool, "cmd", command, "err", err.Error())
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "Error: " + err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"task": info}, Display: "Started task: " + info.ID}
	case "list":
		items := bg.Default().List()
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"tasks": items}}
	case "cleanup":
		n := bg.Default().CleanupFinished()
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"cleaned": n}, Display: fmt.Sprintf("Cleaned %d finished tasks", n)}
	case "info":
		id, _ := params["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: "id required", Display: "Error: id required"}
		}
		info, err := bg.Default().Info(id)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "Error: " + err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"task": info}}
	case "tail":
		id, _ := params["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: "id required", Display: "Error: id required"}
		}
		fromSeq := int64(0)
		if v, ok := params["from_seq"].(float64); ok {
			fromSeq = int64(v)
		} else if v, ok := params["from_seq"].(string); ok && strings.TrimSpace(v) != "" {
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				fromSeq = n
			}
		}
		limit := 200
		if v, ok := params["limit"].(float64); ok && int(v) > 0 {
			limit = int(v)
		}
		res, err := bg.Default().Tail(id, &bg.TailOptions{FromSeq: fromSeq, Limit: limit})
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "Error: " + err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"tail": res}}
	case "kill":
		id, _ := params["id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: "id required", Display: "Error: id required"}
		}
		info, err := bg.Default().Kill(id)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: err.Error(), Display: "Error: " + err.Error()}
		}
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "success", Data: map[string]any{"task": info}, Display: "Killed task: " + id}
	default:
		return ToolResult{Type: "tool_result", Tool: ToolBGTask, Status: "error", Error: fmt.Sprintf("unknown action: %s", action), Display: "Error: unknown action " + action}
	}
}

