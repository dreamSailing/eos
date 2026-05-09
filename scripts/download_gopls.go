//go:build ignore

package main

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// gopls 版本
const goplsVersion = "v0.21.1"

// 平台和架构列表
var platforms = []struct {
	goos   string
	goarch string
	ext    string
}{
	{"windows", "amd64", ".exe"},
	{"windows", "arm64", ".exe"},
	{"darwin", "amd64", ""},
	{"darwin", "arm64", ""},
	{"linux", "amd64", ""},
	{"linux", "arm64", ""},
}

func main() {
	// 确定脚本所在目录
	_, scriptFile, _, _ := runtime.Caller(0)
	scriptDir := filepath.Dir(scriptFile)
	binDir := filepath.Join(scriptDir, "..", "internal", "lsp", "binaries")

	// 创建 binaries 目录
	if err := os.MkdirAll(binDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// 下载所有平台的二进制文件
	successCount := 0
	for _, p := range platforms {
		binaryName := fmt.Sprintf("gopls-%s-%s%s", p.goos, p.goarch, p.ext)
		destPath := filepath.Join(binDir, binaryName)

		// 检查是否已存在
		if _, err := os.Stat(destPath); err == nil {
			fmt.Printf("[SKIP] %s already exists\n", binaryName)
			successCount++
			continue
		}

		// 构建 gopls 下载 URL
		// gopls 从 Go tools 仓库下载
		url := buildGoplsURL(p.goos, p.goarch)

		fmt.Printf("[DOWNLOAD] %s from %s\n", binaryName, url)

		if err := downloadFile(url, destPath); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to download %s: %v\n", binaryName, err)
			continue
		}

		// 设置可执行权限
		if err := os.Chmod(destPath, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to set executable permission for %s: %v\n", binaryName, err)
		}

		successCount++
		fmt.Printf("[OK] Downloaded %s\n", binaryName)
	}

	fmt.Printf("\nDownloaded %d/%d gopls binaries\n", successCount, len(platforms))

	// 创建 README
	readmePath := filepath.Join(binDir, "README.md")
	createReadme(readmePath)

	fmt.Println("\nNow you can build with embedded gopls:")
	fmt.Println("  go build -tags with_gopls")
}

// buildGoplsURL 构建 gopls 下载 URL
func buildGoplsURL(goos, goarch string) string {
	// 使用 GitHub 的 gopls releases
	// URL 格式: https://github.com/golang/tools/releases/download/${version}/gopls_${os}_${arch}.zip
	// 但我们需要直接下载二进制文件

	// 由于 GitHub releases 没有提供直接的二进制下载链接，
	// 我们使用 Go install 的方式

	// 实际上，我们建议用户使用 go install 来安装 gopls
	// 这个脚本主要用于 CI/CD 环境

	// 作为替代方案，我们返回一个空字符串并提示用户
	return ""
}

// downloadFile 下载文件
func downloadFile(url, destPath string) error {
	// 对于 gopls，我们使用 go install 命令
	// 这里只是占位符实现
	return fmt.Errorf("direct download not implemented, use 'go install golang.org/x/tools/gopls@%s' instead", goplsVersion)
}

// createReadme 创建 README 文件
func createReadme(path string) {
	content := `# LSP Binaries

This directory contains LSP server binaries for embedding.

## Adding gopls Binary

To add gopls binary for a specific platform:

\`\`\`bash
# For your current platform
go install golang.org/x/tools/gopls@` + goplsVersion + `

# Copy the binary here
cp $(go env GOPATH)/bin/gopls internal/lsp/binaries/gopls-$(go env GOOS)-$(go env GOARCH)
# On Windows
copy %GOPATH%\bin\gopls.exe internal\lsp\binaries\gopls-windows-amd64.exe
\`\`\`

## Platform Names

- windows-amd64: gopls-windows-amd64.exe
- windows-arm64: gopls-windows-arm64.exe
- darwin-amd64: gopls-darwin-amd64
- darwin-arm64: gopls-darwin-arm64
- linux-amd64: gopls-linux-amd64
- linux-arm64: gopls-linux-arm64

## Build Tags

- Default build: LSP with external servers (requires gopls in PATH)
- \`without_lsp\` build: Minimal build without LSP support
- \`with_gopls\` build: LSP with embedded gopls from this directory
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create README: %v\n", err)
	}
}

// downloadFileWithProgress 带进度显示的下载
func downloadFileWithProgress(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status: %s", resp.Status)
	}

	// 创建目标文件
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 获取文件大小
	size := resp.ContentLength
	if size <= 0 {
		// 如果不知道大小，直接复制
		_, err = io.Copy(out, resp.Body)
		return err
	}

	// 带进度显示的复制
	reader := &progressReader{
		reader: resp.Body,
		size:   size,
		path:   filepath.Base(destPath),
	}
	_, err = io.Copy(out, reader)
	return err
}

type progressReader struct {
	reader io.Reader
	size   int64
	read   int64
	path   string
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	pr.read += int64(n)

	// 显示进度
	if pr.size > 0 {
		percent := float64(pr.read) / float64(pr.size) * 100
		fmt.Printf("\r[%s] %.1f%% (%d/%d bytes)",
			pr.path, percent, pr.read, pr.size)
		if err == io.EOF {
			fmt.Println()
		}
	}

	return
}

// confirmUser 确认用户操作
func confirmUser(prompt string) bool {
	fmt.Print(prompt, " [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.TrimSpace(strings.ToLower(response)) == "y"
}
