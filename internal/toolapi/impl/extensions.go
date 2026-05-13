package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"time"

	"github.com/dreamSailing/eos/internal/browser"
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
	browserRT := browser.NewBuiltinRuntime()
	loadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	mcpCfg := cfg
	mcpCfg.MCP = pluginpkg.MergeMCPEntries(&cfg, workspaceRoot)
	_ = mcpMgr.LoadFromConfig(loadCtx, &mcpCfg)
	mgr.SetMCPManager(mcpMgr)
	mgr.SetBrowserRuntime(browserRT)
}
