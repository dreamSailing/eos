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

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/i18n"
	"github.com/dreamSailing/eos/internal/update"
	"github.com/dreamSailing/eos/internal/version"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cfg, _ := config.Load()
	lang := cfg.Language
	cmd := &cobra.Command{
		Use:   "update",
		Short: i18n.T("update.short", lang),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf(i18n.T("update.current_version", lang), version.AppVersion)
			fmt.Println(i18n.T("update.checking", lang))

			result, err := update.CheckLatest(context.Background())
			if err != nil {
				return fmt.Errorf(i18n.T("update.check_failed", lang), err)
			}

			if !result.HasUpdate {
				fmt.Printf(i18n.T("update.up_to_date", lang), result.LatestVersion)
				return nil
			}

			fmt.Printf(i18n.T("update.new_version", lang), result.LatestVersion)

			if result.DownloadURL == "" {
				fmt.Println(i18n.T("update.no_download_url", lang))
				fmt.Printf("  %s\n", result.ReleaseURL)
				return nil
			}

			return performSelfUpdate(result, lang)
		},
	}
	return cmd
}

func performSelfUpdate(result *update.CheckResult, lang string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf(i18n.T("update.get_exe_failed", lang), err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf(i18n.T("update.eval_symlink_failed", lang), err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(exePath), "eos-update-*.exe")
	if err != nil {
		return fmt.Errorf(i18n.T("update.create_temp_failed", lang), err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	fmt.Printf(i18n.T("update.downloading", lang), result.LatestVersion)
	if err := downloadFile(tmpFile, result.DownloadURL, lang); err != nil {
		tmpFile.Close()
		return fmt.Errorf(i18n.T("update.download_failed", lang), err)
	}
	tmpFile.Close()

	if runtime.GOOS == "windows" {
		backupPath := exePath + ".bak"
		_ = os.Remove(backupPath)
		if err := os.Rename(exePath, backupPath); err != nil {
			return fmt.Errorf(i18n.T("update.backup_failed", lang), err)
		}
		defer os.Remove(backupPath)

		if err := os.Rename(tmpPath, exePath); err != nil {
			os.Rename(backupPath, exePath)
			return fmt.Errorf(i18n.T("update.replace_failed", lang), err)
		}
	} else {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			return fmt.Errorf(i18n.T("update.chmod_failed", lang), err)
		}
		if err := os.Rename(tmpPath, exePath); err != nil {
			return fmt.Errorf(i18n.T("update.replace_failed", lang), err)
		}
	}

	fmt.Printf(i18n.T("update.success", lang), result.LatestVersion)
	if strings.TrimSpace(result.ReleaseNotes) != "" {
		fmt.Println()
		fmt.Println(i18n.T("update.release_notes", lang))
		fmt.Println(result.ReleaseNotes)
	}
	return nil
}

func downloadFile(dst *os.File, url string, lang string) error {
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
		return fmt.Errorf(i18n.T("update.bad_status", lang), resp.StatusCode)
	}

	progress := &progressWriter{total: resp.ContentLength, lang: lang}
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
	lang    string
}

func (p *progressWriter) Write(data []byte) (int, error) {
	n := len(data)
	p.written += int64(n)
	if p.total > 0 {
		pct := int(float64(p.written) / float64(p.total) * 100)
		if pct != p.lastPct {
			fmt.Printf(i18n.T("update.progress", p.lang), pct,
				float64(p.written)/1024/1024, float64(p.total)/1024/1024)
			p.lastPct = pct
		}
	}
	return n, nil
}
