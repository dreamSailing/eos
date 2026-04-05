package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/internal/config"
	plugpkg "github.com/dreamSailing/vb-coding/internal/pkg/plugins"
	"github.com/dreamSailing/vb-coding/internal/runtime"
	"github.com/dreamSailing/vb-coding/internal/toolapi"
	"github.com/dreamSailing/vb-coding/internal/tools"
	"github.com/dreamSailing/vb-coding/internal/tools/bg"
	gitops "github.com/dreamSailing/vb-coding/internal/tools/git"
	"github.com/dreamSailing/vb-coding/internal/ui/panels"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) localize(zh, en string) string {
	if m != nil && strings.EqualFold(m.state.Language, "en") {
		return en
	}
	return zh
}

func (m *AppModel) currentWorkspaceRoot() string {
	if m != nil && m.adapter != nil && m.adapter.GetCore() != nil {
		if root := strings.TrimSpace(m.adapter.GetCore().GetActiveRoot()); root != "" {
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
		"用法: /permissions [default|acceptEdits|plan|auto|dontAsk|bypassPermissions]（兼容 manual / bypass）",
		"Usage: /permissions [default|acceptEdits|plan|auto|dontAsk|bypassPermissions] (legacy manual / bypass aliases still work)",
	)
}

func (m *AppModel) gitOps() *gitops.Ops {
	return gitops.NewOpsWithRoot(m.currentWorkspaceRoot())
}

func (m *AppModel) openModelsPanel() {
	m.activeView = "panel"
	m.activePanel = "models"
	m.shell.ClearInput()
	if modelsPanel, ok := m.panels["models"].(*panels.ModelsPanel); ok && modelsPanel != nil {
		modelsPanel.Refresh()
		modelName, _ := m.adapter.GetModelInfo()
		modelsPanel.SetCurrentModel(modelName)
	}
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

func (m *AppModel) openSettingsPanel() {
	m.activeView = "panel"
	m.activePanel = "settings"
	m.shell.ClearInput()
	if settingsPanel, ok := m.panels["settings"].(*panels.SettingsPanel); ok && settingsPanel != nil {
		settingsPanel.LoadSettings()
	}
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
		m.adapter.GetCore().AddWorkspaceRoot(path)
		m.refreshWorkspacePanel()
		m.appendSystem(m.localize("已添加工作区: ", "Added workspace: ")+path, "success")
	case "remove":
		m.adapter.GetCore().RemoveWorkspaceRoot(path)
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

	if m.adapter.SetActiveModel(name) {
		_ = m.adapter.Reload()
		if modelsPanel, ok := m.panels["models"].(*panels.ModelsPanel); ok && modelsPanel != nil {
			modelsPanel.Refresh()
			modelsPanel.SetCurrentModel(name)
		}
		m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已切换模型", "Switched model"), name), "success")
		return nil
	}

	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("未找到模型", "Model not found"), name), "error")
	return nil
}

func (m *AppModel) handleSessionSlash(args []string) tea.Cmd {
	core := m.adapter.GetCore()
	if len(args) > 0 {
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "save":
			id, err := core.SaveSessionMessages(context.Background(), "", m.sessionTranscript())
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
				path = filepath.Join(core.SessionsDir(), id+".md")
			}
			if strings.TrimSpace(path) == "" {
				m.appendSystem(m.localize("用法: /session export <id> [outputPath]", "Usage: /session export <id> [outputPath]"), "warning")
				return nil
			}
			if err := core.SaveSessionMarkdown(id, path); err != nil {
				m.appendSystem(err.Error(), "error")
				return nil
			}
			m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已导出会话", "Exported session"), path), "success")
			return nil
		}
	}

	metas, err := core.ListSessions()
	if err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}
	currentID, _ := core.CurrentSessionID()
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
	core := m.adapter.GetCore()
	id := ""
	if len(args) > 0 {
		id = strings.TrimSpace(args[0])
	}
	if err := core.ResumeSession(context.Background(), id); err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}

	resolvedID, _ := core.CurrentSessionID()
	m.restoreSessionHistory(resolvedID)
	m.refreshContextPanel()
	m.refreshCostPanel()
	m.updateContextUsageUI()
	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已恢复会话", "Resumed session"), resolvedID), "success")
	return nil
}

