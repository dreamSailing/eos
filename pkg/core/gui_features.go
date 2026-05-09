package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/memory"
	"github.com/dreamSailing/eos/pkg/protocol"
)

type Attachment struct {
	Name string
	Path string
	MIME string
	Kind string
}

type RemoteWorkspace struct {
	ID            string
	Kind          string
	Platform      string
	RepoURL       string
	Owner         string
	Repo          string
	DefaultBranch string
	Branch        string
	Account       string
	LocalPath     string
	Active        bool
	Exists        bool
	LastUsedAt    time.Time
}

type RemoteRepoState struct {
	Mode          string
	Platform      string
	RepoURL       string
	Owner         string
	Repo          string
	DefaultBranch string
	WorkingBranch string
	LocalPath     string
	AccountLogin  string
	AccountName   string
	UpdatedAt     time.Time
}

type PlanSnapshot struct {
	HasPlan          bool
	Content          string
	WorkspaceCurrent string
	UserLatest       string
	UserSnapshot     string
	UpdatedAt        time.Time
}

type MemoryDocument struct {
	Scope     string
	Path      string
	Exists    bool
	Summary   string
	UpdatedAt time.Time
}

type MemorySnapshot struct {
	Documents []MemoryDocument
}

func (r *Runtime) InvokeProtocolWithAttachments(ctx context.Context, input string, attachments []Attachment) (<-chan protocol.Envelope, error) {
	effectiveInput, imagePaths := inputWithAttachments(input, attachments)
	return r.InvokeProtocolWithImages(ctx, effectiveInput, imagePaths)
}

func (r *Runtime) InvokeWithAttachments(ctx context.Context, input string, attachments []Attachment) (<-chan Event, error) {
	effectiveInput, imagePaths := inputWithAttachments(input, attachments)
	return r.InvokeWithImages(ctx, effectiveInput, imagePaths)
}

func inputWithAttachments(input string, attachments []Attachment) (string, []string) {
	input = strings.TrimSpace(input)
	if len(attachments) == 0 {
		return input, nil
	}
	imagePaths := make([]string, 0, len(attachments))
	notes := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		path := strings.TrimSpace(attachment.Path)
		if path == "" {
			continue
		}
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			name = filepath.Base(path)
		}
		mime := strings.TrimSpace(attachment.MIME)
		kind := strings.ToLower(strings.TrimSpace(attachment.Kind))
		if kind == "" {
			kind = attachmentKind(name, mime)
		}
		if kind == "image" {
			imagePaths = append(imagePaths, path)
		}
		label := kind
		if mime != "" {
			label = fmt.Sprintf("%s, %s", kind, mime)
		}
		notes = append(notes, fmt.Sprintf("- %s: %s (%s)", name, path, label))
	}
	if len(notes) == 0 {
		return input, compactCoreImagePaths(imagePaths)
	}
	if input == "" {
		input = "请读取并处理附件。"
	}
	input += "\n\n附件路径（需要时请使用文件/文档工具读取）：\n" + strings.Join(notes, "\n")
	return input, compactCoreImagePaths(imagePaths)
}

func attachmentKind(name, mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	name = strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.Contains(mime, "pdf") || strings.HasSuffix(name, ".pdf"):
		return "pdf"
	case strings.Contains(mime, "word") || strings.HasSuffix(name, ".docx") || strings.HasSuffix(name, ".doc"):
		return "document"
	case strings.Contains(mime, "spreadsheet") || strings.HasSuffix(name, ".xlsx") || strings.HasSuffix(name, ".xls") || strings.HasSuffix(name, ".csv"):
		return "spreadsheet"
	default:
		return "file"
	}
}

func (r *Runtime) PredictNextUserMessage(ctx context.Context, draft string) (string, error) {
	if r == nil || r.core == nil {
		return "", nil
	}
	return r.core.PredictNextUserMessage(ctx, draft)
}

