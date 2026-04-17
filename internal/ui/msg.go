package ui

import (
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/pkg/protocol"
)

// Msg 是所有消息类型的接口
type Msg interface {
	msgType()
}

// ConvertEvent 将 bridge.Event 转换为 UI Msg
func ConvertEvent(e bridge.Event) Msg {
	switch e.Type {
	case "meta", "delta", string(protocol.EventTypeTextDelta):
		return AIResponseMsg{Type: "delta", Content: eventText(e, "text", "message"), RID: e.RID}
	case "final", string(protocol.EventTypeTextFinal):
		return AIResponseMsg{Type: "final", Content: eventText(e, "text", "message"), RID: e.RID}
	case string(protocol.EventTypeRequestDone):
		return InvokeDoneMsg{Content: eventString(e.Data, "text")}
	case "reasoning", "phase.note", string(protocol.EventTypeTextReasoning),
		string(protocol.EventTypeTaskStarted), string(protocol.EventTypeTaskUpdated), string(protocol.EventTypeTaskDone):
		return ThinkingMsg{RID: e.RID, Content: eventText(e, "message", "text", "label", "title"), Done: false}
	case "tool_call", string(protocol.EventTypeToolCall):
		return ToolCallMsg{
			ID:     eventID(e, "id"),
			Name:   eventText(e, "tool_name", "name", "message"),
			Params: eventParams(e),
		}
	case "tool_result", string(protocol.EventTypeToolResult):
		status := eventString(e.Data, "status")
		if status == "" {
			status = "success"
		}
		return ToolResultMsg{
			ID:     eventID(e, "id"),
			Status: status,
			Output: eventText(e, "display", "message", "text", "error"),
		}
	case "agent.task":
		// 调度agent给子agent分配任务
		return AgentTaskMsg{AgentName: e.RID, Task: e.Content, Goal: ""}
	case string(protocol.EventTypeAgentStarted), string(protocol.EventTypeAgentProgress):
		return AgentTaskMsg{
			AgentName: firstNonEmpty(strings.TrimSpace(e.RID), eventString(e.Data, "agent_name")),
			Task:      eventText(e, "task", "message", "text"),
			Goal:      eventString(e.Data, "goal"),
		}
	case "agent.final", string(protocol.EventTypeAgentDone):
		// 子agent的最终输出
		return AgentFinalMsg{
			AgentName: firstNonEmpty(strings.TrimSpace(e.RID), eventString(e.Data, "agent_name")),
			Content:   eventText(e, "text", "message"),
		}
	case "prompt.request", string(protocol.EventTypeApprovalReq), string(protocol.EventTypeInquiryReq):
		return convertPromptEvent(e)
	case "error", string(protocol.EventTypeRequestFailed), string(protocol.EventTypeAgentFailed), string(protocol.EventTypeTaskFailed):
		return AIResponseMsg{Type: "error", Content: eventText(e, "error", "message", "text"), RID: e.RID}
	default:
		return nil
	}
}

func convertPromptEvent(e bridge.Event) PromptRequestMsg {
	msg := PromptRequestMsg{
		ID: eventID(e, "approval_id", "inquiry_id"),
	}

	switch e.Type {
	case string(protocol.EventTypeApprovalReq):
		msg.Kind = "permission"
	case string(protocol.EventTypeInquiryReq):
		msg.Kind = "inquiry"
	default:
		msg.Kind = eventString(e.Data, "kind")
	}
	msg.Title = eventString(e.Data, "title")
	msg.Question = eventText(e, "question", "message", "summary", "text")
	msg.Options = eventStringSlice(e.Data, "options")
	msg.Category = eventString(e.Data, "category")
	msg.Summary = firstNonEmpty(eventString(e.Data, "summary"), eventString(e.Data, "message"))
	msg.Diff = eventString(e.Data, "diff")
	msg.DiffPath = eventString(e.Data, "diff_path")
	msg.AllowText = eventBool(e.Data, "allow_text")
	msg.TextHint = eventString(e.Data, "text_hint")
	if msg.Kind == "" {
		msg.Kind = "permission"
	}
	return msg
}

