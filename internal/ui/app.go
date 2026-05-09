package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/i18n"
	mcppkg "github.com/dreamSailing/eos/internal/mcp"
	"github.com/dreamSailing/eos/internal/memory"
	"github.com/dreamSailing/eos/internal/pkg/clip"
	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/state"
	"github.com/dreamSailing/eos/internal/tools/bg"
	"github.com/dreamSailing/eos/internal/ui/adapter"
	"github.com/dreamSailing/eos/internal/ui/components/messages"
	"github.com/dreamSailing/eos/internal/ui/features/slash"
	"github.com/dreamSailing/eos/internal/ui/panels"
	"github.com/dreamSailing/eos/internal/ui/styles"
	"github.com/dreamSailing/eos/internal/ui/views/confirm"
	"github.com/dreamSailing/eos/internal/ui/views/help"
	"github.com/dreamSailing/eos/internal/ui/views/setup"
	"github.com/dreamSailing/eos/internal/ui/views/shell"
	"github.com/dreamSailing/eos/internal/update"
	"github.com/dreamSailing/eos/internal/version"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

// AppState 应用程序状态
type AppState struct {
	Mode          string // "ai", "bash"
	Processing    bool
	Thinking      bool
	Language      string
	Theme         string
	ExecutionMode string
}

// AppModel 主应用模型
type AppModel struct {
	width, height int
	state         AppState

	adapter *adapter.RuntimeAdapter
	styles  *styles.Styles

	// 消息渲染器
	msgRenderer *messages.Renderer

	// 主视图
	shell *shell.Model

	// 面板系统
	panels      map[string]panels.Panel
	activePanel string

	// 其他视图
	helpView    *help.HelpView
	setupView   any // 可以是 *setup.SetupView 或 *setup.ModelSetupView
	confirmView *confirm.Model
	prevView    string

	// 视图状态
	activeView       string // "shell", "panel", "help", "setup", "confirm"
	initialSetupFlow bool

	// 消息跟踪
	currentAIStartTime time.Time
	currentAITokens    int
	aiLive             strings.Builder
	thinkingLive       strings.Builder
	thinkingExpanded   bool
	toolInflight       map[string]toolTrack
	history            []historyEntry
	delegatedThisRound bool
	lastAgentFinal     string

	copyHits []copyHit

	pendingImagePaths []string

	predictionText        string
	predictionSeq         int
	predictionDebounceSeq int
	predictionEnabled     bool

	trustPendingPath   string
	trustPendingAction string
	activeCancel       context.CancelFunc
	stopRequested      bool
}

type copyHit struct {
	y    int
	x0   int
	x1   int
	idx  int
	text string
}

type ctxUsageTickMsg struct{}
type predictionDebounceMsg struct {
	Seq   int
	Draft string
}

func (m *AppModel) ctxUsageTick() tea.Cmd {
	return tea.Tick(900*time.Millisecond, func(time.Time) tea.Msg { return ctxUsageTickMsg{} })
}

func (m *AppModel) updateContextUsageUI() {
	if m == nil || m.shell == nil || m.adapter == nil {
		return
	}
	if len(m.history) > 0 || m.state.Processing {
		m.shell.SetContextVisible(true)
	}
	cm := m.adapter.GetContext()
	if cm == nil {
		return
	}
	_, tokens, ratio := cm.GetCurrentUsage()
	m.shell.SetContextUsage(tokens, ratio)
}

func (m *AppModel) updateBGTaskCountUI() {
	if m == nil || m.shell == nil {
		return
	}
	m.shell.SetBGTaskCount(len(bg.Default().List()))
}

func (m *AppModel) clearPrediction() {
	if m == nil {
		return
	}
	m.predictionSeq++
	m.predictionDebounceSeq++
	m.predictionText = ""
	if m.shell != nil {
		m.shell.ClearPrediction()
	}
}

func (m *AppModel) syncPredictionState() {
	if m == nil || m.shell == nil {
		return
	}
	if !m.shell.HasPrediction() {
		m.predictionText = ""
	}
}

func (m *AppModel) canPredict() bool {
	return m != nil &&
		m.adapter != nil &&
		m.shell != nil &&
		m.predictionEnabled &&
		m.activeView == "shell" &&
		m.state.Mode == "ai" &&
		!m.state.Processing
}

func (m *AppModel) schedulePrediction(draft string) tea.Cmd {
	if !m.canPredict() {
		m.clearPrediction()
		return nil
	}
	m.predictionDebounceSeq++
	seq := m.predictionDebounceSeq
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg {
		return predictionDebounceMsg{Seq: seq, Draft: draft}
	})
}

func (m *AppModel) requestPrediction(draft string) tea.Cmd {
	if m == nil || m.adapter == nil || m.shell == nil {
		return nil
	}
	if !m.canPredict() {
		m.clearPrediction()
		return nil
	}
	m.predictionSeq++
	seq := m.predictionSeq
	return func() tea.Msg {
		text, err := m.adapter.GetCore().PredictNextUserMessage(context.Background(), draft)
		if err != nil {
			return PredictionUpdateMsg{Seq: seq, Draft: draft}
		}
		return PredictionUpdateMsg{Seq: seq, Draft: draft, Text: text}
	}
}

func (m *AppModel) refreshShellWelcomeInfo() {
	if m == nil || m.shell == nil || m.adapter == nil {
		return
	}
	modelName, modelBase := resolveShellWelcomeInfo(m.adapter)
	m.shell.SetWelcomeInfo(modelName, modelBase, "")
}

