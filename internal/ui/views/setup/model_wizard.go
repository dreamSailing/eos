package setup

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/ui/styles"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModelSetupStep 模型设置步骤
type ModelSetupStep int

const (
	ModelSetupStepProvider ModelSetupStep = iota
	ModelSetupStepModel
	ModelSetupStepConfig
	ModelSetupStepCustom
	ModelSetupStepComplete
)

// ModelSetupView 模型添加向导视图
type ModelSetupView struct {
	width            int
	height           int
	styles           *styles.Styles
	step             ModelSetupStep
	providers        []*ai.ProviderConfig
	providerTable    table.Model
	models           []*ai.ModelCatalogEntry
	modelTable       table.Model
	inputs           []textinput.Model
	focusIndex       int
	selectedProvider *ai.ProviderConfig
	selectedModel    *ai.ModelCatalogEntry
	providerFocused  bool
	modelFocused     bool
	customProvider   bool // 是否选择自定义服务商
	customModel      bool // 是否选择自定义模型
	apiBaseReadOnly  bool // API Base 是否只读
	modelReadOnly    bool // Model 名称是否只读
	language         string
}

// ModelSetupConfig 模型设置配置
type ModelSetupConfig struct {
	Name    string
	APIBase string
	APIKey  string
	Model   string
}

// NewModelSetupWizard 创建新的模型添加向导视图
func NewModelSetupWizard(styles *styles.Styles, lang string) *ModelSetupView {
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

	// 创建服务商表格
	providerTable := table.New(
		table.WithHeight(15),
		table.WithStyles(s),
		table.WithFocused(true),
	)

	// 创建模型表格
	modelTable := table.New(
		table.WithHeight(15),
		table.WithStyles(s),
		table.WithFocused(true),
	)

	// 创建输入框
	inputs := make([]textinput.Model, 4)
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Width = 50
	}
	inputs[1].EchoMode = textinput.EchoPassword // API Key

	v := &ModelSetupView{
		styles:          styles,
		step:            ModelSetupStepProvider,
		providers:       ai.GetAllProviders(),
		providerTable:   providerTable,
		modelTable:      modelTable,
		inputs:          inputs,
		focusIndex:      0,
		providerFocused: true,
		modelFocused:    false,
		customProvider:  false,
		customModel:     false,
		apiBaseReadOnly: false,
		modelReadOnly:   false,
		language:        lang,
	}

	slog.Info("NewModelSetupWizard", "inputs_len", len(inputs))

	v.updateTableColumns()
	v.loadProviders()
	return v
}

func (v *ModelSetupView) updateTableColumns() {
	// Provider Table
	providerColumns := []table.Column{
		{Title: i18n.T("setup.col.provider", v.language), Width: 18},
		{Title: i18n.T("setup.col.website", v.language), Width: 28},
		{Title: i18n.T("setup.col.env", v.language), Width: 18},
		{Title: i18n.T("setup.col.recommend", v.language), Width: 8},
	}
	v.providerTable.SetColumns(providerColumns)

	// Model Table
	modelColumns := []table.Column{
		{Title: i18n.T("setup.col.model", v.language), Width: 35},
		{Title: i18n.T("setup.col.context", v.language), Width: 12},
		{Title: i18n.T("setup.col.tags", v.language), Width: 30},
	}
	v.modelTable.SetColumns(modelColumns)
}

// loadProviders 加载服务商列表
func (v *ModelSetupView) loadProviders() {
	v.providers = ai.GetAllProviders() // 确保获取最新列表
	rows := make([]table.Row, 0, len(v.providers)+1)
	for _, p := range v.providers {
		recommended := ""
		if len(p.DefaultModels) > 0 {
			recommended = "★"
		}
		rows = append(rows, table.Row{p.Name, p.Website, p.APIKeyEnv, recommended})
	}
	// 添加"自定义"选项
	rows = append(rows, table.Row{i18n.T("setup.custom", v.language), "-", "-", ""})
	v.providerTable.SetRows(rows)
}

