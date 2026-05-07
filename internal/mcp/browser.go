package mcp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"encoding/json"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
)

const DefaultBrowserServerName = "playwright"

type BrowserStatus struct {
	ServerName  string `json:"server_name"`
	Configured  bool   `json:"configured"`
	Enabled     bool   `json:"enabled"`
	Loaded      bool   `json:"loaded"`
	Tools       int    `json:"tools"`
	LastError   string `json:"last_error,omitempty"`
	Command     string `json:"command,omitempty"`
	InstallHint string `json:"install_hint,omitempty"`
}

func RecommendedBrowserPreset() config.MCPEntry {
	return config.MCPEntry{
		Name:    DefaultBrowserServerName,
		Type:    config.MCPTypeStdio,
		Command: "npx",
		Args:    []string{"-y", "@playwright/mcp@latest"},
		Envs:    map[string]string{},
		Enabled: true,
	}
}

func RecommendedBrowserPresetJSON() string {
	b, err := json.MarshalIndent([]config.MCPEntry{RecommendedBrowserPreset()}, "", "  ")
	if err != nil {
		return `[
  {
    "name": "playwright",
    "type": "stdio",
    "command": "npx",
    "args": ["-y", "@playwright/mcp@latest"],
    "envs": {},
    "enabled": true
  }
]`
	}
	return string(b)
}

func DetectBrowserStatus(cfg *config.Config, mgr *Manager) BrowserStatus {
	status := BrowserStatus{
		ServerName:  DefaultBrowserServerName,
		InstallHint: "在 /mcp 中添加 Playwright 预设，或配置 `npx -y @playwright/mcp@latest`。",
	}
	if cfg == nil {
		return status
	}

	var entry *config.MCPEntry
	for _, item := range cfg.MCP {
		if !IsBrowserServerEntry(item) {
			continue
		}
		candidate := item
		entry = &candidate
		break
	}
	if entry == nil {
		return status
	}

	status.ServerName = strings.TrimSpace(entry.Name)
	status.Configured = true
	status.Enabled = entry.Enabled
	status.Command = strings.TrimSpace(entry.Command)

	if !entry.Enabled {
		status.InstallHint = "浏览器 MCP 已配置但未启用，请在 /mcp 中启用它。"
		return status
	}

	status.InstallHint = "浏览器 MCP 已启用。若加载失败，请检查 Node.js、npx 以及 Playwright 依赖是否可用。"
	if mgr == nil {
		return status
	}

	for _, srv := range mgr.GetServerStatuses(cfg) {
		if !strings.EqualFold(strings.TrimSpace(srv.Name), status.ServerName) {
			continue
		}
		status.Loaded = srv.Loaded
		status.Tools = srv.Tools
		status.LastError = strings.TrimSpace(srv.LastError)
		if status.Loaded {
			status.InstallHint = "浏览器 MCP 已可用，网页交互任务可直接调用其工具。"
		} else if status.LastError != "" {
			status.InstallHint = "浏览器 MCP 加载失败，请检查 Node.js、npx、网络和 Playwright 首次安装依赖。"
		}
		break
	}

	return status
}

func IsBrowserServerEntry(entry config.MCPEntry) bool {
	fields := []string{
		entry.Name,
		entry.Command,
		entry.BaseURL,
	}
	fields = append(fields, entry.Args...)
	for _, item := range fields {
		s := strings.ToLower(strings.TrimSpace(item))
		if s == "" {
			continue
		}
		if strings.Contains(s, "playwright") || strings.Contains(s, "@playwright/mcp") {
			return true
		}
	}
	return false
}