func (m *AppModel) refreshAILive() {
	if m == nil || m.shell == nil {
		return
	}
	if !m.state.Processing {
		return
	}
	var blocks []string
	thinking := strings.TrimSpace(m.thinkingLive.String())
	thinkingShown := state.Thinking() && thinking != ""
	if state.Thinking() && thinking != "" {
		if m.msgRenderer != nil {
			blocks = append(blocks, m.msgRenderer.RenderThinking(thinking, time.Since(m.currentAIStartTime), m.thinkingExpanded, nil))
		} else {
			blocks = append(blocks, thinking)
		}
	}
	live := strings.TrimSpace(m.aiLive.String())
	liveShown := live != ""
	if live != "" {
		if m.msgRenderer != nil {
			blocks = append(blocks, m.msgRenderer.RenderAIResponseAt(live, 0, 0, false, time.Now()))
		} else {
			blocks = append(blocks, live)
		}
	}
	if len(blocks) == 0 {
		m.shell.ClearLive()
		return
	}
	m.shell.SetStatusHints(thinkingShown || liveShown, thinkingShown)
	m.shell.SetLive(strings.Join(blocks, "\n\n"))
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func (m *AppModel) copyMarks() []string {
	marks := []string{
		i18n.T("op.copy", m.state.Language),
		i18n.T("op.copied", m.state.Language),
		"Copy",
		"Copied",
		"已复制",
	}
	uniq := make([]string, 0, len(marks))
	seen := map[string]struct{}{}
	for _, s := range marks {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		uniq = append(uniq, s)
	}
	return uniq
}

type clearCopiedMsg struct {
	idx int
}

type toolTrack struct {
	name    string
	started time.Time
	idx     int
	params  map[string]any
}

type historyEntry struct {
	kind        string
	content     string
	timestamp   time.Time
	tokens      int
	duration    time.Duration
	level       string
	toolID      string
	toolName    string
	toolParams  map[string]any
	toolOutput  string
	toolSuccess bool
	toolStatus  string
	agentName   string
	task        string
	copiedAt    time.Time
}

func resolveShellWelcomeInfo(adapter *adapter.RuntimeAdapter) (string, string) {
	modelName, modelBase := adapter.GetModelInfo()
	if modelName == "" || modelBase == "" {
		base, _, mdl, _ := adapter.ResolveAPIConfig()
		if modelName == "" {
			modelName = mdl
		}
		if modelBase == "" {
			modelBase = base
		}
	}
	if modelName == "" {
		modelName = "(none)"
	}
	if modelBase == "" {
		modelBase = "(none)"
	}
	return modelName, modelBase
}

// NewAppModel 创建新的应用模型
func NewAppModel(core *bridge.RuntimeCore) *AppModel {
	adapter := adapter.NewRuntimeAdapter(core)
	theme := styles.GetTheme("dark")
	styles := styles.NewStyles(theme)

	// 加载配置
	cfg, _ := config.Load()
	lang := cfg.Language
	if lang == "" {
		lang = "zh"
	}
	predictionEnabled := config.NextMessagePredictionEnabled(&cfg)

	// 创建Shell视图
	shellModel := shell.New(80, 24, styles, lang)
	modelName, modelBase := resolveShellWelcomeInfo(adapter)
	shellModel.SetWelcomeInfo(modelName, modelBase, "")
	shellModel.SetExecutionMode("auto")
	shellModel.SetThinkingExpanded(false)
	adapter.GetCore().SetExecutionMode("auto")

	// 创建面板
	panelMap := make(map[string]panels.Panel)
	panelMap["context"] = panels.NewContextPanel(styles, lang)
	panelMap["memory"] = panels.NewMemoryPanel(styles, lang)
	panelMap["rules"] = panels.NewRulesPanel(styles, lang)
	panelMap["workspace"] = panels.NewWorkspacePanel(styles, lang)
	lspPanel := panels.NewLSPPanel(styles, lang)
	panelMap["lsp"] = lspPanel

	// 创建模型面板（内部会自动从配置文件加载当前模型）
	panelMap["models"] = panels.NewModelsPanel(styles, lang)

	panelMap["settings"] = panels.NewSettingsPanel(styles, adapter.GetSettings(), lang)
	mcpPanel := panels.NewMCPPanel(styles, lang)
	// 加载 MCP 服务器配置
	var mcpServers []panels.MCPServer
	configServers := adapter.GetCore().ListMCPServers()
	for _, s := range configServers {
		mcpServers = append(mcpServers, panels.MCPServer{
			Name:    s.Name,
			Type:    string(s.Type),
			Enabled: s.Enabled,
		})
	}
	mcpPanel.SetServers(mcpServers)
	panelMap["mcp"] = mcpPanel

	panelMap["cost"] = panels.NewCostPanel(styles, lang)
	panelMap["versions"] = panels.NewVersionsPanel(styles)
	panelMap["tasks"] = panels.NewTasksPanel(styles, lang)

	setupView := any(setup.NewSetupView(styles))
	activeView := "shell"
	initialSetupFlow := false
	if len(cfg.Models) == 0 {
		base, key, model, _ := adapter.ResolveAPIConfig()
		if strings.TrimSpace(base) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(model) == "" {
			wizard := setup.NewModelSetupWizard(styles, lang)
			wizard.SetSize(80, 24)
			setupView = wizard
			activeView = "setup"
			initialSetupFlow = true
			shellModel.BlurInput()
		}
	}

	return &AppModel{
		state: AppState{
			Mode:          "ai",
			Language:      lang,
			Theme:         "dark",
			ExecutionMode: "auto",
		},
		adapter:           adapter,
		styles:            styles,
		msgRenderer:       messages.NewRenderer(styles, 80),
		shell:             &shellModel,
		panels:            panelMap,
		helpView:          help.NewHelpView(styles, lang),
		setupView:         setupView,
		activeView:        activeView,
		initialSetupFlow:  initialSetupFlow,
		activePanel:       "",
		toolInflight:      make(map[string]toolTrack),
		history:           make([]historyEntry, 0, 128),
		predictionEnabled: predictionEnabled,
	}
}

// Init 初始化应用模型
func (m *AppModel) Init() tea.Cmd {
	// 初始化时设置默认大小，防止一直显示 Loading...
	m.width = 80
	m.height = 24

	if p, _ := os.Getwd(); p != "" {
		abs := normalizeWorkspacePath(p)
		rememberKnownWorkspace(abs, true)
		if m.isWorkspaceTrusted(abs) {
			m.adapter.GetCore().StartContextEngine(abs)
			settingsPath := filepath.Join(abs, ".eos", "settings.json")
			m.adapter.GetSettings().SetPath(settingsPath)
			_, _ = m.adapter.GetCore().LoadSettings(settingsPath)
		} else {
			m.trustPendingPath = abs
			m.trustPendingAction = "init"
			m.openWorkspaceTrustConfirm(abs)
		}
	}

	// 确保 shell 和所有面板都有初始大小
	m.shell.SetSize(m.width, m.height)
	m.helpView.SetSize(m.width, m.height)
	switch sv := m.setupView.(type) {
	case *setup.SetupView:
		sv.SetSize(m.width, m.height)
	case *setup.ModelSetupView:
		sv.SetSize(m.width, m.height)
	}
	for _, p := range m.panels {
		p.SetSize(m.width, m.height)
	}

	// 启动事件监听（使用非阻塞方式）
	return tea.Batch(
		m.shell.Init(),
		m.ctxUsageTick(),
		func() tea.Msg {
			select {
			case event := <-m.adapter.Events():
				return event
			default:
				return nil
			}
		},
	)
}

// checkForUpdates checks for updates in the background
func (m *AppModel) checkForUpdates() tea.Cmd {
	return func() tea.Msg {
		result, err := update.CheckLatest(context.Background())
		if err != nil || result == nil {
			return nil
		}
		return VersionCheckMsg{Result: result}
	}
}

func (m *AppModel) handleVersionCheck(msg VersionCheckMsg) {
	if msg.Result != nil && msg.Result.HasUpdate {
		m.shell.SetUpdateInfo(msg.Result)
	}
}

// Update 更新应用状态
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ctxUsageTickMsg:
		if m.activeView == "panel" && m.activePanel == "context" {
			m.refreshContextPanel()
		} else if m.activeView == "panel" && m.activePanel == "memory" {
			m.refreshMemoryPanel()
		}
		cmds = append(cmds, m.ctxUsageTick())

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.shell.SetSize(msg.Width, msg.Height)
		m.helpView.SetSize(msg.Width, msg.Height)
		if m.confirmView != nil {
			m.confirmView.SetSize(msg.Width, msg.Height)
		}
		// 更新消息渲染器宽度
		if m.msgRenderer != nil {
			m.msgRenderer.SetWidth(msg.Width - 4)
		}
		m.refreshAILive()
		m.rebuildHistoryContent()
		// 更新 setup 视图大小
		switch sv := m.setupView.(type) {
		case *setup.SetupView:
			sv.SetSize(msg.Width, msg.Height)
		case *setup.ModelSetupView:
			sv.SetSize(msg.Width, msg.Height)
		}
		for _, p := range m.panels {
			p.SetSize(msg.Width, msg.Height)
		}

	case tea.MouseMsg:
		if m.activeView == "shell" && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if cmd := m.tryCopyBubbleAt(msg.X, msg.Y); cmd != nil {
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		}
		if m.activeView == "shell" {
			updatedShell, shellCmd := m.shell.Update(msg)
			*m.shell = updatedShell
			if shellCmd != nil {
				cmds = append(cmds, shellCmd)
			}
		} else if m.activeView == "panel" && m.activePanel != "" {
			if panel, ok := m.panels[m.activePanel]; ok {
				updatedPanel, cmd := panel.Update(msg)
				m.panels[m.activePanel] = updatedPanel
				return m, m.handlePanelMsg(cmd)
			}
		} else if m.activeView == "help" && m.helpView != nil {
			updated, cmd := m.helpView.Update(msg)
			m.helpView = updated
			return m, cmd
		}

	case tea.KeyMsg:
		// 首先检查是否处于特殊视图
		if m.activeView == "confirm" && m.confirmView != nil {
			updated, cmd := m.confirmView.Update(msg)
			m.confirmView = updated
			return m, cmd
		}

		if m.activeView == "help" {
			if msg.String() == "esc" || msg.String() == "q" {
				m.activeView = "shell"
				m.shell.ClearInput()
				return m, nil
			}
			updated, cmd := m.helpView.Update(msg)
			m.helpView = updated
			return m, cmd
		}

		if m.activeView == "setup" {
			switch sv := m.setupView.(type) {
			case *setup.SetupView:
				updated, cmd := sv.Update(msg)
				m.setupView = updated
				return m, cmd
			case *setup.ModelSetupView:
				updated, cmd := sv.Update(msg)
				m.setupView = updated
				return m, cmd
			case *setup.MCPConfigEditorView:
				updated, cmd := sv.Update(msg)
				m.setupView = updated
				return m, cmd
			}
		}

		if m.activeView == "panel" && m.activePanel != "" {
			if msg.String() == "esc" {
				if m.activePanel == "context" {
					if p, ok := m.panels[m.activePanel].(*panels.ContextPanel); ok && p != nil && p.IsViewing() {
						p.ResetView()
						m.panels[m.activePanel] = p
						return m, nil
					}
				}
				if m.activePanel == "tasks" {
					if p, ok := m.panels[m.activePanel].(*panels.TasksPanel); ok && p != nil && p.IsViewing() {
						p.ResetView()
						m.panels[m.activePanel] = p
						return m, nil
					}
				}
				if m.activePanel == "rules" {
					if p, ok := m.panels[m.activePanel].(*panels.RulesPanel); ok && p != nil && p.IsEditing() {
						p.CancelEdit()
						m.panels[m.activePanel] = p
						return m, nil
					}
				}
				if m.activePanel == "memory" {
					if p, ok := m.panels[m.activePanel].(*panels.MemoryPanel); ok && p != nil && p.IsEditing() {
						p.CancelEdit()
						m.panels[m.activePanel] = p
						return m, nil
					}
				}
				m.activeView = "shell"
				m.activePanel = ""
				m.shell.ClearInput()
				return m, nil
			}
			if panel, ok := m.panels[m.activePanel]; ok {
				updatedPanel, cmd := panel.Update(msg)
				m.panels[m.activePanel] = updatedPanel
				return m, m.handlePanelMsg(cmd)
			}
		}

		// 处理全局快捷键
		if cmd := m.handleGlobalKey(msg); cmd != nil {
			cmds = append(cmds, cmd)
		} else {
			inputBeforeKey := m.shell.GetInputValue()

			// 检查是否是 / 键（显示斜杠命令提示）- 只在输入框为空时触发
			if msg.String() == "/" && m.shell.GetInputValue() == "" {
				cmds = append(cmds, func() tea.Msg {
					return ShowHintsMsg{Type: "slash"}
				})
			}

			// 检查是否是 @ 键（显示路径提示）- 只在输入框为空时触发
			if msg.String() == "@" && m.shell.GetInputValue() == "" {
				cmds = append(cmds, func() tea.Msg {
					return ShowHintsMsg{Type: "path"}
				})
			}

			hintsVisibleBeforeKey := m.shell.IsHintsVisible()

			// 让 Shell 处理按键（处理 Enter, Esc, 历史导航等）
			handled, cmd := m.shell.HandleKey(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.syncPredictionState()

			// 如果 Shell 处理了非 Enter 键，不需要后续处理
			// 如果按 Enter 且 hints 显示，隐藏 hints 不发送
			// 如果按 Enter 且 hints 不显示，检查是否需要发送
			shouldUpdateShell := true
			shouldRefreshHints := true
			if hintsVisibleBeforeKey && handled {
				switch msg.String() {
				case "up", "down", "enter", "tab", "esc":
					shouldUpdateShell = false
					shouldRefreshHints = false
				}
			}
			if hintsVisibleBeforeKey && handled && (msg.String() == "enter" || msg.String() == "tab") {
				shouldUpdateShell = false
			}
			if msg.String() == "enter" {
				if hintsVisibleBeforeKey {
					shouldUpdateShell = false // hints 已处理，不需要再更新 shell
					shouldRefreshHints = false
				} else {
					shouldSend, exitCmd := m.shouldSendMessage()
					if exitCmd != nil {
						// /exit 命令，返回退出命令
						cmds = append(cmds, exitCmd)
						return m, tea.Batch(cmds...)
					}
					if shouldSend {
						cmds = append(cmds, m.sendMessage())
					} else if m.shell.GetMode() == shell.ModeBash && !m.state.Processing && strings.TrimSpace(m.shell.GetInputValue()) != "" {
						cmds = append(cmds, m.sendBashCommand())
					}
					// shouldSendMessage 返回 false 时，如果是斜杠命令已处理，输入框已清空，跳过 shell 更新
					// 否则（输入框为空等情况），继续更新 shell
					if m.shell.GetInputValue() == "" {
						shouldUpdateShell = false
					}
				}
			}

			// 更新 Shell 状态（确保输入能够显示）
			if shouldUpdateShell {
				updatedShell, shellCmd := m.shell.Update(msg)
				*m.shell = updatedShell
				m.syncPredictionState()
				if shellCmd != nil {
					cmds = append(cmds, shellCmd)
				}
			}

			// 输入变化后更新 hints
			if shouldRefreshHints {
				m.updateHintsBasedOnInput()
			}

			inputAfterKey := m.shell.GetInputValue()
			if inputAfterKey != inputBeforeKey {
				if cmd := m.schedulePrediction(inputAfterKey); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}

	case bridge.Event:
		return m.handleBridgeEvent(msg)

	case predictionDebounceMsg:
		if msg.Seq != m.predictionDebounceSeq {
			return m, nil
		}
		if !m.canPredict() {
			m.clearPrediction()
			return m, nil
		}
		if m.shell.GetInputValue() != msg.Draft {
			return m, nil
		}
		if cmd := m.requestPrediction(msg.Draft); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case AIResponseMsg:
		if msg.Type == "delta" && !m.state.Processing {
			return m, nil
		}
		if m.delegatedThisRound && msg.Type == "delta" {
			return m, nil
		}
		cmd := m.handleAIResponse(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case InvokeDoneMsg:
		if !m.state.Processing {
			return m, nil
		}
		content := msg.Content
		if content == "" {
			content = strings.TrimSpace(m.aiLive.String())
		}
		if strings.Contains(content, "agent.task:") || strings.Contains(content, "agent.final:") {
			m.cancelProcessingUI()
			return m, nil
		}
		cmd := m.handleAIResponse(AIResponseMsg{Type: "final", Content: content})
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case PredictionUpdateMsg:
		if msg.Seq != m.predictionSeq {
			return m, nil
		}
		if !m.canPredict() {
			m.clearPrediction()
			return m, nil
		}
		if m.shell.GetInputValue() != msg.Draft {
			return m, nil
		}
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			m.clearPrediction()
			return m, nil
		}
		currentInput := m.shell.GetInputValue()
		if currentInput != "" {
			if !strings.HasPrefix(text, currentInput) || text == currentInput {
				m.clearPrediction()
				return m, nil
			}
		}
		m.predictionText = text
		m.shell.SetPrediction(text)

	case ErrorMsg:
		m.appendHistory(historyEntry{kind: "system", content: msg.Err.Error(), level: "error"})
		m.cancelProcessingUI()

	case ThinkingMsg:
		m.state.Thinking = true
		m.thinkingLive.WriteString(msg.Content)
		m.shell.SetThinking(true, "")
		m.refreshAILive()
		if !msg.Done {
			cmds = append(cmds, m.shell.StatusTick())
		}

	case ToolCallMsg:
		cmd := m.handleToolCall(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case ToolResultMsg:
		cmd := m.handleToolResult(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case AgentTaskMsg:
		m.delegatedThisRound = true
		m.shell.ClearLive()
		m.aiLive.Reset()
		m.appendHistory(historyEntry{kind: "agent.task", agentName: msg.AgentName, task: msg.Task, timestamp: time.Now()})

	case AgentFinalMsg:
		m.delegatedThisRound = true
		m.shell.ClearLive()
		m.lastAgentFinal = msg.Content
		m.appendHistory(historyEntry{kind: "agent.final", agentName: msg.AgentName, content: msg.Content, timestamp: time.Now()})
		m.state.Processing = false
		m.shell.SetProcessing(false)

	case PromptRequestMsg:
		if m.confirmView == nil {
			m.prevView = m.activeView
		}
		req := confirm.Request{
			ID:        msg.ID,
			Kind:      strings.TrimSpace(msg.Kind),
			Title:     strings.TrimSpace(msg.Title),
			Question:  strings.TrimSpace(msg.Question),
			Options:   msg.Options,
			Diff:      msg.Diff,
			DiffPath:  msg.DiffPath,
			AllowText: msg.AllowText,
			TextHint:  msg.TextHint,
		}
		if req.Kind == "" {
			req.Kind = "permission"
		}
		if len(req.Options) == 0 {
			if req.Kind == "permission" {
				req.Options = []string{"allow_once", "allow_session", "deny"}
			} else {
				req.Options = []string{"OK"}
			}
		}
		m.confirmView = confirm.New(m.styles, m.state.Language, req)
		m.confirmView.SetSize(m.width, m.height)
		m.activeView = "confirm"
		m.shell.BlurInput()
		return m, nil

	case confirm.ResultMsg:
		if strings.HasPrefix(msg.Kind, "bg_kill:") {
			id, _ := strings.CutPrefix(msg.Kind, "bg_kill:")
			id = strings.TrimSpace(id)
			if msg.Decision == "confirm" && id != "" {
				_, err := bg.Default().Kill(id)
				if err != nil {
					m.appendSystem(fmt.Sprintf("终止后台任务失败: %v", err), "error")
				} else {
					m.appendSystem(fmt.Sprintf("已终止后台任务: %s", id), "warning")
				}
			}
			m.confirmView = nil
			if m.prevView != "" {
				m.activeView = m.prevView
				m.prevView = ""
			} else {
				m.activeView = "shell"
			}
			m.shell.FocusInput()
			return m, nil
		}
		if msg.Kind == "workspace_trust" {
			path := strings.TrimSpace(m.trustPendingPath)
			action := strings.TrimSpace(m.trustPendingAction)
			if msg.Decision == "cancel" || path == "" {
				return m, tea.Quit
			}
			if msg.OptionIndex != 0 {
				return m, tea.Quit
			}
			m.addTrustedWorkspace(path)
			m.trustPendingPath = ""
			m.trustPendingAction = ""
			m.confirmView = nil
			if m.prevView != "" {
				m.activeView = m.prevView
				m.prevView = ""
			} else {
				m.activeView = "shell"
			}
			switch action {
			case "init":
				rememberKnownWorkspace(path, true)
				m.adapter.GetCore().StartContextEngine(path)
				settingsPath := filepath.Join(path, ".eos", "settings.json")
				m.adapter.GetSettings().SetPath(settingsPath)
				_, _ = m.adapter.GetCore().LoadSettings(settingsPath)
				m.refreshWorkspacePanel()
				m.refreshRulesPanel()
				m.refreshLSPPanel()
			case "switch":
				cmd := m.switchWorkspaceTrusted(path)
				if m.activeView == "shell" {
					m.shell.FocusInput()
				}
				return m, cmd
			}
			if m.activeView == "shell" {
				m.shell.FocusInput()
			}
			return m, nil
		}
		if msg.Kind == "workspace_add" {
			if msg.Decision != "confirm" {
				m.confirmView = nil
				if m.prevView != "" {
					m.activeView = m.prevView
					m.prevView = ""
				} else {
					m.activeView = "shell"
				}
				if m.activeView == "shell" {
					m.shell.FocusInput()
				}
				return m, nil
			}
			raw := strings.TrimSpace(msg.Text)
			p, err := resolveWorkspaceInputPath(raw)
			if err != nil {
				m.appendSystem(err.Error(), "warning")
				return m, nil
			}
			fi, err := os.Stat(p)
			if err != nil || fi == nil || !fi.IsDir() {
				m.appendSystem("路径不是目录: "+p, "warning")
				return m, nil
			}
			m.adapter.GetCore().AddWorkspaceRoot(p)
			rememberKnownWorkspace(p, false)
			m.refreshWorkspacePanel()
			m.appendSystem("已添加工作区: "+p, "success")
			m.confirmView = nil
			if m.prevView != "" {
				m.activeView = m.prevView
				m.prevView = ""
			} else {
				m.activeView = "shell"
			}
			if m.activeView == "shell" {
				m.shell.FocusInput()
			}
			return m, nil
		}
		if msg.ID != "" {
			m.adapter.GetCore().SubmitPromptResponse(msg.ID, bridge.PromptResponse{
				Decision:    msg.Decision,
				Option:      msg.Option,
				OptionIndex: msg.OptionIndex,
				Text:        msg.Text,
			})
		}
		m.confirmView = nil
		if m.prevView != "" {
			m.activeView = m.prevView
			m.prevView = ""
		} else {
			m.activeView = "shell"
		}
		m.shell.FocusInput()
		return m, nil

	case clearCopiedMsg:
		if msg.idx >= 0 && msg.idx < len(m.history) {
			m.history[msg.idx].copiedAt = time.Time{}
			m.rebuildHistoryContent()
		}

	case setup.SetupCompleteMsg:
		// 设置完成
		m.activeView = "shell"
		m.initialSetupFlow = false
		m.shell.FocusInput()
		m.appendSystem(fmt.Sprintf("Setup complete! Provider: %s, Model: %s", msg.Config.Provider, msg.Config.Model), "info")

	case setup.SetupCancelMsg:
		// 设置取消
		m.activeView = "shell"
		m.initialSetupFlow = false
		m.shell.FocusInput()
		m.appendSystem("Setup cancelled.", "warning")

	case setup.ModelFormCompleteMsg:
		// 模型表单完成
		m.handleModelFormComplete(msg)

	case setup.MCPConfigCancelMsg:
		m.activeView = "panel"
		m.activePanel = "mcp"
		m.shell.ClearInput()

	case setup.MCPConfigSubmitMsg:
		cmds = append(cmds, m.handleMCPConfigSubmit(msg))

	case ShowHintsMsg:
		// 显示提示
		switch msg.Type {
		case "slash":
			m.shell.ShowSlashHints("")
		case "path":
			m.shell.ShowPathHints("")
		}

	case panels.ModelSelectMsg:
		// 选择模型
		m.handleModelSelect(msg)

	case panels.ModelAddMsg:
		// 添加模型 - 打开向导视图
		m.activeView = "setup"
		wizard := setup.NewModelSetupWizard(m.styles, m.state.Language)
		wizard.SetSize(m.width, m.height)
		m.setupView = wizard

	case panels.ModelDeleteMsg:
		// 删除模型
		m.handleModelDelete(msg)

	case panels.ModelSyncMsg:
		// 同步环境变量
		m.handleModelSyncEnv()

	case panels.LanguageChangeMsg:
		// 广播语言切换消息给所有面板
		for name, panel := range m.panels {
			updatedPanel, panelCmd := panel.Update(msg)
			m.panels[name] = updatedPanel
			if panelCmd != nil {
				cmds = append(cmds, panelCmd)
			}
		}
		// 更新 helpView 语言
		if m.helpView != nil {
			m.helpView.SetLanguage(msg.Language)
		}
		// 更新 shell 语言
		if m.shell != nil {
			m.shell.SetLanguage(msg.Language)
		}
		// 更新状态
		m.state.Language = msg.Language

	case panels.WorkspaceSelectMsg:
		cmds = append(cmds, m.handleWorkspaceUse(msg.Path))

	case panels.WorkspaceDeleteMsg:
		m.handleWorkspaceRemove(msg.Path)

	case panels.WorkspaceAddMsg:
		m.openWorkspaceAddConfirm()
		return m, nil

	case WorkspaceReloadDoneMsg:
		if msg.Err != nil {
			m.appendSystem(fmt.Sprintf("工作区切换后重载失败: %v", msg.Err), "error")
		} else {
			m.appendSystem("工作区已切换并完成重载", "success")
		}
		m.refreshWorkspacePanel()
		m.refreshMemoryPanel()
		m.refreshRulesPanel()

	case MCPReloadDoneMsg:
		if msg.Err != nil {
			m.appendSystem(fmt.Sprintf("MCP 重载失败: %v", msg.Err), "error")
		} else {
			m.appendSystem("MCP 已重载", "success")
		}
		m.refreshMCPPanel()
		m.refreshLSPPanel()

	case LSPReloadDoneMsg:
		if msg.Err != nil {
			m.appendSystem(fmt.Sprintf("LSP 重载失败: %v", msg.Err), "error")
		} else {
			m.appendSystem("LSP 已重载", "success")
		}
		m.refreshLSPPanel()

	// MCP 消息处理
	case panels.MCPToggleMsg:
		cmds = append(cmds, m.handleMCPToggle(msg))
	case panels.MCPAddMsg:
		m.handleMCPAdd()
	case panels.MCPAddBrowserMsg:
		m.handleMCPAddBrowser()
	case panels.MCPEditMsg:
		m.handleMCPEdit(msg)
	case panels.MCPDeleteMsg:
		cmds = append(cmds, m.handleMCPDelete(msg))
	case panels.MCPSaveMsg:
		cmds = append(cmds, m.handleMCPSave())

	// Context 消息处理
	case panels.ContextCompactMsg:
		m.adapter.GetCore().CompactContext()
		m.appendSystem(i18n.T("context.compacted", m.state.Language), "success")
		m.refreshContextPanel()
	case panels.ContextClearMsg:
		m.adapter.GetCore().ClearContext()
		m.shell.ClearContent()
		m.history = m.history[:0]
		m.appendSystem(i18n.T("context.cleared", m.state.Language), "success")
		m.refreshContextPanel()
	case panels.ContextExportMsg:
		// TODO: 实现上下文导出
		m.appendSystem("Export context: Not implemented yet", "info")

	case panels.MemoryRefreshMsg:
		m.refreshMemoryPanel()
	case panels.MemoryRebuildIndexMsg:
		m.handleMemoryRebuildIndex()
	case panels.MemorySaveMsg:
		m.handleMemorySave(msg)

	// Cost 消息处理
	case panels.CostClearMsg:
		m.adapter.GetCore().ClearTokenHistory()
		m.refreshCostPanel()
		m.appendSystem(i18n.T("cost.cleared", m.state.Language), "success")
	case panels.CostExportMsg:
		// TODO: 实现成本统计导出
		m.appendSystem("Export cost stats: Not implemented yet", "info")
	case panels.CostRefreshMsg:
		m.refreshCostPanel()

	// Settings 消息处理
	case panels.SettingsSaveMsg:
		m.handleSettingsSave(msg.Settings, msg.GlobalPredictionEnabled)
	case panels.SettingsResetMsg:
		m.appendSystem(i18n.T("settings.reset", m.state.Language), "warning")

	case panels.VersionsLoadMsg:
		m.handleVersionsLoad(msg.FilePath)
	case panels.VersionsRollbackMsg:
		m.handleVersionsRollback(msg.FilePath, msg.Timestamp)
	case panels.VersionsDeleteMsg:
		m.handleVersionsDelete(msg.FilePath, msg.Timestamp)
	case panels.VersionsDeleteFileMsg:
		m.handleVersionsDeleteFile(msg.FilePath)
	case panels.VersionsDeleteAllMsg:
		m.handleVersionsDeleteAll()

	case panels.TasksTickMsg:
		if m.activeView == "panel" && m.activePanel == "tasks" {
			if panel, ok := m.panels["tasks"]; ok {
				updatedPanel, cmd := panel.Update(msg)
				m.panels["tasks"] = updatedPanel
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}

	case panels.TaskToastMsg:
		m.appendSystem(msg.Text, "info")

	case panels.TaskKillRequestMsg:
		id := strings.TrimSpace(msg.ID)
		if id != "" {
			if m.confirmView == nil {
				m.prevView = m.activeView
			}
			req := confirm.Request{
				Kind:     "bg_kill:" + id,
				Title:    i18n.T("tasks.kill.title", m.state.Language),
				Question: fmt.Sprintf(i18n.T("tasks.kill.question", m.state.Language), id),
				Options:  []string{"OK"},
			}
			m.confirmView = confirm.New(m.styles, m.state.Language, req)
			m.confirmView.SetSize(m.width, m.height)
			m.activeView = "confirm"
			m.shell.BlurInput()
			return m, nil
		}

	case panels.LSPRefreshMsg:
		return m, func() tea.Msg { return LSPReloadDoneMsg{Err: m.adapter.Reload()} }
	case panels.RulesRefreshMsg:
		m.refreshRulesPanel()
	case panels.RulesSaveMsg:
		m.handleRulesSave(msg)
	default:
		if m.activeView == "shell" {
			updatedShell, shellCmd := m.shell.Update(msg)
			*m.shell = updatedShell
			m.syncPredictionState()
			if shellCmd != nil {
				cmds = append(cmds, shellCmd)
			}
		}
	}

	m.updateContextUsageUI()
	m.updateBGTaskCountUI()

	// 继续监听事件
	cmds = append(cmds, m.listenEvents())

	return m, tea.Batch(cmds...)
}

// handlePanelMsg 处理面板消息
func (m *AppModel) handlePanelMsg(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return cmd
}

// shouldSendMessage 检查是否应该发送消息，如果是 /exit 命令返回 true 和退出命令
func (m *AppModel) shouldSendMessage() (bool, tea.Cmd) {
	value := m.shell.GetInputValue()
	if value == "" {
		if m.state.Mode == "ai" && !m.state.Processing && len(m.pendingImagePaths) > 0 {
			return true, nil
		}
		return false, nil
	}

	// 如果只是单独的 / 或 @（用于触发提示），不发送
	if value == "/" || value == "@" {
		return false, nil
	}

	// 检查是否是斜杠命令（必须是有效命令）
	if cmd, args, isCmd := slash.ParseCommand(value); isCmd {
		if normalized := slash.NormalizeCommand(cmd); normalized != "" {
			exitCmd := m.handleSlashCommand(normalized, args)
			m.shell.ClearInput()
			if exitCmd != nil {
				return false, exitCmd
			}
			return false, nil
		}
		if len(cmd) > 1 {
			skillName := strings.TrimPrefix(cmd, "/")
			skillName = strings.TrimSpace(skillName)
			if skillName != "" && m.tryInvokeSkillSlash(skillName, args) {
				m.shell.ClearInput()
				return false, nil
			}
		}
	}

	return m.state.Mode == "ai" && !m.state.Processing, nil
}

// handleSlashCommand 处理斜杠命令
func (m *AppModel) handleSlashCommand(cmd string, args []string) tea.Cmd {
	switch cmd {
	case "/help":
		m.clearPrediction()
		m.activeView = "help"
		if m.helpView != nil {
			m.helpView.ResetScroll()
		}
	case "/clear":
		m.shell.ClearContent()
		m.shell.ClearInput()
		m.shell.ClearLive()
		m.history = m.history[:0]
	case "/exit":
		return tea.Quit
	case "/init":
		m.shell.ClearInput()
		return m.initEOSMD()
	case "/history":
		m.clearPrediction()
		m.activeView = "panel"
		m.activePanel = "versions"
		m.shell.ClearInput()
		m.refreshVersionsPanel()
	case "/model":
		return m.handleModelSlash(args)
	case "/mcp":
		m.clearPrediction()
		m.activeView = "panel"
		m.activePanel = "mcp"
		m.shell.ClearInput()
		m.refreshMCPPanel()
	case "/context":
		m.openContextPanel()
	case "/memory":
		m.openMemoryPanel()
	case "/cost":
		m.clearPrediction()
		m.activeView = "panel"
		m.activePanel = "cost"
		m.shell.ClearInput()
		// 刷新成本统计数据
		m.refreshCostPanel()
	case "/tasks":
		m.clearPrediction()
		m.activeView = "panel"
		m.activePanel = "tasks"
		m.shell.ClearInput()
		if panel, ok := m.panels["tasks"].(*panels.TasksPanel); ok && panel != nil {
			panel.ResetView()
			return panel.Init()
		}
	case "/workspace":
		return m.handleWorkspaceSlash(args)
	case "/config":
		m.openSettingsPanel()
	case "/lsp":
		m.clearPrediction()
		m.activeView = "panel"
		m.activePanel = "lsp"
		m.shell.ClearInput()
		m.refreshLSPPanel()
	case "/rules":
		m.clearPrediction()
		m.activeView = "panel"
		m.activePanel = "rules"
		m.shell.ClearInput()
		m.refreshRulesPanel()
	case "/lang":
		if len(args) > 0 {
			m.state.Language = args[0]
			// 保存配置
			if cfg, path := config.Load(); path != "" {
				cfg.Language = args[0]
				if err := config.Save(cfg, path); err != nil {
					m.appendSystem(fmt.Sprintf("Failed to save language config: %v", err), "error")
				}
			}
			m.appendSystem(fmt.Sprintf("Language changed to: %s", args[0]), "success")
			return func() tea.Msg {
				return panels.LanguageChangeMsg{Language: args[0]}
			}
		}
	case "/compact":
		m.adapter.GetCore().CompactContext()
		m.appendSystem(i18n.T("context.compacted", m.state.Language), "success")
	case "/session":
		return m.handleSessionSlash(args)
	case "/resume":
		return m.handleResumeSlash(args)
	case "/permissions":
		return m.handlePermissionsSlash(args)
	case "/skills":
		return m.handleSkillsSlash(args)
	case "/plugin":
		return m.handlePluginSlash()
	case "/reload-plugins":
		return m.handleReloadPluginsSlash()
	case "/doctor":
		return m.handleDoctorSlash()
	case "/diff":
		return m.handleDiffSlash(args)
	case "/review":
		return m.handleReviewSlash(args)
	case "/plan":
		return m.handlePlanSlash(args)
	case "/plan-style":
		return m.handlePlanStyleSlash(args)
	case "/git":
		return m.handleGitSlash(args)
	case "/remote":
		return m.handleRemoteSlash(args)
	case "/status":
		return m.handleStatusSlash()
	case "/fast":
		return m.handleFastSlash()
	case "/export":
		return m.handleExportSlash(args)
	case "/theme":
		return m.handleThemeSlash(args)
	case "/stats":
		return m.handleStatsSlash()
	case "/rename":
		return m.handleRenameSlash(args)
	case "/share":
		return m.handleShareSlash()
	case "/_legal":
		return m.handleHiddenLegalSlash()
	default:
		m.appendSystem(fmt.Sprintf("Unknown command: %s", cmd), "warning")
	}
	return nil
}

func (m *AppModel) initEOSMD() tea.Cmd {
	root := ""
	if m != nil && m.adapter != nil && m.adapter.GetCore() != nil {
		root = strings.TrimSpace(m.adapter.GetCore().GetActiveRoot())
	}
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			m.appendSystem(fmt.Sprintf("初始化失败: %v", err), "error")
			return nil
		}
		root = wd
	}

	dst := filepath.Join(root, "EOS.md")
	existing := ""
	existed := false
	if raw, err := os.ReadFile(dst); err == nil {
		existed = true
		existing = string(raw)
	}

	template := strings.TrimRight(`# EOS.md

This file provides guidance to EOS when working with code in this repository.

## Project Context

- What this project does:
- Target users:
- Key constraints (performance/security/platform):

## How To Work

- When changing behavior, add/adjust tests when possible.
- Prefer minimal, focused diffs over broad refactors.
- Keep user-facing text consistent with UI language (zh/en).

## Build and Development Commands

`+"```bash"+`
go test ./...
go build -o eos
`+"```"+`

## Repository Map

- UI: internal/ui/
- Bridge: internal/bridge/
- Runtime: internal/runtime/
- Tools: internal/tools/

## Coding Style

- Follow existing patterns and naming.
- Avoid introducing new dependencies unless necessary.
- Don’t log secrets/keys.
`, "\n") + "\n"

	mergeEOS := func(old string) string {
		s := strings.TrimSpace(old)
		if s == "" {
			return template
		}
		s = strings.Replace(s, "# CLAUDE.md", "# EOS.md", 1)
		s = strings.Replace(s, "Claude Code (claude.ai/code)", "EOS", 1)
		s = strings.Replace(s, "guidance to Claude Code", "guidance to EOS", 1)
		if !strings.HasPrefix(strings.TrimSpace(s), "# EOS.md") {
			s = "# EOS.md\n\n" + strings.TrimLeft(s, "\n")
		}
		required := []struct {
			heading string
			block   string
		}{
			{"## Project Context", "## Project Context\n\n- What this project does:\n- Target users:\n- Key constraints (performance/security/platform):\n"},
			{"## How To Work", "## How To Work\n\n- When changing behavior, add/adjust tests when possible.\n- Prefer minimal, focused diffs over broad refactors.\n- Keep user-facing text consistent with UI language (zh/en).\n"},
			{"## Build and Development Commands", "## Build and Development Commands\n\n```bash\ngo test ./...\ngo build -o eos\n```\n"},
			{"## Repository Map", "## Repository Map\n\n- UI: internal/ui/\n- Bridge: internal/bridge/\n- Runtime: internal/runtime/\n- Tools: internal/tools/\n"},
			{"## Coding Style", "## Coding Style\n\n- Follow existing patterns and naming.\n- Avoid introducing new dependencies unless necessary.\n- Don’t log secrets/keys.\n"},
		}
		for _, it := range required {
			if strings.Contains(s, "\n"+it.heading+"\n") || strings.HasPrefix(strings.TrimSpace(s), it.heading+"\n") {
				continue
			}
			s = strings.TrimRight(s, "\n") + "\n\n" + strings.TrimRight(it.block, "\n") + "\n"
		}
		return strings.TrimRight(s, "\n") + "\n"
	}

	content := mergeEOS(existing)

	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		m.appendSystem(fmt.Sprintf("EOS.md 写入失败: %v", err), "error")
		return nil
	}
	if cm := m.adapter.GetCore().GetContext(); cm != nil {
		cm.SetPinnedDoc("EOS.md", content, 20000)
	}
	if existed {
		m.appendSystem("已更新 EOS.md", "success")
	} else {
		m.appendSystem("已生成 EOS.md", "success")
	}
	return nil
}

func (m *AppModel) tryInvokeSkillSlash(skillName string, args []string) bool {
	core := m.adapter.GetCore()
	if core == nil {
		return false
	}
	sm := core.GetSkillManager()
	if sm == nil {
		return false
	}
	s, ok := sm.Get(skillName)
	if !ok || s == nil {
		return false
	}
	if s.UserInvocable != nil && !*s.UserInvocable {
		return false
	}
	arguments := strings.TrimSpace(strings.Join(args, " "))
	msgs, _, err := sm.InjectSkillWithArguments(context.Background(), skillName, arguments)
	if err != nil {
		m.appendSystem(fmt.Sprintf("Skill 启用失败: %v", err), "error")
		return true
	}
	if cm := core.GetContext(); cm != nil {
		for _, mp := range msgs {
			if strings.TrimSpace(mp.Content) == "" {
				continue
			}
			cm.AddEphemeral(mp.Content)
		}
	}
	m.appendSystem("Skill 已启用: "+skillName, "success")
	return true
}

type WorkspaceReloadDoneMsg struct {
	Err error
}

type MCPReloadDoneMsg struct {
	Err error
}

type LSPReloadDoneMsg struct {
	Err error
}

func (m *AppModel) refreshWorkspacePanel() {
	panel, ok := m.panels["workspace"].(*panels.WorkspacePanel)
	if !ok {
		return
	}

	roots := m.adapter.GetCore().GetWorkspaceRoots()
	active := m.adapter.GetCore().GetActiveRoot()
	sort.Strings(roots)

	workspaces := make([]panels.Workspace, 0, len(roots))
	for _, p := range roots {
		workspaces = append(workspaces, panels.Workspace{
			Name: filepath.Base(p),
			Path: p,
		})
	}

	panel.SetWorkspaces(workspaces, active)
}

func (m *AppModel) refreshVersionsPanel() {
	panel, ok := m.panels["versions"].(*panels.VersionsPanel)
	if !ok {
		return
	}
	panel.SetLanguage(m.state.Language)
	panel.Reset()

	files, err := m.adapter.GetCore().ListVersionFiles()
	if err != nil {
		m.appendSystem(fmt.Sprintf("Failed to load versions: %v", err), "error")
		panel.SetFiles(nil)
		return
	}
	items := make([]panels.FileItem, 0, len(files))
	for _, f := range files {
		items = append(items, panels.FileItem{
			Path:  f.PathRel,
			Count: f.VersionCount,
			Last:  f.LastModified,
		})
	}
	panel.SetFiles(items)
}

func (m *AppModel) handleWorkspaceRemove(path string) {
	if path == "" {
		return
	}
	m.adapter.GetCore().RemoveWorkspaceRoot(path)
	forgetKnownWorkspace(path)
	m.refreshWorkspacePanel()
	m.appendSystem("已移除工作区: "+path, "success")
}

func (m *AppModel) handleWorkspaceUse(rawPath string) tea.Cmd {
	if rawPath == "" {
		return nil
	}
	path, err := resolveWorkspaceInputPath(rawPath)
	if err != nil {
		m.appendSystem(err.Error(), "error")
		return nil
	}
	fi, err := os.Stat(path)
	if err != nil || fi == nil || !fi.IsDir() {
		m.appendSystem("路径不是目录: "+path, "warning")
		return nil
	}
	if !m.isWorkspaceTrusted(path) {
		m.trustPendingPath = path
		m.trustPendingAction = "switch"
		m.openWorkspaceTrustConfirm(path)
		return nil
	}
	return m.switchWorkspaceTrusted(path)
}

func (m *AppModel) switchWorkspaceTrusted(path string) tea.Cmd {
	e := m.adapter.GetCore().SetActiveWorkspaceRoot(path)
	if e == nil {
		m.appendSystem("工作区不存在: "+path, "warning")
		return nil
	}
	rememberKnownWorkspace(path, true)
	_ = os.Chdir(path)
	settingsPath := filepath.Join(path, ".eos", "settings.json")
	m.adapter.GetSettings().SetPath(settingsPath)
	_, _ = m.adapter.GetCore().LoadSettings(settingsPath)
	m.refreshWorkspacePanel()
	m.appendSystem("已切换工作区: "+path, "success")
	return func() tea.Msg {
		return WorkspaceReloadDoneMsg{Err: m.adapter.Reload()}
	}
}

func (m *AppModel) openWorkspaceAddConfirm() {
	req := confirm.Request{
		Kind:      "workspace_add",
		Title:     i18n.T("workspace.add.title", m.state.Language),
		Question:  i18n.T("workspace.add.question", m.state.Language),
		Options:   []string{i18n.T("workspace.add.confirm", m.state.Language)},
		AllowText: true,
		TextHint:  i18n.T("workspace.add.hint", m.state.Language),
	}
	m.openConfirm(req)
}

func (m *AppModel) openWorkspaceTrustConfirm(path string) {
	req := confirm.Request{
		Kind:     "workspace_trust",
		Title:    i18n.T("workspace.trust.title", m.state.Language),
		Question: fmt.Sprintf(i18n.T("workspace.trust.question", m.state.Language), path),
		Options: []string{
			i18n.T("workspace.trust.confirm", m.state.Language),
			i18n.T("workspace.trust.exit", m.state.Language),
		},
	}
	m.openConfirm(req)
}

func (m *AppModel) openConfirm(req confirm.Request) {
	if m.confirmView == nil {
		m.prevView = m.activeView
	}
	m.clearPrediction()
	m.confirmView = confirm.New(m.styles, m.state.Language, req)
	m.confirmView.SetSize(m.width, m.height)
	m.activeView = "confirm"
	m.shell.BlurInput()
}

func (m *AppModel) isWorkspaceTrusted(path string) bool {
	cfg, _ := config.Load()
	want := normalizeWorkspacePath(path)
	for _, p := range cfg.TrustedWorkspaces {
		if pathsEqual(normalizeWorkspacePath(p), want) {
			return true
		}
	}
	return false
}

func (m *AppModel) addTrustedWorkspace(path string) {
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return
	}
	want := normalizeWorkspacePath(path)
	for _, p := range cfg.TrustedWorkspaces {
		if pathsEqual(normalizeWorkspacePath(p), want) {
			return
		}
	}
	cfg.TrustedWorkspaces = append(cfg.TrustedWorkspaces, want)
	_ = config.Save(cfg, cfgPath)
}

func resolveWorkspaceInputPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("请输入工作区路径")
	}
	if raw == "~" || strings.HasPrefix(raw, "~"+string(os.PathSeparator)) || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return "", fmt.Errorf("无法解析 ~")
		}
		rest := strings.TrimPrefix(raw, "~")
		rest = strings.TrimPrefix(rest, "/")
		rest = strings.TrimPrefix(rest, "\\")
		raw = filepath.Join(home, rest)
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("路径解析失败: %v", err)
	}
	return normalizeWorkspacePath(abs), nil
}

func normalizeWorkspacePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	p = filepath.Clean(filepath.FromSlash(p))
	if vol := filepath.VolumeName(p); vol != "" {
		rest := strings.TrimPrefix(p, vol)
		p = strings.ToUpper(vol) + rest
	}
	return p
}

func pathsEqual(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(filepath.VolumeName(a), filepath.VolumeName(b)) {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// sendMessage 发送消息
func (m *AppModel) sendMessage() tea.Cmd {
	value := m.shell.GetInputValue()
	expanded := value
	if strings.Contains(strings.ToLower(expanded), "#problems_and_diagnostics") {
		md := ""
		if m.adapter != nil && m.adapter.GetCore() != nil {
			md = m.adapter.GetCore().ProblemsAndDiagnosticsMarkdown()
		}
		if strings.TrimSpace(md) != "" {
			re := regexp.MustCompile(`(?i)#problems_and_diagnostics`)
			expanded = re.ReplaceAllStringFunc(expanded, func(string) string { return md })
		}
	}
	m.shell.AddToHistory(value)
	m.clearPrediction()
	m.shell.ClearInput()
	m.state.Processing = true
	m.shell.SetProcessing(true)
	m.delegatedThisRound = false
	m.aiLive.Reset()
	m.thinkingLive.Reset()
	m.state.Thinking = false
	m.shell.SetThinking(false, "")
	m.shell.ClearLive()

	// 记录AI开始时间
	m.currentAIStartTime = time.Now()
	m.currentAITokens = 0
	m.setActiveCancel(func() {
		m.adapter.CancelForegroundRequest()
	})

	// 使用新的消息渲染器显示用户消息
	imagePaths := m.pendingImagePaths
	m.pendingImagePaths = nil
	if len(imagePaths) > 0 {
		var names []string
		for _, p := range imagePaths {
			b := strings.TrimSpace(filepath.Base(p))
			if b != "" {
				names = append(names, b)
			}
			if len(names) >= 4 {
				break
			}
		}
		if len(names) > 0 {
			m.appendSystem("已附带图片: "+strings.Join(names, ", "), "info")
		} else {
			m.appendSystem("已附带图片", "info")
		}
	}

	display := value
	if strings.TrimSpace(display) == "" && len(imagePaths) > 0 {
		display = i18n.T("chat.image_only", m.state.Language)
	}
	m.appendHistory(historyEntry{kind: "user", content: display, timestamp: time.Now()})

	// 异步调用Runtime
	invoke := func() tea.Msg {
		ctx := context.Background()
		content, err := m.adapter.Invoke(ctx, expanded, m.state.ExecutionMode, imagePaths)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "context canceled") {
				return nil
			}
			return ErrorMsg{Err: err}
		}
		return InvokeDoneMsg{Content: content}
	}
	return tea.Batch(invoke, m.shell.StatusTick())
}

