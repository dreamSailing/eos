//go:build !without_lsp
// +build !without_lsp

package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/lsp"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

// lspManagerEntry LSP 管理器条目
type lspManagerEntry struct {
	manager  *lsp.Manager
	rootDir  string
	langType lsp.LanguageType
}

// initLSPManager 初始化 LSP 管理器
func (rc *RuntimeCore) initLSPManager() *lspManagerEntry {
	cfg, cfgPath := config.Load()
	enabled := cfg.LSP.EnabledValue()
	autoDetect := cfg.LSP.AutoDetectValue()
	if !enabled || !autoDetect {
		slog.Debug("bridge.init_lsp.disabled", "component", utils.ComponentSystem,
			"enabled", enabled, "auto_detect", autoDetect, "config_file", cfgPath)
		return nil
	}

	wd := strings.TrimSpace(rc.workingRoot())
	if wd == "" {
		slog.Debug("bridge.init_lsp.no_working_dir", "component", utils.ComponentSystem, "config_file", cfgPath)
		return nil
	}

	detector := lsp.NewDetector()
	langType := detector.DetectLanguage(wd)

	if langType == "" {
		slog.Debug("bridge.init_lsp.unknown_language", "component", utils.ComponentSystem, "path", wd)
		return nil
	}

	// 创建 LSP 管理器
	mgr := lsp.NewManager(lsp.Config{
		Enabled:    enabled,
		AutoDetect: autoDetect,
		Timeout:    10 * time.Second,
	})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := mgr.GetClientForPath(ctx, wd); err != nil {
			slog.Debug("bridge.init_lsp.start_failed", "component", utils.ComponentSystem,
				"language", langType, "error", err.Error())
			return
		}
		slog.Info("bridge.init_lsp.success", "component", utils.ComponentSystem,
			"language", langType)
	}()

	return &lspManagerEntry{
		manager:  mgr,
		rootDir:  wd,
		langType: langType,
	}
}

func (rc *RuntimeCore) refreshLSPManager() {
	if rc == nil {
		return
	}

	root := strings.TrimSpace(rc.workingRoot())
	rc.mu.RLock()
	current := rc.lspManager
	rc.mu.RUnlock()
	if current != nil && strings.EqualFold(filepath.Clean(current.rootDir), filepath.Clean(root)) {
		return
	}

	next := rc.initLSPManager()
	rc.mu.Lock()
	old := rc.lspManager
	rc.lspManager = next
	rc.mu.Unlock()
	rc.ShutdownLSPManager(old)
}

// ProcessLSPDiagnostics 处理 LSP 诊断信息并添加到上下文
func (rc *RuntimeCore) ProcessLSPDiagnostics(lspEntry *lspManagerEntry) {
	if lspEntry == nil || lspEntry.manager == nil {
		return
	}

	// 获取所有诊断信息
	diagnostics := lspEntry.manager.GetAllDiagnostics()

	if len(diagnostics) == 0 {
		return
	}

	md := formatProblemsAndDiagnosticsMarkdown(diagnostics, lspEntry.rootDir)
	if strings.TrimSpace(md) == "" {
		return
	}

	// 添加为系统消息
	rc.AddPinnedSystem(md)

	slog.Debug("bridge.process_lsp_diagnostics", "component", utils.ComponentSystem,
		"files", len(diagnostics))
}

func (rc *RuntimeCore) ProblemsAndDiagnosticsMarkdown() string {
	if rc == nil || rc.lspManager == nil || rc.lspManager.manager == nil {
		return "## Problems and Diagnostics\n\nNo diagnostics available (LSP disabled).\n"
	}
	diagnostics := rc.lspManager.manager.GetAllDiagnostics()
	if len(diagnostics) == 0 {
		return "## Problems and Diagnostics\n\nNo problems found.\n"
	}
	out := formatProblemsAndDiagnosticsMarkdown(diagnostics, rc.lspManager.rootDir)
	if strings.TrimSpace(out) == "" {
		return "## Problems and Diagnostics\n\nNo problems found.\n"
	}
	return out
}

