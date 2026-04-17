package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/config"
)

func TestMergeMCPEntriesIncludesEnabledPluginServers(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	writePluginManifest(t, pluginRoot, "formatter", "formatter plugin")
	raw := `{
  "mcpServers": {
    "plugin-db": {
      "command": "${CLAUDE_PLUGIN_ROOT}/bin/server",
      "args": ["--data", "${CLAUDE_PLUGIN_DATA}"],
      "env": {
        "PLUGIN_ROOT": "${CLAUDE_PLUGIN_ROOT}"
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(pluginRoot, ".mcp.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write .mcp.json: %v", err)
	}

	entries := MergeMCPEntries(&config.Config{}, workspace)
	if len(entries) != 1 {
		t.Fatalf("len(MergeMCPEntries())=%d, want 1", len(entries))
	}
	if got := entries[0].Command; !strings.Contains(filepath.ToSlash(got), filepath.ToSlash(filepath.Join(pluginRoot, "bin", "server"))) {
		t.Fatalf("Command=%q, want substituted plugin root", got)
	}
	if len(entries[0].Args) != 2 || entries[0].Args[0] != "--data" || !strings.Contains(filepath.ToSlash(entries[0].Args[1]), filepath.ToSlash(PersistentDataDir("formatter"))) {
		t.Fatalf("Args=%v, want substituted plugin data", entries[0].Args)
	}
	if got := entries[0].Envs["PLUGIN_ROOT"]; filepath.ToSlash(got) != filepath.ToSlash(pluginRoot) {
		t.Fatalf("PLUGIN_ROOT=%q, want %q", got, pluginRoot)
	}
}

func TestMergeMCPEntriesSkipsDisabledPluginServers(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	writePluginManifest(t, pluginRoot, "formatter", "formatter plugin")
	if err := os.WriteFile(filepath.Join(pluginRoot, ".mcp.json"), []byte(`{"mcpServers":{"plugin-db":{"command":"db-server"}}}`), 0o644); err != nil {
		t.Fatalf("write .mcp.json: %v", err)
	}

	cfg := config.Config{}
	config.SetPluginEnabled(&cfg, "formatter", false)
	entries := MergeMCPEntries(&cfg, workspace)
	if len(entries) != 0 {
		t.Fatalf("MergeMCPEntries()=%v, want no plugin MCP entries", entries)
	}
}
