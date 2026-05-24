package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dreamSailing/eos/internal/config"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/tools"
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

	skillDir := filepath.Join(home, ".eos", "skills", "review")
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

func TestInferCategoryFileGenerationTools(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{tools.ToolDocumentGenerate, "document"},
		{tools.ToolDocumentConvert, "document"},
		{tools.ToolNotebookEdit, "notebook"},
		{tools.ToolImageGenerate, "multimodal"},
		{tools.ToolVideoGenerate, "multimodal"},
		{tools.ToolSpeechSynthesize, "multimodal"},
		{tools.ToolBrowserScreenshot, "browser"},
		{tools.ToolBrowserNavigate, "browser"},
		{tools.ToolRead, "filesystem"},
		{tools.ToolBash, "shell"},
		{tools.ToolGitStatus, "git"},
		{tools.ToolWebSearch, "search"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferCategory(tt.name)
			if got != tt.expected {
				t.Fatalf("inferCategory(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestEnrichBuiltinToolMetadataFileGeneration(t *testing.T) {
	tests := []struct {
		name           string
		outputType     string
		sandboxGuarded bool
		pathParam      string
		formats        []string
	}{
		{tools.ToolDocumentGenerate, "document", true, "path", []string{"docx", "xlsx", "pdf"}},
		{tools.ToolDocumentConvert, "document", true, "destination_path", []string{"docx", "xlsx", "pdf"}},
		{tools.ToolNotebookEdit, "notebook", true, "path", []string{"ipynb"}},
		{tools.ToolImageGenerate, "image", true, "output_path", []string{"png", "jpg", "webp", "gif"}},
		{tools.ToolVideoGenerate, "video", true, "output_path", []string{"mp4", "webm", "mov"}},
		{tools.ToolSpeechSynthesize, "audio", true, "output_path", []string{"mp3", "wav", "flac", "aac", "ogg"}},
		{tools.ToolBrowserScreenshot, "image", true, "path", []string{"png"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := enrichBuiltinToolMetadata(tt.name)
			if meta == nil {
				t.Fatalf("enrichBuiltinToolMetadata(%q) returned nil", tt.name)
			}
			if got, _ := meta["output_type"].(string); got != tt.outputType {
				t.Fatalf("output_type = %q, want %q", got, tt.outputType)
			}
			if got, _ := meta["sandbox_guarded"].(bool); got != tt.sandboxGuarded {
				t.Fatalf("sandbox_guarded = %v, want %v", got, tt.sandboxGuarded)
			}
			if got, _ := meta["write_path_param"].(string); got != tt.pathParam {
				t.Fatalf("write_path_param = %q, want %q", got, tt.pathParam)
			}
			formats, _ := meta["formats"].([]string)
			if len(formats) != len(tt.formats) {
				t.Fatalf("formats length = %d, want %d", len(formats), len(tt.formats))
			}
		})
	}
}

func TestEnrichBuiltinToolMetadataNonFileGenReturnsNil(t *testing.T) {
	for _, name := range []string{tools.ToolRead, tools.ToolBash, tools.ToolGitStatus, tools.ToolWebSearch} {
		if meta := enrichBuiltinToolMetadata(name); meta != nil {
			t.Fatalf("enrichBuiltinToolMetadata(%q) = %v, want nil", name, meta)
		}
	}
}

func TestIsFileGeneratingTool(t *testing.T) {
	fileGenTools := []string{
		tools.ToolDocumentGenerate, tools.ToolDocumentConvert,
		tools.ToolNotebookEdit,
		tools.ToolImageGenerate, tools.ToolVideoGenerate, tools.ToolSpeechSynthesize,
		tools.ToolBrowserScreenshot,
	}
	for _, name := range fileGenTools {
		if !isFileGeneratingTool(name) {
			t.Fatalf("isFileGeneratingTool(%q) = false, want true", name)
		}
	}
	nonFileGenTools := []string{tools.ToolRead, tools.ToolBash, tools.ToolGitStatus, tools.ToolWebSearch}
	for _, name := range nonFileGenTools {
		if isFileGeneratingTool(name) {
			t.Fatalf("isFileGeneratingTool(%q) = true, want false", name)
		}
	}
}

func TestCatalogListFileGenToolsHaveMetadataAndTags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	pluginpkg.DefaultRegistry().Reset()
	t.Cleanup(func() { pluginpkg.DefaultRegistry().Reset() })

	workspace := t.TempDir()
	defs, err := newCatalog().List(tools.WithWorkspaceRoot(context.Background(), workspace))
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}

	fileGenNames := []string{
		tools.ToolDocumentGenerate, tools.ToolDocumentConvert,
		tools.ToolNotebookEdit,
		tools.ToolImageGenerate, tools.ToolVideoGenerate, tools.ToolSpeechSynthesize,
		tools.ToolBrowserScreenshot,
	}
	for _, name := range fileGenNames {
		t.Run(name, func(t *testing.T) {
			def, ok := toolapi.FindToolDefinition(defs, name)
			if !ok {
				t.Fatalf("missing tool %q", name)
			}
			if def.Metadata == nil {
				t.Fatalf("tool %q has nil metadata", name)
			}
			if guarded, _ := def.Metadata["sandbox_guarded"].(bool); !guarded {
				t.Fatalf("tool %q metadata[sandbox_guarded] = false", name)
			}
			hasFileGenTag := false
			for _, tag := range def.Tags {
				if tag == "file_generation" {
					hasFileGenTag = true
					break
				}
			}
			if !hasFileGenTag {
				t.Fatalf("tool %q missing 'file_generation' tag, tags=%v", name, def.Tags)
			}
		})
	}
}
