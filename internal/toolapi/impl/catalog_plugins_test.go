package impl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/tools"
)

func TestCatalogListIncludesManifestPluginCapability(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	workspace := t.TempDir()
	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "commands"), 0o755); err != nil {
		t.Fatalf("mkdir commands: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format project files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "hooks", "hooks.json"), []byte(`{"hooks":{"PostToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"echo format"}]}]}}`), 0o644); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}

	defs, err := newCatalog().List(tools.WithWorkspaceRoot(context.Background(), workspace))
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	def, ok := toolapi.FindToolDefinition(defs, "formatter")
	if !ok {
		t.Fatalf("missing manifest plugin capability")
	}
	if def.Source != toolapi.SourcePlugin {
		t.Fatalf("source=%q, want %q", def.Source, toolapi.SourcePlugin)
	}
	if def.Invocable {
		t.Fatalf("manifest plugin should be capability-only")
	}
	if got := def.Metadata["origin"]; got != "plugin_manifest" {
		t.Fatalf("origin=%v, want plugin_manifest", got)
	}
	components, _ := def.Metadata["components"].([]string)
	if len(components) != 2 {
		t.Fatalf("components=%v, want 2 entries", def.Metadata["components"])
	}
}

func TestCatalogListIncludesPluginMCPCapability(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	workspace := t.TempDir()
	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format project files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".mcp.json"), []byte(`{"mcpServers":{"plugin-db":{"command":"db-server","args":["--root","${CLAUDE_PLUGIN_ROOT}"]}}}`), 0o644); err != nil {
		t.Fatalf("write .mcp.json: %v", err)
	}

	defs, err := newCatalog().List(tools.WithWorkspaceRoot(context.Background(), workspace))
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	def, ok := toolapi.FindToolDefinition(defs, "mcp:plugin-db")
	if !ok {
		t.Fatalf("missing plugin MCP capability")
	}
	if def.Source != toolapi.SourceMCP {
		t.Fatalf("source=%q, want %q", def.Source, toolapi.SourceMCP)
	}
	if got := def.Metadata["command"]; got != "db-server" {
		t.Fatalf("command=%v, want db-server", got)
	}
}
