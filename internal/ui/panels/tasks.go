package panels

import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/toolapi"
	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
	"github.com/dreamSailing/eos/internal/tools/bg"
	"github.com/dreamSailing/eos/internal/ui/styles"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TasksPanel struct {
	BasePanel
	styles   *styles.Styles
	language string

	table   table.Model
	tasks   []toolapi.TaskInfo
	viewing bool

	viewID    string
	viewSeq   int64
	viewTask  toolapi.TaskInfo
	viewInfo  bg.TaskInfo
	viewLines []string
	vp        viewport.Model
}

func NewTasksPanel(styles *styles.Styles, lang string) *TasksPanel {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#6366f1")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#6366f1")).
		Bold(true)

	t := table.New(
		table.WithFocused(true),
	)
	t.KeyMap.LineUp.SetKeys("up", "k")
	t.KeyMap.LineDown.SetKeys("down", "j")

	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	p := &TasksPanel{
		BasePanel: NewBasePanel("tasks"),
		styles:    styles,
		language:  lang,
		table:     t,
		vp:        vp,
	}
	p.refresh()
	return p
}

func (p *TasksPanel) Init() tea.Cmd {
	return tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg { return TasksTickMsg{} })
}

type TasksTickMsg struct{}

func (p *TasksPanel) Update(msg tea.Msg) (Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case LanguageChangeMsg:
		p.language = msg.Language
		return p, nil
	case TasksTickMsg:
		p.refresh()
		if p.viewing {
			p.refreshView()
		}
		return p, tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg { return TasksTickMsg{} })
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			p.refresh()
			if p.viewing {
				p.refreshView()
			}
			return p, nil
		case "enter":
			if !p.viewing {
				id := p.selectedID()
				if id != "" {
					p.openView(id)
				}
			}
			return p, nil
		case "k":
			if p.viewing {
				return p, func() tea.Msg { return TaskKillRequestMsg{ID: p.viewID} }
			}
			id := p.selectedID()
			if id != "" {
				return p, func() tea.Msg { return TaskKillRequestMsg{ID: id} }
			}
			return p, nil
		case "c":
			n := bg.Default().CleanupFinished()
			p.refresh()
			if n > 0 {
				return p, func() tea.Msg { return TaskToastMsg{Text: fmt.Sprintf(i18n.T("tasks.cleaned", p.language), n)} }
			}
			return p, nil
		}
		if p.viewing {
			var cmd tea.Cmd
			p.vp, cmd = p.vp.Update(msg)
			return p, cmd
		}
		var cmd tea.Cmd
		p.table, cmd = p.table.Update(msg)
		return p, cmd
	case tea.MouseMsg:
		if p.viewing {
			var cmd tea.Cmd
			p.vp, cmd = p.vp.Update(msg)
			return p, cmd
		}
		var cmd tea.Cmd
		p.table, cmd = p.table.Update(msg)
		return p, cmd
	}

	if p.viewing {
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		return p, cmd
	}
	var cmd tea.Cmd
	p.table, cmd = p.table.Update(msg)
	return p, cmd
}

func (p *TasksPanel) View() string {
	if p.viewing {
		return p.viewDetail()
	}
	var b strings.Builder
	b.WriteString(p.styles.TextInfo.Render(i18n.T("tasks.title", p.language)))
	b.WriteString("\n\n")
	b.WriteString(p.table.View())
	b.WriteString("\n\n")
	b.WriteString(p.styles.TextMuted.Render(i18n.T("tasks.help", p.language)))
	return p.RenderBorder(b.String(), "")
}

func (p *TasksPanel) SetSize(width, height int) {
	p.BasePanel.SetSize(width, height)
	p.table.SetWidth(width - 6)
	p.table.SetHeight(height - 10)
	p.vp.Width = width - 6
	p.vp.Height = height - 10
}

func (p *TasksPanel) IsViewing() bool { return p.viewing }

func (p *TasksPanel) ResetView() {
	p.viewing = false
	p.viewID = ""
	p.viewSeq = 0
	p.viewTask = toolapi.TaskInfo{}
	p.viewLines = nil
	p.vp.SetContent("")
}

func (p *TasksPanel) selectedID() string {
	if len(p.tasks) == 0 {
		return ""
	}
	idx := p.table.Cursor()
	if idx < 0 || idx >= len(p.tasks) {
		return ""
	}
	return p.tasks[idx].ID
}

