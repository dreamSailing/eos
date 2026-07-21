package panels

import (
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/ui/components/input"
	"github.com/dreamSailing/eos/internal/ui/styles"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type MemoryRefreshMsg struct{}

type MemoryRebuildIndexMsg struct{}

type MemorySaveMsg struct {
	Scope   string
	Content string
}

type MemoryPanel struct {
	BasePanel
	styles   *styles.Styles
	language string

	activeRoot string

	globalPath    string
	globalContent string
	globalExists  bool

	projectPath    string
	projectContent string
	projectExists  bool

	sessionPath    string
	sessionContent string
	sessionExists  bool

	indexPath    string
	indexContent string
	indexExists  bool

	scope   int
	view    viewport.Model
	editing bool
	editor  input.Model
}

func NewMemoryPanel(styles *styles.Styles, lang string) *MemoryPanel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	ed := input.New()
	ed.SetPlaceholder("Edit memory here...")
	p := &MemoryPanel{
		BasePanel: NewBasePanel("memory"),
		styles:    styles,
		language:  lang,
		view:      vp,
		editor:    ed,
	}
	p.updateViewContent()
	return p
}

func (p *MemoryPanel) Init() tea.Cmd { return nil }

func (p *MemoryPanel) IsEditing() bool { return p != nil && p.editing }

func (p *MemoryPanel) CancelEdit() {
	if p == nil {
		return
	}
	p.editing = false
	p.editor.Blur()
	p.updateViewContent()
}

func (p *MemoryPanel) SetData(activeRoot string, globalPath string, globalContent string, globalExists bool, projectPath string, projectContent string, projectExists bool, sessionPath string, sessionContent string, sessionExists bool, indexPath string, indexContent string, indexExists bool) {
	p.activeRoot = strings.TrimSpace(activeRoot)
	p.globalPath = strings.TrimSpace(globalPath)
	p.globalContent = globalContent
	p.globalExists = globalExists
	p.projectPath = strings.TrimSpace(projectPath)
	p.projectContent = projectContent
	p.projectExists = projectExists
	p.sessionPath = strings.TrimSpace(sessionPath)
	p.sessionContent = sessionContent
	p.sessionExists = sessionExists
	p.indexPath = strings.TrimSpace(indexPath)
	p.indexContent = indexContent
	p.indexExists = indexExists
	p.updateViewContent()
}

func (p *MemoryPanel) scopeLabels() []string {
	return []string{"Global", "Project", "Session", "Index"}
}

func (p *MemoryPanel) scopeLabel() string {
	labels := p.scopeLabels()
	if p.scope < 0 || p.scope >= len(labels) {
		return labels[0]
	}
	return labels[p.scope]
}

func (p *MemoryPanel) scopePath() (string, bool) {
	switch p.scope {
	case 1:
		return p.projectPath, p.projectExists
	case 2:
		return p.sessionPath, p.sessionExists
	case 3:
		return p.indexPath, p.indexExists
	default:
		return p.globalPath, p.globalExists
	}
}

func (p *MemoryPanel) scopeContent() string {
	switch p.scope {
	case 1:
		return p.projectContent
	case 2:
		return p.sessionContent
	case 3:
		return p.indexContent
	default:
		return p.globalContent
	}
}

func (p *MemoryPanel) currentScopeKey() string {
	switch p.scope {
	case 1:
		return "project"
	case 2:
		return "session"
	case 3:
		return "index"
	default:
		return "global"
	}
}

func (p *MemoryPanel) setScopeContent(v string) {
	switch p.scope {
	case 1:
		p.projectContent = v
		p.projectExists = true
	case 2:
		p.sessionContent = v
		p.sessionExists = true
	case 3:
		p.indexContent = v
		p.indexExists = true
	default:
		p.globalContent = v
		p.globalExists = true
	}
}

