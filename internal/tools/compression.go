package tools

import (
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

const (
	// ToolOutputMaxSize 工具输出最大字符数（软限制）
	ToolOutputMaxSize = 8000

	// ToolCompressionThreshold 内容压缩阈值（字符数）
	ToolCompressionThreshold = 4096

	// ToolCompressionSummaryLength 压缩后的摘要长度
	ToolCompressionSummaryLength = 800
)

// ContentInfo 内容信息
type ContentInfo struct {
	OriginalLength int      // 原始长度
	Compressed     bool     // 是否已压缩
	LineCount      int      // 行数
	Language       string   // 检测到的编程语言
	Preview        string   // 预览内容（前 N 行）
	Structure      []string // 结构信息（如函数、类等）
}

// CompressToolOutput 压缩工具输出内容
// 如果内容超过阈值，返回压缩后的内容和标记
func CompressToolOutput(content string, compressType string) (string, bool) {
	if len(content) <= ToolCompressionThreshold {
		return content, false
	}

	slog.Debug("tools.compress.start",
		"component", utils.ComponentTool,
		"original_length", len(content),
		"type", compressType,
	)

	switch compressType {
	case "file":
		return compressFileContent(content), true
	case "diff":
		return compressDiffContent(content), true
	case "search":
		return compressSearchResults(content), true
	default:
		return compressGenericContent(content), true
	}
}

// compressFileContent 压缩文件内容
// 保留文件头部、尾部和结构信息
func compressFileContent(content string) string {
	lines := strings.Split(content, "\n")
	lineCount := len(lines)

	// 对于小文件，直接返回前 N 行
	if lineCount <= 100 {
		if len(content) <= ToolCompressionSummaryLength {
			return content
		}
		return content[:ToolCompressionSummaryLength] + "\n... (content truncated)"
	}

	// 保留头部 30 行
	headerLines := 30
	if lineCount < headerLines*2 {
		headerLines = lineCount / 2
	}

	// 保留尾部 20 行
	footerLines := 20
	if lineCount < headerLines+footerLines {
		footerLines = lineCount - headerLines
	}

	var result strings.Builder

	// 添加头部
	for i := 0; i < headerLines && i < lineCount; i++ {
		result.WriteString(lines[i])
		result.WriteByte('\n')
	}

	// 添加省略标记
	result.WriteString(fmt.Sprintf("\n... [%d lines omitted, total %d lines] ...\n\n",
		lineCount-headerLines-footerLines, lineCount))

	// 添加尾部
	startIdx := lineCount - footerLines
	if startIdx < headerLines {
		startIdx = headerLines
	}
	for i := startIdx; i < lineCount; i++ {
		result.WriteString(lines[i])
		if i < lineCount-1 {
			result.WriteByte('\n')
		}
	}

	return result.String()
}

// compressDiffContent 压缩 diff 内容
// diff 通常较长，重点保留变更行
func compressDiffContent(content string) string {
	lines := strings.Split(content, "\n")
	var compressed []string
	var addedCount, removedCount, contextCount int

	for _, line := range lines {
		// 保留所有变更行
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			compressed = append(compressed, line)
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "++") {
				addedCount++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				removedCount++
			}
			continue
		}

		// 保留部分上下文（每 3 行上下文保留 1 行）
		if strings.HasPrefix(line, "@@") {
			compressed = append(compressed, line)
			contextCount++
			continue
		}

		// 其他上下文行，限制数量
		if contextCount < 50 {
			compressed = append(compressed, line)
			contextCount++
		}
	}

	result := strings.Join(compressed, "\n")
	if len(compressed) < len(lines) {
		result += fmt.Sprintf("\n... [diff compressed: %d → %d lines, +%d/-%d changes shown] ...",
			len(lines), len(compressed), addedCount, removedCount)
	}

	return result
}

// compressSearchResults 压缩搜索结果
// 保留匹配项，限制总数量
func compressSearchResults(content string) string {
	lines := strings.Split(content, "\n")
	const maxResults = 50
	const maxLineLength = 200

	if len(lines) <= maxResults {
		// 即使行数不多，也要检查每行长度
		var compressed []string
		for _, line := range lines {
			if len(line) > maxLineLength {
				compressed = append(compressed, line[:maxLineLength]+"...")
			} else {
				compressed = append(compressed, line)
			}
		}
		result := strings.Join(compressed, "\n")
		if len(compressed) < len(lines) {
			result += fmt.Sprintf("\n... [%d results shown] ...", len(compressed))
		}
		return result
	}

	// 保留前 maxResults 条结果
	compressed := lines[:maxResults]
	result := strings.Join(compressed, "\n")
	result += fmt.Sprintf("\n... [%d more results omitted] ...", len(lines)-maxResults)

	return result
}

