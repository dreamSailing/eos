package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"io"
	ai "github.com/dreamSailing/vb-coding/internal/ai"
	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/hooks"
	pluginpkg "github.com/dreamSailing/vb-coding/internal/pkg/plugins"
	"github.com/dreamSailing/vb-coding/internal/tools"
	"github.com/dreamSailing/vb-coding/internal/tools/shell"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

type HookManager struct {
	mu         sync.RWMutex
	base       hooks.Config
	loaded     bool
	tm         *tools.Manager
	mdl        AIModel
	agentEval  func(context.Context, string) (string, error)
	hookModels map[string]AIModel
	agentEvals map[string]func(context.Context, string) (string, error)
	onceSeen   map[string]bool
	onMeta     func(string)
}

func NewHookManager(tm *tools.Manager) *HookManager {
	return &HookManager{
		tm:         tm,
		hookModels: map[string]AIModel{},
		agentEvals: map[string]func(context.Context, string) (string, error){},
		onceSeen:   map[string]bool{},
	}
}

func (hm *HookManager) SetModel(m AIModel) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.mdl = m
}

func (hm *HookManager) SetAgentEvaluator(fn func(context.Context, string) (string, error)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.agentEval = fn
}

func (hm *HookManager) SetOnMeta(fn func(string)) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.onMeta = fn
}

func (hm *HookManager) LoadFromDefaultLocations(ctx context.Context) error {
	cfg, err := loadHookConfig(tools.WorkspaceRootFromContext(ctx))
	if err != nil {
		return err
	}
	hm.mu.Lock()
	hm.base = cfg
	hm.loaded = true
	hm.mu.Unlock()
	return nil
}

func (hm *HookManager) PreToolUse(ctx context.Context, toolName string, toolInput map[string]any) (hooks.Decision, error) {
	return hm.run(ctx, "PreToolUse", toolName, toolInput, nil)
}

func (hm *HookManager) PostToolUse(ctx context.Context, toolName string, toolInput map[string]any, toolResult map[string]any) (hooks.Decision, error) {
	return hm.run(ctx, "PostToolUse", toolName, toolInput, toolResult)
}

func (hm *HookManager) PostToolUseFailure(ctx context.Context, toolName string, toolInput map[string]any, toolResult map[string]any) (hooks.Decision, error) {
	return hm.run(ctx, "PostToolUseFailure", toolName, toolInput, toolResult)
}

func (hm *HookManager) PermissionRequest(ctx context.Context, toolName string, toolInput map[string]any, permissionSuggestions []map[string]any) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "PermissionRequest", toolName, toolInput, nil, map[string]any{
		"permission_suggestions": permissionSuggestions,
	})
}

func (hm *HookManager) SessionStart(ctx context.Context, source string, model string, agentType string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "SessionStart", source, nil, nil, map[string]any{
		"source":     strings.TrimSpace(source),
		"model":      strings.TrimSpace(model),
		"agent_type": strings.TrimSpace(agentType),
	})
}

func (hm *HookManager) SessionEnd(ctx context.Context, reason string, durationMs int64) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "SessionEnd", strings.TrimSpace(reason), nil, nil, map[string]any{
		"reason":       strings.TrimSpace(reason),
		"duration_ms":  durationMs,
	})
}

func (hm *HookManager) StopFailure(ctx context.Context, reason string, lastAssistantMessage string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "StopFailure", strings.TrimSpace(reason), nil, nil, map[string]any{
		"reason":                strings.TrimSpace(reason),
		"last_assistant_message": strings.TrimSpace(lastAssistantMessage),
	})
}

func (hm *HookManager) SubagentStart(ctx context.Context, agentType string, task string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "SubagentStart", strings.TrimSpace(agentType), nil, nil, map[string]any{
		"agent_type": strings.TrimSpace(agentType),
		"task":       strings.TrimSpace(task),
	})
}

func (hm *HookManager) TaskCreated(ctx context.Context, taskID string, taskType string, description string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "TaskCreated", "", nil, nil, map[string]any{
		"task_id":      strings.TrimSpace(taskID),
		"task_type":    strings.TrimSpace(taskType),
		"description":  strings.TrimSpace(description),
	})
}

func (hm *HookManager) Elicitation(ctx context.Context, elicitationType string, prompt string, options []string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "Elicitation", strings.TrimSpace(elicitationType), nil, nil, map[string]any{
		"elicitation_type": strings.TrimSpace(elicitationType),
		"prompt":           strings.TrimSpace(prompt),
		"options":          options,
	})
}

func (hm *HookManager) ElicitationResult(ctx context.Context, elicitationType string, selectedOption string, responseText string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "ElicitationResult", strings.TrimSpace(elicitationType), nil, nil, map[string]any{
		"elicitation_type":  strings.TrimSpace(elicitationType),
		"selected_option":   strings.TrimSpace(selectedOption),
		"response_text":     strings.TrimSpace(responseText),
	})
}

func (hm *HookManager) InstructionsLoaded(ctx context.Context, source string, instructionCount int) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "InstructionsLoaded", strings.TrimSpace(source), nil, nil, map[string]any{
		"source":           strings.TrimSpace(source),
		"instruction_count": instructionCount,
	})
}

func (hm *HookManager) CwdChanged(ctx context.Context, oldCwd string, newCwd string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "CwdChanged", "", nil, nil, map[string]any{
		"old_cwd": strings.TrimSpace(oldCwd),
		"new_cwd": strings.TrimSpace(newCwd),
	})
}

func (hm *HookManager) FileChanged(ctx context.Context, filePath string, changeType string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "FileChanged", strings.TrimSpace(filePath), nil, nil, map[string]any{
		"file_path":   strings.TrimSpace(filePath),
		"change_type": strings.TrimSpace(changeType),
	})
}

