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
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/i18n"
	mcppkg "github.com/dreamSailing/eos/internal/mcp"
	"github.com/dreamSailing/eos/internal/pkg/clip"
	"github.com/dreamSailing/eos/internal/pkg/filedialog"
	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/state"
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
	"github.com/dreamSailing/eos/pkg/coreapi"
	sidecarclient "github.com/dreamSailing/eos/pkg/coreapi/sidecar/client"

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

	adapter *adapter.CoreClientAdapter
	styles  *styles.Styles

	// 消息渲染器
	msgRenderer *messages.Renderer

	// 主视图
	shell *shell.Model

	// 面板系统
	panels      map[string]panels.Panel
	activePanel string

	// 其他视图
	helpView                 *help.HelpView
	setupView                any // 可以是 *setup.SetupView 或 *setup.ModelSetupView
	confirmView              *confirm.Model
	prevView                 string
	inlinePermissionReq      *confirm.Request
	inlinePermissionSelected int

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

	actionHits []bubbleActionHit

	pendingImagePaths   []string
	pendingPlanDownload *planDownloadRequest

	predictionText        string
	predictionSeq         int
	predictionDebounceSeq int
	predictionEnabled     bool

	trustPendingPath   string
	trustPendingAction string
	activeCancel       context.CancelFunc
	stopRequested      bool
}

type bubbleActionHit struct {
	y      int
	x0     int
	x1     int
	idx    int
	action string
	text   string
}

type planDownloadRequest struct {
	HistoryIndex int
}

var choosePlanDownloadDirectory = filedialog.ChooseDirectory
var writePlanDownloadFile = os.WriteFile
var planDownloadNow = time.Now

func (r bubbleActionHit) matches(action string) bool {
	return strings.EqualFold(strings.TrimSpace(r.action), strings.TrimSpace(action))
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
	tokens, ratio, err := m.adapter.CurrentContextUsage(context.Background())
	if err != nil {
		return
	}
	m.shell.SetContextUsage(tokens, ratio)
}

