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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/internal/config"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	sharedruntime "github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/session"
	skillspkg "github.com/dreamSailing/eos/internal/skills"
	"github.com/dreamSailing/eos/internal/state"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/internal/tools/bg"
	"github.com/dreamSailing/eos/pkg/protocol"
)

type Event struct {
	Type      string
	RequestID string
	Message   string
	Data      map[string]any
}

type Runtime struct {
	core        *bridge.RuntimeCore
	workspaceMu sync.Mutex
}

type Model struct {
	Name                    string
	APIBase                 string
	APIKey                  string
	Model                   string
	Source                  string
	IsActive                bool
	SupportsReasoningEffort bool
}

type BackgroundTask struct {
	ID        string
	Status    string
	StartedAt time.Time
	Label     string
	CanKill   bool
	Workspace string
}

type Workspace struct {
	Path    string
	Trusted bool
	Active  bool
}

type WorkspaceSnapshot struct {
	Path             string
	Name             string
	Trusted          bool
	Active           bool
	SessionCount     int
	CurrentSessionID string
}

type SessionMeta struct {
	ID      string
	SavedAt time.Time
	Model   string
	Summary string
	Preview string
	Title   string
	Rounds  int
	Tokens  int
}

type SessionSnapshot struct {
	ID             string
	WorkspacePath  string
	Title          string
	Preview        string
	UpdatedAt      time.Time
	Running        bool
	NeedsAttention bool
	MessageCount   int
	PendingPrompts int
	Active         bool
}

type SessionMessage struct {
	Role       string
	Type       string
	Content    string
	Time       time.Time
	ImagePaths []string
}

type RuntimeSnapshot struct {
	ForegroundWorkspace string
	Workspaces          []WorkspaceSnapshot
	Sessions            []SessionSnapshot
	CurrentSession      *SessionSnapshot
	Messages            []SessionMessage
	Tasks               []BackgroundTask
}

type MCPServer struct {
	Name    string
	Type    string
	Target  string
	Enabled bool
}

type Settings struct {
	Language       string
	Theme          string
	MidRiskConfirm bool
}

type VersionItem struct {
	ID        string
	File      string
	CreatedAt time.Time
	Summary   string
}

type LSPServer struct {
	Language string
	Status   string
	Command  string
}

type PermissionSnapshot struct {
	ExecutionMode     string
	AllowAll          bool
	AllowedCategories []string
	HasPendingDiff    bool
	PendingDiffPath   string
}

type PendingReview struct {
	Path    string
	Diff    string
	HasDiff bool
}

type SkillInfo struct {
	Name                   string
	Description            string
	Source                 string
	ArgumentHint           string
	Location               string
	BaseDir                string
	AllowedTools           []string
	Enabled                bool
	Active                 bool
	DisableModelInvocation bool
	UserInvocable          bool
	UserInvocableDefined   bool
}

type PluginInfo struct {
	Name        string
	Description string
	Source      string
	Command     string
	Enabled     bool
}

type CostItem struct {
	Time      time.Time
	Model     string
	Input     int
	Reply     int
	Token     int
	CostCents int
}

func NewRuntime() *Runtime {
	cm := session.NewContextManager()
	tm := tools.NewManager()
	return &Runtime{core: bridge.NewRuntimeCore(cm, tm, nil)}
}

func (r *Runtime) Invoke(ctx context.Context, input string) (<-chan Event, error) {
	return r.InvokeWithImages(ctx, input, nil)
}

func (r *Runtime) InvokeWithImages(ctx context.Context, input string, imagePaths []string) (<-chan Event, error) {
	imagePaths = compactCoreImagePaths(imagePaths)
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		done := make(chan error, 1)
		go func() {
			_, err := r.core.GraphInvokePlanWithImages(ctx, input, r.core.ExecutionMode(), imagePaths)
			done <- err
		}()
		for {
			select {
			case <-ctx.Done():
				out <- Event{Type: "Error", Message: ctx.Err().Error()}
				return
			case ev := <-r.core.Events():
				if mapped, ok := mapBridgeEvent(ev); ok {
					out <- mapped
				}
			case err := <-done:
				if err != nil {
					out <- Event{Type: "Error", Message: err.Error()}
				}
				return
			}
		}
	}()
	return out, nil
}

func (r *Runtime) RunBash(ctx context.Context, input string) (<-chan Event, error) {
	out := make(chan Event, 3)
	go func() {
		defer close(out)
		out <- Event{Type: "TextDelta", Message: "执行命令: " + strings.TrimSpace(input)}
		result, err := r.core.ExecuteBash(ctx, input)
		if err != nil {
			out <- Event{Type: "Error", Message: err.Error()}
			return
		}
		out <- Event{Type: "TextFinal", Message: result}
	}()
	return out, nil
}

func (r *Runtime) InvokeProtocol(ctx context.Context, input string) (<-chan protocol.Envelope, error) {
	return r.InvokeProtocolWithImages(ctx, input, nil)
}

func (r *Runtime) InvokeProtocolWithImages(ctx context.Context, input string, imagePaths []string) (<-chan protocol.Envelope, error) {
	sessionID, _ := r.CurrentSessionID()
	sessionID = strings.TrimSpace(sessionID)
	threadID := sessionID
	requestID := newCoreRequestID("req")
	input = strings.TrimSpace(input)
	imagePaths = compactCoreImagePaths(imagePaths)
	out := make(chan protocol.Envelope, 64)
	go func() {
		defer close(out)
		out <- newCoreRequestEvent(protocol.EventTypeRequestStarted, sessionID, threadID, requestID, map[string]any{
			"input":       input,
			"mode":        r.ExecutionMode(),
			"input_kind":  "prompt",
			"image_count": len(imagePaths),
		})
		done := make(chan error, 1)
		go func() {
			_, err := r.core.GraphInvokePlanWithImages(ctx, input, r.core.ExecutionMode(), imagePaths)
			done <- err
		}()
		for {
			select {
			case <-ctx.Done():
				out <- newCoreRequestEvent(protocol.EventTypeRequestFailed, sessionID, threadID, requestID, map[string]any{
					"error":       ctx.Err().Error(),
					"input":       input,
					"mode":        r.ExecutionMode(),
					"input_kind":  "prompt",
					"image_count": len(imagePaths),
					"summary":     ctx.Err().Error(),
				})
				return
			case ev := <-r.core.Events():
				if mapped, ok := bridgeEventToProtocol(ev, sessionID, threadID, requestID, time.Now()); ok {
					out <- mapped
				}
			case err := <-done:
				if err != nil {
					out <- newCoreRequestEvent(protocol.EventTypeRequestFailed, sessionID, threadID, requestID, map[string]any{
						"error":       err.Error(),
						"input":       input,
						"mode":        r.ExecutionMode(),
						"input_kind":  "prompt",
						"image_count": len(imagePaths),
						"summary":     err.Error(),
					})
				} else {
					out <- newCoreRequestEvent(protocol.EventTypeRequestDone, sessionID, threadID, requestID, map[string]any{
						"input":       input,
						"mode":        r.ExecutionMode(),
						"input_kind":  "prompt",
						"image_count": len(imagePaths),
						"message":     "request completed",
						"status":      "success",
						"summary":     "request completed",
					})
				}
				return
			}
		}
	}()
	return out, nil
}

func compactCoreImagePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(path))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, path)
	}
	return out
}