// compressGenericContent 通用内容压缩
// 简单截断，保留开头和结尾
func compressGenericContent(content string) string {
	if len(content) <= ToolCompressionSummaryLength {
		return content
	}

	// 保留前 60% 和后 20%
	splitPoint := len(content) * 3 / 5
	if splitPoint > ToolCompressionSummaryLength-200 {
		splitPoint = ToolCompressionSummaryLength - 200
	}

	result := content[:splitPoint]
	result += "\n\n... [content truncated: "
	result += fmt.Sprintf("%d → %d characters", len(content), splitPoint+200)
	result += "] ...\n\n"

	// 添加尾部 200 字符
	if len(content) > 200 {
		result += content[len(content)-200:]
	}

	return result
}

// AnalyzeContent 分析内容结构
func AnalyzeContent(content, fileType string) ContentInfo {
	info := ContentInfo{
		OriginalLength: len(content),
		LineCount:      strings.Count(content, "\n") + 1,
		Language:       detectLanguage(fileType),
	}

	lines := strings.Split(content, "\n")

	// 生成预览（前 10 行）
	previewLines := 10
	if len(lines) < previewLines {
		previewLines = len(lines)
	}
	info.Preview = strings.Join(lines[:previewLines], "\n")

	// 分析代码结构
	info.Structure = extractStructure(lines, info.Language)

	return info
}

// detectLanguage 根据文件扩展名检测编程语言
func detectLanguage(filename string) string {
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		ext := filename[idx:]
		switch ext {
		case ".go":
			return "go"
		case ".js", ".jsx":
			return "javascript"
		case ".ts", ".tsx":
			return "typescript"
		case ".py":
			return "python"
		case ".java":
			return "java"
		case ".c", ".h":
			return "c"
		case ".cpp", ".hpp", ".cc":
			return "cpp"
		case ".rs":
			return "rust"
		case ".rb":
			return "ruby"
		case ".php":
			return "php"
		case ".cs":
			return "csharp"
		case ".swift":
			return "swift"
		case ".kt":
			return "kotlin"
		case ".lua":
			return "lua"
		case ".sh":
			return "shell"
		case ".json":
			return "json"
		case ".yaml", ".yml":
			return "yaml"
		case ".xml":
			return "xml"
		case ".html":
			return "html"
		case ".css":
			return "css"
		case ".md":
			return "markdown"
		}
	}
	return "unknown"
}

// extractStructure 从代码中提取结构信息（函数、类等）
func extractStructure(lines []string, language string) []string {
	var structure []string

	switch language {
	case "go":
		structure = extractGoStructure(lines)
	case "python":
		structure = extractPythonStructure(lines)
	case "javascript", "typescript":
		structure = extractJSStructure(lines)
	case "java":
		structure = extractJavaStructure(lines)
	}

	return structure
}

// extractGoStructure 提取 Go 代码结构
func extractGoStructure(lines []string) []string {
	var structure []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 匹配 func 关键字
		if strings.HasPrefix(trimmed, "func ") {
			// 提取函数名
			if idx := strings.Index(trimmed, "("); idx > 5 {
				name := trimmed[5:idx]
				name = strings.TrimSpace(name)
				// 处理接收器
				if idx := strings.Index(name, " "); idx > 0 {
					name = name[idx+1:]
				}
				structure = append(structure, "func: "+name)
			}
		}
		// 匹配 type 定义
		if strings.HasPrefix(trimmed, "type ") {
			if idx := strings.Index(trimmed, " "); idx > 5 {
				rest := trimmed[idx+1:]
				if idx := strings.Index(rest, " "); idx > 0 {
					name := rest[:idx]
					kind := rest[idx+1:]
					if strings.Contains(kind, "struct") {
						structure = append(structure, "type: "+name+" struct")
					} else if strings.Contains(kind, "interface") {
						structure = append(structure, "type: "+name+" interface")
					}
				}
			}
		}
	}
	return structure
}

