package bridge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/notify"
	"github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/dreamSailing/eos/internal/tools"

	"github.com/google/uuid"
)

var autoModePromptCategories = map[string]bool{
	"tool-delete":       true,
	"delete_file":       true,
	"git-push":          true,
	"git-reset":         true,
	"git-revert":        true,
	"git-merge":         true,
	"git-rebase":        true,
	"git-stash-apply":   true,
	"git-stash-drop":    true,
	"bg-task-start":     true,
	"bg-task-kill":      true,
	"bash-session-kill": true,
}

func promptNeedsSafetyConfirmation(kind PromptKind, category, summary string) bool {
	if kind != PromptKindPermission {
		return false
	}
	category = strings.ToLower(strings.TrimSpace(category))
	summary = strings.ToLower(strings.TrimSpace(summary))
	if autoModePromptCategories[category] {
		return true
	}
	return strings.Contains(summary, "restore checkpoint") || strings.Contains(summary, "恢复检查点")
}

type PromptKind string

const (
	PromptKindPermission  PromptKind = "permission"
	PromptKindUserConfirm PromptKind = "user_confirm"
)

type PromptRequest struct {
	ID       string
	Kind     PromptKind
	Title    string
	Question string
	Options  []string

	Category string
	Summary  string
	Diff     string
	DiffPath string

	AllowText bool
	TextHint  string
}

func (rc *RuntimeCore) SubmitPromptResponse(id string, r PromptResponse) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	rc.promptMu.Lock()
	ch := rc.prompts[id]
	rc.promptMu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- r:
		return true
	default:
		return false
	}
}

func (rc *RuntimeCore) waitPrompt(ctx context.Context, req PromptRequest) (PromptResponse, error) {
	if rc == nil {
		return PromptResponse{}, errors.New("runtime not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(req.ID) == "" {
		req.ID = uuid.New().String()[:8]
	}

	rc.promptMu.Lock()
	if rc.prompts == nil {
		rc.prompts = make(map[string]chan PromptResponse)
	}
	if _, exists := rc.prompts[req.ID]; exists {
		rc.promptMu.Unlock()
		return PromptResponse{}, errors.New("prompt already pending")
	}
	ch := make(chan PromptResponse, 1)
	rc.prompts[req.ID] = ch
	rc.promptMu.Unlock()

	defer func() {
		rc.promptMu.Lock()
		delete(rc.prompts, req.ID)
		rc.promptMu.Unlock()
	}()

	if pc := tools.PauseControllerFromContext(ctx); pc != nil {
		pc.Pause()
		defer pc.Resume()
	}

	desktopEnabled := true
	if rc != nil {
		s := rc.GetSettings()
		if s.DesktopNotifications != nil {
			desktopEnabled = *s.DesktopNotifications
		}
	}
	shouldNotify := desktopEnabled
	if shouldNotify && rc.securityMgr != nil {
		mode := rc.securityMgr.ExecutionMode()
		if mode == "auto" {
			shouldNotify = promptNeedsSafetyConfirmation(req.Kind, req.Category, req.Summary)
		}
	}
	if shouldNotify {
		switch req.Kind {
		case PromptKindPermission, PromptKindUserConfirm:
			title := strings.TrimSpace(req.Title)
			msg := strings.TrimSpace(req.Question)
			if msg == "" {
				msg = strings.TrimSpace(req.Summary)
			}
			if len(msg) > 220 {
				msg = msg[:220] + "…"
			}
			notify.NotifyAsync(title, msg)
		}
	}

	ev := bridgePromptEvent(req)
	if ev.Data == nil {
		ev.Data = map[string]any{}
	}
	ev.Data["ts"] = time.Now().Unix()
	rc.eventsCh <- ev

	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		return PromptResponse{}, ctx.Err()
	}
}

func (rc *RuntimeCore) promptPermission(ctx context.Context, category, summary string) string {
	if rc != nil && rc.securityMgr != nil {
		mode := toolapi.NormalizeExecutionMode(rc.securityMgr.ExecutionMode())
		switch mode {
		case "plan":
			rc.ClearPendingDiff()
			return "deny"
		case "auto":
			// Use classifier for auto mode decisions
			classifier := runtime.NewClassifier()
			result := classifier.Classify(category, summary)
			switch result.Action {
			case runtime.ActionAllow:
				rc.ClearPendingDiff()
				return "allow"
			case runtime.ActionDeny:
				// Fall through to UI prompt for dangerous operations
			}
			// For ActionAsk or ActionDeny that should still show UI
			if !promptNeedsSafetyConfirmation(PromptKindPermission, category, summary) {
				rc.ClearPendingDiff()
				return "allow"
			}
		}
	}
	req := PromptRequest{
		Kind:     PromptKindPermission,
		Title:    "Permission required",
		Question: summary,
		Options:  []string{"allow_once", "allow_session", "deny"},
		Category: category,
		Summary:  summary,
		Diff:     rc.GetPendingDiff(),
		DiffPath: rc.GetPendingDiffPath(),
	}
	r, err := rc.waitPrompt(ctx, req)
	rc.ClearPendingDiff()
	if err != nil {
		return "deny"
	}
	switch strings.ToLower(strings.TrimSpace(r.Decision)) {
	case "allow", "allow_once":
		return "allow"
	case "session", "allow_session":
		return "session"
	default:
		return "deny"
	}
}

func (rc *RuntimeCore) userConfirmPrompt(ctx context.Context, req tools.UserConfirmRequest) (tools.UserConfirmResponse, error) {
	title := strings.TrimSpace(req.Title)
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return tools.UserConfirmResponse{}, errors.New("question required")
	}
	opts := make([]string, 0, len(req.Options))
	for _, s := range req.Options {
		s = strings.TrimSpace(s)
		if s != "" {
			opts = append(opts, s)
		}
	}

	r, err := rc.waitPrompt(ctx, PromptRequest{
		Kind:      PromptKindUserConfirm,
		Title:     title,
		Question:  question,
		Options:   opts,
		AllowText: req.AllowText,
		TextHint:  strings.TrimSpace(req.TextHint),
	})
	if err != nil {
		return tools.UserConfirmResponse{}, err
	}

	decision := strings.ToLower(strings.TrimSpace(r.Decision))
	confirmed := decision == "confirm" || decision == "ok" || decision == "yes" || decision == "allow"
	return tools.UserConfirmResponse{
		Confirmed:   confirmed,
		Option:      strings.TrimSpace(r.Option),
		OptionIndex: r.OptionIndex,
		Text:        strings.TrimSpace(r.Text),
	}, nil
}

func (rc *RuntimeCore) askUserQuestionPrompt(ctx context.Context, req tools.AskUserQuestionRequest) (tools.AskUserQuestionResponse, error) {
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return tools.AskUserQuestionResponse{}, errors.New("question required")
	}
	opts := make([]string, 0, len(req.Options))
	for _, s := range req.Options {
		s = strings.TrimSpace(s)
		if s != "" {
			opts = append(opts, s)
		}
	}

	r, err := rc.waitPrompt(ctx, PromptRequest{
		Kind:     "inquiry",
		Question: question,
		Options:  opts,
	})
	if err != nil {
		return tools.AskUserQuestionResponse{}, err
	}

	return tools.AskUserQuestionResponse{
		Option: strings.TrimSpace(r.Option),
		Text:   strings.TrimSpace(r.Text),
	}, nil
}
