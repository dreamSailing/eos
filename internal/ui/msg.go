package ui

import (
	"time"
	"github.com/dreamSailing/vb-coding/internal/bridge"
)

// Msg 是所有消息类型的接口
type Msg interface {
    msgType()
}

// ConvertEvent 将 bridge.Event 转换为 UI Msg
func ConvertEvent(e bridge.Event) Msg {
	switch e.Type {
	case "meta":
		return AIResponseMsg{Type: "delta", Content: e.Content, RID: e.RID}
	case "delta":
		return AIResponseMsg{Type: "delta", Content: e.Content, RID: e.RID}
	case "reasoning":
		return ThinkingMsg{RID: e.RID, Content: e.Content, Done: false}
	case "tool_call":
		return ToolCallMsg{ID: e.RID, Name: e.Content, Params: e.Data}
	case "tool_result":
		status := "success"
		if e.Data != nil {
			if s, ok := e.Data["status"].(string); ok && s != "" {
				status = s
			}
		}
		return ToolResultMsg{ID: e.RID, Status: status, Output: e.Content}
	case "tool.call":
		return ToolCallMsg{ID: e.RID, Name: e.Content, Params: e.Data}
	case "tool.result":
		status := "success"
		if e.Data != nil {
			if s, ok := e.Data["status"].(string); ok && s != "" {
				status = s
			}
		}
		return ToolResultMsg{ID: e.RID, Status: status, Output: e.Content}
	case "phase.note":
		return ThinkingMsg{RID: e.RID, Content: e.Content, Done: false}
	case "agent.task":
		// 调度agent给子agent分配任务
		return AgentTaskMsg{AgentName: e.RID, Task: e.Content, Goal: ""}
	case "agent.final":
		// 子agent的最终输出
		return AgentFinalMsg{AgentName: e.RID, Content: e.Content}
	case "prompt.request":
		msg := PromptRequestMsg{ID: e.RID}
		if e.Data != nil {
			if v, ok := e.Data["kind"].(string); ok {
				msg.Kind = v
			}
			if v, ok := e.Data["title"].(string); ok {
				msg.Title = v
			}
			if v, ok := e.Data["question"].(string); ok {
				msg.Question = v
			} else {
				msg.Question = e.Content
			}
			if v, ok := e.Data["category"].(string); ok {
				msg.Category = v
			}
			if v, ok := e.Data["summary"].(string); ok {
				msg.Summary = v
			}
			if v, ok := e.Data["diff"].(string); ok {
				msg.Diff = v
			}
			if v, ok := e.Data["diff_path"].(string); ok {
				msg.DiffPath = v
			}
			if v, ok := e.Data["allow_text"].(bool); ok {
				msg.AllowText = v
			}
			if v, ok := e.Data["text_hint"].(string); ok {
				msg.TextHint = v
			}
			if raw, ok := e.Data["options"].([]any); ok {
				for _, it := range raw {
					if s, ok := it.(string); ok {
						msg.Options = append(msg.Options, s)
					}
				}
			} else if raw, ok := e.Data["options"].([]string); ok {
				msg.Options = append(msg.Options, raw...)
			}
		}
		if msg.Question == "" {
			msg.Question = e.Content
		}
		return msg
	default:
		return nil
	}
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
    X, Y int
    Action string
}

// AI 相关消息
type AIRequestMsg struct {
    Query     string
    Images    []string
    Mode      string
}

type AIResponseMsg struct {
    Type    string  // "delta", "final", "error"
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
func (WindowSizeMsg) msgType() {}
func (TickMsg) msgType() {}
func (KeyMsg) msgType() {}
func (MouseMsg) msgType() {}
func (AIRequestMsg) msgType() {}
func (AIResponseMsg) msgType() {}
func (InvokeDoneMsg) msgType() {}
func (ThinkingMsg) msgType() {}
func (ToolCallMsg) msgType() {}
func (ToolResultMsg) msgType() {}
func (AgentTaskMsg) msgType() {}
func (AgentFinalMsg) msgType() {}
func (PromptRequestMsg) msgType() {}
func (PromptResultMsg) msgType() {}
func (PanelOpenMsg) msgType() {}
func (PanelCloseMsg) msgType() {}
func (PanelActionMsg) msgType() {}
func (SettingsUpdateMsg) msgType() {}
func (ErrorMsg) msgType() {}
