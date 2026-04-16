package tools

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

func (m *Manager) powerShellStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	cmd, _ := params["command"].(string)
	if strings.TrimSpace(cmd) == "" {
		return ToolResult{Type: "tool_result", Tool: ToolPowerShell, Status: "error", Error: "command required"}
	}

	// Determine the PowerShell executable
	psCmd := "pwsh"
	if runtime.GOOS == "windows" {
		psCmd = "powershell"
	}

	// Build the full command with PowerShell invocation
	fullCmd := fmt.Sprintf("%s -NoProfile -NonInteractive -Command %s", psCmd, escapePowerShellArg(cmd))

	out, err := m.shell.ExecuteWithWorkingDirCtx(ctx, fullCmd, WorkspaceRootFromContext(ctx))
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolPowerShell, Status: "error", Error: fmt.Sprintf("%v", err), Data: map[string]interface{}{"stdout": out}}
	}
	return ToolResult{Type: "tool_result", Tool: ToolPowerShell, Status: "success", Data: map[string]interface{}{"stdout": out, "continue": true}, Display: out}
}

// escapePowerShellArg escapes a command string for use as a PowerShell -Command argument
func escapePowerShellArg(s string) string {
	// Wrap in single quotes and escape existing single quotes
	s = strings.ReplaceAll(s, "'", "''")
	return "'" + s + "'"
}
