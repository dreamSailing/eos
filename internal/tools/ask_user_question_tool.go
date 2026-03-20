package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type AskUserQuestionRequest struct {
	Question string
	Options  []string
}

type AskUserQuestionResponse struct {
	Option string
	Text   string
}

var AskUserQuestionPrompt func(context.Context, AskUserQuestionRequest) (AskUserQuestionResponse, error)

func (m *Manager) askUserQuestionStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	q, _ := params["question"].(string)
	q = strings.TrimSpace(q)
	if q == "" {
		return ToolResult{Type: "tool_result", Tool: ToolAskUserQuestion, Status: "error", Error: "question required"}
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

	if AskUserQuestionPrompt == nil {
		return ToolResult{Type: "tool_result", Tool: ToolAskUserQuestion, Status: "error", Error: "user inquiry UI unavailable"}
	}

	res, err := AskUserQuestionPrompt(ctx, AskUserQuestionRequest{
		Question: q,
		Options:  opts,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return ToolResult{Type: "tool_result", Tool: ToolAskUserQuestion, Status: "error", Error: "Canceled"}
		}
		return ToolResult{Type: "tool_result", Tool: ToolAskUserQuestion, Status: "error", Error: err.Error()}
	}

	data := map[string]interface{}{
		"option": res.Option,
		"text":   res.Text,
	}

	display := "User answered"
	if res.Option != "" {
		display += fmt.Sprintf(" (Option: %s)", res.Option)
	}
	if res.Text != "" {
		display += fmt.Sprintf(" (Text: %s)", res.Text)
	}

	return ToolResult{Type: "tool_result", Tool: ToolAskUserQuestion, Status: "success", Data: data, Display: display}
}