func (r *Runtime) RunBashProtocol(ctx context.Context, input string) (<-chan protocol.Envelope, error) {
	sessionID, _ := r.CurrentSessionID()
	sessionID = strings.TrimSpace(sessionID)
	threadID := sessionID
	requestID := newCoreRequestID("bash")
	input = strings.TrimSpace(input)

	out := make(chan protocol.Envelope, 8)
	go func() {
		defer close(out)
		out <- newCoreRequestEvent(protocol.EventTypeRequestStarted, sessionID, threadID, requestID, map[string]any{
			"input":      input,
			"mode":       "bash",
			"input_kind": "bash",
		})
		out <- protocol.NewEvent(protocol.EventTypeTextDelta, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: requestID,
			Timestamp:     time.Now(),
			Source:        protocol.SourceCore,
			Payload:       protocol.TextPayloadMap(protocol.TextPayload{Text: "执行命令: " + input}),
		})

		result, err := r.core.ExecuteBash(ctx, input)
		if err != nil {
			out <- newCoreRequestEvent(protocol.EventTypeRequestFailed, sessionID, threadID, requestID, map[string]any{
				"error":      err.Error(),
				"input":      input,
				"mode":       "bash",
				"input_kind": "bash",
				"summary":    err.Error(),
			})
			return
		}

		out <- protocol.NewEvent(protocol.EventTypeTextFinal, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: requestID,
			Timestamp:     time.Now(),
			Source:        protocol.SourceCore,
			Payload:       protocol.TextPayloadMap(protocol.TextPayload{Text: result}),
		})
		out <- newCoreRequestEvent(protocol.EventTypeRequestDone, sessionID, threadID, requestID, map[string]any{
			"input":      input,
			"mode":       "bash",
			"input_kind": "bash",
			"message":    "request completed",
			"status":     "success",
			"summary":    "request completed",
		})
	}()
	return out, nil
}

func (r *Runtime) SetExecutionMode(mode string) {
	r.core.SetExecutionMode(toRuntimeMode(mode))
}

func (r *Runtime) ExecutionMode() string {
	return fromRuntimeMode(r.core.ExecutionMode())
}

func (r *Runtime) ResolveConfirmation(requestID string, approve bool) {
	decision := "deny"
	if approve {
		decision = "allow_once"
	}
	r.core.SubmitPromptResponse(requestID, bridge.PromptResponse{Decision: decision})
}

func (r *Runtime) ResolveInquiry(requestID string, option, text string) {
	r.core.SubmitPromptResponse(requestID, bridge.PromptResponse{
		Decision: "resolve",
		Option:   option,
		Text:     text,
	})
}

func (r *Runtime) Close() {
	r.core.Shutdown()
}

func (r *Runtime) ListModels() []Model {
	cfg, _ := r.core.LoadFullModelConfig()
	out := make([]Model, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		supportsReasoning := m.SupportsReasoningEffort || ai.SupportsReasoningEffort(m.Model)
		out = append(out, Model{
			Name:                    m.Name,
			APIBase:                 m.APIBase,
			APIKey:                  m.APIKey,
			Model:                   m.Model,
			Source:                  m.Source,
			IsActive:                strings.TrimSpace(cfg.Active) == strings.TrimSpace(m.Name),
			SupportsReasoningEffort: supportsReasoning,
		})
	}
	return out
}

func (r *Runtime) UpsertModel(name, base, key, model string) error {
	name = strings.TrimSpace(name)
	base = strings.TrimSpace(base)
	model = strings.TrimSpace(model)
	if name == "" || base == "" || model == "" {
		return errors.New("name, api base, model required")
	}
	entry := config.ModelEntry{
		Name:                    name,
		APIBase:                 base,
		APIKey:                  strings.TrimSpace(key),
		Model:                   model,
		SupportsReasoningEffort: ai.SupportsReasoningEffort(model),
	}
	if !r.core.UpdateModel(entry) && !r.core.AddModel(entry) {
		return errors.New("upsert model failed")
	}
	return nil
}

func (r *Runtime) ActivateModel(name string) error {
	if !r.core.SetActiveModel(strings.TrimSpace(name)) {
		return errors.New("model not found")
	}
	return r.core.Reload()
}

func (r *Runtime) ReasoningLevel() string {
	cfg, _ := config.Load()
	active := strings.TrimSpace(cfg.Active)
	if active == "" {
		if !cfg.Thinking.Enabled || !state.Thinking() {
			return "off"
		}
		return normalizeReasoningLevel(cfg.Thinking.ReasoningEffort)
	}
	for _, model := range cfg.Models {
		if !strings.EqualFold(strings.TrimSpace(model.Name), active) {
			continue
		}
		supportsReasoning := model.SupportsReasoningEffort || ai.SupportsReasoningEffort(model.Model)
		if !supportsReasoning || !model.ThinkingEnabled || !cfg.Thinking.Enabled || !state.Thinking() {
			return "off"
		}
		return normalizeReasoningLevel(cfg.Thinking.ReasoningEffort)
	}
	if !cfg.Thinking.Enabled || !state.Thinking() {
		return "off"
	}
	return normalizeReasoningLevel(cfg.Thinking.ReasoningEffort)
}

func (r *Runtime) SetReasoningLevel(level string) error {
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return errors.New("config path empty")
	}

	level = normalizeReasoningLevel(level)
	active := strings.TrimSpace(cfg.Active)
	foundActive := false

	cfg.Thinking.Enabled = true
	cfg.Thinking.AutoDetect = true

	for i := range cfg.Models {
		cfg.Models[i].SupportsReasoningEffort = cfg.Models[i].SupportsReasoningEffort || ai.SupportsReasoningEffort(cfg.Models[i].Model)
		if !strings.EqualFold(strings.TrimSpace(cfg.Models[i].Name), active) {
			continue
		}
		foundActive = true
		if level == "off" {
			cfg.Models[i].ThinkingEnabled = false
			break
		}
		if !cfg.Models[i].SupportsReasoningEffort {
			return errors.New("当前模型不支持推理强度")
		}
		cfg.Models[i].ThinkingEnabled = true
		cfg.Thinking.ReasoningEffort = level
	}

	if !foundActive {
		if level == "off" {
			cfg.Thinking.Enabled = false
		} else {
			cfg.Thinking.ReasoningEffort = level
		}
	}

	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}

	state.SetThinking(true)
	if !foundActive && level == "off" {
		state.SetThinking(false)
	}

	return r.core.Reload()
}

func (r *Runtime) DeleteModel(name string) error {
	if !r.core.DeleteModel(strings.TrimSpace(name)) {
		return errors.New("model not found")
	}
	return nil
}

func (r *Runtime) ListTasks() []BackgroundTask {
	items := bg.Default().List()
	out := make([]BackgroundTask, 0, len(items))
	for _, t := range items {
		out = append(out, BackgroundTask{
			ID:        t.ID,
			Status:    string(t.Status),
			StartedAt: t.StartedAt,
			Label:     t.Command,
			CanKill:   t.Status == bg.StatusRunning,
			Workspace: normalizeWorkspacePath(t.WorkingDir),
		})
	}
	slices.SortFunc(out, func(a, b BackgroundTask) int {
		if a.StartedAt.After(b.StartedAt) {
			return -1
		}
		if a.StartedAt.Before(b.StartedAt) {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func (r *Runtime) TailTask(taskID string) ([]string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("task id required")
	}
	res, err := bg.Default().Tail(taskID, &bg.TailOptions{FromSeq: 0, Limit: 200})
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(res.Entries))
	for _, e := range res.Entries {
		lines = append(lines, e.Stream+": "+e.Line)
	}
	return lines, nil
}

func (r *Runtime) KillTask(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("task id required")
	}
	_, err := bg.Default().Kill(taskID)
	return err
}

func (r *Runtime) CleanupTasks() int {
	return bg.Default().CleanupFinished()
}

func (r *Runtime) ListWorkspaces() []Workspace {
	active := normalizeWorkspacePath(r.core.GetActiveRoot())
	roots := r.core.GetWorkspaceRoots()
	trusted := trustedWorkspaceSet()
	known := knownWorkspaceSet()
	out := make([]Workspace, 0, len(roots)+len(trusted)+len(known))
	seen := map[string]struct{}{}
	for _, raw := range roots {
		p := normalizeWorkspacePath(raw)
		if p == "" {
			continue
		}
		seen[p] = struct{}{}
		_, ok := trusted[p]
		out = append(out, Workspace{
			Path:    p,
			Trusted: ok,
			Active:  pathsEqual(p, active),
		})
	}
	for p := range trusted {
		if _, ok := seen[p]; ok {
			continue
		}
		out = append(out, Workspace{Path: p, Trusted: true, Active: pathsEqual(p, active)})
		seen[p] = struct{}{}
	}
	for p := range known {
		if _, ok := seen[p]; ok {
			continue
		}
		_, ok := trusted[p]
		out = append(out, Workspace{Path: p, Trusted: ok, Active: pathsEqual(p, active)})
	}
	slices.SortFunc(out, func(a, b Workspace) int { return strings.Compare(a.Path, b.Path) })
	return out
}

func (r *Runtime) AddWorkspace(path string) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
		return err
	}
	if err := rememberWorkspaceConfig(p, false); err != nil {
		return err
	}
	r.core.AddWorkspaceRoot(p)
	return nil
}

