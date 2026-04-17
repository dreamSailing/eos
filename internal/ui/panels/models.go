package panels

import (
	"fmt"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/ui/styles"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModelsPanel 模型管理面板
type ModelsPanel struct {
	BasePanel
	styles       *styles.Styles
	table        table.Model
	models       []config.ModelEntry // 用户配置的模型列表
	currentModel string
	actionOps    []string
	actionIndex  int
	language     string
}

// NewModelsPanel 创建新的模型管理面板
func NewModelsPanel(styles *styles.Styles, lang string) *ModelsPanel {
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
		table.WithHeight(20),
		table.WithStyles(s),
		table.WithFocused(true),
	)

	// 设置表格的键位映射，确保上下键可用
	t.KeyMap.LineUp.SetKeys("up", "k")
	t.KeyMap.LineDown.SetKeys("down", "j")

	panel := &ModelsPanel{
		BasePanel:    NewBasePanel("models"),
		styles:       styles,
		table:        t,
		models:       make([]config.ModelEntry, 0),
		currentModel: "",
		actionOps:    []string{"Use", "Add", "Delete", "SyncEnv"},
		actionIndex:  0,
		language:     lang,
	}

	panel.updateTableColumns()
	// 加载模型列表
	panel.loadModels()

	return panel
}

func (p *ModelsPanel) updateTableColumns() {
	columns := []table.Column{
		{Title: i18n.T("models.col.name", p.language), Width: 25},
		{Title: i18n.T("models.col.source", p.language), Width: 10},
		{Title: i18n.T("models.col.base", p.language), Width: 35},
		{Title: i18n.T("models.col.model", p.language), Width: 20},
	}
	p.table.SetColumns(columns)
}

// loadModels 从配置中加载所有模型
func (p *ModelsPanel) loadModels() {
	cfg, _ := config.Load()
	p.models = cfg.Models
	p.currentModel = cfg.Active
	p.updateTable()
}

// SetCurrentModel 设置当前模型
func (p *ModelsPanel) SetCurrentModel(current string) {
	p.currentModel = current
	p.updateTable()
}

// updateTable 更新表格
func (p *ModelsPanel) updateTable() {
	rows := make([]table.Row, 0)

	if len(p.models) == 0 {
		rows = append(rows, table.Row{i18n.T("models.empty", p.language), "", "", ""})
	} else {
		for _, m := range p.models {
			status := ""
			if m.Name == p.currentModel {
				status = "* "
			}
			name := status + m.Name
			rows = append(rows, table.Row{name, m.Source, m.APIBase, m.Model})
		}
	}
	p.table.SetRows(rows)
}

// Refresh 刷新模型列表
func (p *ModelsPanel) Refresh() {
	p.loadModels()
}

// GetCurrentAction 获取当前选中的操作
func (p *ModelsPanel) GetCurrentAction() string {
	if p.actionIndex >= 0 && p.actionIndex < len(p.actionOps) {
		return p.actionOps[p.actionIndex]
	}
	return ""
}

// GetSelectedModel 获取选中的模型
func (p *ModelsPanel) GetSelectedModel() *config.ModelEntry {
	i := p.table.Cursor()
	if i >= 0 && i < len(p.models) {
		return &p.models[i]
	}
	return nil
}

// Init 初始化
func (p *ModelsPanel) Init() tea.Cmd {
	return nil
}

