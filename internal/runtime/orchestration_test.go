package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/session"
	"github.com/cloudwego/eino/schema"
)

func TestShouldBypassArchitect_SimpleChangeWithQuestionMark(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("能不能修复一下 warning? 现在有 lint 报错"),
	}
	if !shouldBypassArchitect(msgs) {
		t.Fatalf("expected bypass for code change with question mark")
	}
}

func TestShouldBypassArchitect_QuestionOnly(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("什么是 ContextManager？"),
	}
	if shouldBypassArchitect(msgs) {
		t.Fatalf("expected not bypass for question-only")
	}
}

func TestShouldBypassArchitect_PlanPreferenceFirst(t *testing.T) {
	msgs := []*schema.Message{
		schema.SystemMessage("PLAN_PREFERENCE: prefer_plan_first"),
		schema.UserMessage("修复一个小 bug"),
	}
	if shouldBypassArchitect(msgs) {
		t.Fatalf("expected not bypass when plan preference first")
	}
}

func TestNormalizeDispatchHistory_FoldsLeadingSystemMessagesIntoPrompt(t *testing.T) {
	systemPrompt, history := normalizeDispatchHistory("BASE_PROMPT", []*schema.Message{
		schema.SystemMessage("DOC:EOS.md\ncontent"),
		schema.SystemMessage("DOC:.eos/Rules.md\nrules"),
		schema.UserMessage("你好"),
	})

	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].Role != schema.User || strings.TrimSpace(history[0].Content) != "你好" {
		t.Fatalf("unexpected history[0]: role=%v content=%q", history[0].Role, history[0].Content)
	}
	if !strings.Contains(systemPrompt, "BASE_PROMPT") {
		t.Fatalf("system prompt missing base prompt: %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "## 前置上下文") {
		t.Fatalf("system prompt missing leading context section: %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "DOC:EOS.md\ncontent") || !strings.Contains(systemPrompt, "DOC:.eos/Rules.md\nrules") {
		t.Fatalf("system prompt missing folded system content: %q", systemPrompt)
	}
}

func TestNormalizeDispatchHistory_FoldsTrailingSystemMessagesIntoPrompt(t *testing.T) {
	systemPrompt, history := normalizeDispatchHistory("BASE_PROMPT", []*schema.Message{
		schema.UserMessage("你好"),
		schema.SystemMessage("STOP_HOOK: retry"),
	})

	if !strings.Contains(systemPrompt, "BASE_PROMPT") {
		t.Fatalf("system prompt missing base prompt: %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "STOP_HOOK: retry") {
		t.Fatalf("system prompt missing folded trailing system content: %q", systemPrompt)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].Role != schema.User || strings.TrimSpace(history[0].Content) != "你好" {
		t.Fatalf("unexpected remaining history: role=%v content=%q", history[0].Role, history[0].Content)
	}
}

func TestBindPlanUpdatesForwardsPlanAndEmitsReady(t *testing.T) {
	cm := session.NewContextManager()
	rt := &EinoRuntime{}

	var gotPlan string
	var gotMeta string
	rt.WithOnPlanUpdate(func(plan string) {
		gotPlan = strings.TrimSpace(plan)
	})
	rt.WithOnMeta(func(meta string) {
		gotMeta = strings.TrimSpace(meta)
	})

	rt.bindPlanUpdates(cm)
	cm.SetLastPlan("# Test Plan\n\n- item")

	if gotPlan != "# Test Plan\n\n- item" {
		t.Fatalf("got plan = %q", gotPlan)
	}
	if gotMeta != EventPlanReady {
		t.Fatalf("got meta = %q, want %q", gotMeta, EventPlanReady)
	}
}

func TestBuildAutomaticVerificationTaskIncludesGuardrailAndContext(t *testing.T) {
	task := buildAutomaticVerificationTask("修复附件文件名截断", "已调整附件样式并补充悬停文件名显示")

	if !strings.Contains(task, "不要被 80% 的成功欺骗") {
		t.Fatalf("task missing verification guardrail: %q", task)
	}
	if !strings.Contains(task, "原始需求: 修复附件文件名截断") {
		t.Fatalf("task missing original query: %q", task)
	}
	if !strings.Contains(task, "实现结果摘要: 已调整附件样式并补充悬停文件名显示") {
		t.Fatalf("task missing implementation summary: %q", task)
	}
	if !strings.Contains(task, "VERDICT: PASS|FAIL|PARTIAL") {
		t.Fatalf("task missing verdict requirement: %q", task)
	}
	for _, want := range []string{"验收摘要", "覆盖到的验证项", "未覆盖的风险和空白", "关键证据"} {
		if !strings.Contains(task, want) {
			t.Fatalf("task missing structured heading %q: %q", want, task)
		}
	}
}

func TestExtractLastAssistantTextReturnsLatestAssistantContent(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("先实现"),
		schema.AssistantMessage("第一版结果", nil),
		schema.SystemMessage("HOOK"),
		schema.AssistantMessage("最终实现摘要", nil),
	}

	got := extractLastAssistantText(msgs)
	if got != "最终实现摘要" {
		t.Fatalf("got %q, want %q", got, "最终实现摘要")
	}
}
