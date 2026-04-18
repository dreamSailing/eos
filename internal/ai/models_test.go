package ai

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"testing"
)

func TestGetModelInfo(t *testing.T) {
	tests := []struct {
		name          string
		modelName     string
		wantFound     bool
		wantThinking  ThinkingCapability
		wantReasoning bool
	}{
		{
			name:          "OpenAI o1 model",
			modelName:     "o1",
			wantFound:     true,
			wantThinking:  ThinkingHigh,
			wantReasoning: true,
		},
		{
			name:          "OpenAI o1-mini model",
			modelName:     "o1-mini",
			wantFound:     true,
			wantThinking:  ThinkingMedium,
			wantReasoning: true,
		},
		{
			name:          "OpenAI o1-preview with alias",
			modelName:     "o1-preview-2024-09-12",
			wantFound:     true,
			wantThinking:  ThinkingMedium,
			wantReasoning: true,
		},
		{
			name:          "GPT-4o (non-thinking)",
			modelName:     "gpt-4o",
			wantFound:     true,
			wantThinking:  ThinkingNone,
			wantReasoning: false,
		},
		{
			name:          "Unknown model",
			modelName:     "unknown-model",
			wantFound:     false,
			wantThinking:  ThinkingNone,
			wantReasoning: false,
		},
		{
			name:          "Empty string",
			modelName:     "",
			wantFound:     false,
			wantThinking:  ThinkingNone,
			wantReasoning: false,
		},
		{
			name:          "DeepSeek R1",
			modelName:     "deepseek-r1",
			wantFound:     true,
			wantThinking:  ThinkingHigh,
			wantReasoning: false, // DeepSeek R1 has thinking but not ReasoningEffort API
		},
		{
			name:          "Qwen Thinking",
			modelName:     "qwen3.6-plus",
			wantFound:     true,
			wantThinking:  ThinkingHigh,
			wantReasoning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, found := GetModelInfo(tt.modelName)
			if found != tt.wantFound {
				t.Errorf("GetModelInfo(%q) found = %v, want %v", tt.modelName, found, tt.wantFound)
			}
			if found && info.Thinking != tt.wantThinking {
				t.Errorf("GetModelInfo(%q) Thinking = %v, want %v", tt.modelName, info.Thinking, tt.wantThinking)
			}
			if found && info.SupportsReasoningEffort != tt.wantReasoning {
				t.Errorf("GetModelInfo(%q) SupportsReasoningEffort = %v, want %v",
					tt.modelName, info.SupportsReasoningEffort, tt.wantReasoning)
			}
		})
	}
}

func TestSupportsReasoningEffort(t *testing.T) {
	tests := []struct {
		modelName string
		want      bool
	}{
		{"o1", true},
		{"o1-mini", true},
		{"o1-preview", true},
		{"o1-2024-12-17", true}, // alias
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"deepseek-r1", false},
		{"qwen3.6-plus", false},
		{"unknown-model", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			got := SupportsReasoningEffort(tt.modelName)
			if got != tt.want {
				t.Errorf("SupportsReasoningEffort(%q) = %v, want %v", tt.modelName, got, tt.want)
			}
		})
	}
}

func TestSupportsThinking(t *testing.T) {
	tests := []struct {
		modelName string
		want      bool
	}{
		{"o1", true},
		{"o1-mini", true},
		{"o1-preview", true},
		{"deepseek-r1", true},
		{"qwen3.6-plus", true},
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"unknown-model", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			got := SupportsThinking(tt.modelName)
			if got != tt.want {
				t.Errorf("SupportsThinking(%q) = %v, want %v", tt.modelName, got, tt.want)
			}
		})
	}
}

func TestThinkingCapabilityString(t *testing.T) {
	tests := []struct {
		capability ThinkingCapability
		want       string
	}{
		{ThinkingNone, "none"},
		{ThinkingLow, "low"},
		{ThinkingMedium, "medium"},
		{ThinkingHigh, "high"},
		{ThinkingCapability(99), "none"}, // Unknown values default to "none"
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.capability.String()
			if got != tt.want {
				t.Errorf("ThinkingCapability(%d).String() = %q, want %q", tt.capability, got, tt.want)
			}
		})
	}
}

// TestModelInfoAliases tests that model aliases work correctly
func TestModelInfoAliases(t *testing.T) {
	// Test that o1-2024-12-17 maps to o1
	info1, found1 := GetModelInfo("o1")
	info2, found2 := GetModelInfo("o1-2024-12-17")

	if !found1 || !found2 {
		t.Fatal("Both o1 and o1-2024-12-17 should be found")
	}

	if info1.Thinking != info2.Thinking {
		t.Errorf("o1 and o1-2024-12-17 should have same Thinking capability: %v vs %v",
			info1.Thinking, info2.Thinking)
	}

	if info1.SupportsReasoningEffort != info2.SupportsReasoningEffort {
		t.Errorf("o1 and o1-2024-12-17 should have same SupportsReasoningEffort: %v vs %v",
			info1.SupportsReasoningEffort, info2.SupportsReasoningEffort)
	}
}
