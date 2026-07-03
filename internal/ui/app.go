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
	"github.com/charmbracelet/lipgloss"
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
	actionPopup              *confirm.ActionPopup // 点击消息文本弹出的操作选择框
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
	activeItemID       string // current AgentMessage item being streamed
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

// bubbleActionHit 记录一条可点击消息文本在内容区中的行范围，
// 用于点击 AI/子 Agent 回复文本时弹出操作选择框（复制/下载）。
type bubbleActionHit struct {
	y       int      // 消息起始行号
	lines   int      // 消息占用的行数（含空行）
	idx     int      // 对应历史记录条目的索引
	actions []string // 该条目可用动作（如 "copy"、"download"）
	text    string   // 待复制/下载的文本内容
}

// hasAction 报告该命中区是否提供指定动作。
func (r bubbleActionHit) hasAction(action string) bool {
	for _, a := range r.actions {
		if strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(action)) {
			return true
		}
	}
	return false
}

// planDownloadRequest 记录待下载的计划文件信息
type planDownloadRequest struct {
	HistoryIndex int // 历史记录中该计划的索引
}

var choosePlanDownloadDirectory = filedialog.ChooseDirectory
var writePlanDownloadFile = os.WriteFile
var planDownloadNow = time.Now

// ctxUsageTickMsg 上下文使用率定时刷新消息
type ctxUsageTickMsg struct{}

// predictionDebounceMsg 预测防抖消息，用于延迟触发下一条消息预测
type predictionDebounceMsg struct {
	Seq   int    // 序列号，用于丢弃过期的防抖消息
	Draft string // 当时的输入草稿内容
}

// ctxUsageTick 每 900ms 触发一次上下文使用率刷新
func (m *AppModel) ctxUsageTick() tea.Cmd {
	return tea.Tick(900*time.Millisecond, func(time.Time) tea.Msg { return ctxUsageTickMsg{} })
}

// updateContextUsageUI 从适配器获取当前上下文使用情况并更新 shell 显示
func (m *AppModel) updateContextUsageUI() {
	if m == nil || m.shell == nil || m.adapter == nil {
		return
	}
	// 有历史记录或正在处理时才显示上下文信息
	if len(m.history) > 0 || m.state.Processing {
		m.shell.SetContextVisible(true)
	}
	tokens, ratio, err := m.adapter.CurrentContextUsage(context.Background())
	if err != nil {
		return
	}
	m.shell.SetContextUsage(tokens, ratio)
}

// updateBGTaskCountUI 刷新后台任务数量显示
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

// clearPrediction 清除预测文本，递增序列号使旧预测失效
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

// syncPredictionState 同步 shell 的预测状态到本地缓存
func (m *AppModel) syncPredictionState() {
	if m == nil || m.shell == nil {
		return
	}
	if !m.shell.HasPrediction() {
		m.predictionText = ""
	}
}

// canPredict 判断当前是否可以触发下一条消息预测
// 需要满足：模型适配器就绪、shell 就绪、预测功能开启、
// 处于 shell 视图、AI 模式、且不在处理中
func (m *AppModel) canPredict() bool {
	return m != nil &&
		m.adapter != nil &&
		m.shell != nil &&
		m.predictionEnabled &&
		m.activeView == "shell" &&
		m.state.Mode == "ai" &&
		!m.state.Processing
}

// schedulePrediction 调度一次预测请求，300ms 防抖后触发
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

// requestPrediction 向适配器请求下一条用户消息的预测建议
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

// refreshShellWelcomeInfo 从适配器获取模型信息并更新欢迎卡片显示
func (m *AppModel) refreshShellWelcomeInfo() {
	if m == nil || m.shell == nil || m.adapter == nil {
		return
	}
	modelName, modelBase := resolveShellWelcomeInfo(m.adapter)
	m.shell.SetWelcomeInfo(modelName, modelBase, "")
}

// updateInlinePermissionUI 更新内联权限确认框的显示
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

// showInlinePermission 显示内联权限确认框，用于 AI 请求执行敏感操作时的用户确认
func (m *AppModel) showInlinePermission(req confirm.Request) {
	reqCopy := req
	m.inlinePermissionReq = &reqCopy
	m.inlinePermissionSelected = 0
	m.updateInlinePermissionUI()
	if m.shell != nil {
		m.shell.BlurInput()
	}
}

// clearInlinePermission 清除内联权限确认框状态
func (m *AppModel) clearInlinePermission() {
	m.inlinePermissionReq = nil
	m.inlinePermissionSelected = 0
	if m.shell != nil {
		m.shell.ClearPromptOverlay()
	}
}

