package bridge

import (
	"context"
	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/pkg/workspace"
	"github.com/dreamSailing/eos/internal/skills"
	"github.com/dreamSailing/eos/internal/tools"
)

// SetContextEngine 设置上下文引擎
func (rc *RuntimeCore) SetContextEngine(e *codectx.Engine, m *codectx.MultiEngine) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.ctxEngine = e
	rc.workspaceMgr = m
	rc.wsMgr = workspace.NewManager(m)
}

// GetActiveRoot 获取当前活动的工作区根目录
func (rc *RuntimeCore) GetActiveRoot() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.workspaceMgr == nil {
		return ""
	}
	if a := rc.workspaceMgr.Active(); a != nil {
		return a.Root
	}
	return ""
}

// GetWorkspaceRoots 获取所有工作区根目录
func (rc *RuntimeCore) GetWorkspaceRoots() []string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.workspaceMgr == nil {
		return nil
	}
	return rc.workspaceMgr.Roots()
}

// AddWorkspaceRoot 添加工作区根目录
func (rc *RuntimeCore) AddWorkspaceRoot(path string) *codectx.Engine {
	rc.mu.Lock()
	if rc.workspaceMgr == nil {
		rc.workspaceMgr = codectx.NewMultiEngine()
	}
	e := rc.workspaceMgr.AddRoot(path)
	rc.mu.Unlock()
	go func(p string) {
		rc.reqCh <- hookEventReq{ctx: context.Background(), event: "WorktreeCreate", path: p}
	}(path)
	return e
}

// RemoveWorkspaceRoot 移除工作区根目录
func (rc *RuntimeCore) RemoveWorkspaceRoot(path string) {
	rc.mu.Lock()
	if rc.workspaceMgr != nil {
		rc.workspaceMgr.RemoveRoot(path)
	}
	rc.mu.Unlock()
	go func(p string) {
		rc.reqCh <- hookEventReq{ctx: context.Background(), event: "WorktreeRemove", path: p}
	}(path)
}

// SetActiveWorkspaceRoot 设置活动的工作区根目录
func (rc *RuntimeCore) SetActiveWorkspaceRoot(path string) *codectx.Engine {
	rc.mu.Lock()
	var active *codectx.Engine
	if rc.workspaceMgr != nil {
		if e := rc.workspaceMgr.SetActive(path); e != nil {
			rc.ctxEngine = e
			active = e
		} else {
			rc.mu.Unlock()
			return nil
		}
	} else {
		rc.mu.Unlock()
		return nil
	}
	rc.mu.Unlock()
	rc.refreshLSPManager()
	rc.syncSessionLock()
	return active
}

// SuggestContext 获取上下文建议
func (rc *RuntimeCore) SuggestContext(text string, k int) []codectx.Suggestion {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.ctxEngine == nil {
		return nil
	}
	return rc.ctxEngine.Suggest(text, k)
}

// SetContextDebounce 设置上下文防抖延迟
func (rc *RuntimeCore) SetContextDebounce(ms int) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.ctxEngine != nil {
		rc.ctxEngine.SetDebounce(ms)
	}
}

// GetWorkspaceMgr 获取工作区管理器
func (rc *RuntimeCore) GetWorkspaceMgr() *codectx.MultiEngine {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.workspaceMgr
}

// GetSkillsLoader 获取 Skills 加载器
func (rc *RuntimeCore) GetSkillsLoader() *skills.Loader {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.skillsLoader
}

// GetSkillManager 获取 Skills 管理器
func (rc *RuntimeCore) GetSkillManager() *tools.SkillManager {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.tm != nil {
		return rc.tm.GetSkillManager()
	}
	return nil
}

// StartContextEngine 启动上下文引擎
func (rc *RuntimeCore) StartContextEngine(wd string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.workspaceMgr == nil {
		rc.workspaceMgr = codectx.NewMultiEngine()
	}
	rc.ctxEngine = rc.workspaceMgr.AddRoot(wd)
	rc.wsMgr = workspace.NewManager(rc.workspaceMgr)
	go rc.workspaceMgr.StartBackground(context.Background())
}
