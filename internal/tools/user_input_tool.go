package tools

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

func (m *Manager) userInputStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	q, _ := params["question"].(string)
	q = strings.TrimSpace(q)
	if q == "" {
		return ToolResult{Type: "tool_result", Tool: ToolUserInput, Status: "error", Error: "question required"}
	}
	title, _ := params["title"].(string)
	title = strings.TrimSpace(title)
	textHint, _ := params["text_hint"].(string)
	textHint = strings.TrimSpace(textHint)

	inputType, _ := params["input_type"].(string)
	inputType = strings.ToLower(strings.TrimSpace(inputType))
	if inputType == "" {
		inputType = "text"
	}

	allowEmpty := false
	if v, ok := params["allow_empty"].(bool); ok {
		allowEmpty = v
	}

	lang := strings.ToLower(strings.TrimSpace(LanguageFromContext(ctx)))
	if textHint == "" {
		if strings.HasPrefix(lang, "en") {
			switch inputType {
			case "integer":
				textHint = "Enter an integer"
			case "number":
				textHint = "Enter a number"
			default:
				textHint = "Type here"
			}
		} else {
			switch inputType {
			case "integer":
				textHint = "请输入整数"
			case "number":
				textHint = "请输入数字"
			default:
				textHint = "请输入"
			}
		}
	}

	if UserConfirmPrompt == nil {
		return ToolResult{Type: "tool_result", Tool: ToolUserInput, Status: "error", Error: "user input UI unavailable"}
	}

	res, err := UserConfirmPrompt(ctx, UserConfirmRequest{
		Title:     title,
		Question:  q,
		Options:   nil,
		AllowText: true,
		TextHint:  textHint,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return ToolResult{Type: "tool_result", Tool: ToolUserInput, Status: "error", Error: "Canceled"}
		}
		return ToolResult{Type: "tool_result", Tool: ToolUserInput, Status: "error", Error: err.Error()}
	}

	txt := strings.TrimSpace(res.Text)
	data := map[string]any{
		"confirmed": res.Confirmed,
		"text":      txt,
		"type":      inputType,
	}
	if res.Confirmed && !allowEmpty && txt == "" {
		return ToolResult{Type: "tool_result", Tool: ToolUserInput, Status: "error", Error: "empty input"}
	}

	if res.Confirmed && txt != "" {
		switch inputType {
		case "integer", "int":
			v, e := strconv.Atoi(txt)
			if e != nil {
				return ToolResult{Type: "tool_result", Tool: ToolUserInput, Status: "error", Error: "invalid integer"}
			}
			data["value"] = v
		case "number", "float":
			v, e := strconv.ParseFloat(txt, 64)
			if e != nil {
				return ToolResult{Type: "tool_result", Tool: ToolUserInput, Status: "error", Error: "invalid number"}
			}
			data["value"] = v
		}
	}

	display := "Canceled"
	if res.Confirmed {
		if txt == "" {
			display = "Confirmed"
		} else {
			display = txt
		}
	}
	return ToolResult{Type: "tool_result", Tool: ToolUserInput, Status: "success", Data: data, Display: display}
}
