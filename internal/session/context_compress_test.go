package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func TestContextManager_AutoCompactIfNeededUpdatesStats(t *testing.T) {
	cm := NewContextManager()
	cm.SetModel("gpt-3.5")
	cm.SetCompressThreshold(0.2)

	for i := 0; i < 40; i++ {
		cm.AddUser(strings.Repeat("x", 400))
		cm.AddAssistant(strings.Repeat("y", 400))
	}

	before := cm.GetCompressionStats().TotalCompressions
	cm.AutoCompactIfNeeded()
	after := cm.GetCompressionStats().TotalCompressions
	if after <= before {
		t.Fatalf("expected compression to happen, before=%d after=%d", before, after)
	}
}

func TestContextManager_AggressiveCompactReducesTokenEstimate(t *testing.T) {
	cm := NewContextManager()
	cm.SetModel("gpt-3.5")

	for i := 0; i < 30; i++ {
		cm.AddUser(strings.Repeat("user ", 200))
		cm.AddAssistant(strings.Repeat("assistant ", 220))
	}

	before := estimatePreviewTokens(cm.BuildPreview())
	cm.AggressiveCompact(1500)
	after := estimatePreviewTokens(cm.BuildPreview())
	if after >= before {
		t.Fatalf("expected fewer tokens, before=%d after=%d", before, after)
	}
}

func estimatePreviewTokens(msgs []ai.Message) int {
	total := 0
	for _, m := range msgs {
		total += utils.EstimateTokensWeighted("", m.Content)
	}
	return total
}