func (p *MemoryPanel) updateViewContent() {
	if p == nil {
		return
	}
	body := strings.TrimRight(p.scopeContent(), "\n")
	if strings.TrimSpace(body) == "" {
		path, exists := p.scopePath()
		if strings.TrimSpace(path) == "" {
			body = "(empty)"
		} else if !exists {
			body = "(file not found)"
		} else {
			body = "(empty)"
		}
	}
	p.view.SetContent(body)
}

func (p *MemoryPanel) enterEdit() {
	if p == nil || p.currentScopeKey() == "index" {
		return
	}
	p.editing = true
	p.editor.SetValue(p.scopeContent())
	p.editor.Focus()
}

func (p *MemoryPanel) Update(msg tea.Msg) (Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case LanguageChangeMsg:
		p.language = msg.Language
		return p, nil
	case tea.KeyMsg:
		if p.editing {
			switch msg.String() {
			case "ctrl+s":
				text := p.editor.Value()
				p.editor.AddToHistory(text)
				scope := p.currentScopeKey()
				p.setScopeContent(text)
				p.editing = false
				p.editor.Blur()
				p.updateViewContent()
				return p, func() tea.Msg { return MemorySaveMsg{Scope: scope, Content: text} }
			}
			var cmd tea.Cmd
			p.editor, cmd = p.editor.Update(msg)
			return p, cmd
		}

		switch msg.String() {
		case "tab", "right", "l":
			p.scope = (p.scope + 1) % len(p.scopeLabels())
			p.updateViewContent()
			return p, nil
		case "shift+tab", "left", "h":
			p.scope--
			if p.scope < 0 {
				p.scope = len(p.scopeLabels()) - 1
			}
			p.updateViewContent()
			return p, nil
		case "e":
			if p.currentScopeKey() == "index" {
				return p, nil
			}
			p.enterEdit()
			return p, p.editor.Init()
		case "r":
			return p, func() tea.Msg { return MemoryRefreshMsg{} }
		case "b":
			return p, func() tea.Msg { return MemoryRebuildIndexMsg{} }
		}
		var cmd tea.Cmd
		p.view, cmd = p.view.Update(msg)
		return p, cmd
	case tea.MouseMsg:
		if p.editing {
			var cmd tea.Cmd
			p.editor, cmd = p.editor.Update(msg)
			return p, cmd
		}
		var cmd tea.Cmd
		p.view, cmd = p.view.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p *MemoryPanel) View() string {
	var b strings.Builder
	b.WriteString(p.styles.PanelTitle.Render("Memory"))
	b.WriteString("\n\n")
	if strings.TrimSpace(p.activeRoot) != "" {
		b.WriteString(p.styles.TextMuted.Render("Active workspace: " + filepath.ToSlash(p.activeRoot)))
		b.WriteString("\n")
	}

	var tabs []string
	for i, label := range p.scopeLabels() {
		if i == p.scope {
			tabs = append(tabs, p.styles.TextSuccess.Render("["+label+"]"))
		} else {
			tabs = append(tabs, p.styles.TextMuted.Render(label))
		}
	}
	b.WriteString(strings.Join(tabs, "  "))
	b.WriteString("\n\n")

	path, exists := p.scopePath()
	status := "missing"
	if exists {
		status = "ok"
	}
	b.WriteString(p.styles.TextMuted.Render(p.scopeLabel() + ": " + filepath.ToSlash(path) + " (" + status + ")"))
	b.WriteString("\n\n")

	if p.editing {
		b.WriteString(p.editor.View())
		b.WriteString("\n\n")
		b.WriteString(p.styles.TextMuted.Render(i18n.T("memory.footer.editing", p.language)))
	} else {
		b.WriteString(p.view.View())
		b.WriteString("\n\n")
		b.WriteString(p.styles.TextMuted.Render(i18n.T("memory.footer.browse", p.language)))
	}
	return p.RenderBorder(b.String(), "Memory Panel")
}

func (p *MemoryPanel) SetSize(width, height int) {
	p.BasePanel.SetSize(width, height)
	w := width - 6
	h := height - 10
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	p.view.Width = w
	p.view.Height = h
	p.editor.SetSize(w, h)
	p.updateViewContent()
}
