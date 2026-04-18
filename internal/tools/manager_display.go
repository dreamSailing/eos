package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"fmt"
	"strings"
)

func (r ToolResult) FormatForUI() string {
	displayName := getToolDisplayName(r.Tool, r.Data)

	toolRest := ""
	if label, ok := extractTargetLabel(r.Data); ok {
		toolRest = " (" + label + ")"
	}

	style := GetToolStyle(r.Status)

	var lines []string

	mainLine := style.Prefix + " " + displayName + toolRest
	lines = append(lines, "["+style.Color+"]"+mainLine+"[-]")

	if r.Status == ToolStatusError && r.Error != "" {
		errLine := "  " + style.Prefix + " " + TruncateDisplay(r.Error, 100)
		lines = append(lines, "["+style.Color+"]"+errLine+"[-]")
	}

	if r.Status == ToolStatusSuccess {
		summary := r.extractSummary()
		if summary != "" {
			summaryLine := "  → " + summary
			lines = append(lines, "[gray]"+summaryLine+"[-]")
		}
	}

	return strings.Join(lines, "\n")
}

func extractTargetLabel(data map[string]interface{}) (string, bool) {
	targets := extractTargets(data)
	if len(targets) == 0 {
		return "", false
	}
	if len(targets) == 1 {
		return targets[0], true
	}
	if len(targets) <= 3 {
		return strings.Join(targets, ", "), true
	}
	return fmt.Sprintf("%d 项", len(targets)), true
}

func extractTargets(data map[string]interface{}) []string {
	if data == nil {
		return nil
	}

	getStr := func(v any) string {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		return ""
	}

	var out []string
	for _, k := range []string{"path", "file", "source"} {
		if s := getStr(data[k]); s != "" {
			out = append(out, s)
			return out
		}
	}

	for _, k := range []string{"paths", "files", "targets"} {
		if v, ok := data[k]; ok && v != nil {
			switch vv := v.(type) {
			case []string:
				for _, s := range vv {
					s = strings.TrimSpace(s)
					if s != "" {
						out = append(out, s)
					}
				}
				if len(out) > 0 {
					return out
				}
			case []any:
				for _, it := range vv {
					if s := getStr(it); s != "" {
						out = append(out, s)
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
	}

	if res, ok := data["results"].([]map[string]interface{}); ok {
		for _, m := range res {
			if s := getStr(m["path"]); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if res, ok := data["results"].([]any); ok {
		for _, it := range res {
			m, ok := it.(map[string]interface{})
			if !ok {
				continue
			}
			if s := getStr(m["path"]); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}

	if params, ok := data["params"].(map[string]interface{}); ok {
		if s := getStr(params["path"]); s != "" {
			return []string{s}
		}
		if s := getStr(params["file"]); s != "" {
			return []string{s}
		}
	}

	return nil
}

func getToolDisplayName(tool string, data map[string]interface{}) string {
	switch tool {
	case "read":
		return "Read"
	case "write_file":
		return "Write"
	case "edit":
		return "Edit"
	case "search":
		return "Search"
	case "grep":
		return "Grep"
	case "list":
		return "List"
	case "fs":
		if data != nil {
			if mode, ok := data["mode"].(string); ok {
				switch mode {
				case "write":
					return "Write"
				case "create":
					if t, ok := data["type"].(string); ok && t == "dir" {
						return "Mkdir"
					}
					return "Create"
				case "mkdir":
					return "Mkdir"
				case "delete":
					return "Delete"
				case "move":
					return "Move"
				case "copy":
					return "Copy"
				case "diff":
					return "Diff"
				}
			}
		}
		return "FS"
	case "bash_session", "bash":
		return "Bash"
	case "git":
		return "Git"
	default:
		return toPascalToolName(tool)
	}
}

func toPascalToolName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if i := strings.LastIndex(raw, "."); i >= 0 && i+1 < len(raw) {
		raw = raw[i+1:]
	}
	if !strings.Contains(raw, "_") && !strings.Contains(raw, "-") {
		if raw != strings.ToLower(raw) {
			return raw
		}
	}
	l := strings.ToLower(raw)
	known := map[string]string{
		"fs":            "FS",
		"mcp":           "MCP",
		"lsp":           "LSP",
		"ui":            "UI",
		"time_now":      "TimeNow",
		"runcommand":    "RunCommand",
		"searchcodebase": "SearchCodebase",
		"websearch":     "WebSearch",
		"webfetch":      "WebFetch",
	}
	if v, ok := known[l]; ok {
		return v
	}
	raw = strings.ReplaceAll(raw, "-", "_")
	parts := strings.Split(raw, "_")
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pl := strings.ToLower(p)
		switch pl {
		case "fs", "mcp", "lsp", "ui", "url", "http", "https", "json", "yaml", "xml", "sql":
			b.WriteString(strings.ToUpper(pl))
		default:
			b.WriteString(strings.ToUpper(pl[:1]))
			if len(pl) > 1 {
				b.WriteString(pl[1:])
			}
		}
	}
	if b.Len() == 0 {
		return raw
	}
	return b.String()
}

func (r ToolResult) extractSummary() string {
	if r.Display != "" {
		return TruncateDisplay(strings.TrimSpace(r.Display), 100)
	}

	if r.Data == nil {
		return ""
	}

	switch r.Tool {
	case "read":
		if mode, ok := r.Data["mode"].(string); ok {
			switch mode {
			case "file":
				switch v := r.Data["bytes"].(type) {
				case int:
					return fmt.Sprintf("%d bytes", v)
				case float64:
					return fmt.Sprintf("%d bytes", int(v))
				}
			case "directory":
				if entries, ok := r.Data["entries"].([]string); ok {
					return fmt.Sprintf("%d entries", len(entries))
				}
			case "exists":
				if exists, ok := r.Data["exists"].(bool); ok {
					return fmt.Sprintf("exists: %v", exists)
				}
			}
		}
	case "fs":
		if bytes, ok := r.Data["bytes_written"].(int); ok {
			return fmt.Sprintf("%d bytes written", bytes)
		}
	case "bash":
		if stdout, ok := r.Data["stdout"].(string); ok && stdout != "" {
			return TruncateDisplay(strings.TrimSpace(stdout), 100)
		}
	}

	return ""
}