// extractPythonStructure 提取 Python 代码结构
func extractPythonStructure(lines []string) []string {
	var structure []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 匹配 class 和 def
		if strings.HasPrefix(trimmed, "class ") {
			if idx := strings.Index(trimmed, "("); idx > 6 {
				name := trimmed[6:idx]
				structure = append(structure, "class: "+name)
			} else if idx := strings.Index(trimmed, ":"); idx > 6 {
				name := trimmed[6:idx]
				structure = append(structure, "class: "+name)
			}
		}
		if strings.HasPrefix(trimmed, "def ") {
			if idx := strings.Index(trimmed, "("); idx > 4 {
				name := trimmed[4:idx]
				structure = append(structure, "def: "+name)
			} else if idx := strings.Index(trimmed, ":"); idx > 4 {
				name := trimmed[4:idx]
				structure = append(structure, "def: "+name)
			}
		}
	}
	return structure
}

// extractJSStructure 提取 JavaScript/TypeScript 代码结构
func extractJSStructure(lines []string) []string {
	var structure []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 匹配 function, class, const arrow functions
		if strings.HasPrefix(trimmed, "function ") {
			if idx := strings.Index(trimmed, "("); idx > 9 {
				name := trimmed[9:idx]
				structure = append(structure, "function: "+name)
			}
		}
		if strings.HasPrefix(trimmed, "class ") {
			if idx := strings.IndexAny(trimmed, " {"); idx > 6 {
				name := trimmed[6:idx]
				structure = append(structure, "class: "+name)
			}
		}
		// 匹配 export function, export class
		if strings.HasPrefix(trimmed, "export function ") {
			if idx := strings.Index(trimmed, "("); idx > 16 {
				name := trimmed[16:idx]
				structure = append(structure, "function: "+name)
			}
		}
		if strings.HasPrefix(trimmed, "export class ") {
			if idx := strings.IndexAny(trimmed, " {"); idx > 13 {
				name := trimmed[13:idx]
				structure = append(structure, "class: "+name)
			}
		}
	}
	return structure
}

// extractJavaStructure 提取 Java 代码结构
func extractJavaStructure(lines []string) []string {
	var structure []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 匹配 class, interface, enum
		if strings.Contains(trimmed, "class ") {
			if idx := strings.Index(trimmed, "class "); idx >= 0 {
				rest := trimmed[idx+6:]
				if idx := strings.IndexAny(rest, " {<"); idx > 0 {
					name := rest[:idx]
					structure = append(structure, "class: "+name)
				}
			}
		}
		if strings.Contains(trimmed, "interface ") {
			if idx := strings.Index(trimmed, "interface "); idx >= 0 {
				rest := trimmed[idx+10:]
				if idx := strings.IndexAny(rest, " {<"); idx > 0 {
					name := rest[:idx]
					structure = append(structure, "interface: "+name)
				}
			}
		}
	}
	return structure
}

// IsBinaryContent 检查内容是否为二进制
func IsBinaryContent(content string) bool {
	for _, r := range content {
		if r == 0 {
			return true
		}
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) && r != '\t' && r != '\n' && r != '\r' {
			// 如果非打印字符过多，可能是二进制
			return true
		}
	}
	return false
}

// FormatCompressedOutput 格式化压缩后的输出，添加标记
func FormatCompressedOutput(original, compressed string, compressType string) string {
	var info ContentInfo
	if compressType == "file" {
		info = AnalyzeContent(original, "")
	}

	var result strings.Builder
	result.WriteString("[Content compressed: ")
	result.WriteString(fmt.Sprintf("%d → %d characters", len(original), len(compressed)))

	if info.LineCount > 0 {
		result.WriteString(fmt.Sprintf(", %d lines", info.LineCount))
	}

	if len(info.Structure) > 0 && len(info.Structure) <= 20 {
		result.WriteString("]\n\nStructure:\n")
		for _, s := range info.Structure {
			result.WriteString("  - ")
			result.WriteString(s)
			result.WriteByte('\n')
		}
	} else {
		result.WriteString("]")
	}

	result.WriteString("\n\n")
	result.WriteString(compressed)

	return result.String()
}

// ShouldCompress 判断内容是否需要压缩
func ShouldCompress(content string) bool {
	return len(content) > ToolCompressionThreshold
}

// TruncateOutput 截断输出到指定长度
func TruncateOutput(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	if maxLength < 50 {
		return content[:maxLength] + "..."
	}

	// 保留前 80% 和后 20%
	splitPoint := maxLength * 4 / 5
	result := content[:splitPoint]
	result += "\n\n... ["
	result += fmt.Sprintf("%d characters truncated", len(content)-maxLength)
	result += "] ...\n\n"
	result += content[len(content)-(maxLength-splitPoint):]

	return result
}
