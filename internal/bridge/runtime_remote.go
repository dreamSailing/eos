//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/internal/tools"
)

func (rc *RuntimeCore) onRemoteRepoContextChanged(traceID string, remote tools.RemoteRepoContext) {
	root := strings.TrimSpace(remote.LocalPath)
	if root == "" {
		return
	}
	if rc.GetActiveRoot() != root {
		rc.AddWorkspaceRoot(root)
		rc.SetActiveWorkspaceRoot(root)
	}
	rc.eventsCh <- Event{
		Type:    "workspace.remote.changed",
		Content: "remote workspace active",
		Data: map[string]any{
			"trace_id":       strings.TrimSpace(traceID),
			"platform":       remote.Platform,
			"repo_url":       remote.RepoURL,
			"owner":          remote.Owner,
			"repo":           remote.Repo,
			"default_branch": remote.DefaultBranch,
			"working_branch": remote.WorkingBranch,
			"local_path":     filepath.ToSlash(root),
			"account_login":  remote.AccountLogin,
		},
	}
}

func (rc *RuntimeCore) onRemoteRepoContextCleared(traceID string, remote tools.RemoteRepoContext) {
	rc.eventsCh <- Event{
		Type:    "workspace.remote.cleared",
		Content: "remote workspace cleared",
		Data: map[string]any{
			"trace_id":   strings.TrimSpace(traceID),
			"platform":   remote.Platform,
			"repo_url":   remote.RepoURL,
			"local_path": filepath.ToSlash(strings.TrimSpace(remote.LocalPath)),
		},
	}
}

func (rc *RuntimeCore) CurrentRemoteRepo() (tools.RemoteRepoContext, bool) {
	return tools.CurrentRemoteRepoContext()
}
