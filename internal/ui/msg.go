package ui

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"encoding/json"
	"strings"
	"time"

	uiadapter "github.com/dreamSailing/eos/internal/ui/adapter"
	"github.com/dreamSailing/eos/internal/update"
	"github.com/dreamSailing/eos/pkg/protocol"
)

// Msg 是所有消息类型的接口
type Msg interface {
	msgType()
}

// ConvertEvent 将 runtime event 转换为 UI Msg
func ConvertEvent(e uiadapter.RuntimeEvent) Msg {
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
		// 注意：NormalizeEventType 把 turn.tool_call_start 与 turn.tool_call_done
		// 都归一化成 tool.call，原始类型保留在 Data["original_event_type"]。
		// Rust runtime 在 ToolCallDone 触发前会累积所有 arguments_delta，
		// 所以 done 事件携带完整 arguments（见 eos-core-runtime/src/lib.rs）。
		// 因此无需在 TUI 逐事件累积 delta：start 先建空参卡片，done 到达时
		// handleToolCall 会用真实参数补全同一张卡片。
		return ToolCallMsg{
			ID:     eventID(e, "id"),
			Name:   eventText(e, "tool_name", "name", "tool", "message"),
			Params: eventParams(e),
		}
	case "tool_result", string(protocol.EventTypeToolResult):
		status := eventString(e.Data, "status")
		if status == "" {
			status = "success"
		}
		return ToolResultMsg{
			ID:     eventID(e, "id", "request_id"),
			Status: status,
			Output: eventText(e, "display", "message", "text", "error"),
		}
	case "agent.task":
		// 调度agent给子agent分配任务
		sourceName, sourceID := eventAgentSource(e.Data)
		return AgentTaskMsg{
			AgentName:       firstNonEmpty(strings.TrimSpace(e.RID), eventString(e.Data, "agent_name")),
			AgentID:         eventString(e.Data, "agent_id"),
			SourceAgentName: sourceName,
			SourceAgentID:   sourceID,
			Event:           "dispatch",
			Task:            eventText(e, "task", "message", "text"),
			Goal:            eventString(e.Data, "goal"),
		}
	case string(protocol.EventTypeAgentStarted), string(protocol.EventTypeAgentProgress):
		sourceName, sourceID := eventAgentSource(e.Data)
		return AgentTaskMsg{
			AgentName:       firstNonEmpty(strings.TrimSpace(e.RID), eventString(e.Data, "agent_name")),
			AgentID:         eventString(e.Data, "agent_id"),
			SourceAgentName: sourceName,
			SourceAgentID:   sourceID,
			Event:           agentEventKind(e.Type),
			Task:            eventText(e, "task", "message", "text"),
			Goal:            eventString(e.Data, "goal"),
		}
	case "agent.final", string(protocol.EventTypeAgentDone):
		// 子agent的最终输出
		sourceName, sourceID := eventAgentSource(e.Data)
		return AgentFinalMsg{
			AgentName:       firstNonEmpty(strings.TrimSpace(e.RID), eventString(e.Data, "agent_name")),
			AgentID:         eventString(e.Data, "agent_id"),
			SourceAgentName: sourceName,
			SourceAgentID:   sourceID,
			Event:           "result",
			Content:         eventText(e, "text", "message"),
		}
	case "prompt.request", string(protocol.EventTypeApprovalReq), string(protocol.EventTypeInquiryReq):
		return convertPromptEvent(e)
	case string(protocol.EventTypeModeChanged):
		return ModeChangedMsg{Mode: eventString(e.Data, "new_mode"), PreviousMode: eventString(e.Data, "old_mode")}
	case string(protocol.EventTypeAgentFailed), string(protocol.EventTypeAgentCancelled):
		sourceName, sourceID := eventAgentSource(e.Data)
		return AgentFinalMsg{
			AgentName:       firstNonEmpty(strings.TrimSpace(e.RID), eventString(e.Data, "agent_name")),
			AgentID:         eventString(e.Data, "agent_id"),
			SourceAgentName: sourceName,
			SourceAgentID:   sourceID,
			Event:           agentEventKind(e.Type),
			Content:         eventText(e, "error", "reason", "message", "text"),
		}
	case "error", string(protocol.EventTypeRequestFailed), string(protocol.EventTypeTaskFailed):
		return AIResponseMsg{Type: "error", Content: eventText(e, "error", "message", "text"), RID: e.RID}
	default:
		return nil
	}
}

