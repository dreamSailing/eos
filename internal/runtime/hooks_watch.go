package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/fsnotify/fsnotify"
)

func startHooksWatcher(ctx context.Context, dt *DispatchTools) {
	if dt == nil || dt.hookMgr == nil {
		return
	}

	home, _ := os.UserHomeDir()
	dt.mu.RLock()
	currentCtx := dt.currentCtx
	dt.mu.RUnlock()
	wd := strings.TrimSpace(tools.WorkspaceRootFromContext(currentCtx))
	if wd == "" {
		wd, _ = os.Getwd()
	}
	if wd == "" {
		return
	}

	type watchFile struct {
		path   string
		source string
	}
	watchFiles := make([]watchFile, 0, 8)
	if home != "" {
		watchFiles = append(watchFiles, watchFile{path: filepath.Join(home, ".eos", "settings.json"), source: "user_settings"})
		watchFiles = append(watchFiles, watchFile{path: filepath.Join(home, ".claude", "settings.json"), source: "user_settings"})
		watchFiles = append(watchFiles, watchFile{path: filepath.Join(home, ".trae", "settings.json"), source: "user_settings"})
	}
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".eos", "settings.json"), source: "project_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".eos", "settings.local.json"), source: "local_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".claude", "settings.json"), source: "project_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".claude", "settings.local.json"), source: "local_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".trae", "settings.json"), source: "project_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".trae", "settings.local.json"), source: "local_settings"})

	skillsDirs := []string{
		filepath.Join(wd, ".eos", "skills"),
		filepath.Join(wd, ".claude", "skills"),
		filepath.Join(wd, ".trae", "skills"),
	}
	pluginDirs := pluginpkg.ResolveScanDirs(wd)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	addPath := func(p string) {
		if strings.TrimSpace(p) == "" {
			return
		}
		_ = w.Add(p)
	}

	for _, wf := range watchFiles {
		addPath(filepath.Dir(wf.path))
		addPath(wf.path)
	}
	for _, sd := range skillsDirs {
		addPath(filepath.Dir(sd))
		addPath(sd)
	}
	for _, pd := range pluginDirs {
		addPath(filepath.Dir(pd))
		addPath(pd)
	}

	for _, sd := range skillsDirs {
		_ = filepath.WalkDir(sd, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				_ = w.Add(path)
			}
			return nil
		})
	}
	for _, pd := range pluginDirs {
		_ = filepath.WalkDir(pd, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				_ = w.Add(path)
			}
			return nil
		})
	}

	type pendingEvt struct {
		source   string
		filePath string
	}
	pending := make(map[string]pendingEvt)
	ticker := time.NewTicker(800 * time.Millisecond)

	go func() {
		defer func() {
			ticker.Stop()
			_ = w.Close()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-w.Events:
				if strings.TrimSpace(ev.Name) == "" {
					continue
				}
				if (ev.Op & (fsnotify.Write | fsnotify.Create | fsnotify.Rename)) == 0 {
					continue
				}

				src := ""
				p := filepath.Clean(ev.Name)
				for _, wf := range watchFiles {
					if strings.TrimSpace(wf.path) == "" {
						continue
					}
					if strings.EqualFold(p, filepath.Clean(wf.path)) {
						src = wf.source
						break
					}
				}
				if src == "" {
					if pathUnderHookRoots(p, skillsDirs) {
						src = "skills"
					}
				}
				if src == "" {
					if pathUnderHookRoots(p, pluginDirs) {
						src = "plugins"
					}
				}
				if src == "" {
					continue
				}
				pending[p] = pendingEvt{source: src, filePath: p}

				if (ev.Op & fsnotify.Create) != 0 {
					if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
						_ = w.Add(ev.Name)
					}
				}
			case <-ticker.C:
				if len(pending) == 0 {
					continue
				}
				snap := pending
				pending = make(map[string]pendingEvt)

				for _, pe := range snap {
					dec, _ := dt.hookMgr.ConfigChange(context.Background(), pe.source, pe.filePath)
					if strings.TrimSpace(dec.AdditionalContext) != "" && dt.onMeta != nil {
						dt.onMeta("phase.note:" + strings.TrimSpace(dec.AdditionalContext))
					}
					if strings.EqualFold(dec.Decision, "block") || strings.EqualFold(dec.Decision, "deny") {
						if dt.onMeta != nil {
							msg := "ConfigChange blocked: " + pe.source
							if strings.TrimSpace(dec.Reason) != "" {
								msg += " - " + strings.TrimSpace(dec.Reason)
							}
							dt.onMeta("phase.note:" + msg)
						}
						continue
					}

					switch pe.source {
					case "skills":
						if dt.toolsManager != nil {
							if sm := dt.toolsManager.GetSkillManager(); sm != nil {
								_ = sm.ReloadPreserveActive()
							}
						}
					default:
						dt.mu.RLock()
						currentCtx := dt.currentCtx
						dt.mu.RUnlock()
						_ = dt.hookMgr.LoadFromDefaultLocations(currentCtx)
					}
				}
			case <-w.Errors:
			}
		}
	}()
}

func pathUnderHookRoots(path string, roots []string) bool {
	lp := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		normRoot := strings.ToLower(filepath.ToSlash(filepath.Clean(root)))
		if lp == normRoot || strings.HasPrefix(lp, normRoot+"/") {
			return true
		}
	}
	return false
}
