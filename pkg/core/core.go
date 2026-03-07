package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/internal/config"
	sharedruntime "github.com/dreamSailing/vb-coding/internal/runtime"
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
		MidRiskConfirm: true,
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
	wd, _ := os.Getwd()
	out := make([]VersionItem, 0)
	for _, f := range files {
		abs := filepath.Join(wd, filepath.FromSlash(f.PathRel))
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
		wd, _ := os.Getwd()
		abs = filepath.Join(wd, path)
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
	root := strings.TrimSpace(r.core.GetActiveRoot())
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	if strings.TrimSpace(root) == "" {
		return filepath.Join(".vb", "Rules.md")
	}
	return filepath.Join(root, ".vb", "Rules.md")
}

func (r *Runtime) settingsPath() string {
	root := strings.TrimSpace(r.core.GetActiveRoot())
	if root != "" {
		return filepath.Join(root, ".vb", "settings.json")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".vb", "settings.json")
	}
	return filepath.Join(".vb", "settings.json")
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