func (m *AppModel) updateBGTaskCountUI() {
	if m == nil || m.shell == nil || m.adapter == nil {
		return
	}
	tasks, err := m.adapter.Tasks(context.Background())
	if err != nil {
		return
	}
	m.shell.SetBGTaskCount(len(tasks))
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
		text, err := m.adapter.PredictNextUserMessage(context.Background(), draft)
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

func (m *AppModel) updateInlinePermissionUI() {
	if m == nil || m.shell == nil {
		return
	}
	if m.inlinePermissionReq == nil {
		m.shell.ClearPromptOverlay()
		return
	}
	m.shell.SetPromptOverlay(confirm.RenderInlinePermission(
		m.styles,
		m.state.Language,
		*m.inlinePermissionReq,
		m.inlinePermissionSelected,
		m.width,
	))
}

func (m *AppModel) showInlinePermission(req confirm.Request) {
	reqCopy := req
	m.inlinePermissionReq = &reqCopy
	m.inlinePermissionSelected = 0
	m.updateInlinePermissionUI()
	if m.shell != nil {
		m.shell.BlurInput()
	}
}

func (m *AppModel) clearInlinePermission() {
	m.inlinePermissionReq = nil
	m.inlinePermissionSelected = 0
	if m.shell != nil {
		m.shell.ClearPromptOverlay()
	}
}

func (m *AppModel) refreshAILive() {
	if m == nil || m.shell == nil {
		return
	}
	if !m.state.Processing {
		m.shell.ClearLive()
		m.shell.SetStatusHints(false, false)
		return
	}
	var blocks []string
	thinking := strings.TrimSpace(m.thinkingLive.String())
	thinkingShown := m.state.Thinking && state.Thinking() && thinking != ""
	if thinkingShown {
		if m.msgRenderer != nil {
			hint := i18n.T("status.hint.thinking_expand", m.state.Language)
			blocks = append(blocks, m.msgRenderer.RenderThinkingWithHint(thinking, time.Since(m.currentAIStartTime), m.thinkingExpanded, nil, hint))
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
		m.shell.SetStatusHints(false, false)
		m.shell.ClearLive()
		return
	}
	m.shell.SetStatusHints(liveShown, thinkingShown)
	m.shell.SetLive(strings.Join(blocks, "\n\n"))
}

func (m *AppModel) clearCurrentThinking() {
	if m == nil {
		return
	}
	m.thinkingLive.Reset()
	m.state.Thinking = false
	m.thinkingExpanded = false
	if m.shell != nil {
		m.shell.SetThinking(false, "")
		m.shell.SetThinkingExpanded(false)
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

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
	kind          string
	content       string
	timestamp     time.Time
	tokens        int
	duration      time.Duration
	level         string
	toolID        string
	toolName      string
	toolParams    map[string]any
	toolOutput    string
	toolSuccess   bool
	toolStatus    string
	agentName     string
	agentID       string
	agentEvent    string
	sourceAgent   string
	sourceAgentID string
	task          string
	executionMode string
	rawMarkdown   string
	copiedAt      time.Time
}

func resolveShellWelcomeInfo(adapter *adapter.CoreClientAdapter) (string, string) {
	modelName, modelBase := adapter.GetModelInfo()
	if modelName == "" {
		modelName = "(none)"
	}
	if modelBase == "" {
		modelBase = "(none)"
	}
	return modelName, modelBase
}

// NewAppModelFromCoreClient 用已 handshake 的 sidecar Client 构造 AppModel。
// 这是生产路径：TUI 通过 pkg/coreapi/sidecar/client 启动 eos-core --app-server --stdio。
func NewAppModelFromCoreClient(client *sidecarclient.Client) *AppModel {
	return newAppModel(hydrateCatalogFromAdapter(adapter.NewCoreClientAdapter(client)))
}

// NewAppModelFromCoreEngine 直接用 coreapi.Engine 构造 AppModel。
// 供测试场景使用（不启动 sidecar 子进程）。
func NewAppModelFromCoreEngine(engine coreapi.Engine) *AppModel {
	return newAppModel(hydrateCatalogFromAdapter(adapter.NewCoreClientAdapterFromEngine(engine)))
}

func hydrateCatalogFromAdapter(coreAdapter *adapter.CoreClientAdapter) *adapter.CoreClientAdapter {
	if coreAdapter == nil {
		return nil
	}
	catalog, err := coreAdapter.ModelCatalog(context.Background())
	if err != nil {
		ai.ApplyCoreModelCatalog(coreapi.ModelCatalogState{})
		return coreAdapter
	}
	ai.ApplyCoreModelCatalog(catalog)
	return coreAdapter
}

func newAppModel(adapter *adapter.CoreClientAdapter) *AppModel {
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
	_ = adapter.SetExecutionMode(context.Background(), "auto")

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

	panelMap["settings"] = panels.NewSettingsPanel(styles, nil, lang)
	mcpPanel := panels.NewMCPPanel(styles, lang)
	// 加载 MCP 服务器配置
	var mcpServers []panels.MCPServer
	configServers, _ := adapter.MCPServers(context.Background())
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
	panelMap["tasks"] = panels.NewTasksPanel(styles, lang, adapter)

	setupView := any(setup.NewSetupView(styles))
	activeView := "shell"
	initialSetupFlow := false
	models, _, _ := adapter.ModelEntries(context.Background())
	hasConfiguredModel := len(models) > 0
	if !hasConfiguredModel {
		wizard := setup.NewModelSetupWizard(styles, lang)
		wizard.SetSize(80, 24)
		setupView = wizard
		activeView = "setup"
		initialSetupFlow = true
		shellModel.BlurInput()
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
		abs := config.NormalizeWorkspacePath(p)
		rememberKnownWorkspace(abs, true)
		if m.isWorkspaceTrusted(abs) {
			_ = m.adapter.StartContextEngine(context.Background(), abs)
			_, _ = m.adapter.Settings(context.Background())
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
		m.updateInlinePermissionUI()
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
			if cmd := m.tryHandleBubbleActionAt(msg.X, msg.Y); cmd != nil {
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

		if m.activeView == "shell" && m.inlinePermissionReq != nil {
			if handled, cmd := m.handleInlinePermissionKey(msg); handled {
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
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

	case adapter.RuntimeEvent:
		return m.handleRuntimeEvent(msg)

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
		if !m.state.Processing {
			return m, nil
		}
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

	case ModeChangedMsg:
		mode := strings.TrimSpace(msg.Mode)
		if mode != "" {
			m.state.ExecutionMode = mode
			m.shell.SetExecutionMode(mode)
		}

	case AgentTaskMsg:
		m.delegatedThisRound = true
		m.shell.ClearLive()
		m.aiLive.Reset()
		m.clearCurrentThinking()
		m.appendHistory(historyEntry{
			kind:          "agent.task",
			agentName:     msg.AgentName,
			agentID:       msg.AgentID,
			agentEvent:    msg.Event,
			sourceAgent:   msg.SourceAgentName,
			sourceAgentID: msg.SourceAgentID,
			task:          msg.Task,
			timestamp:     time.Now(),
		})

	case AgentFinalMsg:
		m.delegatedThisRound = true
		m.shell.ClearLive()
		m.aiLive.Reset()
		m.clearCurrentThinking()
		m.lastAgentFinal = msg.Content
		m.appendHistory(historyEntry{
			kind:          "agent.final",
			agentName:     msg.AgentName,
			agentID:       msg.AgentID,
			agentEvent:    msg.Event,
			sourceAgent:   msg.SourceAgentName,
			sourceAgentID: msg.SourceAgentID,
			content:       msg.Content,
			rawMarkdown:   msg.Content,
			executionMode: m.state.ExecutionMode,
			timestamp:     time.Now(),
		})
		m.state.Processing = false
		m.shell.SetProcessing(false)
		m.shell.SetStatusHints(false, false)
		m.activeCancel = nil
		m.stopRequested = false

	case PromptRequestMsg:
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
		if req.Kind == "permission" {
			m.showInlinePermission(req)
			return m, nil
		}
		if m.confirmView == nil {
			m.prevView = m.activeView
		}
		m.confirmView = confirm.New(m.styles, m.state.Language, req)
		m.confirmView.SetSize(m.width, m.height)
		m.activeView = "confirm"
		m.shell.BlurInput()
		return m, nil

	case confirm.ResultMsg:
		if msg.Kind == "permission" {
			m.clearInlinePermission()
			if msg.ID != "" {
				_ = m.adapter.RespondPrompt(context.Background(), msg.ID, msg.Kind, adapter.PromptResponse{
					Decision:    msg.Decision,
					Option:      msg.Option,
					OptionIndex: msg.OptionIndex,
					Text:        msg.Text,
				})
			}
			if m.activeView == "shell" {
				m.shell.FocusInput()
			}
			return m, nil
		}
		if strings.HasPrefix(msg.Kind, "bg_kill:") {
			id, _ := strings.CutPrefix(msg.Kind, "bg_kill:")
			id = strings.TrimSpace(id)
			if msg.Decision == "confirm" && id != "" {
				if err := m.adapter.KillTask(context.Background(), id); err != nil {
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
			if err := m.addTrustedWorkspace(path); err != nil {
				m.appendSystem(fmt.Sprintf("信任工作区失败: %v", err), "error")
				return m, nil
			}
			if err := m.adapter.TrustWorkspace(context.Background(), path); err != nil {
				m.appendSystem(fmt.Sprintf("信任工作区已保存，但同步到核心失败: %v", err), "warning")
			}
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
				_ = m.adapter.StartContextEngine(context.Background(), path)
				_, _ = m.adapter.Settings(context.Background())
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
			if err := m.adapter.AddWorkspace(context.Background(), p); err != nil {
				m.appendSystem(err.Error(), "error")
				return m, nil
			}
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
		if msg.Kind == "plan_download_path" {
			req := m.pendingPlanDownload
			m.pendingPlanDownload = nil
			if msg.Decision != "confirm" || req == nil {
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
			dir := strings.TrimSpace(msg.Text)
			path, err := m.savePlanHistoryEntryToDir(req.HistoryIndex, dir)
			if err != nil {
				m.appendSystem(err.Error(), "error")
			} else {
				m.appendSystem(fmt.Sprintf(i18n.T("plan.download.saved", m.state.Language), path), "success")
			}
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
			_ = m.adapter.RespondPrompt(context.Background(), msg.ID, msg.Kind, adapter.PromptResponse{
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

	case panels.ModelRefreshMsg:
		m.refreshModelsPanel()

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
		m.updateInlinePermissionUI()

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
		if message, err := m.adapter.CompactContext(context.Background()); err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("上下文压缩失败", "Context compact failed"), err), "error")
		} else if strings.TrimSpace(message) != "" {
			m.appendSystem(message, "success")
		} else {
			m.appendSystem(i18n.T("context.compacted", m.state.Language), "success")
		}
		m.refreshContextPanel()
	case panels.ContextClearMsg:
		if err := m.adapter.ClearContext(context.Background()); err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("清空上下文失败", "Context clear failed"), err), "error")
		} else {
			m.shell.ClearContent()
			m.history = m.history[:0]
			m.actionHits = nil
			m.appendSystem(i18n.T("context.cleared", m.state.Language), "success")
		}
		m.refreshContextPanel()
	case panels.ContextExportMsg:
		exportPath := filepath.Join(m.currentWorkspaceRoot(), ".eos", fmt.Sprintf("context-%s.md", time.Now().Format("20060102-150405")))
		if err := m.adapter.ExportContext(context.Background(), exportPath); err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("上下文导出失败", "Context export failed"), err), "error")
		} else {
			m.appendSystem(fmt.Sprintf("%s: %s", m.localize("上下文已导出", "Context exported"), exportPath), "success")
		}

	case panels.MemoryRefreshMsg:
		m.refreshMemoryPanel()
	case panels.MemoryRebuildIndexMsg:
		m.handleMemoryRebuildIndex()
	case panels.MemorySaveMsg:
		m.handleMemorySave(msg)

	// Cost 消息处理
	case panels.CostClearMsg:
		m.adapter.ClearTokenHistory()
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
		m.actionHits = nil
	case "/exit":
		return tea.Quit
	case "/init":
		m.shell.ClearInput()
		return m.initEOSMD()
	case "/init-verifiers":
		return m.handleInitVerifiersSlash(args)
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
		if message, err := m.adapter.CompactContext(context.Background()); err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("上下文压缩失败", "Context compact failed"), err), "error")
		} else if strings.TrimSpace(message) != "" {
			m.appendSystem(message, "success")
		} else {
			m.appendSystem(i18n.T("context.compacted", m.state.Language), "success")
		}
		m.refreshContextPanel()
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
	case "/verify":
		return m.handleVerifySlash(args)
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
	if m != nil && m.adapter != nil {
		root = strings.TrimSpace(m.adapter.ActiveWorkspace(context.Background()))
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
	_ = m.adapter.PinContextDocument(context.Background(), "EOS.md", content, 20000)
	if existed {
		m.appendSystem("已更新 EOS.md", "success")
	} else {
		m.appendSystem("已生成 EOS.md", "success")
	}
	return nil
}

func (m *AppModel) tryInvokeSkillSlash(skillName string, args []string) bool {
	arguments := strings.TrimSpace(strings.Join(args, " "))
	invoked, err := m.adapter.InvokeSkill(context.Background(), skillName, arguments)
	if err != nil {
		m.appendSystem(fmt.Sprintf("Skill 启用失败: %v", err), "error")
		return true
	}
	if !invoked {
		return false
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

	items, err := m.adapter.Workspaces(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("Failed to load workspaces: %v", err), "error")
		panel.SetWorkspaces(nil, "")
		return
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Path) < strings.ToLower(items[j].Path) })

	active := ""
	workspaces := make([]panels.Workspace, 0, len(items))
	for _, item := range items {
		p := strings.TrimSpace(item.Path)
		if p == "" {
			continue
		}
		if item.Active {
			active = p
		}
		workspaces = append(workspaces, panels.Workspace{
			Name: filepath.Base(p),
			Path: p,
		})
	}
	if strings.TrimSpace(active) == "" {
		active = m.currentWorkspaceRoot()
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

	versions, err := m.adapter.Versions(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("Failed to load versions: %v", err), "error")
		panel.SetFiles(nil)
		return
	}
	byFile := map[string]panels.FileItem{}
	for _, version := range versions {
		file := filepath.ToSlash(strings.TrimSpace(version.File))
		if file == "" {
			continue
		}
		item := byFile[file]
		item.Path = file
		item.Count++
		if version.CreatedAt.After(item.Last) {
			item.Last = version.CreatedAt
		}
		byFile[file] = item
	}
	items := make([]panels.FileItem, 0, len(byFile))
	for _, item := range byFile {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].Last.Equal(items[j].Last) {
			return items[i].Last.After(items[j].Last)
		}
		return strings.ToLower(items[i].Path) < strings.ToLower(items[j].Path)
	})
	panel.SetFiles(items)
}

