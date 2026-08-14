package panels

// 记忆面板：只读查看 ~/.eos/memories 的两个核心文件（memory_summary.md +
// MEMORY.md，数据来自内核 memory/snapshot），并提供「添加记忆笔记」入口
//（走内核 memory/save，语义 = 写一条 ad_hoc note）。交互对齐 Codex /memories
// 的只读设计：面板不编辑记忆文件本身，生成/合并由内核写管线负责。

import (
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/ui/components/input"
	"github.com/dreamSailing/eos/internal/ui/styles"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// MemoryRefreshMsg 请求刷新记忆快照。
type MemoryRefreshMsg struct{}

// MemorySaveMsg 携带一条 ad_hoc 记忆笔记内容（内核 memory/save 落盘）。
type MemorySaveMsg struct {
	Content string
}

// MemoryDoc 是面板侧的记忆文档投影；Scope 固定为 memory_summary.md / MEMORY.md。
type MemoryDoc struct {
	Scope   string
	Path    string
	Content string
	Exists  bool
}

// memoryDocScopes 是面板展示的文档顺序（与内核 panel_snapshot 一致）。
var memoryDocScopes = [2]string{"memory_summary.md", "MEMORY.md"}

type MemoryPanel struct {
	BasePanel
	styles   *styles.Styles
	language string

	docs [2]MemoryDoc
	tab  int

	view      viewport.Model
	composing bool
	editor    input.Model
}

func NewMemoryPanel(styles *styles.Styles, lang string) *MemoryPanel {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	ed := input.New()
	p := &MemoryPanel{
		BasePanel: NewBasePanel("memory"),
		styles:    styles,
		language:  lang,
		view:      vp,
		editor:    ed,
	}
	p.updateEditorPlaceholder()
	p.updateViewContent()
	return p
}

func (p *MemoryPanel) Init() tea.Cmd { return nil }

func (p *MemoryPanel) IsEditing() bool { return p != nil && p.composing }

func (p *MemoryPanel) CancelEdit() {
	if p == nil {
		return
	}
	p.composing = false
	p.editor.Blur()
	p.updateViewContent()
}

// SetData 注入快照数据；docs 顺序固定为 [memory_summary.md, MEMORY.md]，
// 缺失的文档以 Exists=false 呈现空态。
func (p *MemoryPanel) SetData(docs []MemoryDoc) {
	for i := range p.docs {
		p.docs[i] = MemoryDoc{Scope: memoryDocScopes[i]}
	}
	for i, doc := range docs {
		if i < len(p.docs) {
			p.docs[i] = doc
		}
	}
	p.updateViewContent()
}

func (p *MemoryPanel) currentDoc() MemoryDoc {
	if p.tab < 0 || p.tab >= len(p.docs) {
		return p.docs[0]
	}
	return p.docs[p.tab]
}

func (p *MemoryPanel) updateViewContent() {
	if p == nil {
		return
	}
	doc := p.currentDoc()
	body := strings.TrimRight(doc.Content, "\n")
	if !doc.Exists || strings.TrimSpace(body) == "" {
		// 空态：记忆由内核写管线在会话提炼后生成，尚未生成时提示而非报错。
		body = i18n.T("memory.empty", p.language)
	}
	p.view.SetContent(body)
}

func (p *MemoryPanel) enterCompose() {
	if p == nil {
		return
	}
	p.updateEditorPlaceholder()
	p.composing = true
	p.editor.SetValue("")
	p.editor.Focus()
}

func (p *MemoryPanel) updateEditorPlaceholder() {
	p.editor.SetPlaceholder(i18n.T("memory.note.placeholder", p.language))
}

func (p *MemoryPanel) Update(msg tea.Msg) (Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case LanguageChangeMsg:
		p.language = msg.Language
		p.updateEditorPlaceholder()
		p.updateViewContent()
		return p, nil
	case tea.KeyMsg:
		if p.composing {
			switch msg.String() {
			case "ctrl+s":
				text := p.editor.Value()
				p.editor.AddToHistory(text)
				p.composing = false
				p.editor.Blur()
				p.updateViewContent()
				return p, func() tea.Msg { return MemorySaveMsg{Content: text} }
			}
			var cmd tea.Cmd
			p.editor, cmd = p.editor.Update(msg)
			return p, cmd
		}

		switch msg.String() {
		case "tab", "right", "l":
			p.tab = (p.tab + 1) % len(p.docs)
			p.updateViewContent()
			return p, nil
		case "shift+tab", "left", "h":
			p.tab--
			if p.tab < 0 {
				p.tab = len(p.docs) - 1
			}
			p.updateViewContent()
			return p, nil
		case "a":
			p.enterCompose()
			return p, p.editor.Init()
		case "r":
			return p, func() tea.Msg { return MemoryRefreshMsg{} }
		}
		var cmd tea.Cmd
		p.view, cmd = p.view.Update(msg)
		return p, cmd
	case tea.MouseMsg:
		if p.composing {
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

	var tabs []string
	for i, label := range memoryDocScopes {
		if i == p.tab {
			tabs = append(tabs, p.styles.TextSuccess.Render("["+label+"]"))
		} else {
			tabs = append(tabs, p.styles.TextMuted.Render(label))
		}
	}
	b.WriteString(strings.Join(tabs, "  "))
	b.WriteString("\n\n")

	doc := p.currentDoc()
	status := "missing"
	if doc.Exists {
		status = "ok"
	}
	b.WriteString(p.styles.TextMuted.Render(filepath.ToSlash(doc.Path) + " (" + status + ")"))
	b.WriteString("\n\n")

	if p.composing {
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
