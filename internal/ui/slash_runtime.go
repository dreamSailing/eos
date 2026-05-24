package ui

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runpkg "runtime"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/ui/panels"
	"github.com/dreamSailing/eos/pkg/coreapi"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) localize(zh, en string) string {
	if m != nil && strings.EqualFold(m.state.Language, "en") {
		return en
	}
	return zh
}

func (m *AppModel) currentWorkspaceRoot() string {
	if m != nil && m.adapter != nil {
		if root := strings.TrimSpace(m.adapter.ActiveWorkspace(context.Background())); root != "" {
			return root
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func isSupportedExecutionModeInput(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	normalized := toolapi.NormalizeExecutionMode(raw)
	for _, item := range toolapi.SupportedExecutionModes() {
		if item.Name == normalized {
			return true
		}
	}
	return false
}

func (m *AppModel) executionModeUsage() string {
	return m.localize(
		"用法: /permissions [auto|plan|access <mode>|approval <mode>]",
		"Usage: /permissions [auto|plan|access <mode>|approval <mode>]",
	)
}

func isSupportedAccessModeInput(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	normalized := toolapi.NormalizeAccessMode(raw)
	for _, item := range toolapi.SupportedAccessModes() {
		if item.Name == normalized {
			return true
		}
	}
	return false
}

func isSupportedApprovalModeInput(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	normalized := toolapi.NormalizeApprovalMode(raw)
	for _, item := range toolapi.SupportedApprovalModes() {
		if item.Name == normalized {
			return true
		}
	}
	return false
}

func (m *AppModel) openModelsPanel() {
	m.activeView = "panel"
	m.activePanel = "models"
	m.shell.ClearInput()
	m.refreshModelsPanel()
}

func (m *AppModel) openContextPanel() {
	m.activeView = "panel"
	m.activePanel = "context"
	m.shell.ClearInput()
	if panel, ok := m.panels["context"].(*panels.ContextPanel); ok && panel != nil {
		panel.ResetView()
	}
	m.refreshContextPanel()
}

func (m *AppModel) openMemoryPanel() {
	m.activeView = "panel"
	m.activePanel = "memory"
	m.shell.ClearInput()
	if panel, ok := m.panels["memory"].(*panels.MemoryPanel); ok && panel != nil {
		if panel.IsEditing() {
			panel.CancelEdit()
		}
	}
	m.refreshMemoryPanel()
}

func (m *AppModel) openSettingsPanel() {
	m.activeView = "panel"
	m.activePanel = "settings"
	m.shell.ClearInput()
	m.refreshSettingsPanel()
}

func (m *AppModel) handleWorkspaceSlash(args []string) tea.Cmd {
	if len(args) == 0 || strings.EqualFold(args[0], "list") {
		m.activeView = "panel"
		m.activePanel = "workspace"
		m.shell.ClearInput()
		m.refreshWorkspacePanel()
		return nil
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	if len(args) < 2 && (sub == "add" || sub == "remove" || sub == "use") {
		m.appendSystem(m.localize("用法: /workspace add|remove|use <path>", "Usage: /workspace add|remove|use <path>"), "warning")
		return nil
	}

	rawPath := strings.TrimSpace(args[1])
	path, err := resolveWorkspaceInputPath(rawPath)
	if err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}

	switch sub {
	case "add":
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			m.appendSystem(m.localize("路径不是目录: ", "Path is not a directory: ")+path, "warning")
			return nil
		}
		if err := m.adapter.AddWorkspace(context.Background(), path); err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		m.refreshWorkspacePanel()
		m.appendSystem(m.localize("已添加工作区: ", "Added workspace: ")+path, "success")
	case "remove":
		if err := m.adapter.RemoveWorkspace(context.Background(), path); err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		m.refreshWorkspacePanel()
		m.appendSystem(m.localize("已移除工作区: ", "Removed workspace: ")+path, "success")
	case "use":
		return m.handleWorkspaceUse(path)
	default:
		m.appendSystem(m.localize("用法: /workspace add|remove|use <path> 或 /workspace list", "Usage: /workspace add|remove|use <path> or /workspace list"), "warning")
	}
	return nil
}

func (m *AppModel) handleModelSlash(args []string) tea.Cmd {
	if len(args) == 0 {
		m.openModelsPanel()
		return nil
	}

	if strings.EqualFold(args[0], "current") {
		modelName, modelBase := m.adapter.GetModelInfo()
		if strings.TrimSpace(modelName) == "" || strings.TrimSpace(modelBase) == "" {
			base, _, model, _ := m.adapter.ResolveAPIConfig()
			if modelName == "" {
				modelName = model
			}
			if modelBase == "" {
				modelBase = base
			}
		}
		m.appendSystem(fmt.Sprintf("%s: %s (%s)", m.localize("当前模型", "Current model"), strings.TrimSpace(modelName), strings.TrimSpace(modelBase)), "info")
		return nil
	}

	name := strings.TrimSpace(strings.Join(args, " "))
	if strings.EqualFold(args[0], "use") && len(args) > 1 {
		name = strings.TrimSpace(strings.Join(args[1:], " "))
	}
	if name == "" {
		m.appendSystem(m.localize("用法: /model [use <name>]", "Usage: /model [use <name>]"), "warning")
		return nil
	}

	if err := m.adapter.ActivateModel(context.Background(), name); err != nil {
		m.appendSystem(fmt.Sprintf("%s: %s", m.localize("未找到模型", "Model not found"), name), "error")
		return nil
	}

	m.refreshModelsPanel()
	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已切换模型", "Switched model"), name), "success")
	return nil
}

func (m *AppModel) handleSessionSlash(args []string) tea.Cmd {
	if len(args) > 0 {
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "save":
			id, err := m.adapter.SaveSessionMessages(context.Background(), "", m.sessionTranscript())
			if err != nil {
				m.appendSystem(err.Error(), "error")
				return nil
			}
			m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已保存会话", "Saved session"), id), "success")
			return nil
		case "export":
			id := ""
			if len(args) >= 2 {
				id = strings.TrimSpace(args[1])
			}
			path := ""
			if len(args) >= 3 {
				path = strings.TrimSpace(args[2])
			} else if id != "" {
				path = filepath.Join(m.adapter.SessionsDir(context.Background()), id+".md")
			}
			if strings.TrimSpace(path) == "" {
				m.appendSystem(m.localize("用法: /session export <id> [outputPath]", "Usage: /session export <id> [outputPath]"), "warning")
				return nil
			}
			if err := m.adapter.ExportSessionMarkdown(context.Background(), id, path); err != nil {
				m.appendSystem(err.Error(), "error")
				return nil
			}
			m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已导出会话", "Exported session"), path), "success")
			return nil
		}
	}

	metas, err := m.adapter.ListSessions(context.Background())
	if err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}
	currentID, _ := m.adapter.CurrentSessionID(context.Background())
	if len(metas) == 0 {
		m.appendSystem(m.localize("暂无已保存会话。使用 /session save 保存当前会话。", "No saved sessions. Use /session save to persist the current session."), "info")
		return nil
	}

	lines := []string{
		m.localize("会话列表", "Sessions"),
		fmt.Sprintf("%s: %d", m.localize("总数", "Total"), len(metas)),
	}
	limit := len(metas)
	if limit > 12 {
		limit = 12
	}
	for i := 0; i < limit; i++ {
		meta := metas[i]
		ts := time.Unix(meta.SavedAt, 0).Format("2006-01-02 15:04")
		label := strings.TrimSpace(meta.Title)
		if label == "" {
			label = strings.TrimSpace(meta.Summary)
		}
		if label == "" {
			label = strings.TrimSpace(meta.Preview)
		}
		prefix := " "
		if meta.ID == currentID {
			prefix = "*"
		}
		line := fmt.Sprintf("%s %s  %s  rounds=%d tokens=%d", prefix, meta.ID, ts, meta.Rounds, meta.Tokens)
		if label != "" {
			line += "  " + label
		}
		lines = append(lines, line)
	}
	lines = append(lines, m.localize("使用 /resume [id] 恢复，或 /session save 保存当前会话。", "Use /resume [id] to restore, or /session save to persist the current session."))
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleResumeSlash(args []string) tea.Cmd {
	id := ""
	if len(args) > 0 {
		id = strings.TrimSpace(args[0])
	}
	if err := m.adapter.ResumeSession(context.Background(), id); err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}

	resolvedID, _ := m.adapter.CurrentSessionID(context.Background())
	m.restoreSessionHistory(resolvedID)
	m.refreshContextPanel()
	m.refreshCostPanel()
	m.updateContextUsageUI()
	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已恢复会话", "Resumed session"), resolvedID), "success")
	return nil
}