func (m *AppModel) handleWorkspaceRemove(path string) {
	if path == "" {
		return
	}
	if err := m.adapter.RemoveWorkspace(context.Background(), path); err != nil {
		m.appendSystem(err.Error(), "error")
		return
	}
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
	if err := m.adapter.UseWorkspace(context.Background(), path); err != nil {
		m.appendSystem(fmt.Sprintf("工作区切换失败: %v", err), "error")
		return nil
	}
	_ = os.Chdir(path)
	_, _ = m.adapter.Settings(context.Background())
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

func (m *AppModel) buildInlinePermissionResult(decision string) confirm.ResultMsg {
	if m.inlinePermissionReq == nil {
		return confirm.ResultMsg{Kind: "permission", Decision: decision, OptionIndex: -1}
	}
	req := m.inlinePermissionReq
	option := ""
	idx := m.inlinePermissionSelected
	if idx >= 0 && idx < len(req.Options) {
		option = req.Options[idx]
	}
	if decision == "" {
		switch option {
		case "allow_once":
			decision = "allow"
		case "allow_session":
			decision = "session"
		default:
			decision = "deny"
		}
	}
	return confirm.ResultMsg{
		ID:          req.ID,
		Kind:        req.Kind,
		Decision:    decision,
		Option:      option,
		OptionIndex: idx,
	}
}

func (m *AppModel) handleInlinePermissionKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if m == nil || m.inlinePermissionReq == nil {
		return false, nil
	}
	switch msg.String() {
	case "up":
		if m.inlinePermissionSelected > 0 {
			m.inlinePermissionSelected--
			m.updateInlinePermissionUI()
		}
		return true, nil
	case "down":
		if m.inlinePermissionSelected < len(m.inlinePermissionReq.Options)-1 {
			m.inlinePermissionSelected++
			m.updateInlinePermissionUI()
		}
		return true, nil
	case "enter":
		result := m.buildInlinePermissionResult("")
		return true, func() tea.Msg { return result }
	case "esc":
		result := m.buildInlinePermissionResult("deny")
		return true, func() tea.Msg { return result }
	default:
		if len(msg.String()) == 1 {
			k := msg.String()[0]
			if k >= '1' && k <= '9' {
				idx := int(k - '1')
				if idx >= 0 && idx < len(m.inlinePermissionReq.Options) {
					m.inlinePermissionSelected = idx
					m.updateInlinePermissionUI()
					return true, nil
				}
			}
		}
	}
	return false, nil
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
	if config.IsWorkspaceTrustedLocal(path) {
		return true
	}
	cfg, _ := config.Load()
	want := config.NormalizeWorkspacePath(path)
	for _, p := range cfg.TrustedWorkspaces {
		if config.PathsEqual(config.NormalizeWorkspacePath(p), want) {
			return true
		}
	}
	return false
}

