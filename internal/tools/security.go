package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
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
	case ToolPatch:
		mode, _ := call.Parameters["mode"].(string)
		if strings.EqualFold(strings.TrimSpace(mode), "dry_run") {
			return "", "low", "patch dry_run", false
		}
		return "overwrite_file", "medium", "patch", true
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
	case ToolMCPListResources, ToolMCPReadResource, ToolStructuredOutput, ToolSnip, ToolBrowserStatus, ToolBrowserUserTabs, ToolBrowserDevLogs, ToolBrowserDownloads:
		return "", "low", call.Tool, false
	case ToolBrowserNavigate, ToolBrowserSnapshot, ToolBrowserInspect, ToolBrowserTabs, ToolBrowserWait, ToolBrowserConsole, ToolBrowserNetwork, ToolBrowserBack, ToolBrowserForward, ToolBrowserHover, ToolBrowserPressKey, ToolBrowserScroll, ToolBrowserReload, ToolBrowserViewport, ToolBrowserVisibility, ToolBrowserSessionName:
		if browserActionLooksHighRisk(call.Parameters) {
			return "browser-high-risk", "high", "browser action: " + extractParamSummary(call.Parameters), true
		}
		return "browser", "medium", call.Tool, false
	case ToolBrowserClick, ToolBrowserType, ToolBrowserSelect, ToolBrowserScreenshot, ToolBrowserClipboard, ToolBrowserCUA, ToolBrowserDOMCUA, ToolBrowserLocator:
		if browserActionLooksHighRisk(call.Parameters) {
			return "browser-high-risk", "high", "browser action: " + extractParamSummary(call.Parameters), true
		}
		return "browser", "medium", call.Tool, false
	case ToolPowerShell:
		return "shell:powershell", "high", "powershell", true
	case ToolDocumentGenerate:
		return "office:generate", "medium", "document generate", true
	case ToolDocumentConvert:
		return "office:convert", "medium", "document convert", true
	case ToolTeamCreate, ToolTeamDelete:
		return "team", "medium", call.Tool, true
	case ToolTeamSendMsg:
		return "", "low", "team_send_message", false
	}

	return "unknown", "low", call.Tool, false
}

func AccessModeAllowsToolCall(ctx context.Context, call ToolCall) (bool, string) {
	switch normalizeAccessMode(AccessModeFromContext(ctx)) {
	case "", "danger-full-access":
		return true, ""
	case "workspace-write":
		if reason := workspaceWriteBoundaryViolation(ctx, call); reason != "" {
			return false, reason
		}
		return true, ""
	case "read-only":
		if isReadOnlyToolCall(call) {
			return true, ""
		}
		return false, "access mode read-only blocks mutating tools"
	default:
		return true, ""
	}
}