func eventID(e bridge.Event, keys ...string) string {
	for _, key := range keys {
		if v := eventString(e.Data, key); v != "" {
			return v
		}
	}
	return strings.TrimSpace(e.RID)
}

func eventText(e bridge.Event, keys ...string) string {
	if text := eventString(e.Data, keys...); text != "" {
		return text
	}
	return strings.TrimSpace(e.Content)
}

func eventParams(e bridge.Event) map[string]any {
	if e.Data == nil {
		return nil
	}
	if raw, ok := e.Data["arguments"].(map[string]any); ok && len(raw) > 0 {
		return raw
	}
	if raw, ok := e.Data["arguments"].(map[string]interface{}); ok && len(raw) > 0 {
		out := make(map[string]any, len(raw))
		for k, v := range raw {
			out[k] = v
		}
		return out
	}
	return e.Data
}

func eventString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := data[key]
		if !ok {
			continue
		}
		if text, ok := raw.(string); ok {
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func eventStringSlice(data map[string]any, key string) []string {
	if data == nil {
		return nil
	}
	raw, ok := data[key]
	if !ok {
		return nil
	}
	switch items := raw.(type) {
	case []string:
		out := make([]string, len(items))
		copy(out, items)
		return out
	case []interface{}:
		out := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			out = append(out, text)
		}
		return out
	default:
		return nil
	}
}

func eventBool(data map[string]any, key string) bool {
	if data == nil {
		return false
	}
	raw, ok := data[key]
	if !ok {
		return false
	}
	v, _ := raw.(bool)
	return v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

// 系统消息
type WindowSizeMsg struct {
	Width, Height int
}

type TickMsg struct {
	Time time.Time
}

// 用户输入消息
type KeyMsg struct {
	Key string
}

type MouseMsg struct {
	X, Y   int
	Action string
}

// AI 相关消息
type AIRequestMsg struct {
	Query  string
	Images []string
	Mode   string
}

type AIResponseMsg struct {
	Type    string // "delta", "final", "error"
	Content string
	RID     string
}

type InvokeDoneMsg struct {
	Content string
}

type ThinkingMsg struct {
	RID     string
	Content string
	Done    bool
}

// 工具消息
type ToolCallMsg struct {
	ID     string
	Name   string
	Params map[string]any
}

type ToolResultMsg struct {
	ID     string
	Status string
	Output string
}

// Agent 消息
type AgentTaskMsg struct {
	AgentName string
	Task      string
	Goal      string
}

type AgentFinalMsg struct {
	AgentName string
	Content   string
}

type PromptRequestMsg struct {
	ID        string
	Kind      string
	Title     string
	Question  string
	Options   []string
	Category  string
	Summary   string
	Diff      string
	DiffPath  string
	AllowText bool
	TextHint  string
}

type PromptResultMsg struct {
	ID          string
	Kind        string
	Decision    string
	Option      string
	OptionIndex int
	Text        string
}

// 面板消息
type PanelOpenMsg struct {
	Panel string
}

type PanelCloseMsg struct{}

type PanelActionMsg struct {
	Panel  string
	Action string
	Data   any
}

// 设置消息
type SettingsUpdateMsg struct {
	Key   string
	Value any
}

// 错误消息
type ErrorMsg struct {
	Err   error
	Fatal bool
}

// 实现 msgType 方法以满足接口要求
func (WindowSizeMsg) msgType()     {}
func (TickMsg) msgType()           {}
func (KeyMsg) msgType()            {}
func (MouseMsg) msgType()          {}
func (AIRequestMsg) msgType()      {}
func (AIResponseMsg) msgType()     {}
func (InvokeDoneMsg) msgType()     {}
func (ThinkingMsg) msgType()       {}
func (ToolCallMsg) msgType()       {}
func (ToolResultMsg) msgType()     {}
func (AgentTaskMsg) msgType()      {}
func (AgentFinalMsg) msgType()     {}
func (PromptRequestMsg) msgType()  {}
func (PromptResultMsg) msgType()   {}
func (PanelOpenMsg) msgType()      {}
func (PanelCloseMsg) msgType()     {}
func (PanelActionMsg) msgType()    {}
func (SettingsUpdateMsg) msgType() {}
func (ErrorMsg) msgType()          {}
