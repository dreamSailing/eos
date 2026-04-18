package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"errors"
	"strconv"
	"strings"
)

func (m *Manager) userSelectStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	q, _ := params["question"].(string)
	q = strings.TrimSpace(q)
	if q == "" {
		return ToolResult{Type: "tool_result", Tool: ToolUserSelect, Status: "error", Error: "question required"}
	}
	title, _ := params["title"].(string)
	title = strings.TrimSpace(title)
	textHint, _ := params["text_hint"].(string)
	textHint = strings.TrimSpace(textHint)

	multi := false
	if v, ok := params["multi"].(bool); ok {
		multi = v
	}

	allowText := false
	if v, ok := params["allow_text"].(bool); ok {
		allowText = v
	}
	if multi {
		allowText = true
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
	if len(opts) == 0 {
		return ToolResult{Type: "tool_result", Tool: ToolUserSelect, Status: "error", Error: "options required"}
	}

	lang := strings.ToLower(strings.TrimSpace(LanguageFromContext(ctx)))
	if textHint == "" {
		if multi {
			if strings.HasPrefix(lang, "en") {
				textHint = "Multi-select: type 1,3,5"
			} else {
				textHint = "多选：在输入框输入 1,3,5"
			}
		} else if allowText {
			if strings.HasPrefix(lang, "en") {
				textHint = "Optional notes"
			} else {
				textHint = "可选：补充说明"
			}
		}
	}

	if UserConfirmPrompt == nil {
		return ToolResult{Type: "tool_result", Tool: ToolUserSelect, Status: "error", Error: "user selection UI unavailable"}
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
			return ToolResult{Type: "tool_result", Tool: ToolUserSelect, Status: "error", Error: "Canceled"}
		}
		return ToolResult{Type: "tool_result", Tool: ToolUserSelect, Status: "error", Error: err.Error()}
	}

	indices := []int{}
	rawText := strings.TrimSpace(res.Text)
	if res.Confirmed {
		if multi {
			indices = parseSelectionIndices(rawText, len(opts))
			if len(indices) == 0 && res.OptionIndex >= 0 && res.OptionIndex < len(opts) {
				indices = []int{res.OptionIndex}
			}
		} else {
			if res.OptionIndex >= 0 && res.OptionIndex < len(opts) {
				indices = []int{res.OptionIndex}
			}
		}
	}

	selected := []string{}
	for _, i := range indices {
		if i >= 0 && i < len(opts) {
			selected = append(selected, opts[i])
		}
	}

	data := map[string]any{
		"confirmed":        res.Confirmed,
		"multi":            multi,
		"options":          opts,
		"selected_indices": indices,
		"selected_options": selected,
		"text":             rawText,
		"option":           strings.TrimSpace(res.Option),
		"option_index":     res.OptionIndex,
	}

	display := "Canceled"
	if res.Confirmed {
		if len(selected) > 0 {
			display = strings.Join(selected, ", ")
		} else {
			display = "Confirmed"
		}
	}
	return ToolResult{Type: "tool_result", Tool: ToolUserSelect, Status: "success", Data: data, Display: display}
}

func parseSelectionIndices(s string, n int) []int {
	s = strings.TrimSpace(s)
	if s == "" || n <= 0 {
		return nil
	}
	s = strings.ReplaceAll(s, "，", ",")
	s = strings.ReplaceAll(s, ";", ",")
	s = strings.ReplaceAll(s, " ", ",")
	parts := strings.Split(s, ",")
	seen := map[int]bool{}
	var out []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "-") {
			rs := strings.SplitN(p, "-", 2)
			a, errA := strconv.Atoi(strings.TrimSpace(rs[0]))
			b, errB := strconv.Atoi(strings.TrimSpace(rs[1]))
			if errA == nil && errB == nil {
				if a > b {
					a, b = b, a
				}
				for i := a; i <= b; i++ {
					idx := i - 1
					if idx >= 0 && idx < n && !seen[idx] {
						seen[idx] = true
						out = append(out, idx)
					}
				}
			}
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		idx := v - 1
		if idx >= 0 && idx < n && !seen[idx] {
			seen[idx] = true
			out = append(out, idx)
		}
	}
	return out
}
