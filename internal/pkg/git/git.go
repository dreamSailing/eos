package git

import (
	"context"
	"fmt"
	"strings"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"
)

// UI 接口，用于回调 UI 显示和权限询问，解耦 UI 与逻辑
type UI interface {
	WriteLine(color, text string)
	PromptPermission(category, summary string) string
	IsAllowed(category string) bool
	AllowSession(category string)
	T(key string) string
	SetAbortCancel(cancel context.CancelFunc)
}

// Manager 负责处理 Git 相关的业务逻辑
type Manager struct {
	tools *tools.Manager
	ctxm  *session.ContextManager
}

// NewManager 创建新的 Git 管理器
func NewManager(tm *tools.Manager, cm *session.ContextManager) *Manager {
	return &Manager{
		tools: tm,
		ctxm:  cm,
	}
}

// HandleCommand 解析并处理 Git 命令
func (m *Manager) HandleCommand(ui UI, name []string) bool {
	if len(name) >= 2 {
		sub := strings.ToLower(name[1])
		var tc tools.ToolCall
		switch sub {
		case "status":
			tc = tools.ToolCall{Tool: "git_status", Parameters: map[string]interface{}{}}
		case "branches":
			tc = tools.ToolCall{Tool: "git_branch_list", Parameters: map[string]interface{}{}}
		case "add":
			var ps []interface{}
			if len(name) > 2 {
				for _, p := range name[2:] {
					ps = append(ps, p)
				}
			}
			tc = tools.ToolCall{Tool: "git_add", Parameters: map[string]interface{}{"paths": ps}}
		case "commit":
			msg := ""
			if len(name) > 2 {
				msg = strings.Join(name[2:], " ")
			}
			tc = tools.ToolCall{Tool: "git_commit", Parameters: map[string]interface{}{"message": msg}}
		case "checkout":
			if len(name) >= 3 {
				create := false
				if len(name) >= 4 {
					create = strings.ToLower(name[3]) == "create"
				}
				tc = tools.ToolCall{Tool: "git_checkout", Parameters: map[string]interface{}{"name": name[2], "create": create}}
			}
		case "init":
			tc = tools.ToolCall{Tool: "git_init", Parameters: map[string]interface{}{}}
		case "diff":
			if len(name) >= 3 {
				tc = tools.ToolCall{Tool: "git_diff", Parameters: map[string]interface{}{"path": name[2]}}
			}
		case "pull":
			remote := ""
			branch := ""
			if len(name) >= 3 {
				remote = name[2]
			}
			if len(name) >= 4 {
				branch = name[3]
			}
			tc = tools.ToolCall{Tool: "git_pull", Parameters: map[string]interface{}{"remote": remote, "branch": branch}}
		case "push":
			remote := ""
			branch := ""
			if len(name) >= 3 {
				remote = name[2]
			}
			if len(name) >= 4 {
				branch = name[3]
			}
			tc = tools.ToolCall{Tool: "git_push", Parameters: map[string]interface{}{"remote": remote, "branch": branch}}
		case "log":
			limit := 20
			oneline := true
			graph := false
			all := false
			path := ""
			for _, a := range name[2:] {
				al := strings.ToLower(strings.TrimSpace(a))
				if al == "" {
					continue
				}
				switch al {
				case "graph":
					graph = true
					continue
				case "all":
					all = true
					continue
				case "oneline":
					oneline = true
					continue
				case "full":
					oneline = false
					continue
				}
				if n, ok := tryParseInt(al); ok && n > 0 {
					limit = n
					continue
				}
				if path == "" {
					path = a
				}
			}
			tc = tools.ToolCall{Tool: "git_log", Parameters: map[string]interface{}{"limit": limit, "oneline": oneline, "graph": graph, "all": all, "path": path}}
		case "show":
			revision := "HEAD"
			path := ""
			if len(name) >= 3 {
				revision = strings.TrimSpace(name[2])
			}
			if len(name) >= 4 {
				path = strings.TrimSpace(name[3])
			}
			tc = tools.ToolCall{Tool: "git_show", Parameters: map[string]interface{}{"revision": revision, "path": path}}
		case "stash":
			action := "list"
			if len(name) >= 3 {
				action = strings.ToLower(strings.TrimSpace(name[2]))
			}
			idx := 0
			msg := ""
			includeUntracked := false
			rest := []string{}
			if len(name) > 3 {
				rest = name[3:]
			}
			if action == "save" {
				for _, a := range rest {
					if strings.EqualFold(a, "-u") || strings.EqualFold(a, "--include-untracked") {
						includeUntracked = true
						continue
					}
					msg += a + " "
				}
				msg = strings.TrimSpace(msg)
			} else if action == "pop" || action == "apply" || action == "drop" {
				if len(rest) > 0 {
					if n, ok := tryParseInt(rest[0]); ok {
						idx = n
						rest = rest[1:]
					}
				}
			}
			tc = tools.ToolCall{Tool: "git_stash", Parameters: map[string]interface{}{"action": action, "message": msg, "index": idx, "include_untracked": includeUntracked}}
		case "reset":
			if len(name) >= 4 {
				mode := strings.ToLower(strings.TrimSpace(name[2]))
				target := strings.TrimSpace(name[3])
				tc = tools.ToolCall{Tool: "git_reset", Parameters: map[string]interface{}{"mode": mode, "target": target}}
			}
		case "revert":
			if len(name) >= 3 {
				tc = tools.ToolCall{Tool: "git_revert", Parameters: map[string]interface{}{"commit": name[2], "no_edit": true}}
			}
		case "merge":
			if len(name) >= 3 {
				tc = tools.ToolCall{Tool: "git_merge", Parameters: map[string]interface{}{"branch": name[2], "no_edit": true}}
			}
		case "rebase":
			if len(name) >= 3 {
				act := strings.ToLower(strings.TrimSpace(name[2]))
				if act == "continue" || act == "abort" || act == "skip" {
					tc = tools.ToolCall{Tool: "git_rebase", Parameters: map[string]interface{}{"action": act}}
				} else {
					tc = tools.ToolCall{Tool: "git_rebase", Parameters: map[string]interface{}{"action": "start", "upstream": name[2]}}
				}
			}
		}
		if tc.Tool != "" {
			return m.ExecuteToolCall(ui, tc)
		}
	}
	ui.WriteLine("yellow", "Usage: /git status|branches|add <paths...>|commit <msg>|checkout <name> [create]|init|diff <path>|pull [remote] [branch]|push [remote] [branch]|log [graph] [all] [limit] [path]|show [revision] [path]|stash [list|save|pop|apply|drop] [args...]|reset <soft|mixed|hard> <target>|revert <commit>|merge <branch>|rebase <upstream>|rebase <continue|abort|skip>")
	return false
}

func tryParseInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// ExecuteToolCall 执行 Git 工具调用，包含安全检查和结果显示
func (m *Manager) ExecuteToolCall(ui UI, tc tools.ToolCall) bool {
	ctxW, cancel := context.WithCancel(context.Background())
	ui.SetAbortCancel(cancel)

	go func() {
		cat, _, sum, dang := tools.ClassifyToolDanger(tc)
		if dang && !ui.IsAllowed(cat) {
			dec := ui.PromptPermission(cat, sum)
			if dec == "deny" {
				ui.WriteLine("white", fmt.Sprintf(ui.T("denied"), sum))
				return
			}
			if dec == "session" {
				ui.AllowSession(cat)
			}
		}

		srs := m.tools.ExecuteStructured(ctxW, []tools.ToolCall{tc})
		for _, r := range srs {
			out := r.Display
			if strings.TrimSpace(out) == "" {
				out = fmt.Sprintf("Status: %s", r.Status)
			}
			ui.WriteLine("white", out)
			m.ctxm.AddToolObservation(r)
		}
	}()

	return true
}
