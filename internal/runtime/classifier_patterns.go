package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

// defaultDenyRules returns rules for dangerous commands that always require confirmation
func defaultDenyRules() []ClassifierRule {
	return []ClassifierRule{
		// File system destruction
		{Pattern: `rm\s+(-[a-zA-Z]*f[a-zA-Z]*\s+|--no-preserve-root)`, Action: ActionDeny, Category: "bash", Description: "dangerous rm flags", Source: "default"},
		{Pattern: `del\s+/[sS]`, Action: ActionDeny, Category: "bash", Description: "recursive delete (Windows)", Source: "default"},
		{Pattern: `rmdir\s+/[sS]`, Action: ActionDeny, Category: "bash", Description: "recursive rmdir (Windows)", Source: "default"},
		{Pattern: `mkfs`, Action: ActionDeny, Category: "bash", Description: "format filesystem", Source: "default"},
		{Pattern: `format\s+[a-zA-Z]:`, Action: ActionDeny, Category: "bash", Description: "format drive (Windows)", Source: "default"},

		// Git destructive operations
		{Pattern: `git\s+push\s+.*--force`, Action: ActionDeny, Category: "git-push", Description: "force push", Source: "default"},
		{Pattern: `git\s+push\s+.*-f\b`, Action: ActionDeny, Category: "git-push", Description: "force push (short flag)", Source: "default"},
		{Pattern: `git\s+reset\s+--hard`, Action: ActionDeny, Category: "git-reset", Description: "hard reset", Source: "default"},
		{Pattern: `git\s+clean\s+-`, Action: ActionDeny, Category: "git", Description: "git clean", Source: "default"},

		// Privilege escalation
		{Pattern: `sudo\s+`, Action: ActionDeny, Category: "bash", Description: "sudo command", Source: "default"},
		{Pattern: `runas\s+`, Action: ActionDeny, Category: "bash", Description: "runas command (Windows)", Source: "default"},
		{Pattern: `chmod\s+777`, Action: ActionDeny, Category: "bash", Description: "chmod 777", Source: "default"},
		{Pattern: `icacls\s+.*grant\s+.*:F`, Action: ActionDeny, Category: "bash", Description: "full permission grant (Windows)", Source: "default"},

		// Pipe to shell
		{Pattern: `curl\s+.*\|\s*(ba)?sh`, Action: ActionDeny, Category: "bash", Description: "curl pipe to shell", Source: "default"},
		{Pattern: `wget\s+.*\|\s*(ba)?sh`, Action: ActionDeny, Category: "bash", Description: "wget pipe to shell", Source: "default"},

		// Category-based deny rules for auto mode
		{Pattern: "tool-delete", Action: ActionDeny, Category: "tool-delete", Description: "delete operations", Source: "default"},
		{Pattern: "delete_file", Action: ActionDeny, Category: "delete_file", Description: "file deletion", Source: "default"},
		{Pattern: "git-push", Action: ActionDeny, Category: "git-push", Description: "git push", Source: "default"},
		{Pattern: "git-reset", Action: ActionDeny, Category: "git-reset", Description: "git reset", Source: "default"},
		{Pattern: "git-revert", Action: ActionDeny, Category: "git-revert", Description: "git revert", Source: "default"},
		{Pattern: "git-merge", Action: ActionDeny, Category: "git-merge", Description: "git merge", Source: "default"},
		{Pattern: "git-rebase", Action: ActionDeny, Category: "git-rebase", Description: "git rebase", Source: "default"},
		{Pattern: "git-stash-apply", Action: ActionDeny, Category: "git-stash-apply", Description: "git stash apply", Source: "default"},
		{Pattern: "git-stash-drop", Action: ActionDeny, Category: "git-stash-drop", Description: "git stash drop", Source: "default"},
		{Pattern: "bg-task-start", Action: ActionDeny, Category: "bg-task-start", Description: "background task start", Source: "default"},
		{Pattern: "bg-task-kill", Action: ActionDeny, Category: "bg-task-kill", Description: "background task kill", Source: "default"},
		{Pattern: "bash-session-kill", Action: ActionDeny, Category: "bash-session-kill", Description: "bash session kill", Source: "default"},
		{Pattern: "browser-high-risk", Action: ActionDeny, Category: "browser-high-risk", Description: "high risk browser action", Source: "default"},

		// Restore checkpoint
		{Pattern: `restore\s+checkpoint`, Action: ActionDeny, Category: "history", Description: "restore checkpoint", Source: "default"},
	}
}