func (m *AppModel) sendBashCommand() tea.Cmd {
	value := strings.TrimSpace(m.shell.GetInputValue())
	m.shell.AddToHistory(value)
	m.clearPrediction()
	m.shell.ClearInput()
	m.state.Processing = true
	m.shell.SetProcessing(true)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	m.setActiveCancel(cancel)

	id := fmt.Sprintf("bash:%d", time.Now().UnixNano())
	m.handleToolCall(ToolCallMsg{ID: id, Name: "bash", Params: map[string]any{"command": value}})

	return func() tea.Msg {
		defer cancel()
		out, err := m.adapter.GetCore().ExecuteBash(ctx, value)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "context canceled") {
				return ToolResultMsg{ID: id, Status: "canceled"}
			}
			msg := strings.ReplaceAll(err.Error(), "\r\n", "\n")
			msg = strings.ReplaceAll(msg, "\r", "")
			return ToolResultMsg{ID: id, Status: "error", Output: msg}
		}
		out = strings.ReplaceAll(out, "\r\n", "\n")
		out = strings.ReplaceAll(out, "\r", "")
		return ToolResultMsg{ID: id, Status: "success", Output: strings.TrimRight(out, "\n")}
	}
}

func (m *AppModel) refreshContextPanel() {
	if m == nil || m.adapter == nil {
		return
	}
	panel, ok := m.panels["context"].(*panels.ContextPanel)
	if !ok || panel == nil {
		return
	}
	cm := m.adapter.GetContext()
	if cm == nil {
		return
	}
	st := cm.ExportState()

	pinnedTokens := 0
	if len(st.Pinned) > 0 {
		pinnedTokens = cm.EstimateMessageTokens(st.Pinned)
	}
	_, convTokens, _ := cm.GetConversationUsage()

	model := strings.TrimSpace(st.ModelName)
	if model == "" {
		_, _, mdl, _ := m.adapter.ResolveAPIConfig()
		model = strings.TrimSpace(mdl)
	}
	panel.SetStats(model, st.MaxPromptTokens, pinnedTokens, convTokens)

	msgs := make([]panels.ContextMessage, 0, len(st.Pinned)+len(st.Ephem)+len(st.Recent)+len(st.CurrentFull)+len(st.ToolObs)+len(st.Tools))

	addMsg := func(role string, msg ai.Message) {
		toks := cm.EstimateMessageTokens([]ai.Message{msg})
		msgs = append(msgs, panels.ContextMessage{Role: role, Content: msg.Content, Tokens: toks})
	}
	for _, msg := range st.Pinned {
		addMsg("pinned", msg)
	}
	for _, e := range st.Ephem {
		s := strings.TrimSpace(e)
		if s == "" {
			continue
		}
		addMsg("ephem", ai.Message{Role: "system", Content: s})
	}
	for _, msg := range st.Recent {
		addMsg(msg.Role, msg)
	}
	for _, msg := range st.CurrentFull {
		addMsg("full", msg)
	}
	for _, t := range st.ToolObs {
		s := strings.TrimSpace(t)
		if s == "" {
			continue
		}
		addMsg("toolObs", ai.Message{Role: "system", Content: s})
	}
	for _, t := range st.Tools {
		s := strings.TrimSpace(t)
		if s == "" {
			continue
		}
		addMsg("tool", ai.Message{Role: "system", Content: s})
	}

	panel.SetMessages(msgs)
}