func (m *AppModel) addTrustedWorkspace(path string) error {
	return config.TrustWorkspaceLocal(path)
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
	return config.NormalizeWorkspacePath(abs), nil
}

// sendMessage 发送消息
func (m *AppModel) sendMessage() tea.Cmd {
	value := m.shell.GetInputValue()
	expanded := value
	if strings.Contains(strings.ToLower(expanded), "#problems_and_diagnostics") {
		md := ""
		if m.adapter != nil {
			md = m.adapter.LSPDiagnosticsMarkdown(context.Background())
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
	m.clearCurrentThinking()
	m.shell.SetStatusHints(false, false)
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
		out, err := m.adapter.ExecuteBash(ctx, value)
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

	ctx := context.Background()
	preview, err := m.adapter.ContextPreview(ctx)
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("刷新上下文失败", "Failed to refresh context"), err), "error")
		return
	}
	stats, err := m.adapter.ContextStats(ctx)
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("刷新上下文统计失败", "Failed to refresh context stats"), err), "error")
		return
	}

	model, _ := m.adapter.GetModelInfo()
	panel.SetStats(model, ai.ContextWindowTokens(model), 0, stats.Estimated)

	msgs := make([]panels.ContextMessage, 0, len(preview))
	for _, line := range preview {
		role, content := parseContextPreviewLine(line)
		if strings.TrimSpace(content) == "" {
			continue
		}
		msgs = append(msgs, panels.ContextMessage{
			Role:    role,
			Content: content,
			Tokens:  estimateDisplayTokens(content),
		})
	}
	panel.SetMessages(msgs)
}

func parseContextPreviewLine(line string) (string, string) {
	line = strings.TrimSpace(line)
	role, content, ok := strings.Cut(line, ":")
	if !ok {
		return "message", line
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = "message"
	}
	return role, strings.TrimSpace(content)
}

func estimateDisplayTokens(content string) int {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0
	}
	runes := len([]rune(content))
	tokens := (runes + 3) / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}

func rulesSnapshotDocument(snapshot coreapi.RulesSnapshot, scope string) coreapi.RuleDocument {
	scope = strings.ToLower(strings.TrimSpace(scope))
	for _, doc := range snapshot.Documents {
		if strings.EqualFold(strings.TrimSpace(doc.Scope), scope) {
			return doc
		}
	}
	return coreapi.RuleDocument{Scope: scope}
}

func memorySnapshotDocument(snapshot coreapi.MemorySnapshot, scopes ...string) coreapi.MemoryDocument {
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		for _, doc := range snapshot.Documents {
			if strings.EqualFold(strings.TrimSpace(doc.Scope), scope) {
				return doc
			}
		}
	}
	if len(scopes) > 0 {
		return coreapi.MemoryDocument{Scope: strings.ToLower(strings.TrimSpace(scopes[0]))}
	}
	return coreapi.MemoryDocument{}
}

func (m *AppModel) refreshMemoryPanel() {
	if m == nil || m.adapter == nil {
		return
	}
	panel, ok := m.panels["memory"].(*panels.MemoryPanel)
	if !ok || panel == nil {
		return
	}
	root := strings.TrimSpace(m.currentWorkspaceRoot())
	snap, err := m.adapter.MemorySnapshot(context.Background())
	if err != nil {
		panel.SetData(root, "", "", false, "", "", false, "", "", false, "", "", false)
		return
	}
	global := memorySnapshotDocument(snap, "global")
	project := memorySnapshotDocument(snap, "project")
	sessionDoc := memorySnapshotDocument(snap, "session")
	index := memorySnapshotDocument(snap, "index", "project-index")
	panel.SetData(root, global.Path, global.Content, global.Exists, project.Path, project.Content, project.Exists, sessionDoc.Path, sessionDoc.Content, sessionDoc.Exists, index.Path, index.Content, index.Exists)
}

