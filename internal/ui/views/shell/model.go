package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/state"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/ui/components/content"
	"github.com/dreamSailing/eos/internal/ui/components/hints"
	"github.com/dreamSailing/eos/internal/ui/components/input"
	"github.com/dreamSailing/eos/internal/ui/features/slash"
	"github.com/dreamSailing/eos/internal/ui/styles"
	"github.com/dreamSailing/eos/internal/update"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Mode 是Shell模式
type Mode string

const (
	ModeAI   Mode = "ai"
	ModeBash Mode = "bash"
)

// Model 是Shell主界面模型
type Model struct {
	width  int
	height int
	mode   Mode

	// 组件
	content content.Model
	input   input.Model
	hints   hints.Model
	welcome *WelcomeCard

	// 状态
	processing       bool
	thinking         bool
	thinkingText     string
	showWelcome      bool
	language         string
	live             string
	liveHeight       int
	livePanelMode    int
	hintLive         bool
	hintThinking     bool
	contentH         int
	statusAnim       int
	ctxTokens        int
	ctxRatio         float64
	ctxVisible       bool
	bgTaskCount      int
	executionMode    string
	thinkingExpanded bool

	// 样式
	styles *styles.Styles
}

type statusTickMsg struct{}

// New 创建新的Shell模型
func New(width, height int, s *styles.Styles, lang string) Model {
	// 计算各区域高度
	statusHeight := 1

	// 创建输入组件
	inputModel := input.New()
	inputModel.SetSize(width-2, 0)
	inputModel.SetStyle(s.Input, s.InputFocus)
	inputModel.SetPlaceholder("Enter message... (Press ? for help)")
	inputHeight := inputModel.ViewHeight()
	contentHeight := max(height-inputHeight-statusHeight-4, 10) // 减去边框和间距

	contentWidth := max(width-4, 1)

	// 创建内容组件
	contentModel := content.New(contentWidth, contentHeight)
	contentModel.SetStyle(s.Content)

	// 创建提示组件
	hintsModel := hints.New()
	hintsModel.SetSize(width-2, 3)
	hintsModel.SetStyle(s.Hints)

	welcome := NewWelcomeCard(s)
	welcome.SetSize(contentWidth+2, contentHeight)

	return Model{
		width:            width,
		height:           height,
		mode:             ModeAI,
		content:          contentModel,
		input:            inputModel,
		hints:            hintsModel,
		welcome:          welcome,
		showWelcome:      true,
		styles:           s,
		language:         lang,
		contentH:         contentHeight,
		executionMode:    "auto",
		thinkingExpanded: false,
	}
}

// SetSize 设置大小
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// 重新计算各区域高度
	statusHeight := 1
	hintsHeight := 0
	if m.hints.Visible() {
		hintsHeight = m.hints.Height()
	}
	m.input.SetSize(width-2, 0)
	inputHeight := m.input.ViewHeight()
	contentHeight := max(height-inputHeight-hintsHeight-statusHeight-4-m.livePanelOuterHeight(), 10)
	m.contentH = contentHeight

	hh := 3
	if m.hints.Visible() {
		hh = max(m.hints.Height(), 3)
	}
	m.hints.SetSize(width-2, hh)
	contentWidth := max(width-4, 1)
	if m.welcome != nil {
		m.welcome.SetSize(contentWidth+2, contentHeight)
	}
	m.relayout()
}

func (m *Model) SetLive(view string) {
	m.live = view
	if m.live == "" {
		m.liveHeight = 0
	} else {
		m.liveHeight = lipgloss.Height(m.live)
	}
	if strings.TrimSpace(m.live) == "" {
		m.hintLive = false
		m.hintThinking = false
	}
	m.relayout()
}

func (m *Model) ClearLive() {
	m.SetLive("")
}

