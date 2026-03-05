package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (m *Manager) userConfirmStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	q, _ := params["question"].(string)
	q = strings.TrimSpace(q)
	if q == "" {
		return ToolResult{Type: "tool_result", Tool: ToolUserConfirm, Status: "error", Error: "question required"}
	}
	title, _ := params["title"].(string)
	title = strings.TrimSpace(title)
	textHint, _ := params["text_hint"].(string)
	textHint = strings.TrimSpace(textHint)

	allowText := true
	if v, ok := params["allow_text"].(bool); ok {
		allowText = v
	}

	var opts []string
	if raw, ok := params["options"].([]any); ok {
		for _, it := range raw {
			if s, ok := it.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					opts = append(opts, s)
				}
			}
		}
	} else if raw, ok := params["options"].([]string); ok {
		for _, s := range raw {
			s = strings.TrimSpace(s)
			if s != "" {
				opts = append(opts, s)
			}
		}
	}

	if UserConfirmPrompt == nil {
		return ToolResult{Type: "tool_result", Tool: ToolUserConfirm, Status: "error", Error: "user confirmation UI unavailable"}
	}

	res, err := UserConfirmPrompt(ctx, UserConfirmRequest{
		Title:     title,
		Question:  q,
		Options:   opts,
		AllowText: allowText,
		TextHint:  textHint,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return ToolResult{Type: "tool_result", Tool: ToolUserConfirm, Status: "error", Error: "Canceled"}
		}
		return ToolResult{Type: "tool_result", Tool: ToolUserConfirm, Status: "error", Error: err.Error()}
	}

	data := map[string]interface{}{
		"confirmed":    res.Confirmed,
		"option":       res.Option,
		"option_index": res.OptionIndex,
		"text":         res.Text,
	}

	display := "Canceled"
	if res.Confirmed {
		if strings.TrimSpace(res.Option) != "" {
			display = "Confirmed: " + strings.TrimSpace(res.Option)
		} else {
			display = "Confirmed"
		}
		if strings.TrimSpace(res.Text) != "" {
			display += " | " + strings.TrimSpace(res.Text)
		}
	}

	return ToolResult{Type: "tool_result", Tool: ToolUserConfirm, Status: "success", Data: data, Display: fmt.Sprintf("%s", display)}
}