func (m *AppModel) handlePermissionsSlash(args []string) tea.Cmd {
	if len(args) > 0 {
		switch {
		case len(args) == 1 && isSupportedExecutionModeInput(args[0]):
			mode := toolapi.NormalizeExecutionMode(args[0])
			if err := m.adapter.SetExecutionMode(context.Background(), mode); err != nil {
				m.appendSystem(err.Error(), "error")
				return nil
			}
			m.state.ExecutionMode = mode
			m.shell.SetExecutionMode(mode)
			m.appendSystem(fmt.Sprintf("%s %s", m.localize("执行模式已切换为", "Execution mode switched to"), m.executionModeLabel(mode)), "success")
		case len(args) >= 2 && strings.EqualFold(strings.TrimSpace(args[0]), "access") && isSupportedAccessModeInput(args[1]):
			mode := toolapi.NormalizeAccessMode(args[1])
			if err := m.adapter.SetAccessMode(context.Background(), mode); err != nil {
				m.appendSystem(err.Error(), "error")
				return nil
			}
			m.appendSystem(fmt.Sprintf("%s %s", m.localize("访问模式已切换为", "Access mode switched to"), mode), "success")
		case len(args) >= 2 && strings.EqualFold(strings.TrimSpace(args[0]), "approval") && isSupportedApprovalModeInput(args[1]):
			mode := toolapi.NormalizeApprovalMode(args[1])
			if err := m.adapter.SetApprovalMode(context.Background(), mode); err != nil {
				m.appendSystem(err.Error(), "error")
				return nil
			}
			m.appendSystem(fmt.Sprintf("%s %s", m.localize("审批模式已切换为", "Approval mode switched to"), mode), "success")
		default:
			m.appendSystem(m.executionModeUsage(), "warning")
			return nil
		}
	}

	snap, err := m.adapter.PermissionSnapshot(context.Background())
	if err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}
	lines := []string{
		m.localize("权限与审批状态", "Permissions and approvals"),
		fmt.Sprintf("%s: %s", m.localize("执行模式", "Execution mode"), m.executionModeLabel(snap.ExecutionMode)),
		fmt.Sprintf("%s: %s", m.localize("访问模式", "Access mode"), snap.AccessMode),
		fmt.Sprintf("%s: %s", m.localize("审批模式", "Approval mode"), snap.ApprovalMode),
		fmt.Sprintf("%s: %s", m.localize("沙箱模式", "Sandbox mode"), snap.SandboxMode),
		fmt.Sprintf("%s: %t", m.localize("全局放行", "Allow all"), snap.AllowAll),
	}
	if len(snap.AllowedCategories) > 0 {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("会话内已放行类别", "Session-allowed categories"), strings.Join(snap.AllowedCategories, ", ")))
	} else {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("会话内已放行类别", "Session-allowed categories"), m.localize("无", "none")))
	}
	if snap.HasPendingDiff {
		target := snap.PendingDiffPath
		if strings.TrimSpace(target) == "" {
			target = m.localize("(未标记路径)", "(path unavailable)")
		}
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("待审批 diff", "Pending diff"), target))
	} else {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("待审批 diff", "Pending diff"), m.localize("无", "none")))
	}
	if strings.TrimSpace(snap.LastAuthorization) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("最近授权结果", "Last authorization"), snap.LastAuthorization))
		if strings.TrimSpace(snap.LastAuthorizationKind) != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", m.localize("最近授权类别", "Last authorization kind"), snap.LastAuthorizationKind))
		}
		if strings.TrimSpace(snap.LastAuthorizationTarget) != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", m.localize("最近升级目标", "Last escalation target"), snap.LastAuthorizationTarget))
		}
		if strings.TrimSpace(snap.LastAuthorizationNote) != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", m.localize("最近授权说明", "Last authorization note"), snap.LastAuthorizationNote))
		}
	} else {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("最近授权结果", "Last authorization"), m.localize("无", "none")))
	}
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handlePlanSlash(args []string) tea.Cmd {
	if len(args) > 0 && isSupportedExecutionModeInput(args[0]) {
		return m.handlePermissionsSlash(args)
	}

	items, _ := m.adapter.Todos(context.Background())
	modeSnapshot, _ := m.adapter.ModeSnapshot(context.Background())
	if strings.TrimSpace(modeSnapshot.ExecutionMode) == "" {
		modeSnapshot.ExecutionMode = m.state.ExecutionMode
	}
	lines := []string{
		m.localize("当前计划与待办", "Current plan and todos"),
		fmt.Sprintf("%s: %s", m.localize("执行模式", "Execution mode"), m.executionModeLabel(modeSnapshot.ExecutionMode)),
	}
	if len(items) == 0 {
		lines = append(lines, m.localize("暂无待办。", "No todo items."))
	} else {
		for idx, item := range items {
			line := fmt.Sprintf("%d. [%s] %s", idx+1, strings.TrimSpace(item.Status), strings.TrimSpace(item.Content))
			if item.ID != "" {
				line += " (" + item.ID + ")"
			}
			lines = append(lines, line)
		}
	}
	lines = append(lines, m.localize("使用 /plan auto|plan 可直接切换执行模式。", "Use /plan auto|plan to switch execution mode directly."))
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleSkillsSlash(args []string) tea.Cmd {
	if len(args) > 0 && strings.EqualFold(args[0], "reload") {
		if err := m.adapter.ReloadSkills(context.Background()); err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		m.appendSystem(m.localize("已重载 skills。", "Reloaded skills."), "success")
	}

	skills, err := m.adapter.Skills(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("读取 skills 失败", "Failed to read skills"), err), "error")
		return nil
	}
	sort.Slice(skills, func(i, j int) bool { return strings.ToLower(skills[i].Name) < strings.ToLower(skills[j].Name) })
	lines := []string{fmt.Sprintf("%s: %d", m.localize("Skills", "Skills"), len(skills))}
	if len(skills) == 0 {
		lines = append(lines, m.localize("暂无可用 skills。", "No skills available."))
	} else {
		for _, skill := range skills {
			prefix := " "
			if skill.Active {
				prefix = "*"
			}
			desc := strings.TrimSpace(skill.Description)
			if desc == "" {
				desc = m.localize("(无描述)", "(no description)")
			}
			origin := strings.TrimSpace(skill.Location)
			if origin == "" {
				origin = strings.TrimSpace(skill.BaseDir)
			}
			if origin == "" {
				origin = strings.TrimSpace(skill.Source)
			}
			lines = append(lines, fmt.Sprintf("%s %s [%s] - %s", prefix, skill.Name, blankFallback(origin, m.localize("unknown", "unknown")), desc))
		}
	}
	lines = append(lines, m.localize("使用 /skills reload 重新扫描并保留当前激活状态。", "Use /skills reload to rescan while preserving active skills."))
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handlePluginSlash() tea.Cmd {
	rows, err := m.adapter.Plugins(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("读取插件失败", "Failed to read plugins"), err), "error")
		return nil
	}
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name) })
	lines := []string{fmt.Sprintf("%s: %d", m.localize("插件", "Plugins"), len(rows))}
	if len(rows) == 0 {
		lines = append(lines, m.localize("暂无可用插件。", "No plugins available."))
	} else {
		for _, plugin := range rows {
			status := m.localize("enabled", "enabled")
			if !plugin.Enabled {
				status = m.localize("disabled", "disabled")
			}
			desc := strings.TrimSpace(plugin.Description)
			if desc == "" {
				desc = m.localize("(无描述)", "(no description)")
			}
			source := strings.TrimSpace(plugin.Source)
			if source == "" {
				source = strings.TrimSpace(plugin.Command)
			}
			lines = append(lines, fmt.Sprintf("- %s [%s, %s]: %s", plugin.Name, blankFallback(source, "plugin"), status, desc))
		}
	}
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleReloadPluginsSlash() tea.Cmd {
	if err := m.adapter.Reload(); err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("插件重载失败", "Plugin reload failed"), err), "error")
		return nil
	}
	m.refreshContextPanel()
	m.refreshMCPPanel()
	m.refreshLSPPanel()
	m.appendSystem(
		m.localize("已重载插件扩展与目录发现。", "Reloaded plugin extensions and discovery."),
		"success",
	)
	return nil
}

