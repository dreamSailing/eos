package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/memory"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ProjectContext 项目上下文信息，用于动态注入到系统提示词
type ProjectContext struct {
	Language     string   // 检测到的主要编程语言
	Framework    string   // 检测到的框架（如有）
	DirSummary   string   // 顶层目录结构摘要
	RecentFiles  []string // 最近修改的文件列表
	CustomPrompt string   // 用户自定义提示词片段
	CodingStyle  string   // 编码风格约定摘要
}

// maxCustomPromptBytes 用户自定义提示词最大字节数
const maxCustomPromptBytes = 6000

// maxCodingStyleBytes 编码风格文件最大字节数
const maxCodingStyleBytes = 6000

// maxRecentFiles 最近修改文件的最大数量
const maxRecentFiles = 10

// CollectProjectContext 收集当前项目的上下文信息
func CollectProjectContext() ProjectContext {
	ctx := ProjectContext{}

	cwd, err := os.Getwd()
	if err != nil {
		slog.Debug("prompt_context.collect.cwd.error", "error", err)
		return ctx
	}

	ctx.Language, ctx.Framework = detectProjectType(cwd)
	ctx.DirSummary = buildDirSummary(cwd)
	ctx.RecentFiles = CollectRecentFiles(cwd)
	ctx.CustomPrompt = LoadCustomPrompt(cwd)
	ctx.CodingStyle = loadCodingStyle(cwd)

	slog.Debug("prompt_context.collect.success",
		"language", ctx.Language,
		"framework", ctx.Framework,
		"recent_files", len(ctx.RecentFiles),
		"has_custom_prompt", ctx.CustomPrompt != "",
		"has_coding_style", ctx.CodingStyle != "")

	return ctx
}

// FormatProjectContext 将项目上下文格式化为 Markdown 字符串
func FormatProjectContext(ctx ProjectContext) string {
	var sb strings.Builder

	sb.WriteString("**项目信息**：\n")

	if ctx.Language != "" {
		sb.WriteString("- 主要语言: " + ctx.Language + "\n")
	}
	if ctx.Framework != "" {
		sb.WriteString("- 框架: " + ctx.Framework + "\n")
	}
	if ctx.DirSummary != "" {
		sb.WriteString("- 目录结构:\n```\n" + ctx.DirSummary + "```\n")
	}
	if len(ctx.RecentFiles) > 0 {
		sb.WriteString("- 最近修改的文件: " + strings.Join(ctx.RecentFiles, ", ") + "\n")
	}

	if ctx.CodingStyle != "" {
		sb.WriteString("\n**编码规范摘要**：\n" + ctx.CodingStyle + "\n")
	}

	return sb.String()
}

// LoadCustomPrompt 从项目根目录的 .eos/prompt.md 读取用户自定义提示词
func LoadCustomPrompt(rootDir string) string {
	promptPath := filepath.Join(rootDir, ".eos", "prompt.md")
	return readFileTruncated(promptPath, maxCustomPromptBytes)
}

// loadCodingStyle 加载项目规则/约定文件（优先 .eos/Rules.md，其次 ~/.eos/Rules.md）
func loadCodingStyle(rootDir string) string {
	home, _ := os.UserHomeDir()
	// 按优先级尝试加载
	candidates := []string{
		filepath.Join(rootDir, ".eos", "Rules.md"),
		filepath.Join(home, ".eos", "Rules.md"),
		filepath.Join(rootDir, "EOS.md"),
	}

	for _, path := range candidates {
		content := readFileTruncated(path, maxCodingStyleBytes)
		if content != "" {
			return content
		}
	}
	return ""
}

// LoadProjectMemory 读取项目长期记忆文件，与规则文件职责分离。
func LoadProjectMemory(rootDir string) string {
	return readFileTruncated(memory.ProjectMemoryPath(rootDir), maxCodingStyleBytes)
}

// LoadGlobalMemory 读取全局用户记忆文件。
func LoadGlobalMemory() string {
	return readFileTruncated(memory.GlobalMemoryPath(), maxCodingStyleBytes)
}