// handleRuntimeEvent 处理 runtime event 消息
func (m *AppModel) handleRuntimeEvent(e adapter.RuntimeEvent) (tea.Model, tea.Cmd) {
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
	m.clearCurrentThinking()
	m.shell.SetStatusHints(false, false)
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
		return m.msgRenderer.RenderAIResponseAtWithActions(e.content, e.tokens, e.duration, true, e.timestamp, m.bubbleActionsForEntry(e))
	case "agent.task":
		return m.msgRenderer.RenderAgentTaskAt(e.agentName, e.agentID, e.sourceAgent, e.sourceAgentID, e.agentEvent, e.task, e.timestamp)
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
		return m.msgRenderer.RenderAgentFinalAtWithActions(e.agentName, e.agentID, e.sourceAgent, e.sourceAgentID, e.agentEvent, e.content, e.timestamp, m.bubbleActionsForEntry(e))
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

func (m *AppModel) bubbleActionsForEntry(e historyEntry) []messages.BubbleAction {
	if (e.kind != "ai" && e.kind != "agent.final") || strings.TrimSpace(e.content) == "" {
		return nil
	}
	actions := []messages.BubbleAction{
		{Kind: "copy", Label: m.copyButtonLabel(e)},
	}
	if strings.EqualFold(strings.TrimSpace(e.executionMode), "plan") && strings.TrimSpace(e.rawMarkdown) != "" {
		actions = append(actions, messages.BubbleAction{
			Kind:  "download",
			Label: i18n.T("op.download", m.state.Language),
		})
	}
	return actions
}

func (m *AppModel) appendHistory(e historyEntry) {
	m.history = append(m.history, e)
	rendered := m.renderHistoryEntry(e)
	m.trackBubbleActionsAt(m.shell.ContentLineCount(), len(m.history)-1, e, rendered)
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
	m.actionHits = nil
	var sb strings.Builder
	lineCount := 1
	for idx, e := range m.history {
		rendered := m.renderHistoryEntry(e)
		m.trackBubbleActionsAt(lineCount, idx, e, rendered)
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

func (m *AppModel) trackBubbleActionsAt(startLine int, idx int, e historyEntry, rendered string) {
	if m.msgRenderer == nil {
		return
	}
	actions := m.bubbleActionsForEntry(e)
	if len(actions) == 0 {
		return
	}
	payload := strings.TrimSpace(e.content)
	if payload == "" {
		return
	}
	lines := strings.Split(stripANSI(rendered), "\n")
	for i, line := range lines {
		for _, action := range actions {
			label := strings.TrimSpace(action.Label)
			if label == "" {
				continue
			}
			bi := strings.LastIndex(line, label)
			if bi < 0 {
				continue
			}
			x0 := runeIndex(line, bi)
			x1 := x0 + len([]rune(label)) - 1
			m.actionHits = append(m.actionHits, bubbleActionHit{
				y:      startLine + i,
				x0:     x0,
				x1:     x1,
				idx:    idx,
				action: action.Kind,
				text:   payload,
			})
		}
	}
}

func (m *AppModel) tryHandleBubbleActionAt(x, y int) tea.Cmd {
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
	for _, h := range m.actionHits {
		if h.y != line {
			continue
		}
		if lx < h.x0 || lx > h.x1 {
			continue
		}
		switch {
		case h.matches("copy"):
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
		case h.matches("download"):
			return m.handlePlanDownloadAction(h.idx)
		}
	}
	return nil
}

func (m *AppModel) handlePlanDownloadAction(idx int) tea.Cmd {
	if _, ok := m.planDownloadEntry(idx); !ok {
		m.appendSystem(i18n.T("plan.download.unavailable", m.state.Language), "warning")
		return func() tea.Msg { return nil }
	}
	dir, err := choosePlanDownloadDirectory(i18n.T("plan.download.chooser.title", m.state.Language))
	switch {
	case err == nil:
		path, saveErr := m.savePlanHistoryEntryToDir(idx, dir)
		if saveErr != nil {
			m.appendSystem(saveErr.Error(), "error")
		} else {
			m.appendSystem(fmt.Sprintf(i18n.T("plan.download.saved", m.state.Language), path), "success")
		}
	case filedialog.IsCanceled(err):
		return func() tea.Msg { return nil }
	case filedialog.IsUnavailable(err):
		m.pendingPlanDownload = &planDownloadRequest{HistoryIndex: idx}
		m.openConfirm(confirm.Request{
			Kind:      "plan_download_path",
			Title:     i18n.T("plan.download.fallback.title", m.state.Language),
			Question:  i18n.T("plan.download.fallback.question", m.state.Language),
			Options:   []string{i18n.T("op.save", m.state.Language)},
			AllowText: true,
			TextHint:  i18n.T("plan.download.fallback.hint", m.state.Language),
		})
	default:
		m.appendSystem(fmt.Sprintf(i18n.T("plan.download.failed", m.state.Language), err), "error")
	}
	return func() tea.Msg { return nil }
}

func (m *AppModel) planDownloadEntry(idx int) (historyEntry, bool) {
	if idx < 0 || idx >= len(m.history) {
		return historyEntry{}, false
	}
	entry := m.history[idx]
	if !strings.EqualFold(strings.TrimSpace(entry.executionMode), "plan") {
		return historyEntry{}, false
	}
	if strings.TrimSpace(entry.rawMarkdown) == "" {
		return historyEntry{}, false
	}
	return entry, true
}

func (m *AppModel) savePlanHistoryEntryToDir(idx int, rawDir string) (string, error) {
	entry, ok := m.planDownloadEntry(idx)
	if !ok {
		return "", fmt.Errorf("%s", i18n.T("plan.download.unavailable", m.state.Language))
	}
	dir, err := resolveWorkspaceInputPath(rawDir)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(dir)
	if err != nil || fi == nil || !fi.IsDir() {
		return "", fmt.Errorf(i18n.T("plan.download.not_directory", m.state.Language), dir)
	}
	path := filepath.Join(dir, m.nextPlanDownloadFileName(entry.timestamp))
	path = uniqueAvailablePath(path)
	if err := writePlanDownloadFile(path, []byte(entry.rawMarkdown), 0o644); err != nil {
		return "", fmt.Errorf(i18n.T("plan.download.failed", m.state.Language), err)
	}
	return path, nil
}

func (m *AppModel) nextPlanDownloadFileName(ts time.Time) string {
	stamp := ts
	if stamp.IsZero() {
		stamp = planDownloadNow()
	}
	name := "plan"
	if m != nil && m.adapter != nil {
		if sessionID, err := m.adapter.CurrentSessionID(context.Background()); err == nil {
			if cleaned := sanitizePlanFileNameSegment(sessionID); cleaned != "" {
				name += "-" + cleaned
			}
		}
	}
	return fmt.Sprintf("%s-%s.md", name, stamp.Format("20060102-150405"))
}

func sanitizePlanFileNameSegment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func uniqueAvailablePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	for i := 2; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
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
	if strings.TrimSpace(modelName) != "" && !ai.SupportsVisionFromCatalog(modelName) {
		m.appendSystem("当前模型可能不具备视觉能力，图片可能无法解析", "warning")
	}
	return func() tea.Msg { return nil }
}

func (m *AppModel) toggleThinkingExpand() tea.Cmd {
	if m == nil || !m.state.Thinking || strings.TrimSpace(m.thinkingLive.String()) == "" {
		return nil
	}
	m.thinkingExpanded = !m.thinkingExpanded
	if m.shell != nil {
		m.shell.SetThinkingExpanded(m.thinkingExpanded)
	}
	m.refreshAILive()
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
		if m.activeView == "shell" && m.shell != nil && m.shell.GetMode() == shell.ModeAI && m.state.Thinking && strings.TrimSpace(m.thinkingLive.String()) != "" {
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
		if strings.TrimSpace(msg.Content) != "" && strings.TrimSpace(m.thinkingLive.String()) != "" {
			m.clearCurrentThinking()
		}
		m.aiLive.WriteString(msg.Content)
		m.currentAITokens += len(msg.Content) / 4
		m.refreshAILive()
	case "final":
		duration := time.Since(m.currentAIStartTime)
		m.shell.ClearLive()
		m.aiLive.Reset()
		m.clearCurrentThinking()
		m.shell.SetStatusHints(false, false)
		mainContent := strings.TrimSpace(msg.Content)
		agentContent := strings.TrimSpace(m.lastAgentFinal)
		if !(m.delegatedThisRound && mainContent != "" && agentContent != "" && mainContent == agentContent) {
			m.appendHistory(historyEntry{
				kind:          "ai",
				content:       msg.Content,
				rawMarkdown:   msg.Content,
				executionMode: m.state.ExecutionMode,
				timestamp:     time.Now(),
				tokens:        m.currentAITokens,
				duration:      duration,
			})
		}
		m.state.Processing = false
		m.shell.SetProcessing(false)
		m.activeCancel = nil
		m.stopRequested = false
		return m.schedulePrediction(m.shell.GetInputValue())
	case "error":
		m.clearPrediction()
		m.shell.ClearLive()
		m.aiLive.Reset()
		m.clearCurrentThinking()
		m.shell.SetStatusHints(false, false)
		m.appendHistory(historyEntry{kind: "system", content: msg.Content, level: "error"})
		m.state.Processing = false
		m.shell.SetProcessing(false)
		m.activeCancel = nil
		m.stopRequested = false
	}
	return nil
}

// handleToolCall 处理工具调用
func (m *AppModel) handleToolCall(msg ToolCallMsg) tea.Cmd {
	m.clearCurrentThinking()
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

// handleModelSelect 处理模型选择
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
		m.appendSystem("Failed to sync model from environment (EOS_API_BASE and EOS_API_KEY required)", "error")
		return
	}
	m.refreshModelsPanel()
	m.appendSystem("Synced model from environment variables", "success")
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

// handleMCPToggle 处理 MCP 服务器切换
func (m *AppModel) handleMCPToggle(msg panels.MCPToggleMsg) tea.Cmd {
	configServers, err := m.adapter.MCPServers(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("Failed to toggle MCP server: %s", msg.Name), "error")
		return nil
	}
	for _, s := range configServers {
		if s.Name != msg.Name {
			continue
		}
		next := !s.Enabled
		if err := m.adapter.SetMCPEnabled(context.Background(), msg.Name, next); err != nil {
			m.appendSystem(fmt.Sprintf("Failed to toggle MCP server: %s", msg.Name), "error")
			return nil
		}
		m.refreshMCPPanel()
		status := i18n.T("mcp.status.disabled", m.state.Language)
		if next {
			status = i18n.T("mcp.status.enabled", m.state.Language)
		}
		m.appendSystem(fmt.Sprintf(i18n.T("mcp.msg.toggled", m.state.Language), status, msg.Name), "success")
		return func() tea.Msg {
			return MCPReloadDoneMsg{Err: m.adapter.Reload()}
		}
	}
	m.appendSystem(fmt.Sprintf("Failed to toggle MCP server: %s", msg.Name), "error")
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
	entries, err := m.adapter.MCPServers(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("加载 MCP 配置失败: %v", err), "error")
		return
	}
	for _, e := range entries {
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
	if err := m.adapter.DeleteMCPServer(context.Background(), msg.Name); err != nil {
		m.appendSystem(fmt.Sprintf("Failed to delete MCP server: %s", msg.Name), "error")
		return nil
	}
	m.refreshMCPPanel()
	m.appendSystem(fmt.Sprintf(i18n.T("mcp.msg.deleted", m.state.Language), msg.Name), "success")
	return func() tea.Msg {
		return MCPReloadDoneMsg{Err: m.adapter.Reload()}
	}
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
	cfgServers, err := m.adapter.MCPServers(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("Failed to load MCP servers: %v", err), "error")
		mcpPanel.SetServers(nil)
		return
	}
	out := make([]panels.MCPServer, 0, len(cfgServers))
	for _, s := range cfgServers {
		out = append(out, panels.MCPServer{
			Name:    s.Name,
			Type:    string(s.Type),
			Enabled: s.Enabled,
		})
	}
	mcpPanel.SetServers(out)
	browser, err := m.adapter.BrowserStatus(context.Background())
	if err != nil {
		browser = coreapi.BrowserStatus{}
	}
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
	if m.adapter == nil {
		p.SetStatus(panels.LSPPanelSummary{Message: "no core"}, nil)
		return
	}
	servers, err := m.adapter.LSPServers(context.Background())
	if err != nil {
		p.SetStatus(panels.LSPPanelSummary{Message: err.Error()}, nil)
		return
	}
	sum := panels.LSPPanelSummary{
		Enabled:    true,
		AutoDetect: true,
		Workspace:  m.currentWorkspaceRoot(),
		Message:    "via JSON-RPC",
	}
	rows := make([]panels.LSPServerRow, 0, len(servers))
	for _, it := range servers {
		found := !strings.EqualFold(strings.TrimSpace(it.Status), "not_found")
		if strings.EqualFold(strings.TrimSpace(it.Status), "running") {
			sum.ActiveLanguage = strings.TrimSpace(it.Language)
			sum.ActiveServer = strings.TrimSpace(it.Command)
			sum.ActiveRoot = sum.Workspace
		}
		rows = append(rows, panels.LSPServerRow{
			Language: it.Language,
			Command:  it.Command,
			Found:    found,
		})
	}
	p.SetStatus(sum, rows)
}

