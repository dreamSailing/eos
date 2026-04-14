package render

import (
	"fmt"
	"strings"
	"sync"

	uistyles "github.com/dreamSailing/vb-coding/internal/ui/styles"

	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	glstyles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// MarkdownRenderer Markdown渲染器
type MarkdownRenderer struct {
	width  int
	styles *RenderStyles

	mu   sync.Mutex
	tr   *glamour.TermRenderer
	trWW int
}

// RenderStyles 渲染样式
type RenderStyles struct {
	Header      lipgloss.Style
	CodeBlock   lipgloss.Style
	InlineCode  lipgloss.Style
	Link        lipgloss.Style
	Bold        lipgloss.Style
	Italic      lipgloss.Style
	Strike      lipgloss.Style
	List        lipgloss.Style
	Quote       lipgloss.Style
	Table       lipgloss.Style
	TableHeader lipgloss.Style
	TableCell   lipgloss.Style
	Normal      lipgloss.Style
}

// NewRenderStyles 创建新的渲染样式
func NewRenderStyles() *RenderStyles {
	return &RenderStyles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#6366f1")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#1e293b")).
			Foreground(lipgloss.Color("#f1f5f9")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#334155")).
			Foreground(lipgloss.Color("#f1f5f9")),
		Link: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3b82f6")).
			Underline(true),
		Bold: lipgloss.NewStyle().
			Bold(true),
		Italic: lipgloss.NewStyle().
			Italic(true),
		Strike: lipgloss.NewStyle().
			Strikethrough(true),
		List: lipgloss.NewStyle().
			MarginLeft(2),
		Quote: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94a3b8")).
			BorderLeft(true).
			BorderForeground(lipgloss.Color("#64748b")).
			PaddingLeft(1),
		Table: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()),
		TableHeader: lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#334155")),
		TableCell: lipgloss.NewStyle().
			Padding(0, 1),
		Normal: lipgloss.NewStyle(),
	}
}

func NewPlainRenderStyles() *RenderStyles {
	return &RenderStyles{
		Header:      lipgloss.NewStyle(),
		CodeBlock:   lipgloss.NewStyle().MarginLeft(2),
		InlineCode:  lipgloss.NewStyle(),
		Link:        lipgloss.NewStyle(),
		Bold:        lipgloss.NewStyle(),
		Italic:      lipgloss.NewStyle(),
		Strike:      lipgloss.NewStyle(),
		List:        lipgloss.NewStyle().MarginLeft(2),
		Quote:       lipgloss.NewStyle().MarginLeft(1),
		Table:       lipgloss.NewStyle(),
		TableHeader: lipgloss.NewStyle(),
		TableCell:   lipgloss.NewStyle(),
		Normal:      lipgloss.NewStyle(),
	}
}

func NewThemeRenderStyles(s *uistyles.Styles) *RenderStyles {
	if s == nil || s.Theme == nil {
		return NewPlainRenderStyles()
	}
	t := s.Theme
	headerStyle := s.MarkdownHeader
	linkStyle := s.MarkdownLink
	return &RenderStyles{
		Header: headerStyle,
		CodeBlock: lipgloss.NewStyle().
			Background(t.SurfaceAlt).
			BorderLeft(true).
			BorderForeground(t.Primary).
			Padding(0, 1).
			PaddingLeft(1),
		InlineCode: lipgloss.NewStyle().
			Background(t.SurfaceAlt).
			Foreground(t.Text),
		Link: linkStyle,
		Bold: lipgloss.NewStyle().Bold(true),
		Italic: lipgloss.NewStyle().
			Italic(true),
		Strike: lipgloss.NewStyle().Strikethrough(true),
		List: lipgloss.NewStyle().
			Foreground(t.Text),
		Quote: lipgloss.NewStyle().
			Foreground(t.TextMuted).
			BorderLeft(true).
			BorderForeground(t.Muted).
			PaddingLeft(1),
		Table:       lipgloss.NewStyle().Foreground(t.Text),
		TableHeader: lipgloss.NewStyle().Bold(true).Foreground(t.Text),
		TableCell:   lipgloss.NewStyle().Foreground(t.Text),
		Normal:      lipgloss.NewStyle().Foreground(t.Text),
	}
}