func (m *AppModel) handleDoctorSlash() tea.Cmd {
	modelName, modelBase := m.adapter.GetModelInfo()
	if modelName == "" || modelBase == "" {
		base, _, model, _ := m.adapter.ResolveAPIConfig()
		if modelName == "" {
			modelName = model
		}
		if modelBase == "" {
			modelBase = base
		}
	}
	snap, _ := m.adapter.PermissionSnapshot(context.Background())
	sessions, _ := m.adapter.ListSessions(context.Background())
	currentSessionID, _ := m.adapter.CurrentSessionID(context.Background())
	bgTasks, _ := m.adapter.Tasks(context.Background())
	agentTasks, _ := m.adapter.Agents(context.Background())
	todos, _ := m.adapter.Todos(context.Background())
	traces, _ := m.adapter.ToolTraces(context.Background())
	stats, _ := m.adapter.ToolStats(context.Background())
	browser, _ := m.adapter.BrowserStatus(context.Background())
	skills, _ := m.adapter.Skills(context.Background())
	plugins, _ := m.adapter.Plugins(context.Background())

	lines := []string{
		m.localize("Doctor 摘要", "Doctor summary"),
		fmt.Sprintf("%s: %s", m.localize("工作区", "Workspace"), m.currentWorkspaceRoot()),
		fmt.Sprintf("%s: %s", m.localize("模型", "Model"), strings.TrimSpace(modelName)),
		fmt.Sprintf("%s: %s", m.localize("API Base", "API Base"), strings.TrimSpace(modelBase)),
		fmt.Sprintf("%s: %s", m.localize("执行模式", "Execution mode"), m.executionModeLabel(snap.ExecutionMode)),
		fmt.Sprintf("%s: %d", m.localize("保存会话数", "Saved sessions"), len(sessions)),
		fmt.Sprintf("%s: %s", m.localize("当前会话", "Current session"), blankFallback(currentSessionID, m.localize("无", "none"))),
		fmt.Sprintf("%s: %d", m.localize("后台任务", "Background tasks"), len(bgTasks)),
		fmt.Sprintf("%s: %d", m.localize("代理任务", "Agent tasks"), len(agentTasks)),
		fmt.Sprintf("%s: %d", m.localize("待办项", "Todo items"), len(todos)),
		fmt.Sprintf("%s: %d", m.localize("可用 skills", "Available skills"), len(skills)),
		fmt.Sprintf("%s: %d", m.localize("已注册插件", "Registered plugins"), len(plugins)),
		fmt.Sprintf("%s: %d", m.localize("工具追踪数", "Tool traces"), len(traces)),
		fmt.Sprintf("%s: %s", m.localize("浏览器 MCP", "Browser MCP"), m.browserStatusLabel(browser)),
	}
	if strings.TrimSpace(browser.LastError) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("浏览器错误", "Browser error"), browser.LastError))
	}
	if len(stats) > 0 {
		lines = append(lines, m.localize("工具统计:", "Tool stats:"))
		sort.Slice(stats, func(i, j int) bool {
			if stats[i].TotalCalls == stats[j].TotalCalls {
				return stats[i].Tool < stats[j].Tool
			}
			return stats[i].TotalCalls > stats[j].TotalCalls
		})
		if len(stats) > 5 {
			stats = stats[:5]
		}
		for _, stat := range stats {
			lines = append(lines, fmt.Sprintf("- %s: calls=%d avg=%s", stat.Tool, stat.TotalCalls, stat.AvgDuration.Round(time.Millisecond)))
		}
	}
	if len(traces) > 0 {
		lines = append(lines, m.localize("最近工具时间线:", "Recent tool timeline:"))
		start := len(traces) - 5
		if start < 0 {
			start = 0
		}
		for _, trace := range traces[start:] {
			status := "ok"
			if !trace.Success {
				status = "error"
			}
			if trace.Cached {
				status += ",cached"
			}
			lines = append(lines, fmt.Sprintf("- %s [%s] %s", trace.Tool, status, trace.Duration.Round(time.Millisecond)))
		}
	}
	diag := strings.TrimSpace(m.adapter.LSPDiagnosticsMarkdown(context.Background()))
	if diag != "" {
		lines = append(lines, m.localize("LSP 诊断摘要:", "LSP diagnostics:"))
		lines = append(lines, truncateBlock(diag, 12, 1200))
	}
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleDiffSlash(args []string) tea.Cmd {
	if len(args) == 0 {
		review, _ := m.adapter.PendingReview(context.Background())
		if strings.TrimSpace(review.Diff) != "" {
			target := strings.TrimSpace(review.Path)
			if target == "" {
				target = m.localize("(当前待审批改动)", "(current pending edit)")
			}
			m.appendSystem(fmt.Sprintf("%s: %s\n%s", m.localize("待审批 diff", "Pending diff"), target, truncateBlock(review.Diff, 40, 5000)), "info")
			return nil
		}
		changes, err := m.adapter.GitStatus(context.Background())
		if err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		if len(changes) == 0 {
			m.appendSystem(m.localize("当前没有检测到 Git 改动。", "No Git changes detected."), "info")
			return nil
		}
		lines := []string{m.localize("当前改动文件", "Changed files")}
		for _, change := range changes {
			lines = append(lines, fmt.Sprintf("- [%s] %s", change.State, change.Path))
		}
		lines = append(lines, m.localize("使用 /diff <path> 查看某个文件的统一 diff。", "Use /diff <path> to inspect a file diff."))
		m.appendSystem(strings.Join(lines, "\n"), "info")
		return nil
	}

	path := strings.TrimSpace(args[0])
	diff, err := m.adapter.GitDiff(context.Background(), path)
	if err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}
	if strings.TrimSpace(diff) == "" {
		m.appendSystem(fmt.Sprintf("%s: %s", m.localize("该文件没有差异", "No diff for file"), path), "info")
		return nil
	}
	m.appendSystem(fmt.Sprintf("%s: %s\n%s", m.localize("文件 diff", "File diff"), path, truncateBlock(diff, 80, 7000)), "info")
	return nil
}

