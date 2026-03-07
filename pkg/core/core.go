package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"
	"github.com/dreamSailing/vb-coding/internal/tools/bg"
)

type Event struct {
	Type      string
	RequestID string
	Message   string
}

type Runtime struct {
	core *bridge.RuntimeCore
}

type Model struct {
	Name     string
	APIBase  string
	APIKey   string
	Model    string
	Source   string
	IsActive bool
}

type BackgroundTask struct {
	ID        string
	Status    string
	StartedAt time.Time
	Label     string
	CanKill   bool
}

type Workspace struct {
	Path    string
	Trusted bool
	Active  bool
}

func NewRuntime() *Runtime {
	cm := session.NewContextManager()
	tm := tools.NewManager()
	return &Runtime{core: bridge.NewRuntimeCore(cm, tm, nil)}
}

func (r *Runtime) Invoke(ctx context.Context, input string) (<-chan Event, error) {
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		done := make(chan error, 1)
		go func() {
			_, err := r.core.GraphInvokePlanWithImages(ctx, input, r.core.ExecutionMode(), nil)
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

func (r *Runtime) Close() {
	r.core.Shutdown()
}

func (r *Runtime) ListModels() []Model {
	cfg, _ := r.core.LoadFullModelConfig()
	out := make([]Model, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		out = append(out, Model{
			Name:     m.Name,
			APIBase:  m.APIBase,
			APIKey:   m.APIKey,
			Model:    m.Model,
			Source:   m.Source,
			IsActive: strings.TrimSpace(cfg.Active) == strings.TrimSpace(m.Name),
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
		Name:    name,
		APIBase: base,
		APIKey:  strings.TrimSpace(key),
		Model:   model,
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
	return nil
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
	out := make([]Workspace, 0, len(roots)+len(trusted))
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
	}
	slices.SortFunc(out, func(a, b Workspace) int { return strings.Compare(a.Path, b.Path) })
	return out
}

func (r *Runtime) AddWorkspace(path string) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
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
	r.core.RemoveWorkspaceRoot(p)
	return nil
}

func (r *Runtime) UseWorkspace(path string) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
		return err
	}
	if r.core.SetActiveWorkspaceRoot(p) == nil {
		return errors.New("workspace not found")
	}
	return nil
}

func (r *Runtime) TrustWorkspace(path string) error {
	p, err := resolveWorkspacePath(path)
	if err != nil {
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

func resolveWorkspacePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("workspace path required")
	}
	if raw == "~" || strings.HasPrefix(raw, "~"+string(os.PathSeparator)) || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return "", errors.New("failed to resolve home path")
		}
		rest := strings.TrimPrefix(raw, "~")
		rest = strings.TrimPrefix(rest, "/")
		rest = strings.TrimPrefix(rest, "\\")
		raw = filepath.Join(home, rest)
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	return normalizeWorkspacePath(abs), nil
}

func normalizeWorkspacePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.Clean(filepath.FromSlash(p))
	if vol := filepath.VolumeName(p); vol != "" {
		rest := strings.TrimPrefix(p, vol)
		p = strings.ToUpper(vol) + rest
	}
	return p
}

func pathsEqual(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(filepath.VolumeName(a), filepath.VolumeName(b)) {
		return strings.EqualFold(a, b)
	}
	return a == b
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

func mapBridgeEvent(ev bridge.Event) (Event, bool) {
	switch ev.Type {
	case "prompt.request":
		msg := strings.TrimSpace(ev.Content)
		if v, ok := ev.Data["question"].(string); ok && strings.TrimSpace(v) != "" {
			msg = strings.TrimSpace(v)
		}
		return Event{Type: "ConfirmRequired", RequestID: ev.RID, Message: msg}, true
	case "final":
		return Event{Type: "TextFinal", RequestID: ev.RID, Message: ev.Content}, true
	case "error":
		return Event{Type: "Error", RequestID: ev.RID, Message: ev.Content}, true
	case "delta":
		return Event{Type: "TextDelta", RequestID: ev.RID, Message: ev.Content}, true
	case "tool_call", "tool_result", "reasoning":
		return Event{Type: "ToolStep", RequestID: ev.RID, Message: ev.Content}, true
	default:
		if strings.TrimSpace(ev.Content) == "" {
			return Event{}, false
		}
		return Event{Type: "TextDelta", RequestID: ev.RID, Message: ev.Content}, true
	}
}

func toRuntimeMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "计划优先", "plan":
		return "plan"
	case "自动无人值守", "auto":
		return "auto"
	default:
		return "manual"
	}
}

func fromRuntimeMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "plan":
		return "计划优先"
	case "auto":
		return "自动无人值守"
	default:
		return "手动确认"
	}
}