func (m *Model) relayout() {
	atBottom := m.content.AtBottom()
	h := max(m.contentH, 5)
	contentWidth := max(m.width-4, 1)
	m.content.SetSize(contentWidth, h)
	if atBottom {
		m.content.GotoBottom()
	}
}

func (m *Model) recomputeLayout() {
	statusHeight := 1
	hintsHeight := 0
	if m.hints.Visible() {
		hintsHeight = m.hints.Height()
	}
	inputHeight := m.input.ViewHeight()
	contentHeight := max(m.height-inputHeight-hintsHeight-statusHeight-4-m.livePanelOuterHeight(), 10)
	m.contentH = contentHeight
	m.relayout()
}

func (m *Model) CycleLivePanelMode() {
	m.livePanelMode = (m.livePanelMode + 1) % 3
	m.recomputeLayout()
}

func (m Model) livePanelBodyHeight() int {
	switch m.livePanelMode {
	case 1:
		return 4
	case 2:
		h := min(max(m.height/3, 7), 16)
		return h
	default:
		return 0
	}
}

func (m Model) livePanelInnerHeight() int {
	body := m.livePanelBodyHeight()
	if body <= 0 {
		return 0
	}
	return body + 1
}

func (m Model) livePanelOuterHeight() int {
	inner := m.livePanelInnerHeight()
	if inner <= 0 {
		return 0
	}
	return inner + 2
}

func tailLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func padToHeight(s string, n int) string {
	if n <= 0 {
		return ""
	}
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return strings.Repeat("\n", n-1)
	}
	lines := strings.Split(s, "\n")
	if len(lines) >= n {
		return strings.Join(lines[:n], "\n")
	}
	return strings.Join(append(lines, make([]string, n-len(lines))...), "\n")
}

func (m Model) renderLivePanel() string {
	if m.livePanelMode == 0 {
		return ""
	}
	modeLabel := i18n.T("help.live.summary", m.language)
	if m.livePanelMode == 2 {
		modeLabel = i18n.T("help.live.detail", m.language)
	}
	title := m.styles.PanelTitle.Render("Live · " + modeLabel)
	bodyH := m.livePanelBodyHeight()

	body := m.live
	if m.livePanelMode == 1 {
		body = tailLines(body, bodyH)
	} else {
		body = tailLines(body, bodyH)
	}
	if strings.TrimSpace(body) == "" {
		body = m.styles.TextMuted.Render(i18n.T("live.empty", m.language))
	}
	body = padToHeight(body, bodyH)

	return m.styles.Panel.Render(lipgloss.JoinVertical(lipgloss.Left, title, body))
}

// SetMode 设置模式
func (m *Model) SetMode(mode Mode) {
	m.mode = mode
	if mode == ModeAI {
		m.input.SetPlaceholder("Enter message... (Press ? for help)")
	} else {
		m.input.ClearPrediction()
		m.input.SetPlaceholder("Enter bash command...")
	}
}

// GetMode 获取模式
func (m *Model) GetMode() Mode {
	return m.mode
}

// ToggleMode 切换模式
func (m *Model) ToggleMode() {
	if m.mode == ModeAI {
		m.SetMode(ModeBash)
	} else {
		m.SetMode(ModeAI)
	}
}

// SetProcessing 设置处理状态
func (m *Model) SetProcessing(v bool) {
	m.processing = v
}

// Processing 获取处理状态
func (m *Model) Processing() bool {
	return m.processing
}

// SetThinking 设置思考状态
func (m *Model) SetThinking(v bool, text string) {
	m.thinking = v
	m.thinkingText = text
}

func (m *Model) SetContextUsage(tokens int, ratio float64) {
	m.ctxTokens = tokens
	m.ctxRatio = ratio
}

func (m *Model) SetExecutionMode(mode string) {
	m.executionMode = toolapi.NormalizeExecutionMode(mode)
}

func (m *Model) SetThinkingExpanded(v bool) {
	m.thinkingExpanded = v
}