func (m *AppModel) handleReviewSlash(args []string) tea.Cmd {
	lines := []string{m.localize("审查摘要", "Review summary")}
	if len(args) > 0 {
		path := strings.TrimSpace(args[0])
		if diff, err := m.adapter.GitDiff(context.Background(), path); err == nil && strings.TrimSpace(diff) != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", m.localize("目标文件", "Target file"), path))
			lines = append(lines, truncateBlock(diff, 40, 5000))
		} else if err != nil {
			lines = append(lines, fmt.Sprintf("%s: %v", m.localize("读取 diff 失败", "Failed to read diff"), err))
		}
	} else if review, _ := m.adapter.PendingReview(context.Background()); strings.TrimSpace(review.Diff) != "" {
		target := blankFallback(review.Path, m.localize("(当前待审批改动)", "(current pending edit)"))
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("待审改动", "Pending change"), target))
		lines = append(lines, truncateBlock(review.Diff, 40, 5000))
	} else {
		changes, err := m.adapter.GitStatus(context.Background())
		if err == nil && len(changes) > 0 {
			lines = append(lines, m.localize("当前 Git 改动:", "Current Git changes:"))
			for _, change := range changes {
				lines = append(lines, fmt.Sprintf("- [%s] %s", change.State, change.Path))
			}
		}
	}

	diag := strings.TrimSpace(m.adapter.LSPDiagnosticsMarkdown(context.Background()))
	if diag != "" {
		lines = append(lines, m.localize("诊断:", "Diagnostics:"))
		lines = append(lines, truncateBlock(diag, 12, 1200))
	}

	traces, _ := m.adapter.ToolTraces(context.Background())
	if len(traces) > 0 {
		lines = append(lines, m.localize("最近工具调用:", "Recent tool calls:"))
		start := len(traces) - 5
		if start < 0 {
			start = 0
		}
		for _, trace := range traces[start:] {
			result := "ok"
			if !trace.Success {
				result = "error"
			}
			lines = append(lines, fmt.Sprintf("- %s [%s] %s", trace.Tool, result, trace.Duration.Round(time.Millisecond)))
		}
	}

	lines = append(lines, m.localize("建议：先看 /diff，再结合 /doctor 与 /tasks 判断是否需要继续审查。", "Tip: inspect /diff first, then combine /doctor and /tasks to continue the review flow."))
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleGitSlash(args []string) tea.Cmd {
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "status":
		changes, err := m.adapter.GitStatus(context.Background())
		if err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		if len(changes) == 0 {
			m.appendSystem(m.localize("Git 工作区干净。", "Git working tree is clean."), "info")
			return nil
		}
		lines := []string{m.localize("Git 状态", "Git status")}
		for _, change := range changes {
			lines = append(lines, fmt.Sprintf("- [%s] %s", change.State, change.Path))
		}
		m.appendSystem(strings.Join(lines, "\n"), "info")
	case "branches":
		out, err := m.adapter.GitBranches(context.Background())
		if err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		lines := []string{fmt.Sprintf("%s: %s", m.localize("当前分支", "Current branch"), out.Current)}
		for _, branch := range out.Branches {
			lines = append(lines, "- "+branch)
		}
		m.appendSystem(strings.Join(lines, "\n"), "info")
	case "log":
		out, err := m.adapter.GitLog(context.Background(), coreapi.GitLogRequest{Limit: 20, Oneline: true})
		if err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		m.appendSystem(fmt.Sprintf("%s\n%s", m.localize("Git 日志", "Git log"), truncateBlock(out.Text, 30, 3000)), "info")
	case "show":
		revision := "HEAD"
		path := ""
		if len(args) > 1 {
			revision = strings.TrimSpace(args[1])
		}
		if len(args) > 2 {
			path = strings.TrimSpace(args[2])
		}
		out, err := m.adapter.GitShow(context.Background(), coreapi.GitShowRequest{Revision: revision, Path: path})
		if err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		m.appendSystem(fmt.Sprintf("%s %s\n%s", m.localize("Git 显示", "Git show"), revision, truncateBlock(out.Text, 40, 5000)), "info")
	case "diff":
		if len(args) < 2 {
			return m.handleDiffSlash(nil)
		}
		return m.handleDiffSlash(args[1:])
	default:
		m.appendSystem(m.localize("暂支持: /git status|branches|log|show|diff", "Supported: /git status|branches|log|show|diff"), "warning")
	}
	return nil
}

