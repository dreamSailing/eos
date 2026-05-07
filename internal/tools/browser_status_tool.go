package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
	mcppkg "github.com/dreamSailing/eos/internal/mcp"
	plugpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
)

func (m *Manager) browserStatusStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	_ = params

	cfg, _ := config.Load()
	cfg.MCP = plugpkg.MergeMCPEntries(&cfg, workspaceRootOrPWD(ctx))
	status := mcppkg.DetectBrowserStatus(&cfg, m.mcpManager)

	lines := []string{
		fmt.Sprintf("server=%s", strings.TrimSpace(status.ServerName)),
		fmt.Sprintf("configured=%t", status.Configured),
		fmt.Sprintf("enabled=%t", status.Enabled),
		fmt.Sprintf("loaded=%t", status.Loaded),
	}
	if status.Tools > 0 {
		lines = append(lines, fmt.Sprintf("tools=%d", status.Tools))
	}
	if strings.TrimSpace(status.LastError) != "" {
		lines = append(lines, "last_error="+strings.TrimSpace(status.LastError))
	}
	if strings.TrimSpace(status.InstallHint) != "" {
		lines = append(lines, "hint="+strings.TrimSpace(status.InstallHint))
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolBrowserStatus,
		Status: "success",
		Data: map[string]interface{}{
			"server_name":  status.ServerName,
			"configured":   status.Configured,
			"enabled":      status.Enabled,
			"loaded":       status.Loaded,
			"tools":        status.Tools,
			"last_error":   status.LastError,
			"command":      status.Command,
			"install_hint": status.InstallHint,
		},
		Display: strings.Join(lines, "\n"),
	}
}
