package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"strings"

	"github.com/dreamSailing/eos/internal/tools/shell"
)

func (m *Manager) powerShellStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	cmd, _ := params["command"].(string)
	if strings.TrimSpace(cmd) == "" {
		return ToolResult{Type: "tool_result", Tool: ToolPowerShell, Status: "error", Error: "command required"}
	}

	out, err := m.shell.ExecuteTypedWithWorkingDirCtx(ctx, shell.ShellTypePowerShell, cmd, WorkspaceRootFromContext(ctx))
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolPowerShell, Status: "error", Error: fmt.Sprintf("%v", err), Data: map[string]interface{}{"stdout": out}}
	}
	return ToolResult{Type: "tool_result", Tool: ToolPowerShell, Status: "success", Data: map[string]interface{}{"stdout": out, "continue": true}, Display: out}
}