func (m *AppModel) refreshRulesPanel() {
	p, ok := m.panels["rules"].(*panels.RulesPanel)
	if !ok || p == nil {
		return
	}
	if m.adapter == nil {
		p.SetData("", "", "", false, "", "", false)
		return
	}
	snapshot, err := m.adapter.RulesSnapshot(context.Background())
	if err != nil {
		p.SetData("", "", "", false, "", "", false)
		return
	}
	project := rulesSnapshotDocument(snapshot, "project")
	global := rulesSnapshotDocument(snapshot, "global")
	p.SetData(snapshot.ActiveRoot, project.Path, project.Content, project.Exists, global.Path, global.Content, global.Exists)
}

func (m *AppModel) refreshSettingsPanel() {
	if m == nil || m.adapter == nil {
		return
	}
	p, ok := m.panels["settings"].(*panels.SettingsPanel)
	if !ok || p == nil {
		return
	}
	settings, err := m.adapter.Settings(context.Background())
	if err == nil {
		p.SetSettings(&settings)
	}
	cfg, _ := config.Load()
	p.SetGlobalPredictionEnabled(config.NextMessagePredictionEnabled(&cfg))
}

func (m *AppModel) handleRulesSave(msg panels.RulesSaveMsg) {
	if m == nil || m.adapter == nil {
		return
	}
	scope := strings.ToLower(strings.TrimSpace(msg.Scope))
	if scope == "" {
		scope = "project"
	}
	if err := m.adapter.SaveRules(context.Background(), scope, msg.Content); err != nil {
		m.appendSystem(fmt.Sprintf("Rules.md 保存失败: %v", err), "error")
		return
	}

	if scope == "global" {
		m.appendSystem("已保存全局 Rules.md", "success")
	} else {
		m.appendSystem("已保存项目 Rules.md", "success")
	}
	m.refreshRulesPanel()
}

