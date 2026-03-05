package runtime

import (
	"context"
	"strings"
	"testing"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/tools"

	"github.com/cloudwego/eino/schema"
)

func TestToolsNode_DisplayHeaderFormat(t *testing.T) {
	cm := session.NewContextManager()
	mgr := tools.NewManager()

	// 设置 ObservationConsumer 以便工具可以正常执行
	tools.ObservationConsumer = func(res tools.ToolResult) {
		// 在测试中，我们不需要验证 ObservationConsumer 的调用
		// 只需要确保它不会崩溃
	}

	rt := &EinoRuntime{ctxm: cm, tools: mgr, recentToolCalls: map[string]int{}, recentAssistantHashes: map[string]int{}}
	payload := `{"tool":"read","parameters":{"path":"."}}`
	rs, ok, _ := rt.ToolsNode(context.Background(), payload)
	if !ok || len(rs) == 0 {
		t.Fatalf("tools not executed, results: %v", rs)
	}

	// 验证：ToolsNode 应该返回 tool.call 事件（新格式）
	foundCall := false
	for _, r := range rs {
		if strings.HasPrefix(r, EventToolCall+":") && strings.Contains(r, "read") {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Fatalf("tool.call event not found in results: %v", rs)
	}

	// 注意：tool.result 事件是通过 ObservationConsumer 异步发送的
	// 实际使用中由 WithSafety 设置的 ObservationConsumer 会处理
}

func TestParseDispatchDirective_AssignExtractsFields(t *testing.T) {
	msg := schema.AssistantMessage(`{"type":"assign","role":"senior-dev","task":"do work","tools_allowed":["read","bash"]}`, nil)
	d, ok := parseDispatchDirective(msg)
	if !ok {
		t.Fatalf("expected ok")
	}
	if d.Type != "assign" || d.Role != "senior-dev" || d.Task != "do work" {
		t.Fatalf("unexpected directive: %#v", d)
	}
	if len(d.ToolsAllowed) != 2 || d.ToolsAllowed[0] != "read" || d.ToolsAllowed[1] != "bash" {
		t.Fatalf("unexpected tools_allowed: %#v", d.ToolsAllowed)
	}
}

func TestParseDispatchDirective_AssignExtractsFromFencedCode(t *testing.T) {
	msg := schema.AssistantMessage("```json\n{\"type\":\"direct_response\"}\n```", nil)
	d, ok := parseDispatchDirective(msg)
	if !ok {
		t.Fatalf("expected ok")
	}
	if d.Type != "direct_response" {
		t.Fatalf("unexpected directive: %#v", d)
	}
	if d.Role != "" || d.Task != "" || len(d.ToolsAllowed) != 0 {
		t.Fatalf("unexpected tools_allowed: %#v", d.ToolsAllowed)
	}
}

func TestIntersectAllowedTools_IntersectOnlyKeepsAllowed(t *testing.T) {
	base := map[string]bool{"read": true, "bash": true}
	got := intersectAllowedTools(base, []string{"read", "write"})
	if got["read"] != true || got["bash"] || got["write"] {
		t.Fatalf("unexpected result: %#v", got)
	}
	if base["bash"] != true || base["read"] != true {
		t.Fatalf("base mutated: %#v", base)
	}
}
