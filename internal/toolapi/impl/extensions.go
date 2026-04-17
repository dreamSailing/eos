package impl

import (
	"context"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/mcp"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/skills"
	"github.com/dreamSailing/eos/internal/tools"
)

func configureManagerExtensions(ctx context.Context, mgr *tools.Manager, workspaceRoot string) {
	if mgr == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
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
	loadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	mcpCfg := cfg
	mcpCfg.MCP = pluginpkg.MergeMCPEntries(&cfg, workspaceRoot)
	_ = mcpMgr.LoadFromConfig(loadCtx, &mcpCfg)
	mgr.SetMCPManager(mcpMgr)
}
