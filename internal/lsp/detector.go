//go:build !without_lsp

package lsp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"github.com/eosaios/eos/internal/pkg/utils"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LanguageType 语言类型
type LanguageType string

const (
	LanguageGo         LanguageType = "go"
	LanguagePython     LanguageType = "python"
	LanguageTypeScript LanguageType = "typescript"
	LanguageJavaScript LanguageType = "javascript"
)

// ServerInfo 语言服务器信息
type ServerInfo struct {
	Language LanguageType
	Command  string
	Args     []string
}

// Detector 语言检测器
type Detector struct {
	mu              sync.RWMutex
	enabledServers  map[LanguageType]bool
	serverCommands  map[LanguageType]string
	embeddedServers map[LanguageType]bool
}

// NewDetector 创建检测器
func NewDetector() *Detector {
	return &Detector{
		enabledServers:  make(map[LanguageType]bool),
		serverCommands:  make(map[LanguageType]string),
		embeddedServers: make(map[LanguageType]bool),
	}
}

// SetEmbedded 设置内置服务器
func (d *Detector) SetEmbedded(lang LanguageType) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.embeddedServers[lang] = true
}

// DetectLanguage 检测项目语言
func (d *Detector) DetectLanguage(rootPath string) LanguageType {
	// 检查 go.mod
	if _, err := os.Stat(filepath.Join(rootPath, "go.mod")); err == nil {
		return LanguageGo
	}

	// 检查 pyproject.toml, setup.py, requirements.txt
	for _, name := range []string{"pyproject.toml", "setup.py", "requirements.txt", "setup.cfg"} {
		if _, err := os.Stat(filepath.Join(rootPath, name)); err == nil {
			return LanguagePython
		}
	}

	// 检查 package.json, tsconfig.json
	for _, name := range []string{"package.json", "tsconfig.json"} {
		if _, err := os.Stat(filepath.Join(rootPath, name)); err == nil {
			if _, err := os.Stat(filepath.Join(rootPath, "tsconfig.json")); err == nil {
				return LanguageTypeScript
			}
			return LanguageJavaScript
		}
	}

	// 检查文件扩展名统计
	goCount := 0
	pyCount := 0
	tsCount := 0
	jsCount := 0

	_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// 跳过隐藏目录与依赖目录（按目录名判断——按路径子串匹配
			// 在 Windows 上因分隔符不同而失效）。
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		switch filepath.Ext(path) {
		case ".go":
			goCount++
		case ".py":
			pyCount++
		case ".ts":
			tsCount++
		case ".js":
			jsCount++
		}

		return nil
	})

	maxCount := goCount
	lang := LanguageGo
	if pyCount > maxCount {
		maxCount = pyCount
		lang = LanguagePython
	}
	if tsCount > maxCount {
		maxCount = tsCount
		lang = LanguageTypeScript
	}
	if jsCount > maxCount {
		maxCount = jsCount
		lang = LanguageJavaScript
	}

	if maxCount > 0 {
		return lang
	}

	return ""
}

// FindServer 查找语言服务器
func (d *Detector) FindServer(lang LanguageType) (*ServerInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 检查是否有内置服务器
	if d.embeddedServers[lang] {
		return d.getEmbeddedServer(lang)
	}

	// 查找外部服务器
	return d.findExternalServer(lang)
}

