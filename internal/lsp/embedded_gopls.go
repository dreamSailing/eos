//go:build with_gopls

package lsp

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

//go:embed binaries/*
var embeddedFS embed.FS

var (
	extractedOnce   sync.Once
	extractedPath   string
	extractErr      error
	binariesChecked bool
	hasBinaries     bool
)

// init 检查是否有可用的二进制文件
func init() {
	entries, err := embeddedFS.ReadDir("binaries")
	binariesChecked = true
	if err != nil {
		hasBinaries = false
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != ".gitkeep" && entry.Name() != "README.md" {
			hasBinaries = true
			return
		}
	}
}

// extractBinary 提取嵌入的 gopls 二进制
func extractBinary() (string, error) {
	extractedOnce.Do(func() {
		// 获取对应平台的二进制文件
		var binaryName string
		switch runtime.GOOS {
		case "windows":
			if runtime.GOARCH == "arm64" {
				binaryName = "gopls-windows-arm64.exe"
			} else {
				binaryName = "gopls-windows-amd64.exe"
			}
		case "darwin":
			if runtime.GOARCH == "arm64" {
				binaryName = "gopls-darwin-arm64"
			} else {
				binaryName = "gopls-darwin-amd64"
			}
		default:
			if runtime.GOARCH == "arm64" {
				binaryName = "gopls-linux-arm64"
			} else {
				binaryName = "gopls-linux-amd64"
			}
		}

		// 读取嵌入的文件
		data, readErr := embeddedFS.ReadFile(filepath.Join("binaries", binaryName))
		if readErr != nil {
			extractErr = fmt.Errorf("embedded gopls not found: %w", readErr)
			return
		}

		// 创建临时目录
		tempDir, createErr := os.MkdirTemp("", "vb-coding-gopls-*")
		if createErr != nil {
			extractErr = createErr
			return
		}

		// 写入二进制文件
		destPath := filepath.Join(tempDir, "gopls")
		if runtime.GOOS == "windows" {
			destPath += ".exe"
		}

		if writeErr := os.WriteFile(destPath, data, 0755); writeErr != nil {
			extractErr = writeErr
			return
		}

		extractedPath = destPath
	})

	if extractErr != nil {
		return "", extractErr
	}

	return extractedPath, nil
}

// GetEmbeddedGoplsPath 获取嵌入的 gopls 路径
func GetEmbeddedGoplsPath() (string, error) {
	return extractBinary()
}

// HasEmbeddedGopls 检查是否有嵌入的 gopls
func HasEmbeddedGopls() bool {
	if !binariesChecked {
		// Try to read to trigger check
		_, err := embeddedFS.ReadDir("binaries")
		if err != nil {
			return false
		}
	}
	return hasBinaries
}

// getEmbeddedServer 获取内置服务器（实现 detector 接口）
func (d *Detector) getEmbeddedServer(lang LanguageType) (*ServerInfo, error) {
	if lang != LanguageGo {
		return nil, fmt.Errorf("no embedded server for: %s", lang)
	}

	path, err := extractBinary()
	if err != nil {
		return nil, err
	}

	slog.Debug("lsp.embedded.gopls_loaded",
		"component", utils.ComponentSystem,
		"path", path)

	return &ServerInfo{
		Language: LanguageGo,
		Command:  path,
		Args:     []string{"serve"},
	}, nil
}
