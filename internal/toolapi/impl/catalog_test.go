package impl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dreamSailing/vb-coding/internal/config"
	pluginpkg "github.com/dreamSailing/vb-coding/internal/pkg/plugins"
	"github.com/dreamSailing/vb-coding/internal/toolapi"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

type testPlugin struct {
	name string
	desc string
}

func (p *testPlugin) Name() string        { return p.name }
func (p *testPlugin) Description() string { return p.desc }
func (p *testPlugin) Execute(_ context.Context, _ map[string]any) (any, error) {
	return map[string]any{"ok": true}, nil
}

func TestCatalogListIncludesUnifiedCapabilities(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginpkg.DefaultRegistry().Reset()
	t.Cleanup(func() { pluginpkg.DefaultRegistry().Reset() })
	pluginpkg.DefaultRegistry().Register(&testPlugin{name: "echo_plugin", desc: "echo plugin"})

	cfg := config.Config{
		MCP: []config.MCPEntry{
			{Name: "demo", Type: config.MCPTypeStdio, Command: "demo-mcp", Enabled: true},
		},
	}
	if err := config.Save(cfg, config.Path()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	skillDir := filepath.Join(home, ".vb", "skills", "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := "---\nname: review\ndescription: code review helper\n---\n\nbody"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	workspace := t.TempDir()
	defs, err := newCatalog().List(tools.WithWorkspaceRoot(context.Background(), workspace))
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}

	check := func(name string, source toolapi.CapabilitySource, invocable bool) {
		t.Helper()
		def, ok := toolapi.FindToolDefinition(defs, name)
		if !ok {
			t.Fatalf("missing capability %q", name)
		}
		if def.Source != source {
			t.Fatalf("capability %q source=%q want=%q", name, def.Source, source)
		}
		if def.Invocable != invocable {
			t.Fatalf("capability %q invocable=%v want=%v", name, def.Invocable, invocable)
		}
	}

	check("read", toolapi.SourceBuiltin, true)
	check("duckduckgo_search", toolapi.SourceRuntime, false)
	check("spawn_agent", toolapi.SourceAgent, false)
	check("skill:review", toolapi.SourceSkill, false)
	check("echo_plugin", toolapi.SourcePlugin, true)
	check("mcp:demo", toolapi.SourceMCP, false)
	check("lsp", toolapi.SourceLSP, false)
}
