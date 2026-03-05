package messages

import (
	"fmt"
	"strings"
	"time"

	"github.com/dreamSailing/vb-coding/internal/ui/render"
	"github.com/dreamSailing/vb-coding/internal/ui/styles"

	"github.com/charmbracelet/lipgloss"
)

// Renderer 消息渲染器
type Renderer struct {
	styles *styles.Styles
	width  int
	agentNameMap map[string]string
	mainAgent    string
	mainLabel    string
	subLabel     string
	mdEnabled    bool
	md           *render.MarkdownRenderer
}

// NewRenderer 创建新的消息渲染器
func NewRenderer(s *styles.Styles, width int) *Renderer {
	r := &Renderer{
		styles:       s,
		width:        width,
		agentNameMap: map[string]string{},
		mainAgent:    "Assistant",
		mainLabel:    "",
		subLabel:     "",
		mdEnabled:    true,
		md:           render.NewMarkdownRenderer(width),
	}
	r.md.SetStyles(render.NewThemeRenderStyles(s))
	return r
}

// SetWidth 设置渲染宽度
func (r *Renderer) SetWidth(width int) {
	r.width = width
	if r.md != nil {
		r.md.SetWidth(width)
	}
}

func (r *Renderer) SetAgentNameMap(m map[string]string) {
	r.agentNameMap = map[string]string{}
	for k, v := range m {
		r.agentNameMap[k] = v
	}
}

func (r *Renderer) SetAgentLabels(mainLabel, subLabel string) {
	r.mainLabel = mainLabel
	r.subLabel = subLabel
}

func (r *Renderer) SetMainAgentName(name string) {
	if name != "" {
		r.mainAgent = name
	}
}

func (r *Renderer) EnableMarkdown(v bool) {
	r.mdEnabled = v
}

func (r *Renderer) displayAgentName(raw string) string {
	if raw == "" {
		return raw
	}
	if v, ok := r.agentNameMap[raw]; ok && v != "" {
		return v
	}
	return raw
}

func (r *Renderer) maybeRenderMarkdown(content string, done bool) string {
	if !done || !r.mdEnabled || r.md == nil {
		return content
	}
	return r.md.Render(content)
}

func (r *Renderer) RenderUserInputAt(content string, ts time.Time) string {
	msg := &UserMessage{
		Content:   content,
		Timestamp: ts,
	}
	return msg.Render(r.styles, r.width)
}

// RenderUserInput 渲染用户输入
func (r *Renderer) RenderUserInput(content string) string {
	return r.RenderUserInputAt(content, time.Now())
}

func (r *Renderer) RenderAIResponseAt(content string, tokens int, duration time.Duration, done bool, ts time.Time) string {
	msg := &AgentBubbleMessage{
		Name:      r.mainAgent,
		Label:     r.mainLabel,
		IsMain:    true,
		PreStyled: done && r.mdEnabled,
		Content:   r.maybeRenderMarkdown(content, done),
		Timestamp: ts,
		Tokens:    tokens,
		Duration:  duration,
		Done:      done,
	}
	return msg.Render(r.styles, r.width)
}

func (r *Renderer) RenderAIResponseAtWithCopy(content string, tokens int, duration time.Duration, done bool, ts time.Time, copyLabel string) string {
	msg := &AgentBubbleMessage{
		Name:      r.mainAgent,
		Label:     r.mainLabel,
		IsMain:    true,
		PreStyled: done && r.mdEnabled,
		Content:   r.maybeRenderMarkdown(content, done),
		Timestamp: ts,
		Tokens:    tokens,
		Duration:  duration,
		Done:      done,
		CopyLabel: copyLabel,
	}
	return msg.Render(r.styles, r.width)
}

// RenderAIResponse 渲染AI响应
func (r *Renderer) RenderAIResponse(content string, tokens int, duration time.Duration, done bool) string {
	return r.RenderAIResponseAt(content, tokens, duration, done, time.Now())
}

// RenderToolCall 渲染工具调用
func (r *Renderer) RenderToolCall(name string, params map[string]any) string {
	msg := &ToolCallMessage{
		Name:      name,
		Params:    params,
		Status:    "running",
		Timestamp: time.Now(),
	}
	return msg.Render(r.styles, r.width)
}

func (r *Renderer) RenderToolEvent(name string, params map[string]any, status string, result string, duration time.Duration) string {
	msg := &ToolCallMessage{
		Name:     name,
		Params:   params,
		Status:   status,
		Result:   result,
		Duration: duration,
	}
	return msg.Render(r.styles, r.width)
}

// RenderToolResult 渲染工具结果
func (r *Renderer) RenderToolResult(name string, result string, success bool, duration time.Duration) string {
	status := "success"
	if !success {
		status = "error"
	}
	msg := &ToolCallMessage{
		Name:     name,
		Status:   status,
		Result:   result,
		Duration: duration,
	}
	return msg.Render(r.styles, r.width)
}

// RenderAgentTask 渲染子Agent任务（蓝色圆点 - 调度）
func (r *Renderer) RenderAgentTask(name, task, goal string, progress, step, totalSteps int, status string, duration time.Duration, results []string) string {
	return r.RenderAgentTaskAt(name, task, time.Now())
}

func (r *Renderer) RenderAgentTaskAt(name, task string, ts time.Time) string {
	msg := &AgentDispatchMessage{
		AgentName:  r.displayAgentName(name),
		Task:       task,
		Timestamp:  ts,
	}
	return msg.Render(r.styles, r.width)
}