func (m *AppModel) refreshMemoryPanel() {
	if m == nil || m.adapter == nil {
		return
	}
	panel, ok := m.panels["memory"].(*panels.MemoryPanel)
	if !ok || panel == nil {
		return
	}
	root := strings.TrimSpace(m.adapter.GetCore().GetActiveRoot())
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	if strings.TrimSpace(root) != "" {
		_ = memory.EnsureWorkspaceMemory(root)
	}
	snap := memory.LoadSnapshot(root)
	panel.SetData(root, snap.GlobalPath, snap.GlobalContent, snap.GlobalExists, snap.ProjectPath, snap.ProjectContent, snap.ProjectExists, snap.SessionPath, snap.SessionContent, snap.SessionExists, snap.IndexPath, snap.IndexContent, snap.IndexExists)
}

// handleBridgeEvent 处理 bridge.Event 消息
func (m *AppModel) handleBridgeEvent(e bridge.Event) (tea.Model, tea.Cmd) {
	uiMsg := ConvertEvent(e)
	if uiMsg != nil {
		return m.Update(uiMsg)
	}
	return m, m.listenEvents()
}

// View 渲染应用视图
func (m *AppModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.activeView {
	case "shell":
		return m.shell.View()
	case "confirm":
		if m.confirmView != nil {
			return m.confirmView.View()
		}
		return m.shell.View()
	case "help":
		return m.helpView.View()
	case "setup":
		switch sv := m.setupView.(type) {
		case *setup.SetupView:
			return sv.View()
		case *setup.ModelSetupView:
			return sv.View()
		case *setup.MCPConfigEditorView:
			return sv.View()
		}
		return "Loading..."
	case "panel":
		if panel, ok := m.panels[m.activePanel]; ok {
			return panel.View()
		}
		return m.styles.App.Render("Panel not found: " + m.activePanel)
	default:
		return m.styles.App.Render("Welcome to EOS!")
	}
}

// listenEvents 监听运行时事件
func (m *AppModel) listenEvents() tea.Cmd {
	return func() tea.Msg {
		event := <-m.adapter.Events()
		return event
	}
}

func (m *AppModel) cancelProcessingUI() {
	m.state.Processing = false
	m.shell.SetProcessing(false)
	m.shell.ClearLive()
	m.aiLive.Reset()
	m.thinkingLive.Reset()
	m.state.Thinking = false
	m.shell.SetThinking(false, "")
	m.toolInflight = make(map[string]toolTrack)
	m.activeCancel = nil
	m.stopRequested = false
}

func (m *AppModel) setActiveCancel(cancel context.CancelFunc) {
	m.activeCancel = cancel
	m.stopRequested = false
}

func (m *AppModel) cancelActiveRequest() bool {
	if m == nil || m.activeCancel == nil || m.stopRequested {
		return false
	}
	cancel := m.activeCancel
	m.activeCancel = nil
	m.stopRequested = true
	cancel()
	return true
}

func (m *AppModel) markInflightToolsCanceled(output string) {
	if len(m.toolInflight) == 0 {
		return
	}
	for _, track := range m.toolInflight {
		if track.idx < 0 || track.idx >= len(m.history) {
			continue
		}
		e := m.history[track.idx]
		e.toolStatus = "canceled"
		e.toolSuccess = false
		if strings.TrimSpace(e.toolOutput) == "" {
			e.toolOutput = output
		}
		e.duration = time.Since(track.started)
		m.history[track.idx] = e
	}
	m.rebuildHistoryContent()
}