func (hm *HookManager) Notification(ctx context.Context, notificationType string, message string, title string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "Notification", strings.TrimSpace(notificationType), nil, nil, map[string]any{
		"notification_type": strings.TrimSpace(notificationType),
		"message":           strings.TrimSpace(message),
		"title":             strings.TrimSpace(title),
	})
}

func (hm *HookManager) PreCompact(ctx context.Context, trigger string, customInstructions string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "PreCompact", strings.TrimSpace(trigger), nil, nil, map[string]any{
		"trigger":             strings.TrimSpace(trigger),
		"custom_instructions": strings.TrimSpace(customInstructions),
	})
}

func (hm *HookManager) PostCompact(ctx context.Context, trigger string, originalTokens, savedTokens int) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "PostCompact", strings.TrimSpace(trigger), nil, nil, map[string]any{
		"trigger":         strings.TrimSpace(trigger),
		"original_tokens": originalTokens,
		"saved_tokens":    savedTokens,
	})
}

func (hm *HookManager) WorktreeCreate(ctx context.Context, path string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "WorktreeCreate", "", nil, nil, map[string]any{
		"path": strings.TrimSpace(path),
	})
}

func (hm *HookManager) WorktreeRemove(ctx context.Context, path string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "WorktreeRemove", "", nil, nil, map[string]any{
		"path": strings.TrimSpace(path),
	})
}

func (hm *HookManager) TaskCompleted(ctx context.Context, task string, success bool, errorMsg string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "TaskCompleted", "", nil, nil, map[string]any{
		"task":    strings.TrimSpace(task),
		"success": success,
		"error":   strings.TrimSpace(errorMsg),
	})
}

func (hm *HookManager) TaskCompletedDetailed(ctx context.Context, task string, success bool, errorMsg string, meta map[string]any) (hooks.Decision, error) {
	extra := map[string]any{
		"task":    strings.TrimSpace(task),
		"success": success,
		"error":   strings.TrimSpace(errorMsg),
	}
	for k, v := range meta {
		extra[k] = v
	}
	return hm.runWithExtra(ctx, "TaskCompleted", "", nil, nil, extra)
}

func (hm *HookManager) TeammateIdle(ctx context.Context, agentType string, success bool, errorMsg string, durationMs int64) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "TeammateIdle", "", nil, nil, map[string]any{
		"agent_type":  strings.TrimSpace(agentType),
		"success":     success,
		"error":       strings.TrimSpace(errorMsg),
		"duration_ms": durationMs,
	})
}

func (hm *HookManager) ConfigChange(ctx context.Context, source string, filePath string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "ConfigChange", strings.TrimSpace(source), nil, nil, map[string]any{
		"source":    strings.TrimSpace(source),
		"file_path": strings.TrimSpace(filePath),
	})
}

func (hm *HookManager) UserPromptSubmit(ctx context.Context, prompt string) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "UserPromptSubmit", "", nil, nil, map[string]any{
		"prompt": strings.TrimSpace(prompt),
	})
}

func (hm *HookManager) Stop(ctx context.Context, lastAssistantMessage string, stopHookActive bool) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "Stop", "", nil, nil, map[string]any{
		"stop_hook_active":       stopHookActive,
		"last_assistant_message": strings.TrimSpace(lastAssistantMessage),
	})
}

func (hm *HookManager) SubagentStop(ctx context.Context, agentType string, lastAssistantMessage string, stopHookActive bool) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, "SubagentStop", agentType, nil, nil, map[string]any{
		"stop_hook_active":       stopHookActive,
		"agent_type":             strings.TrimSpace(agentType),
		"last_assistant_message": strings.TrimSpace(lastAssistantMessage),
	})
}

func (hm *HookManager) run(ctx context.Context, event string, toolName string, toolInput map[string]any, toolResult map[string]any) (hooks.Decision, error) {
	return hm.runWithExtra(ctx, event, toolName, toolInput, toolResult, nil)
}

