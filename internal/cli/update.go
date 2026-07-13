package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/update"
	"github.com/dreamSailing/eos/internal/version"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "检查并安装最新版本",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("当前版本: %s\n", version.AppVersion)
			fmt.Println("正在检查更新...")

			result, err := update.CheckLatest(context.Background())
			if err != nil {
				return fmt.Errorf("检查更新失败: %w", err)
			}

			if !result.HasUpdate {
				fmt.Printf("已是最新版本 (%s)\n", result.LatestVersion)
				return nil
			}

			fmt.Printf("发现新版本: %s\n", result.LatestVersion)

			if result.DownloadURL == "" {
				fmt.Println("未找到适合当前平台的下载链接，请手动下载：")
				fmt.Printf("  %s\n", result.ReleaseURL)
				return nil
			}

			return performSelfUpdate(result)
		},
	}
	return cmd
}

func performSelfUpdate(result *update.CheckResult) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前程序路径失败: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("解析程序路径失败: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(exePath), "eos-update-*.exe")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	fmt.Printf("正在下载 %s ...\n", result.LatestVersion)
	if err := downloadFile(tmpFile, result.DownloadURL); err != nil {
		tmpFile.Close()
		return fmt.Errorf("下载失败: %w", err)
	}
	tmpFile.Close()

	if runtime.GOOS == "windows" {
		backupPath := exePath + ".bak"
		_ = os.Remove(backupPath)
		if err := os.Rename(exePath, backupPath); err != nil {
			return fmt.Errorf("备份当前版本失败: %w", err)
		}
		defer os.Remove(backupPath)

		if err := os.Rename(tmpPath, exePath); err != nil {
			os.Rename(backupPath, exePath)
			return fmt.Errorf("替换程序失败: %w", err)
		}
	} else {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			return fmt.Errorf("设置权限失败: %w", err)
		}
		if err := os.Rename(tmpPath, exePath); err != nil {
			return fmt.Errorf("替换程序失败: %w", err)
		}
	}

	fmt.Printf("已成功更新到 %s\n", result.LatestVersion)
	if strings.TrimSpace(result.ReleaseNotes) != "" {
		fmt.Println()
		fmt.Println("更新内容:")
		fmt.Println(result.ReleaseNotes)
	}
	return nil
}

func downloadFile(dst *os.File, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "EOS-Update")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回状态码 %d", resp.StatusCode)
	}

	progress := &progressWriter{total: resp.ContentLength}
	_, err = io.Copy(dst, io.TeeReader(resp.Body, progress))
	if err == nil {
		fmt.Println()
	}
	return err
}

type progressWriter struct {
	total   int64
	written int64
	lastPct int
}

func (p *progressWriter) Write(data []byte) (int, error) {
	n := len(data)
	p.written += int64(n)
	if p.total > 0 {
		pct := int(float64(p.written) / float64(p.total) * 100)
		if pct != p.lastPct {
			fmt.Printf("\r  下载进度: %d%% (%.1f MB / %.1f MB)", pct,
				float64(p.written)/1024/1024, float64(p.total)/1024/1024)
			p.lastPct = pct
		}
	}
	return n, nil
}