// defaultAllowRules returns rules for safe commands that auto-allowed
func defaultAllowRules() []ClassifierRule {
	return []ClassifierRule{
		// File reading tools (always safe)
		{Pattern: "read", Action: ActionAllow, Category: "file", Description: "read file/directory", Source: "default"},
		{Pattern: "search", Action: ActionAllow, Category: "search", Description: "search files/content", Source: "default"},
		{Pattern: "tool_search", Action: ActionAllow, Category: "meta", Description: "search tools", Source: "default"},
		{Pattern: "time_now", Action: ActionAllow, Category: "meta", Description: "get current time", Source: "default"},
		{Pattern: "ProjectStructure", Action: ActionAllow, Category: "meta", Description: "project structure", Source: "default"},
		{Pattern: "plan_steps", Action: ActionAllow, Category: "planning", Description: "plan steps", Source: "default"},
		{Pattern: "todo_read", Action: ActionAllow, Category: "planning", Description: "read todo list", Source: "default"},
		{Pattern: "skills_list", Action: ActionAllow, Category: "meta", Description: "list skills", Source: "default"},
		{Pattern: "mcp_status", Action: ActionAllow, Category: "meta", Description: "MCP status", Source: "default"},
		{Pattern: "browser_status", Action: ActionAllow, Category: "browser", Description: "browser status", Source: "default"},
		{Pattern: "browser_snapshot", Action: ActionAllow, Category: "browser", Description: "browser snapshot", Source: "default"},
		{Pattern: "browser_inspect", Action: ActionAllow, Category: "browser", Description: "browser inspect", Source: "default"},
		{Pattern: "browser_console", Action: ActionAllow, Category: "browser", Description: "browser console logs", Source: "default"},
		{Pattern: "browser_network", Action: ActionAllow, Category: "browser", Description: "browser network logs", Source: "default"},
		{Pattern: "browser_dev_logs", Action: ActionAllow, Category: "browser", Description: "browser development logs", Source: "default"},
		{Pattern: "browser_downloads", Action: ActionAllow, Category: "browser", Description: "browser download events", Source: "default"},
		{Pattern: "browser_user_tabs", Action: ActionAllow, Category: "browser", Description: "browser user tabs", Source: "default"},
		{Pattern: "git_status", Action: ActionAllow, Category: "git", Description: "git status", Source: "default"},
		{Pattern: "git_branch_list", Action: ActionAllow, Category: "git", Description: "list branches", Source: "default"},
		{Pattern: "git_diff", Action: ActionAllow, Category: "git", Description: "git diff", Source: "default"},
		{Pattern: "git_log", Action: ActionAllow, Category: "git", Description: "git log", Source: "default"},
		{Pattern: "git_show", Action: ActionAllow, Category: "git", Description: "git show", Source: "default"},
		{Pattern: "enter_plan_mode", Action: ActionAllow, Category: "mode", Description: "enter plan mode", Source: "default"},
		{Pattern: "exit_plan_mode", Action: ActionAllow, Category: "mode", Description: "exit plan mode", Source: "default"},

		// Safe bash commands
		{Pattern: `^ls\b`, Action: ActionAllow, Category: "bash", Description: "list directory", Source: "default"},
		{Pattern: `^dir\b`, Action: ActionAllow, Category: "bash", Description: "list directory (Windows)", Source: "default"},
		{Pattern: `^cat\b`, Action: ActionAllow, Category: "bash", Description: "cat file", Source: "default"},
		{Pattern: `^head\b`, Action: ActionAllow, Category: "bash", Description: "head file", Source: "default"},
		{Pattern: `^tail\b`, Action: ActionAllow, Category: "bash", Description: "tail file", Source: "default"},
		{Pattern: `^type\b`, Action: ActionAllow, Category: "bash", Description: "type file (Windows)", Source: "default"},
		{Pattern: `^echo\b`, Action: ActionAllow, Category: "bash", Description: "echo text", Source: "default"},
		{Pattern: `^pwd\b`, Action: ActionAllow, Category: "bash", Description: "print working directory", Source: "default"},
		{Pattern: `^cd\b`, Action: ActionAllow, Category: "bash", Description: "change directory", Source: "default"},
		{Pattern: `^which\b`, Action: ActionAllow, Category: "bash", Description: "which command", Source: "default"},
		{Pattern: `^where\b`, Action: ActionAllow, Category: "bash", Description: "where command (Windows)", Source: "default"},
		{Pattern: `^git\s+status\b`, Action: ActionAllow, Category: "bash", Description: "git status via bash", Source: "default"},
		{Pattern: `^git\s+log\b`, Action: ActionAllow, Category: "bash", Description: "git log via bash", Source: "default"},
		{Pattern: `^git\s+diff\b`, Action: ActionAllow, Category: "bash", Description: "git diff via bash", Source: "default"},
		{Pattern: `^git\s+branch\b`, Action: ActionAllow, Category: "bash", Description: "git branch list via bash", Source: "default"},
		{Pattern: `^go\s+build\b`, Action: ActionAllow, Category: "bash", Description: "go build", Source: "default"},
		{Pattern: `^go\s+test\b`, Action: ActionAllow, Category: "bash", Description: "go test", Source: "default"},
		{Pattern: `^go\s+vet\b`, Action: ActionAllow, Category: "bash", Description: "go vet", Source: "default"},
		{Pattern: `^npm\s+list\b`, Action: ActionAllow, Category: "bash", Description: "npm list", Source: "default"},
		{Pattern: `^pip\s+list\b`, Action: ActionAllow, Category: "bash", Description: "pip list", Source: "default"},
		{Pattern: `^node\s+--version\b`, Action: ActionAllow, Category: "bash", Description: "node version", Source: "default"},
		{Pattern: `^python\s+--version\b`, Action: ActionAllow, Category: "bash", Description: "python version", Source: "default"},
		{Pattern: `^go\s+version\b`, Action: ActionAllow, Category: "bash", Description: "go version", Source: "default"},
	}
}
