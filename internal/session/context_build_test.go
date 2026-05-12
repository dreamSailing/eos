package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"github.com/dreamSailing/eos/internal/ai"
	"strings"
	"testing"
)

func TestContextManager_BuildTrimsToBudget(t *testing.T) {
	cm := NewContextManager()
	cm.SetMaxChars(400)
	cm.AddPinned(aiMessage("system", "pinned"))

	for i := 0; i < 12; i++ {
		cm.AddUser(strings.Repeat("中文", 120))
		cm.AddAssistant(strings.Repeat("abcd", 120))
	}

	msgs := cm.Build()
	got := cm.EstimateMessageTokens(msgs)
	if got > 100 {
		t.Fatalf("expected tokens <= 100, got %d", got)
	}
}

func TestContextManager_AutoCompactUsesTokens(t *testing.T) {
	cm := NewContextManager()
	cm.SetMaxChars(600)

	for i := 0; i < 10; i++ {
		cm.AddUser(strings.Repeat("中文", 80))
		cm.AddAssistant(strings.Repeat("abcd", 160))
	}

	_ = cm.Build()

	stats := cm.GetCompressionStats()
	if stats.TotalCompressions == 0 {
		t.Fatalf("expected compressions > 0, got %#v", stats)
	}
}

func TestContextManager_BuildPackagesAuxContextWithinBudget(t *testing.T) {
	cm := NewContextManager()
	cm.SetMaxChars(1200)
	cm.AddPinned(aiMessage("system", "pinned"))
	cm.AddUser("focus on runtime injection")
	cm.AddAssistant("ready")
	cm.AddToolFull("@internal/bridge/runtime_invoke.go\n" + strings.Repeat("alpha beta gamma delta epsilon zeta eta theta\n", 120))
	cm.AddToolFull("@internal/session/context_build.go\n" + strings.Repeat("one two three four five six seven eight\n", 120))

	msgs := cm.Build()
	got := cm.EstimateMessageTokens(msgs)
	if got > cm.maxPromptTokens {
		t.Fatalf("expected tokens <= %d, got %d", cm.maxPromptTokens, got)
	}

	foundTrimmed := false
	foundSummary := false
	for _, msg := range msgs {
		if strings.Contains(msg.Content, "[trimmed to fit prompt budget]") {
			foundTrimmed = true
		}
		if strings.HasPrefix(msg.Content, "Context package: ") {
			foundSummary = true
		}
	}
	if !foundTrimmed {
		t.Fatalf("expected trimmed aux context entry in build result")
	}
	if !foundSummary {
		t.Fatalf("expected context package summary in build result")
	}
}

func aiMessage(role, content string) ai.Message {
	return ai.Message{Role: role, Content: content}
}
