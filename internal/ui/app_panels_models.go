package ui

// app_panels_models.go — 模型（models）面板的刷新与增删改。
//
// 本文件包含：
//   - refreshModelsPanel：从适配器加载模型列表并更新面板
//   - handleModelSelect / handleModelDelete / handleModelSyncEnv
//   - handleModelFormComplete：模型表单完成（添加或编辑）
//   - Update 中模型相关面板消息分支的处理方法
//
// 代码原位于 app.go，仅做物理拆分，不改行为。

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/ui/panels"
	"github.com/dreamSailing/eos/internal/ui/views/setup"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) refreshModelsPanel() {
	panel, ok := m.panels["models"].(*panels.ModelsPanel)
	if !ok || panel == nil || m.adapter == nil {
		return
	}
	models, active, err := m.adapter.ModelEntries(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("刷新模型列表失败", "Failed to refresh models"), err), "error")
		return
	}
	if snapshot, snapErr := m.adapter.ModelContext(context.Background()); snapErr == nil && strings.TrimSpace(snapshot.ResolvedModelName) != "" {
		active = strings.TrimSpace(snapshot.ResolvedModelName)
	}
	panel.SetModels(models, active)
}

// handleModelSelect 处理模型选择，根据作用域显示不同的成功消息
func (m *AppModel) handleModelSelect(msg panels.ModelSelectMsg) {
	scope, err := m.adapter.SelectModelForCurrentContext(context.Background(), msg.Name)
	if err != nil {
		m.appendSystem(fmt.Sprintf("Failed to switch model: %s", msg.Name), "error")
		return
	}
	m.refreshModelsPanel()
	m.refreshShellWelcomeInfo()
	switch scope {
	case "session":
		m.appendSystem(fmt.Sprintf("Switched current session model: %s", msg.Name), "success")
	case "workspace":
		m.appendSystem(fmt.Sprintf("Switched workspace model: %s", msg.Name), "success")
	default:
		m.appendSystem(fmt.Sprintf("Switched global default model: %s", msg.Name), "success")
	}
}

// handleModelDelete 处理模型删除
func (m *AppModel) handleModelDelete(msg panels.ModelDeleteMsg) {
	if err := m.adapter.DeleteModel(context.Background(), msg.Name); err != nil {
		m.appendSystem(fmt.Sprintf("Failed to delete model: %s (may be env model or active model)", msg.Name), "error")
		return
	}
	m.refreshModelsPanel()
	m.appendSystem(fmt.Sprintf("Deleted model: %s", msg.Name), "success")
}

// handleModelSyncEnv 处理环境变量同步
func (m *AppModel) handleModelSyncEnv() {
	if err := m.adapter.SyncEnvModel(context.Background()); err != nil {
		m.appendSystem(i18n.T("model.sync_env_failed", m.state.Language), "error")
		return
	}
	m.refreshModelsPanel()
	m.appendSystem(i18n.T("model.sync_env_ok", m.state.Language), "success")
}

// handleModelFormComplete 处理模型表单完成事件
// 根据编辑模式决定是更新还是添加模型
func (m *AppModel) handleModelFormComplete(msg setup.ModelFormCompleteMsg) {
	m.activeView = "shell"
	m.shell.FocusInput()
	// 初始设置流程中不显示成功消息
	suppressSuccessMessage := m.initialSetupFlow && len(m.history) == 0 && !msg.EditMode

	// 生成模型名称
	name := msg.Config.Name
	if name == "" {
		name = fmt.Sprintf("model-%d", time.Now().Unix()%100000)
	}

	entry := config.ModelEntry{
		Name:    name,
		APIBase: msg.Config.APIBase,
		APIKey:  msg.Config.APIKey,
		Model:   msg.Config.Model,
		Source:  "user",
	}

	if msg.EditMode {
		// 编辑模式：更新现有模型
		if err := m.adapter.UpsertModelEntry(context.Background(), entry); err != nil {
			m.appendSystem(fmt.Sprintf("Failed to update model: %s", name), "error")
		} else {
			m.appendSystem(fmt.Sprintf("Updated model: %s", name), "success")
		}
	} else {
		// 添加模式：添加新模型并设置为当前上下文模型
		if err := m.adapter.UpsertModelEntry(context.Background(), entry); err == nil {
			_, _ = m.adapter.SelectModelForCurrentContext(context.Background(), name)
			m.refreshShellWelcomeInfo()
			if !suppressSuccessMessage {
				m.appendSystem(fmt.Sprintf("Added and selected model: %s", name), "success")
			}
		} else {
			m.appendSystem(fmt.Sprintf("Failed to add model: %s", name), "error")
		}
	}
	m.initialSetupFlow = false

	// 刷新模型列表面板
	m.refreshModelsPanel()
}

// handleModelSelectMsg 处理 panels.ModelSelectMsg（Update 分支提取）。fall-through。
func (m *AppModel) handleModelSelectMsg(msg panels.ModelSelectMsg) (tea.Model, tea.Cmd) {
	m.handleModelSelect(msg)
	return m, m.finalizeUpdate(nil)
}

// handleModelAddMsg 处理 panels.ModelAddMsg（Update 分支提取）。fall-through。
func (m *AppModel) handleModelAddMsg(_ panels.ModelAddMsg) (tea.Model, tea.Cmd) {
	// 添加模型 - 打开向导视图
	m.activeView = "setup"
	wizard := setup.NewModelSetupWizard(m.styles, m.state.Language)
	wizard.SetSize(m.width, m.height)
	m.setupView = wizard
	return m, m.finalizeUpdate(nil)
}

// handleModelDeleteMsg 处理 panels.ModelDeleteMsg（Update 分支提取）。fall-through。
func (m *AppModel) handleModelDeleteMsg(msg panels.ModelDeleteMsg) (tea.Model, tea.Cmd) {
	m.handleModelDelete(msg)
	return m, m.finalizeUpdate(nil)
}

// handleModelSyncMsg 处理 panels.ModelSyncMsg（Update 分支提取）。fall-through。
func (m *AppModel) handleModelSyncMsg(_ panels.ModelSyncMsg) (tea.Model, tea.Cmd) {
	m.handleModelSyncEnv()
	return m, m.finalizeUpdate(nil)
}

// handleModelRefreshMsg 处理 panels.ModelRefreshMsg（Update 分支提取）。fall-through。
func (m *AppModel) handleModelRefreshMsg(_ panels.ModelRefreshMsg) (tea.Model, tea.Cmd) {
	m.refreshModelsPanel()
	return m, m.finalizeUpdate(nil)
}

// handleModelFormCompleteMsg 处理 setup.ModelFormCompleteMsg（Update 分支提取）。fall-through。
func (m *AppModel) handleModelFormCompleteMsg(msg setup.ModelFormCompleteMsg) (tea.Model, tea.Cmd) {
	m.handleModelFormComplete(msg)
	return m, m.finalizeUpdate(nil)
}