func (m *AppModel) renderHistoryEntry(e historyEntry) string {
	if m.msgRenderer == nil {
		switch e.kind {
		case "user", "ai", "system":
			return e.content
		case "tool":
			return e.toolOutput
		case "agent.final":
			return e.content
		default:
			return e.content
		}
	}

	switch e.kind {
	case "user":
		return m.msgRenderer.RenderUserInputAt(e.content, e.timestamp)
	case "ai":
		return m.msgRenderer.RenderAIResponseAtWithCopy(e.content, e.tokens, e.duration, true, e.timestamp, m.copyButtonLabel(e))
	case "agent.task":
		return m.msgRenderer.RenderAgentTaskAt(e.agentName, e.task, e.timestamp)
	case "tool":
		status := e.toolStatus
		if status == "" {
			if e.toolSuccess {
				status = "success"
			} else if e.toolOutput != "" {
				status = "error"
			} else {
				status = "running"
			}
		}
		return m.msgRenderer.RenderToolEvent(e.toolName, e.toolParams, status, e.toolOutput, e.duration)
	case "agent.final":
		return m.msgRenderer.RenderAgentFinalAtWithCopy(e.agentName, e.content, e.timestamp, m.copyButtonLabel(e))
	case "system":
		return m.msgRenderer.RenderSystem(e.content, e.level)
	default:
		return e.content
	}
}

func (m *AppModel) copyButtonLabel(e historyEntry) string {
	if !e.copiedAt.IsZero() && time.Since(e.copiedAt) <= 1500*time.Millisecond {
		return i18n.T("op.copied", m.state.Language)
	}
	return i18n.T("op.copy", m.state.Language)
}

func (m *AppModel) appendHistory(e historyEntry) {
	m.history = append(m.history, e)
	rendered := m.renderHistoryEntry(e)
	m.trackCopyHitAt(m.shell.ContentLineCount(), len(m.history)-1, e, rendered)
	block := "\n" + rendered + "\n\n"
	m.shell.AppendContent(block)
}

func (m *AppModel) appendHistoryIndex(e historyEntry) int {
	idx := len(m.history)
	m.appendHistory(e)
	return idx
}

func (m *AppModel) rebuildHistoryContent() {
	if len(m.history) == 0 {
		return
	}
	m.copyHits = nil
	var sb strings.Builder
	lineCount := 1
	for idx, e := range m.history {
		rendered := m.renderHistoryEntry(e)
		m.trackCopyHitAt(lineCount, idx, e, rendered)
		sb.WriteString("\n")
		sb.WriteString(rendered)
		sb.WriteString("\n\n")
		lineCount += strings.Count(rendered, "\n") + 3
	}
	m.shell.SetContentPreserveOffset(sb.String())
}

func stripANSI(s string) string {
	if strings.IndexByte(s, 0x1b) < 0 {
		return s
	}
	return ansiRe.ReplaceAllString(s, "")
}

func runeIndex(s string, byteIdx int) int {
	if byteIdx <= 0 {
		return 0
	}
	if byteIdx >= len(s) {
		return len([]rune(s))
	}
	return len([]rune(s[:byteIdx]))
}

func (m *AppModel) trackCopyHitAt(startLine int, idx int, e historyEntry, rendered string) {
	if m.msgRenderer == nil {
		return
	}
	if e.kind != "ai" && e.kind != "agent.final" {
		return
	}
	payload := strings.TrimSpace(e.content)
	if payload == "" {
		return
	}
	lines := strings.Split(stripANSI(rendered), "\n")
	marks := m.copyMarks()
	for i, line := range lines {
		found := false
		bi := -1
		markLen := 0
		for _, mark := range marks {
			bi = strings.LastIndex(line, " "+mark+" ")
			markLen = len([]rune(" " + mark + " "))
			if bi >= 0 {
				found = true
				break
			}
			bi = strings.LastIndex(line, mark)
			markLen = len([]rune(mark))
			if bi >= 0 {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		x0 := runeIndex(line, bi)
		x1 := x0 + markLen - 1
		m.copyHits = append(m.copyHits, copyHit{y: startLine + i, x0: x0, x1: x1, idx: idx, text: payload})
		return
	}
}

func (m *AppModel) tryCopyBubbleAt(x, y int) tea.Cmd {
	ox, oy := m.shell.ContentOrigin()
	if x < ox || y < oy {
		return nil
	}
	lx := x - ox
	ly := y - oy
	if ly < 0 || ly >= m.shell.ContentHeight() {
		return nil
	}
	line := m.shell.ContentYOffset() + ly
	for _, h := range m.copyHits {
		if h.y != line {
			continue
		}
		if lx < h.x0 || lx > h.x1 {
			continue
		}
		if err := clipboard.WriteAll(h.text); err != nil {
			m.appendSystem(i18n.T("tool.error.copy_error", m.state.Language, err), "error")
			return func() tea.Msg { return nil }
		}
		if h.idx >= 0 && h.idx < len(m.history) {
			m.history[h.idx].copiedAt = time.Now()
		}
		m.rebuildHistoryContent()
		m.appendSystem(i18n.T("clipboard.copied", m.state.Language), "success")
		return tea.Tick(1600*time.Millisecond, func(time.Time) tea.Msg { return clearCopiedMsg{idx: h.idx} })
	}
	return nil
}

func (m *AppModel) appendSystem(text, level string) {
	m.appendHistory(historyEntry{kind: "system", content: text, level: level})
}

func (m *AppModel) pasteClipboardImage() tea.Cmd {
	if m.activeView != "shell" {
		return func() tea.Msg { return nil }
	}
	if m.state.Mode != "ai" {
		m.appendSystem("Bash 模式下无法发送图片，请切换到 AI 模式", "warning")
		return func() tea.Msg { return nil }
	}
	b, err := clip.ReadImage()
	if err != nil {
		if strings.Contains(err.Error(), "empty clipboard image") {
			m.appendSystem("剪贴板里没有图片", "warning")
		} else {
			m.appendSystem("粘贴图片失败: "+err.Error(), "error")
		}
		return func() tea.Msg { return nil }
	}
	wd, err := os.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		m.appendSystem("粘贴图片失败: 无法获取工作目录", "error")
		return func() tea.Msg { return nil }
	}
	dir := filepath.Join(wd, ".eos", "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.appendSystem("粘贴图片失败: "+err.Error(), "error")
		return func() tea.Msg { return nil }
	}
	path := filepath.Join(dir, fmt.Sprintf("clipboard-%d.png", time.Now().UnixNano()))
	if err := os.WriteFile(path, b, 0o600); err != nil {
		m.appendSystem("粘贴图片失败: "+err.Error(), "error")
		return func() tea.Msg { return nil }
	}
	m.pendingImagePaths = append(m.pendingImagePaths, path)
	m.appendSystem("已添加图片: "+filepath.Base(path), "success")
	modelName, _ := m.adapter.GetModelInfo()
	if strings.TrimSpace(modelName) == "" {
		_, _, mdl, _ := m.adapter.ResolveAPIConfig()
		modelName = mdl
	}
	if strings.TrimSpace(modelName) != "" && !ai.SupportsVisionFromCatalog(modelName) {
		m.appendSystem("当前模型可能不具备视觉能力，图片可能无法解析", "warning")
	}
	return func() tea.Msg { return nil }
}

func (m *AppModel) toggleThinkingExpand() tea.Cmd {
	m.thinkingExpanded = !m.thinkingExpanded
	if m.shell != nil {
		m.shell.SetThinkingExpanded(m.thinkingExpanded)
	}
	m.refreshAILive()
	if m.thinkingExpanded {
		m.appendSystem("思考已展开", "info")
	} else {
		m.appendSystem("思考已折叠", "info")
	}
	return func() tea.Msg { return nil }
}

// handleGlobalKey 处理全局键盘输入
func (m *AppModel) handleGlobalKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		if m.state.Processing {
			m.cancelProcessingUI()
			return nil
		}
		return tea.Quit

	case "f2":
		// 切换模式
		m.shell.ToggleMode()
		if m.shell.GetMode() == shell.ModeAI {
			m.state.Mode = "ai"
		} else {
			m.state.Mode = "bash"
		}
		return nil

	case "?":
		// 打开帮助面板
		m.clearPrediction()
		m.activeView = "help"
		if m.helpView != nil {
			m.helpView.ResetScroll()
			m.helpView.SetSize(m.width, m.height)
		}
		return nil

	case "esc":
		if m.state.Processing {
			if m.cancelActiveRequest() {
				m.markInflightToolsCanceled(i18n.T("toast.stopped", m.state.Language))
				m.cancelProcessingUI()
				m.appendSystem(i18n.T("toast.stopped", m.state.Language), "warning")
				return func() tea.Msg { return nil }
			}
		}
		m.shell.HideHints()
		m.clearPrediction()
		m.shell.ClearInput()
		return func() tea.Msg { return nil } // 返回非nil阻止进入else分支

	case "tab":
		if m.activeView == "shell" && m.shell.GetMode() == shell.ModeAI && !m.shell.IsHintsVisible() {
			if m.shell.CanAcceptPrediction() {
				m.shell.HandleKey(msg)
				m.syncPredictionState()
				return func() tea.Msg { return nil }
			}
			state.SetThinking(!state.Thinking())
			m.refreshAILive()
			return func() tea.Msg { return nil }
		}
	case "ctrl+o":
		if m.activeView == "shell" && m.shell != nil && m.shell.GetMode() == shell.ModeAI {
			m.shell.CycleLivePanelMode()
			return func() tea.Msg { return nil }
		}
	case "alt+v":
		return m.pasteClipboardImage()
	case "alt+h":
		if m.activeView == "shell" && m.shell != nil && m.shell.GetMode() == shell.ModeAI && state.Thinking() {
			return m.toggleThinkingExpand()
		}
	}
	return nil
}

// ShowHintsMsg 显示提示消息
type ShowHintsMsg struct {
	Type string // "help" 或 "slash"
}

// updateHintsBasedOnInput 根据输入内容更新 hints
func (m *AppModel) updateHintsBasedOnInput() {
	text := m.shell.GetInputValue()

	// 如果以 ? 开头，显示帮助提示（但现在 ? 是打开帮助面板，所以这里不需要）
	// 实际处理在 handleGlobalKey 中

	// 如果以 / 开头，显示斜杠命令提示
	if cmdLine, ok := strings.CutPrefix(text, "/"); ok {
		// 检查是否有空格（有参数时不显示提示）
		if !strings.Contains(cmdLine, " ") {
			m.shell.ShowSlashHints(cmdLine)
		} else {
			m.shell.HideHints()
		}
		return
	}

	// 如果包含 @，显示路径提示
	if strings.Contains(text, "@") {
		// 检查 @ 后面是否有空格
		i := strings.LastIndex(text, "@")
		if i >= 0 {
			// @ 是最后一个字符，或者 @ 后面没有空格
			if i+1 >= len(text) {
				// @ 是最后一个字符，显示所有路径
				m.shell.ShowPathHints("")
				return
			}
			q := text[i+1:]
			if !strings.ContainsAny(q, " \n\t") {
				m.shell.ShowPathHints(q)
				return
			}
		}
	}

	// 其他情况隐藏 hints
	m.shell.HideHints()
}

// handleAIResponse 处理 AI 响应
func (m *AppModel) handleAIResponse(msg AIResponseMsg) tea.Cmd {
	switch msg.Type {
	case "delta":
		m.clearPrediction()
		m.aiLive.WriteString(msg.Content)
		m.currentAITokens += len(msg.Content) / 4
		m.refreshAILive()
	case "final":
		duration := time.Since(m.currentAIStartTime)
		m.shell.ClearLive()
		m.aiLive.Reset()
		m.thinkingLive.Reset()
		m.state.Thinking = false
		m.shell.SetThinking(false, "")
		mainContent := strings.TrimSpace(msg.Content)
		agentContent := strings.TrimSpace(m.lastAgentFinal)
		if !(m.delegatedThisRound && mainContent != "" && agentContent != "" && mainContent == agentContent) {
			m.appendHistory(historyEntry{kind: "ai", content: msg.Content, timestamp: time.Now(), tokens: m.currentAITokens, duration: duration})
		}
		m.state.Processing = false
		m.shell.SetProcessing(false)
		return m.schedulePrediction(m.shell.GetInputValue())
	case "error":
		m.clearPrediction()
		m.shell.ClearLive()
		m.aiLive.Reset()
		m.thinkingLive.Reset()
		m.state.Thinking = false
		m.shell.SetThinking(false, "")
		m.appendHistory(historyEntry{kind: "system", content: msg.Content, level: "error"})
		m.state.Processing = false
		m.shell.SetProcessing(false)
	}
	return nil
}