func (hm *HookManager) runWithExtra(ctx context.Context, event string, toolName string, toolInput map[string]any, toolResult map[string]any, extra map[string]any) (hooks.Decision, error) {
	hm.mu.RLock()
	base := hm.base
	mdl := hm.mdl
	agentEval := hm.agentEval
	tm := hm.tm
	onMeta := hm.onMeta
	hm.mu.RUnlock()
	if base.DisableAllHooks {
		return hooks.Decision{Decision: "allow"}, nil
	}
	enabledSources := normalizeSources(base.EnabledHookSources)
	if base.ManagedHooksOnly && len(enabledSources) == 0 {
		enabledSources = map[string]bool{
			"project_settings": true,
			"local_settings":   true,
			"skills":           true,
			"plugins":          true,
		}
	}

	groups := make([]hooks.MatcherGroup, 0)
	if base.Hooks != nil {
		for _, g := range base.Hooks[event] {
			if allowHookSource(enabledSources, g.Source) {
				groups = append(groups, g)
			}
		}
	}
	if hm.tm != nil {
		if sm := hm.tm.GetSkillManager(); sm != nil {
			for _, s := range sm.GetActive() {
				if s == nil || s.Hooks == nil {
					continue
				}
				if hs, ok := s.Hooks[event]; ok {
					if allowHookSource(enabledSources, "skills") {
						for i := range hs {
							hs[i].Source = "skills"
							hs[i].BaseDir = strings.TrimSpace(s.BaseDir)
							hs[i].SourcePath = strings.TrimSpace(s.SkillMdPath)
							groups = append(groups, hs[i])
						}
					}
				}
			}
		}
	}

	type workItem struct {
		idx     int
		key     string
		typ     string
		source  string
		baseDir string
		h       hooks.Handler
	}
	items := make([]workItem, 0, 8)
	seen := map[string]bool{}
	idx := 0

	for _, g := range groups {
		if supportsMatcher(event) && !hookMatcherMatch(g.Matcher, toolName) {
			continue
		}
		for _, h := range g.Hooks {
			ht := strings.ToLower(strings.TrimSpace(h.Type))
			key := handlerKey(event, h)
			if h.Once {
				hm.mu.Lock()
				if hm.onceSeen[key] {
					hm.mu.Unlock()
					continue
				}
				hm.onceSeen[key] = true
				hm.mu.Unlock()
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			items = append(items, workItem{
				idx:     idx,
				key:     key,
				typ:     ht,
				source:  strings.TrimSpace(g.Source),
				baseDir: strings.TrimSpace(g.BaseDir),
				h:       h,
			})
			idx++
		}
	}

	payload := hookInputPayload(ctx, event, toolName, toolInput, toolResult, extra)

	type res struct {
		idx int
		dec hooks.Decision
		err error
	}
	ch := make(chan res, len(items))
	pending := 0

	for _, it := range items {
		h := it.h
		switch it.typ {
		case "command":
			cmd := strings.TrimSpace(h.Command)
			if cmd == "" {
				continue
			}
			if onMeta != nil && strings.TrimSpace(h.StatusMessage) != "" {
				onMeta("phase.note:HOOK_STATUS:" + strings.TrimSpace(h.StatusMessage))
			}
			to := h.Timeout
			if to <= 0 {
				to = 600
			}
			if h.Async {
				go func(cmd string, source string, baseDir string, timeout int) {
					c2, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
					defer cancel()
					_, _ = runHookCommand(c2, event, cmd, toolName, toolInput, toolResult, extra, source, baseDir, time.Duration(timeout)*time.Second)
				}(cmd, it.source, it.baseDir, to)
				continue
			}
			pending++
			go func(i int, cmd string, source string, baseDir string, timeout int) {
				dec, err := runHookCommand(ctx, event, cmd, toolName, toolInput, toolResult, extra, source, baseDir, time.Duration(timeout)*time.Second)
				ch <- res{idx: i, dec: dec, err: err}
			}(it.idx, cmd, it.source, it.baseDir, to)
		case "prompt":
			p := strings.TrimSpace(h.Prompt)
			if p == "" {
				continue
			}
			if onMeta != nil && strings.TrimSpace(h.StatusMessage) != "" {
				onMeta("phase.note:HOOK_STATUS:" + strings.TrimSpace(h.StatusMessage))
			}
			to := h.Timeout
			if to <= 0 {
				to = 30
			}
			pending++
			go func(i int, prompt string, timeout int, modelName string) {
				m := mdl
				if strings.TrimSpace(modelName) != "" {
					m = hm.getOrCreateHookModel(ctx, strings.TrimSpace(modelName))
				}
				if m == nil {
					ch <- res{idx: i, err: errors.New("model unavailable")}
					return
				}
				dec, err := runHookPrompt(ctx, m, event, prompt, payload, time.Duration(timeout)*time.Second)
				ch <- res{idx: i, dec: dec, err: err}
			}(it.idx, p, to, h.Model)
		case "agent":
			p := strings.TrimSpace(h.Prompt)
			if p == "" {
				continue
			}
			if onMeta != nil && strings.TrimSpace(h.StatusMessage) != "" {
				onMeta("phase.note:HOOK_STATUS:" + strings.TrimSpace(h.StatusMessage))
			}
			to := h.Timeout
			if to <= 0 {
				to = 60
			}
			pending++
			go func(i int, prompt string, timeout int, modelName string) {
				eval := agentEval
				if strings.TrimSpace(modelName) != "" {
					eval = hm.getOrCreateAgentEval(ctx, strings.TrimSpace(modelName), tm)
				}
				if eval == nil {
					ch <- res{idx: i, err: errors.New("agent evaluator unavailable")}
					return
				}
				dec, err := runHookAgent(ctx, eval, event, prompt, payload, time.Duration(timeout)*time.Second)
				ch <- res{idx: i, dec: dec, err: err}
			}(it.idx, p, to, h.Model)
		case "http":
			if onMeta != nil && strings.TrimSpace(h.StatusMessage) != "" {
				onMeta("phase.note:HOOK_STATUS:" + strings.TrimSpace(h.StatusMessage))
			}
			to := h.Timeout
			if to <= 0 {
				to = 30
			}
			pending++
			go func(i int, handler hooks.Handler, timeout int) {
				dec, err := runHookHTTP(ctx, handler, payload, time.Duration(timeout)*time.Second)
				ch <- res{idx: i, dec: dec, err: err}
			}(it.idx, h, to)
		}
	}

	out := make([]res, 0, pending)
	for i := 0; i < pending; i++ {
		out = append(out, <-ch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].idx < out[j].idx })

	var addCtx strings.Builder
	updated := map[string]any{}
	allowSession := false
	finalDecision := "allow"
	finalReason := ""

	for _, r := range out {
		if r.err != nil {
			continue
		}
		if r.dec.UpdatedInput != nil {
			for k, v := range r.dec.UpdatedInput {
				updated[k] = v
			}
		}
		if r.dec.AllowSession {
			allowSession = true
		}
		if strings.TrimSpace(r.dec.AdditionalContext) != "" {
			if addCtx.Len() > 0 {
				addCtx.WriteString("\n")
			}
			addCtx.WriteString(strings.TrimSpace(r.dec.AdditionalContext))
		}
		if finalDecision == "allow" {
			if strings.EqualFold(r.dec.Decision, "deny") || strings.EqualFold(r.dec.Decision, "block") {
				finalDecision = strings.ToLower(strings.TrimSpace(r.dec.Decision))
				finalReason = strings.TrimSpace(r.dec.Reason)
			}
		}
	}

	if len(updated) == 0 {
		updated = nil
	}
	return hooks.Decision{
		Decision:          finalDecision,
		Reason:            finalReason,
		AdditionalContext: strings.TrimSpace(addCtx.String()),
		UpdatedInput:      updated,
		AllowSession:      allowSession,
	}, nil
}

