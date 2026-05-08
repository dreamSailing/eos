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
	"time"

	"github.com/dreamSailing/eos/internal/config"
	gitops "github.com/dreamSailing/eos/internal/tools/git"
)

func (m *Manager) remoteRepoConnectStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	platform := strings.ToLower(strings.TrimSpace(asString(params["platform"])))
	if platform == "" {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoConnect, Status: "error", Error: "platform required"}
	}
	provider, err := remoteProviderFor(platform)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoConnect, Status: "error", Error: err.Error()}
	}
	if err := confirmRemoteAction(ctx, "连接远程仓库平台", fmt.Sprintf("确认授权 agent 连接 %s 账号吗？", platform)); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoConnect, Status: "error", Error: err.Error()}
	}
	cfg, cfgPath := loadRemoteConfig()
	auth, authHint, err := provider.ResolveToken(ctx, &cfg, params)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoConnect, Status: "error", Error: err.Error()}
	}
	if authHint != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolRemoteRepoConnect,
			Status:  authHint.Status,
			Data:    authHint.Data,
			Display: fmt.Sprintf("远程平台 %s 需要完成授权", platform),
		}
	}
	account, err := provider.CurrentUser(ctx, auth.AccessToken)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoConnect, Status: "error", Error: err.Error()}
	}
	auth.Platform = config.RemotePlatformType(platform)
	auth.AccountID = account.ID
	auth.AccountName = firstNonEmpty(account.Name, account.Login)
	auth.Login = firstNonEmpty(account.Login, auth.Login)
	ensureRemoteMaps(&cfg)
	cfg.RemoteAuth[platform] = auth
	if err := saveRemoteConfig(cfg, cfgPath); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoConnect, Status: "error", Error: err.Error()}
	}
	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolRemoteRepoConnect,
		Status: "success",
		Data: map[string]any{
			"platform":     platform,
			"account_id":   auth.AccountID,
			"account_name": auth.AccountName,
			"login":        auth.Login,
			"scope":        auth.Scope,
		},
		Display: fmt.Sprintf("已连接 %s 账号 %s", platform, firstNonEmpty(auth.Login, auth.AccountName)),
	}
}

func (m *Manager) remoteRepoStatusStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	_ = params
	traceID := TraceIDFromContext(ctx)
	remote, ok := GetRemoteRepoContext(traceID)
	cfg, _ := loadRemoteConfig()
	ensureRemoteMaps(&cfg)
	auths := make([]map[string]any, 0, len(cfg.RemoteAuth))
	for platform, auth := range cfg.RemoteAuth {
		auths = append(auths, map[string]any{
			"platform":     platform,
			"account_name": auth.AccountName,
			"login":        auth.Login,
			"scope":        auth.Scope,
		})
	}
	data := map[string]any{
		"mode":           "local",
		"workspace_root": WorkspaceRootFromContext(ctx),
		"authorized":     auths,
	}
	display := "当前未进入远程仓库上下文"
	if ok {
		data["mode"] = "remote"
		data["remote"] = remote
		display = fmt.Sprintf("远程仓库 %s/%s @ %s", remote.Owner, remote.Repo, firstNonEmpty(remote.WorkingBranch, remote.DefaultBranch))
	}
	return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoStatus, Status: "success", Data: data, Display: display}
}