// loadModels 加载指定服务商的模型列表
func (v *ModelSetupView) loadModels(provider *ai.ProviderConfig) {
	if provider == nil {
		slog.Error("loadModels called with nil provider")
		return
	}
	v.models = ai.GetModelsByProvider(provider.Type)
	rows := make([]table.Row, 0, len(v.models))
	for _, m := range v.models {
		ctx := fmt.Sprintf("%d", m.ContextWindow)
		if m.ContextWindow >= 1000 {
			ctx = fmt.Sprintf("%.1fK", float64(m.ContextWindow)/1000)
		}
		tags := strings.Join(m.Tags, ", ")
		rows = append(rows, table.Row{m.Name, ctx, tags})
	}
	// 添加"自定义"选项
	rows = append(rows, table.Row{i18n.T("setup.custom", v.language), "-", "-"})
	v.modelTable.SetRows(rows)
	// 重置光标到首位
	v.modelTable.SetCursor(0)
	slog.Info("loadModels", "provider", provider.Name, "models_len", len(v.models), "inputs_len", len(v.inputs))
}

// SetSize 设置大小
func (v *ModelSetupView) SetSize(width, height int) {
	v.width = width
	v.height = height

	providerWidth := width - 12
	modelWidth := width - 12
	if providerWidth < 20 {
		providerWidth = 20
	}
	if modelWidth < 20 {
		modelWidth = 20
	}

	v.providerTable.SetWidth(providerWidth)
	v.modelTable.SetWidth(modelWidth)

	for i := range v.inputs {
		v.inputs[i].Width = width - 20
	}
}

// Init 初始化
func (v *ModelSetupView) Init() tea.Cmd {
	return textinput.Blink
}

