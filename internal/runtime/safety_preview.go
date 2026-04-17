package runtime

import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools"
	gitops "github.com/dreamSailing/eos/internal/tools/git"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func setSafetyPreview(ctx context.Context, tm *tools.Manager, call tools.ToolCall) {
	if tools.SetPendingDiff == nil {
		return
	}
	if tm == nil {
		return
	}
	name := strings.ToLower(strings.TrimSpace(call.Tool))
	switch name {
	case tools.ToolFS:
		mode, _ := call.Parameters["mode"].(string)
		mode = strings.ToLower(strings.TrimSpace(mode))
		if mode == "" {
			mode = "write"
		}
		switch mode {
		case "write":
			path, okP := call.Parameters["path"].(string)
			content, okC := call.Parameters["content"].(string)
			if okP && okC {
				rr := tm.ExecuteStructured(ctx, []tools.ToolCall{{Tool: tools.ToolFS, Parameters: map[string]any{"mode": "diff", "path": path, "content": content}}})
				if len(rr) > 0 && rr[0].Status == "success" {
					if diffText, ok := rr[0].Data["text"].(string); ok && strings.TrimSpace(diffText) != "" {
						tools.SetPendingDiff(diffText)
						return
					}
				}
			}
		case "create", "mkdir":
			path, _ := call.Parameters["path"].(string)
			path = strings.TrimSpace(path)
			if path == "" {
				return
			}
			res := utils.ResolvePath(path)
			if !res.IsValid {
				return
			}
			abs := filepath.ToSlash(res.AbsPath)
			typ, _ := call.Parameters["type"].(string)
			typ = strings.TrimSpace(typ)
			if mode == "mkdir" || strings.EqualFold(typ, "dir") {
				tools.SetPendingDiff("Operation: fs mkdir\nTarget: " + abs)
				return
			}
			if content, ok := call.Parameters["content"].(string); ok {
				rr := tm.ExecuteStructured(ctx, []tools.ToolCall{{Tool: tools.ToolFS, Parameters: map[string]any{"mode": "diff", "path": path, "content": content}}})
				if len(rr) > 0 && rr[0].Status == "success" {
					if diffText, ok := rr[0].Data["text"].(string); ok && strings.TrimSpace(diffText) != "" {
						tools.SetPendingDiff(diffText)
						return
					}
				}
			}
			tools.SetPendingDiff("Operation: fs create\nTarget: " + abs)
			return
		case "delete":
			path, _ := call.Parameters["path"].(string)
			path = strings.TrimSpace(path)
			if path == "" {
				return
			}
			res := utils.ResolvePath(path)
			if !res.IsValid {
				return
			}
			ap := res.AbsPath
			abs := filepath.ToSlash(ap)
			fi, err := os.Stat(ap)
			if err != nil {
				tools.SetPendingDiff("Operation: fs delete\nTarget: " + abs + "\nStatus: not found")
				return
			}
			if !fi.IsDir() {
				tools.SetPendingDiff("Operation: fs delete\nTarget: " + abs + "\nType: file")
				return
			}
			items, truncated := listDirSample(ap, 40)
			var b strings.Builder
			b.WriteString("Operation: fs delete\nTarget: ")
			b.WriteString(abs)
			b.WriteString("\nType: directory\nSample:\n")
			for _, it := range items {
				b.WriteString("- ")
				b.WriteString(it)
				b.WriteString("\n")
			}
			if truncated {
				b.WriteString("... (truncated)\n")
			}
			tools.SetPendingDiff(strings.TrimRight(b.String(), "\n"))
			return
		case "move", "copy":
			src, _ := call.Parameters["source"].(string)
			dst, _ := call.Parameters["destination"].(string)
			src = strings.TrimSpace(src)
			dst = strings.TrimSpace(dst)
			if src == "" || dst == "" {
				return
			}
			rS := utils.ResolvePath(src)
			rD := utils.ResolvePath(dst)
			if !rS.IsValid || !rD.IsValid {
				return
			}
			srcAbs := filepath.ToSlash(rS.AbsPath)
			dstAbs := filepath.ToSlash(rD.AbsPath)
			exists := false
			if _, err := os.Stat(rD.AbsPath); err == nil {
				exists = true
			}
			op := "fs " + mode
			preview := "Operation: " + op + "\nSource: " + srcAbs + "\nDestination: " + dstAbs
			if exists {
				preview += "\nNote: destination exists (may overwrite)"
			}
			tools.SetPendingDiff(preview)
			return
		}
	case tools.ToolEdit:
		cp := map[string]any{}
		for k, v := range call.Parameters {
			cp[k] = v
		}
		cp["previewOnly"] = true
		rr := tm.ExecuteStructured(ctx, []tools.ToolCall{{Tool: tools.ToolEdit, Parameters: cp}})
		if len(rr) == 0 || rr[0].Status != "success" {
			return
		}
		r0 := rr[0]
		if diff, ok := r0.Data["diff"].(string); ok && strings.TrimSpace(diff) != "" {
			tools.SetPendingDiff(diff)
			return
		}
		if rs, ok := r0.Data["results"].([]map[string]any); ok {
			var b strings.Builder
			for _, m := range rs {
				if d, ok := m["diff"].(string); ok && strings.TrimSpace(d) != "" {
					b.WriteString(d)
					if !strings.HasSuffix(d, "\n") {
						b.WriteString("\n")
					}
				}
				if b.Len() > 16000 {
					break
				}
			}
			out := strings.TrimSpace(b.String())
			if out != "" {
				tools.SetPendingDiff(out)
				return
			}
		}
		if rs, ok := r0.Data["results"].([]any); ok {
			var b strings.Builder
			for _, it := range rs {
				m, ok := it.(map[string]any)
				if !ok {
					continue
				}
				if d, ok := m["diff"].(string); ok && strings.TrimSpace(d) != "" {
					b.WriteString(d)
					if !strings.HasSuffix(d, "\n") {
						b.WriteString("\n")
					}
				}
				if b.Len() > 16000 {
					break
				}
			}
			out := strings.TrimSpace(b.String())
			if out != "" {
				tools.SetPendingDiff(out)
				return
			}
		}
	case tools.ToolBash, "bash_session":
		cmd, _ := call.Parameters["command"].(string)
		cmd = strings.TrimSpace(cmd)
		if cmd != "" {
			tools.SetPendingDiff("Command:\n" + cmd)
		}
	default:
		if strings.HasPrefix(name, "git_") {
			var b strings.Builder
			b.WriteString("Git status:\n")
			st := tm.ExecuteStructured(ctx, []tools.ToolCall{{Tool: tools.ToolGitStatus}})
			if len(st) > 0 && st[0].Status == "success" {
				if raw, ok := st[0].Data["changes"].([]gitops.Change); ok {
					sort.Slice(raw, func(i, j int) bool { return raw[i].Path < raw[j].Path })
					for i, c := range raw {
						if i >= 40 {
							b.WriteString("... (truncated)\n")
							break
						}
						b.WriteString("- ")
						b.WriteString(c.State)
						b.WriteString(": ")
						b.WriteString(c.Path)
						b.WriteString("\n")
					}
				} else if v, ok := st[0].Data["changes"].([]any); ok {
					type item struct{ Path, State string }
					list := make([]item, 0, len(v))
					for _, it := range v {
						m, ok := it.(map[string]any)
						if !ok {
							continue
						}
						p, _ := m["Path"].(string)
						if p == "" {
							p, _ = m["path"].(string)
						}
						s, _ := m["State"].(string)
						if s == "" {
							s, _ = m["state"].(string)
						}
						list = append(list, item{Path: p, State: s})
					}
					sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
					for i, c := range list {
						if i >= 40 {
							b.WriteString("... (truncated)\n")
							break
						}
						b.WriteString("- ")
						b.WriteString(strings.TrimSpace(c.State))
						b.WriteString(": ")
						b.WriteString(strings.TrimSpace(c.Path))
						b.WriteString("\n")
					}
				} else {
					b.WriteString(st[0].Display)
					b.WriteString("\n")
				}
			} else if len(st) > 0 {
				b.WriteString("Error: ")
				b.WriteString(strings.TrimSpace(st[0].Error))
				b.WriteString("\n")
			}
			df := tm.ExecuteStructured(ctx, []tools.ToolCall{{Tool: tools.ToolGitDiff, Parameters: map[string]any{"path": "."}}})
			if len(df) > 0 && df[0].Status == "success" {
				if txt, ok := df[0].Data["text"].(string); ok && strings.TrimSpace(txt) != "" {
					b.WriteString("\nGit diff (truncated):\n")
					b.WriteString(truncate(txt, 12000))
				}
			}
			out := strings.TrimSpace(b.String())
			if out != "" {
				tools.SetPendingDiff(out)
			}
		}
	}
}

func listDirSample(root string, max int) ([]string, bool) {
	var out []string
	truncated := false
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			rel += "/"
		}
		out = append(out, rel)
		if len(out) >= max {
			truncated = true
			return fmt.Errorf("stop")
		}
		return nil
	})
	sort.Strings(out)
	return out, truncated
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}