// handleToolCall 处理工具调用
func (m *AppModel) handleToolCall(msg ToolCallMsg) tea.Cmd {
	entry := historyEntry{
		kind:       "tool",
		toolID:     msg.ID,
		toolName:   msg.Name,
		toolParams: msg.Params,
		toolStatus: "running",
		timestamp:  time.Now(),
	}
	idx := m.appendHistoryIndex(entry)
	m.toolInflight[msg.ID] = toolTrack{name: msg.Name, started: time.Now(), idx: idx, params: msg.Params}
	if m.msgRenderer != nil {
		m.shell.SetLive(m.msgRenderer.RenderToolCall(msg.Name, msg.Params))
	} else {
		m.shell.SetLive(fmt.Sprintf("[Tool Call] %s", msg.Name))
	}
	return nil
}

// handleToolResult 处理工具结果
func (m *AppModel) handleToolResult(msg ToolResultMsg) tea.Cmd {
	success := msg.Status == "success"
	track, ok := m.toolInflight[msg.ID]
	if ok {
		delete(m.toolInflight, msg.ID)
	}
	m.shell.ClearLive()
	name := msg.ID
	var duration time.Duration
	if ok {
		name = track.name
		duration = time.Since(track.started)
	}
	if ok && track.idx >= 0 && track.idx < len(m.history) {
		e := m.history[track.idx]
		e.kind = "tool"
		e.toolID = msg.ID
		e.toolName = name
		if e.toolParams == nil {
			e.toolParams = track.params
		}
		e.toolOutput = msg.Output
		e.toolSuccess = success
		e.toolStatus = msg.Status
		e.duration = duration
		m.history[track.idx] = e
		m.rebuildHistoryContent()
		if strings.EqualFold(name, "bash") {
			m.state.Processing = false
			m.shell.SetProcessing(false)
			m.activeCancel = nil
			m.stopRequested = false
		}
		return nil
	}
	if msg.Status == "canceled" {
		if strings.EqualFold(name, "bash") {
			m.activeCancel = nil
			m.stopRequested = false
		}
		return nil
	}
	m.appendHistory(historyEntry{kind: "tool", toolID: msg.ID, toolName: name, toolOutput: msg.Output, toolSuccess: success, toolStatus: msg.Status, duration: duration, timestamp: time.Now()})
	if strings.EqualFold(name, "bash") {
		m.state.Processing = false
		m.shell.SetProcessing(false)
		m.activeCancel = nil
		m.stopRequested = false
	}
	return nil
}

// handleModelSelect 处理模型选择
func (m *AppModel) handleModelSelect(msg panels.ModelSelectMsg) {
	if m.adapter.SetActiveModel(msg.Name) {
		// 重新加载运行时环境
		_ = m.adapter.Reload()
		// 更新面板中的当前模型
		if modelsPanel, ok := m.panels["models"].(*panels.ModelsPanel); ok {
			modelsPanel.SetCurrentModel(msg.Name)
		}
		m.appendSystem(fmt.Sprintf("Switched model: %s", msg.Name), "success")
	} else {
		m.appendSystem(fmt.Sprintf("Failed to switch model: %s", msg.Name), "error")
	}
}

// handleModelDelete 处理模型删除
func (m *AppModel) handleModelDelete(msg panels.ModelDeleteMsg) {
	if m.adapter.GetCore().DeleteModel(msg.Name) {
		// 刷新模型列表面板
		if modelsPanel, ok := m.panels["models"].(*panels.ModelsPanel); ok {
			modelsPanel.Refresh()
		}
		m.appendSystem(fmt.Sprintf("Deleted model: %s", msg.Name), "success")
	} else {
		m.appendSystem(fmt.Sprintf("Failed to delete model: %s (may be env model or active model)", msg.Name), "error")
	}
}

// handleModelSyncEnv 处理环境变量同步
func (m *AppModel) handleModelSyncEnv() {
	if m.adapter.GetCore().SyncEnvModel() {
		// 重新加载运行时环境
		_ = m.adapter.Reload()
		// 刷新模型列表面板
		if modelsPanel, ok := m.panels["models"].(*panels.ModelsPanel); ok {
			modelsPanel.Refresh()
			modelName, _ := m.adapter.GetModelInfo()
			modelsPanel.SetCurrentModel(modelName)
		}
		m.appendSystem("Synced model from environment variables", "success")
	} else {
		m.appendSystem("Failed to sync model from environment (EOS_API_BASE and EOS_API_KEY required)", "error")
	}
}

// handleModelFormComplete 处理模型表单完成
func (m *AppModel) handleModelFormComplete(msg setup.ModelFormCompleteMsg) {
	m.activeView = "shell"
	m.shell.FocusInput()
	suppressSuccessMessage := m.initialSetupFlow && len(m.history) == 0 && !msg.EditMode

	// 使用配置中的名称
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
		if m.adapter.GetCore().UpdateModel(entry) {
			m.appendSystem(fmt.Sprintf("Updated model: %s", name), "success")
		} else {
			m.appendSystem(fmt.Sprintf("Failed to update model: %s", name), "error")
		}
	} else {
		// 添加模式：添加新模型并设置为活动
		if m.adapter.GetCore().AddModel(entry) {
			m.adapter.SetActiveModel(name)
			// 重新加载运行时环境
			_ = m.adapter.Reload()
			m.refreshShellWelcomeInfo()
			if !suppressSuccessMessage {
				m.appendSystem(fmt.Sprintf("Added and switched to model: %s", name), "success")
			}
		} else {
			m.appendSystem(fmt.Sprintf("Failed to add model: %s", name), "error")
		}
	}
	m.initialSetupFlow = false

	// 刷新模型列表面板
	if modelsPanel, ok := m.panels["models"].(*panels.ModelsPanel); ok {
		modelsPanel.Refresh()
		modelsPanel.SetCurrentModel(name)
	}
}

// handleMCPToggle 处理 MCP 服务器切换
func (m *AppModel) handleMCPToggle(msg panels.MCPToggleMsg) tea.Cmd {
	if m.adapter.GetCore().ToggleMCPServer(msg.Name) {
		var status string
		configServers := m.adapter.GetCore().ListMCPServers()
		for _, s := range configServers {
			if s.Name == msg.Name {
				if s.Enabled {
					status = i18n.T("mcp.status.enabled", m.state.Language)
				} else {
					status = i18n.T("mcp.status.disabled", m.state.Language)
				}
				break
			}
		}
		m.refreshMCPPanel()
		if status == "" {
			status = i18n.T("mcp.status.disabled", m.state.Language)
		}
		m.appendSystem(fmt.Sprintf(i18n.T("mcp.msg.toggled", m.state.Language), status, msg.Name), "success")
		return func() tea.Msg {
			return MCPReloadDoneMsg{Err: m.adapter.Reload()}
		}
	} else {
		m.appendSystem(fmt.Sprintf("Failed to toggle MCP server: %s", msg.Name), "error")
	}
	return nil
}

// handleMCPAdd 处理添加 MCP 服务器
func (m *AppModel) handleMCPAdd() {
	initial := `[
  {
    "name": "my-mcp",
    "type": "stdio",
    "command": "",
    "args": [],
    "envs": {},
    "enabled": true
  }
]`
	m.activeView = "setup"
	editor := setup.NewMCPConfigEditorView(m.styles, m.state.Language, initial, false, "")
	editor.SetSize(m.width, m.height)
	m.setupView = editor
}

func (m *AppModel) handleMCPAddBrowser() {
	m.activeView = "setup"
	editor := setup.NewMCPConfigEditorView(m.styles, m.state.Language, mcppkg.RecommendedBrowserPresetJSON(), false, "")
	editor.SetSize(m.width, m.height)
	m.setupView = editor
}

// handleMCPEdit 处理编辑 MCP 服务器
func (m *AppModel) handleMCPEdit(msg panels.MCPEditMsg) {
	var entry *config.MCPEntry
	for _, e := range m.adapter.GetCore().ListMCPServers() {
		if e.Name == msg.Name {
			e2 := e
			entry = &e2
			break
		}
	}
	if entry == nil {
		m.appendSystem("未找到 MCP 服务器: "+msg.Name, "warning")
		return
	}
	b, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		m.appendSystem(fmt.Sprintf("序列化 MCP 配置失败: %v", err), "error")
		return
	}
	m.activeView = "setup"
	editor := setup.NewMCPConfigEditorView(m.styles, m.state.Language, string(b), true, entry.Name)
	editor.SetSize(m.width, m.height)
	m.setupView = editor
}

// handleMCPDelete 处理删除 MCP 服务器
func (m *AppModel) handleMCPDelete(msg panels.MCPDeleteMsg) tea.Cmd {
	if m.adapter.GetCore().DeleteMCPServer(msg.Name) {
		m.refreshMCPPanel()
		m.appendSystem(fmt.Sprintf(i18n.T("mcp.msg.deleted", m.state.Language), msg.Name), "success")
		return func() tea.Msg {
			return MCPReloadDoneMsg{Err: m.adapter.Reload()}
		}
	} else {
		m.appendSystem(fmt.Sprintf("Failed to delete MCP server: %s", msg.Name), "error")
	}
	return nil
}

// handleMCPSave 处理保存 MCP 配置
func (m *AppModel) handleMCPSave() tea.Cmd {
	m.refreshMCPPanel()
	return func() tea.Msg {
		return MCPReloadDoneMsg{Err: m.adapter.Reload()}
	}
}

func (m *AppModel) refreshMCPPanel() {
	mcpPanel, ok := m.panels["mcp"].(*panels.MCPPanel)
	if !ok {
		return
	}
	cfgServers := m.adapter.GetCore().ListMCPServers()
	out := make([]panels.MCPServer, 0, len(cfgServers))
	for _, s := range cfgServers {
		out = append(out, panels.MCPServer{
			Name:    s.Name,
			Type:    string(s.Type),
			Enabled: s.Enabled,
		})
	}
	mcpPanel.SetServers(out)
	browser := m.adapter.GetCore().BrowserStatus()
	mcpPanel.SetBrowserSummary(panels.BrowserSummary{
		Configured: browser.Configured,
		Enabled:    browser.Enabled,
		Loaded:     browser.Loaded,
		ServerName: browser.ServerName,
		Hint:       browser.InstallHint,
	})
}

func (m *AppModel) refreshLSPPanel() {
	p, ok := m.panels["lsp"].(*panels.LSPPanel)
	if !ok || p == nil {
		return
	}
	if m.adapter == nil || m.adapter.GetCore() == nil {
		p.SetStatus(panels.LSPPanelSummary{Message: "no core"}, nil)
		return
	}
	st := m.adapter.GetCore().LSPStatus()
	sum := panels.LSPPanelSummary{
		Enabled:          st.Enabled,
		AutoDetect:       st.AutoDetect,
		ConfigFile:       st.ConfigFile,
		Workspace:        st.Workspace,
		DetectedLanguage: st.DetectedLanguage,
		ActiveLanguage:   st.ActiveLanguage,
		ActiveServer:     st.ActiveServer,
		ActiveRoot:       st.ActiveRoot,
		Message:          st.Message,
	}
	rows := make([]panels.LSPServerRow, 0, len(st.Servers))
	for _, it := range st.Servers {
		rows = append(rows, panels.LSPServerRow{
			Language: it.Language,
			Command:  it.Command,
			Found:    it.Found,
		})
	}
	p.SetStatus(sum, rows)
}

func (m *AppModel) refreshRulesPanel() {
	p, ok := m.panels["rules"].(*panels.RulesPanel)
	if !ok || p == nil {
		return
	}
	if m.adapter == nil || m.adapter.GetCore() == nil {
		p.SetData("", "", "", false, "", "", false)
		return
	}
	root := strings.TrimSpace(m.adapter.GetCore().GetActiveRoot())
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}

	projectPath := ""
	projectContent := ""
	projectExists := false
	if strings.TrimSpace(root) != "" {
		projectPath = filepath.Join(root, ".eos", "Rules.md")
		if _, err := os.Stat(projectPath); err == nil {
			projectExists = true
			if raw, err2 := os.ReadFile(projectPath); err2 == nil {
				projectContent = string(raw)
			}
		}
	}

	globalPath := ""
	globalContent := ""
	globalExists := false
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		globalPath = filepath.Join(home, ".eos", "Rules.md")
		if _, err := os.Stat(globalPath); err == nil {
			globalExists = true
			if raw, err2 := os.ReadFile(globalPath); err2 == nil {
				globalContent = string(raw)
			}
		}
	}

	p.SetData(root, projectPath, projectContent, projectExists, globalPath, globalContent, globalExists)
}