func (rc *RuntimeCore) LSPServersMarkdown() string {
	wd := strings.TrimSpace(rc.workingRoot())
	cfg, cfgPath := config.Load()
	enabled := cfg.LSP.EnabledValue()
	autoDetect := cfg.LSP.AutoDetectValue()

	activeLang := ""
	activeCmd := ""
	activeRoot := ""
	if rc != nil && rc.lspManager != nil {
		activeLang = string(rc.lspManager.langType)
		activeRoot = rc.lspManager.rootDir
		if rc.lspManager.manager != nil {
			if c, ok := rc.lspManager.manager.GetRunningClientForPath(activeRoot); ok && c != nil {
				activeCmd = strings.TrimSpace(c.CommandLine())
			}
		}
	}

	d := lsp.NewDetector()
	detected := string(d.DetectLanguage(wd))

	type item struct {
		lang string
		out  string
	}
	var items []item
	for _, lang := range []lsp.LanguageType{lsp.LanguageGo, lsp.LanguagePython, lsp.LanguageTypeScript, lsp.LanguageJavaScript} {
		info, err := d.FindServer(lang)
		if err != nil || info == nil {
			items = append(items, item{lang: string(lang), out: "not found"})
			continue
		}
		cmd := strings.TrimSpace(info.Command)
		if len(info.Args) > 0 {
			cmd = cmd + " " + strings.Join(info.Args, " ")
		}
		if strings.TrimSpace(cmd) == "" {
			cmd = "found"
		}
		items = append(items, item{lang: string(lang), out: cmd})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].lang < items[j].lang })

	var b strings.Builder
	b.WriteString("## LSP\n\n")
	b.WriteString(fmt.Sprintf("- Config: enabled=%v auto_detect=%v\n", enabled, autoDetect))
	if strings.TrimSpace(cfgPath) != "" {
		b.WriteString(fmt.Sprintf("- Config file: %s\n", cfgPath))
	}
	if strings.TrimSpace(wd) != "" {
		b.WriteString(fmt.Sprintf("- Workspace: %s\n", wd))
	}
	if strings.TrimSpace(detected) != "" {
		b.WriteString(fmt.Sprintf("- Detected language: %s\n", detected))
	} else {
		b.WriteString("- Detected language: (unknown)\n")
	}
	if strings.TrimSpace(activeLang) != "" {
		b.WriteString(fmt.Sprintf("- Active: %s\n", activeLang))
		if strings.TrimSpace(activeCmd) != "" {
			b.WriteString(fmt.Sprintf("- Active server: %s\n", activeCmd))
		}
		if strings.TrimSpace(activeRoot) != "" {
			b.WriteString(fmt.Sprintf("- Active root: %s\n", activeRoot))
		}
	} else {
		b.WriteString("- Active: (not running)\n")
	}
	b.WriteString("\n### Available servers\n")
	for _, it := range items {
		b.WriteString(fmt.Sprintf("- %s: %s\n", it.lang, it.out))
	}
	return b.String()
}

func (rc *RuntimeCore) LSPStatus() LSPStatus {
	wd := strings.TrimSpace(rc.workingRoot())
	cfg, cfgPath := config.Load()
	enabled := cfg.LSP.EnabledValue()
	autoDetect := cfg.LSP.AutoDetectValue()

	activeLang := ""
	activeCmd := ""
	activeRoot := ""
	if rc != nil && rc.lspManager != nil {
		activeLang = string(rc.lspManager.langType)
		activeRoot = rc.lspManager.rootDir
		if rc.lspManager.manager != nil {
			if c, ok := rc.lspManager.manager.GetRunningClientForPath(activeRoot); ok && c != nil {
				activeCmd = strings.TrimSpace(c.CommandLine())
			}
		}
	}

	d := lsp.NewDetector()
	detected := string(d.DetectLanguage(wd))

	var servers []LSPServerInfo
	for _, lang := range []lsp.LanguageType{lsp.LanguageGo, lsp.LanguagePython, lsp.LanguageTypeScript, lsp.LanguageJavaScript} {
		info, err := d.FindServer(lang)
		if err != nil || info == nil {
			servers = append(servers, LSPServerInfo{Language: string(lang), Command: "not found", Found: false})
			continue
		}
		cmd := strings.TrimSpace(info.Command)
		if len(info.Args) > 0 {
			cmd = cmd + " " + strings.Join(info.Args, " ")
		}
		if strings.TrimSpace(cmd) == "" {
			cmd = "found"
		}
		servers = append(servers, LSPServerInfo{Language: string(lang), Command: cmd, Found: true})
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].Language < servers[j].Language })

	out := LSPStatus{
		Enabled:          enabled,
		AutoDetect:       autoDetect,
		ConfigFile:       cfgPath,
		Workspace:        wd,
		DetectedLanguage: detected,
		ActiveLanguage:   activeLang,
		ActiveServer:     activeCmd,
		ActiveRoot:       activeRoot,
		Servers:          servers,
	}
	if !enabled {
		out.Message = "disabled"
	} else if !autoDetect {
		out.Message = "auto_detect_disabled"
	}
	return out
}

func uriToLocalPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "file://") {
		u, err := url.Parse(raw)
		if err == nil && u != nil && strings.EqualFold(u.Scheme, "file") {
			p := u.Path
			if p != "" {
				if up, err2 := url.PathUnescape(p); err2 == nil {
					p = up
				}
				if runtime.GOOS == "windows" && strings.HasPrefix(p, "/") && len(p) >= 3 && p[2] == ':' {
					p = p[1:]
				}
				return filepath.FromSlash(p)
			}
		}
	}
	return raw
}

func relPathBestEffort(rootDir string, p string) string {
	rootDir = strings.TrimSpace(rootDir)
	p = strings.TrimSpace(p)
	if rootDir == "" || p == "" {
		return p
	}
	ap, err1 := filepath.Abs(p)
	ar, err2 := filepath.Abs(rootDir)
	if err1 != nil || err2 != nil {
		return p
	}
	rel, err := filepath.Rel(ar, ap)
	if err != nil {
		return p
	}
	if strings.HasPrefix(rel, "..") {
		return p
	}
	return rel
}

func formatProblemsAndDiagnosticsMarkdown(diagnostics map[string][]lsp.Diagnostic, rootDir string) string {
	var files []string
	for k := range diagnostics {
		files = append(files, k)
	}
	sort.Strings(files)

	var b strings.Builder
	b.WriteString("## Problems and Diagnostics (LSP)\n\n")

	totalErrors := 0
	totalWarnings := 0
	totalInfos := 0
	filesWithIssues := 0

	for _, file := range files {
		fileDiags := diagnostics[file]
		if len(fileDiags) == 0 {
			continue
		}
		errors := 0
		warnings := 0
		infos := 0
		for _, d := range fileDiags {
			switch d.Severity {
			case lsp.SeverityError:
				errors++
			case lsp.SeverityWarning:
				warnings++
			default:
				infos++
			}
		}
		if errors == 0 && warnings == 0 && infos == 0 {
			continue
		}

		filesWithIssues++
		totalErrors += errors
		totalWarnings += warnings
		totalInfos += infos

		local := uriToLocalPath(file)
		label := relPathBestEffort(rootDir, local)
		base := filepath.Base(local)
		if strings.TrimSpace(base) == "" {
			base = filepath.Base(label)
		}
		if strings.TrimSpace(base) == "" {
			base = file
		}

		b.WriteString(fmt.Sprintf("### %s\n", base))
		if strings.TrimSpace(label) != "" {
			b.WriteString(fmt.Sprintf("Path: %s\n", label))
		}

		count := 0
		for _, d := range fileDiags {
			if count >= 8 {
				break
			}
			severity := "Info"
			switch d.Severity {
			case lsp.SeverityError:
				severity = "Error"
			case lsp.SeverityWarning:
				severity = "Warning"
			}
			msg := strings.TrimSpace(d.Message)
			if msg == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("- [%s] Line %d: %s\n", severity, d.Range.Start.Line+1, msg))
			count++
		}
		if len(fileDiags) > count {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", len(fileDiags)-count))
		}
		b.WriteString("\n")
	}

	if filesWithIssues == 0 {
		return ""
	}
	b.WriteString(fmt.Sprintf("**Summary**: %d files (%d errors, %d warnings, %d infos)\n", filesWithIssues, totalErrors, totalWarnings, totalInfos))
	return b.String()
}

// ShutdownLSPManager 关闭 LSP 管理器
func (rc *RuntimeCore) ShutdownLSPManager(lspEntry *lspManagerEntry) {
	if lspEntry == nil || lspEntry.manager == nil {
		return
	}
	if err := lspEntry.manager.Close(); err != nil {
		slog.Warn("bridge.shutdown_lsp.error", "component", utils.ComponentSystem, "error", err.Error())
	}
}