func (r *Runtime) RemoveWorkspace(path string) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
		return err
	}
	if pathsEqual(p, config.DefaultWorkspacePath()) {
		return errors.New("default workspace cannot be removed")
	}
	if err := removeTrustedWorkspace(p); err != nil {
		return err
	}
	if err := forgetWorkspaceConfig(p); err != nil {
		return err
	}
	r.core.RemoveWorkspaceRoot(p)
	return nil
}

func (r *Runtime) UseWorkspace(path string) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
		return err
	}
	if err := rememberWorkspaceConfig(p, true); err != nil {
		return err
	}
	r.core.AddWorkspaceRoot(p)
	if r.core.SetActiveWorkspaceRoot(p) == nil {
		return errors.New("workspace not found")
	}
	return r.ReloadSkills()
}

func (r *Runtime) RememberWorkspace(path string, foreground bool) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
		return err
	}
	if err := rememberWorkspaceConfig(p, foreground); err != nil {
		return err
	}
	r.core.AddWorkspaceRoot(p)
	return nil
}

func (r *Runtime) ForgetWorkspace(path string) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
		return err
	}
	if pathsEqual(p, config.DefaultWorkspacePath()) {
		return errors.New("default workspace cannot be removed")
	}
	if err := forgetWorkspaceConfig(p); err != nil {
		return err
	}
	r.core.RemoveWorkspaceRoot(p)
	return removeTrustedWorkspace(p)
}

func (r *Runtime) LastWorkspace() string {
	cfg, _ := config.Load()
	return config.ResolveForegroundWorkspace(cfg, "")
}

func (r *Runtime) DefaultWorkspacePath() string {
	return config.DefaultWorkspacePath()
}

func (r *Runtime) ResolveForegroundWorkspace(preferred string) (string, error) {
	cfg, _ := config.Load()
	target := config.ResolveForegroundWorkspace(cfg, preferred)
	target = normalizeWorkspacePath(target)
	if target == "" {
		return "", errors.New("workspace path required")
	}
	if err := r.RememberWorkspace(target, true); err != nil {
		return "", err
	}
	return target, nil
}

func (r *Runtime) TrustWorkspace(path string) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
		return err
	}
	if err := rememberWorkspaceConfig(p, false); err != nil {
		return err
	}
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return errors.New("config path empty")
	}
	want := normalizeWorkspacePath(p)
	for _, cur := range cfg.TrustedWorkspaces {
		if pathsEqual(normalizeWorkspacePath(cur), want) {
			return nil
		}
	}
	cfg.TrustedWorkspaces = append(cfg.TrustedWorkspaces, want)
	return config.Save(cfg, cfgPath)
}

func (r *Runtime) SetForegroundWorkspace(path string) error {
	return r.UseWorkspace(path)
}

func (r *Runtime) ListWorkspaceSessions(workspacePath string) ([]SessionMeta, error) {
	resolved, err := resolveWorkspacePath(workspacePath)
	if err != nil {
		return nil, err
	}
	items, err := r.core.ListSessionsForWorkspaceRoot(resolved)
	if err != nil {
		return nil, err
	}
	return sessionMetasFromPersisted(items), nil
}

func (r *Runtime) CreateWorkspaceSession(workspacePath, title string, messages []SessionMessage) (SessionMeta, error) {
	return withRuntimeWorkspace(r, workspacePath, func(resolved string) (SessionMeta, error) {
		var zero SessionMeta
		if err := rememberWorkspaceConfig(resolved, false); err != nil {
			return zero, err
		}
		sessionID, err := r.SaveSessionMessages("", messages)
		if err != nil {
			return zero, err
		}
		if strings.TrimSpace(title) != "" {
			if err := r.UpdateSessionTitle(sessionID, title); err != nil {
				return zero, err
			}
		}
		if err := r.SetCurrentSession(sessionID); err != nil {
			return zero, err
		}
		for _, meta := range r.listCurrentWorkspaceSessions() {
			if strings.TrimSpace(meta.ID) == sessionID {
				return meta, nil
			}
		}
		return SessionMeta{
			ID:      sessionID,
			SavedAt: time.Now(),
			Title:   strings.TrimSpace(title),
			Preview: sessionPreview(messages),
			Summary: sessionPreview(messages),
			Rounds:  len(messages),
		}, nil
	})
}

func (r *Runtime) SaveWorkspaceSessionMessages(workspacePath, id string, messages []SessionMessage) (string, error) {
	return withRuntimeWorkspace(r, workspacePath, func(_ string) (string, error) {
		return r.SaveSessionMessages(id, messages)
	})
}

func (r *Runtime) LoadWorkspaceSessionMessages(workspacePath, id string) ([]SessionMessage, error) {
	resolved, err := resolveWorkspacePath(workspacePath)
	if err != nil {
		return nil, err
	}
	items, err := r.core.LoadSessionMessagesForWorkspaceRoot(resolved, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return sessionMessagesFromBridge(items), nil
}

func (r *Runtime) GetWorkspaceCurrentSession(workspacePath string) (string, error) {
	resolved, err := resolveWorkspacePath(workspacePath)
	if err != nil {
		return "", err
	}
	return r.core.CurrentSessionIDForWorkspaceRoot(resolved)
}

func (r *Runtime) SetWorkspaceCurrentSession(workspacePath, id string) error {
	_, err := withRuntimeWorkspace(r, workspacePath, func(_ string) (struct{}, error) {
		return struct{}{}, r.SetCurrentSession(id)
	})
	return err
}

func (r *Runtime) UpdateWorkspaceSessionTitle(workspacePath, id, title string) error {
	_, err := withRuntimeWorkspace(r, workspacePath, func(_ string) (struct{}, error) {
		return struct{}{}, r.UpdateSessionTitle(id, title)
	})
	return err
}

func (r *Runtime) ResumeWorkspaceSession(workspacePath, id string) error {
	_, err := withRuntimeWorkspace(r, workspacePath, func(_ string) (struct{}, error) {
		return struct{}{}, r.ResumeSession(id)
	})
	return err
}

func (r *Runtime) DeleteWorkspaceSession(workspacePath, id string) error {
	_, err := withRuntimeWorkspace(r, workspacePath, func(_ string) (struct{}, error) {
		return struct{}{}, r.DeleteSession(id)
	})
	return err
}

func (r *Runtime) ResolveSessionWorkspace(sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", errors.New("session id required")
	}
	for _, workspace := range r.ListWorkspaces() {
		metas, err := r.ListWorkspaceSessions(workspace.Path)
		if err != nil {
			continue
		}
		for _, meta := range metas {
			if strings.TrimSpace(meta.ID) == sessionID {
				return workspace.Path, nil
			}
		}
	}
	return "", os.ErrNotExist
}

