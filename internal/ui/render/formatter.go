package render

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"fmt"
	"strings"
	"time"
)

// Formatter 格式化工具
type Formatter struct{}

// NewFormatter 创建新的格式化工具
func NewFormatter() *Formatter {
	return &Formatter{}
}

// FormatDuration 格式化持续时间
func (f *Formatter) FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}

// FormatBytes 格式化字节大小
func (f *Formatter) FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatNumber 格式化数字（添加千位分隔符）
func (f *Formatter) FormatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	var parts []string
	for n > 0 {
		parts = append([]string{fmt.Sprintf("%03d", n%1000)}, parts...)
		n /= 1000
	}

	// 第一个部分不需要前导零
	parts[0] = fmt.Sprintf("%d", atoi(parts[0]))
	return strings.Join(parts, ",")
}

// Truncate 截断文本
func (f *Formatter) Truncate(text string, maxLen int, suffix string) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= len(suffix) {
		return suffix
	}
	return text[:maxLen-len(suffix)] + suffix
}

// WrapText 包装文本到指定宽度
func (f *Formatter) WrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// 按单词分割并包装
		words := strings.Fields(line)
		currentLine := ""
		for _, word := range words {
			if len(currentLine)+len(word)+1 > width {
				result.WriteString(strings.TrimSpace(currentLine))
				result.WriteString("\n")
				currentLine = word + " "
			} else {
				currentLine += word + " "
			}
		}
		if currentLine != "" {
			result.WriteString(strings.TrimSpace(currentLine))
			result.WriteString("\n")
		}
	}

	return strings.TrimRight(result.String(), "\n")
}

// Indent 缩进文本
func (f *Formatter) Indent(text string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

// StripANSI 移除ANSI转义序列
func (f *Formatter) StripANSI(text string) string {
	// 简单的ANSI移除实现
	var result strings.Builder
	inEscape := false
	for _, r := range text {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// Center 居中文本
func (f *Formatter) Center(text string, width int) string {
	if len(text) >= width {
		return text
	}
	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text + strings.Repeat(" ", width-len(text)-padding)
}

// PadLeft 左填充
func (f *Formatter) PadLeft(text string, width int, pad rune) string {
	if len(text) >= width {
		return text
	}
	return strings.Repeat(string(pad), width-len(text)) + text
}

// PadRight 右填充
func (f *Formatter) PadRight(text string, width int, pad rune) string {
	if len(text) >= width {
		return text
	}
	return text + strings.Repeat(string(pad), width-len(text))
}

// atoi 辅助函数
func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