func (m *AppModel) handleMemorySave(msg panels.MemorySaveMsg) {
	if m == nil || m.adapter == nil {
		return
	}
	scope := strings.ToLower(strings.TrimSpace(msg.Scope))
	if scope == "" {
		scope = "project"
	}
	if err := m.adapter.SaveMemory(context.Background(), scope, msg.Content); err != nil {
		m.appendSystem(fmt.Sprintf("Memory 保存失败: %v", err), "error")
		return
	}
	m.appendSystem("已保存 "+scope+" memory", "success")
	m.refreshMemoryPanel()
}

func (m *AppModel) handleMemoryRebuildIndex() {
	if m == nil || m.adapter == nil {
		return
	}
	if err := m.adapter.RebuildMemoryIndex(context.Background()); err != nil {
		m.appendSystem(fmt.Sprintf("Memory 索引重建失败: %v", err), "error")
		return
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
			if err := m.adapter.AddMCPEntries(context.Background(), []config.MCPEntry{entry}); err != nil {
				m.appendSystem(fmt.Sprintf("新增（用于重命名）失败: %v", err), "error")
				return nil
			}
			if err := m.adapter.DeleteMCPServer(context.Background(), msg.OriginalName); err != nil {
				_ = m.adapter.DeleteMCPServer(context.Background(), entry.Name)
				m.appendSystem("删除旧名称失败: "+msg.OriginalName, "error")
				return nil
			}
		} else {
			if err := m.adapter.UpsertMCPEntry(context.Background(), entry); err != nil {
				m.appendSystem("更新失败: "+err.Error(), "error")
				return nil
			}
		}
	} else {
		if err := m.adapter.AddMCPEntries(context.Background(), entries); err != nil {
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
		ctx := context.Background()
		items, err := m.adapter.CostItems(ctx)
		if err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("刷新成本明细失败", "Failed to refresh cost items"), err), "error")
			return
		}
		total, err := m.adapter.UsageSummary(ctx)
		if err != nil {
			m.appendSystem(fmt.Sprintf("%s: %v", m.localize("刷新成本统计失败", "Failed to refresh usage summary"), err), "error")
			return
		}

		modelStats := aggregateCostItemsByModel(items)
		stats := make([]panels.CostStats, 0, len(modelStats))
		for _, s := range modelStats {
			stats = append(stats, panels.CostStats{
				Model:  s.Model,
				Rounds: s.Rounds,
				Input:  m.optionalIntLabel(s.Input),
				Reply:  m.optionalIntLabel(s.Reply),
				Total:  m.optionalIntLabel(s.Total),
			})
		}

		costPanel.SetStats(stats, panels.TotalStats{
			TotalRounds: total.Rounds,
			TotalInput:  m.optionalIntLabel(total.InputTokens),
			TotalReply:  m.optionalIntLabel(total.ReplyTokens),
			TotalTokens: m.optionalIntLabel(total.TotalTokens),
		})
	}
}