func (m *Model) SetContextVisible(v bool) {
	m.ctxVisible = v
}

func (m *Model) SetBGTaskCount(n int) {
	if n < 0 {
		n = 0
	}
	m.bgTaskCount = n
}

func (m *Model) SetStatusHints(liveHint bool, thinkingHint bool) {
	m.hintLive = liveHint
	m.hintThinking = thinkingHint
}

// SetWelcomeInfo 设置欢迎卡片信息
func (m *Model) SetWelcomeInfo(modelName, apiInfo, workDir string) {
	if m.welcome != nil {
		m.welcome.SetInfo(modelName, apiInfo, workDir)
	}
}

// SetUpdateInfo sets version update info on the welcome card
func (m *Model) SetUpdateInfo(info *update.CheckResult) {
	if m.welcome != nil {
		m.welcome.SetUpdateInfo(info)
	}
}

// AppendContent 追加内容
func (m *Model) AppendContent(text string) {
	m.content.Append(text)
	m.showWelcome = false // 有内容时隐藏欢迎面板
}

// AppendContentLine 追加内容行
func (m *Model) AppendContentLine(line string) {
	m.content.AppendLine(line)
	m.showWelcome = false // 有内容时隐藏欢迎面板
}

// ClearContent 清空内容
func (m *Model) ClearContent() {
	m.content.Clear()
}

func (m *Model) SetContent(text string) {
	m.content.SetContent(text)
	m.showWelcome = text == ""
}

func (m *Model) SetContentPreserveOffset(text string) {
	m.content.SetContentPreserveOffset(text)
	m.showWelcome = text == ""
}

// GetInputValue 获取输入值
func (m *Model) GetInputValue() string {
	return m.input.Value()
}

// SetInputValue 设置输入值
func (m *Model) SetInputValue(text string) {
	m.input.SetValue(text)
}

// ClearInput 清空输入
func (m *Model) ClearInput() {
	m.input.Clear()
}

func (m *Model) SetPrediction(text string) {
	m.input.SetPrediction(text)
}

func (m *Model) ClearPrediction() {
	m.input.ClearPrediction()
}

func (m *Model) HasPrediction() bool {
	return m.input.HasPrediction()
}

func (m *Model) CanAcceptPrediction() bool {
	return m.input.CanAcceptPrediction()
}

// AddToHistory 添加到历史
func (m *Model) AddToHistory(text string) {
	m.input.AddToHistory(text)
}

// FocusInput 聚焦输入
func (m *Model) FocusInput() {
	m.input.Focus()
}

// BlurInput 失焦输入
func (m *Model) BlurInput() {
	m.input.Blur()
}

// SetLanguage 设置语言
func (m *Model) SetLanguage(lang string) {
	m.language = lang
}

// ShowSlashHints 显示斜杠命令提示（支持查询过滤）
func (m *Model) ShowSlashHints(query string) {
	var slashHints []hints.Hint
	for _, cmd := range slash.GetSuggestions(query) {
		slashHints = append(slashHints, hints.Hint{
			Key:   cmd.DisplayText(),
			Desc:  cmd.Description(m.language),
			Value: cmd.Name,
		})
	}

	if len(slashHints) == 0 {
		slashHints = append(slashHints, hints.Hint{Key: "(无匹配命令)", Desc: ""})
	}

	m.hints.SetHints(slashHints)
	m.hints.SetHeight(10)
	m.hints.Show()
	m.SetSize(m.width, m.height)
}