func convertPromptEvent(e uiadapter.RuntimeEvent) PromptRequestMsg {
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

func eventID(e uiadapter.RuntimeEvent, keys ...string) string {
	for _, key := range keys {
		if v := eventString(e.Data, key); v != "" {
			return v
		}
	}
	return strings.TrimSpace(e.RID)
}

func eventText(e uiadapter.RuntimeEvent, keys ...string) string {
	if text := eventString(e.Data, keys...); text != "" {
		return text
	}
	return strings.TrimSpace(e.Content)
}

func eventParams(e uiadapter.RuntimeEvent) map[string]any {
	if e.Data == nil {
		return nil
	}
	// 按 arguments -> input/parameters/params 的顺序解析工具参数。
	// 绝不能在找不到参数时把整个信封 Data 当参数返回——那会让 UI 把
	// event_id/session_id/turn_id 等元数据渲染成工具参数。
	for _, key := range []string{"arguments", "input", "parameters", "params"} {
		raw, ok := e.Data[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case map[string]any:
			if len(v) > 0 {
				return v
			}
		case string:
			// JSON 字符串是多数模型适配器的承载形态。
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			var out map[string]any
			if err := json.Unmarshal([]byte(v), &out); err == nil && len(out) > 0 {
				return out
			}
			// 非 JSON 字符串：忽略，不回退到信封。
		}
	}
	return nil
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

func agentEventKind(eventType string) string {
	switch strings.TrimSpace(eventType) {
	case "agent.task", string(protocol.EventTypeAgentStarted):
		return "dispatch"
	case string(protocol.EventTypeAgentProgress):
		return "update"
	case "agent.final", string(protocol.EventTypeAgentDone):
		return "result"
	case string(protocol.EventTypeAgentFailed):
		return "failed"
	case string(protocol.EventTypeAgentCancelled):
		return "cancelled"
	default:
		return ""
	}
}

func eventAgentSource(data map[string]any) (string, string) {
	name := firstNonEmpty(
		eventString(data, "source_agent_name"),
		eventString(data, "caller_agent_name"),
		eventString(data, "parent_agent_name"),
	)
	id := firstNonEmpty(
		eventString(data, "source_agent_id"),
		eventString(data, "caller_agent_id"),
		eventString(data, "parent_agent_id"),
	)
	if name == "" {
		name = "assistant"
	}
	return name, id
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

type PredictionUpdateMsg struct {
	Text  string
	Seq   int
	Draft string
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
	AgentName       string
	AgentID         string
	SourceAgentName string
	SourceAgentID   string
	Event           string
	Task            string
	Goal            string
}

type AgentFinalMsg struct {
	AgentName       string
	AgentID         string
	SourceAgentName string
	SourceAgentID   string
	Event           string
	Content         string
}

type ModeChangedMsg struct {
	Mode         string
	PreviousMode string
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

// VersionCheckMsg 版本检查结果
type VersionCheckMsg struct {
	Result *update.CheckResult
}

// 实现 msgType 方法以满足接口要求
func (WindowSizeMsg) msgType()       {}
func (TickMsg) msgType()             {}
func (KeyMsg) msgType()              {}
func (MouseMsg) msgType()            {}
func (AIRequestMsg) msgType()        {}
func (AIResponseMsg) msgType()       {}
func (InvokeDoneMsg) msgType()       {}
func (PredictionUpdateMsg) msgType() {}
func (ThinkingMsg) msgType()         {}
func (ToolCallMsg) msgType()         {}
func (ToolResultMsg) msgType()       {}
func (AgentTaskMsg) msgType()        {}
func (AgentFinalMsg) msgType()       {}
func (ModeChangedMsg) msgType()      {}
func (PromptRequestMsg) msgType()    {}
func (PromptResultMsg) msgType()     {}
func (PanelOpenMsg) msgType()        {}
func (PanelCloseMsg) msgType()       {}
func (PanelActionMsg) msgType()      {}
func (SettingsUpdateMsg) msgType()   {}
func (VersionCheckMsg) msgType()     {}
func (ErrorMsg) msgType()            {}