func (m *Manager) remoteRepoCloneOrOpenStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	platform := strings.ToLower(strings.TrimSpace(asString(params["platform"])))
	repoURL := strings.TrimSpace(asString(params["repo_url"]))
	baseBranch := strings.TrimSpace(asString(params["base_branch"]))
	if platform == "" || repoURL == "" {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCloneOrOpen, Status: "error", Error: "platform and repo_url required"}
	}
	ref, auth, err := remoteRepoAccess(ctx, platform, repoURL)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCloneOrOpen, Status: "error", Error: err.Error()}
	}
	targetDir := remoteRepoDir(ref)
	if _, err := os.Stat(filepath.Join(targetDir, ".git")); err != nil {
		if !os.IsNotExist(err) {
			return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCloneOrOpen, Status: "error", Error: err.Error()}
		}
		if _, err := os.Stat(targetDir); err == nil {
			_ = os.RemoveAll(targetDir)
		}
		cloneOut, err := gitops.Clone(ctx, ref.CloneURL, targetDir, baseBranch, providerUsername(platform, auth), auth.AccessToken)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCloneOrOpen, Status: "error", Error: err.Error()}
		}
		if baseBranch == "" {
			baseBranch = cloneOut.DefaultBranch
		}
	}
	ops := gitops.NewOpsWithRoot(targetDir)
	if strings.TrimSpace(baseBranch) != "" {
		if _, err := ops.Checkout(baseBranch, false); err != nil {
			// 分支不存在时保留已克隆分支，不阻塞首次接入
		}
	}
	currentBranch, _ := ops.HeadBranch()
	remoteCtx := RemoteRepoContext{
		Platform:      platform,
		RepoURL:       ref.CloneURL,
		Owner:         ref.Owner,
		Repo:          ref.Repo,
		DefaultBranch: firstNonEmpty(baseBranch, currentBranch),
		WorkingBranch: currentBranch,
		LocalPath:     targetDir,
		AccountLogin:  auth.Login,
		AccountName:   auth.AccountName,
	}
	SetRemoteRepoContext(TraceIDFromContext(ctx), remoteCtx)

	cfg, cfgPath := loadRemoteConfig()
	upsertRemoteRepo(&cfg, config.RemoteRepoEntry{
		Platform:      config.RemotePlatformType(platform),
		RepoURL:       ref.CloneURL,
		Owner:         ref.Owner,
		Repo:          ref.Repo,
		DefaultBranch: remoteCtx.DefaultBranch,
		LocalPath:     targetDir,
		LastBranch:    currentBranch,
		LastUsedUnix:  time.Now().Unix(),
	})
	if err := saveRemoteConfig(cfg, cfgPath); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCloneOrOpen, Status: "error", Error: err.Error()}
	}
	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolRemoteRepoCloneOrOpen,
		Status: "success",
		Data: map[string]any{
			"platform":       platform,
			"repo_url":       ref.CloneURL,
			"owner":          ref.Owner,
			"repo":           ref.Repo,
			"local_path":     filepath.ToSlash(targetDir),
			"default_branch": remoteCtx.DefaultBranch,
			"working_branch": remoteCtx.WorkingBranch,
		},
		Display: fmt.Sprintf("已进入远程仓库 %s/%s，工作目录切换到 %s", ref.Owner, ref.Repo, filepath.ToSlash(targetDir)),
	}
}

func (m *Manager) remoteRepoCheckoutStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	remote, auth, err := currentRemoteAccess(ctx)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCheckout, Status: "error", Error: err.Error()}
	}
	branch := strings.TrimSpace(asString(params["branch"]))
	if branch == "" {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCheckout, Status: "error", Error: "branch required"}
	}
	create, _ := params["create"].(bool)
	ops := gitops.NewOpsWithRoot(remote.LocalPath)
	out, err := ops.Checkout(branch, create)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCheckout, Status: "error", Error: err.Error()}
	}
	remote.WorkingBranch = out
	remote.AccountLogin = firstNonEmpty(remote.AccountLogin, auth.Login)
	remote.AccountName = firstNonEmpty(remote.AccountName, auth.AccountName)
	SetRemoteRepoContext(TraceIDFromContext(ctx), remote)
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolRemoteRepoCheckout,
		Status:  "success",
		Data:    map[string]any{"branch": out, "create": create},
		Display: "已切换远程分支 " + out,
	}
}

