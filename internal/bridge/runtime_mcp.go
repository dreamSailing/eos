package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/mcp"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// ListMCPServers 列出所有 MCP 服务器
func (rc *RuntimeCore) ListMCPServers() []config.MCPEntry {
	cfg, _ := config.Load()
	return pluginpkg.MergeMCPEntries(&cfg, rc.workingRoot())
}

func (rc *RuntimeCore) BrowserStatus() mcp.BrowserStatus {
	cfg, _ := config.Load()
	cfg.MCP = pluginpkg.MergeMCPEntries(&cfg, rc.workingRoot())
	return mcp.DetectBrowserStatus(&cfg, rc.mcpMgr)
}

// UpdateMCPServer 更新 MCP 服务器
func (rc *RuntimeCore) UpdateMCPServer(entry config.MCPEntry) bool {
	cfg, p := config.Load()
	if !config.UpdateMCPServer(&cfg, entry) {
		return false
	}
	if err := config.Save(cfg, p); err != nil {
		slog.Error("mcp.update.save.error", "component", utils.ComponentSystem, "error", err.Error())
		return false
	}
	return true
}

// DeleteMCPServer 删除 MCP 服务器
func (rc *RuntimeCore) DeleteMCPServer(name string) bool {
	cfg, p := config.Load()
	if !config.DeleteMCPServer(&cfg, name) {
		return false
	}
	if err := config.Save(cfg, p); err != nil {
		slog.Error("mcp.delete.save.error", "component", utils.ComponentSystem, "error", err.Error())
		return false
	}
	return true
}

// ToggleMCPServer 切换 MCP 服务器状态
func (rc *RuntimeCore) ToggleMCPServer(name string) bool {
	cfg, p := config.Load()
	if !config.ToggleMCPServer(&cfg, name) {
		return false
	}
	if err := config.Save(cfg, p); err != nil {
		slog.Error("mcp.toggle.save.error", "component", utils.ComponentSystem, "error", err.Error())
		return false
	}
	return true
}

// AddMCPServers 批量添加 MCP 服务器
func (rc *RuntimeCore) AddMCPServers(entries []config.MCPEntry) error {
	if len(entries) == 0 {
		return fmt.Errorf("empty config")
	}
	cfg, p := config.Load()
	existing := make(map[string]struct{}, len(cfg.MCP))
	for _, e := range cfg.MCP {
		existing[e.Name] = struct{}{}
	}
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			return fmt.Errorf("missing server name")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate server name: %s", name)
		}
		if _, ok := existing[name]; ok {
			return fmt.Errorf("server already exists: %s", name)
		}
		seen[name] = struct{}{}
		if !config.AddMCPServer(&cfg, e) {
			return fmt.Errorf("failed to add server: %s", name)
		}
	}
	return config.Save(cfg, p)
}