func (m *AppModel) handlePermissionsSlash(args []string) tea.Cmd {
	core := m.adapter.GetCore()
	if len(args) > 0 {
		raw := strings.TrimSpace(args[0])
		if isSupportedExecutionModeInput(raw) {
			mode := toolapi.NormalizeExecutionMode(raw)
			core.SetExecutionMode(mode)
			m.state.ExecutionMode = mode
			m.shell.SetExecutionMode(mode)
			m.appendSystem(fmt.Sprintf("%s %s", m.localize("执行模式已切换为", "Execution mode switched to"), m.executionModeLabel(mode)), "success")
		} else {
			m.appendSystem(m.executionModeUsage(), "warning")
			return nil
		}
	}

	snap := core.PermissionSnapshot()
	lines := []string{
		m.localize("权限与审批状态", "Permissions and approvals"),
		fmt.Sprintf("%s: %s", m.localize("执行模式", "Execution mode"), m.executionModeLabel(snap.ExecutionMode)),
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
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handlePlanSlash(args []string) tea.Cmd {
	if len(args) > 0 && isSupportedExecutionModeInput(args[0]) {
		return m.handlePermissionsSlash(args)
	}

	items := tools.DefaultTodoStore().List()
	lines := []string{
		m.localize("当前计划与待办", "Current plan and todos"),
		fmt.Sprintf("%s: %s", m.localize("执行模式", "Execution mode"), m.executionModeLabel(m.adapter.GetCore().ExecutionMode())),
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
	lines = append(lines, m.localize("使用 /plan default|acceptEdits|plan|auto|dontAsk|bypassPermissions 可直接切换执行模式，旧别名 manual / bypass 也兼容。", "Use /plan default|acceptEdits|plan|auto|dontAsk|bypassPermissions to switch execution mode directly; legacy manual / bypass aliases still work."))
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleSkillsSlash(args []string) tea.Cmd {
	sm := m.adapter.GetCore().GetSkillManager()
	if sm == nil {
		m.appendSystem(m.localize("Skills 管理器尚未初始化。", "Skill manager is not initialized."), "warning")
		return nil
	}

	if len(args) > 0 && strings.EqualFold(args[0], "reload") {
		if err := sm.ReloadPreserveActive(); err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		m.appendSystem(m.localize("已重载 skills。", "Reloaded skills."), "success")
	}

	skills := sm.List()
	active := sm.GetActive()
	sort.Slice(skills, func(i, j int) bool { return strings.ToLower(skills[i].Name) < strings.ToLower(skills[j].Name) })
	lines := []string{fmt.Sprintf("%s: %d", m.localize("Skills", "Skills"), len(skills))}
	if len(skills) == 0 {
		lines = append(lines, m.localize("暂无可用 skills。", "No skills available."))
	} else {
		for _, skill := range skills {
			prefix := " "
			if _, ok := active[skill.Name]; ok {
				prefix = "*"
			}
			desc := strings.TrimSpace(skill.Description)
			if desc == "" {
				desc = m.localize("(无描述)", "(no description)")
			}
			origin := strings.TrimSpace(skill.Location)
			if strings.TrimSpace(skill.PluginName) != "" {
				if origin != "" {
					origin = "plugin:" + strings.TrimSpace(skill.PluginName) + "/" + origin
				} else {
					origin = "plugin:" + strings.TrimSpace(skill.PluginName)
				}
			}
			lines = append(lines, fmt.Sprintf("%s %s [%s] - %s", prefix, skill.Name, blankFallback(origin, m.localize("unknown", "unknown")), desc))
		}
	}
	lines = append(lines, m.localize("使用 /skills reload 重新扫描并保留当前激活状态。", "Use /skills reload to rescan while preserving active skills."))
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handlePluginSlash() tea.Cmd {
	type pluginRow struct {
		name        string
		description string
		source      string
		enabled     bool
	}
	rows := make([]pluginRow, 0)
	seen := map[string]struct{}{}
	cfg, _ := config.Load()
	for _, plugin := range plugpkg.DefaultRegistry().List() {
		if plugin == nil {
			continue
		}
		name := strings.TrimSpace(plugin.Name())
		if name == "" {
			continue
		}
		seen[strings.ToLower(name)] = struct{}{}
		enabled := plugpkg.DefaultRegistry().IsEnabled(name)
		if cfgEnabled, ok := config.PluginEnabled(&cfg, name); ok {
			enabled = cfgEnabled
		}
		rows = append(rows, pluginRow{
			name:        name,
			description: strings.TrimSpace(plugin.Description()),
			source:      strings.TrimSpace(plugpkg.MetadataOf(plugin).Source),
			enabled:     enabled,
		})
	}
	if discovered, err := plugpkg.Discover(m.currentWorkspaceRoot()); err == nil {
		for _, item := range discovered {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				continue
			}
			if _, ok := seen[strings.ToLower(name)]; ok {
				continue
			}
			enabled := true
			if cfgEnabled, ok := config.PluginEnabled(&cfg, name); ok {
				enabled = cfgEnabled
			}
			rows = append(rows, pluginRow{
				name:        name,
				description: strings.TrimSpace(item.Description),
				source:      "directory:" + strings.TrimSpace(item.Location),
				enabled:     enabled,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].name) < strings.ToLower(rows[j].name) })
	lines := []string{fmt.Sprintf("%s: %d", m.localize("插件", "Plugins"), len(rows))}
	if len(rows) == 0 {
		lines = append(lines, m.localize("暂无可用插件。", "No plugins available."))
	} else {
		for _, plugin := range rows {
			status := m.localize("enabled", "enabled")
			if !plugin.enabled {
				status = m.localize("disabled", "disabled")
			}
			desc := strings.TrimSpace(plugin.description)
			if desc == "" {
				desc = m.localize("(无描述)", "(no description)")
			}
			lines = append(lines, fmt.Sprintf("- %s [%s, %s]: %s", plugin.name, blankFallback(plugin.source, "plugin"), status, desc))
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
	core := m.adapter.GetCore()
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
	snap := core.PermissionSnapshot()
	sessions, _ := core.ListSessions()
	currentSessionID, _ := core.CurrentSessionID()
	bgTasks := bg.Default().List()
	agentTasks := runtime.DefaultAgentRegistry().ListSnapshots()
	todos := tools.DefaultTodoStore().List()
	traces := m.adapter.GetTools().GetToolTraces()
	stats := m.adapter.GetTools().GetToolStats()
	skillsCount := 0
	if sm := core.GetSkillManager(); sm != nil {
		skillsCount = len(sm.List())
	}
	pluginsCount := len(plugpkg.DefaultRegistry().List())
	if discovered, err := plugpkg.Discover(m.currentWorkspaceRoot()); err == nil {
		seen := map[string]struct{}{}
		for _, item := range plugpkg.DefaultRegistry().List() {
			if item == nil {
				continue
			}
			seen[strings.ToLower(strings.TrimSpace(item.Name()))] = struct{}{}
		}
		for _, item := range discovered {
			if _, ok := seen[strings.ToLower(strings.TrimSpace(item.Name))]; ok {
				continue
			}
			pluginsCount++
		}
	}

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
		fmt.Sprintf("%s: %d", m.localize("可用 skills", "Available skills"), skillsCount),
		fmt.Sprintf("%s: %d", m.localize("已注册插件", "Registered plugins"), pluginsCount),
		fmt.Sprintf("%s: %d", m.localize("工具追踪数", "Tool traces"), len(traces)),
	}
	if len(stats) > 0 {
		lines = append(lines, m.localize("工具统计:", "Tool stats:"))
		type statLine struct {
			name  string
			calls int
			avg   time.Duration
		}
		var items []statLine
		for name, stat := range stats {
			if stat == nil {
				continue
			}
			items = append(items, statLine{name: name, calls: stat.TotalCalls, avg: stat.AvgDuration})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].calls == items[j].calls {
				return items[i].name < items[j].name
			}
			return items[i].calls > items[j].calls
		})
		if len(items) > 5 {
			items = items[:5]
		}
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("- %s: calls=%d avg=%s", item.name, item.calls, item.avg.Round(time.Millisecond)))
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
	diag := strings.TrimSpace(core.ProblemsAndDiagnosticsMarkdown())
	if diag != "" {
		lines = append(lines, m.localize("LSP 诊断摘要:", "LSP diagnostics:"))
		lines = append(lines, truncateBlock(diag, 12, 1200))
	}
	m.appendSystem(strings.Join(lines, "\n"), "info")
	return nil
}

func (m *AppModel) handleDiffSlash(args []string) tea.Cmd {
	core := m.adapter.GetCore()
	if len(args) == 0 {
		diff := strings.TrimSpace(core.GetPendingDiff())
		if diff != "" {
			target := strings.TrimSpace(core.GetPendingDiffPath())
			if target == "" {
				target = m.localize("(当前待审批改动)", "(current pending edit)")
			}
			m.appendSystem(fmt.Sprintf("%s: %s\n%s", m.localize("待审批 diff", "Pending diff"), target, truncateBlock(diff, 40, 5000)), "info")
			return nil
		}
		changes, err := m.gitOps().Status()
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
	diff, err := m.gitOps().Diff(path)
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
		if diff, err := m.gitOps().Diff(path); err == nil && strings.TrimSpace(diff) != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", m.localize("目标文件", "Target file"), path))
			lines = append(lines, truncateBlock(diff, 40, 5000))
		} else if err != nil {
			lines = append(lines, fmt.Sprintf("%s: %v", m.localize("读取 diff 失败", "Failed to read diff"), err))
		}
	} else if diff := strings.TrimSpace(m.adapter.GetCore().GetPendingDiff()); diff != "" {
		target := blankFallback(m.adapter.GetCore().GetPendingDiffPath(), m.localize("(当前待审批改动)", "(current pending edit)"))
		lines = append(lines, fmt.Sprintf("%s: %s", m.localize("待审改动", "Pending change"), target))
		lines = append(lines, truncateBlock(diff, 40, 5000))
	} else {
		changes, err := m.gitOps().Status()
		if err == nil && len(changes) > 0 {
			lines = append(lines, m.localize("当前 Git 改动:", "Current Git changes:"))
			for _, change := range changes {
				lines = append(lines, fmt.Sprintf("- [%s] %s", change.State, change.Path))
			}
		}
	}

	diag := strings.TrimSpace(m.adapter.GetCore().ProblemsAndDiagnosticsMarkdown())
	if diag != "" {
		lines = append(lines, m.localize("诊断:", "Diagnostics:"))
		lines = append(lines, truncateBlock(diag, 12, 1200))
	}

	traces := m.adapter.GetTools().GetToolTraces()
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
	ops := m.gitOps()
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "status":
		changes, err := ops.Status()
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
		branches, current, err := ops.BranchList()
		if err != nil {
			m.appendSystem(err.Error(), "error")
			return nil
		}
		lines := []string{fmt.Sprintf("%s: %s", m.localize("当前分支", "Current branch"), current)}
		for _, branch := range branches {
			lines = append(lines, "- "+branch)
		}
		m.appendSystem(strings.Join(lines, "\n"), "info")
	case "log":
		out, err := ops.Log(20, true, false, false, "")
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
		out, err := ops.Show(revision, path)
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

func (m *AppModel) sessionTranscript() []bridge.SessionTranscriptMessage {
	out := make([]bridge.SessionTranscriptMessage, 0, len(m.history))
	for _, item := range m.history {
		content := strings.TrimSpace(item.content)
		if content == "" {
			continue
		}
		msg := bridge.SessionTranscriptMessage{
			Content:   content,
			Timestamp: item.timestamp.Unix(),
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
	m.copyHits = nil
	m.shell.ClearContent()
	m.shell.ClearLive()

	messages, err := m.adapter.GetCore().LoadSessionMessages(id)
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
	case "default":
		return m.localize("默认只读", "default")
	case "acceptEdits":
		return m.localize("接受编辑", "accept edits")
	case "plan":
		return m.localize("先出计划", "plan first")
	case "dontAsk":
		return m.localize("拒绝询问", "don't ask")
	case "bypassPermissions":
		return m.localize("绕过审批", "bypass permissions")
	default:
		return m.localize("自动", "auto")
	}
}

func truncateBlock(text string, maxLines int, maxBytes int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	text = strings.ReplaceAll(text, "\r", "\n")
	if maxBytes > 0 && len(text) > maxBytes {
		text = text[:maxBytes]
	}
	lines := strings.Split(text, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = append(lines[:maxLines], "...")
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
