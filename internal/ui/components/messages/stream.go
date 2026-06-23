package messages

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

// stream.go 提供文本流布局的渲染工具。
//
// 文本流布局参考 codex TUI（history_cell/messages.rs）：无边框、无背景填充、
// 无右对齐，仅以首行前缀 + 续行缩进表达一条消息，消息之间以空行分隔。
// 这比圆角气泡更贴合 CLI 的从上至下文本流视觉。

import (
	"fmt"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ui/styles"
)

// 文本流前缀。首行用语义前缀，续行统一两空格缩进。
const (
	streamContinueIndent = "  "
)

// prefixLines 给逻辑行的每一行加上首行/续行前缀。
// 对齐 codex 的 prefix_lines：第一行用 first，其余行用 rest。
func prefixLines(lines []string, first, rest string) []string {
	if len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	out = append(out, first+lines[0])
	for _, line := range lines[1:] {
		out = append(out, rest+line)
	}
	return out
}

// renderStreamLines 把已按宽度换行的逻辑行加上前缀，拼成单段文本。
// 空白行（纯空格）会被规整为空串，避免前缀污染空行。
func renderStreamLines(lines []string, firstPrefix, contPrefix string) string {
	prefixed := prefixLines(normalizeBlankLines(lines), firstPrefix, contPrefix)
	return strings.Join(prefixed, "\n")
}

// normalizeBlankLines 将纯空白行规整为空串，前缀只加在有内容的行上。
func normalizeBlankLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			out[i] = ""
			continue
		}
		out[i] = line
	}
	return out
}

// renderUserStream 渲染用户输入文本流。
// 视觉：首行 "› "（粗体 dim），续行 "  " 缩进。
func renderUserStream(s *styles.Styles, content string, width int) string {
	wrapW := streamContentWidth(width)
	first := s.StreamUserPrefix.Render("› ") + " "
	rest := streamContinueIndent
	lines := wrapText(content, wrapW)
	return renderStreamLines(lines, first, rest)
}

// renderAIStream 渲染 AI 输出文本流。
// 视觉：首行 "• "（dim），续行 "  " 缩进。PreStyled 内容（已含 ANSI）直接按行切分换行。
func renderAIStream(s *styles.Styles, content string, preStyled bool, width int) string {
	wrapW := streamContentWidth(width)
	first := s.StreamAIPrefix.Render("• ") + " "
	rest := streamContinueIndent

	var lines []string
	if preStyled {
		lines = splitAndWrapANSI(content, wrapW)
	} else {
		lines = wrapText(content, wrapW)
	}
	return renderStreamLines(lines, first, rest)
}

// renderMetaLine 渲染完成态元信息行（tokens · duration），dim 风格。
func renderMetaLine(s *styles.Styles, tokens int, duration time.Duration) string {
	var parts []string
	if tokens > 0 {
		parts = append(parts, fmt.Sprintf("%d tokens", tokens))
	}
	if duration > 0 {
		parts = append(parts, fmt.Sprintf("%.1fs", duration.Seconds()))
	}
	if len(parts) == 0 {
		return ""
	}
	return s.StreamMeta.Render("  " + strings.Join(parts, " · "))
}

// renderAgentHeader 渲染子 Agent 文本流头部行：
// "● [event] source -> name <id> ts"
func renderAgentHeader(s *styles.Styles, event, sourceName, agentName, agentID string, ts time.Time) string {
	dot := renderAgentEventDot(s, event, false, false)
	eventLabel := renderAgentEventLabel(s, firstNonEmptyString(strings.TrimSpace(event), "result"))
	route := renderAgentRoute(s, sourceName, agentName)
	id := renderAgentID(s, agentID)

	parts := []string{dot, eventLabel, route}
	if id != "" {
		parts = append(parts, id)
	}
	if !ts.IsZero() {
		parts = append(parts, s.StreamMeta.Render(ts.Format("15:04:05")))
	}
	return strings.Join(parts, " ")
}

// renderAgentTaskStream 渲染子 Agent dispatch 文本流：头部行 + 任务文本缩进。
func renderAgentTaskStream(s *styles.Styles, event, sourceName, agentName, agentID, task string, ts time.Time, width int) string {
	var b strings.Builder
	b.WriteString(renderAgentHeader(s, event, sourceName, agentName, agentID, ts))

	taskText := strings.TrimSpace(task)
	if taskText != "" {
		wrapW := streamContentWidth(width)
		lines := wrapText(taskText, wrapW)
		b.WriteString("\n")
		b.WriteString(renderStreamLines(lines, streamContinueIndent, streamContinueIndent))
	}
	return b.String()
}

// renderAgentFinalStream 渲染子 Agent 最终输出文本流：
// 头部行 + 内容缩进 + 可选元信息行。
func renderAgentFinalStream(s *styles.Styles, event, sourceName, agentName, agentID string, content string, preStyled bool, tokens int, duration time.Duration, done bool, ts time.Time, width int) string {
	var b strings.Builder
	b.WriteString(renderAgentHeader(s, event, sourceName, agentName, agentID, ts))

	if content != "" {
		wrapW := streamContentWidth(width)
		var lines []string
		if preStyled {
			lines = splitAndWrapANSI(content, wrapW)
		} else {
			lines = wrapText(content, wrapW)
		}
		b.WriteString("\n")
		b.WriteString(renderStreamLines(lines, streamContinueIndent, streamContinueIndent))
	}

	if done {
		if meta := renderMetaLine(s, tokens, duration); meta != "" {
			b.WriteString("\n")
			b.WriteString(meta)
		}
	}
	return b.String()
}

// streamContentWidth 计算文本流内容的可用换行宽度。
// 文本流每行有 2 列前缀/缩进，需预留；极窄终端下退化为可用宽度。
func streamContentWidth(width int) int {
	w := bubbleWidth(width) - 2
	if w < 10 {
		w = width - 2
	}
	if w < 1 {
		w = 1
	}
	return w
}
