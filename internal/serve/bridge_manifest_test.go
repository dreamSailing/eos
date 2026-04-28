package serve

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/config"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
)

func TestBuildBridgeManifestIncludesLaunchAndCatalogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginpkg.DefaultRegistry().Reset()
	t.Cleanup(func() { pluginpkg.DefaultRegistry().Reset() })
	pluginpkg.DefaultRegistry().Register(&serveTestPlugin{name: "echo_plugin", desc: "echo plugin"})

	cfg := config.Config{
		MCP: []config.MCPEntry{
			{Name: "demo", Type: config.MCPTypeStdio, Command: "demo-mcp", Enabled: true},
		},
	}
	if err := config.Save(cfg, config.Path()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	skillDir := filepath.Join(home, ".eos", "skills", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := "---\nname: review\ndescription: code review helper\n---\n\nbody"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	workspace := t.TempDir()
	manifest, err := BuildBridgeManifest(Options{
		Transport:             "stdio",
		DefaultWorkspacePath:  workspace,
		DefaultAllowedTools:   []string{"read", "skills_list", "mcp_status", "echo_plugin"},
		DefaultSandboxMode:    "full_access",
		RequireApprovalDigest: true,
	}, BridgeManifestOptions{
		LaunchCommand:       "eos",
		Services:            toolapiimpl.NewServices(),
		IncludeTools:        true,
		IncludeCapabilities: true,
	})
	if err != nil {
		t.Fatalf("BuildBridgeManifest() error = %v", err)
	}

	workspaceAbs, _ := filepath.Abs(workspace)
	if manifest.SchemaVersion != bridgeSchemaVersion {
		t.Fatalf("SchemaVersion=%q, want %q", manifest.SchemaVersion, bridgeSchemaVersion)
	}
	if manifest.ProtocolVersion != serveProtocolVersion {
		t.Fatalf("ProtocolVersion=%q, want %q", manifest.ProtocolVersion, serveProtocolVersion)
	}
	if manifest.Transport != "stdio" {
		t.Fatalf("Transport=%q, want stdio", manifest.Transport)
	}
	if manifest.Launch.Command != "eos" {
		t.Fatalf("Launch.Command=%q, want eos", manifest.Launch.Command)
	}
	if manifest.Launch.Cwd != workspaceAbs {
		t.Fatalf("Launch.Cwd=%q, want %q", manifest.Launch.Cwd, workspaceAbs)
	}
	if manifest.SessionDefaults.WorkspacePath != workspaceAbs {
		t.Fatalf("WorkspacePath=%q, want %q", manifest.SessionDefaults.WorkspacePath, workspaceAbs)
	}
	if !manifest.SessionDefaults.RequireApprovalDigest {
		t.Fatal("RequireApprovalDigest should be true")
	}
	if manifest.SessionDefaults.ExecutionMode != "auto" {
		t.Fatalf("ExecutionMode=%q, want auto", manifest.SessionDefaults.ExecutionMode)
	}
	if manifest.SessionDefaults.SandboxMode != "full_access" {
		t.Fatalf("SandboxMode=%q, want full_access", manifest.SessionDefaults.SandboxMode)
	}
	if len(manifest.ExecutionModes) == 0 {
		t.Fatalf("ExecutionModes should not be empty")
	}
	if !manifest.ServerCapabilities.CapabilityCatalog || !manifest.ServerCapabilities.Tasks || !manifest.ServerCapabilities.Sessions {
		t.Fatalf("ServerCapabilities=%+v, want tasks/sessions/capabilityCatalog enabled", manifest.ServerCapabilities)
	}
	for _, method := range []string{"capability.list", "session.resume", "task.resume", "task.close"} {
		if !slicesContain(manifest.Methods, method) {
			t.Fatalf("Methods=%v, want %q", manifest.Methods, method)
		}
	}

	argsJoined := strings.Join(manifest.Launch.Args, " ")
	for _, part := range []string{"serve", "--transport", "--workspace", workspaceAbs, "--allowed-tools"} {
		if !strings.Contains(argsJoined, part) {
			t.Fatalf("Launch.Args=%v, missing %q", manifest.Launch.Args, part)
		}
	}

	if findToolDefinition(manifest.Tools, "read") == nil {
		t.Fatalf("manifest tools missing read: %+v", manifest.Tools)
	}
	if findToolDefinition(manifest.Tools, "echo_plugin") == nil {
		t.Fatalf("manifest tools missing echo_plugin: %+v", manifest.Tools)
	}
	if findToolDefinition(manifest.Tools, "skill:review") != nil {
		t.Fatalf("skill capability should not appear in executable tools: %+v", manifest.Tools)
	}
	if entry := findToolDefinition(manifest.Tools, "read"); entry == nil || entry.Access == nil || !entry.Access.Executable {
		t.Fatalf("manifest read tool should include executable access metadata: %+v", entry)
	}
	if findToolDefinition(manifest.Capabilities, "skill:review") == nil {
		t.Fatalf("manifest capabilities missing skill:review: %+v", manifest.Capabilities)
	}
	if findToolDefinition(manifest.Capabilities, "mcp:demo") == nil {
		t.Fatalf("manifest capabilities missing mcp:demo: %+v", manifest.Capabilities)
	}
	if findToolDefinition(manifest.Capabilities, "spawn_agent") == nil {
		t.Fatalf("manifest capabilities missing spawn_agent: %+v", manifest.Capabilities)
	} else if entry := findToolDefinition(manifest.Capabilities, "spawn_agent"); entry.Access == nil || entry.Access.Reason != "non_invocable" {
		t.Fatalf("spawn_agent should include non_invocable access metadata: %+v", entry)
	}
}

func TestServerCapabilitiesPayloadIncludesBridgeFields(t *testing.T) {
	payload := serverCapabilitiesPayload()
	for _, key := range []string{"events", "invoke", "tools", "confirmations", "sessions", "requests", "tasks", "capabilityCatalog"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("payload missing %q: %v", key, payload)
		}
	}
	if got, _ := payload["capabilityCatalog"].(bool); !got {
		t.Fatalf("capabilityCatalog=%v, want true", payload["capabilityCatalog"])
	}
}

func findToolDefinition(items []toolDefinitionDTO, name string) *toolDefinitionDTO {
	for i := range items {
		if items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func slicesContain(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