func handlerKey(event string, h hooks.Handler) string {
	typ := strings.ToLower(strings.TrimSpace(h.Type))
	cmd := strings.TrimSpace(h.Command)
	p := strings.TrimSpace(h.Prompt)
	model := strings.TrimSpace(h.Model)
	return strings.TrimSpace(event) + "|" + typ + "|" + cmd + "|" + p + "|" + model + "|" + fmt.Sprintf("%d", h.Timeout) + "|" + fmt.Sprintf("%t", h.Async) + "|" + fmt.Sprintf("%t", h.Once)
}

func (hm *HookManager) getOrCreateHookModel(ctx context.Context, modelName string) AIModel {
	name := strings.TrimSpace(modelName)
	if name == "" {
		return nil
	}
	hm.mu.RLock()
	if m := hm.hookModels[name]; m != nil {
		hm.mu.RUnlock()
		return m
	}
	hm.mu.RUnlock()

	apiKey, base, _ := ai.ResolveAPISettings()
	m, err := NewChatModelWithSettings(ctx, apiKey, base, name, "")
	if err != nil {
		return nil
	}

	hm.mu.Lock()
	if ex := hm.hookModels[name]; ex != nil {
		hm.mu.Unlock()
		return ex
	}
	hm.hookModels[name] = m
	hm.mu.Unlock()
	return m
}

func (hm *HookManager) getOrCreateAgentEval(ctx context.Context, modelName string, tm *tools.Manager) func(context.Context, string) (string, error) {
	name := strings.TrimSpace(modelName)
	if name == "" || tm == nil {
		return nil
	}
	hm.mu.RLock()
	if fn := hm.agentEvals[name]; fn != nil {
		hm.mu.RUnlock()
		return fn
	}
	hm.mu.RUnlock()

	m := hm.getOrCreateHookModel(ctx, name)
	provider, ok := m.(ToolCallingProvider)
	if !ok {
		return nil
	}
	toolsList := BuildRuntimeReadOnlyToolsWithMCP(ctx, tm, nil, nil)
	cfg0, _ := config.Load()
	maxStep := cfg0.Agent.MaxStep
	if maxStep <= 0 {
		maxStep = 160
	}
	if maxStep < 12 {
		maxStep = 12
	}
	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: provider.ToolCalling(),
		ToolsConfig:      buildToolsNodeConfig(toolsList, nil),
		MaxStep:          maxStep,
	})
	if err != nil {
		return nil
	}
	fn := func(ctx context.Context, prompt string) (string, error) {
		out, err := agent.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
		if err != nil {
			return "", err
		}
		return out.Content, nil
	}

	hm.mu.Lock()
	if ex := hm.agentEvals[name]; ex != nil {
		hm.mu.Unlock()
		return ex
	}
	hm.agentEvals[name] = fn
	hm.mu.Unlock()
	return fn
}

func hookInputPayload(ctx context.Context, event string, toolName string, toolInput map[string]any, toolResult map[string]any, extra map[string]any) map[string]any {
	payload := map[string]any{
		"hook_event_name": event,
	}
	if strings.TrimSpace(toolName) != "" {
		payload["tool_name"] = toolName
	}
	if toolInput != nil {
		payload["tool_input"] = toolInput
	}
	if toolResult != nil {
		payload["tool_result"] = toolResult
	}
	for k, v := range extra {
		payload[k] = v
	}
	if tid := tools.TraceIDFromContext(ctx); strings.TrimSpace(tid) != "" {
		payload["trace_id"] = tid
	}
	if wd := strings.TrimSpace(tools.WorkspaceRootFromContext(ctx)); wd != "" {
		payload["cwd"] = filepath.ToSlash(wd)
	} else if wd, err := os.Getwd(); err == nil {
		payload["cwd"] = filepath.ToSlash(wd)
	}
	return payload
}

func supportsMatcher(event string) bool {
	switch strings.TrimSpace(event) {
	case "UserPromptSubmit", "Stop", "TeammateIdle", "TaskCompleted", "WorktreeCreate", "WorktreeRemove":
		return false
	default:
		return true
	}
}

func hookMatcherMatch(matcher string, toolName string) bool {
	m := strings.TrimSpace(matcher)
	if m == "" || m == "*" {
		return true
	}

	// Check if this is a glob pattern (contains *, ?, [, but not regex chars)
	if isGlobPattern(m) {
		return globMatch(m, toolName)
	}

	// Default: regex matching
	re, err := regexp.Compile(m)
	if err != nil {
		return false
	}
	return re.MatchString(toolName)
}

// isGlobPattern detects if a pattern is a glob (not a regex)
func isGlobPattern(pattern string) bool {
	// If it contains glob-specific characters without regex escaping
	hasGlob := strings.ContainsAny(pattern, "?[")
	// Simple heuristic: if it has * but not .* or similar regex patterns
	if strings.Contains(pattern, "*") && !strings.Contains(pattern, ".*") {
		hasGlob = true
	}
	return hasGlob && !strings.ContainsAny(pattern, `+\()|`)
}