type costModelAggregate struct {
	Model  string
	Rounds int
	Input  *int
	Reply  *int
	Total  *int
}

func aggregateCostItemsByModel(items []coreapi.CostItem) []costModelAggregate {
	byModel := map[string]*costModelAggregate{}
	for _, item := range items {
		model := strings.TrimSpace(item.Model)
		if model == "" {
			model = "unknown"
		}
		agg := byModel[model]
		if agg == nil {
			agg = &costModelAggregate{Model: model}
			byModel[model] = agg
		}
		agg.Rounds++
		agg.Input = addOptionalInt(agg.Input, item.InputTokens)
		agg.Reply = addOptionalInt(agg.Reply, item.ReplyTokens)
		agg.Total = addOptionalInt(agg.Total, item.TotalTokens)
	}
	out := make([]costModelAggregate, 0, len(byModel))
	for _, item := range byModel {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Model) < strings.ToLower(out[j].Model)
	})
	return out
}

func addOptionalInt(total *int, value *int) *int {
	if value == nil {
		return total
	}
	next := *value
	if total != nil {
		next += *total
	}
	return &next
}

func (m *AppModel) optionalIntLabel(value *int) string {
	if value == nil {
		return m.localize("未知", "unknown")
	}
	return fmt.Sprintf("%d", *value)
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

	if err := m.adapter.SaveSettings(context.Background(), *settings); err != nil {
		m.appendSystem(fmt.Sprintf("Failed to save workspace settings: %v", err), "error")
		return
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
	versions, err := m.adapter.Versions(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("Failed to load versions: %v", err), "error")
		return
	}
	items := make([]panels.VersionItem, 0)
	for _, v := range versions {
		if !versionFileMatches(v.File, pathRel) {
			continue
		}
		items = append(items, panels.VersionItem{
			Timestamp: v.ID,
			Size:      versionSummarySize(v.Summary),
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
	if err := m.adapter.RollbackVersion(context.Background(), versionID); err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("版本回滚失败", "Version rollback failed"), err), "error")
		return
	}
	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已回滚版本", "Rolled back version"), versionID), "warning")
	m.handleVersionsLoad(pathRel)
}

func (m *AppModel) handleVersionsDelete(pathRel string, versionID string) {
	pathRel = strings.TrimSpace(pathRel)
	versionID = strings.TrimSpace(versionID)
	if pathRel == "" || versionID == "" {
		return
	}
	if err := m.adapter.DeleteVersion(context.Background(), versionID); err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("删除版本失败", "Version delete failed"), err), "error")
		return
	}
	m.appendSystem(fmt.Sprintf("%s: %s", m.localize("已删除版本", "Deleted version"), versionID), "warning")
	m.handleVersionsLoad(pathRel)
}

func (m *AppModel) handleVersionsDeleteFile(pathRel string) {
	pathRel = strings.TrimSpace(pathRel)
	if pathRel == "" {
		return
	}
	count, err := m.adapter.DeleteFileVersions(context.Background(), pathRel)
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("删除文件版本失败", "Failed to delete file versions"), err), "error")
		return
	}
	m.appendSystem(fmt.Sprintf("%s: %s (%d)", m.localize("已删除文件版本", "Deleted file versions"), pathRel, count), "warning")
	m.refreshVersionsPanel()
}

func (m *AppModel) handleVersionsDeleteAll() {
	count, err := m.adapter.ClearVersions(context.Background())
	if err != nil {
		m.appendSystem(fmt.Sprintf("%s: %v", m.localize("清空版本历史失败", "Failed to clear version history"), err), "error")
		return
	}
	m.appendSystem(fmt.Sprintf("%s: %d", m.localize("已清空版本历史", "Cleared version history"), count), "warning")
	m.refreshVersionsPanel()
}

func versionFileMatches(file, target string) bool {
	file = normalizeVersionPath(file)
	target = normalizeVersionPath(target)
	if file == "" || target == "" {
		return false
	}
	if strings.EqualFold(file, target) {
		return true
	}
	if isAbsVersionPath(file) && !isAbsVersionPath(target) {
		return strings.HasSuffix(strings.ToLower(file), "/"+strings.ToLower(target))
	}
	if isAbsVersionPath(target) && !isAbsVersionPath(file) {
		return strings.HasSuffix(strings.ToLower(target), "/"+strings.ToLower(file))
	}
	return false
}

func normalizeVersionPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if path == "." {
		return ""
	}
	path = strings.TrimPrefix(path, "./")
	return path
}

func isAbsVersionPath(path string) bool {
	return filepath.IsAbs(filepath.FromSlash(path))
}

func versionSummarySize(summary string) int {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return 0
	}
	match := regexp.MustCompile(`(?:^|[,\s])size=(\d+)`).FindStringSubmatch(summary)
	if len(match) != 2 {
		return 0
	}
	var size int
	_, _ = fmt.Sscanf(match[1], "%d", &size)
	return size
}

func (m *AppModel) handleHiddenLegalSlash() tea.Cmd {
	m.appendSystem("Copyright (c) 2026 DreamSailing", "info")
	m.appendSystem("License: EOS Non-Commercial License v1.1 (EOS-NCL-1.1)", "info")
	m.appendSystem("SPDX-License-Identifier: EOS-NCL-1.1", "info")
	m.appendSystem("Contact: smart-os@qq.com", "info")
	m.appendSystem(fmt.Sprintf("Version: %s | Commit: %s | Build: %s", version.AppVersion, version.BuildCommit, version.BuildDate), "info")
	return nil
}
