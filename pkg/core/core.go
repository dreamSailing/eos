package core

import (
	"context"
	"errors"
	"strings"

	"github.com/dreamSailing/vb-coding/internal/bridge"
	"github.com/dreamSailing/vb-coding/internal/config"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"
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