// findExternalServer 查找外部服务器
func (d *Detector) findExternalServer(lang LanguageType) (*ServerInfo, error) {
	switch lang {
	case LanguageGo:
		return d.findGoServer()
	case LanguagePython:
		return d.findPythonServer()
	case LanguageTypeScript, LanguageJavaScript:
		return d.findTypeScriptServer()
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

// findGoServer 查找 Go 服务器 (gopls)
func (d *Detector) findGoServer() (*ServerInfo, error) {
	// 先检查 GOPATH/bin
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(os.Getenv("HOME"), "go")
		if runtime.GOOS == "windows" {
			gopath = os.Getenv("USERPROFILE")
			if gopath != "" {
				gopath = filepath.Join(gopath, "go")
			}
		}
	}

	goplsPath := filepath.Join(gopath, "bin", "gopls")
	if runtime.GOOS == "windows" {
		goplsPath += ".exe"
	}

	if _, err := exec.LookPath("gopls"); err == nil {
		return &ServerInfo{
			Language: LanguageGo,
			Command:  "gopls",
			Args:     []string{"serve"},
		}, nil
	}

	// 检查 GOPATH/bin
	if _, err := os.Stat(goplsPath); err == nil {
		return &ServerInfo{
			Language: LanguageGo,
			Command:  goplsPath,
			Args:     []string{"serve"},
		}, nil
	}

	return nil, fmt.Errorf("gopls not found")
}

// findPythonServer 查找 Python 服务器 (pylsp)
func (d *Detector) findPythonServer() (*ServerInfo, error) {
	// 检查 pylsp
	if _, err := exec.LookPath("pylsp"); err == nil {
		return &ServerInfo{
			Language: LanguagePython,
			Command:  "pylsp",
			Args:     []string{},
		}, nil
	}

	// 尝试用 python -m pylsp
	pythonCmd := "python3"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python"
		if _, err := exec.LookPath(pythonCmd); err != nil {
			return nil, fmt.Errorf("python not found")
		}
	}

	// 测试是否可以运行 pylsp
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := utils.CommandContext(ctx, pythonCmd, "-m", "pylsp", "--help")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pylsp not available")
	}

	return &ServerInfo{
		Language: LanguagePython,
		Command:  pythonCmd,
		Args:     []string{"-m", "pylsp"},
	}, nil
}

// findTypeScriptServer 查找 TypeScript 服务器 (tsserver)
func (d *Detector) findTypeScriptServer() (*ServerInfo, error) {
	// tsserver 通常随 typescript 安装
	// 检查常见的 node_modules/.bin 路径
	candidates := []string{
		"node_modules/.bin/tsserver",
		filepath.Join(os.Getenv("HOME"), ".npm-global/bin/tsserver"),
	}

	if runtime.GOOS == "windows" {
		candidates = append(candidates, "node_modules\\.bin\\tsserver.cmd")
	}

	for _, cmd := range candidates {
		// 检查相对路径
		if filepath.IsAbs(cmd) {
			if _, err := os.Stat(cmd); err == nil {
				return &ServerInfo{
					Language: LanguageTypeScript,
					Command:  cmd,
					Args:     []string{},
				}, nil
			}
		}
	}

	// 尝试全局查找
	if _, err := exec.LookPath("tsserver"); err == nil {
		return &ServerInfo{
			Language: LanguageTypeScript,
			Command:  "tsserver",
			Args:     []string{},
		}, nil
	}

	return nil, fmt.Errorf("tsserver not found")
}

// getEmbeddedServer 获取内置服务器
func (d *Detector) getEmbeddedServer(lang LanguageType) (*ServerInfo, error) {
	// 内嵌服务器实现将在 embedded_gopls.go 中定义
	// 这里返回占位符
	return nil, fmt.Errorf("embedded server not implemented for: %s", lang)
}

// IsLanguageSupported 检查语言是否支持
func (d *Detector) IsLanguageSupported(lang LanguageType) bool {
	switch lang {
	case LanguageGo, LanguagePython, LanguageTypeScript, LanguageJavaScript:
		return true
	default:
		return false
	}
}

// DisableServer 禁用某语言的服务器
func (d *Detector) DisableServer(lang LanguageType) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabledServers[lang] = false
}

// EnableServer 启用某语言的服务器
func (d *Detector) EnableServer(lang LanguageType) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabledServers[lang] = true
}

// IsServerEnabled 检查服务器是否启用
func (d *Detector) IsServerEnabled(lang LanguageType) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.enabledServers[lang]
}
