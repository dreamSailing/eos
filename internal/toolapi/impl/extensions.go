package impl

import (
	"context"
	"time"

	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/mcp"
	"github.com/dreamSailing/vb-coding/internal/skills"
	"github.com/dreamSailing/vb-coding/internal/tools"
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
	_ = mcpMgr.LoadFromConfig(loadCtx, &cfg)
	mgr.SetMCPManager(mcpMgr)
}