func (m *AppModel) sessionTranscript() []coreapi.SessionMessage {
	out := make([]coreapi.SessionMessage, 0, len(m.history))
	for _, item := range m.history {
		content := strings.TrimSpace(item.content)
		if content == "" {
			continue
		}
		msg := coreapi.SessionMessage{
			Content: content,
			Time:    item.timestamp,
		}
		switch item.kind {
		case "user":
			msg.Role = "user"
			msg.Type = "user"
		case "ai", "agent.final":
			msg.Role = "assistant"
			msg.Type = "assistant"
		case "system":
			msg.Role = "system"
			msg.Type = "system"
		default:
			continue
		}
		out = append(out, msg)
	}
	return out
}

func (m *AppModel) restoreSessionHistory(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	m.cancelProcessingUI()
	m.history = m.history[:0]
	m.actionHits = nil
	m.shell.ClearContent()
	m.shell.ClearLive()

	messages, err := m.adapter.LoadSessionMessages(context.Background(), id)
	if err != nil {
		m.appendSystem(err.Error(), "error")
		return
	}

	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		entry := historyEntry{
			content:   content,
			timestamp: time.Now(),
		}
		if msg.Timestamp > 0 {
			entry.timestamp = time.Unix(msg.Timestamp, 0)
		}
		switch strings.ToLower(strings.TrimSpace(msg.Role)) {
		case "assistant":
			entry.kind = "ai"
		case "system", "tool":
			entry.kind = "system"
			entry.level = "info"
		default:
			entry.kind = "user"
		}
		m.appendHistory(entry)
	}
}