// Update 更新
func (v *ModelSetupView) Update(msg tea.Msg) (*ModelSetupView, tea.Cmd) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("model_wizard.Update panic recovered",
				"step", v.step,
				"customProvider", v.customProvider,
				"customModel", v.customModel,
				"selectedProvider", v.selectedProvider,
				"selectedModel", v.selectedModel,
				"models_len", len(v.models),
				"inputs_len", len(v.inputs),
				"recover", r,
			)
		}
	}()

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			switch v.step {
			case ModelSetupStepProvider:
				return v, func() tea.Msg { return SetupCancelMsg{} }
			case ModelSetupStepModel:
				// 返回服务商选择
				v.step = ModelSetupStepProvider
				v.providerFocused = true
				v.modelFocused = false
				// 重置模型表格光标到首位
				v.modelTable.SetCursor(0)
				return v, nil
			case ModelSetupStepConfig:
				// 返回模型选择
				v.step = ModelSetupStepModel
				v.modelFocused = true
				v.providerFocused = false
				return v, nil
			case ModelSetupStepCustom:
				// 返回服务商选择
				v.step = ModelSetupStepProvider
				v.providerFocused = true
				v.modelFocused = false
				// 重置模型表格光标到首位
				v.modelTable.SetCursor(0)
				return v, nil
			}

		case "enter":
			switch v.step {
			case ModelSetupStepProvider:
				row := v.providerTable.Cursor()
				providers := ai.GetAllProviders()
				if row >= 0 && row < len(providers) {
					// 选择内置服务商
					v.selectedProvider = providers[row]
					v.step = ModelSetupStepModel
					v.loadModels(v.selectedProvider)
					v.providerFocused = false
					v.modelFocused = true
					v.customProvider = false
				} else if row == len(providers) {
					// 选择自定义服务商
					v.step = ModelSetupStepCustom
					v.customProvider = true
					v.customModel = false
					v.apiBaseReadOnly = false
					v.modelReadOnly = false
					if len(v.inputs) > 0 {
						v.inputs[0].Placeholder = i18n.T("setup.label.display_name", v.language)
						v.inputs[0].Focus()
					}
					if len(v.inputs) > 1 {
						v.inputs[1].Placeholder = i18n.T("setup.label.api_base", v.language)
					}
					if len(v.inputs) > 2 {
						v.inputs[2].Placeholder = i18n.T("setup.label.api_key", v.language)
					}
					if len(v.inputs) > 3 {
						v.inputs[3].Placeholder = i18n.T("setup.label.model_name", v.language)
					}
					return v, nil
				}

			case ModelSetupStepModel:
				row := v.modelTable.Cursor()
				slog.Debug("ModelSetupStepModel enter",
					"row", row,
					"models_len", len(v.models),
					"selectedProvider", v.selectedProvider,
				)
				if row >= 0 && row < len(v.models) {
					// 选择内置模型
					model := v.models[row]
					if model == nil {
						slog.Error("Selected model is nil", "row", row)
						return v, nil
					}
					v.selectedModel = model
					v.step = ModelSetupStepConfig
					v.modelFocused = false
					v.customModel = false
					v.apiBaseReadOnly = true
					v.modelReadOnly = true

					// 设置显示名称（默认用模型 ID）
					v.inputs[0].SetValue(v.selectedModel.ID)
					v.inputs[0].Placeholder = i18n.T("setup.label.display_name", v.language) + " (e.g., " + v.selectedModel.Name + ")"

					// 设置 API Base
					base := v.selectedProvider.DefaultAPIBase
					if v.selectedModel.APIType == ai.APITypeCodePlan && v.selectedProvider.CodePlanAPIBase != "" {
						base = v.selectedProvider.CodePlanAPIBase
					}
					v.inputs[1].SetValue(base)
					v.inputs[1].Placeholder = i18n.T("setup.label.api_base", v.language) + " (fixed)"

					v.inputs[2].SetValue("")
					v.inputs[2].Placeholder = v.selectedProvider.APIKeyEnv

					v.inputs[3].SetValue(v.selectedModel.ModelName)
					v.inputs[3].Placeholder = i18n.T("setup.label.model_name", v.language) + " (fixed)"

					v.inputs[0].Focus()
					v.focusIndex = 0
				} else if row == len(v.models) {
					// 选择自定义模型
					v.step = ModelSetupStepConfig
					v.customModel = true
					v.apiBaseReadOnly = true // API Base 固定为服务商的默认值
					v.modelReadOnly = false  // 模型名称可编辑

					// 设置显示名称
					v.inputs[0].SetValue("")
					v.inputs[0].Placeholder = i18n.T("setup.label.display_name", v.language)

					// 设置 API Base（固定为服务商默认值）
					base := v.selectedProvider.DefaultAPIBase
					v.inputs[1].SetValue(base)
					v.inputs[1].Placeholder = i18n.T("setup.label.api_base", v.language) + " (fixed)"

					v.inputs[2].SetValue("")
					v.inputs[2].Placeholder = v.selectedProvider.APIKeyEnv

					v.inputs[3].SetValue("")
					v.inputs[3].Placeholder = i18n.T("setup.label.model_name", v.language)

					v.inputs[0].Focus()
					v.focusIndex = 0
				}

			case ModelSetupStepConfig, ModelSetupStepCustom:
				// 保存配置
				return v, v.handleSave()
			}
		}
	}

	// 根据当前步骤更新相应的组件
	switch v.step {
	case ModelSetupStepProvider:
		if v.providerFocused {
			var cmd tea.Cmd
			v.providerTable, cmd = v.providerTable.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case ModelSetupStepModel:
		if v.modelFocused {
			var cmd tea.Cmd
			v.modelTable, cmd = v.modelTable.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case ModelSetupStepConfig, ModelSetupStepCustom:
		// 根据选择情况处理输入
		if v.customProvider {
			// 自定义服务商：所有字段都可编辑
			var cmd tea.Cmd
			v.inputs[v.focusIndex], cmd = v.inputs[v.focusIndex].Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

			// 处理 Tab 切换
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				switch keyMsg.String() {
				case "tab":
					v.inputs[v.focusIndex].Blur()
					v.focusIndex = (v.focusIndex + 1) % len(v.inputs)
					v.inputs[v.focusIndex].Focus()
				case "shift+tab":
					v.inputs[v.focusIndex].Blur()
					v.focusIndex--
					if v.focusIndex < 0 {
						v.focusIndex = len(v.inputs) - 1
					}
					v.inputs[v.focusIndex].Focus()
				}
			}
		} else if v.customModel {
			// 自定义模型：API Base 只读，其他可编辑
			// 如果当前聚焦在只读字段，跳过
			if v.focusIndex == 1 && v.apiBaseReadOnly {
				v.focusIndex = 0
				v.inputs[0].Focus()
			}
			var cmd tea.Cmd
			v.inputs[v.focusIndex], cmd = v.inputs[v.focusIndex].Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

			// 处理 Tab 切换（跳过只读字段）
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				switch keyMsg.String() {
				case "tab":
					v.inputs[v.focusIndex].Blur()
					v.focusIndex = (v.focusIndex + 1) % len(v.inputs)
					// 跳过 API Base (index 1)
					if v.focusIndex == 1 {
						v.focusIndex = (v.focusIndex + 1) % len(v.inputs)
					}
					v.inputs[v.focusIndex].Focus()
				case "shift+tab":
					v.inputs[v.focusIndex].Blur()
					v.focusIndex--
					if v.focusIndex < 0 {
						v.focusIndex = len(v.inputs) - 1
					}
					// 跳过 API Base (index 1)
					if v.focusIndex == 1 {
						v.focusIndex--
						if v.focusIndex < 0 {
							v.focusIndex = len(v.inputs) - 1
						}
					}
					v.inputs[v.focusIndex].Focus()
				}
			}
		} else {
			// 固定模型：只有名称和 API Key 可编辑
			// 确保 focusIndex 在可编辑范围内 (0 或 2)
			if v.focusIndex == 1 || v.focusIndex == 3 {
				v.focusIndex = 0
			}
			var cmd tea.Cmd
			v.inputs[v.focusIndex], cmd = v.inputs[v.focusIndex].Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

			// 处理 Tab 切换（只在可编辑字段间切换）
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				switch keyMsg.String() {
				case "tab":
					v.inputs[v.focusIndex].Blur()
					// 在 0 和 2 之间切换
					if v.focusIndex == 0 {
						v.focusIndex = 2
					} else {
						v.focusIndex = 0
					}
					v.inputs[v.focusIndex].Focus()
				case "shift+tab":
					v.inputs[v.focusIndex].Blur()
					// 在 0 和 2 之间切换
					if v.focusIndex == 0 {
						v.focusIndex = 2
					} else {
						v.focusIndex = 0
					}
					v.inputs[v.focusIndex].Focus()
				}
			}
		}
	}

	return v, tea.Batch(cmds...)
}

