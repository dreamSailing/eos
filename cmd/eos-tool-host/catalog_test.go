//go:build legacy

package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar/toolhost"
)

func TestConvertToolDefinitionBuildsRustCompatibleSchema(t *testing.T) {
	def := tools.ToolDefinition{
		Name:        "write_file",
		Description: "write file",
		RiskLevel:   tools.RiskLevelMedium,
		Category:    "filesystem",
		Params: map[string]*schema.ParameterInfo{
			"path": {
				Type:     schema.String,
				Required: true,
				Desc:     "target path",
			},
			"force": {
				Type:     schema.Boolean,
				Required: false,
				Desc:     "force write",
			},
		},
		ConcurrencySafe:    true,
		NeedsSandboxRunner: true,
	}

	got := convertToolDefinition(def)
	if got.Name != "write_file" || got.RiskLevel != "medium" || got.Source != "go-legacy" {
		t.Fatalf("definition=%+v, want mapped identity fields", got)
	}
	if got.ReadOnly {
		t.Fatal("medium risk write tool should not be read-only")
	}
	if !got.Invocable {
		t.Fatal("tool should be invocable")
	}
	if got.Metadata["needs_sandbox_runner"] != true {
		t.Fatalf("metadata=%+v, want needs_sandbox_runner=true", got.Metadata)
	}

	var schemaDoc map[string]any
	if err := json.Unmarshal(got.ParamsSchema, &schemaDoc); err != nil {
		t.Fatalf("params_schema invalid JSON: %v", err)
	}
	required, ok := schemaDoc["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "path" {
		t.Fatalf("required=%v, want [path]", schemaDoc["required"])
	}
	properties := schemaDoc["properties"].(map[string]any)
	path := properties["path"].(map[string]any)
	if path["type"] != "string" || path["description"] != "target path" {
		t.Fatalf("path schema=%+v, want string target path", path)
	}
}

func TestManagerRunnerListToolsReturnsRealCatalog(t *testing.T) {
	runner := &managerRunner{}
	defs, err := runner.ListTools(context.Background(), toolhost.CatalogRequest{
		IncludeTools: []string{tools.ToolRead, tools.ToolBash},
	})
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("defs len=%d, want 2: %+v", len(defs), defs)
	}

	byName := map[string]toolhost.ToolDefinition{}
	for _, def := range defs {
		byName[def.Name] = def
	}
	read := byName[tools.ToolRead]
	if read.Name == "" || read.RiskLevel != "low" || !read.ReadOnly || len(read.ParamsSchema) == 0 {
		t.Fatalf("read=%+v, want low read-only tool with schema", read)
	}
	bash := byName[tools.ToolBash]
	if bash.Name == "" || bash.RiskLevel != "high" || bash.ReadOnly {
		t.Fatalf("bash=%+v, want high non-read-only tool", bash)
	}
}

func TestManagerRunnerListToolsFiltersAllowedTools(t *testing.T) {
	runner := &managerRunner{}
	defs, err := runner.ListTools(context.Background(), toolhost.CatalogRequest{
		IncludeTools: []string{tools.ToolRead, tools.ToolBash},
		AllowedTools: []string{tools.ToolRead},
	})
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}
	if len(defs) != 1 || defs[0].Name != tools.ToolRead {
		t.Fatalf("defs=%+v, want only read", defs)
	}
}
