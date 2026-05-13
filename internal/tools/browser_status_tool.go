package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"strings"

	"github.com/dreamSailing/eos/internal/browser"
	"github.com/dreamSailing/eos/internal/config"
	mcppkg "github.com/dreamSailing/eos/internal/mcp"
	plugpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
)

func (m *Manager) browserStatusStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	_ = params

	cfg, _ := config.Load()
	cfg.MCP = plugpkg.MergeMCPEntries(&cfg, workspaceRootOrPWD(ctx))
	status := mcppkg.DetectBrowserStatus(&cfg, m.mcpManager)
	traceID := strings.TrimSpace(TraceIDFromContext(ctx))

	builtinReady := false
	builtinLastError := ""
	builtinCaps := []string(nil)
	sessionCount := 0
	var currentSession map[string]interface{}
	if m.browserRT != nil {
		rtStatus := m.browserRT.Status()
		builtinReady = rtStatus.Ready
		builtinLastError = strings.TrimSpace(rtStatus.LastError)
		builtinCaps = append([]string(nil), rtStatus.Capabilities...)
		snapshots := m.browserRT.SessionSnapshots()
		sessionCount = len(snapshots)
		if traceID != "" {
			for _, snap := range snapshots {
				if snap.TraceID != traceID {
					continue
				}
				currentSession = map[string]interface{}{
					"trace_id":   snap.TraceID,
					"active_tab": snap.ActiveTab,
					"tab_count":  snap.TabCount,
					"tabs":       snap.Tabs,
				}
				break
			}
		}
	}

	lines := []string{
		fmt.Sprintf("builtin_ready=%t", builtinReady),
		fmt.Sprintf("session_count=%d", sessionCount),
		fmt.Sprintf("server=%s", strings.TrimSpace(status.ServerName)),
		fmt.Sprintf("configured=%t", status.Configured),
		fmt.Sprintf("enabled=%t", status.Enabled),
		fmt.Sprintf("loaded=%t", status.Loaded),
	}
	if len(builtinCaps) > 0 {
		lines = append(lines, "builtin_capabilities="+strings.Join(builtinCaps, ","))
	}
	if builtinLastError != "" {
		lines = append(lines, "builtin_last_error="+builtinLastError)
	}
	if currentSession != nil {
		lines = append(lines,
			"trace_id="+traceID,
			fmt.Sprintf("active_tab=%v", currentSession["active_tab"]),
			fmt.Sprintf("tab_count=%v", currentSession["tab_count"]),
		)
		if tabs, ok := currentSession["tabs"].([]browser.TabInfo); ok && len(tabs) > 0 {
			for _, tab := range tabs {
				marker := "-"
				if tab.Active {
					marker = "*"
				}
				lines = append(lines, fmt.Sprintf("%s [%d] %s %s %s", marker, tab.Index, tab.ID, strings.TrimSpace(tab.Title), strings.TrimSpace(tab.URL)))
			}
		}
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
			"builtin_ready":        builtinReady,
			"builtin_last_error":   builtinLastError,
			"builtin_capabilities": builtinCaps,
			"session_count":        sessionCount,
			"current_session":      currentSession,
			"server_name":          status.ServerName,
			"configured":           status.Configured,
			"enabled":              status.Enabled,
			"loaded":               status.Loaded,
			"tools":                status.Tools,
			"last_error":           status.LastError,
			"command":              status.Command,
			"install_hint":         status.InstallHint,
		},
		Display: strings.Join(lines, "\n"),
	}
}