func (r *Runtime) PlanSnapshot() PlanSnapshot {
	if r == nil || r.core == nil {
		return PlanSnapshot{}
	}
	snap := r.core.PlanSnapshot()
	return PlanSnapshot{
		HasPlan:          snap.HasPlan,
		Content:          snap.Content,
		WorkspaceCurrent: snap.WorkspaceCurrent,
		UserLatest:       snap.UserLatest,
		UserSnapshot:     snap.UserSnapshot,
		UpdatedAt:        snap.UpdatedAt,
	}
}

func (r *Runtime) MemorySnapshot() MemorySnapshot {
	if r == nil {
		return MemorySnapshot{}
	}
	root := r.workingRoot()
	docs := []MemoryDocument{
		memoryDoc("global", memory.GlobalMemoryPath()),
		memoryDoc("project", memory.ProjectMemoryPath(root)),
		memoryDoc("project-index", memory.ProjectMemoryIndexPath(root)),
	}
	return MemorySnapshot{Documents: docs}
}

func memoryDoc(scope, path string) MemoryDocument {
	doc := MemoryDocument{Scope: scope, Path: path}
	if strings.TrimSpace(path) == "" {
		return doc
	}
	info, err := os.Stat(path)
	if err != nil {
		return doc
	}
	doc.Exists = true
	doc.UpdatedAt = info.ModTime()
	if b, err := os.ReadFile(path); err == nil {
		doc.Summary = summarizeMemoryText(string(b))
	}
	return doc
}

func summarizeMemoryText(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= 3 {
			break
		}
	}
	summary := strings.Join(out, " ")
	if len(summary) > 220 {
		return strings.TrimSpace(summary[:220]) + "..."
	}
	return summary
}

func (r *Runtime) ListRemoteWorkspaces() []RemoteWorkspace {
	cfg, _ := config.Load()
	active := normalizeWorkspacePath(r.core.GetActiveRoot())
	out := make([]RemoteWorkspace, 0, len(cfg.RemoteRepos))
	for _, repo := range cfg.RemoteRepos {
		item := remoteWorkspaceFromConfig(repo, cfg.RemoteAuth, active)
		if item.ID == "" {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastUsedAt.After(out[j].LastUsedAt)
	})
	return out
}

func remoteWorkspaceFromConfig(repo config.RemoteRepoEntry, auth map[string]config.RemoteAuthToken, active string) RemoteWorkspace {
	localPath := normalizeWorkspacePath(repo.LocalPath)
	repoURL := strings.TrimSpace(repo.RepoURL)
	if repoURL == "" && localPath == "" {
		return RemoteWorkspace{}
	}
	account := ""
	if token, ok := auth[string(repo.Platform)]; ok {
		account = strings.TrimSpace(token.Login)
		if account == "" {
			account = strings.TrimSpace(token.AccountName)
		}
	}
	branch := strings.TrimSpace(repo.LastBranch)
	if branch == "" {
		branch = strings.TrimSpace(repo.DefaultBranch)
	}
	item := RemoteWorkspace{
		ID:            remoteWorkspaceID(repo.Platform, repoURL, localPath),
		Kind:          "remote",
		Platform:      string(repo.Platform),
		RepoURL:       repoURL,
		Owner:         strings.TrimSpace(repo.Owner),
		Repo:          strings.TrimSpace(repo.Repo),
		DefaultBranch: strings.TrimSpace(repo.DefaultBranch),
		Branch:        branch,
		Account:       account,
		LocalPath:     localPath,
		Active:        localPath != "" && pathsEqual(active, localPath),
		Exists:        gitDirExists(localPath),
	}
	if repo.LastUsedUnix > 0 {
		item.LastUsedAt = time.Unix(repo.LastUsedUnix, 0)
	}
	return item
}

