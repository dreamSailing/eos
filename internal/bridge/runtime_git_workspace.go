package bridge

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
