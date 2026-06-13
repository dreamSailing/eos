//go:build legacy

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dreamSailing/eos/internal/browser"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/mcp"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/skills"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar/toolhost"
)

// managerRunner adapts tools.Manager to the toolhost.ToolRunner interface.
//
// TODO: This adapter couples pkg/coreapi/sidecar/toolhost to internal/tools,
// internal/config, internal/skills, internal/mcp, internal/browser, and
// internal/pkg/plugins. When the tool execution layer is refactored behind a
// clean interface (e.g., coreapi.ToolExecutor), this adapter should be replaced
// by a thin implementation that does not import internal packages directly.
type managerRunner struct {
	mgr *tools.Manager
}

func newManagerRunner(workspaceRoot string) (*managerRunner, error) {
	m := tools.NewManager()
	if m == nil {
		return nil, fmt.Errorf("tools.NewManager returned nil")
	}
	m.SetWorkspaceRoot(workspaceRoot)
	configureManagerExtensions(context.Background(), m, workspaceRoot)
	return &managerRunner{mgr: m}, nil
}

func (r *managerRunner) ExecuteTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, string, error) {
	var params map[string]interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, "", fmt.Errorf("invalid tool args: %w", err)
		}
	}
	if params == nil {
		params = make(map[string]interface{})
	}

	call := tools.ToolCall{
		Tool:       name,
		Parameters: params,
	}

	results := r.mgr.ExecuteStructured(ctx, []tools.ToolCall{call})
	if len(results) == 0 {
		return nil, "", fmt.Errorf("tool %s: no result returned", name)
	}

	res := results[0]
	if res.Status == "error" {
		return nil, res.Display, fmt.Errorf("%s", res.Error)
	}

	output, err := json.Marshal(res.Data)
	if err != nil {
		return nil, res.Display, fmt.Errorf("marshal tool output: %w", err)
	}

	return output, res.Display, nil
}

// configureManagerExtensions wires skills, MCP, and browser runtime into a
// tools.Manager. This mirrors internal/toolapi/impl/legacy_bridge.go.
//
// TODO: Extract this into a shared helper once the tool execution layer has a
// clean public interface. Currently this is duplicated from legacy_bridge.go.
func configureManagerExtensions(ctx context.Context, mgr *tools.Manager, workspaceRoot string) {
	if mgr == nil {
		return
	}

	cfg, _ := config.Load()

	loader := skills.NewLoader()
	skillDirs := skills.ResolveScanDirs(workspaceRoot, &cfg)
	if len(skillDirs) > 0 {
		loader.SetSkillsDirs(skillDirs)
		_ = loader.Scan()
	}
	mgr.SetSkillManager(tools.NewSkillManager(loader, mgr))

	mcpMgr := mcp.NewManager()
	browserRT := browser.NewBuiltinRuntime()
	loadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	mcpCfg := cfg
	mcpCfg.MCP = pluginpkg.MergeMCPEntries(&cfg, workspaceRoot)
	_ = mcpMgr.LoadFromConfig(loadCtx, &mcpCfg)
	mgr.SetMCPManager(mcpMgr)
	mgr.SetBrowserRuntime(browserRT)
}

// Ensure managerRunner satisfies the ToolRunner interface at compile time.
var _ toolhost.ToolRunner = (*managerRunner)(nil)
