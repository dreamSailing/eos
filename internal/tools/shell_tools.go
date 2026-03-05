package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

func validateBashCommand(cmd string) error {
	c := strings.TrimSpace(cmd)
	if c == "" {
		return fmt.Errorf("command required")
	}
	if strings.ContainsAny(c, "\r\n") {
		return fmt.Errorf("multi-line command not allowed")
	}
	if strings.Contains(c, "`") {
		return fmt.Errorf("dangerous shell constructs not allowed")
	}
	if strings.Contains(c, "$(") {
		return fmt.Errorf("subexpressions not allowed")
	}
	lc := strings.ToLower(c)
	if strings.Contains(lc, "invoke-expression") || strings.Contains(lc, "iex ") || lc == "iex" {
		return fmt.Errorf("command not allowed")
	}
	return nil
}

func (m *Manager) bashStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	cmd, _ := params["command"].(string)
	if err := validateBashCommand(cmd); err != nil {
		return ToolResult{Type: "tool_result", Tool: "bash", Status: "error", Error: err.Error()}
	}
	out, err := m.shell.ExecuteCtx(ctx, cmd)
	if err != nil {
		slog.Error("bash.error", "component", utils.ComponentTool, "cmd", cmd, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "bash", Status: "error", Error: fmt.Sprintf("%v", err), Data: map[string]interface{}{"stdout": out}}
	}
	return ToolResult{Type: "tool_result", Tool: "bash", Status: "success", Data: map[string]interface{}{"stdout": out, "continue": true}, Display: out}
}

// bashSessionStructured 统一的 bash 会话工具入口
func (m *Manager) bashSessionStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	mode, _ := params["mode"].(string)
	if mode == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    "bash_session",
			Status:  "error",
			Error:   "mode parameter is required (valid: start, output, kill)",
			Display: "Error: mode parameter is required (valid: start, output, kill)",
		}
	}

	switch mode {
	case "start":
		return m.bashSessionStart(params)
	case "output":
		return m.bashSessionOutput(params)
	case "kill":
		return m.bashSessionKill(params)
	default:
		return ToolResult{
			Type:    "tool_result",
			Tool:    "bash_session",
			Status:  "error",
			Error:   fmt.Sprintf("unknown mode: %s (valid: start, output, kill)", mode),
			Display: fmt.Sprintf("Error: unknown mode '%s'", mode),
		}
	}
}

// bashSessionStart 启动后台 shell 会话
func (m *Manager) bashSessionStart(params map[string]interface{}) ToolResult {
	cmd, _ := params["command"].(string)
	if err := validateBashCommand(cmd); err != nil {
		return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "error", Error: err.Error(), Display: "Error: " + err.Error()}
	}
	id, err := m.shell.StartAsync(cmd)
	if err != nil {
		slog.Error("bash_session.start.error", "component", utils.ComponentTool, "cmd", cmd, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "error", Error: fmt.Sprintf("%v", err), Display: fmt.Sprintf("Error: %v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "success", Data: map[string]interface{}{"id": id}, Display: "Started shell session: " + id}
}

// bashSessionOutput 获取后台会话输出
func (m *Manager) bashSessionOutput(params map[string]interface{}) ToolResult {
	id, _ := params["id"].(string)
	if id == "" {
		return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "error", Error: "id required", Display: "Error: id required"}
	}
	out, errOut, done, err := m.shell.Output(id)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "error", Error: err.Error(), Display: "Error: " + err.Error()}
	}
	st := map[string]interface{}{"stdout": out, "stderr": errOut, "done": done}
	return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "success", Data: st, Display: out}
}

// bashSessionKill 终止后台会话
func (m *Manager) bashSessionKill(params map[string]interface{}) ToolResult {
	id, _ := params["id"].(string)
	if id == "" {
		return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "error", Error: "id required", Display: "Error: id required"}
	}
	if err := m.shell.Kill(id); err != nil {
		slog.Error("bash_session.kill.error", "component", utils.ComponentTool, "id", id, "err", err.Error())
		return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "error", Error: fmt.Sprintf("%v", err), Display: fmt.Sprintf("Error: %v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "bash_session", Status: "success", Data: map[string]interface{}{"id": id}, Display: "Killed shell session: " + id}
}