// NewMarkdownRenderer 创建新的Markdown渲染器
func NewMarkdownRenderer(width int) *MarkdownRenderer {
	r := &MarkdownRenderer{
		width:  width,
		styles: NewRenderStyles(),
	}
	r.rebuild()
	return r
}

// SetWidth 设置宽度
func (r *MarkdownRenderer) SetWidth(width int) {
	r.width = width
	r.rebuild()
}

func (r *MarkdownRenderer) SetStyles(styles *RenderStyles) {
	if styles != nil {
		r.styles = styles
	}
	r.rebuild()
}

// Render 渲染Markdown文本
func (r *MarkdownRenderer) Render(text string) string {
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = unwrapMarkdownFences(text)

	r.mu.Lock()
	tr := r.tr
	r.mu.Unlock()
	if tr == nil {
		return strings.TrimRight(text, "\n")
	}
	out, err := tr.Render(text)
	if err != nil {
		return strings.TrimRight(text, "\n")
	}
	return strings.TrimRight(out, "\n")
}

func (r *MarkdownRenderer) rebuild() {
	ww := r.width
	if ww < 20 {
		ww = 20
	}

	profile := termenv.EnvColorProfile()
	chromaFmt := "terminal16"
	switch profile {
	case termenv.ANSI256:
		chromaFmt = "terminal256"
	case termenv.TrueColor:
		chromaFmt = "terminal16m"
	}

	cfg := glstyles.TokyoNightStyleConfig
	cfg.Heading.Prefix = ""
	cfg.H1.Prefix = ""
	cfg.H2.Prefix = ""
	cfg.H3.Prefix = ""
	cfg.H4.Prefix = ""
	cfg.H5.Prefix = ""
	cfg.H6.Prefix = ""
	cfg.CodeBlock.Theme = "monokai"
	codeBG := "#1e293b"
	codeFG := "#e5e7eb"
	indentToken := "│ "
	margin := uint(1)
	indent := uint(1)
	cfg.CodeBlock.BackgroundColor = &codeBG
	cfg.CodeBlock.Color = &codeFG
	cfg.CodeBlock.Margin = &margin
	cfg.CodeBlock.Indent = &indent
	cfg.CodeBlock.IndentToken = &indentToken
	_ = gansi.TemplateFuncMap

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tr != nil && r.trWW == ww {
		return
	}
	tr, err := glamour.NewTermRenderer(
		glamour.WithStyles(cfg),
		glamour.WithWordWrap(ww),
		glamour.WithPreservedNewLines(),
		glamour.WithColorProfile(profile),
		glamour.WithChromaFormatter(chromaFmt),
	)
	if err != nil {
		r.tr = nil
		r.trWW = ww
		return
	}
	r.tr = tr
	r.trWW = ww
}

func unwrapMarkdownFences(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	in := false
	fence := ""
	for _, line := range lines {
		trim := strings.TrimLeft(line, " \t")
		if !in {
			if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
				fence = "```"
				if strings.HasPrefix(trim, "~~~") {
					fence = "~~~"
				}
				lang := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trim, fence)))
				if lang == "markdown" || lang == "md" {
					in = true
					continue
				}
			}
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(trim, fence) {
			in = false
			fence = ""
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// RenderToolCall 渲染工具调用
func (r *MarkdownRenderer) RenderToolCall(name string, params map[string]any) string {
	var result strings.Builder
	result.WriteString("▶ ")
	result.WriteString(name)
	if len(params) > 0 {
		result.WriteString("(")
		var parts []string
		for k, v := range params {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		result.WriteString(strings.Join(parts, ", "))
		result.WriteString(")")
	}
	return result.String()
}

// RenderToolResult 渲染工具结果
func (r *MarkdownRenderer) RenderToolResult(status, output string) string {
	icon := "✓"
	if status != "success" {
		icon = "✗"
	}
	return fmt.Sprintf("%s %s", icon, output)
}