func (m *AppModel) executionModeLabel(mode string) string {
	switch toolapi.NormalizeExecutionMode(mode) {
	case "plan":
		return "plan"
	default:
		return "auto"
	}
}

func normalizePlanPromptStyle(raw string) string {
	style := strings.TrimSpace(raw)
	if style == "" {
		return "concise"
	}
	lower := strings.ToLower(style)
	switch lower {
	case "concise", "detailed":
		return lower
	}
	if strings.HasPrefix(lower, "custom:") {
		body := strings.TrimSpace(style[len("custom:"):])
		if body == "" {
			return "concise"
		}
		return "custom:" + body
	}
	return "custom:" + style
}

func truncateBlock(text string, maxLines int, maxBytes int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	text = strings.ReplaceAll(text, "\r", "\n")
	if maxBytes > 0 {
		runes := []rune(text)
		if len(runes) > maxBytes {
			text = string(runes[:maxBytes]) + "\n[truncated]"
		}
	}
	lines := strings.Split(text, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = append(lines[:maxLines], "[truncated]")
	}
	return strings.Join(lines, "\n")
}

func blankFallback(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func optionalIntText(value *int, fallback string) string {
	if value == nil {
		return fallback
	}
	return fmt.Sprintf("%d", *value)
}

func optionalFloatText(value *float64, fallback string) string {
	if value == nil {
		return fallback
	}
	return fmt.Sprintf("$%.6f", *value)
}

func (m *AppModel) handleStatusSlash() tea.Cmd {
	modelName, modelBase := m.adapter.GetModelInfo()
	if modelName == "" {
		base, _, model, _ := m.adapter.ResolveAPIConfig()
		if modelName == "" {
			modelName = model
		}
		if modelBase == "" {
			modelBase = base
		}
	}
	snap, _ := m.adapter.PermissionSnapshot(context.Background())
	currentSessionID, _ := m.adapter.CurrentSessionID(context.Background())
	browser, _ := m.adapter.BrowserStatus(context.Background())

	lines := []string{
		m.localize("当前状态", "Status"),
		fmt.Sprintf("%s: %s", m.localize("工作区", "Workspace"), m.currentWorkspaceRoot()),
		fmt.Sprintf("%s: %s (%s)", m.localize("模型", "Model"), strings.TrimSpace(modelName), strings.TrimSpace(modelBase)),
		fmt.Sprintf("%s: %s", m.localize("执行模式", "Mode"), m.executionModeLabel(snap.ExecutionMode)),
		fmt.Sprintf("%s: %s", m.localize("访问模式", "Access mode"), snap.AccessMode),
		fmt.Sprintf("%s: %s", m.localize("审批模式", "Approval mode"), snap.ApprovalMode),
		fmt.Sprintf("%s: %s", m.localize("沙箱模式", "Sandbox mode"), snap.SandboxMode),
		fmt.Sprintf("%s: %s", m.localize("当前会话", "Session"), blankFallback(currentSessionID, m.localize("无", "none"))),
		fmt.Sprintf("%s: %s", m.localize("浏览器 MCP", "Browser MCP"), m.browserStatusLabel(browser)),
	}
	if strings.TrimSpace(snap.LastAuthorization) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("最近授权", "Last authorization"), snap.LastAuthorization))
	}
	if strings.TrimSpace(snap.LastAuthorizationTarget) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("最近升级目标", "Last escalation target"), snap.LastAuthorizationTarget))
	}
	if strings.TrimSpace(snap.LastAuthorizationNote) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("最近授权说明", "Last authorization note"), snap.LastAuthorizationNote))
	}
	if remote, ok, _ := m.adapter.CurrentRemoteRepo(context.Background()); ok {
		lines = append(lines,
			fmt.Sprintf("%s: %s/%s", m.localize("远程仓库", "Remote repo"), remote.Owner, remote.Repo),
			fmt.Sprintf("%s: %s", m.localize("远程分支", "Remote branch"), blankFallback(remote.WorkingBranch, remote.DefaultBranch)),
			fmt.Sprintf("%s: %s", m.localize("远程目录", "Remote path"), remote.LocalPath),
		)
	}
	if strings.TrimSpace(browser.LastError) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("浏览器错误", "Browser error"), browser.LastError))
	}

	// Context usage
	ctxWindowTokens, _ := m.adapter.ContextWindowTokens(context.Background())
	if ctxWindowTokens > 0 {
		lines = append(lines, fmt.Sprintf("%s: %d tokens", m.localize("上下文窗口", "Context window"), ctxWindowTokens))
	}

	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleRemoteSlash(args []string) tea.Cmd {
	_ = args
	remote, ok, err := m.adapter.CurrentRemoteRepo(context.Background())
	if err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}
	if !ok {
		m.appendSystem(m.localize("当前没有活跃的远程仓库上下文。", "No active remote repository context."), "info")
		return nil
	}
	lines := []string{
		m.localize("远程仓库上下文", "Remote repository context"),
		fmt.Sprintf("%s: %s", m.localize("平台", "Platform"), remote.Platform),
		fmt.Sprintf("%s: %s/%s", m.localize("仓库", "Repository"), remote.Owner, remote.Repo),
		fmt.Sprintf("%s: %s", m.localize("地址", "URL"), remote.RepoURL),
		fmt.Sprintf("%s: %s", m.localize("当前分支", "Current branch"), blankFallback(remote.WorkingBranch, remote.DefaultBranch)),
		fmt.Sprintf("%s: %s", m.localize("本地目录", "Local path"), remote.LocalPath),
	}
	if strings.TrimSpace(remote.AccountLogin) != "" || strings.TrimSpace(remote.AccountName) != "" {
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("账号", "Account"), blankFallback(remote.AccountLogin, remote.AccountName)))
	}
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) browserStatusLabel(status coreapi.BrowserStatus) string {
	switch {
	case status.Configured && status.Enabled && status.Loaded:
		return m.localize("已可用", "ready")
	case status.Configured && status.Enabled:
		return m.localize("已配置，待加载", "configured, pending load")
	case status.Configured:
		return m.localize("已配置，未启用", "configured, disabled")
	default:
		return m.localize("未配置", "not configured")
	}
}

