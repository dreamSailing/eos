package help

import (
	"strings"

	"github.com/dreamSailing/vb-coding/internal/i18n"
	"github.com/dreamSailing/vb-coding/internal/ui/styles"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpView 帮助视图
type HelpView struct {
	width    int
	height   int
	styles   *styles.Styles
	language string
	vp       viewport.Model
	content  string
	resetTop bool
}

// NewHelpView 创建新的帮助视图
func NewHelpView(styles *styles.Styles, lang string) *HelpView {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	return &HelpView{
		styles:   styles,
		language: lang,
		vp:       vp,
		resetTop: true,
	}
}

// SetLanguage 设置语言
func (h *HelpView) SetLanguage(lang string) {
	h.language = lang
	h.resetTop = true
}

// SetSize 设置大小
func (h *HelpView) SetSize(width, height int) {
	h.width = width
	h.height = height
	h.relayout()
}

func (h *HelpView) ResetScroll() {
	h.resetTop = true
}

func (h *HelpView) relayout() {
	w := h.width
	hh := h.height
	if w <= 0 {
		w = 80
	}
	if hh <= 0 {
		hh = 40
	}
	innerW := w - 2 - 4
	innerH := hh - 2 - 2
	if innerW < 10 {
		innerW = 10
	}
	if innerH < 5 {
		innerH = 5
	}
	h.vp.Width = innerW
	h.vp.Height = innerH
}

// Init 初始化
func (h *HelpView) Init() tea.Cmd {
	return nil
}

// Update 更新
func (h *HelpView) Update(msg tea.Msg) (*HelpView, tea.Cmd) {
	switch msg := msg.(type) {
	case LanguageChangeMsg:
		h.language = msg.Language
		h.resetTop = true
	case tea.KeyMsg, tea.MouseMsg:
		var cmd tea.Cmd
		h.vp, cmd = h.vp.Update(msg)
		return h, cmd
	}
	return h, nil
}

// View 渲染
func (h *HelpView) View() string {
	if h.width == 0 {
		h.width = 80
	}
	if h.height == 0 {
		h.height = 40
	}
	h.relayout()

	lang := h.language
	var b strings.Builder

	// 标题
	b.WriteString(h.styles.PanelTitle.Render(i18n.T("help.title", lang)))
	b.WriteString("\n\n")

	// 全局快捷键
	b.WriteString(h.styles.TextInfo.Render(i18n.T("help.global", lang) + "\n"))
	b.WriteString(h.renderKey("F2", i18n.T("help.key.f2", lang)) + "\n")
	b.WriteString(h.renderKey("Tab", i18n.T("help.key.tab", lang)) + "\n")
	b.WriteString(h.renderKey("Alt+V", i18n.T("help.key.alt_v", lang)) + "\n")
	b.WriteString(h.renderKey("Ctrl+J", i18n.T("help.key.ctrl_j", lang)) + "\n")
	b.WriteString(h.renderKey("Esc Esc", i18n.T("help.key.esc_esc", lang)) + "\n")
	b.WriteString(h.renderKey("Ctrl+C", i18n.T("help.key.ctrl_c", lang)) + "\n")
	b.WriteString(h.renderKey("PgUp/PgDn", i18n.T("help.key.pgup_pgdn", lang)) + "\n")
	b.WriteString(h.renderKey("Home / End", i18n.T("help.key.home_end", lang)) + "\n")
	b.WriteString(h.renderKey("↑/↓", i18n.T("help.key.up_down", lang)) + "\n")
	b.WriteString("\n")

	// 斜杠命令
	b.WriteString(h.styles.TextInfo.Render(i18n.T("help.slash_commands", lang) + "\n"))
	b.WriteString(h.renderCmd("/help", i18n.T("help.cmd.help", lang)) + "\n")
	b.WriteString(h.renderCmd("/clear", i18n.T("help.cmd.clear", lang)) + "\n")
	b.WriteString(h.renderCmd("/exit", i18n.T("help.cmd.exit", lang)) + "\n")
	b.WriteString(h.renderCmd("/history", i18n.T("help.cmd.history", lang)) + "\n")
	b.WriteString(h.renderCmd("/models", i18n.T("help.cmd.models", lang)) + "\n")
	b.WriteString(h.renderCmd("/mcp", i18n.T("help.cmd.mcp", lang)) + "\n")
	b.WriteString(h.renderCmd("/ctx", i18n.T("help.cmd.ctx", lang)) + "\n")
	b.WriteString(h.renderCmd("/cost", i18n.T("help.cmd.cost", lang)) + "\n")
	b.WriteString(h.renderCmd("/tasks", i18n.T("help.cmd.tasks", lang)) + "\n")
	b.WriteString(h.renderCmd("/lsp", i18n.T("help.cmd.lsp", lang)) + "\n")
	b.WriteString(h.renderCmd("/rules", i18n.T("help.cmd.rules", lang)) + "\n")
	b.WriteString(h.renderCmd("/workspace", i18n.T("help.cmd.workspace", lang)) + "\n")
	b.WriteString(h.renderCmd("/git", i18n.T("help.cmd.git", lang)) + "\n")
	b.WriteString(h.renderCmd("/lang zh|en", i18n.T("help.cmd.lang", lang)) + "\n")
	b.WriteString(h.renderCmd("/compact", i18n.T("help.cmd.compact", lang)) + "\n")
	b.WriteString(h.renderCmd("/settings", i18n.T("help.cmd.settings", lang)) + "\n")
	b.WriteString("\n")

	b.WriteString(h.styles.TextMuted.Render(i18n.T("footer.close_help", lang)))

	next := b.String()
	if next != h.content {
		h.content = next
		h.vp.SetContent(next)
		h.resetTop = true
	}
	if h.resetTop {
		h.vp.GotoTop()
		h.resetTop = false
	}

	// 包装在面板样式中
	panelStyle := lipgloss.NewStyle().
		Width(h.width).
		Height(h.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#6366f1")).
		Padding(1, 2)

	return panelStyle.Render(h.vp.View())
}

// renderKey 渲染快捷键
func (h *HelpView) renderKey(key, desc string) string {
	keyStyle := h.styles.TextInfo.Copy().Bold(true)
	keyText := key
	pad := 12 - lipgloss.Width(keyText)
	if pad < 1 {
		pad = 1
	}
	return keyStyle.Render(keyText) + strings.Repeat(" ", pad) + h.styles.TextMuted.Render(desc)
}

// renderCmd 渲染命令
func (h *HelpView) renderCmd(cmd, desc string) string {
	cmdStyle := h.styles.TextInfo.Copy().Bold(true)
	cmdText := cmd
	pad := 14 - lipgloss.Width(cmdText)
	if pad < 1 {
		pad = 1
	}
	return cmdStyle.Render(cmdText) + strings.Repeat(" ", pad) + h.styles.TextMuted.Render("- "+desc)
}

// LanguageChangeMsg 语言切换消息
type LanguageChangeMsg struct {
	Language string
}
