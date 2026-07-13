package codectx

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"strings"
)

// InputProcessor 处理用户输入，提取文件提及并清理文本
type InputProcessor struct {
	// 可配置项
	workingDir string
}

// NewInputProcessor 创建输入处理器
func NewInputProcessor() *InputProcessor {
	wd, _ := os.Getwd()
	return &InputProcessor{workingDir: wd}
}

// ProcessInput 处理用户输入，返回 (mentions, cleanedText, ephemeralHint)
// mentions: 提取的文件路径列表
// cleanedText: 去掉 @ 符号后的清理文本
// ephemeralHint: 需要添加到上下文的临时提示（如 Mentions: files=[...]）
func (p *InputProcessor) ProcessInput(text string) (mentions []string, cleanedText string, ephemeralHint string) {
	mentions, cleanedText = p.extractMentionsAndClean(text)

	if len(mentions) > 0 {
		ephemeralHint = p.buildMentionsHint(mentions)
	}

	return mentions, cleanedText, ephemeralHint
}

// extractMentionsAndCleanText 提取文件提及并清理文本中的 @ 符号
func (p *InputProcessor) extractMentionsAndClean(text string) ([]string, string) {
	var mentions []string
	var cleaned strings.Builder

	parts := strings.Split(text, " ")
	for i, part := range parts {
		if i > 0 {
			cleaned.WriteString(" ")
		}

		idx := strings.Index(part, "@")
		if idx >= 0 {
			// 提取路径
			path := strings.TrimSpace(part[idx+1:])
			if path != "" {
				ap := filepath.Join(p.workingDir, filepath.FromSlash(path))
				rel, err := filepath.Rel(p.workingDir, ap)
				if err == nil && !strings.HasPrefix(rel, "..") {
					rel = filepath.ToSlash(rel)
					mentions = append(mentions, rel)
					// 写入清理后的文本（去掉 @）
					cleaned.WriteString(part[:idx] + rel)
					continue
				}
			}
		}
		cleaned.WriteString(part)
	}

	return mentions, cleaned.String()
}

// buildMentionsHint 构建提及提示信息
func (p *InputProcessor) buildMentionsHint(mentions []string) string {
	var parts []string
	var files []string
	var dirs []string

	for _, rel := range mentions {
		ap := filepath.Join(p.workingDir, filepath.FromSlash(rel))
		if fi, err := os.Stat(ap); err == nil {
			if fi.IsDir() {
				dirs = append(dirs, rel)
			} else {
				files = append(files, rel)
			}
		}
	}

	if len(files) > 0 {
		parts = append(parts, "files=["+strings.Join(files, ", ")+"]")
	}
	if len(dirs) > 0 {
		parts = append(parts, "dirs=["+strings.Join(dirs, ", ")+"]")
	}

	return "Mentions: " + strings.Join(parts, "; ")
}
