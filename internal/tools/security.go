package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"os"
	"path/filepath"
	"strings"
)

// ClassifyToolDanger 分类工具危险等级
func ClassifyToolDanger(call ToolCall) (category string, level string, summary string, dangerous bool) {
	switch call.Tool {
	case "fs":
		mode, _ := call.Parameters["mode"].(string)
		if mode == "" {
			mode = "write"
		}
		switch mode {
		case "diff":
			return "", "low", "fs diff", false
		case "write":
			if p, ok := call.Parameters["path"].(string); ok {
				wd, _ := os.Getwd()
				ap := p
				if !filepath.IsAbs(ap) {
					ap = filepath.Join(wd, filepath.FromSlash(p))
				}
				return "file_write", "medium", "fs write " + filepath.ToSlash(ap), true
			}
			return "file_write", "medium", "fs write", true
		case "create", "mkdir":
			if p, ok := call.Parameters["path"].(string); ok {
				wd, _ := os.Getwd()
				ap := p
				if !filepath.IsAbs(ap) {
					ap = filepath.Join(wd, filepath.FromSlash(p))
				}
				return "file_write", "medium", "fs create " + filepath.ToSlash(ap), true
			}
			return "file_write", "medium", "fs create", true
		case "delete":
			if p, ok := call.Parameters["path"].(string); ok {
				wd, _ := os.Getwd()
				ap := p
				if !filepath.IsAbs(ap) {
					ap = filepath.Join(wd, filepath.FromSlash(p))
				}
				return "tool-delete", "high", "fs delete " + filepath.ToSlash(ap), true
			}
			return "tool-delete", "high", "fs delete", true
		case "move":
			_, okS := call.Parameters["source"].(string)
			dst, okD := call.Parameters["destination"].(string)
			if okS && okD {
				wd, _ := os.Getwd()
				d := dst
				if !filepath.IsAbs(d) {
					d = filepath.Join(wd, filepath.FromSlash(dst))
				}
				if fi, err := os.Stat(d); err == nil {
					cat := "move-overwrite"
					if fi.IsDir() {
						cat = "move-dir-overwrite"
					}
					return cat, "medium", "fs move to existing " + filepath.ToSlash(d), true
				}
				return "move-dir", "medium", "fs move to " + filepath.ToSlash(d), true
			}
			return "move-dir", "medium", "fs move", true
		case "copy":
			dst, okD := call.Parameters["destination"].(string)
			if okD {
				wd, _ := os.Getwd()
				d := dst
				if !filepath.IsAbs(d) {
					d = filepath.Join(wd, filepath.FromSlash(dst))
				}
				if _, err := os.Stat(d); err == nil {
					return "copy-overwrite", "medium", "fs copy overwrite " + filepath.ToSlash(d), true
				}
				return "copy-dir", "medium", "fs copy to " + filepath.ToSlash(d), true
			}
			return "copy-dir", "medium", "fs copy", true
		}
		return "unknown", "medium", "fs " + mode, true
	case ToolEdit:
		mode, _ := call.Parameters["mode"].(string)
		mode = strings.ToLower(strings.TrimSpace(mode))
		switch mode {
		case "batch":
			return "overwrite_file", "medium", "edit batch", true
		case "multi", "single":
			return "overwrite_file", "medium", "edit " + mode, true
		}
		return "overwrite_file", "medium", "edit", true
	case ToolHistory:
		mode, _ := call.Parameters["mode"].(string)
		mode = strings.ToLower(strings.TrimSpace(mode))
		switch mode {
		case "list_files", "list_versions", "read_version", "list_checkpoints":
			return "", "low", "history " + mode, false
		case "rollback":
			return "file_write", "medium", "history rollback", true
		case "restore_checkpoint":
			return "file_write", "high", "history restore checkpoint", true
		}
		return "unknown", "medium", "history", true
	case "bash":
		if c, ok := call.Parameters["command"].(string); ok {
			cat, lvl, sum, dang := ClassifyBashDanger(c)
			if dang {
				return cat, lvl, sum, true
			}
		}
	case "bash_session":
		mode, _ := call.Parameters["mode"].(string)
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "start":
			if c, ok := call.Parameters["command"].(string); ok {
				cat, lvl, sum, dang := ClassifyBashDanger(c)
				if dang {
					return cat, lvl, sum, true
				}
			}
		case "kill":
			id, _ := call.Parameters["id"].(string)
			return "bash-session-kill", "high", "bash_session kill " + id, true
		}
	case ToolBGTask:
		action, _ := call.Parameters["action"].(string)
		switch strings.ToLower(strings.TrimSpace(action)) {
		case "start":
			if c, ok := call.Parameters["command"].(string); ok {
				cat, lvl, sum, dang := ClassifyBashDanger(c)
				if dang {
					return cat, lvl, sum, true
				}
			}
			return "bg-task-start", "high", "bg_task start", true
		case "kill":
			id, _ := call.Parameters["id"].(string)
			return "bg-task-kill", "high", "bg_task kill " + id, true
		}
	case ToolGitPush:
		return "git-push", "high", "git push", true
	case ToolGitPull:
		return "git-pull", "medium", "git pull", true
	case ToolRemoteRepoConnect:
		platform, _ := call.Parameters["platform"].(string)
		return "remote-repo-connect", "medium", "remote repo connect " + strings.TrimSpace(platform), true
	case ToolRemoteRepoCloneOrOpen:
		repoURL, _ := call.Parameters["repo_url"].(string)
		return "remote-repo-open", "high", "remote repo open " + strings.TrimSpace(repoURL), true
	case ToolRemoteRepoCheckout:
		branch, _ := call.Parameters["branch"].(string)
		return "remote-repo-checkout", "medium", "remote repo checkout " + strings.TrimSpace(branch), true
	case ToolRemoteRepoCommitAndPush:
		return "remote-repo-push", "high", "remote repo commit and push", true
	case ToolRemoteRepoCreatePR:
		return "remote-repo-pr", "high", "remote repo create pr", true
	case ToolRemoteRepoCreateMR:
		return "remote-repo-mr", "high", "remote repo create mr", true
	case ToolRemoteRepoDisconnect:
		return "remote-repo-disconnect", "medium", "remote repo disconnect", true
	case ToolRemoteRepoStatus:
		return "", "low", "remote repo status", false
	case ToolGitReset:
		if t, ok := call.Parameters["target"].(string); ok && strings.TrimSpace(t) != "" {
			return "git-reset", "high", "git reset " + strings.TrimSpace(t), true
		}
		return "git-reset", "high", "git reset", true
	case ToolGitRevert:
		if c, ok := call.Parameters["commit"].(string); ok && strings.TrimSpace(c) != "" {
			return "git-revert", "high", "git revert " + strings.TrimSpace(c), true
		}
		return "git-revert", "high", "git revert", true
	case ToolGitMerge:
		if b, ok := call.Parameters["branch"].(string); ok && strings.TrimSpace(b) != "" {
			return "git-merge", "high", "git merge " + strings.TrimSpace(b), true
		}
		return "git-merge", "high", "git merge", true
	case ToolGitRebase:
		action, _ := call.Parameters["action"].(string)
		action = strings.ToLower(strings.TrimSpace(action))
		if action == "" {
			action = "start"
		}
		return "git-rebase", "high", "git rebase " + action, true
	case ToolGitStash:
		action, _ := call.Parameters["action"].(string)
		action = strings.ToLower(strings.TrimSpace(action))
		switch action {
		case "list":
			return "", "low", "git stash list", false
		case "save":
			return "git-stash-save", "medium", "git stash save", true
		case "apply", "pop":
			return "git-stash-apply", "high", "git stash " + action, true
		case "drop":
			return "git-stash-drop", "high", "git stash drop", true
		default:
			return "git-stash", "high", "git stash", true
		}
	case ToolGitCheckout:
		if name, ok := call.Parameters["name"].(string); ok {
			return "git-checkout", "medium", "git checkout " + name, true
		}
		return "git-checkout", "medium", "git checkout", true
	case ToolGitCommit:
		return "git-commit", "medium", "git commit", true
	case ToolGitInit:
		return "git-init", "medium", "git init", true
	case ToolGitAdd:
		return "git-add", "low", "git add", false
	case ToolGitStatus, ToolGitBranchList, ToolGitDiff, ToolGitLog, ToolGitShow:
		return "", "low", call.Tool, false
	case ToolWebSearch:
		return "web:search", "low", "web search: " + extractParamSummary(call.Parameters), false
	case ToolWebFetch:
		url, _ := call.Parameters["url"].(string)
		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://localhost") {
			return "web:fetch", "medium", "non-HTTPS web fetch: " + url, true
		}
		return "web:fetch", "low", "web fetch: " + url, false
	case ToolSuggestMemory:
		return "memory:suggest", "low", "suggest memory", false
	case ToolEnterWorktree:
		return "git:worktree", "medium", "create git worktree", false
	case ToolMCPListResources, ToolMCPReadResource, ToolStructuredOutput, ToolSnip, ToolBrowserStatus:
		return "", "low", call.Tool, false
	case ToolPowerShell:
		return "shell:powershell", "high", "powershell", true
	case ToolTeamCreate, ToolTeamDelete:
		return "team", "medium", call.Tool, true
	case ToolTeamSendMsg:
		return "", "low", "team_send_message", false
	}

	return "unknown", "low", call.Tool, false
}