func (p *TasksPanel) refresh() {
	items, err := toolapiimpl.NewServices().Tasks().List(context.Background())
	if err != nil {
		p.tasks = nil
	} else {
		p.tasks = items
	}
	w, _ := p.GetSize()
	tableW := w - 6
	if tableW < 40 {
		tableW = 40
	}
	fixed := 14 + 12 + 10 + 16
	gaps := 8
	cmdW := tableW - fixed - gaps
	if cmdW < 20 {
		cmdW = 20
	}
	cols := []table.Column{
		{Title: i18n.T("tasks.col.id", p.language), Width: 14},
		{Title: "Kind", Width: 12},
		{Title: i18n.T("tasks.col.status", p.language), Width: 10},
		{Title: i18n.T("tasks.col.started", p.language), Width: 16},
		{Title: i18n.T("tasks.col.command", p.language), Width: cmdW},
	}
	rows := make([]table.Row, 0, len(p.tasks))
	for _, t := range p.tasks {
		start := t.UpdatedAt
		if start.IsZero() {
			start = t.StartedAt
		}
		cmd := strings.TrimSpace(t.Label)
		if cmd == "" {
			cmd = strings.TrimSpace(t.Summary)
		}
		if lipgloss.Width(cmd) > cmdW {
			runes := []rune(cmd)
			if len(runes) > cmdW-1 && cmdW > 1 {
				cmd = string(runes[:cmdW-1]) + "…"
			}
		}
		rows = append(rows, table.Row{t.ID, t.Kind, t.Status, start.Format("01-02 15:04:05"), cmd})
	}
	p.table.SetColumns(cols)
	p.table.SetRows(rows)
}

func (p *TasksPanel) findTask(id string) (toolapi.TaskInfo, bool) {
	for _, task := range p.tasks {
		if task.ID == id {
			return task, true
		}
	}
	return toolapi.TaskInfo{}, false
}

func (p *TasksPanel) openView(id string) {
	p.viewing = true
	p.viewID = id
	p.viewSeq = 0
	p.viewLines = nil
	if task, ok := p.findTask(id); ok {
		p.viewTask = task
	}
	p.vp.GotoTop()
	p.refreshView()
}

func (p *TasksPanel) refreshView() {
	if strings.TrimSpace(p.viewID) == "" {
		return
	}
	if task, ok := p.findTask(p.viewID); ok {
		p.viewTask = task
	}
	if p.viewTask.Kind != "shell_task" {
		lines := make([]string, 0, 8)
		if strings.TrimSpace(p.viewTask.Summary) != "" {
			lines = append(lines, p.viewTask.Summary)
		}
		if len(p.viewTask.Metadata) > 0 {
			lines = append(lines, "")
			for key, value := range p.viewTask.Metadata {
				lines = append(lines, fmt.Sprintf("%s: %v", key, value))
			}
		}
		if len(lines) == 0 {
			lines = append(lines, "(no details)")
		}
		p.viewLines = lines
		p.vp.SetContent(strings.Join(p.viewLines, "\n"))
		p.vp.GotoTop()
		return
	}
	res, err := bg.Default().Tail(p.viewID, &bg.TailOptions{FromSeq: p.viewSeq, Limit: 400})
	if err != nil {
		p.viewLines = []string{"Error: " + err.Error()}
		p.vp.SetContent(strings.Join(p.viewLines, "\n"))
		return
	}
	p.viewInfo = res.Info
	p.viewSeq = res.NextSeq
	for _, e := range res.Entries {
		line := e.Line
		if strings.TrimSpace(line) == "" {
			line = " "
		}
		p.viewLines = append(p.viewLines, fmt.Sprintf("[%s] %s", e.Stream, line))
	}
	if len(p.viewLines) > 5000 {
		p.viewLines = append([]string(nil), p.viewLines[len(p.viewLines)-5000:]...)
	}
	p.vp.SetContent(strings.Join(p.viewLines, "\n"))
	p.vp.GotoBottom()
}

func (p *TasksPanel) viewDetail() string {
	title := fmt.Sprintf("%s %s", i18n.T("tasks.detail", p.language), p.viewID)
	meta := ""
	if p.viewTask.Kind == "shell_task" {
		info := p.viewInfo
		status := string(info.Status)
		meta = fmt.Sprintf("%s: %s  PID: %d  %s: %s",
			i18n.T("tasks.meta.status", p.language), status,
			info.PID,
			i18n.T("tasks.meta.started", p.language), info.StartedAt.Format("2006-01-02 15:04:05"))
	} else {
		started := p.viewTask.StartedAt.Format("2006-01-02 15:04:05")
		meta = fmt.Sprintf("%s: %s  Kind: %s  %s: %s",
			i18n.T("tasks.meta.status", p.language), p.viewTask.Status,
			p.viewTask.Kind,
			i18n.T("tasks.meta.started", p.language), started)
	}

	var b strings.Builder
	b.WriteString(p.styles.TextInfo.Render(title))
	b.WriteString("\n")
	b.WriteString(p.styles.TextMuted.Render(meta))
	b.WriteString("\n\n")
	b.WriteString(p.vp.View())
	b.WriteString("\n\n")
	b.WriteString(p.styles.TextMuted.Render(i18n.T("tasks.detail_help", p.language)))
	return p.RenderBorder(b.String(), "")
}

type TaskKillRequestMsg struct {
	ID string
}

type TaskToastMsg struct {
	Text string
}