func (m *AppModel) handleFastSlash() tea.Cmd {
	cfg, _ := config.Load()
	if cfg.FastModel == "" {
		m.appendSystem(m.localize("快速模型未配置。请在配置文件中设置 fast_model。", "Fast model not configured. Set fast_model in config."), "warning")
		return nil
	}

	// Toggle fast mode by switching to/from fast model
	currentModel, _ := m.adapter.GetModelInfo()
	if currentModel == cfg.FastModel {
		// Switch back to active model
		if active := cfg.Active; active != "" {
			m.adapter.SetActiveModel(active)
			_ = m.adapter.Reload()
			m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已切换回标准模型", "Switched back to standard model"), active), "success")
		}
	} else {
		m.adapter.SetActiveModel(cfg.FastModel)
		_ = m.adapter.Reload()
		m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已切换到快速模型", "Switched to fast model"), cfg.FastModel), "success")
	}
	return nil
}

func (m *AppModel) handleExportSlash(args []string) tea.Cmd {
	format := "markdown"
	path := ""

	if len(args) > 0 {
		f := strings.ToLower(strings.TrimSpace(args[0]))
		if f == "json" || f == "markdown" || f == "md" {
			format = f
			if format == "md" {
				format = "markdown"
			}
		}
	}
	if len(args) > 1 {
		path = strings.TrimSpace(args[1])
	}

	currentID, _ := m.adapter.CurrentSessionID(context.Background())
	if currentID == "" {
		m.appendSystem(m.localize("没有当前会话可导出。", "No current session to export."), "warning")
		return nil
	}

	if path == "" {
		ext := ".md"
		if format == "json" {
			ext = ".json"
		}
		path = filepath.Join(m.adapter.SessionsDir(context.Background()), currentID+ext)
	}

	if format == "json" {
		// Export as JSON
		messages := m.sessionTranscript()
		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("导出失败", "Export failed"), err), "error")
			return nil
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("导出失败", "Export failed"), err), "error")
			return nil
		}
	} else {
		if err := m.adapter.ExportSessionMarkdown(context.Background(), currentID, path); err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("导出失败", "Export failed"), err), "error")
			return nil
		}
	}

	m.appendSystem(fmt.Sprintf("%s: %s (%s)", m.localize("已导出会话", "Exported session"), path, format), "success")
	return nil
}

func (m *AppModel) handleThemeSlash(args []string) tea.Cmd {
	if len(args) == 0 {
		// Show current theme
		s, err := m.adapter.Settings(context.Background())
		if err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("读取主题失败", "Failed to read theme"), err), "error")
			return nil
		}
		current := s.Theme
		if current == "" {
			current = "dark"
		}
		m.appendSystem(fmt.Sprintf("%s: %s", m.localize("当前主题", "Current theme"), current), "info")
		return nil
	}

	theme := strings.ToLower(strings.TrimSpace(args[0]))
	s, err := m.adapter.Settings(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("读取主题失败", "Failed to read theme"), err), "error")
		return nil
	}
	s.Theme = theme
	if err := m.adapter.SaveSettings(context.Background(), s); err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("保存主题失败", "Failed to save theme"), err), "error")
		return nil
	}
	m.state.Theme = theme
	m.applyTheme(theme)
	m.refreshSettingsPanel()
	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已切换主题", "Switched theme to"), theme), "success")
	return nil
}

