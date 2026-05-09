package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RemoteRepoContext 表示当前会话绑定的远程仓库上下文。
type RemoteRepoContext struct {
	Mode          string    `json:"mode"`
	Platform      string    `json:"platform"`
	RepoURL       string    `json:"repo_url"`
	Owner         string    `json:"owner"`
	Repo          string    `json:"repo"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	WorkingBranch string    `json:"working_branch,omitempty"`
	LocalPath     string    `json:"local_path"`
	AccountLogin  string    `json:"account_login,omitempty"`
	AccountName   string    `json:"account_name,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

var OnRemoteRepoContextChanged func(traceID string, ctx RemoteRepoContext)
var OnRemoteRepoContextCleared func(traceID string, ctx RemoteRepoContext)

type remoteContextStore struct {
	mu       sync.RWMutex
	byTrace  map[string]RemoteRepoContext
	byRoot   map[string]RemoteRepoContext
	lastSeen RemoteRepoContext
}

var defaultRemoteContextStore = &remoteContextStore{
	byTrace: make(map[string]RemoteRepoContext),
	byRoot:  make(map[string]RemoteRepoContext),
}

func normalizeRemoteRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	root = filepath.Clean(root)
	return filepath.ToSlash(root)
}

func (s *remoteContextStore) set(traceID string, ctx RemoteRepoContext) {
	traceID = strings.TrimSpace(traceID)
	ctx.LocalPath = normalizeRemoteRoot(ctx.LocalPath)
	ctx.Mode = "remote"
	ctx.UpdatedAt = time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	if traceID != "" {
		s.byTrace[traceID] = ctx
	}
	if ctx.LocalPath != "" {
		s.byRoot[ctx.LocalPath] = ctx
	}
	s.lastSeen = ctx
}

func (s *remoteContextStore) getByTrace(traceID string) (RemoteRepoContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, ok := s.byTrace[strings.TrimSpace(traceID)]
	return ctx, ok
}

func (s *remoteContextStore) getByRoot(root string) (RemoteRepoContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, ok := s.byRoot[normalizeRemoteRoot(root)]
	return ctx, ok
}

func (s *remoteContextStore) clear(traceID string) (RemoteRepoContext, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	traceID = strings.TrimSpace(traceID)
	ctx, ok := s.byTrace[traceID]
	if !ok {
		return RemoteRepoContext{}, false
	}
	delete(s.byTrace, traceID)
	if ctx.LocalPath != "" {
		delete(s.byRoot, ctx.LocalPath)
	}
	return ctx, true
}

func (s *remoteContextStore) current() (RemoteRepoContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strings.TrimSpace(s.lastSeen.LocalPath) == "" {
		return RemoteRepoContext{}, false
	}
	return s.lastSeen, true
}

func SetRemoteRepoContext(traceID string, ctx RemoteRepoContext) {
	defaultRemoteContextStore.set(traceID, ctx)
	if OnRemoteRepoContextChanged != nil {
		OnRemoteRepoContextChanged(strings.TrimSpace(traceID), ctx)
	}
}

func GetRemoteRepoContext(traceID string) (RemoteRepoContext, bool) {
	return defaultRemoteContextStore.getByTrace(traceID)
}

func GetRemoteRepoContextByRoot(root string) (RemoteRepoContext, bool) {
	return defaultRemoteContextStore.getByRoot(root)
}

func CurrentRemoteRepoContext() (RemoteRepoContext, bool) {
	return defaultRemoteContextStore.current()
}

func ClearRemoteRepoContext(traceID string) {
	ctx, ok := defaultRemoteContextStore.clear(traceID)
	if ok && OnRemoteRepoContextCleared != nil {
		OnRemoteRepoContextCleared(strings.TrimSpace(traceID), ctx)
	}
}
