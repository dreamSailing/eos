package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"github.com/dreamSailing/eos/internal/pkg/git"
	"github.com/dreamSailing/eos/internal/pkg/workspace"
)

// ExecuteGit 执行 Git 命令
func (rc *RuntimeCore) ExecuteGit(ui git.UI, args []string) bool {
	return rc.gitMgr.HandleCommand(ui, args)
}

// ExecuteWorkspace 执行工作区命令
func (rc *RuntimeCore) ExecuteWorkspace(ui workspace.UI, args []string) bool {
	if rc.wsMgr == nil {
		rc.wsMgr = workspace.NewManager(rc.workspaceMgr)
	}
	rc.wsMgr.SetMultiEngine(rc.workspaceMgr)
	return rc.wsMgr.HandleCommand(ui, args)
}