// handleSave 处理保存操作
func (v *ModelSetupView) handleSave() tea.Cmd {
	displayName := v.inputs[0].Value()
	apiBase := v.inputs[1].Value()
	apiKey := v.inputs[2].Value()
	model := v.inputs[3].Value()

	// 如果没有设置显示名称，使用默认值
	if displayName == "" {
		if v.customProvider {
			displayName = fmt.Sprintf("model-%d", time.Now().Unix()%100000)
		} else if v.customModel {
			// 自定义模型：使用模型名称作为显示名称
			displayName = model
			if displayName == "" {
				displayName = fmt.Sprintf("custom-model-%d", time.Now().Unix()%100000)
			}
		} else {
			// 固定模型：使用选中的模型名称
			if v.selectedModel != nil {
				displayName = v.selectedModel.Name
			} else {
				displayName = fmt.Sprintf("model-%d", time.Now().Unix()%100000)
			}
		}
	}

	// Provider 字段：内置服务商使用服务商 ID，自定义使用"custom"
	providerID := ""
	if v.customProvider {
		providerID = "custom"
	} else {
		providerID = v.selectedProvider.ID
	}

	return func() tea.Msg {
		return ModelFormCompleteMsg{
			Config: SetupConfig{
				Name:     displayName,
				Provider: providerID,
				APIBase:  apiBase,
				APIKey:   apiKey,
				Model:    model,
			},
			EditMode: false,
		}
	}
}