// extractParamSummary extracts a short summary from tool parameters for display
func extractParamSummary(params map[string]interface{}) string {
	if params == nil {
		return ""
	}
	// Try common parameter names
	for _, key := range []string{"query", "command", "path", "url", "message"} {
		if v, ok := params[key].(string); ok && v != "" {
			if len(v) > 80 {
				return v[:80] + "..."
			}
			return v
		}
	}
	return ""
}

// ClassifyBashDanger 分类 Bash 命令危险等级
func ClassifyBashDanger(cmd string) (category string, level string, summary string, dangerous bool) {
	raw := strings.TrimSpace(cmd)
	s := strings.ToLower(raw)
	fields := strings.Fields(s)
	head := ""
	if len(fields) > 0 {
		head = fields[0]
	}

	deleteHeads := map[string]bool{
		"rm":          true,
		"del":         true,
		"rmdir":       true,
		"rd":          true,
		"remove-item": true,
		"erase":       true,
	}
	if deleteHeads[head] {
		return "delete_file", "high", raw, true
	}

	if strings.Contains(s, "-recurse") || strings.Contains(s, "-force") || strings.Contains(s, "/s") || strings.Contains(s, "/q") || strings.Contains(s, "-rf") {
		return "delete_file", "high", raw, true
	}

	if strings.Contains(s, " > ") || strings.Contains(s, ">>") || strings.Contains(s, "| tee") ||
		strings.Contains(s, " out-file") || strings.Contains(s, " set-content") || strings.Contains(s, " add-content") ||
		strings.Contains(s, " redirection") {
		return "overwrite_file", "medium", raw, true
	}

	networkHeads := map[string]bool{
		"wget":              true,
		"curl":              true,
		"invoke-webrequest": true,
		"iwr":               true,
		"invoke-restmethod": true,
		"irm":               true,
	}
	if networkHeads[head] {
		return "network", "medium", raw, true
	}

	if head == "powershell" || head == "pwsh" {
		if strings.Contains(s, "-encodedcommand") || strings.Contains(s, "iex") {
			return "system", "high", raw, true
		}
		return "system", "medium", raw, true
	}

	return "", "", raw, false
}