func remoteWorkspaceID(platform config.RemotePlatformType, repoURL string, localPath string) string {
	key := strings.TrimSpace(repoURL)
	if key == "" {
		key = strings.TrimSpace(localPath)
	}
	if key == "" {
		return ""
	}
	return string(platform) + ":" + strings.ToLower(key)
}

func gitDirExists(root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil && info != nil
}

func (r *Runtime) CurrentRemoteRepo() (RemoteRepoState, bool) {
	if r == nil || r.core == nil {
		return RemoteRepoState{}, false
	}
	ctx, ok := r.core.CurrentRemoteRepo()
	if !ok {
		return RemoteRepoState{}, false
	}
	return RemoteRepoState{
		Mode:          ctx.Mode,
		Platform:      ctx.Platform,
		RepoURL:       ctx.RepoURL,
		Owner:         ctx.Owner,
		Repo:          ctx.Repo,
		DefaultBranch: ctx.DefaultBranch,
		WorkingBranch: ctx.WorkingBranch,
		LocalPath:     ctx.LocalPath,
		AccountLogin:  ctx.AccountLogin,
		AccountName:   ctx.AccountName,
		UpdatedAt:     ctx.UpdatedAt,
	}, true
}

func (r *Runtime) OpenRemoteWorkspace(idOrPath string) (RemoteWorkspace, error) {
	target := strings.TrimSpace(idOrPath)
	if target == "" {
		return RemoteWorkspace{}, errors.New("remote workspace is required")
	}
	cfg, _ := config.Load()
	active := normalizeWorkspacePath(r.core.GetActiveRoot())
	for _, repo := range cfg.RemoteRepos {
		item := remoteWorkspaceFromConfig(repo, cfg.RemoteAuth, active)
		if remoteWorkspaceMatches(item, target) {
			if !item.Exists {
				return RemoteWorkspace{}, fmt.Errorf("remote checkout does not exist: %s", item.LocalPath)
			}
			if err := r.UseWorkspace(item.LocalPath); err != nil {
				return RemoteWorkspace{}, err
			}
			item.Active = true
			r.notifyStateChanged(StateTopicWorkspace, "workspace.remote.open")
			return item, nil
		}
	}
	return RemoteWorkspace{}, fmt.Errorf("remote workspace not found: %s", target)
}

func remoteWorkspaceMatches(item RemoteWorkspace, target string) bool {
	targetKey := strings.ToLower(strings.TrimSpace(target))
	return targetKey == strings.ToLower(item.ID) ||
		targetKey == strings.ToLower(item.RepoURL) ||
		pathsEqual(target, item.LocalPath)
}

func (r *Runtime) ForgetRemoteWorkspace(idOrPath string) error {
	return r.updateRemoteRepos(idOrPath, false)
}

func (r *Runtime) ClearRemoteWorkspaceCache(idOrPath string) error {
	return r.updateRemoteRepos(idOrPath, true)
}

func (r *Runtime) updateRemoteRepos(idOrPath string, deleteCache bool) error {
	target := strings.TrimSpace(idOrPath)
	if target == "" {
		return errors.New("remote workspace is required")
	}
	cfg, cfgPath := config.Load()
	active := normalizeWorkspacePath(r.core.GetActiveRoot())
	next := make([]config.RemoteRepoEntry, 0, len(cfg.RemoteRepos))
	found := false
	var localPath string
	for _, repo := range cfg.RemoteRepos {
		item := remoteWorkspaceFromConfig(repo, cfg.RemoteAuth, active)
		if remoteWorkspaceMatches(item, target) {
			found = true
			localPath = item.LocalPath
			continue
		}
		next = append(next, repo)
	}
	if !found {
		return fmt.Errorf("remote workspace not found: %s", target)
	}
	cfg.RemoteRepos = next
	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}
	if deleteCache && strings.TrimSpace(localPath) != "" {
		if err := os.RemoveAll(localPath); err != nil {
			return err
		}
	}
	r.notifyStateChanged(StateTopicWorkspace, "workspace.remote.forget")
	return nil
}