func (m *AppModel) handlePlanStyleSlash(args []string) tea.Cmd {
	s, err := m.adapter.Settings(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("读取计划提示风格失败", "Failed to read plan prompt style"), err), "error")
		return nil
	}
	current := normalizePlanPromptStyle(s.PlanPromptStyle)
	if len(args) == 0 {
		m.appendSystem(fmt.Sprintf("%s: %s", m.localize("当前计划提示风格", "Current plan prompt style"), current), "info")
		return nil
	}

	raw := strings.TrimSpace(strings.Join(args, " "))
	if strings.EqualFold(strings.TrimSpace(args[0]), "custom") {
		if len(args) == 1 {
			m.appendSystem(m.localize("用法: /plan-style [concise|detailed|custom:<text>]", "Usage: /plan-style [concise|detailed|custom:<text>]"), "warning")
			return nil
		}
		raw = "custom:" + strings.TrimSpace(strings.Join(args[1:], " "))
	}
	normalized := normalizePlanPromptStyle(raw)
	s.PlanPromptStyle = normalized

	root := m.currentWorkspaceRoot()
	if strings.TrimSpace(root) == "" {
		m.appendSystem(m.localize("没有可用工作区，无法保存计划提示风格。", "No workspace is available; cannot save plan prompt style."), "warning")
		return nil
	}
	if err := m.adapter.SaveSettings(context.Background(), s); err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("保存计划提示风格失败", "Failed to save plan prompt style"), err), "error")
		return nil
	}
	if settingsPanel, ok := m.panels["settings"].(*panels.SettingsPanel); ok && settingsPanel != nil {
		settingsPanel.SetSettings(&s)
	}

	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已设置计划提示风格", "Set plan prompt style to"), normalized), "success")
	return nil
}

func (m *AppModel) handleStatsSlash() tea.Cmd {
	stats, err := m.adapter.UsageSummary(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("读取统计失败", "Failed to read statistics"), err), "error")
		return nil
	}
	toolStats, err := m.adapter.ToolStats(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("读取工具统计失败", "Failed to read tool stats"), err), "error")
		return nil
	}

	lines := []string{
		m.localize("统计信息", "Statistics"),
		fmt.Sprintf("%s: %d", m.localize("对话轮数", "Rounds"), stats.Rounds),
		fmt.Sprintf("%s: %s", m.localize("输入 Tokens", "Input tokens"), optionalIntText(stats.InputTokens, m.localize("未知", "unknown"))),
		fmt.Sprintf("%s: %s", m.localize("输出 Tokens", "Reply tokens"), optionalIntText(stats.ReplyTokens, m.localize("未知", "unknown"))),
		fmt.Sprintf("%s: %s", m.localize("总 Tokens", "Total tokens"), optionalIntText(stats.TotalTokens, m.localize("未知", "unknown"))),
		fmt.Sprintf("%s: %s", m.localize("总成本", "Total cost"), optionalFloatText(stats.CostUSD, m.localize("未知", "unknown"))),
	}

	if len(toolStats) > 0 {
		lines = append(lines, m.localize("工具调用统计:", "Tool call stats:"))
		sort.Slice(toolStats, func(i, j int) bool { return toolStats[i].TotalCalls > toolStats[j].TotalCalls })
		for _, stat := range toolStats {
			lines = append(lines, fmt.Sprintf("  - %s: %d calls, avg %s", stat.Tool, stat.TotalCalls, stat.AvgDuration.Round(time.Millisecond)))
		}
	}

	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleRenameSlash(args []string) tea.Cmd {
	if len(args) == 0 {
		m.appendSystem(m.localize("用法: /rename <title>", "Usage: /rename <title>"), "warning")
		return nil
	}
	title := strings.TrimSpace(strings.Join(args, " "))
	currentID, _ := m.adapter.CurrentSessionID(context.Background())
	if currentID == "" {
		m.appendSystem(m.localize("没有当前会话。", "No current session."), "warning")
		return nil
	}
	if err := m.adapter.RenameSession(context.Background(), currentID, title); err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}
	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已重命名会话", "Renamed session to"), title), "success")
	return nil
}

func (m *AppModel) handleShareSlash() tea.Cmd {
	currentID, _ := m.adapter.CurrentSessionID(context.Background())
	if currentID == "" {
		m.appendSystem(m.localize("没有当前会话可分享。", "No current session to share."), "warning")
		return nil
	}

	messages := m.sessionTranscript()
	var sb strings.Builder
	sb.WriteString("# Session: ")
	sb.WriteString(currentID)
	sb.WriteString("\n\n")
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "unknown"
		}
		sb.WriteString(fmt.Sprintf("**%s**: %s\n\n", role, strings.TrimSpace(msg.Content)))
	}

	content := sb.String()

	// Try to copy to clipboard
	if err := copyToClipboard(content); err != nil {
		// Fallback: save to file
		path := filepath.Join(m.adapter.SessionsDir(context.Background()), currentID+"_shared.md")
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("分享失败", "Share failed"), err), "error")
			return nil
		}
		m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已保存到文件", "Saved to file"), path), "success")
		return nil
	}

	m.appendSystem(m.localize("已复制会话到剪贴板。", "Session copied to clipboard."), "success")
	return nil
}

func copyToClipboard(text string) error {
	// Try using clip command on Windows, pbcopy on macOS, xclip on Linux
	var cmd *exec.Cmd
	switch runpkg.GOOS {
	case "windows":
		cmd = exec.Command("clip")
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// applyTheme applies a theme by name
func (m *AppModel) applyTheme(name string) {
	// Theme is stored and will be applied on next render
	// The actual theme application happens in styles package
	m.updateContextUsageUI()
}