func (m *AppModel) handleRulesSave(msg panels.RulesSaveMsg) {
	if m == nil || m.adapter == nil || m.adapter.GetCore() == nil {
		return
	}
	scope := strings.ToLower(strings.TrimSpace(msg.Scope))
	content := msg.Content

	dst := ""
	docID := ""
	switch scope {
	case "global":
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			dst = filepath.Join(home, ".eos", "Rules.md")
			docID = "~/.eos/Rules.md"
		}
	default:
		root := strings.TrimSpace(m.adapter.GetCore().GetActiveRoot())
		if root == "" {
			if wd, err := os.Getwd(); err == nil {
				root = wd
			}
		}
		if strings.TrimSpace(root) != "" {
			dst = filepath.Join(root, ".eos", "Rules.md")
			docID = ".eos/Rules.md"
		}
	}

	if strings.TrimSpace(dst) == "" {
		m.appendSystem("Rules.md 保存失败: 路径为空", "error")
		return
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		m.appendSystem(fmt.Sprintf("Rules.md 保存失败: %v", err), "error")
		return
	}
	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		m.appendSystem(fmt.Sprintf("Rules.md 保存失败: %v", err), "error")
		return
	}

	if cm := m.adapter.GetCore().GetContext(); cm != nil && strings.TrimSpace(docID) != "" {
		cm.SetPinnedDoc(docID, content, 20000)
	}

	if scope == "global" {
		m.appendSystem("已保存全局 Rules.md", "success")
	} else {
		m.appendSystem("已保存项目 Rules.md", "success")
	}
	m.refreshRulesPanel()
}

func (m *AppModel) handleMemorySave(msg panels.MemorySaveMsg) {
	if m == nil || m.adapter == nil || m.adapter.GetCore() == nil {
		return
	}
	scope := strings.ToLower(strings.TrimSpace(msg.Scope))
	root := strings.TrimSpace(m.adapter.GetCore().GetActiveRoot())
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}

	dst := ""
	docID := ""
	switch scope {
	case "global":
		dst = memory.GlobalMemoryPath()
		docID = memory.GlobalMemoryDocID
	case "session":
		dst = filepath.Join(root, ".eos", "session-memory", "session.md")
	case "index":
		dst = memory.ProjectMemoryIndexPath(root)
		docID = memory.ProjectIndexDocID
	default:
		dst = memory.ProjectMemoryPath(root)
		docID = memory.ProjectMemoryDocID
		scope = "project"
	}
	if strings.TrimSpace(dst) == "" {
		m.appendSystem("Memory 保存失败: 路径为空", "error")
		return
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		m.appendSystem(fmt.Sprintf("Memory 保存失败: %v", err), "error")
		return
	}
	if err := os.WriteFile(dst, []byte(msg.Content), 0o644); err != nil {
		m.appendSystem(fmt.Sprintf("Memory 保存失败: %v", err), "error")
		return
	}
	if scope == "global" || scope == "project" || scope == "index" {
		_ = memory.NewStore(root).RebuildIndex()
	}
	if cm := m.adapter.GetCore().GetContext(); cm != nil && strings.TrimSpace(docID) != "" {
		cm.SetPinnedDoc(docID, msg.Content, 20000)
	}
	m.appendSystem("已保存 "+scope+" memory", "success")
	m.refreshMemoryPanel()
}

func (m *AppModel) handleMemoryRebuildIndex() {
	if m == nil || m.adapter == nil || m.adapter.GetCore() == nil {
		return
	}
	root := strings.TrimSpace(m.adapter.GetCore().GetActiveRoot())
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	if err := memory.NewStore(root).RebuildIndex(); err != nil {
		m.appendSystem(fmt.Sprintf("Memory 索引重建失败: %v", err), "error")
		return
	}
	if cm := m.adapter.GetCore().GetContext(); cm != nil {
		snap := memory.LoadSnapshot(root)
		if snap.IndexExists && strings.TrimSpace(snap.IndexContent) != "" {
			cm.SetPinnedDoc(memory.ProjectIndexDocID, snap.IndexContent, 8000)
		}
	}
	m.appendSystem("已重建 memory 索引", "success")
	m.refreshMemoryPanel()
}

func (m *AppModel) handleMCPConfigSubmit(msg setup.MCPConfigSubmitMsg) tea.Cmd {
	raw := strings.TrimSpace(msg.Text)
	if raw == "" {
		m.appendSystem("请输入 MCP 配置 JSON", "warning")
		return nil
	}

	parseEntries := func(text string) ([]config.MCPEntry, error) {
		if entries, err := config.ParseLegacyMCPServersJSON([]byte(text)); err == nil && len(entries) > 0 {
			return entries, nil
		}
		var arr []config.MCPEntry
		if err := json.Unmarshal([]byte(text), &arr); err == nil && len(arr) > 0 {
			return arr, nil
		}
		var one config.MCPEntry
		if err := json.Unmarshal([]byte(text), &one); err != nil {
			return nil, err
		}
		return []config.MCPEntry{one}, nil
	}

	entries, err := parseEntries(raw)
	if err != nil {
		m.appendSystem(fmt.Sprintf("JSON 解析失败: %v", err), "error")
		return nil
	}

	if msg.Edit {
		if len(entries) != 1 {
			m.appendSystem("编辑模式只支持一个 MCPEntry 对象", "warning")
			return nil
		}
		entry := entries[0]
		if strings.TrimSpace(entry.Name) == "" {
			m.appendSystem("缺少 name 字段", "warning")
			return nil
		}
		if msg.OriginalName != "" && entry.Name != msg.OriginalName {
			if err := m.adapter.GetCore().AddMCPServers([]config.MCPEntry{entry}); err != nil {
				m.appendSystem(fmt.Sprintf("新增（用于重命名）失败: %v", err), "error")
				return nil
			}
			if !m.adapter.GetCore().DeleteMCPServer(msg.OriginalName) {
				_ = m.adapter.GetCore().DeleteMCPServer(entry.Name)
				m.appendSystem("删除旧名称失败: "+msg.OriginalName, "error")
				return nil
			}
		} else {
			if !m.adapter.GetCore().UpdateMCPServer(entry) {
				m.appendSystem("更新失败（name 不存在）: "+entry.Name, "error")
				return nil
			}
		}
	} else {
		if err := m.adapter.GetCore().AddMCPServers(entries); err != nil {
			m.appendSystem(fmt.Sprintf("新增失败: %v", err), "error")
			return nil
		}
	}

	m.activeView = "panel"
	m.activePanel = "mcp"
	m.shell.ClearInput()
	m.refreshMCPPanel()

	return func() tea.Msg {
		return MCPReloadDoneMsg{Err: m.adapter.Reload()}
	}
}

// refreshCostPanel 刷新成本统计面板
func (m *AppModel) refreshCostPanel() {
	if costPanel, ok := m.panels["cost"].(*panels.CostPanel); ok {
		// 获取模型统计
		modelStats := m.adapter.GetCore().GetModelTokenStats()
		stats := make([]panels.CostStats, 0, len(modelStats))
		for _, s := range modelStats {
			stats = append(stats, panels.CostStats{
				Model:  s.Model,
				Rounds: s.Rounds,
				Input:  s.Input,
				Reply:  s.Reply,
				Total:  s.Total,
			})
		}

		// 获取总计统计
		total := m.adapter.GetCore().GetTokenStats()
		costPanel.SetStats(stats, panels.TotalStats{
			TotalRounds: total.Rounds,
			TotalInput:  total.Input,
			TotalReply:  total.Reply,
			TotalTokens: total.Total,
		})
	}
}

// handleSettingsSave 处理保存设置
func (m *AppModel) handleSettingsSave(settings *settings.Settings, globalPredictionEnabled *bool) {
	if settings == nil {
		return
	}

	// 检查语言是否改变
	langChanged := settings.Language != m.state.Language

	// 保存到核心
	cfg, path := config.Load()
	if path != "" {
		cfg.Language = settings.Language
		if globalPredictionEnabled != nil {
			enabled := *globalPredictionEnabled
			cfg.NextMessagePredictionEnabled = &enabled
		}
		// 保存配置
		if err := config.Save(cfg, path); err != nil {
			m.appendSystem(fmt.Sprintf("Failed to save settings: %v", err), "error")
			return
		}
	}

	if wd, _ := os.Getwd(); wd != "" {
		abs := normalizeWorkspacePath(wd)
		settingsPath := filepath.Join(abs, ".eos", "settings.json")
		if err := m.adapter.GetCore().SaveSettings(settingsPath, settings); err != nil {
			m.appendSystem(fmt.Sprintf("Failed to save workspace settings: %v", err), "error")
			return
		}
	}

	// 如果语言改变了，发送语言切换消息
	if langChanged {
		m.state.Language = settings.Language
		m.shell.SetLanguage(settings.Language)
		m.Update(panels.LanguageChangeMsg{Language: settings.Language})
	}
	if globalPredictionEnabled != nil {
		m.predictionEnabled = *globalPredictionEnabled
		if !m.predictionEnabled {
			m.clearPrediction()
		}
	}

	m.appendSystem(i18n.T("settings.saved", m.state.Language), "success")
}

func (m *AppModel) handleVersionsLoad(pathRel string) {
	pathRel = strings.TrimSpace(pathRel)
	if pathRel == "" {
		return
	}
	panel, ok := m.panels["versions"].(*panels.VersionsPanel)
	if !ok {
		return
	}
	wd, _ := os.Getwd()
	abs := filepath.Join(wd, filepath.FromSlash(pathRel))
	vs, err := m.adapter.GetCore().ListVersionsForPath(abs)
	if err != nil {
		m.appendSystem(fmt.Sprintf("Failed to load versions: %v", err), "error")
		return
	}
	items := make([]panels.VersionItem, 0, len(vs))
	for _, v := range vs {
		items = append(items, panels.VersionItem{
			Timestamp: v.ID,
			Size:      v.Size,
		})
	}
	panel.SetVersions(filepath.ToSlash(pathRel), items)
}

func (m *AppModel) handleVersionsRollback(pathRel string, versionID string) {
	pathRel = strings.TrimSpace(pathRel)
	versionID = strings.TrimSpace(versionID)
	if pathRel == "" || versionID == "" {
		return
	}
	out := m.adapter.GetCore().RollbackFile(pathRel, versionID)
	m.appendSystem(out, "warning")
	m.handleVersionsLoad(pathRel)
}

func (m *AppModel) handleVersionsDelete(pathRel string, versionID string) {
	pathRel = strings.TrimSpace(pathRel)
	versionID = strings.TrimSpace(versionID)
	if pathRel == "" || versionID == "" {
		return
	}
	out := m.adapter.GetCore().DeleteVersion(pathRel, versionID)
	m.appendSystem(out, "warning")
	m.handleVersionsLoad(pathRel)
}

func (m *AppModel) handleVersionsDeleteFile(pathRel string) {
	pathRel = strings.TrimSpace(pathRel)
	if pathRel == "" {
		return
	}
	out := m.adapter.GetCore().DeleteAllVersions(pathRel)
	m.appendSystem(out, "warning")
	m.refreshVersionsPanel()
}

func (m *AppModel) handleVersionsDeleteAll() {
	out := m.adapter.GetCore().DeleteAllFileVersions()
	m.appendSystem(out, "warning")
	m.refreshVersionsPanel()
}

func (m *AppModel) handleHiddenLegalSlash() tea.Cmd {
	m.appendSystem("Copyright (c) 2026 DreamSailing", "info")
	m.appendSystem("License: EOS Non-Commercial License v1.1 (EOS-NCL-1.1)", "info")
	m.appendSystem("SPDX-License-Identifier: EOS-NCL-1.1", "info")
	m.appendSystem("Contact: smart-os@qq.com", "info")
	m.appendSystem(fmt.Sprintf("Version: %s | Commit: %s | Build: %s", version.AppVersion, version.BuildCommit, version.BuildDate), "info")
	return nil
}