// RenderAgentFinal 渲染子Agent最终结果（绿色圆点）
func (r *Renderer) RenderAgentFinal(agentName, content string) string {
	return r.RenderAgentFinalAt(agentName, content, time.Now())
}

func (r *Renderer) RenderAgentFinalAt(agentName, content string, ts time.Time) string {
	msg := &AgentBubbleMessage{
		Name:      r.displayAgentName(agentName),
		Label:     r.subLabel,
		IsMain:    false,
		PreStyled: r.mdEnabled,
		Content:   r.maybeRenderMarkdown(content, true),
		Timestamp: ts,
		Done:      true,
	}
	return msg.Render(r.styles, r.width)
}

func (r *Renderer) RenderAgentFinalAtWithCopy(agentName, content string, ts time.Time, copyLabel string) string {
	msg := &AgentBubbleMessage{
		Name:      r.displayAgentName(agentName),
		Label:     r.subLabel,
		IsMain:    false,
		PreStyled: r.mdEnabled,
		Content:   r.maybeRenderMarkdown(content, true),
		Timestamp: ts,
		Done:      true,
		CopyLabel: copyLabel,
	}
	return msg.Render(r.styles, r.width)
}

// RenderPlan 渲染计划
func (r *Renderer) RenderPlan(title, description string, steps []PlanStep) string {
	msg := &PlanMessage{
		Title:       title,
		Description: description,
		Steps:       steps,
		Status:      "pending",
	}
	return msg.Render(r.styles, r.width)
}

// RenderThinking 渲染思考过程
func (r *Renderer) RenderThinking(content string, duration time.Duration, expanded bool, steps []ThinkingStep) string {
	msg := &ThinkingMessage{
		Content:  content,
		Duration: duration,
		Expanded: expanded,
		Steps:    steps,
	}
	return msg.Render(r.styles, r.width)
}

// RenderSystem 渲染系统消息
func (r *Renderer) RenderSystem(content, level string) string {
	msg := &SystemMessage{
		Content: content,
		Level:   level,
	}
	return msg.Render(r.styles, r.width)
}

// MessageBuilder 消息构建器，用于构建复杂消息
type MessageBuilder struct {
	lines []string
	style lipgloss.Style
}

// NewMessageBuilder 创建新的消息构建器
func NewMessageBuilder(style lipgloss.Style) *MessageBuilder {
	return &MessageBuilder{
		lines: []string{},
		style: style,
	}
}

// AddLine 添加一行
func (b *MessageBuilder) AddLine(line string) *MessageBuilder {
	b.lines = append(b.lines, line)
	return b
}

// AddFormatted 添加格式化行
func (b *MessageBuilder) AddFormatted(format string, args ...any) *MessageBuilder {
	b.lines = append(b.lines, lipgloss.NewStyle().Render(fmt.Sprintf(format, args...)))
	return b
}

// AddIndented 添加缩进行
func (b *MessageBuilder) AddIndented(indent int, content string) *MessageBuilder {
	prefix := strings.Repeat("  ", indent)
	b.lines = append(b.lines, prefix+content)
	return b
}

// AddSeparator 添加分隔线
func (b *MessageBuilder) AddSeparator(width int) *MessageBuilder {
	sep := strings.Repeat("─", width)
	b.lines = append(b.lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b")).Render(sep))
	return b
}

// Build 构建消息
func (b *MessageBuilder) Build() string {
	return b.style.Render(strings.Join(b.lines, "\n"))
}

// RenderSimpleUser 渲染简单用户消息（用于快速显示）
func (r *Renderer) RenderSimpleUser(content string) string {
	prefix := r.styles.MsgUserPrefix.Render("▶")
	return r.styles.MsgUserBorder.Render(prefix + " " + content)
}

// RenderSimpleAI 渲染简单AI消息（用于流式输出）
func (r *Renderer) RenderSimpleAI(content string) string {
	header := r.styles.MsgAIHeader.Render("🤖 Assistant")
	return header + "\n" + r.styles.MsgAI.Render(content)
}

// RenderSimpleTool 渲染简单工具消息
func (r *Renderer) RenderSimpleTool(name, status string) string {
	var icon string
	switch status {
	case "running":
		icon = "⏳"
	case "success":
		icon = "✓"
	case "error":
		icon = "✗"
	default:
		icon = "🔧"
	}

	return r.styles.MsgToolHeader.Render(fmt.Sprintf("%s %s", icon, name))
}

// RenderTokenStats 渲染Token统计
func (r *Renderer) RenderTokenStats(input, output, total int) string {
	var result strings.Builder

	result.WriteString(r.styles.TextMuted.Render("📊 Token Usage\n"))
	result.WriteString(r.styles.TextMuted.Render(strings.Repeat("─", 30)) + "\n")

	if total > 0 {
		inputPct := (input * 100) / total
		outputPct := (output * 100) / total

		inputBar := renderMiniBar(inputPct, 15, r.styles.TextInfo)
		outputBar := renderMiniBar(outputPct, 15, r.styles.TextSuccess)

		result.WriteString(fmt.Sprintf("Input:  %s %d (%d%%)\n", inputBar, input, inputPct))
		result.WriteString(fmt.Sprintf("Output: %s %d (%d%%)\n", outputBar, output, outputPct))
		result.WriteString(fmt.Sprintf("Total:  %d tokens\n", total))
	}

	return result.String()
}

// 辅助函数

func renderMiniBar(percentage, width int, style lipgloss.Style) string {
	if width < 2 {
		return ""
	}

	filled := (percentage * width) / 100
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	empty := width - filled

	return style.Render(strings.Repeat("█", filled)) + lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b")).Render(strings.Repeat("░", empty))
}