// globMatch performs simple glob matching
func globMatch(pattern, name string) bool {
	// Simple glob: convert glob to regex
	regexPattern := globToRegex(pattern)
	re, err := regexp.Compile("(?i)^" + regexPattern + "$")
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

// globToRegex converts a glob pattern to a regex pattern
func globToRegex(glob string) string {
	var sb strings.Builder
	sb.Grow(len(glob) * 2)
	for _, ch := range glob {
		switch ch {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString(".")
		case '.', '^', '$', '|', '+', '(', ')', '{', '}', '\\':
			sb.WriteRune('\\')
			sb.WriteRune(ch)
		default:
			sb.WriteRune(ch)
		}
	}
	return sb.String()
}

func runHookCommand(ctx context.Context, event string, command string, toolName string, toolInput map[string]any, toolResult map[string]any, extra map[string]any, source string, baseDir string, timeout time.Duration) (hooks.Decision, error) {
	call := tools.ToolCall{Tool: "bash", Parameters: map[string]any{"command": command}}
	if tools.SafetyGateClassify != nil {
		category, _, summary, dangerous := tools.SafetyGateClassify(call)
		if dangerous && (tools.SafetyGateSessionAllowed == nil || !tools.SafetyGateSessionAllowed(category)) {
			if tools.SafetyGatePrompt == nil {
				return hooks.Decision{}, errors.New("permission required")
			}
			dec := tools.SafetyGatePrompt(ctx, category, summary)
			if dec == "deny" {
				return hooks.Decision{Decision: "deny", Reason: summary}, nil
			}
			if dec == "session" && tools.SafetyGateAllowSession != nil {
				tools.SafetyGateAllowSession(category)
			}
		}
	}

	payload := map[string]any{
		"hook_event_name": event,
		"tool_name":       toolName,
		"tool_input":      toolInput,
	}
	for k, v := range extra {
		payload[k] = v
	}
	if toolResult != nil {
		payload["tool_result"] = toolResult
	}
	if tid := tools.TraceIDFromContext(ctx); strings.TrimSpace(tid) != "" {
		payload["trace_id"] = tid
	}
	if wd := strings.TrimSpace(tools.WorkspaceRootFromContext(ctx)); wd != "" {
		payload["cwd"] = filepath.ToSlash(wd)
	} else if wd, err := os.Getwd(); err == nil {
		payload["cwd"] = filepath.ToSlash(wd)
	}
	in, _ := json.Marshal(payload)

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	workingDir := currentHookWorkingDir(ctx)
	env := hookCommandEnv(source, baseDir, workingDir)
	stdout, stderr, exitCode, err := shell.ExecuteWithStdinEnv(ctx, command, workingDir, string(in), env)
	if exitCode == 2 {
		msg := strings.TrimSpace(stderr)
		if msg == "" && err != nil {
			msg = strings.TrimSpace(err.Error())
		}
		switch event {
		case "PreToolUse", "PermissionRequest":
			return hooks.Decision{Decision: "deny", Reason: msg}, nil
		case "UserPromptSubmit", "Stop", "SubagentStop", "ConfigChange":
			return hooks.Decision{Decision: "block", Reason: msg}, nil
		default:
			return hooks.Decision{Decision: "allow", AdditionalContext: msg}, nil
		}
	}
	if err != nil {
		return hooks.Decision{}, err
	}

	out := strings.TrimSpace(stdout)
	if out == "" {
		return hooks.Decision{Decision: "allow"}, nil
	}

	if strings.HasPrefix(out, "{") {
		var m map[string]any
		if e := json.Unmarshal([]byte(out), &m); e == nil {
			sysMsg, _ := m["systemMessage"].(string)
			sysMsg = strings.TrimSpace(sysMsg)
			suppress, _ := m["suppressOutput"].(bool)
			if cont, ok := m["continue"].(bool); ok {
				if !cont {
					reason, _ := m["stopReason"].(string)
					if strings.TrimSpace(reason) == "" {
						reason, _ = m["reason"].(string)
					}
					dec := hooks.Decision{Decision: decisionForOkFalse(event), Reason: strings.TrimSpace(reason), AdditionalContext: sysMsg}
					if suppress {
						dec.AdditionalContext = ""
					}
					return dec, nil
				}
				if sysMsg != "" {
					dec := hooks.Decision{Decision: "allow", AdditionalContext: sysMsg}
					if suppress {
						dec.AdditionalContext = ""
					}
					return dec, nil
				}
			}
			if d, ok := m["decision"].(string); ok {
				d = strings.ToLower(strings.TrimSpace(d))
				if d == "block" {
					reason, _ := m["reason"].(string)
					dec := hooks.Decision{Decision: "block", Reason: strings.TrimSpace(reason), AdditionalContext: sysMsg}
					if suppress {
						dec.AdditionalContext = ""
					}
					return dec, nil
				}
			}
			if hs, ok := m["hookSpecificOutput"].(map[string]any); ok {
				if hn, ok := hs["hookEventName"].(string); ok && strings.TrimSpace(hn) != "" {
					_ = hn
				}
				if b, ok := hs["suppressOutput"].(bool); ok && b {
					suppress = true
				}
				if sm, ok := hs["systemMessage"].(string); ok && strings.TrimSpace(sm) != "" {
					sysMsg = strings.TrimSpace(sm)
				}
				if dm, ok := hs["decision"].(map[string]any); ok {
					if b, ok := dm["behavior"].(string); ok {
						behavior := strings.ToLower(strings.TrimSpace(b))
						dec := hooks.Decision{Decision: behavior}
						if behavior == "deny" {
							if msg, ok := dm["message"].(string); ok {
								dec.Reason = strings.TrimSpace(msg)
							}
						}
						if ui := asMapStringAny(dm["updatedInput"]); ui != nil {
							dec.UpdatedInput = ui
						}
						if dm["updatedPermissions"] != nil {
							dec.AllowSession = true
						}
						if ac, ok := hs["additionalContext"].(string); ok && strings.TrimSpace(ac) != "" {
							dec.AdditionalContext = strings.TrimSpace(ac)
						}
						if dec.AdditionalContext == "" {
							dec.AdditionalContext = sysMsg
						} else if sysMsg != "" {
							dec.AdditionalContext = strings.TrimSpace(dec.AdditionalContext + "\n" + sysMsg)
						}
						if suppress {
							dec.AdditionalContext = ""
						}
						return dec, nil
					}
				}
				if pd, ok := hs["permissionDecision"].(string); ok {
					reason, _ := hs["permissionDecisionReason"].(string)
					dec := hooks.Decision{Decision: strings.ToLower(strings.TrimSpace(pd)), Reason: strings.TrimSpace(reason)}
					if ui := asMapStringAny(hs["updatedInput"]); ui != nil {
						dec.UpdatedInput = ui
					}
					if ac, ok := hs["additionalContext"].(string); ok && strings.TrimSpace(ac) != "" {
						dec.AdditionalContext = strings.TrimSpace(ac)
					}
					if dec.AdditionalContext == "" {
						dec.AdditionalContext = sysMsg
					} else if sysMsg != "" {
						dec.AdditionalContext = strings.TrimSpace(dec.AdditionalContext + "\n" + sysMsg)
					}
					if suppress {
						dec.AdditionalContext = ""
					}
					return dec, nil
				}
				if ac, ok := hs["additionalContext"].(string); ok && strings.TrimSpace(ac) != "" {
					dec := hooks.Decision{Decision: "allow", AdditionalContext: strings.TrimSpace(ac)}
					if sysMsg != "" {
						dec.AdditionalContext = strings.TrimSpace(dec.AdditionalContext + "\n" + sysMsg)
					}
					if suppress {
						dec.AdditionalContext = ""
					}
					return dec, nil
				}
			}
			if ac, ok := m["additionalContext"].(string); ok && strings.TrimSpace(ac) != "" {
				dec := hooks.Decision{Decision: "allow", AdditionalContext: strings.TrimSpace(ac)}
				if sysMsg != "" {
					dec.AdditionalContext = strings.TrimSpace(dec.AdditionalContext + "\n" + sysMsg)
				}
				if suppress {
					dec.AdditionalContext = ""
				}
				return dec, nil
			}
			if sysMsg != "" && !suppress {
				return hooks.Decision{Decision: "allow", AdditionalContext: sysMsg}, nil
			}
		}
	}

	return hooks.Decision{Decision: "allow", AdditionalContext: out}, nil
}

func runHookPrompt(ctx context.Context, mdl AIModel, event string, prompt string, payload map[string]any, timeout time.Duration) (hooks.Decision, error) {
	in, _ := json.Marshal(payload)
	p := strings.TrimSpace(prompt)
	arg := string(in)
	if strings.Contains(p, "$ARGUMENTS") {
		p = strings.ReplaceAll(p, "$ARGUMENTS", arg)
	} else {
		p = strings.TrimSpace(p + "\n\n" + arg)
	}
	p = strings.TrimSpace("Respond with JSON only: {\"ok\": true|false, \"reason\": \"...\", \"systemMessage\": \"...\", \"suppressOutput\": false}\n\n" + p)

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	out, err := mdl.Chat(ctx, []ai.Message{{Role: "user", Content: p}})
	if err != nil {
		return hooks.Decision{}, err
	}
	ok, reason, sysMsg, suppress, perr := parseHookOk(out)
	if perr != nil {
		return hooks.Decision{}, perr
	}
	if ok {
		dec := hooks.Decision{Decision: "allow", AdditionalContext: sysMsg}
		if suppress {
			dec.AdditionalContext = ""
		}
		return dec, nil
	}
	dec := hooks.Decision{Decision: decisionForOkFalse(event), Reason: strings.TrimSpace(reason), AdditionalContext: sysMsg}
	if suppress {
		dec.AdditionalContext = ""
	}
	return dec, nil
}

func runHookAgent(ctx context.Context, eval func(context.Context, string) (string, error), event string, prompt string, payload map[string]any, timeout time.Duration) (hooks.Decision, error) {
	in, _ := json.Marshal(payload)
	p := strings.TrimSpace(prompt)
	arg := string(in)
	if strings.Contains(p, "$ARGUMENTS") {
		p = strings.ReplaceAll(p, "$ARGUMENTS", arg)
	} else {
		p = strings.TrimSpace(p + "\n\n" + arg)
	}
	p = strings.TrimSpace("Respond with JSON only: {\"ok\": true|false, \"reason\": \"...\", \"systemMessage\": \"...\", \"suppressOutput\": false}\n\n" + p)

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	out, err := eval(ctx, p)
	if err != nil {
		return hooks.Decision{}, err
	}
	ok, reason, sysMsg, suppress, perr := parseHookOk(out)
	if perr != nil {
		return hooks.Decision{}, perr
	}
	if ok {
		dec := hooks.Decision{Decision: "allow", AdditionalContext: sysMsg}
		if suppress {
			dec.AdditionalContext = ""
		}
		return dec, nil
	}
	dec := hooks.Decision{Decision: decisionForOkFalse(event), Reason: strings.TrimSpace(reason), AdditionalContext: sysMsg}
	if suppress {
		dec.AdditionalContext = ""
	}
	return dec, nil
}

func decisionForOkFalse(event string) string {
	switch strings.TrimSpace(event) {
	case "PreToolUse", "PermissionRequest":
		return "deny"
	default:
		return "block"
	}
}

// runHookHTTP executes an HTTP-based hook handler
func runHookHTTP(ctx context.Context, h hooks.Handler, payload map[string]any, timeout time.Duration) (hooks.Decision, error) {
	// Extract HTTP config from handler
	// The HTTP config is encoded in the Command field as JSON or URL
	url := strings.TrimSpace(h.Command)
	if url == "" {
		return hooks.Decision{Decision: "allow"}, nil
	}

	// Check if Command is a JSON-encoded HTTP config
	if strings.HasPrefix(url, "{") {
		var httpCfg hooks.HTTPHandler
		if err := json.Unmarshal([]byte(url), &httpCfg); err == nil && httpCfg.URL != "" {
			return executeHTTPHook(ctx, &httpCfg, payload, timeout)
		}
	}

	// Treat Command as a plain URL
	method := "POST"
	if h.Prompt != "" {
		method = strings.ToUpper(strings.TrimSpace(h.Prompt))
	}

	httpCfg := &hooks.HTTPHandler{
		URL:     url,
		Method:  method,
		Timeout: int(timeout.Seconds()),
		Headers: map[string]string{"Content-Type": "application/json"},
	}

	return executeHTTPHook(ctx, httpCfg, payload, timeout)
}

// executeHTTPHook performs the actual HTTP request for a hook
func executeHTTPHook(ctx context.Context, cfg *hooks.HTTPHandler, payload map[string]any, timeout time.Duration) (hooks.Decision, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(payload)
	if err != nil {
		return hooks.Decision{}, fmt.Errorf("marshal hook payload: %w", err)
	}

	req, err := stdhttp.NewRequestWithContext(ctx, cfg.Method, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return hooks.Decision{}, fmt.Errorf("create hook request: %w", err)
	}

	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := stdhttp.DefaultClient.Do(req)
	if err != nil {
		return hooks.Decision{}, fmt.Errorf("execute hook request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return hooks.Decision{}, fmt.Errorf("read hook response: %w", err)
	}

	expectedStatus := cfg.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	if resp.StatusCode != expectedStatus {
		return hooks.Decision{Decision: "allow"}, nil
	}

	// Parse response as JSON
	ok, reason, sysMsg, suppress, perr := parseHookOk(string(respBody))
	if perr != nil {
		return hooks.Decision{}, perr
	}
	if ok {
		dec := hooks.Decision{Decision: "allow", AdditionalContext: sysMsg}
		if suppress {
			dec.AdditionalContext = ""
		}
		return dec, nil
	}
	dec := hooks.Decision{Decision: "block", Reason: strings.TrimSpace(reason), AdditionalContext: sysMsg}
	if suppress {
		dec.AdditionalContext = ""
	}
	return dec, nil
}

func parseHookOk(s string) (ok bool, reason string, sysMsg string, suppress bool, err error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return false, "", "", false, errors.New("empty hook response")
	}
	i := strings.Index(raw, "{")
	j := strings.LastIndex(raw, "}")
	if i >= 0 && j > i {
		raw = raw[i : j+1]
	}
	var m map[string]any
	if e := json.Unmarshal([]byte(raw), &m); e != nil {
		return false, "", "", false, fmt.Errorf("invalid hook json: %w", e)
	}
	if b, ok2 := m["suppressOutput"].(bool); ok2 {
		suppress = b
	}
	if sm, ok2 := m["systemMessage"].(string); ok2 {
		sysMsg = strings.TrimSpace(sm)
	}
	if cont, ok2 := m["continue"].(bool); ok2 {
		if !cont {
			sr, _ := m["stopReason"].(string)
			if strings.TrimSpace(sr) == "" {
				sr, _ = m["reason"].(string)
			}
			return false, strings.TrimSpace(sr), sysMsg, suppress, nil
		}
		return true, "", sysMsg, suppress, nil
	}
	if b, ok2 := m["ok"].(bool); ok2 {
		r, _ := m["reason"].(string)
		return b, strings.TrimSpace(r), sysMsg, suppress, nil
	}
	if sysMsg != "" {
		return true, "", sysMsg, suppress, nil
	}
	return false, "", "", false, errors.New("missing ok/continue in hook response")
}

func asMapStringAny(v any) map[string]any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map {
		return nil
	}
	out := map[string]any{}
	iter := rv.MapRange()
	for iter.Next() {
		k := iter.Key()
		if k.Kind() != reflect.String {
			continue
		}
		out[k.String()] = iter.Value().Interface()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func loadHookConfig(workspaceRoot string) (hooks.Config, error) {
	var cfg hooks.Config
	cfg.Hooks = map[string][]hooks.MatcherGroup{}
	workspaceRoot = resolveHookWorkspaceRoot(workspaceRoot)

	home, _ := os.UserHomeDir()
	type p2 struct {
		path   string
		source string
	}
	paths := []p2{}

	fileExists := func(p string) bool {
		if strings.TrimSpace(p) == "" {
			return false
		}
		fi, err := os.Stat(p)
		return err == nil && !fi.IsDir()
	}

	addIfExists := func(p, src string) {
		if fileExists(p) {
			paths = append(paths, p2{path: p, source: src})
		}
	}

	if home != "" {
		vbUser := filepath.Join(home, ".vb", "settings.json")
		if fileExists(vbUser) {
			addIfExists(vbUser, "user_settings")
		} else {
			addIfExists(filepath.Join(home, ".claude", "settings.json"), "user_settings")
			addIfExists(filepath.Join(home, ".trae", "settings.json"), "user_settings")
		}
	}

	if wd := strings.TrimSpace(workspaceRoot); wd != "" {
		vbProject := filepath.Join(wd, ".vb", "settings.json")
		vbLocal := filepath.Join(wd, ".vb", "settings.local.json")
		workspaceUsesVB := fileExists(vbProject) || fileExists(vbLocal)

		if workspaceUsesVB {
			addIfExists(vbProject, "project_settings")
			addIfExists(vbLocal, "local_settings")
		} else {
			addIfExists(filepath.Join(wd, ".claude", "settings.json"), "project_settings")
			addIfExists(filepath.Join(wd, ".claude", "settings.local.json"), "local_settings")
			addIfExists(filepath.Join(wd, ".trae", "settings.json"), "project_settings")
			addIfExists(filepath.Join(wd, ".trae", "settings.local.json"), "local_settings")
		}
	} else if wd, err := os.Getwd(); err == nil {
		vbProject := filepath.Join(wd, ".vb", "settings.json")
		vbLocal := filepath.Join(wd, ".vb", "settings.local.json")
		workspaceUsesVB := fileExists(vbProject) || fileExists(vbLocal)

		if workspaceUsesVB {
			addIfExists(vbProject, "project_settings")
			addIfExists(vbLocal, "local_settings")
		} else {
			addIfExists(filepath.Join(wd, ".claude", "settings.json"), "project_settings")
			addIfExists(filepath.Join(wd, ".claude", "settings.local.json"), "local_settings")
			addIfExists(filepath.Join(wd, ".trae", "settings.json"), "project_settings")
			addIfExists(filepath.Join(wd, ".trae", "settings.local.json"), "local_settings")
		}
	}

	for _, it := range paths {
		b, err := os.ReadFile(it.path)
		if err != nil {
			continue
		}
		var raw hooks.Config
		if e := json.Unmarshal(b, &raw); e != nil {
			continue
		}
		mergeHookConfig(&cfg, raw, it.source, it.path, workspaceRoot, true)
	}

	appCfg, _ := config.Load()
	plugins, _ := pluginpkg.Discover(workspaceRoot)
	for _, plugin := range plugins {
		if !plugin.HasHooks {
			continue
		}
		enabled := true
		if cfgEnabled, ok := config.PluginEnabled(&appCfg, plugin.Name); ok {
			enabled = cfgEnabled
		}
		if !enabled {
			continue
		}
		b, err := os.ReadFile(plugin.HookConfigPath())
		if err != nil {
			continue
		}
		var raw struct {
			Description string                          `json:"description"`
			Hooks       map[string][]hooks.MatcherGroup `json:"hooks"`
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			continue
		}
		mergeHookConfig(&cfg, hooks.Config{Hooks: raw.Hooks}, "plugin:"+strings.TrimSpace(plugin.Name), plugin.HookConfigPath(), plugin.RootDir, false)
	}
	return cfg, nil
}

func mergeHookConfig(dst *hooks.Config, raw hooks.Config, source string, sourcePath string, baseDir string, includeFlags bool) {
	if dst == nil {
		return
	}
	if dst.Hooks == nil {
		dst.Hooks = map[string][]hooks.MatcherGroup{}
	}
	if includeFlags {
		if raw.DisableAllHooks {
			dst.DisableAllHooks = true
		}
		if raw.ManagedHooksOnly {
			dst.ManagedHooksOnly = true
		}
		if len(raw.EnabledHookSources) > 0 {
			dst.EnabledHookSources = append(dst.EnabledHookSources, raw.EnabledHookSources...)
		}
	}
	for ev, groups := range raw.Hooks {
		for i := range groups {
			groups[i].Source = source
			groups[i].BaseDir = strings.TrimSpace(baseDir)
			groups[i].SourcePath = strings.TrimSpace(sourcePath)
		}
		dst.Hooks[ev] = append(dst.Hooks[ev], groups...)
	}
}

func resolveHookWorkspaceRoot(workspaceRoot string) string {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot != "" {
		return workspaceRoot
	}
	if wd, err := os.Getwd(); err == nil {
		return strings.TrimSpace(wd)
	}
	return ""
}

func normalizeSources(in []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			out[s] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func allowHookSource(enabled map[string]bool, src string) bool {
	if enabled == nil {
		return true
	}
	s := strings.ToLower(strings.TrimSpace(src))
	if s == "" {
		s = "unknown"
	}
	if enabled[s] {
		return true
	}
	base := hookSourceCategory(s)
	if enabled[base] {
		return true
	}
	if base == "plugin" && enabled["plugins"] {
		return true
	}
	return false
}

func hookSourceCategory(src string) string {
	src = strings.ToLower(strings.TrimSpace(src))
	if src == "" {
		return "unknown"
	}
	if idx := strings.Index(src, ":"); idx > 0 {
		return src[:idx]
	}
	return src
}

func currentHookWorkingDir(ctx context.Context) string {
	if wd := strings.TrimSpace(tools.WorkspaceRootFromContext(ctx)); wd != "" {
		return wd
	}
	if wd, err := os.Getwd(); err == nil {
		return strings.TrimSpace(wd)
	}
	return ""
}

func hookCommandEnv(source string, baseDir string, projectDir string) []string {
	env := append([]string{}, os.Environ()...)
	if strings.TrimSpace(projectDir) != "" {
		env = setHookEnv(env, "CLAUDE_PROJECT_DIR", projectDir)
	}
	if hookSourceCategory(source) == "plugin" && strings.TrimSpace(baseDir) != "" {
		env = setHookEnv(env, "CLAUDE_PLUGIN_ROOT", baseDir)
		pluginName := ""
		if plugin, ok := pluginpkg.FindOwningManifest(baseDir); ok {
			pluginName = strings.TrimSpace(plugin.Name)
		}
		if pluginName == "" {
			parts := strings.SplitN(strings.TrimSpace(source), ":", 2)
			if len(parts) == 2 {
				pluginName = strings.TrimSpace(parts[1])
			}
		}
		if dataDir := pluginpkg.PersistentDataDir(pluginName); strings.TrimSpace(dataDir) != "" {
			env = setHookEnv(env, "CLAUDE_PLUGIN_DATA", dataDir)
		}
	}
	return env
}

func setHookEnv(env []string, key string, value string) []string {
	key = strings.TrimSpace(key)
	if key == "" {
		return env
	}
	entry := key + "=" + strings.TrimSpace(value)
	prefix := key + "="
	for i := range env {
		if strings.HasPrefix(strings.ToUpper(env[i]), strings.ToUpper(prefix)) {
			env[i] = entry
			return env
		}
	}
	return append(env, entry)
}