// ShowPathHints 显示路径提示（支持查询过滤和子目录）
func (m *Model) ShowPathHints(query string) {
	wd, _ := os.Getwd()

	// 如果查询包含 /，说明在浏览子目录
	searchDir := wd
	searchQuery := query
	if strings.Contains(query, "/") {
		parts := strings.Split(query, "/")
		// 构建目录路径
		subDir := strings.Join(parts[:len(parts)-1], "/")
		searchDir = filepath.Join(wd, subDir)
		searchQuery = parts[len(parts)-1]
	}

	entries, _ := os.ReadDir(searchDir)

	var pathHints []hints.Hint
	limit := 20
	lq := strings.ToLower(searchQuery)

	for _, e := range entries {
		if len(pathHints) >= limit {
			break
		}
		name := e.Name()
		// 跳过隐藏文件
		if strings.HasPrefix(name, ".") {
			continue
		}

		// 如果有查询，进行匹配
		if searchQuery != "" {
			ln := strings.ToLower(name)
			if !strings.HasPrefix(ln, lq) && !strings.Contains(ln, lq) {
				continue
			}
		}

		desc := "文件"
		if e.IsDir() {
			desc = "目录"
			name += "/"
		}

		// 如果在子目录中，添加前缀
		if searchDir != wd {
			relPath, _ := filepath.Rel(wd, searchDir)
			name = filepath.ToSlash(filepath.Join(relPath, name))
		}

		pathHints = append(pathHints, hints.Hint{Key: name, Desc: desc, Value: name})
	}

	if len(pathHints) == 0 {
		pathHints = append(pathHints, hints.Hint{Key: "(无匹配文件)", Desc: ""})
	}

	m.hints.SetHints(pathHints)
	m.hints.SetHeight(10)
	m.hints.Show()
	m.SetSize(m.width, m.height)
}

// HideHints 隐藏提示
func (m *Model) HideHints() {
	m.hints.Hide()
	m.SetSize(m.width, m.height)
}

func (m *Model) ContentOrigin() (int, int) {
	return 2, 1
}

func (m *Model) ContentYOffset() int {
	return m.content.YOffset()
}

func (m *Model) ContentHeight() int {
	return m.content.Height()
}

func (m *Model) ContentLineCount() int {
	return m.content.LineCount()
}

// acceptHint 接受选中的 hint
func (m *Model) acceptHint(selected string) {
	text := m.input.Value()

	// 检查是路径提示还是命令提示
	if strings.Contains(text, "@") {
		// 路径提示：找到最后一个 @，替换后面的内容
		pos := strings.LastIndex(text, "@")
		if pos >= 0 {
			head := text[:pos+1] // 包括 @
			m.input.SetValue(head + selected + " ")
		}
	} else if strings.HasPrefix(text, "/") {
		// 命令提示：直接替换为选择的命令
		m.input.SetValue(selected + " ")
	}
}

// IsHintsVisible 检查提示是否显示
func (m *Model) IsHintsVisible() bool {
	return m.hints.Visible()
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.content.Init(),
		m.input.Init(),
		m.hints.Init(),
	)
}

