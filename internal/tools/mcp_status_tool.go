package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func (m *Manager) mcpStatusStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	_ = ctx
	name, _ := params["name"].(string)
	name = strings.TrimSpace(name)

	if m.mcpManager == nil {
		return ToolResult{Type: "tool_result", Tool: ToolMCPStatus, Status: "error", Error: "MCP manager not initialized"}
	}

	cfg, _ := config.Load()
	statuses := m.mcpManager.GetServerStatuses(&cfg)
	servers := make([]map[string]any, 0, len(statuses))
	for _, s := range statuses {
		if name != "" && !strings.EqualFold(s.Name, name) {
			continue
		}
		servers = append(servers, map[string]any{
			"name":       s.Name,
			"enabled":    s.Enabled,
			"loaded":     s.Loaded,
			"tools":      s.Tools,
			"last_error": s.LastError,
		})
	}

	if name != "" {
		if len(servers) == 0 {
			return ToolResult{
				Type:   "tool_result",
				Tool:   ToolMCPStatus,
				Status: "success",
				Data: map[string]any{
					"servers": []map[string]any{},
				},
				Display: "No matching MCP server: " + name,
			}
		}
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolMCPStatus,
			Status: "success",
			Data: map[string]any{
				"servers": servers,
			},
			Display: formatMCPStatusDisplay(servers),
		}
	}

	sort.Slice(servers, func(i, j int) bool {
		return strings.ToLower(servers[i]["name"].(string)) < strings.ToLower(servers[j]["name"].(string))
	})

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolMCPStatus,
		Status: "success",
		Data: map[string]any{
			"servers": servers,
		},
		Display: formatMCPStatusDisplay(servers),
	}
}

func formatMCPStatusDisplay(servers []map[string]any) string {
	lines := make([]string, 0, len(servers)+1)
	lines = append(lines, "MCP status:")
	for _, v := range servers {
		name, _ := v["name"].(string)
		enabled, _ := v["enabled"].(bool)
		loaded, _ := v["loaded"].(bool)
		tools, _ := v["tools"].(int)
		lastErr, _ := v["last_error"].(string)
		line := fmt.Sprintf("- %s | enabled=%v | loaded=%v | tools=%d", name, enabled, loaded, tools)
		if strings.TrimSpace(lastErr) != "" {
			line += " | error=" + strings.TrimSpace(lastErr)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m *Manager) skillsListStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	_ = params
	if m.skillManager == nil {
		return ToolResult{Type: "tool_result", Tool: ToolSkillsList, Status: "error", Error: "skill manager not initialized"}
	}

	stats := m.skillManager.GetStats()
	var names []string
	if v, ok := stats["names"].([]string); ok {
		names = v
	}
	sort.Strings(names)

	display := "Skills:\n"
	if dirs, ok := stats["scan_dirs"].([]string); ok && len(dirs) > 0 {
		display += "scan_dirs:\n"
		for _, d := range dirs {
			display += "- " + d + "\n"
		}
	}
	if len(names) == 0 {
		display += "(none)"
	} else {
		display += "names:\n"
		for _, n := range names {
			display += "- " + n + "\n"
		}
	}

	slog.Debug("tools.skills_list", "component", utils.ComponentTool, "count", len(names))
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolSkillsList,
		Status:  "success",
		Data:    stats,
		Display: strings.TrimSpace(display),
	}
}