func (m *Manager) remoteRepoCommitAndPushStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	remote, auth, err := currentRemoteAccess(ctx)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: err.Error()}
	}
	ops := gitops.NewOpsWithRoot(remote.LocalPath)
	changes, err := ops.Status()
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: err.Error()}
	}
	if len(changes) == 0 {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: "没有可提交的改动"}
	}
	msg := strings.TrimSpace(asString(params["message"]))
	if msg == "" {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: "message required"}
	}
	branch := strings.TrimSpace(asString(params["branch"]))
	if branch == "" {
		branch, _ = ops.HeadBranch()
	}
	if branch == "" {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: "无法确定当前分支"}
	}
	if _, err := ops.Add([]string{"."}); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: err.Error()}
	}
	out, err := ops.Commit(msg, asString(params["author_name"]), asString(params["author_email"]))
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: err.Error()}
	}
	if err := confirmRemoteAction(ctx, "推送远程仓库", fmt.Sprintf("确认将提交 %s 推送到 %s/%s 的分支 %s 吗？", out.Hash, remote.Owner, remote.Repo, branch)); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: err.Error()}
	}
	pushStatus, err := ops.PushBranch("origin", branch, providerUsername(remote.Platform, auth), auth.AccessToken, true)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCommitAndPush, Status: "error", Error: err.Error()}
	}
	remote.WorkingBranch = branch
	SetRemoteRepoContext(TraceIDFromContext(ctx), remote)
	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolRemoteRepoCommitAndPush,
		Status: "success",
		Data: map[string]any{
			"branch":   branch,
			"hash":     out.Hash,
			"files":    out.Files,
			"status":   pushStatus,
			"repo_url": remote.RepoURL,
		},
		Display: fmt.Sprintf("已推送远程分支 %s，提交 %s", branch, out.Hash),
	}
}

func (m *Manager) remoteRepoCreatePRStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	remote, auth, err := currentRemoteAccess(ctx)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreatePR, Status: "error", Error: err.Error()}
	}
	if remote.Platform != string(config.RemotePlatformGitHub) {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreatePR, Status: "error", Error: "当前远程仓库不是 GitHub"}
	}
	title := strings.TrimSpace(asString(params["title"]))
	if title == "" {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreatePR, Status: "error", Error: "title required"}
	}
	base := firstNonEmpty(strings.TrimSpace(asString(params["base"])), remote.DefaultBranch)
	head := firstNonEmpty(strings.TrimSpace(asString(params["head"])), remote.WorkingBranch)
	body := strings.TrimSpace(asString(params["body"]))
	if err := confirmRemoteAction(ctx, "创建 GitHub PR", fmt.Sprintf("确认在 %s/%s 上创建 PR 吗？%s -> %s", remote.Owner, remote.Repo, head, base)); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreatePR, Status: "error", Error: err.Error()}
	}
	provider, _ := remoteProviderFor(remote.Platform)
	pr, err := provider.CreatePullRequest(ctx, auth.AccessToken, remoteRepoRef{
		Platform: remote.Platform,
		Owner:    remote.Owner,
		Repo:     remote.Repo,
	}, title, body, base, head)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreatePR, Status: "error", Error: err.Error()}
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolRemoteRepoCreatePR,
		Status:  "success",
		Data:    map[string]any{"number": pr.Number, "url": pr.URL, "base": base, "head": head},
		Display: fmt.Sprintf("GitHub PR 已创建：%s", pr.URL),
	}
}

func (m *Manager) remoteRepoCreateMRStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	remote, auth, err := currentRemoteAccess(ctx)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreateMR, Status: "error", Error: err.Error()}
	}
	if remote.Platform != string(config.RemotePlatformGitee) {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreateMR, Status: "error", Error: "当前远程仓库不是 Gitee"}
	}
	title := strings.TrimSpace(asString(params["title"]))
	if title == "" {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreateMR, Status: "error", Error: "title required"}
	}
	base := firstNonEmpty(strings.TrimSpace(asString(params["base"])), remote.DefaultBranch)
	head := firstNonEmpty(strings.TrimSpace(asString(params["head"])), remote.WorkingBranch)
	body := strings.TrimSpace(asString(params["body"]))
	if err := confirmRemoteAction(ctx, "创建 Gitee PR/MR", fmt.Sprintf("确认在 %s/%s 上创建 PR/MR 吗？%s -> %s", remote.Owner, remote.Repo, head, base)); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreateMR, Status: "error", Error: err.Error()}
	}
	provider, _ := remoteProviderFor(remote.Platform)
	pr, err := provider.CreatePullRequest(ctx, auth.AccessToken, remoteRepoRef{
		Platform: remote.Platform,
		Owner:    remote.Owner,
		Repo:     remote.Repo,
	}, title, body, base, head)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoCreateMR, Status: "error", Error: err.Error()}
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolRemoteRepoCreateMR,
		Status:  "success",
		Data:    map[string]any{"number": pr.Number, "url": pr.URL, "base": base, "head": head},
		Display: fmt.Sprintf("Gitee PR/MR 已创建：%s", pr.URL),
	}
}

func (m *Manager) remoteRepoDisconnectStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	traceID := TraceIDFromContext(ctx)
	remote, ok := GetRemoteRepoContext(traceID)
	if !ok {
		return ToolResult{Type: "tool_result", Tool: ToolRemoteRepoDisconnect, Status: "success", Data: map[string]any{"cleared": false}, Display: "当前没有远程仓库上下文"}
	}
	cleanup, _ := params["cleanup_local"].(bool)
	ClearRemoteRepoContext(traceID)
	if cleanup && strings.TrimSpace(remote.LocalPath) != "" {
		_ = os.RemoveAll(remote.LocalPath)
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolRemoteRepoDisconnect,
		Status:  "success",
		Data:    map[string]any{"cleared": true, "cleanup_local": cleanup},
		Display: fmt.Sprintf("已断开远程仓库 %s/%s", remote.Owner, remote.Repo),
	}
}

func remoteRepoAccess(ctx context.Context, platform, repoURL string) (remoteRepoRef, config.RemoteAuthToken, error) {
	cfg, _ := loadRemoteConfig()
	ensureRemoteMaps(&cfg)
	auth, ok := cfg.RemoteAuth[platform]
	if !ok || strings.TrimSpace(auth.AccessToken) == "" {
		return remoteRepoRef{}, config.RemoteAuthToken{}, fmt.Errorf("平台 %s 尚未授权，请先调用 remote_repo_connect", platform)
	}
	provider, err := remoteProviderFor(platform)
	if err != nil {
		return remoteRepoRef{}, config.RemoteAuthToken{}, err
	}
	ref, err := provider.NormalizeRepoURL(repoURL)
	if err != nil {
		return remoteRepoRef{}, config.RemoteAuthToken{}, err
	}
	return ref, auth, nil
}

func currentRemoteAccess(ctx context.Context) (RemoteRepoContext, config.RemoteAuthToken, error) {
	traceID := TraceIDFromContext(ctx)
	remote, ok := GetRemoteRepoContext(traceID)
	if !ok {
		return RemoteRepoContext{}, config.RemoteAuthToken{}, fmt.Errorf("当前会话未进入远程仓库上下文")
	}
	cfg, _ := loadRemoteConfig()
	ensureRemoteMaps(&cfg)
	auth, ok := cfg.RemoteAuth[remote.Platform]
	if !ok || strings.TrimSpace(auth.AccessToken) == "" {
		return RemoteRepoContext{}, config.RemoteAuthToken{}, fmt.Errorf("平台 %s 尚未授权，请先调用 remote_repo_connect", remote.Platform)
	}
	return remote, auth, nil
}

func confirmRemoteAction(ctx context.Context, title, question string) error {
	if UserConfirmPrompt == nil {
		return nil
	}
	res, err := UserConfirmPrompt(ctx, UserConfirmRequest{
		Title:     title,
		Question:  question,
		Options:   []string{"确认", "取消"},
		AllowText: false,
	})
	if err != nil {
		return err
	}
	if !res.Confirmed {
		return fmt.Errorf("用户取消操作")
	}
	return nil
}

func providerUsername(platform string, auth config.RemoteAuthToken) string {
	provider, err := remoteProviderFor(platform)
	if err != nil {
		return "oauth2"
	}
	return provider.AuthUsername(auth)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