// View 渲染
func (v *ModelSetupView) View() string {
	if v.width == 0 {
		v.width = 80
	}
	if v.height == 0 {
		v.height = 24
	}

	var content strings.Builder

	// 标题
	var title string
	switch v.step {
	case ModelSetupStepProvider:
		title = i18n.T("setup.step.provider", v.language)
	case ModelSetupStepModel:
		if v.selectedProvider != nil {
			title = fmt.Sprintf(i18n.T("setup.step.model", v.language), v.selectedProvider.Name)
		} else {
			title = i18n.T("setup.step.provider", v.language)
		}
	case ModelSetupStepConfig:
		title = i18n.T("setup.step.config", v.language)
	case ModelSetupStepCustom:
		title = i18n.T("setup.step.custom", v.language)
	default:
		title = i18n.T("models.action.add", v.language)
	}

	titleStyle := v.styles.PanelTitle.
		Width(v.width - 12).
		Align(lipgloss.Center)
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// 根据步骤渲染内容
	switch v.step {
	case ModelSetupStepProvider:
		content.WriteString(v.styles.Text.Render(i18n.T("setup.provider.select", v.language)))
		content.WriteString("\n\n")
		content.WriteString(v.providerTable.View())

	case ModelSetupStepModel:
		if v.selectedProvider != nil {
			content.WriteString(v.styles.Text.Render(fmt.Sprintf(i18n.T("setup.model.select", v.language), v.selectedProvider.Name)))
			content.WriteString("\n\n")
			content.WriteString(v.modelTable.View())
		} else {
			content.WriteString(v.styles.Text.Render("Error: Provider not selected"))
		}

	case ModelSetupStepConfig, ModelSetupStepCustom:
		// 根据选择情况渲染不同的输入框
		if v.customProvider {
			// 自定义服务商：显示所有字段
			labels := []string{
				i18n.T("setup.label.display_name", v.language),
				i18n.T("setup.label.api_base", v.language),
				i18n.T("setup.label.api_key", v.language),
				i18n.T("setup.label.model_name", v.language),
			}
			for i, label := range labels {
				content.WriteString(v.styles.Text.Render(label))
				content.WriteString("\n")
				content.WriteString(v.inputs[i].View())
				content.WriteString("\n\n")
			}
		} else if v.customModel {
			// 选择服务商 + 自定义模型：显示名称、API Key、模型名（API Base 固定）
			content.WriteString(v.styles.Text.Render(i18n.T("setup.label.display_name", v.language)))
			content.WriteString("\n")
			content.WriteString(v.inputs[0].View())
			content.WriteString("\n\n")

			content.WriteString(v.styles.TextMuted.Render(i18n.T("setup.label.api_base", v.language) + " " + v.inputs[1].Value() + " (fixed)"))
			content.WriteString("\n\n")

			content.WriteString(v.styles.Text.Render(i18n.T("setup.label.api_key", v.language)))
			content.WriteString("\n")
			content.WriteString(v.inputs[2].View())
			content.WriteString("\n\n")

			content.WriteString(v.styles.Text.Render(i18n.T("setup.label.model_name", v.language)))
			content.WriteString("\n")
			content.WriteString(v.inputs[3].View())
			content.WriteString("\n\n")
		} else {
			// 选择服务商 + 固定模型：只显示名称和 API Key
			content.WriteString(v.styles.Text.Render(i18n.T("setup.label.display_name", v.language)))
			content.WriteString("\n")
			content.WriteString(v.inputs[0].View())
			content.WriteString("\n\n")

			content.WriteString(v.styles.TextMuted.Render(i18n.T("setup.label.api_base", v.language) + " " + v.inputs[1].Value() + " (fixed)"))
			content.WriteString("\n\n")

			content.WriteString(v.styles.Text.Render(i18n.T("setup.label.api_key", v.language)))
			content.WriteString("\n")
			content.WriteString(v.inputs[2].View())
			content.WriteString("\n\n")

			content.WriteString(v.styles.TextMuted.Render(i18n.T("setup.label.model_name", v.language) + " " + v.inputs[3].Value() + " (fixed)"))
			content.WriteString("\n\n")
		}
	}

	// 底部提示
	var hint string
	switch v.step {
	case ModelSetupStepProvider, ModelSetupStepModel:
		hint = i18n.T("setup.hint.provider", v.language)
	case ModelSetupStepConfig, ModelSetupStepCustom:
		if v.customProvider {
			hint = i18n.T("setup.hint.config", v.language)
		} else if v.customModel {
			hint = i18n.T("setup.hint.config", v.language)
		} else {
			hint = i18n.T("setup.hint.config", v.language)
		}
	}
	content.WriteString(v.styles.TextMuted.Render(hint))

	// 包装在面板样式中
	panelStyle := lipgloss.NewStyle().
		Width(v.width-4).
		Height(v.height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(v.styles.Theme.Primary).
		Padding(2, 4)

	return panelStyle.Render(content.String())
}