func workspaceWriteBoundaryViolation(ctx context.Context, call ToolCall) string {
	workspaceRoot := strings.TrimSpace(WorkspaceRootFromContext(ctx))
	if workspaceRoot == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(call.Tool)) {
	case strings.ToLower(ToolFS), strings.ToLower(ToolEdit), strings.ToLower(ToolNotebookEdit):
		for _, key := range []string{"path", "file", "source", "destination"} {
			if reason := boundaryViolationForParam(workspaceRoot, key, call.Parameters); reason != "" {
				return reason
			}
		}
	case strings.ToLower(ToolPatch):
		if patchesRaw, ok := call.Parameters["patches"].([]interface{}); ok {
			for _, pRaw := range patchesRaw {
				pm, ok := pRaw.(map[string]interface{})
				if !ok {
					continue
				}
				if p, ok := pm["path"].(string); ok {
					abs := resolveBoundaryPath(workspaceRoot, p)
					if abs != "" && !isPathAllowedInWorkspaceWrite(workspaceRoot, abs) {
						return fmt.Sprintf("access mode workspace-write blocks writes outside workspace or temporary directories: %s", filepath.ToSlash(abs))
					}
				}
			}
		}
	case strings.ToLower(ToolBGTask):
		action, _ := call.Parameters["action"].(string)
		if strings.EqualFold(strings.TrimSpace(action), "start") {
			if reason := boundaryViolationForParam(workspaceRoot, "working_dir", call.Parameters); reason != "" {
				return reason
			}
			if command, _ := call.Parameters["command"].(string); strings.TrimSpace(command) != "" {
				if reason := shellCommandBoundaryViolation(workspaceRoot, command); reason != "" {
					return reason
				}
			}
		}
	case strings.ToLower(ToolBash):
		command, _ := call.Parameters["command"].(string)
		return shellCommandBoundaryViolation(workspaceRoot, command)
	case strings.ToLower(ToolBashSession):
		mode, _ := call.Parameters["mode"].(string)
		if strings.EqualFold(strings.TrimSpace(mode), "start") {
			command, _ := call.Parameters["command"].(string)
			return shellCommandBoundaryViolation(workspaceRoot, command)
		}
	case strings.ToLower(ToolPowerShell):
		command, _ := call.Parameters["command"].(string)
		return shellCommandBoundaryViolation(workspaceRoot, command)
	case strings.ToLower(ToolBrowserScreenshot):
		if reason := boundaryViolationForParam(workspaceRoot, "path", call.Parameters); reason != "" {
			return reason
		}
	}
	return ""
}

