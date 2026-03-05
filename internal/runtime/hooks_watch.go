package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func startHooksWatcher(ctx context.Context, dt *DispatchTools) {
	if dt == nil || dt.hookMgr == nil {
		return
	}

	home, _ := os.UserHomeDir()
	wd, _ := os.Getwd()
	if wd == "" {
		return
	}

	type watchFile struct {
		path   string
		source string
	}
	watchFiles := make([]watchFile, 0, 8)
	if home != "" {
		watchFiles = append(watchFiles, watchFile{path: filepath.Join(home, ".vb", "settings.json"), source: "user_settings"})
		watchFiles = append(watchFiles, watchFile{path: filepath.Join(home, ".claude", "settings.json"), source: "user_settings"})
		watchFiles = append(watchFiles, watchFile{path: filepath.Join(home, ".trae", "settings.json"), source: "user_settings"})
	}
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".vb", "settings.json"), source: "project_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".vb", "settings.local.json"), source: "local_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".claude", "settings.json"), source: "project_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".claude", "settings.local.json"), source: "local_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".trae", "settings.json"), source: "project_settings"})
	watchFiles = append(watchFiles, watchFile{path: filepath.Join(wd, ".trae", "settings.local.json"), source: "local_settings"})

	skillsDirs := []string{
		filepath.Join(wd, ".vb", "skills"),
		filepath.Join(wd, ".claude", "skills"),
		filepath.Join(wd, ".trae", "skills"),
	}

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
		addPath(wf.path)
	}
	for _, sd := range skillsDirs {
		addPath(sd)
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
				if (ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename)) == 0 {
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
					lp := strings.ToLower(filepath.ToSlash(p))
					for _, sd := range skillsDirs {
						if strings.TrimSpace(sd) == "" {
							continue
						}
						root := strings.ToLower(filepath.ToSlash(filepath.Clean(sd)))
						if lp == root || strings.HasPrefix(lp, root+"/") {
							src = "skills"
							break
						}
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
						_ = dt.hookMgr.LoadFromDefaultLocations()
					}
				}
			case <-w.Errors:
			}
		}
	}()
}