func (r *Runtime) RuntimeSnapshot() RuntimeSnapshot {
	foreground := normalizeWorkspacePath(r.core.GetActiveRoot())
	workspaces := r.ListWorkspaces()
	if foreground == "" {
		foreground = normalizeWorkspacePath(r.LastWorkspace())
	}
	if foreground == "" && len(workspaces) > 0 {
		foreground = workspaces[0].Path
	}

	out := RuntimeSnapshot{
		ForegroundWorkspace: foreground,
		Workspaces:          make([]WorkspaceSnapshot, 0, len(workspaces)),
		Sessions:            make([]SessionSnapshot, 0),
		Tasks:               r.ListTasks(),
	}

	for _, workspace := range workspaces {
		currentID, _ := r.GetWorkspaceCurrentSession(workspace.Path)
		metas, err := r.ListWorkspaceSessions(workspace.Path)
		if err != nil {
			metas = nil
		}
		out.Workspaces = append(out.Workspaces, WorkspaceSnapshot{
			Path:             workspace.Path,
			Name:             workspaceDisplayName(workspace.Path),
			Trusted:          workspace.Trusted,
			Active:           pathsEqual(workspace.Path, foreground) || workspace.Active,
			SessionCount:     len(metas),
			CurrentSessionID: strings.TrimSpace(currentID),
		})

		for _, meta := range metas {
			messageCount := meta.Rounds
			var messages []SessionMessage
			isActiveSession := pathsEqual(workspace.Path, foreground) && strings.TrimSpace(currentID) == strings.TrimSpace(meta.ID)
			if isActiveSession {
				if loaded, err := r.LoadWorkspaceSessionMessages(workspace.Path, meta.ID); err == nil {
					messages = loaded
					if len(loaded) > 0 {
						messageCount = len(loaded)
					}
				}
			}
			snapshot := SessionSnapshot{
				ID:             meta.ID,
				WorkspacePath:  workspace.Path,
				Title:          strings.TrimSpace(meta.Title),
				Preview:        fallbackText(strings.TrimSpace(meta.Preview), strings.TrimSpace(meta.Summary)),
				UpdatedAt:      meta.SavedAt,
				MessageCount:   messageCount,
				PendingPrompts: 0,
				Active:         isActiveSession,
			}
			out.Sessions = append(out.Sessions, snapshot)
			if snapshot.Active {
				current := snapshot
				out.CurrentSession = &current
				out.Messages = append([]SessionMessage(nil), messages...)
			}
		}
	}

	slices.SortFunc(out.Sessions, func(a, b SessionSnapshot) int {
		if a.UpdatedAt.After(b.UpdatedAt) {
			return -1
		}
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func (r *Runtime) ListSessions() []SessionMeta {
	return r.listCurrentWorkspaceSessions()
}

func (r *Runtime) SaveSession(id string) (string, error) {
	return r.core.SaveSession(context.Background(), strings.TrimSpace(id))
}

func (r *Runtime) SaveSessionMessages(id string, messages []SessionMessage) (string, error) {
	items := make([]bridge.SessionTranscriptMessage, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		ts := int64(0)
		if !msg.Time.IsZero() {
			ts = msg.Time.Unix()
		}
		items = append(items, bridge.SessionTranscriptMessage{
			Role:       strings.TrimSpace(msg.Role),
			Type:       strings.TrimSpace(msg.Type),
			Content:    content,
			Timestamp:  ts,
			ImagePaths: append([]string{}, msg.ImagePaths...),
		})
	}
	return r.core.SaveSessionMessages(context.Background(), strings.TrimSpace(id), items)
}

func (r *Runtime) LoadSessionMessages(id string) ([]SessionMessage, error) {
	items, err := r.core.LoadSessionMessages(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	out := make([]SessionMessage, 0, len(items))
	for _, item := range items {
		var ts time.Time
		if item.Timestamp > 0 {
			ts = time.Unix(item.Timestamp, 0)
		}
		out = append(out, SessionMessage{
			Role:       item.Role,
			Type:       item.Type,
			Content:    item.Content,
			Time:       ts,
			ImagePaths: append([]string{}, item.ImagePaths...),
		})
	}
	return out, nil
}

func (r *Runtime) CurrentSessionID() (string, error) {
	return r.core.CurrentSessionID()
}

func (r *Runtime) SetCurrentSession(id string) error {
	return r.core.SetCurrentSession(strings.TrimSpace(id))
}

func (r *Runtime) UpdateSessionTitle(id, title string) error {
	return r.core.UpdateSessionTitle(strings.TrimSpace(id), strings.TrimSpace(title))
}

func (r *Runtime) ResumeSession(id string) error {
	return r.core.ResumeSession(context.Background(), strings.TrimSpace(id))
}

func (r *Runtime) DeleteSession(id string) error {
	return r.core.DeleteSession(strings.TrimSpace(id))
}

func (r *Runtime) ListMCP() []MCPServer {
	items := r.core.ListMCPServers()
	out := make([]MCPServer, 0, len(items))
	for _, it := range items {
		target := strings.TrimSpace(it.Command)
		if strings.TrimSpace(it.BaseURL) != "" {
			target = strings.TrimSpace(it.BaseURL)
		}
		out = append(out, MCPServer{
			Name:    it.Name,
			Type:    string(it.Type),
			Target:  target,
			Enabled: it.Enabled,
		})
	}
	slices.SortFunc(out, func(a, b MCPServer) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (r *Runtime) UpsertMCP(name, kind, target string, enabled bool) error {
	name = strings.TrimSpace(name)
	kind = strings.ToLower(strings.TrimSpace(kind))
	target = strings.TrimSpace(target)
	if name == "" || target == "" {
		return errors.New("name and target required")
	}
	entry := config.MCPEntry{
		Name:    name,
		Type:    config.MCPClientType(kind),
		Enabled: enabled,
	}
	if kind == "sse" {
		entry.BaseURL = target
	} else {
		entry.Type = config.MCPClientType("stdio")
		entry.Command = target
	}
	if !r.core.UpdateMCPServer(entry) {
		return r.core.AddMCPServers([]config.MCPEntry{entry})
	}
	return nil
}

func (r *Runtime) DeleteMCP(name string) error {
	if !r.core.DeleteMCPServer(strings.TrimSpace(name)) {
		return errors.New("mcp server not found")
	}
	return nil
}

func (r *Runtime) SetMCPEnabled(name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name required")
	}
	items := r.core.ListMCPServers()
	for _, it := range items {
		if strings.TrimSpace(it.Name) != name {
			continue
		}
		if it.Enabled == enabled {
			return nil
		}
		if r.core.ToggleMCPServer(name) {
			return nil
		}
		return errors.New("toggle mcp server failed")
	}
	return errors.New("mcp server not found")
}

func (r *Runtime) GetRules() string {
	path := r.projectRulesPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return sharedruntime.RulesMdTemplate()
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return sharedruntime.RulesMdTemplate()
	}
	return text
}

func (r *Runtime) SaveRules(v string) error {
	path := r.projectRulesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := strings.TrimSpace(v)
	if content == "" {
		content = sharedruntime.RulesMdTemplate()
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (r *Runtime) ResetRules() error {
	return r.SaveRules(sharedruntime.RulesMdTemplate())
}

func (r *Runtime) GetSettings() Settings {
	path := r.settingsPath()
	cur := r.core.GetSettings()
	s, err := r.core.LoadSettings(path)
	if err == nil && s != nil {
		cur = *s
	}
	return Settings{
		Language:       cur.Language,
		Theme:          cur.Theme,
		MidRiskConfirm: false,
	}
}

func (r *Runtime) SaveSettings(v Settings) error {
	path := r.settingsPath()
	cur := r.core.GetSettings()
	if strings.TrimSpace(v.Language) != "" {
		cur.Language = strings.TrimSpace(v.Language)
	}
	if strings.TrimSpace(v.Theme) != "" {
		cur.Theme = strings.TrimSpace(v.Theme)
	}
	return r.core.SaveSettings(path, &cur)
}

func (r *Runtime) ListVersions() []VersionItem {
	files, err := r.core.ListVersionFiles()
	if err != nil || len(files) == 0 {
		return nil
	}
	root := r.workingRoot()
	out := make([]VersionItem, 0)
	for _, f := range files {
		abs := filepath.Join(root, filepath.FromSlash(f.PathRel))
		versions, verErr := r.core.ListVersionsForPath(abs)
		if verErr != nil {
			continue
		}
		for _, v := range versions {
			out = append(out, VersionItem{
				ID:        v.ID,
				File:      filepath.ToSlash(v.PathRel),
				CreatedAt: v.Timestamp,
				Summary:   fmt.Sprintf("size=%d", v.Size),
			})
		}
	}
	slices.SortFunc(out, func(a, b VersionItem) int {
		if a.CreatedAt.After(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.Before(b.CreatedAt) {
			return 1
		}
		if d := strings.Compare(a.File, b.File); d != 0 {
			return d
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func (r *Runtime) RollbackVersion(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("version id required")
	}
	it, err := r.findVersionByID(id)
	if err != nil {
		return err
	}
	res := r.core.RollbackFile(it.File, it.ID)
	if e := resultToError(res); e != nil {
		return e
	}
	return nil
}

func (r *Runtime) DeleteVersion(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("version id required")
	}
	it, err := r.findVersionByID(id)
	if err != nil {
		return err
	}
	res := r.core.DeleteVersion(it.File, it.ID)
	if e := resultToError(res); e != nil {
		return e
	}
	return nil
}

func (r *Runtime) DeleteFileVersions(file string) int {
	target := filepath.ToSlash(strings.TrimSpace(file))
	if target == "" {
		return 0
	}
	count := 0
	for _, v := range r.ListVersions() {
		if filepath.ToSlash(v.File) == target {
			count++
		}
	}
	if count == 0 {
		return 0
	}
	res := r.core.DeleteAllVersions(target)
	if resultToError(res) != nil {
		return 0
	}
	return count
}

func (r *Runtime) ClearVersions() int {
	items := r.ListVersions()
	if len(items) == 0 {
		return 0
	}
	res := r.core.DeleteAllFileVersions()
	if resultToError(res) != nil {
		return 0
	}
	return len(items)
}

func (r *Runtime) ListLSP() []LSPServer {
	st := r.core.LSPStatus()
	out := make([]LSPServer, 0, len(st.Servers))
	for _, it := range st.Servers {
		status := "stopped"
		cmd := strings.TrimSpace(it.Command)
		if !it.Found {
			status = "not_found"
		}
		if strings.EqualFold(strings.TrimSpace(st.ActiveLanguage), strings.TrimSpace(it.Language)) {
			status = "running"
			if strings.TrimSpace(st.ActiveServer) != "" {
				cmd = strings.TrimSpace(st.ActiveServer)
			}
		}
		out = append(out, LSPServer{
			Language: it.Language,
			Status:   status,
			Command:  cmd,
		})
	}
	return out
}

func (r *Runtime) DetectLSP(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	if lang == "" {
		lang = strings.ToLower(strings.TrimSpace(r.core.LSPStatus().DetectedLanguage))
	}
	st := r.core.LSPStatus()
	for _, it := range st.Servers {
		if strings.ToLower(strings.TrimSpace(it.Language)) != lang {
			continue
		}
		if !it.Found {
			return it.Language + " server not found"
		}
		return it.Language + ": " + strings.TrimSpace(it.Command)
	}
	return "language not supported: " + language
}

func (r *Runtime) StartLSP(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	if lang == "" {
		lang = strings.ToLower(strings.TrimSpace(r.core.LSPStatus().DetectedLanguage))
	}
	if lang == "" {
		return "cannot detect language"
	}
	st := r.core.LSPStatus()
	if strings.EqualFold(strings.TrimSpace(st.ActiveLanguage), lang) && strings.TrimSpace(st.ActiveServer) != "" {
		return st.ActiveLanguage + " already running"
	}
	for _, it := range st.Servers {
		if strings.ToLower(strings.TrimSpace(it.Language)) != lang {
			continue
		}
		if !it.Found {
			return it.Language + " server not found"
		}
		return "LSP auto-starts when needed: " + it.Language
	}
	return "language not supported: " + language
}

func (r *Runtime) LSPDiagnostics() []string {
	raw := strings.TrimSpace(r.core.ProblemsAndDiagnosticsMarkdown())
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "##") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func (r *Runtime) PermissionSnapshot() PermissionSnapshot {
	snap := r.core.PermissionSnapshot()
	return PermissionSnapshot{
		ExecutionMode:     strings.TrimSpace(snap.ExecutionMode),
		AllowAll:          snap.AllowAll,
		AllowedCategories: append([]string(nil), snap.AllowedCategories...),
		HasPendingDiff:    snap.HasPendingDiff,
		PendingDiffPath:   strings.TrimSpace(snap.PendingDiffPath),
	}
}

func (r *Runtime) PendingReview() PendingReview {
	diff := strings.TrimSpace(r.core.GetPendingDiff())
	path := strings.TrimSpace(r.core.GetPendingDiffPath())
	return PendingReview{
		Path:    path,
		Diff:    diff,
		HasDiff: diff != "",
	}
}

func (r *Runtime) ClearPendingReview() {
	r.core.ClearPendingDiff()
}

func (r *Runtime) ListSkills() []SkillInfo {
	var raw []*skillspkg.Skill
	if loader := r.core.GetSkillsLoader(); loader != nil {
		raw = loader.List()
	}
	sm := r.core.GetSkillManager()
	if len(raw) == 0 && sm != nil {
		raw = sm.List()
	}
	active := map[string]bool{}
	if sm != nil {
		for name := range sm.GetActive() {
			active[strings.TrimSpace(name)] = true
		}
	}
	cfg, _ := config.Load()
	out := make([]SkillInfo, 0, len(raw))
	for _, skill := range raw {
		if skill == nil {
			continue
		}
		name := strings.TrimSpace(skill.Name)
		enabled := sm == nil || !sm.IsDisabled(name)
		if sm == nil {
			enabled = !config.IsSkillDisabled(&cfg, name)
		}
		item := SkillInfo{
			Name:                   name,
			Description:            strings.TrimSpace(skill.Description),
			Source:                 skillInfoSource(skill),
			ArgumentHint:           strings.TrimSpace(skill.ArgumentHint),
			Location:               strings.TrimSpace(skill.Location),
			BaseDir:                strings.TrimSpace(skill.BaseDir),
			AllowedTools:           skill.AllowedTools.Values(),
			Enabled:                enabled,
			Active:                 enabled && active[name],
			DisableModelInvocation: skill.DisableModelInvocation,
		}
		if skill.UserInvocable != nil {
			item.UserInvocableDefined = true
			item.UserInvocable = *skill.UserInvocable
		}
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b SkillInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}

func (r *Runtime) ReloadSkills() error {
	cfg, _ := config.Load()
	skillDirs := skillspkg.ResolveScanDirs(runtimePluginWorkspaceRoot(r), &cfg)
	if sm := r.core.GetSkillManager(); sm != nil {
		if loader := sm.GetLoader(); loader != nil {
			loader.SetSkillsDirs(skillDirs)
		}
		sm.SetDisabledSkills(cfg.DisabledSkills)
		return sm.ReloadPreserveActive()
	}
	if loader := r.core.GetSkillsLoader(); loader != nil {
		loader.SetSkillsDirs(skillDirs)
		return loader.Reload()
	}
	return nil
}

func (r *Runtime) SetSkillEnabled(name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name required")
	}
	found := false
	currentEnabled := true
	for _, item := range r.ListSkills() {
		if !strings.EqualFold(strings.TrimSpace(item.Name), name) {
			continue
		}
		found = true
		name = item.Name
		currentEnabled = item.Enabled
		break
	}
	if !found {
		return errors.New("skill not found")
	}
	if currentEnabled == enabled {
		if sm := r.core.GetSkillManager(); sm != nil {
			sm.SetDisabled(name, !enabled)
		}
		return nil
	}
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return errors.New("config path empty")
	}
	config.SetSkillDisabled(&cfg, name, !enabled)
	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}
	if sm := r.core.GetSkillManager(); sm != nil {
		sm.SetDisabled(name, !enabled)
	}
	return nil
}

func (r *Runtime) ListPlugins() []PluginInfo {
	items := pluginpkg.DefaultRegistry().List()
	out := make([]PluginInfo, 0, len(items))
	cfg, _ := config.Load()
	seen := map[string]struct{}{}
	for _, item := range items {
		if item == nil {
			continue
		}
		name := strings.TrimSpace(item.Name())
		if name == "" {
			continue
		}
		seen[strings.ToLower(name)] = struct{}{}
		enabled := pluginpkg.DefaultRegistry().IsEnabled(name)
		if cfgEnabled, ok := config.PluginEnabled(&cfg, name); ok {
			enabled = cfgEnabled
		}
		meta := pluginpkg.MetadataOf(item)
		out = append(out, PluginInfo{
			Name:        name,
			Description: strings.TrimSpace(item.Description()),
			Source:      strings.TrimSpace(meta.Source),
			Command:     strings.TrimSpace(meta.Command),
			Enabled:     enabled,
		})
	}
	workspaceRoot := runtimePluginWorkspaceRoot(r)
	discovered, _ := pluginpkg.Discover(workspaceRoot)
	for _, item := range discovered {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(name)]; ok {
			continue
		}
		enabled := true
		if cfgEnabled, ok := config.PluginEnabled(&cfg, name); ok {
			enabled = cfgEnabled
		}
		out = append(out, PluginInfo{
			Name:        name,
			Description: strings.TrimSpace(item.Description),
			Source:      "directory:" + strings.TrimSpace(item.Location),
			Enabled:     enabled,
		})
	}
	slices.SortFunc(out, func(a, b PluginInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}

func (r *Runtime) SetPluginEnabled(name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name required")
	}
	found := false
	currentEnabled := true
	for _, item := range r.ListPlugins() {
		if !strings.EqualFold(strings.TrimSpace(item.Name), name) {
			continue
		}
		found = true
		name = item.Name
		currentEnabled = item.Enabled
		break
	}
	if !found {
		return errors.New("plugin not found")
	}
	if currentEnabled == enabled {
		pluginpkg.DefaultRegistry().SetEnabled(name, enabled)
		return nil
	}
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return errors.New("config path empty")
	}
	config.SetPluginEnabled(&cfg, name, enabled)
	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}
	pluginpkg.DefaultRegistry().SetEnabled(name, enabled)
	return r.ReloadSkills()
}

func (r *Runtime) ContextPreview() []string {
	msgs := r.core.BuildPreview()
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		out = append(out, strings.TrimSpace(m.Role)+": "+content)
	}
	return out
}

func (r *Runtime) ContextStats() (int, int) {
	cm := r.core.GetContext()
	if cm == nil {
		return 0, 0
	}
	_, tokens, _ := cm.GetConversationUsage()
	return len(r.core.BuildPreview()), tokens
}

func (r *Runtime) CompactContext() string {
	beforeCount, beforeTokens := r.ContextStats()
	r.core.CompactContext()
	afterCount, afterTokens := r.ContextStats()
	return fmt.Sprintf("context compacted: messages %d→%d, tokens %d→%d", beforeCount, afterCount, beforeTokens, afterTokens)
}

func (r *Runtime) ClearContext() {
	r.core.ClearContext()
}

func (r *Runtime) ExportContext(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("path required")
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(r.workingRoot(), path)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	content := strings.Join(r.ContextPreview(), "\n")
	return os.WriteFile(abs, []byte(content), 0o644)
}

func (r *Runtime) CostSummary() string {
	s := r.core.GetTokenStats()
	return fmt.Sprintf("rounds=%d input=%d reply=%d total=%d", s.Rounds, s.Input, s.Reply, s.Total)
}

func (r *Runtime) CostItems() []CostItem {
	raw := r.core.GetTokenHistory()
	items := make([]CostItem, 0, len(raw))
	for _, it := range raw {
		items = append(items, CostItem{
			Time:      it.Timestamp,
			Model:     it.Model,
			Input:     it.Input,
			Reply:     it.Reply,
			Token:     it.Total,
			CostCents: 0,
		})
	}
	slices.SortFunc(items, func(a, b CostItem) int {
		if a.Time.After(b.Time) {
			return -1
		}
		if a.Time.Before(b.Time) {
			return 1
		}
		return strings.Compare(a.Model, b.Model)
	})
	return items
}

func (r *Runtime) projectRulesPath() string {
	root := r.workingRoot()
	if strings.TrimSpace(root) == "" {
		return filepath.Join(".eos", "Rules.md")
	}
	return filepath.Join(root, ".eos", "Rules.md")
}

func (r *Runtime) workingRoot() string {
	root := strings.TrimSpace(r.core.GetActiveRoot())
	if root != "" {
		return root
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

func (r *Runtime) settingsPath() string {
	root := strings.TrimSpace(r.core.GetActiveRoot())
	if root != "" {
		return filepath.Join(root, ".eos", "settings.json")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".eos", "settings.json")
	}
	return filepath.Join(".eos", "settings.json")
}

func (r *Runtime) findVersionByID(id string) (VersionItem, error) {
	for _, it := range r.ListVersions() {
		if strings.TrimSpace(it.ID) == id {
			return it, nil
		}
	}
	return VersionItem{}, errors.New("version not found")
}

func resultToError(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(s), "error:") {
		return errors.New(strings.TrimSpace(s[6:]))
	}
	return nil
}

func resolveWorkspacePath(raw string) (string, error) {
	return config.ResolveWorkspacePath(raw)
}

func normalizeWorkspacePath(p string) string {
	return config.NormalizeWorkspacePath(p)
}

func pathsEqual(a, b string) bool {
	return config.PathsEqual(a, b)
}

func skillInfoSource(skill *skillspkg.Skill) string {
	if skill == nil {
		return ""
	}
	if strings.TrimSpace(skill.PluginName) != "" {
		return "plugin:" + strings.TrimSpace(skill.PluginName)
	}
	source := strings.TrimSpace(skill.Location)
	if source != "" {
		return source
	}
	if strings.TrimSpace(skill.BaseDir) != "" {
		return "scanner"
	}
	return ""
}

func runtimePluginWorkspaceRoot(r *Runtime) string {
	if r == nil || r.core == nil {
		if wd, err := os.Getwd(); err == nil {
			return normalizeWorkspacePath(wd)
		}
		return ""
	}
	if root := normalizeWorkspacePath(r.core.GetActiveRoot()); root != "" {
		return root
	}
	for _, root := range r.core.GetWorkspaceRoots() {
		if normalized := normalizeWorkspacePath(root); normalized != "" {
			return normalized
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return normalizeWorkspacePath(wd)
	}
	return ""
}

func trustedWorkspaceSet() map[string]struct{} {
	cfg, _ := config.Load()
	out := map[string]struct{}{}
	for _, p := range cfg.TrustedWorkspaces {
		np := normalizeWorkspacePath(p)
		if np == "" {
			continue
		}
		out[np] = struct{}{}
	}
	return out
}

func knownWorkspaceSet() map[string]struct{} {
	cfg, _ := config.Load()
	out := map[string]struct{}{}
	for _, path := range config.NormalizeKnownWorkspaces(cfg.KnownWorkspaces) {
		normalized := normalizeWorkspacePath(path)
		if normalized == "" {
			continue
		}
		out[normalized] = struct{}{}
	}
	return out
}

func (r *Runtime) listCurrentWorkspaceSessions() []SessionMeta {
	items, err := r.core.ListSessions()
	if err != nil {
		return nil
	}
	return sessionMetasFromPersisted(items)
}

func sessionMetasFromPersisted(items []bridge.PersistedSessionMeta) []SessionMeta {
	out := make([]SessionMeta, 0, len(items))
	for _, it := range items {
		out = append(out, SessionMeta{
			ID:      it.ID,
			SavedAt: time.Unix(it.SavedAt, 0),
			Model:   it.Model,
			Summary: it.Summary,
			Preview: it.Preview,
			Title:   it.Title,
			Rounds:  it.Rounds,
			Tokens:  it.Tokens,
		})
	}
	return out
}

func sessionMessagesFromBridge(items []bridge.SessionTranscriptMessage) []SessionMessage {
	out := make([]SessionMessage, 0, len(items))
	for _, item := range items {
		var ts time.Time
		if item.Timestamp > 0 {
			ts = time.Unix(item.Timestamp, 0)
		}
		out = append(out, SessionMessage{
			Role:       item.Role,
			Type:       item.Type,
			Content:    item.Content,
			Time:       ts,
			ImagePaths: append([]string{}, item.ImagePaths...),
		})
	}
	return out
}

func withRuntimeWorkspace[T any](r *Runtime, workspacePath string, fn func(string) (T, error)) (T, error) {
	var zero T
	resolved, err := resolveWorkspacePath(workspacePath)
	if err != nil {
		return zero, err
	}

	r.workspaceMu.Lock()
	defer r.workspaceMu.Unlock()

	previous := normalizeWorkspacePath(r.core.GetActiveRoot())
	if !pathsEqual(previous, resolved) {
		r.core.AddWorkspaceRoot(resolved)
		if r.core.SetActiveWorkspaceRoot(resolved) == nil {
			return zero, errors.New("workspace not found")
		}
	}

	defer func() {
		if previous == "" || pathsEqual(previous, resolved) {
			return
		}
		_ = r.core.SetActiveWorkspaceRoot(previous)
	}()

	return fn(resolved)
}

func sessionPreview(messages []SessionMessage) string {
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		content = strings.ReplaceAll(content, "\r", " ")
		content = strings.ReplaceAll(content, "\n", " ")
		if len(content) > 96 {
			content = content[:96]
		}
		return content
	}
	return ""
}

func workspaceDisplayName(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	if base == "." || base == string(filepath.Separator) || strings.TrimSpace(base) == "" {
		return strings.TrimSpace(path)
	}
	return base
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func normalizeReasoningLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "low":
		return "low"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "off"
	}
}

func removeTrustedWorkspace(path string) error {
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return errors.New("config path empty")
	}
	filtered, changed := filterTrustedWorkspaces(cfg.TrustedWorkspaces, path)
	if !changed {
		return nil
	}
	cfg.TrustedWorkspaces = filtered
	return config.Save(cfg, cfgPath)
}

func rememberWorkspaceConfig(path string, foreground bool) error {
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return errors.New("config path empty")
	}
	if !config.RememberWorkspace(&cfg, path, foreground) {
		return nil
	}
	return config.Save(cfg, cfgPath)
}

func forgetWorkspaceConfig(path string) error {
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return errors.New("config path empty")
	}
	if !config.ForgetWorkspace(&cfg, path) {
		return nil
	}
	return config.Save(cfg, cfgPath)
}

