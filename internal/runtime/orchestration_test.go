package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"testing"

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

