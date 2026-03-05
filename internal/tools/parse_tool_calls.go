package tools

import (
	"encoding/json"
	"strings"
)

// ParseToolCalls parses AI response to extract tool calls
func ParseToolCalls(response string) []ToolCall {
	var calls []ToolCall
	var buf []rune
	depth := 0
	inString := false
	escape := false

	for _, r := range response {
		if inString {
			buf = append(buf, r)
			if escape {
				escape = false
				continue
			}
			if r == '\\' {
				escape = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		if r == '"' {
			inString = true
			buf = append(buf, r)
			continue
		}
		if r == '{' {
			depth++
			buf = append(buf, r)
			continue
		}
		if depth > 0 {
			buf = append(buf, r)
		}
		if r == '}' && depth > 0 {
			depth--
			if depth == 0 {
				s := string(buf)
				buf = buf[:0]
				var call ToolCall
				if json.Unmarshal([]byte(s), &call) == nil && call.Tool != "" {
					calls = append(calls, call)
				}
			}
		}
	}
	return calls
}

// ParseToolCallsStrict only accepts a single strict JSON object or fenced JSON
// and returns exactly one ToolCall when valid. Otherwise returns nil,false.
func ParseToolCallsStrict(response string) ([]ToolCall, bool) {
	s := strings.TrimSpace(response)
	if s == "" {
		return nil, false
	}
	// fenced block
	if strings.HasPrefix(s, "```") {
		idx := strings.Index(s, "\n")
		if idx >= 0 {
			s = s[idx+1:]
		}
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
		s = strings.TrimSpace(s)
	}
	// strict single JSON object
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, false
	}
	var call ToolCall
	if json.Unmarshal([]byte(s), &call) == nil && strings.TrimSpace(call.Tool) != "" {
		return []ToolCall{call}, true
	}
	return nil, false
}
