//go:build legacy

package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/config"
)

func TestRuntimeImportMCPJSONNewConfig(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()

	raw := `{
		"mcp": [
			{
				"name": "filesystem",
				"type": "stdio",
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-filesystem", "C:/repo"],
				"env": {"TOKEN": "secret"}
			},
			{
				"name": "remote",
				"type": "streamable-http",
				"base_url": "https://example.com/mcp",
				"enabled": false,
				"auth": {"type": "bearer", "token": "abc"}
			}
		]
	}`
	if err := rt.ImportMCPJSON(raw); err != nil {
		t.Fatalf("ImportMCPJSON() error = %v", err)
	}

	cfg, _ := config.Load()
	if len(cfg.MCP) != 2 {
		t.Fatalf("len(cfg.MCP)=%d, want 2", len(cfg.MCP))
	}
	if got := cfg.MCP[0]; got.Name != "filesystem" || got.Type != config.MCPTypeStdio || got.Command != "npx" || !got.Enabled {
		t.Fatalf("filesystem entry = %#v", got)
	}
	if got := cfg.MCP[0].Envs["TOKEN"]; got != "secret" {
		t.Fatalf("env TOKEN=%q, want secret", got)
	}
	if got := cfg.MCP[1]; got.Name != "remote" || got.Type != config.MCPTypeStreamableHTTP || got.BaseURL != "https://example.com/mcp" || got.Enabled {
		t.Fatalf("remote entry = %#v", got)
	}
	if cfg.MCP[1].Auth == nil || cfg.MCP[1].Auth.Token != "abc" {
		t.Fatalf("remote auth = %#v", cfg.MCP[1].Auth)
	}
}

func TestRuntimeImportMCPJSONDirectArrayAndLegacyServers(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()

	if err := rt.ImportMCPJSON(`[{"name":"direct","command":"mcp-direct"}]`); err != nil {
		t.Fatalf("ImportMCPJSON(direct array) error = %v", err)
	}
	if err := rt.ImportMCPJSON(`{
		"mcpServers": {
			"legacy": {
				"command": "mcp-legacy",
				"args": ["--port", "3333"],
				"env": {"LEGACY_TOKEN": "token"}
			},
			"legacy-sse": {
				"url": "http://127.0.0.1:8080/sse"
			}
		}
	}`); err != nil {
		t.Fatalf("ImportMCPJSON(legacy) error = %v", err)
	}

	cfg, _ := config.Load()
	if len(cfg.MCP) != 3 {
		t.Fatalf("len(cfg.MCP)=%d, want 3", len(cfg.MCP))
	}
	if got := mcpEntryByName(cfg.MCP, "direct"); got.Command != "mcp-direct" || !got.Enabled {
		t.Fatalf("direct entry = %#v", got)
	}
	if got := mcpEntryByName(cfg.MCP, "legacy"); got.Command != "mcp-legacy" || got.Envs["LEGACY_TOKEN"] != "token" {
		t.Fatalf("legacy entry = %#v", got)
	}
	if got := mcpEntryByName(cfg.MCP, "legacy-sse"); got.Type != config.MCPTypeSSE || got.BaseURL != "http://127.0.0.1:8080/sse" {
		t.Fatalf("legacy-sse entry = %#v", got)
	}
}

func TestRuntimeImportMCPJSONValidation(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "invalid json", raw: `{`, want: "invalid JSON"},
		{name: "empty config", raw: `{"mcp":[]}`, want: "empty config"},
		{name: "missing name", raw: `[{"command":"mcp"}]`, want: "missing server name"},
		{name: "duplicate name", raw: `[{"name":"dup","command":"a"},{"name":"dup","command":"b"}]`, want: "duplicate server name: dup"},
		{name: "missing command", raw: `[{"name":"local","type":"stdio"}]`, want: "missing command for server: local"},
		{name: "missing base url", raw: `[{"name":"remote","type":"streamable-http"}]`, want: "missing base_url for server: remote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configureCoreWorkspaceTestEnv(t)
			rt := NewRuntime()
			err := rt.ImportMCPJSON(tt.raw)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ImportMCPJSON() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestRuntimeImportMCPJSONRejectsExistingName(t *testing.T) {
	configureCoreWorkspaceTestEnv(t)
	writeCoreModelConfig(t, config.Config{
		MCP: []config.MCPEntry{
			{Name: "existing", Type: config.MCPTypeStdio, Command: "mcp-existing", Enabled: true},
		},
	})
	rt := NewRuntime()

	err := rt.ImportMCPJSON(`[{"name":"existing","command":"mcp-next"}]`)
	if err == nil || !strings.Contains(err.Error(), "server already exists: existing") {
		t.Fatalf("ImportMCPJSON() error = %v, want existing-name error", err)
	}
}

func mcpEntryByName(entries []config.MCPEntry, name string) config.MCPEntry {
	for _, entry := range entries {
		if entry.Name == name {
			return entry
		}
	}
	return config.MCPEntry{}
}