// CollectRecentFiles 通过 git 获取最近修改的文件列表
func CollectRecentFiles(rootDir string) []string {
	cmd := utils.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = rootDir

	out, err := cmd.Output()
	if err != nil {
		// 非 git 目录或无更改，尝试获取最近 commit 的文件
		cmd2 := utils.Command("git", "diff", "--name-only", "HEAD~1", "HEAD")
		cmd2.Dir = rootDir
		out, err = cmd2.Output()
		if err != nil {
			slog.Debug("prompt_context.recent_files.git.error", "error", err)
			return nil
		}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, line)
		if len(result) >= maxRecentFiles {
			break
		}
	}
	return result
}

// detectProjectType 通过项目签名文件检测项目类型
func detectProjectType(rootDir string) (language, framework string) {
	// Go 项目
	if fileExists(filepath.Join(rootDir, "go.mod")) {
		language = "Go"
		if fileExists(filepath.Join(rootDir, "go.sum")) {
			// 检测常见 Go 框架
			if content := readFileTruncated(filepath.Join(rootDir, "go.mod"), 1024); content != "" {
				if strings.Contains(content, "gin-gonic/gin") {
					framework = "Gin"
				} else if strings.Contains(content, "gofiber/fiber") {
					framework = "Fiber"
				} else if strings.Contains(content, "cloudwego/eino") {
					framework = "Eino"
				}
			}
		}
		return
	}

	// Node.js 项目
	if fileExists(filepath.Join(rootDir, "package.json")) {
		language = "JavaScript/TypeScript"
		if content := readFileTruncated(filepath.Join(rootDir, "package.json"), 1024); content != "" {
			if strings.Contains(content, "\"react\"") {
				framework = "React"
			} else if strings.Contains(content, "\"vue\"") {
				framework = "Vue"
			} else if strings.Contains(content, "\"next\"") {
				framework = "Next.js"
			} else if strings.Contains(content, "\"express\"") {
				framework = "Express"
			}
		}
		if fileExists(filepath.Join(rootDir, "tsconfig.json")) {
			language = "TypeScript"
		}
		return
	}

	// Python 项目
	if fileExists(filepath.Join(rootDir, "pyproject.toml")) || fileExists(filepath.Join(rootDir, "requirements.txt")) {
		language = "Python"
		if fileExists(filepath.Join(rootDir, "manage.py")) {
			framework = "Django"
		}
		return
	}

	// Rust 项目
	if fileExists(filepath.Join(rootDir, "Cargo.toml")) {
		language = "Rust"
		return
	}

	// Java 项目
	if fileExists(filepath.Join(rootDir, "pom.xml")) || fileExists(filepath.Join(rootDir, "build.gradle")) {
		language = "Java"
		return
	}

	return "", ""
}

// buildDirSummary 生成顶层目录结构摘要（仅第一层）
func buildDirSummary(rootDir string) string {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return ""
	}

	ignoreDirs := map[string]bool{
		".git": true, ".idea": true, ".vscode": true,
		"node_modules": true, "dist": true, "build": true,
		"vendor": true, "__pycache__": true, ".DS_Store": true,
		"bin": true, "obj": true, "target": true, ".eos": true,
	}

	var sb strings.Builder
	count := 0
	for _, e := range entries {
		name := e.Name()
		if ignoreDirs[name] {
			continue
		}
		if strings.HasPrefix(name, ".") && name != ".github" {
			continue
		}
		if e.IsDir() {
			sb.WriteString(name + "/\n")
		} else {
			sb.WriteString(name + "\n")
		}
		count++
		if count >= 20 {
			sb.WriteString("... (更多文件省略)\n")
			break
		}
	}
	return sb.String()
}

// readFileTruncated 读取文件内容，超过 maxBytes 时截断
func readFileTruncated(path string, maxBytes int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	if len(content) > maxBytes {
		content = content[:maxBytes] + "\n... (内容已截断)"
	}
	return strings.TrimSpace(content)
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
