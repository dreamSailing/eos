package confirm

import (
	"strings"

	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/ui/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Request struct {
	ID        string
	Kind      string
	Title     string
	Question  string
	Options   []string
	Diff      string
	DiffPath  string
	AllowText bool
	TextHint  string
}

type ResultMsg struct {
	ID          string
	Kind        string
	Decision    string
	Option      string
	OptionIndex int
	Text        string
}

type Model struct {
	width    int
	height   int
	styles   *styles.Styles
	language string

	req      Request
	selected int
	focusTxt bool
	input    textinput.Model

	panel lipgloss.Style
	title lipgloss.Style
	box   lipgloss.Style
	muted lipgloss.Style
	opt   lipgloss.Style
	sel   lipgloss.Style
}

func New(styles *styles.Styles, lang string, req Request) *Model {
	in := textinput.New()
	in.Width = 60
	in.Placeholder = req.TextHint
	in.Prompt = "> "
	in.Focus()
	if !req.AllowText {
		in.Blur()
	}
	m := &Model{
		styles:   styles,
		language: lang,
		req:      req,
		input:    in,
	}
	m.panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f59e0b")).
		Padding(1, 2)
	m.title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f59e0b"))
	m.box = lipgloss.NewStyle().Background(lipgloss.Color("#0f172a")).Foreground(lipgloss.Color("#e2e8f0")).Padding(1, 2)
	m.muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))
	m.opt = lipgloss.NewStyle().Padding(0, 2)
	m.sel = lipgloss.NewStyle().Background(lipgloss.Color("#6366f1")).Foreground(lipgloss.Color("#ffffff")).Padding(0, 2)
	return m
}

func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	if w > 10 {
		m.input.Width = w - 10
		if m.input.Width > 80 {
			m.input.Width = 80
		}
		if m.input.Width < 30 {
			m.input.Width = 30
		}
	}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.req.Kind == "permission" {
				return m, func() tea.Msg {
					return ResultMsg{ID: m.req.ID, Kind: m.req.Kind, Decision: "deny", OptionIndex: -1}
				}
			}
			return m, func() tea.Msg {
				return ResultMsg{ID: m.req.ID, Kind: m.req.Kind, Decision: "cancel", OptionIndex: -1}
			}
		case "tab":
			if m.req.AllowText {
				m.focusTxt = !m.focusTxt
				if m.focusTxt {
					m.input.Focus()
				} else {
					m.input.Blur()
				}
			}
			return m, nil
		case "up":
			if !m.focusTxt && m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down":
			if !m.focusTxt && m.selected < len(m.req.Options)-1 {
				m.selected++
			}
			return m, nil
		case "enter":
			opt := ""
			if m.selected >= 0 && m.selected < len(m.req.Options) {
				opt = m.req.Options[m.selected]
			}
			decision := ""
			switch m.req.Kind {
			case "permission":
				switch opt {
				case "allow_once":
					decision = "allow"
				case "allow_session":
					decision = "session"
				default:
					decision = "deny"
				}
			default:
				decision = "confirm"
			}
			txt := strings.TrimSpace(m.input.Value())
			return m, func() tea.Msg {
				return ResultMsg{ID: m.req.ID, Kind: m.req.Kind, Decision: decision, Option: opt, OptionIndex: m.selected, Text: txt}
			}
		default:
			if !m.focusTxt && len(msg.String()) == 1 {
				k := msg.String()[0]
				if k >= '1' && k <= '9' {
					idx := int(k - '1')
					if idx >= 0 && idx < len(m.req.Options) {
						m.selected = idx
					}
				}
			}
		}
		if m.focusTxt {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) View() string {
	title := m.req.Title
	if strings.TrimSpace(title) == "" {
		if m.req.Kind == "permission" {
			title = i18n.T("status.header", m.language)
		} else {
			title = i18n.T("op.confirm", m.language)
		}
	}

	var b strings.Builder
	b.WriteString(m.title.Render(title))
	b.WriteString("\n\n")

	q := strings.TrimSpace(m.req.Question)
	if q != "" {
		b.WriteString(m.box.Render(q))
		b.WriteString("\n\n")
	}

	if strings.TrimSpace(m.req.Diff) != "" {
		diff := m.req.Diff
		if len(diff) > 5000 {
			diff = diff[:5000] + "..."
		}
		if strings.TrimSpace(m.req.DiffPath) != "" {
			b.WriteString(m.muted.Render(m.req.DiffPath))
			b.WriteString("\n")
		}
		b.WriteString(m.box.Render(diff))
		b.WriteString("\n\n")
	}

	for i, opt := range m.req.Options {
		style := m.opt
		if i == m.selected {
			style = m.sel
		}
		label := opt
		switch m.req.Kind {
		case "permission":
			switch opt {
			case "allow_once":
				label = "1. " + i18n.T("allow.once", m.language)
			case "allow_session":
				label = "2. " + i18n.T("allow.session", m.language)
			case "deny":
				label = "3. " + i18n.T("deny", m.language)
			}
		default:
			label = strconvItoa(i+1) + ". " + opt
		}
		b.WriteString(style.Render(label))
		b.WriteString("\n")
	}

	if m.req.AllowText {
		b.WriteString("\n")
		b.WriteString(m.muted.Render(m.req.TextHint))
		b.WriteString("\n")
		b.WriteString(m.input.View())
	}

	b.WriteString("\n\n")
	helpKey := "confirm.help"
	if m.req.AllowText {
		helpKey = "confirm.help.text"
	}
	b.WriteString(m.muted.Render(i18n.T(helpKey, m.language)))

	return m.panel.Render(b.String())
}

func strconvItoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