func filterTrustedWorkspaces(trusted []string, target string) ([]string, bool) {
	want := normalizeWorkspacePath(target)
	if want == "" {
		return append([]string(nil), trusted...), false
	}
	filtered := make([]string, 0, len(trusted))
	changed := false
	for _, cur := range trusted {
		if pathsEqual(normalizeWorkspacePath(cur), want) {
			changed = true
			continue
		}
		filtered = append(filtered, cur)
	}
	return filtered, changed
}

func newCoreRequestID(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "req"
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func newCoreRequestEvent(eventType protocol.EventType, sessionID, threadID, requestID string, payload map[string]any) protocol.Envelope {
	return protocol.NewEvent(eventType, protocol.EventOptions{
		SessionID:     strings.TrimSpace(sessionID),
		ThreadID:      strings.TrimSpace(threadID),
		RequestID:     strings.TrimSpace(requestID),
		CorrelationID: strings.TrimSpace(requestID),
		Timestamp:     time.Now(),
		Source:        protocol.SourceCore,
		Payload:       payload,
	})
}

func bridgeEventToProtocol(ev bridge.Event, sessionID, threadID, fallbackRequestID string, ts time.Time) (protocol.Envelope, bool) {
	eventType := strings.TrimSpace(ev.Type)
	requestID := strings.TrimSpace(ev.RID)
	if requestID == "" {
		requestID = strings.TrimSpace(fallbackRequestID)
	}
	payload := protocol.ClonePayload(ev.Data)
	if payload == nil {
		payload = map[string]any{}
	}

	switch eventType {
	case "meta", "delta", string(protocol.EventTypeTextDelta):
		if _, ok := payload["text"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["text"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeTextDelta, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: requestID,
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case "final", string(protocol.EventTypeTextFinal):
		if _, ok := payload["text"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["text"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeTextFinal, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: requestID,
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case "reasoning", "phase.note", string(protocol.EventTypeTextReasoning):
		if _, ok := payload["text"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["text"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeTextReasoning, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: requestID,
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case "tool_call", string(protocol.EventTypeToolCall):
		if _, ok := payload["tool_name"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["tool_name"] = ev.Content
		}
		if _, ok := payload["id"]; !ok && requestID != "" {
			payload["id"] = requestID
		}
		return protocol.NewEvent(protocol.EventTypeToolCall, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: firstCoreString(payload, "related_call_id", "id", "tool_call_id"),
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case "tool_result", string(protocol.EventTypeToolResult):
		if _, ok := payload["display"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["display"] = ev.Content
		}
		if _, ok := payload["id"]; !ok && requestID != "" {
			payload["id"] = requestID
		}
		return protocol.NewEvent(protocol.EventTypeToolResult, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: firstCoreString(payload, "related_call_id", "id", "tool_call_id"),
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case "prompt.request", string(protocol.EventTypeApprovalReq), string(protocol.EventTypeInquiryReq):
		kind := strings.ToLower(strings.TrimSpace(firstCoreString(payload, "kind")))
		if eventType == string(protocol.EventTypeInquiryReq) || kind == "inquiry" {
			if _, ok := payload["inquiry_id"]; !ok && requestID != "" {
				payload["inquiry_id"] = requestID
			}
			if _, ok := payload["question"]; !ok && strings.TrimSpace(ev.Content) != "" {
				payload["question"] = ev.Content
			}
			return protocol.NewEvent(protocol.EventTypeInquiryReq, protocol.EventOptions{
				SessionID:     sessionID,
				ThreadID:      threadID,
				RequestID:     requestID,
				CorrelationID: firstCoreString(payload, "related_call_id"),
				Timestamp:     ts,
				Source:        protocol.SourceCore,
				Payload:       payload,
			}), true
		}
		if _, ok := payload["approval_id"]; !ok && requestID != "" {
			payload["approval_id"] = requestID
		}
		if _, ok := payload["message"]; !ok {
			payload["message"] = firstCoreString(payload, "summary", "question")
			if strings.TrimSpace(firstCoreString(payload, "message")) == "" && strings.TrimSpace(ev.Content) != "" {
				payload["message"] = ev.Content
			}
		}
		return protocol.NewEvent(protocol.EventTypeApprovalReq, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: firstCoreString(payload, "related_call_id"),
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case string(protocol.EventTypeAgentStarted):
		if _, ok := payload["agent_name"]; !ok && requestID != "" {
			payload["agent_name"] = requestID
		}
		if _, ok := payload["task"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["task"] = ev.Content
		}
		if _, ok := payload["message"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["message"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeAgentStarted, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: firstCoreString(payload, "agent_id", "agent_name"),
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case "agent.task", string(protocol.EventTypeAgentProgress):
		if _, ok := payload["agent_name"]; !ok && requestID != "" {
			payload["agent_name"] = requestID
		}
		if _, ok := payload["task"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["task"] = ev.Content
		}
		if _, ok := payload["message"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["message"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeAgentProgress, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: firstCoreString(payload, "agent_id", "agent_name"),
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case "agent.final", string(protocol.EventTypeAgentDone):
		if _, ok := payload["agent_name"]; !ok && requestID != "" {
			payload["agent_name"] = requestID
		}
		if _, ok := payload["text"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["text"] = ev.Content
		}
		if _, ok := payload["message"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["message"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeAgentDone, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: firstCoreString(payload, "agent_id", "agent_name"),
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case string(protocol.EventTypeAgentFailed):
		if _, ok := payload["agent_name"]; !ok && requestID != "" {
			payload["agent_name"] = requestID
		}
		if _, ok := payload["error"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["error"] = ev.Content
		}
		if _, ok := payload["message"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["message"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeAgentFailed, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: firstCoreString(payload, "agent_id", "agent_name"),
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case string(protocol.EventTypeAgentCancelled):
		if _, ok := payload["agent_name"]; !ok && requestID != "" {
			payload["agent_name"] = requestID
		}
		if _, ok := payload["reason"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["reason"] = ev.Content
		}
		if _, ok := payload["message"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["message"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeAgentCancelled, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: firstCoreString(payload, "agent_id", "agent_name"),
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	case "error", string(protocol.EventTypeRequestFailed), string(protocol.EventTypeTaskFailed):
		if _, ok := payload["error"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["error"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeRequestFailed, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: requestID,
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	default:
		if strings.TrimSpace(ev.Content) == "" && len(payload) == 0 {
			return protocol.Envelope{}, false
		}
		if _, ok := payload["text"]; !ok && strings.TrimSpace(ev.Content) != "" {
			payload["text"] = ev.Content
		}
		return protocol.NewEvent(protocol.EventTypeTextDelta, protocol.EventOptions{
			SessionID:     sessionID,
			ThreadID:      threadID,
			RequestID:     requestID,
			CorrelationID: requestID,
			Timestamp:     ts,
			Source:        protocol.SourceCore,
			Payload:       payload,
		}), true
	}
}

func legacyEventToProtocol(ev Event, sessionID, threadID string, ts time.Time) protocol.Envelope {
	requestID := strings.TrimSpace(ev.RequestID)
	correlationID := requestID
	payload := protocol.ClonePayload(ev.Data)
	if payload == nil {
		payload = map[string]any{}
	}

	eventType := protocol.EventTypeTextDelta
	switch strings.TrimSpace(ev.Type) {
	case "TextDelta":
		eventType = protocol.EventTypeTextDelta
		payload["text"] = ev.Message
	case "TextFinal":
		eventType = protocol.EventTypeTextFinal
		payload["text"] = ev.Message
	case "ToolStep":
		eventType = protocol.EventTypeToolStep
		payload["message"] = ev.Message
	case "ConfirmRequired":
		eventType = protocol.EventTypeApprovalReq
		payload["approval_id"] = requestID
		if _, ok := payload["message"]; !ok {
			payload["message"] = ev.Message
		}
	case "Inquiry":
		eventType = protocol.EventTypeInquiryReq
		payload["inquiry_id"] = requestID
		if _, ok := payload["question"]; !ok {
			payload["question"] = ev.Message
		}
	case "Error":
		eventType = protocol.EventTypeRequestFailed
		payload["error"] = ev.Message
	default:
		if strings.TrimSpace(ev.Message) != "" {
			payload["text"] = ev.Message
		}
	}

	return protocol.NewEvent(eventType, protocol.EventOptions{
		SessionID:     sessionID,
		ThreadID:      threadID,
		RequestID:     requestID,
		CorrelationID: correlationID,
		Timestamp:     ts,
		Source:        protocol.SourceCore,
		Payload:       payload,
	})
}

func mapBridgeEvent(ev bridge.Event) (Event, bool) {
	switch ev.Type {
	case "prompt.request", string(protocol.EventTypeApprovalReq), string(protocol.EventTypeInquiryReq):
		msg := strings.TrimSpace(ev.Content)
		if v, ok := ev.Data["question"].(string); ok && strings.TrimSpace(v) != "" {
			msg = strings.TrimSpace(v)
		}
		if ev.Type == string(protocol.EventTypeInquiryReq) {
			if v, ok := ev.Data["question"].(string); ok && strings.TrimSpace(v) != "" {
				msg = strings.TrimSpace(v)
			}
			return Event{Type: "Inquiry", RequestID: ev.RID, Message: msg, Data: ev.Data}, true
		}
		if kind, ok := ev.Data["kind"].(string); ok && kind == "inquiry" {
			return Event{Type: "Inquiry", RequestID: ev.RID, Message: msg, Data: ev.Data}, true
		}
		if v, ok := ev.Data["message"].(string); ok && strings.TrimSpace(v) != "" {
			msg = strings.TrimSpace(v)
		}
		return Event{Type: "ConfirmRequired", RequestID: ev.RID, Message: msg, Data: ev.Data}, true
	case "final", string(protocol.EventTypeTextFinal):
		if v, ok := ev.Data["text"].(string); ok && strings.TrimSpace(v) != "" {
			return Event{Type: "TextFinal", RequestID: ev.RID, Message: strings.TrimSpace(v), Data: ev.Data}, true
		}
		return Event{Type: "TextFinal", RequestID: ev.RID, Message: ev.Content}, true
	case "error", string(protocol.EventTypeRequestFailed), string(protocol.EventTypeAgentFailed), string(protocol.EventTypeAgentCancelled), string(protocol.EventTypeTaskFailed):
		if v, ok := ev.Data["error"].(string); ok && strings.TrimSpace(v) != "" {
			return Event{Type: "Error", RequestID: ev.RID, Message: strings.TrimSpace(v), Data: ev.Data}, true
		}
		return Event{Type: "Error", RequestID: ev.RID, Message: ev.Content}, true
	case "meta", "delta", string(protocol.EventTypeTextDelta):
		if v, ok := ev.Data["text"].(string); ok && strings.TrimSpace(v) != "" {
			return Event{Type: "TextDelta", RequestID: ev.RID, Message: strings.TrimSpace(v), Data: ev.Data}, true
		}
		return Event{Type: "TextDelta", RequestID: ev.RID, Message: ev.Content}, true
	case "tool_call", "tool_result", "reasoning", "phase.note",
		string(protocol.EventTypeToolCall), string(protocol.EventTypeToolResult), string(protocol.EventTypeTextReasoning),
		string(protocol.EventTypeAgentStarted), string(protocol.EventTypeAgentProgress), string(protocol.EventTypeAgentDone),
		string(protocol.EventTypeTaskStarted), string(protocol.EventTypeTaskUpdated), string(protocol.EventTypeTaskDone):
		msg := strings.TrimSpace(ev.Content)
		if msg == "" {
			msg = firstCoreString(ev.Data, "message", "display", "tool_name", "task", "text", "label")
		}
		if msg == "" {
			msg = firstCoreString(ev.Data, "tool_name")
		}
		return Event{Type: "ToolStep", RequestID: ev.RID, Message: msg, Data: ev.Data}, true
	case "agent.task":
		return Event{Type: "ToolStep", RequestID: ev.RID, Message: strings.TrimSpace(ev.Content), Data: ev.Data}, true
	case "agent.final":
		return Event{Type: "ToolStep", RequestID: ev.RID, Message: strings.TrimSpace(ev.Content), Data: ev.Data}, true
	case string(protocol.EventTypeRequestDone):
		return Event{Type: "TextFinal", RequestID: ev.RID, Message: firstCoreString(ev.Data, "text", "message"), Data: ev.Data}, true
	case string(protocol.EventTypeRequestStarted):
		return Event{Type: "ToolStep", RequestID: ev.RID, Message: firstCoreString(ev.Data, "message", "text"), Data: ev.Data}, true
	case string(protocol.EventTypeSessionUpdated):
		return Event{Type: "ToolStep", RequestID: ev.RID, Message: firstCoreString(ev.Data, "message", "title", "preview"), Data: ev.Data}, true
	case string(protocol.EventTypeApprovalDone), string(protocol.EventTypeInquiryDone):
		return Event{Type: "ToolStep", RequestID: ev.RID, Message: firstCoreString(ev.Data, "message", "decision", "option", "text"), Data: ev.Data}, true
	case string(protocol.EventTypeToolStep):
		return Event{Type: "ToolStep", RequestID: ev.RID, Message: firstCoreString(ev.Data, "message", "text", "display", "tool_name"), Data: ev.Data}, true
	default:
		if strings.TrimSpace(ev.Content) == "" {
			return Event{}, false
		}
		return Event{Type: "TextDelta", RequestID: ev.RID, Message: ev.Content}, true
	}
}

func firstCoreString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		if text, ok := raw.(string); ok {
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func toRuntimeMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "计划优先", "先出计划", "plan":
		return "plan"
	default:
		return toolapi.NormalizeExecutionMode(mode)
	}
}

func fromRuntimeMode(mode string) string {
	switch toolapi.NormalizeExecutionMode(mode) {
	case "plan":
		return "plan"
	case "auto":
		return "auto"
	default:
		return "auto"
	}
}