// Update 更新
func (p *ModelsPanel) Update(msg tea.Msg) (Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case LanguageChangeMsg:
		p.language = msg.Language
		p.updateTableColumns()
		p.updateTable()
		return p, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			// 向左切换操作
			if p.actionIndex > 0 {
				p.actionIndex--
			} else {
				p.actionIndex = len(p.actionOps) - 1
			}
			return p, nil
		case "right", "l":
			// 向右切换操作
			if p.actionIndex < len(p.actionOps)-1 {
				p.actionIndex++
			} else {
				p.actionIndex = 0
			}
			return p, nil
		case "enter":
			// 执行当前选中的操作
			action := p.GetCurrentAction()
			switch action {
			case "Use":
				// 设置当前模型
				if m := p.GetSelectedModel(); m != nil && m.Name != "" {
					return p, func() tea.Msg {
						return ModelSelectMsg{Name: m.Name}
					}
				}
			case "Add":
				// 添加模型
				return p, func() tea.Msg {
					return ModelAddMsg{}
				}
			case "Delete":
				// 删除模型
				if m := p.GetSelectedModel(); m != nil && m.Name != "" {
					return p, func() tea.Msg {
						return ModelDeleteMsg{Name: m.Name}
					}
				}
			case "SyncEnv":
				// 同步环境变量
				return p, func() tea.Msg {
					return ModelSyncMsg{}
				}
			}
		case "u", "U":
			// 直接执行使用操作
			if m := p.GetSelectedModel(); m != nil && m.Name != "" {
				return p, func() tea.Msg {
					return ModelSelectMsg{Name: m.Name}
				}
			}
		case "a", "A":
			// 直接执行新增操作
			return p, func() tea.Msg {
				return ModelAddMsg{}
			}
		case "d", "D":
			// 直接执行删除操作
			if m := p.GetSelectedModel(); m != nil && m.Name != "" {
				return p, func() tea.Msg {
					return ModelDeleteMsg{Name: m.Name}
				}
			}
		case "r", "R":
			// 刷新模型列表
			p.Refresh()
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.table, cmd = p.table.Update(msg)
	return p, cmd
}

// View 渲染
func (p *ModelsPanel) View() string {
	var content strings.Builder

	content.WriteString(p.styles.PanelTitle.Render(i18n.T("models.list.title", p.language)))
	content.WriteString("\n\n")

	if p.currentModel != "" {
		content.WriteString(fmt.Sprintf("%s %s\n\n",
			i18n.T("models.current", p.language),
			p.styles.TextSuccess.Render(p.currentModel)))
	}

	content.WriteString(p.table.View())
	content.WriteString("\n\n")

	// 显示当前选中的模型信息
	selected := p.GetSelectedModel()
	if selected != nil && selected.Name != "" {
		selectedText := fmt.Sprintf("%s [%s] %s (%s) - %s",
			i18n.T("models.selected", p.language),
			p.styles.TextInfo.Render(selected.Name),
			p.styles.TextMuted.Render(selected.Source),
			p.styles.TextMuted.Render(selected.APIBase),
			p.styles.TextMuted.Render(selected.Model))
		content.WriteString(selectedText)
		content.WriteString("\n\n")
	}

	// 显示操作列表
	var opStrs []string
	for i, op := range p.actionOps {
		key := ""
		switch op {
		case "Use":
			key = "models.action.use"
		case "Add":
			key = "models.action.add"
		case "Delete":
			key = "models.action.delete"
		case "SyncEnv":
			key = "models.action.sync_env"
		}
		text := i18n.T(key, p.language)
		if i == p.actionIndex {
			opStrs = append(opStrs, p.styles.TextSuccess.Render("["+text+"]"))
		} else {
			opStrs = append(opStrs, p.styles.TextMuted.Render(text))
		}
	}
	content.WriteString(fmt.Sprintf("%s %s\n\n",
		i18n.T("models.action", p.language),
		strings.Join(opStrs, "  ")))

	content.WriteString(p.styles.TextMuted.Render(i18n.T("models.help", p.language)))

	return p.RenderBorder(content.String(), "Models Panel")
}

// SetSize 设置大小
func (p *ModelsPanel) SetSize(width, height int) {
	p.BasePanel.SetSize(width, height)
	p.table.SetWidth(width - 4)
	p.table.SetHeight(height - 12)
}

// ModelSelectMsg 选择模型消息
type ModelSelectMsg struct {
	Name string
}

// ModelAddMsg 添加模型消息
type ModelAddMsg struct{}

// ModelDeleteMsg 删除模型消息
type ModelDeleteMsg struct {
	Name string
}

// ModelSyncMsg 同步模型消息
type ModelSyncMsg struct{}

// ModelFormMsg 添加/编辑模型表单消息
type ModelFormMsg struct {
	Name    string
	APIBase string
	APIKey  string
	Model   string
}