func boundaryViolationForParam(workspaceRoot, key string, params map[string]any) string {
	if params == nil {
		return ""
	}
	raw, _ := params[key].(string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	abs := resolveBoundaryPath(workspaceRoot, raw)
	if abs == "" {
		return ""
	}
	if isPathAllowedInWorkspaceWrite(workspaceRoot, abs) {
		return ""
	}
	return fmt.Sprintf("access mode workspace-write blocks writes outside workspace or temporary directories: %s", filepath.ToSlash(abs))
}

func shellCommandBoundaryViolation(workspaceRoot, command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	lower := strings.ToLower(command)
	if strings.Contains(lower, "npm install -g") ||
		strings.Contains(lower, "pnpm add -g") ||
		strings.Contains(lower, "yarn global add") ||
		strings.Contains(lower, "pip install --user") ||
		strings.Contains(lower, "sudo ") ||
		strings.Contains(lower, "apt install") ||
		strings.Contains(lower, "apt-get install") ||
		strings.Contains(lower, "brew install") ||
		strings.Contains(lower, "yum install") ||
		strings.Contains(lower, "dnf install") ||
		strings.Contains(lower, "pacman -s") {
		return "access mode workspace-write blocks global system changes"
	}
	for _, token := range strings.Fields(command) {
		cleaned := strings.Trim(token, "\"'`,;()[]{}")
		if cleaned == "" {
			continue
		}
		if strings.HasPrefix(cleaned, "~/") {
			return fmt.Sprintf("access mode workspace-write blocks writes outside workspace or temporary directories: %s", cleaned)
		}
		if strings.HasPrefix(cleaned, "/") && !filepath.IsAbs(cleaned) && strings.Contains(strings.TrimPrefix(cleaned, "/"), "/") {
			return fmt.Sprintf("access mode workspace-write blocks writes outside workspace or temporary directories: %s", cleaned)
		}
		if !filepath.IsAbs(cleaned) {
			continue
		}
		abs := filepath.Clean(cleaned)
		if isPathAllowedInWorkspaceWrite(workspaceRoot, abs) {
			continue
		}
		return fmt.Sprintf("access mode workspace-write blocks writes outside workspace or temporary directories: %s", filepath.ToSlash(abs))
	}
	return ""
}

func resolveBoundaryPath(workspaceRoot, raw string) string {
	raw = normalizePathPlaceholder(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	joined := filepath.Join(workspaceRoot, filepath.FromSlash(raw))
	abs, err := filepath.Abs(joined)
	if err != nil {
		return filepath.Clean(joined)
	}
	return filepath.Clean(abs)
}

func isPathAllowedInWorkspaceWrite(workspaceRoot, target string) bool {
	target = filepath.Clean(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	if isSubpath(workspaceRoot, target) {
		return true
	}
	for _, dir := range allowedTemporaryDirs() {
		if isSubpath(dir, target) {
			return true
		}
	}
	return false
}

func allowedTemporaryDirs() []string {
	candidates := []string{
		os.TempDir(),
		os.Getenv("TMPDIR"),
		os.Getenv("TMP"),
		os.Getenv("TEMP"),
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(candidates))
	for _, dir := range candidates {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(dir)
		if err == nil {
			dir = abs
		}
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	return out
}

func isSubpath(base, target string) bool {
	base = filepath.Clean(strings.TrimSpace(base))
	target = filepath.Clean(strings.TrimSpace(target))
	if base == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func IsSandboxPolicyError(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "workspace-write blocks") ||
		strings.Contains(normalized, "outside workspace") ||
		strings.Contains(normalized, "outside working directory") ||
		strings.Contains(normalized, "路径超出工作目录") ||
		strings.Contains(normalized, "outside_root")
}

func isReadOnlyToolCall(call ToolCall) bool {
	toolName := strings.ToLower(strings.TrimSpace(call.Tool))
	switch toolName {
	case strings.ToLower(ToolRead),
		strings.ToLower(ToolSearch),
		strings.ToLower(ToolToolSearch),
		strings.ToLower(ToolSkillsList),
		strings.ToLower(ToolTimeNow),
		strings.ToLower(ToolTodoRead),
		strings.ToLower(ToolMCPStatus),
		strings.ToLower(ToolBrowserStatus),
		strings.ToLower(ToolBrowserSnapshot),
		strings.ToLower(ToolBrowserInspect),
		strings.ToLower(ToolBrowserConsole),
		strings.ToLower(ToolBrowserNetwork),
		strings.ToLower(ToolBrowserDevLogs),
		strings.ToLower(ToolBrowserDownloads),
		strings.ToLower(ToolBrowserUserTabs),
		strings.ToLower(ToolGitStatus),
		strings.ToLower(ToolGitBranchList),
		strings.ToLower(ToolGitDiff),
		strings.ToLower(ToolGitLog),
		strings.ToLower(ToolGitShow),
		strings.ToLower(ToolSuggestMemory),
		strings.ToLower(ToolWebSearch),
		strings.ToLower(ToolMCPListResources),
		strings.ToLower(ToolMCPReadResource),
		strings.ToLower(ToolStructuredOutput),
		strings.ToLower(ToolSnip),
		strings.ToLower(ToolRemoteRepoStatus):
		return true
	case strings.ToLower(ToolHistory):
		mode, _ := call.Parameters["mode"].(string)
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "list_files", "list_versions", "read_version", "list_checkpoints":
			return true
		default:
			return false
		}
	case strings.ToLower(ToolFS):
		mode, _ := call.Parameters["mode"].(string)
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "diff", "read", "exists", "directory", "resolve":
			return true
		default:
			return false
		}
	case strings.ToLower(ToolPatch):
		mode, _ := call.Parameters["mode"].(string)
		return strings.EqualFold(strings.TrimSpace(mode), "dry_run")
	case strings.ToLower(ToolGitAdd),
		strings.ToLower(ToolEdit),
		strings.ToLower(ToolBash),
		strings.ToLower(ToolBashSession),
		strings.ToLower(ToolBGTask),
		strings.ToLower(ToolPowerShell),
		strings.ToLower(ToolGitCommit),
		strings.ToLower(ToolGitCheckout),
		strings.ToLower(ToolGitInit),
		strings.ToLower(ToolGitPull),
		strings.ToLower(ToolGitPush),
		strings.ToLower(ToolGitStash),
		strings.ToLower(ToolGitReset),
		strings.ToLower(ToolGitRevert),
		strings.ToLower(ToolGitMerge),
		strings.ToLower(ToolGitRebase),
		strings.ToLower(ToolEnterWorktree),
		strings.ToLower(ToolExitWorktree),
		strings.ToLower(ToolNotebookEdit),
		strings.ToLower(ToolDocumentGenerate),
		strings.ToLower(ToolDocumentConvert),
		strings.ToLower(ToolImageGenerate),
		strings.ToLower(ToolVideoGenerate),
		strings.ToLower(ToolSpeechSynthesize),
		strings.ToLower(ToolTeamCreate),
		strings.ToLower(ToolTeamDelete),
		strings.ToLower(ToolRemoteRepoConnect),
		strings.ToLower(ToolRemoteRepoCloneOrOpen),
		strings.ToLower(ToolRemoteRepoCheckout),
		strings.ToLower(ToolRemoteRepoCommitAndPush),
		strings.ToLower(ToolRemoteRepoCreatePR),
		strings.ToLower(ToolRemoteRepoCreateMR),
		strings.ToLower(ToolRemoteRepoDisconnect),
		strings.ToLower(ToolBrowserNavigate),
		strings.ToLower(ToolBrowserTabs),
		strings.ToLower(ToolBrowserBack),
		strings.ToLower(ToolBrowserForward),
		strings.ToLower(ToolBrowserClick),
		strings.ToLower(ToolBrowserHover),
		strings.ToLower(ToolBrowserType),
		strings.ToLower(ToolBrowserPressKey),
		strings.ToLower(ToolBrowserSelect),
		strings.ToLower(ToolBrowserWait),
		strings.ToLower(ToolBrowserScroll),
		strings.ToLower(ToolBrowserScreenshot),
		strings.ToLower(ToolBrowserReload),
		strings.ToLower(ToolBrowserViewport),
		strings.ToLower(ToolBrowserVisibility),
		strings.ToLower(ToolBrowserClipboard),
		strings.ToLower(ToolBrowserCUA),
		strings.ToLower(ToolBrowserDOMCUA),
		strings.ToLower(ToolBrowserLocator),
		strings.ToLower(ToolBrowserSessionName):
		return false
	}
	_, _, _, dangerous := ClassifyToolDanger(call)
	return !dangerous
}

func normalizeAccessMode(mode string) string {
	key := strings.ToLower(strings.TrimSpace(mode))
	switch key {
	case "readonly", "read_only", "read-only":
		return "read-only"
	case "workspacewrite", "workspace_write", "workspace-write", "workspace", "sandbox":
		return "workspace-write"
	case "dangerfullaccess", "danger_full_access", "danger-full-access", "fullaccess", "full_access", "full-access":
		return "danger-full-access"
	default:
		return key
	}
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

func browserActionLooksHighRisk(params map[string]interface{}) bool {
	if params == nil {
		return false
	}
	var parts []string
	var walk func(interface{})
	walk = func(v interface{}) {
		switch x := v.(type) {
		case string:
			parts = append(parts, x)
		case []string:
			parts = append(parts, strings.Join(x, " "))
		case []interface{}:
			for _, item := range x {
				walk(item)
			}
		case map[string]interface{}:
			for k, item := range x {
				parts = append(parts, k)
				walk(item)
			}
		}
	}
	for key, value := range params {
		parts = append(parts, key)
		walk(value)
	}
	text := strings.ToLower(strings.Join(parts, " "))
	if strings.TrimSpace(text) == "" {
		return false
	}
	needles := []string{
		"delete", "remove", "destroy", "drop", "cancel subscription",
		"pay", "payment", "purchase", "checkout", "place order", "buy",
		"send", "email", "mail", "submit", "publish", "post",
		"upload", "attach", "file input",
		"token", "api key", "apikey", "secret", "private key", "password", "credential",
		"captcha", "2fa", "mfa", "verification code",
		"删除", "移除", "支付", "付款", "购买", "下单", "发送", "发信", "邮件", "上传", "密钥", "密码", "验证码",
	}
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
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
