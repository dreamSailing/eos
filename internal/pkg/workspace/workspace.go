package workspace

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"fmt"
	codectx "github.com/dreamSailing/eos/internal/context"
	"os"
	"path/filepath"
	"strings"
)

// UI 接口，用于回调 UI 操作
type UI interface {
	WriteLine(color, text string)
	PromptPermission(category, summary string) string
	T(key string) string
	ShowWorkspaceManager()
	SaveSettings()
	UpdateSettingsPath(path string)
	SetActiveEngine(e *codectx.Engine)
}

// Manager 负责处理 Workspace 相关的业务逻辑
type Manager struct {
	mgr *codectx.MultiEngine
}

// NewManager 创建新的 Workspace 管理器
func NewManager(mgr *codectx.MultiEngine) *Manager {
	return &Manager{mgr: mgr}
}

// SetMultiEngine 设置多引擎管理器
func (m *Manager) SetMultiEngine(mgr *codectx.MultiEngine) {
	m.mgr = mgr
}

// HandleCommand 解析并处理 Workspace 命令
func (m *Manager) HandleCommand(ui UI, name []string) bool {
	if len(name) < 2 {
		ui.ShowWorkspaceManager()
		return true
	}
	sub := strings.ToLower(name[1])
	switch sub {
	case "add":
		return m.handleAdd(ui, name)
	case "remove":
		return m.handleRemove(ui, name)
	case "use":
		return m.handleUse(ui, name)
	case "list":
		ui.ShowWorkspaceManager()
		return true
	default:
		ui.WriteLine("yellow", "Usage: /workspace add <path>|remove <path>|use <path>|list")
		return false
	}
}

func (m *Manager) handleAdd(ui UI, name []string) bool {
	if len(name) < 3 {
		ui.WriteLine("yellow", "Usage: /workspace add <path>")
		return false
	}
	p := name[2]
	fi, err := os.Stat(p)
	if err != nil || !fi.IsDir() {
		ui.WriteLine("yellow", "Path not a directory")
		return false
	}
	go func() {
		dec := ui.PromptPermission("workspace-add", "Add workspace: "+p)
		if dec == "deny" {
			ui.WriteLine("white", fmt.Sprintf(ui.T("denied"), "workspace add"))
			return
		}
		if m.mgr == nil {
			m.mgr = codectx.NewMultiEngine()
		}
		e := m.mgr.AddRoot(p)
		if e != nil {
			ui.WriteLine("white", "Workspace added: "+p)
		}
	}()
	return true
}

func (m *Manager) handleRemove(ui UI, name []string) bool {
	if len(name) < 3 {
		ui.WriteLine("yellow", "Usage: /workspace remove <path>")
		return false
	}
	p := name[2]
	if m.mgr != nil {
		m.mgr.RemoveRoot(p)
		if a := m.mgr.Active(); a != nil {
			ui.SetActiveEngine(a)
		}
	}
	ui.WriteLine("white", "Workspace removed: "+p)
	return true
}

func (m *Manager) handleUse(ui UI, name []string) bool {
	if len(name) < 3 {
		ui.WriteLine("yellow", "Usage: /workspace use <path>")
		return false
	}
	p := name[2]
	if m.mgr == nil {
		ui.WriteLine("yellow", "No workspaces. Add one with /workspace add <path>")
		return false
	}
	go func() {
		dec := ui.PromptPermission("workspace-use", "Use workspace: "+p)
		if dec == "deny" {
			ui.WriteLine("white", fmt.Sprintf(ui.T("denied"), "workspace use"))
			return
		}
		if e := m.mgr.SetActive(p); e != nil {
			_ = os.Chdir(p)
			ui.SetActiveEngine(e)
			if p2, _ := os.Getwd(); p2 != "" {
				ui.UpdateSettingsPath(filepath.Join(p2, ".eos", "settings.json"))
			}
			ui.SaveSettings()
			ui.WriteLine("white", "Active workspace: "+p)
		} else {
			ui.WriteLine("yellow", "Workspace not found: "+p)
		}
	}()
	return true
}