// Update 更新
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if _, ok := msg.(statusTickMsg); ok {
		if !m.processing && !m.thinking {
			return m, nil
		}
		m.statusAnim = (m.statusAnim + 1) % 4
		return m, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return statusTickMsg{} })
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd

	atBottomBefore := m.content.AtBottom()
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "pgup", "pageup":
			m.content.HalfViewUp()
			if atBottomBefore != m.content.AtBottom() {
				m.relayout()
			}
			return m, nil
		case "pgdown", "pagedown":
			m.content.HalfViewDown()
			if atBottomBefore != m.content.AtBottom() {
				m.relayout()
			}
			return m, nil
		case "home":
			m.content.GotoTop()
			if atBottomBefore != m.content.AtBottom() {
				m.relayout()
			}
			return m, nil
		case "end":
			m.content.GotoBottom()
			if atBottomBefore != m.content.AtBottom() {
				m.relayout()
			}
			return m, nil
		}
	}
	// 如果 hints 显示，上下键只控制 hints 选择，不传递给内容区域和输入框
	if m.hints.Visible() {
		// 鼠标滚轮仍然允许滚动内容区（避免无法回看历史）
		if _, ok := msg.(tea.MouseMsg); ok {
			m.content, cmd = m.content.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		// 检查是否是上下键
		isNavKey := false
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "up", "down":
				isNavKey = true
			}
		}

		// 如果不是导航键，才更新提示组件（避免双重处理）
		if !isNavKey {
			m.hints, cmd = m.hints.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		// 如果不是导航键，才更新输入组件（保持光标闪烁等）
		if !isNavKey {
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}

	// 正常模式：更新所有组件
	// 更新内容组件
	m.content, cmd = m.content.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if atBottomBefore != m.content.AtBottom() {
		m.relayout()
	}

	// 更新输入组件
	inputHBefore := m.input.ViewHeight()
	m.input, cmd = m.input.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if inputHBefore != m.input.ViewHeight() {
		m.recomputeLayout()
	}

	// 更新提示组件
	m.hints, cmd = m.hints.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View 渲染视图
func (m Model) View() string {
	var sections []string

	// 内容区域 - 如果没有内容则显示欢迎面板
	contentView := m.content.View()
	// 使用原始内容判断是否为空，因为 View() 返回的字符串包含样式（padding等）
	hasContent := m.content.Content() != ""
	if m.showWelcome && !hasContent {
		sections = append(sections, m.styles.Panel.Render(m.welcome.View()))
	} else {
		sections = append(sections, m.styles.Panel.Render(contentView))
	}

	if livePanel := m.renderLivePanel(); livePanel != "" {
		sections = append(sections, livePanel)
	}

	// 状态栏
	statusBar := m.renderStatusBar()
	sections = append(sections, m.styles.StatusBar.Render(statusBar))

	// 输入区域
	sections = append(sections, m.input.View())

	// 提示区域
	if m.hints.Visible() {
		sections = append(sections, m.hints.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderStatusBar 渲染状态栏
func (m Model) renderStatusBar() string {
	var leftParts []string

	aiLabel := "[ " + i18n.T("status.mode_ai", m.language) + " ]"
	if m.mode == ModeAI {
		leftParts = append(leftParts, m.styles.TextInfo.Render(aiLabel))
	} else {
		bashLabel := "[ " + i18n.T("status.mode_bash", m.language) + " ]"
		leftParts = append(leftParts, m.styles.TextInfo.Render(bashLabel))
	}

	var rightParts []string
	if m.mode == ModeAI {
		if m.hintLive {
			rightParts = append(rightParts, m.styles.TextMuted.Render(i18n.T("status.hint.live", m.language)))
		}
		if m.hintThinking {
			rightParts = append(rightParts, m.styles.TextMuted.Render(i18n.T("status.hint.thinking_expand", m.language)))
		}
	}
	if m.bgTaskCount > 0 {
		rightParts = append(rightParts, m.styles.TextMuted.Render(i18n.T("status.tasks", m.language)+fmt.Sprintf("%d", m.bgTaskCount)))
	}
	if m.mode == ModeAI {
		rightParts = append(rightParts, m.styles.TextMuted.Render(i18n.T("status.exec", m.language)+executionModeLabel(m.language, m.executionMode)))
	}

	var ctxPart string
	if m.mode == ModeAI && m.ctxVisible && m.ctxRatio > 0 {
		r := m.ctxRatio
		if r < 0 {
			r = 0
		}
		if r > 1 {
			r = 1
		}
		pctStr := fmt.Sprintf("%.1f%%", r*100)
		valStyle := m.styles.TextSuccess
		if r >= 0.85 {
			valStyle = m.styles.TextError
		} else if r >= 0.65 {
			valStyle = m.styles.TextWarning
		}
		buckets := 10
		filled := int(math.Round(r * float64(buckets)))
		filled = min(max(filled, 0), buckets)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", buckets-filled)
		ctxPart = m.styles.TextMuted.Render(i18n.T("status.ctx", m.language)) + valStyle.Render(pctStr) + " " + m.styles.TextMuted.Render(bar)
	} else if m.mode == ModeAI && m.ctxVisible && m.ctxTokens > 0 {
		ctxPart = m.styles.TextMuted.Render(i18n.T("status.ctx", m.language) + fmt.Sprintf("%d", m.ctxTokens))
	}
	if strings.TrimSpace(ctxPart) != "" {
		rightParts = append(rightParts, ctxPart)
	}

	if m.mode == ModeAI {
		thinkingLabel := i18n.T("status.thinking", m.language)
		var thinkingPart string
		if !state.Thinking() {
			thinkingPart = m.styles.TextMuted.Render(thinkingLabel + ":" + i18n.T("status.off", m.language))
		} else {
			stateLabel := i18n.T("thinking.state.collapsed", m.language)
			if m.thinkingExpanded {
				stateLabel = i18n.T("thinking.state.expanded", m.language)
			}
			if m.thinking {
				thinkingPart = m.styles.TextMuted.Render(thinkingLabel + strings.Repeat(".", m.statusAnim) + "(" + stateLabel + ")")
			} else {
				thinkingPart = m.styles.TextMuted.Render(thinkingLabel + ":" + stateLabel)
			}
		}
		leftParts = append(leftParts, thinkingPart)

	}

	if m.processing {
		leftParts = append(leftParts, m.styles.TextMuted.Render(i18n.T("status.processing", m.language)+strings.Repeat(".", m.statusAnim)))
	}

	left := strings.Join(leftParts, " ")
	rightPart := strings.Join(rightParts, " · ")
	if strings.TrimSpace(rightPart) == "" {
		return left
	}

	innerW := max(m.width-2, 10)
	gap := innerW - lipgloss.Width(left) - lipgloss.Width(rightPart)
	gap = max(gap, 1)
	return left + strings.Repeat(" ", gap) + rightPart
}

func executionModeLabel(_ string, mode string) string {
	switch toolapi.NormalizeExecutionMode(mode) {
	case "plan":
		return "plan"
	default:
		return "auto"
	}
}

func (m *Model) StatusTick() tea.Cmd {
	if m == nil {
		return nil
	}
	if !m.processing && !m.thinking {
		return nil
	}
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return statusTickMsg{} })
}

// HandleKey 处理按键
func (m *Model) HandleKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd) {
	// 如果 hints 显示，优先处理 hints 导航
	if m.hints.Visible() {
		switch msg.String() {
		case "up":
			m.hints.CursorUp()
			return true, nil
		case "down":
			m.hints.CursorDown()
			return true, nil
		case "tab":
			// 接受选中的 hint
			selected := m.hints.Selected()
			if selected != "" {
				m.acceptHint(selected)
			}
			m.hints.Hide()
			m.input.Focus()
			return true, nil
		case "enter":
			// 接受选中的 hint 并隐藏 hints
			selected := m.hints.Selected()
			if selected != "" {
				m.acceptHint(selected)
			}
			m.hints.Hide()
			m.input.Focus()
			return true, nil
		case "esc":
			m.hints.Hide()
			m.input.Clear()
			m.input.Focus()
			return true, nil
		}
	}

	switch msg.String() {
	case "shift+enter":
		// 插入换行
		m.input.InsertNewline()
		return true, nil
	case "alt+enter":
		m.input.InsertNewline()
		return true, nil
	case "ctrl+j":
		m.input.InsertNewline()
		return true, nil
	case "up":
		if m.input.IsMultiLine() {
			return false, nil
		}
		m.input.HistoryUp()
		return true, nil
	case "down":
		if m.input.IsMultiLine() {
			return false, nil
		}
		m.input.HistoryDown()
		return true, nil
	case "esc":
		// 清空输入或取消
		m.input.Clear()
		return true, nil
	case "tab":
		if m.input.AcceptPrediction() {
			return true, nil
		}
	case "right":
		if m.input.AcceptPrediction() {
			return true, nil
		}
	}
	return false, nil
}