// refreshAILive 刷新 AI 实时响应区域，包括思考过程和流式回复
func (m *AppModel) refreshAILive() {
	if m == nil || m.shell == nil {
		return
	}
	// 非处理状态时清除实时显示
	if !m.state.Processing {
		m.shell.ClearLive()
		m.shell.SetStatusHints(false, false)
		return
	}
	var blocks []string
	thinking := strings.TrimSpace(m.thinkingLive.String())
	thinkingShown := m.state.Thinking && state.Thinking() && thinking != ""
	// 渲染思考过程块
	if thinkingShown {
		if m.msgRenderer != nil {
			hint := i18n.T("status.hint.thinking_expand", m.state.Language)
			blocks = append(blocks, m.msgRenderer.RenderThinkingWithHint(thinking, time.Since(m.currentAIStartTime), m.thinkingExpanded, nil, hint))
		} else {
			blocks = append(blocks, thinking)
		}
	}
	// 渲染流式回复块
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

// clearCurrentThinking 清除当前思考状态，包括内容缓冲区和展开状态
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

// ansiRe 匹配 ANSI 转义序列，用于清理文本中的终端样式代码
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// clearCopiedMsg 清除复制成功提示的消息
type clearCopiedMsg struct {
	idx int // 要清除的历史记录索引
}

// toolTrack 跟踪正在执行的工具调用状态
type toolTrack struct {
	name    string         // 工具名称
	started time.Time      // 开始执行时间
	idx     int            // 在历史记录中的索引
	params  map[string]any // 工具调用参数
}

// historyEntry 历史记录条目，支持多种类型：用户输入、AI 回复、工具调用、系统消息等
type historyEntry struct {
	// 通用字段
	kind          string        // 条目类型："user", "ai", "tool", "system", "agent.task", "agent.final"
	content       string        // 条目内容
	timestamp     time.Time     // 时间戳
	tokens        int           // token 数量（用于 AI 回复）
	duration      time.Duration // 执行耗时
	level         string        // 消息级别（用于系统消息）："error", "warning", "success", "info"
	executionMode string        // 执行模式（用于 AI 回复）："auto", "plan" 等
	rawMarkdown   string        // 原始 markdown 内容（用于计划下载）
	copiedAt      time.Time     // 复制时间（用于显示复制成功提示）

	// 工具调用相关字段
	toolID      string         // 工具调用唯一 ID
	toolName    string         // 工具名称
	toolParams  map[string]any // 工具调用参数
	toolOutput  string         // 工具执行输出
	toolSuccess bool           // 工具是否执行成功
	toolStatus  string         // 工具执行状态："running", "success", "error", "canceled"

	// Agent 相关字段
	agentName     string // Agent 名称
	agentID       string // Agent ID
	agentEvent    string // Agent 事件类型
	sourceAgent   string // 来源 Agent 名称
	sourceAgentID string // 来源 Agent ID
	task          string // Agent 任务描述
}

// resolveShellWelcomeInfo 从适配器获取模型信息，用于欢迎卡片显示
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

// hydrateCatalogFromAdapter 从适配器加载模型目录并应用到全局状态
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

// newAppModel 初始化应用模型，创建所有视图和面板
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

	// 启动事件监听（阻塞式：parked goroutine，事件到达即返回并重武装）
	// 之前用非阻塞 select + default 返回 nil，nil 消息不会重武装泵，
	// 导致 Init 后泵永久死亡、流式 item.delta 事件无人消费，UI 只能等
	// 阻塞 Invoke() 返回最终文本时一次性显示。改为阻塞 listenEvents()。
	return tea.Batch(
		m.shell.Init(),
		m.ctxUsageTick(),
		m.listenEvents(),
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
		if m.actionPopup != nil {
			m.actionPopup.SetSize(msg.Width, msg.Height)
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

		// 操作弹框拦截按键（覆盖在 shell 之上，不切换 activeView）
		if m.actionPopup != nil {
			updated, cmd := m.actionPopup.Update(msg)
			m.actionPopup = updated
			if cmd != nil {
				return m, cmd
			}
			return m, nil
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
	case ItemStartedMsg:
		// A new AgentMessage text segment begins. Create a fresh history entry
		// and reset the live buffer so each round renders as its own paragraph.
		if msg.ItemType == "agent_message" || msg.ItemType == "" {
			m.startAgentMessageItem(msg.ItemID)
		}
		return m, nil
	case ItemDeltaMsg:
		if !m.state.Processing {
			return m, nil
		}
		m.handleItemDelta(msg)
		// Early return: per-token deltas must NOT fall through to the default path,
		// which calls updateContextUsageUI + updateBGTaskCountUI (2 synchronous
		// JSON-RPC round-trips). With hundreds of deltas per turn that blocks the
		// Update loop and freezes streaming until all RPCs complete.
		return m, nil
	case ItemCompletedMsg:
		m.handleItemCompleted(msg)
	case InvokeDoneMsg:
		if !m.state.Processing {
			return m, nil
		}
		// turn.completed: archive any remaining live text, then finalize.
		// Under the item model, most segments were already archived via
		// item_completed; this is a safety net for any trailing buffer.
		m.archiveAgentMessage()
		m.aiLive.Reset()
		m.activeItemID = ""
		m.shell.ClearLive()
		m.clearCurrentThinking()
		m.shell.SetStatusHints(false, false)
		m.state.Processing = false
		m.shell.SetProcessing(false)
		m.activeCancel = nil
		m.stopRequested = false
		_ = msg.Content
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
		return m, tea.Batch(cmds...)

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
				req.Options = []string{"accept", "acceptForSession", "decline", "cancel"}
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
				if err := m.adapter.RespondPrompt(context.Background(), msg.ID, msg.Kind, adapter.PromptResponse{
					Decision:    msg.Decision,
					Option:      msg.Option,
					OptionIndex: msg.OptionIndex,
					Text:        msg.Text,
				}); err != nil {
					m.appendSystem(fmt.Sprintf("审批响应失败: %v", err), "error")
				}
			}
			// 审批响应后 turn 恢复执行（工具重跑或继续生成），
			// 保持“处理中”指示器直到 turn.completed，避免审批后 spinner 一闪没。
			m.state.Processing = true
			m.shell.SetProcessing(true)
			if m.activeView == "shell" {
				m.shell.FocusInput()
			}
			// Re-arm the status animation tick: the prior turn's tick loop may
			// have stopped while waiting for approval, so without this the
			// spinner stays frozen even though processing resumed. Mirrors
			// codex's "busy ⇒ keep animating" self-scheduling spinner.
			return m, m.shell.StatusTick()
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
			if err := m.adapter.RespondPrompt(context.Background(), msg.ID, msg.Kind, adapter.PromptResponse{
				Decision:    msg.Decision,
				Option:      msg.Option,
				OptionIndex: msg.OptionIndex,
				Text:        msg.Text,
			}); err != nil {
				m.appendSystem(fmt.Sprintf("审批响应失败: %v", err), "error")
			}
		}
		// 审批响应后 turn 恢复执行，保持“处理中”直到 turn.completed。
		m.state.Processing = true
		m.shell.SetProcessing(true)
		m.confirmView = nil
		if m.prevView != "" {
			m.activeView = m.prevView
			m.prevView = ""
		} else {
			m.activeView = "shell"
		}
		m.shell.FocusInput()
		// Re-arm the status animation tick after approval (see comment above).
		return m, m.shell.StatusTick()

	case clearCopiedMsg:
		if msg.idx >= 0 && msg.idx < len(m.history) {
			m.history[msg.idx].copiedAt = time.Time{}
			m.rebuildHistoryContent()
		}

	case confirm.ActionResultMsg:
		return m, m.handleActionResult(msg)

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
	handler, ok := slashCommandHandler(m)[cmd]
	if !ok {
		// 别名查找：commands.go 的 Aliases 字段
		for _, c := range slash.Commands {
			for _, alias := range c.Aliases {
				if alias == cmd && c.Name != cmd {
					handler, ok = slashCommandHandler(m)[c.Name]
					break
				}
			}
			if ok {
				break
			}
		}
	}
	if ok {
		return handler(args)
	}
	m.appendSystem(fmt.Sprintf("Unknown command: %s", cmd), "warning")
	return nil
}

// slashCommandHandler 构建命令名→处理函数的映射表。
// 新增命令只需在此 map 加一行，不再需要改 switch。
func slashCommandHandler(m *AppModel) map[string]func(args []string) tea.Cmd {
	return map[string]func(args []string) tea.Cmd{
		"/help": func(_ []string) tea.Cmd {
			m.clearPrediction()
			m.activeView = "help"
			if m.helpView != nil {
				m.helpView.ResetScroll()
			}
			return nil
		},
		"/clear": func(_ []string) tea.Cmd {
			m.shell.ClearContent()
			m.shell.ClearInput()
			m.shell.ClearLive()
			m.history = m.history[:0]
			m.actionHits = nil
			return nil
		},
		"/exit":            func(_ []string) tea.Cmd { return tea.Quit },
		"/init":            func(_ []string) tea.Cmd { m.shell.ClearInput(); return m.initEOSMD() },
		"/init-verifiers":  m.handleInitVerifiersSlash,
		"/history": func(_ []string) tea.Cmd {
			m.clearPrediction()
			m.activeView = "panel"
			m.activePanel = "versions"
			m.shell.ClearInput()
			m.refreshVersionsPanel()
			return nil
		},
		"/model":           m.handleModelSlash,
		"/mcp":             func(_ []string) tea.Cmd { m.clearPrediction(); m.activeView = "panel"; m.activePanel = "mcp"; m.shell.ClearInput(); m.refreshMCPPanel(); return nil },
		"/context":         func(_ []string) tea.Cmd { m.openContextPanel(); return nil },
		"/memory":          func(_ []string) tea.Cmd { m.openMemoryPanel(); return nil },
		"/cost":            func(_ []string) tea.Cmd { m.clearPrediction(); m.activeView = "panel"; m.activePanel = "cost"; m.shell.ClearInput(); m.refreshCostPanel(); return nil },
		"/tasks": func(_ []string) tea.Cmd {
			m.clearPrediction()
			m.activeView = "panel"
			m.activePanel = "tasks"
			m.shell.ClearInput()
			if panel, ok := m.panels["tasks"].(*panels.TasksPanel); ok && panel != nil {
				panel.ResetView()
				return panel.Init()
			}
			return nil
		},
		"/workspace":      m.handleWorkspaceSlash,
		"/config":          func(_ []string) tea.Cmd { m.openSettingsPanel(); return nil },
		"/lsp":             func(_ []string) tea.Cmd { m.clearPrediction(); m.activeView = "panel"; m.activePanel = "lsp"; m.shell.ClearInput(); m.refreshLSPPanel(); return nil },
		"/rules":           func(_ []string) tea.Cmd { m.clearPrediction(); m.activeView = "panel"; m.activePanel = "rules"; m.shell.ClearInput(); m.refreshRulesPanel(); return nil },
		"/lang": func(args []string) tea.Cmd {
			if len(args) > 0 {
				m.state.Language = args[0]
				if cfg, path := config.Load(); path != "" {
					cfg.Language = args[0]
					if err := config.Save(cfg, path); err != nil {
						m.appendSystem(fmt.Sprintf("Failed to save language config: %v", err), "error")
					}
				}
				m.appendSystem(fmt.Sprintf("Language changed to: %s", args[0]), "success")
				return func() tea.Msg { return panels.LanguageChangeMsg{Language: args[0]} }
			}
			return nil
		},
		"/compact": func(_ []string) tea.Cmd {
			if message, err := m.adapter.CompactContext(context.Background()); err != nil {
				m.appendSystem(fmt.Sprintf("%s: %v", m.localize("上下文压缩失败", "Context compact failed"), err), "error")
			} else if strings.TrimSpace(message) != "" {
				m.appendSystem(message, "success")
			} else {
				m.appendSystem(i18n.T("context.compacted", m.state.Language), "success")
			}
			m.refreshContextPanel()
			return nil
		},
		"/session":        m.handleSessionSlash,
		"/resume":          m.handleResumeSlash,
		"/permissions":     m.handlePermissionsSlash,
		"/skills":          m.handleSkillsSlash,
		"/plugin":          func(_ []string) tea.Cmd { return m.handlePluginSlash() },
		"/reload-plugins":  func(_ []string) tea.Cmd { return m.handleReloadPluginsSlash() },
		"/doctor":          func(_ []string) tea.Cmd { return m.handleDoctorSlash() },
		"/diff":            m.handleDiffSlash,
		"/review":          m.handleReviewSlash,
		"/verify":          m.handleVerifySlash,
		"/plan":            m.handlePlanSlash,
		"/plan-style":      m.handlePlanStyleSlash,
		"/git":             m.handleGitSlash,
		"/remote":          m.handleRemoteSlash,
		"/status":          func(_ []string) tea.Cmd { return m.handleStatusSlash() },
		"/fast":            func(_ []string) tea.Cmd { return m.handleFastSlash() },
		"/export":          m.handleExportSlash,
		"/theme":           m.handleThemeSlash,
		"/stats":           func(_ []string) tea.Cmd { return m.handleStatsSlash() },
		"/rename":          m.handleRenameSlash,
		"/share":           func(_ []string) tea.Cmd { return m.handleShareSlash() },
		"/_legal":          func(_ []string) tea.Cmd { return m.handleHiddenLegalSlash() },
	}
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

- UI: internal/ui/ (TUI 基于 Bubble Tea)
- Engine: pkg/coreapi/sidecar/ (通过 JSON-RPC 调用 Rust 内核 eos-core)
- CLI: internal/cli/ (cobra 子命令)
- Config: internal/config/

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
	// Option keys are canonical decision values; the selected option IS the
	// decision. Esc passes "decline" explicitly.
	if decision == "" {
		decision = option
		if decision == "" {
			decision = "decline"
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
		result := m.buildInlinePermissionResult("decline")
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

// sendMessage 发送用户消息到 AI 引擎
// 1. 展开特殊宏（如 #problems_and_diagnostics）
// 2. 更新 UI 状态为处理中
// 3. 处理图片附件
// 4. 异步调用适配器执行 AI 请求
func (m *AppModel) sendMessage() tea.Cmd {
	value := m.shell.GetInputValue()
	expanded := value
	// 展开 LSP 诊断宏
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

	// 记录 AI 开始时间和 token 计数
	m.currentAIStartTime = time.Now()
	m.currentAITokens = 0
	m.setActiveCancel(func() {
		m.adapter.CancelForegroundRequest()
	})

	// 处理图片附件
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

	// 显示用户消息
	display := value
	if strings.TrimSpace(display) == "" && len(imagePaths) > 0 {
		display = i18n.T("chat.image_only", m.state.Language)
	}
	m.appendHistory(historyEntry{kind: "user", content: display, timestamp: time.Now()})

	// 异步调用 AI 引擎
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

// sendBashCommand 执行 Bash 命令
// 带有 30 秒超时，执行结果通过 ToolResultMsg 返回
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

	exec := func() tea.Msg {
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
	// Keep the status-bar spinner animating for the whole bash run.
	return tea.Batch(exec, m.shell.StatusTick())
}

// refreshContextPanel 刷新上下文面板的数据
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

// parseContextPreviewLine 解析上下文预览行，格式为 "role: content"
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

// estimateDisplayTokens 估算内容的 token 数量（约 4 个字符一个 token）
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

// rulesSnapshotDocument 从规则快照中获取指定作用域的文档
func rulesSnapshotDocument(snapshot coreapi.RulesSnapshot, scope string) coreapi.RuleDocument {
	scope = strings.ToLower(strings.TrimSpace(scope))
	for _, doc := range snapshot.Documents {
		if strings.EqualFold(strings.TrimSpace(doc.Scope), scope) {
			return doc
		}
	}
	return coreapi.RuleDocument{Scope: scope}
}

// memorySnapshotDocument 从记忆快照中获取指定作用域的文档
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

// refreshMemoryPanel 刷新记忆面板的数据，包括全局、项目、会话和索引四个作用域
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

// overlayCenter 将弹框内容居中叠加到底层视图之上。
// 采用 lipgloss.Place 按尺寸居中，背景仍透出底层 shell 文本流。
func overlayCenter(width, height int, background, overlay string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("")),
	)
}

// handleRuntimeEvent 处理 runtime event 消息
func (m *AppModel) handleRuntimeEvent(e adapter.RuntimeEvent) (tea.Model, tea.Cmd) {
	uiMsg := ConvertEvent(e)
	if uiMsg != nil {
		_, cmd := m.Update(uiMsg)
		// Always re-arm the event listener so the pump keeps draining events
		// even when the inner Update returned a nil cmd (e.g. ItemDeltaMsg).
		return m, tea.Batch(cmd, m.listenEvents())
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
		view := m.shell.View()
		if m.actionPopup != nil {
			return overlayCenter(m.width, m.height, view, m.actionPopup.View())
		}
		return view
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

// cancelProcessingUI 取消处理状态，清除所有进行中的 UI 状态
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

// setActiveCancel 设置当前活跃的取消函数
func (m *AppModel) setActiveCancel(cancel context.CancelFunc) {
	m.activeCancel = cancel
	m.stopRequested = false
}

// cancelActiveRequest 取消当前活跃的请求，返回是否成功取消
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

// markInflightToolsCanceled 将所有进行中的工具标记为已取消
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

func (m *AppModel) bubbleActionsForEntry(e historyEntry) []messages.BubbleAction {
	if (e.kind != "ai" && e.kind != "agent.final") || strings.TrimSpace(e.content) == "" {
		return nil
	}
	// 文本流布局下 Label 不再用于内联按钮；弹框展示文案由 actionLabel(kind) 解析。
	actions := []messages.BubbleAction{{Kind: "copy"}}
	if strings.EqualFold(strings.TrimSpace(e.executionMode), "plan") && strings.TrimSpace(e.rawMarkdown) != "" {
		actions = append(actions, messages.BubbleAction{Kind: "download"})
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

// stripANSI 清除字符串中的 ANSI 转义序列
func stripANSI(s string) string {
	if strings.IndexByte(s, 0x1b) < 0 {
		return s
	}
	return ansiRe.ReplaceAllString(s, "")
}

// runeIndex 将字节索引转换为 rune 索引
func runeIndex(s string, byteIdx int) int {
	if byteIdx <= 0 {
		return 0
	}
	if byteIdx >= len(s) {
		return len([]rune(s))
	}
	return len([]rune(s[:byteIdx]))
}

// trackBubbleActionsAt 登记一条可点击消息文本在内容区中的行范围。
// 文本流布局下不再有内联按钮，改为把整条 AI/子 Agent 回复文本登记为
// 可点击区，点击时弹出操作选择框（复制/下载）。
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
	kinds := make([]string, 0, len(actions))
	for _, a := range actions {
		if k := strings.TrimSpace(a.Kind); k != "" {
			kinds = append(kinds, k)
		}
	}
	if len(kinds) == 0 {
		return
	}
	lineCount := strings.Count(rendered, "\n") + 1
	m.actionHits = append(m.actionHits, bubbleActionHit{
		y:       startLine,
		lines:   lineCount,
		idx:     idx,
		actions: kinds,
		text:    payload,
	})
}

// tryHandleBubbleActionAt 尝试处理指定坐标处的消息点击。
// 命中可点击消息文本时弹出操作选择框；未命中返回 nil。
func (m *AppModel) tryHandleBubbleActionAt(x, y int) tea.Cmd {
	if m.actionPopup != nil {
		return nil
	}
	ox, oy := m.shell.ContentOrigin()
	if x < ox || y < oy {
		return nil
	}
	ly := y - oy
	if ly < 0 || ly >= m.shell.ContentHeight() {
		return nil
	}
	line := m.shell.ContentYOffset() + ly
	for _, h := range m.actionHits {
		if line < h.y || line >= h.y+h.lines {
			continue
		}
		m.openActionPopup(h)
		return nil
	}
	return nil
}

// openActionPopup 根据命中区构造操作选择弹框。
func (m *AppModel) openActionPopup(h bubbleActionHit) {
	items := make([]confirm.ActionItem, 0, len(h.actions))
	for _, kind := range h.actions {
		items = append(items, confirm.ActionItem{
			Kind:  kind,
			Label: m.actionLabel(kind),
		})
	}
	m.actionPopup = confirm.NewActionPopup(m.styles, m.state.Language, confirm.ActionRequest{
		Actions: items,
		Payload: h.text,
		Index:   h.idx,
	})
	m.actionPopup.SetSize(m.width, m.height)
	m.shell.BlurInput()
}

// actionLabel 返回动作在弹框中的展示文案。
func (m *AppModel) actionLabel(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "copy":
		return i18n.T("op.copy", m.state.Language)
	case "download":
		return i18n.T("op.download", m.state.Language)
	default:
		return kind
	}
}

// handleActionResult 处理操作弹框的结果：执行复制/下载，并关闭弹框。
func (m *AppModel) handleActionResult(msg confirm.ActionResultMsg) tea.Cmd {
	idx := msg.Index
	closePopup := func() {
		m.actionPopup = nil
		if m.activeView == "shell" {
			m.shell.FocusInput()
		}
	}

	switch strings.TrimSpace(strings.ToLower(msg.Kind)) {
	case "cancel":
		closePopup()
		return nil
	case "copy":
		closePopup()
		if err := clipboard.WriteAll(msg.Payload); err != nil {
			m.appendSystem(i18n.T("tool.error.copy_error", m.state.Language, err), "error")
			return func() tea.Msg { return nil }
		}
		if idx >= 0 && idx < len(m.history) {
			m.history[idx].copiedAt = time.Now()
		}
		m.rebuildHistoryContent()
		m.appendSystem(i18n.T("clipboard.copied", m.state.Language), "success")
		return tea.Tick(1600*time.Millisecond, func(time.Time) tea.Msg { return clearCopiedMsg{idx: idx} })
	case "download":
		closePopup()
		return m.handlePlanDownloadAction(idx)
	default:
		closePopup()
		return nil
	}
}

// handlePlanDownloadAction 处理计划文件下载操作
// 尝试打开目录选择器，如果不可用则回退到文本输入确认框
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
		// 目录选择器不可用，回退到文本输入
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

// planDownloadEntry 获取指定索引的计划下载条目
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

// savePlanHistoryEntryToDir 将计划文件保存到指定目录
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

// nextPlanDownloadFileName 生成计划下载文件名，格式：plan-{sessionID}-{timestamp}.md
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

// sanitizePlanFileNameSegment 清理文件名中的非法字符
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

// uniqueAvailablePath 确保文件路径唯一，如果已存在则添加数字后缀
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

// pasteClipboardImage 粘贴剪贴板中的图片到附件列表
// 仅在 AI 模式的 shell 视图中有效
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
	// 保存图片到 .eos/attachments 目录
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
	// 检查模型是否支持视觉
	modelName, _ := m.adapter.GetModelInfo()
	if strings.TrimSpace(modelName) != "" && !ai.SupportsVisionFromCatalog(modelName) {
		m.appendSystem("当前模型可能不具备视觉能力，图片可能无法解析", "warning")
	}
	return func() tea.Msg { return nil }
}

// toggleThinkingExpand 切换思考过程的展开/折叠状态
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

// handleGlobalKey 处理全局快捷键，这些快捷键在所有视图中都有效
func (m *AppModel) handleGlobalKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		// Ctrl+C：如果正在处理则取消，否则退出应用
		if m.state.Processing {
			m.cancelProcessingUI()
			return nil
		}
		return tea.Quit

	case "f2":
		// F2：切换 AI/Bash 模式
		m.shell.ToggleMode()
		if m.shell.GetMode() == shell.ModeAI {
			m.state.Mode = "ai"
		} else {
			m.state.Mode = "bash"
		}
		return nil

	case "?":
		// ?：打开帮助面板
		m.clearPrediction()
		m.activeView = "help"
		if m.helpView != nil {
			m.helpView.ResetScroll()
			m.helpView.SetSize(m.width, m.height)
		}
		return nil

	case "esc":
		// Esc：如果正在处理则取消当前请求，否则隐藏提示并清空输入
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
		// Tab：如果有预测则接受预测，否则切换思考状态
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
		// Ctrl+O：循环切换实时面板显示模式
		if m.activeView == "shell" && m.shell != nil && m.shell.GetMode() == shell.ModeAI {
			m.shell.CycleLivePanelMode()
			return func() tea.Msg { return nil }
		}
	case "alt+v":
		// Alt+V：粘贴剪贴板中的图片
		return m.pasteClipboardImage()
	case "alt+h":
		// Alt+H：切换思考过程展开/折叠
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

// handleAIResponse 处理 AI 响应消息，支持三种类型：
// - "delta": 流式回复片段，追加到实时显示区域
// - "final": 完整回复，保存到历史记录并结束处理状态
// - "error": 错误消息，作为系统消息记录
func (m *AppModel) handleAIResponse(msg AIResponseMsg) tea.Cmd {
	switch msg.Type {
	case "delta":
		// 流式回复：清除预测、清除思考过程、追加内容到实时显示
		m.clearPrediction()
		if strings.TrimSpace(msg.Content) != "" && strings.TrimSpace(m.thinkingLive.String()) != "" {
			m.clearCurrentThinking()
		}
		m.aiLive.WriteString(msg.Content)
		m.currentAITokens += len(msg.Content) / 4
		m.refreshAILive()
	case "final":
		// 完整回复：清除实时显示、保存到历史、恢复空闲状态、调度预测
		duration := time.Since(m.currentAIStartTime)
		m.shell.ClearLive()
		m.aiLive.Reset()
		m.clearCurrentThinking()
		m.shell.SetStatusHints(false, false)
		mainContent := strings.TrimSpace(msg.Content)
		agentContent := strings.TrimSpace(m.lastAgentFinal)
		// 避免重复记录：如果本轮有委派且内容与 agent final 相同则跳过
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
		// 错误：清除所有状态，记录错误消息
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

// startAgentMessageItem begins a new text-segment item. It archives the
// current aiLive buffer (if any) into a history entry, then resets the buffer
// so each round of multi-round output renders as its own paragraph. This is
// the codex-style interleaving: [text段1] → [tool] → [text段2].
func (m *AppModel) startAgentMessageItem(itemID string) {
	// Archive any accumulated live text from a previous segment.
	if m.aiLive.Len() > 0 {
		m.archiveAgentMessage()
	}
	m.activeItemID = itemID
	m.aiLive.Reset()
	m.currentAITokens = 0
}

// archiveAgentMessage saves the current aiLive content as a finalized history
// entry (an "ai" paragraph). Called when a segment ends (item_completed) or
// when a new segment/tool starts.
func (m *AppModel) archiveAgentMessage() {
	text := strings.TrimSpace(m.aiLive.String())
	if text == "" {
		return
	}
	duration := time.Since(m.currentAIStartTime)
	m.appendHistory(historyEntry{
		kind:          "ai",
		content:       m.aiLive.String(),
		rawMarkdown:   m.aiLive.String(),
		executionMode: m.state.ExecutionMode,
		timestamp:     time.Now(),
		tokens:        m.currentAITokens,
		duration:      duration,
	})
}

// handleItemDelta appends an incremental chunk to the current item's live
// buffer and refreshes the display.
func (m *AppModel) handleItemDelta(msg ItemDeltaMsg) {
	if msg.DeltaType == "text" || msg.DeltaType == "" {
		m.clearPrediction()
		m.aiLive.WriteString(msg.Delta)
		m.currentAITokens += len(msg.Delta) / 4
		m.refreshAILive()
	}
}

// handleItemCompleted finalizes an AgentMessage item: archive the live text
// into a history entry and clear the buffer for the next segment.
func (m *AppModel) handleItemCompleted(msg ItemCompletedMsg) {
	if msg.ItemType != "agent_message" && msg.ItemType != "" {
		return
	}
	// If the completed event carries full text, prefer it over the buffer.
	if strings.TrimSpace(msg.Text) != "" {
		m.aiLive.Reset()
		m.aiLive.WriteString(msg.Text)
	}
	m.archiveAgentMessage()
	m.aiLive.Reset()
	m.activeItemID = ""
	m.shell.ClearLive()
}

// handleToolCall 处理工具调用消息
// 工具调用分两个阶段：
// 1. tool_call_start：创建工具卡片（可能无参数）
// 2. tool_call_done：补充真实参数
// 如果已有同 ID 的进行中卡片，只更新参数而非新建
func (m *AppModel) handleToolCall(msg ToolCallMsg) tea.Cmd {
	// A tool call starts: archive any in-progress text segment so the tool
	// card appears after it (codex-style [text]→[tool] interleaving).
	m.archiveAgentMessage()
	m.aiLive.Reset()
	m.activeItemID = ""
	m.shell.ClearLive()
	m.clearCurrentThinking()
	// 如果已有同 ID 的进行中卡片，更新参数
	if track, ok := m.toolInflight[msg.ID]; ok {
		if len(msg.Params) > 0 {
			track.params = msg.Params
			m.toolInflight[msg.ID] = track
			if track.idx >= 0 && track.idx < len(m.history) {
				e := m.history[track.idx]
				if len(e.toolParams) == 0 {
					e.toolParams = msg.Params
					m.history[track.idx] = e
					m.rebuildHistoryContent()
				}
			}
			if m.msgRenderer != nil {
				m.shell.SetLive(m.msgRenderer.RenderToolCall(track.name, msg.Params))
			}
		}
		return nil
	}
	// 新建工具卡片
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

// handleToolResult 处理工具执行结果
// 如果有对应的进行中卡片，更新其状态；否则创建新记录
// 对于 bash 工具，完成后会结束处理状态
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
	// 更新已存在的工具卡片
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
		// bash 工具完成后恢复空闲状态
		if strings.EqualFold(name, "bash") {
			m.state.Processing = false
			m.shell.SetProcessing(false)
			m.activeCancel = nil
			m.stopRequested = false
		}
		return nil
	}
	// 处理取消的 bash 工具
	if msg.Status == "canceled" {
		if strings.EqualFold(name, "bash") {
			m.activeCancel = nil
			m.stopRequested = false
		}
		return nil
	}
	// 创建新的工具记录（无对应卡片的情况）
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
		m.appendSystem("Failed to sync model from environment (EOS_API_BASE and EOS_API_KEY required)", "error")
		return
	}
	m.refreshModelsPanel()
	m.appendSystem("Synced model from environment variables", "success")
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

// handleMCPToggle 切换 MCP 服务器的启用/禁用状态
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
		// 重新加载 MCP 配置
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
	editor := setup.NewMCPConfigEditorView(m.styles, m.state.Language, recommendedBrowserPresetJSON(), false, "")
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

// handleMCPConfigSubmit 处理 MCP 配置提交
// 支持两种格式：旧版 JSON 标签格式和新版数组/对象格式
func (m *AppModel) handleMCPConfigSubmit(msg setup.MCPConfigSubmitMsg) tea.Cmd {
	raw := strings.TrimSpace(msg.Text)
	if raw == "" {
		m.appendSystem("请输入 MCP 配置 JSON", "warning")
		return nil
	}

	// 尝试多种格式解析 MCP 配置
	parseEntries := func(text string) ([]config.MCPEntry, error) {
		// 尝试旧版格式
		if entries, err := config.ParseLegacyMCPServersJSON([]byte(text)); err == nil && len(entries) > 0 {
			return entries, nil
		}
		// 尝试数组格式
		var arr []config.MCPEntry
		if err := json.Unmarshal([]byte(text), &arr); err == nil && len(arr) > 0 {
			return arr, nil
		}
		// 尝试单对象格式
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
		// 编辑模式：只支持单个 MCPEntry
		if len(entries) != 1 {
			m.appendSystem("编辑模式只支持一个 MCPEntry 对象", "warning")
			return nil
		}
		entry := entries[0]
		if strings.TrimSpace(entry.Name) == "" {
			m.appendSystem("缺少 name 字段", "warning")
			return nil
		}
		// 处理重命名：先添加新名称，再删除旧名称
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
			// 直接更新
			if err := m.adapter.UpsertMCPEntry(context.Background(), entry); err != nil {
				m.appendSystem("更新失败: "+err.Error(), "error")
				return nil
			}
		}
	} else {
		// 添加模式
		if err := m.adapter.AddMCPEntries(context.Background(), entries); err != nil {
			m.appendSystem(fmt.Sprintf("新增失败: %v", err), "error")
			return nil
		}
	}

	// 刷新 MCP 面板并重新加载配置
	m.activeView = "panel"
	m.activePanel = "mcp"
	m.shell.ClearInput()
	m.refreshMCPPanel()

	return func() tea.Msg {
		return MCPReloadDoneMsg{Err: m.adapter.Reload()}
	}
}

// refreshCostPanel 刷新成本统计面板
// 获取成本明细和使用汇总，按模型聚合后更新面板
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

		// 按模型聚合成本数据
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

// costModelAggregate 按模型聚合的成本统计
type costModelAggregate struct {
	Model  string // 模型名称
	Rounds int    // 调用轮次
	Input  *int   // 输入 token 数（可能为 nil）
	Reply  *int   // 回复 token 数（可能为 nil）
	Total  *int   // 总 token 数（可能为 nil）
}

// aggregateCostItemsByModel 按模型聚合成本统计数据
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

// addOptionalInt 累加两个可选整数
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

// optionalIntLabel 将可选整数转换为显示标签
func (m *AppModel) optionalIntLabel(value *int) string {
	if value == nil {
		return m.localize("未知", "unknown")
	}
	return fmt.Sprintf("%d", *value)
}

// handleSettingsSave 处理设置保存
// 1. 保存配置到本地文件
// 2. 保存工作区设置到核心
// 3. 处理语言切换
// 4. 更新预测功能状态
func (m *AppModel) handleSettingsSave(settings *settings.Settings, globalPredictionEnabled *bool) {
	if settings == nil {
		return
	}

	// 检查语言是否改变
	langChanged := settings.Language != m.state.Language

	// 保存配置到本地文件
	cfg, path := config.Load()
	if path != "" {
		cfg.Language = settings.Language
		if globalPredictionEnabled != nil {
			enabled := *globalPredictionEnabled
			cfg.NextMessagePredictionEnabled = &enabled
		}
		if err := config.Save(cfg, path); err != nil {
			m.appendSystem(fmt.Sprintf("Failed to save settings: %v", err), "error")
			return
		}
	}

	// 保存工作区设置到核心
	if err := m.adapter.SaveSettings(context.Background(), *settings); err != nil {
		m.appendSystem(fmt.Sprintf("Failed to save workspace settings: %v", err), "error")
		return
	}

	// 处理语言切换
	if langChanged {
		m.state.Language = settings.Language
		m.shell.SetLanguage(settings.Language)
		m.Update(panels.LanguageChangeMsg{Language: settings.Language})
	}
	// 更新预测功能状态
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

// versionFileMatches 检查版本文件路径是否匹配目标路径
// 支持绝对路径和相对路径的模糊匹配
func versionFileMatches(file, target string) bool {
	file = normalizeVersionPath(file)
	target = normalizeVersionPath(target)
	if file == "" || target == "" {
		return false
	}
	if strings.EqualFold(file, target) {
		return true
	}
	// 绝对路径与相对路径的后缀匹配
	if isAbsVersionPath(file) && !isAbsVersionPath(target) {
		return strings.HasSuffix(strings.ToLower(file), "/"+strings.ToLower(target))
	}
	if isAbsVersionPath(target) && !isAbsVersionPath(file) {
		return strings.HasSuffix(strings.ToLower(target), "/"+strings.ToLower(file))
	}
	return false
}

// normalizeVersionPath 标准化版本文件路径
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

// isAbsVersionPath 检查路径是否为绝对路径
func isAbsVersionPath(path string) bool {
	return filepath.IsAbs(filepath.FromSlash(path))
}

// versionSummarySize 从版本摘要中提取文件大小
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
