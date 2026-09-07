package ai

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"testing"

	"github.com/eosaios/eos/internal/config"
	"github.com/eosaios/eos/pkg/coreapi"
)

// applyTestCatalog 注入一个最小测试 catalog，让 o1/o1-mini/o1-preview 被识别为
// 支持推理与思考、gpt-4o 不支持。模拟生产环境内核推送目录后的状态。
// 测试结束后恢复空 catalog，避免污染其它测试。
func applyTestCatalog(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		globalRegistry = NewProviderRegistry()
		globalCatalog = NewModelCatalog()
	})
	ApplyCoreModelCatalog(coreapi.ModelCatalogState{
		Providers: []coreapi.ModelProviderOption{{
			ID:        "openai",
			Name:      "OpenAI",
			Endpoints: []coreapi.ProviderEndpoint{ep("api", "openai_chat", "https://api.openai.com/v1")},
		}},
		Presets: []coreapi.ModelPresetOption{
			{ID: "o1", Name: "o1", ProviderID: "openai", ModelName: "o1", Plan: "api", Format: "openai_chat", SupportsReasoningEffort: true, SupportsTools: true},
			{ID: "o1-mini", Name: "o1-mini", ProviderID: "openai", ModelName: "o1-mini", Plan: "api", Format: "openai_chat", SupportsReasoningEffort: true, SupportsTools: true},
			{ID: "o1-preview", Name: "o1-preview", ProviderID: "openai", ModelName: "o1-preview", Plan: "api", Format: "openai_chat", SupportsReasoningEffort: true, SupportsTools: true},
			{ID: "gpt-4o", Name: "gpt-4o", ProviderID: "openai", ModelName: "gpt-4o", Plan: "api", Format: "openai_chat", SupportsTools: true},
		},
		AllowCustomProvider: true,
		AllowCustomModel:    true,
	})
}

func TestDetectThinkingCapability(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  ThinkingCapability
	}{
		{
			name:  "OpenAI o1 prefix",
			model: "o1-custom",
			want:  ThinkingHigh,
		},
		{
			name:  "OpenAI o1-mini prefix",
			model: "o1-mini-v2",
			want:  ThinkingMedium,
		},
		{
			name:  "DeepSeek R1 prefix",
			model: "deepseek-r1-local",
			want:  ThinkingHigh,
		},
		{
			name:  "Qwen thinking prefix",
			model: "qwen-thinking-max",
			want:  ThinkingMedium,
		},
		{
			name:  "Claude sonnet (no thinking)",
			model: "claude-3-5-sonnet",
			want:  ThinkingNone,
		},
		{
			name:  "GPT prefix (no thinking)",
			model: "gpt-4-turbo",
			want:  ThinkingNone,
		},
		{
			name:  "Unknown model",
			model: "unknown-xyz",
			want:  ThinkingNone,
		},
		{
			name:  "Empty string",
			model: "",
			want:  ThinkingNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectThinkingCapability(tt.model)
			if got != tt.want {
				t.Errorf("DetectThinkingCapability(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestShouldEnableThinkingForModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		cfg   *config.Config
		want  bool
	}{
		{
			name:  "Known thinking model with empty config",
			model: "o1",
			cfg:   &config.Config{},
			want:  false, // Empty config means disabled
		},
		{
			name:  "Known thinking model with enabled config",
			model: "o1",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled: true,
				},
			},
			want: true,
		},
		{
			name:  "Non-thinking model with enabled config",
			model: "gpt-4o",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled:    true,
					AutoDetect: true,
				},
			},
			want: false, // gpt-4o doesn't support thinking
		},
		{
			name:  "Unknown thinking model with auto-detect enabled",
			model: "o1-custom-v2",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled:    true,
					AutoDetect: true,
				},
			},
			want: true, // auto-detect should catch o1 prefix
		},
		{
			name:  "Disabled thinking config",
			model: "o1",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled: false,
				},
			},
			want: false,
		},
		{
			name:  "Custom model in whitelist",
			model: "custom-thinking-model",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled:      true,
					CustomModels: []string{"custom-thinking-model"},
				},
			},
			want: true,
		},
		{
			name:  "DeepSeek R1 with auto-detect",
			model: "deepseek-r1-32b",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled:    true,
					AutoDetect: true,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyTestCatalog(t)
			got := ShouldEnableThinkingForModel(tt.model, tt.cfg)
			if got != tt.want {
				t.Errorf("ShouldEnableThinkingForModel(%q, cfg) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestGetReasoningEffortLevel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		cfg   *config.Config
		want  string
	}{
		{
			name:  "Empty config returns empty",
			model: "o1",
			cfg:   &config.Config{},
			want:  "",
		},
		{
			name:  "Config with medium level",
			model: "o1",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled:         true,
					ReasoningEffort: "medium",
				},
			},
			want: "medium",
		},
		{
			name:  "Config with high level",
			model: "o1-mini",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled:         true,
					ReasoningEffort: "high",
				},
			},
			want: "high",
		},
		{
			name:  "Config with low level",
			model: "o1-preview",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled:         true,
					ReasoningEffort: "low",
				},
			},
			want: "low",
		},
		{
			name:  "Invalid level defaults to medium",
			model: "o1",
			cfg: &config.Config{
				Thinking: config.ThinkingConfig{
					Enabled:         true,
					ReasoningEffort: "invalid",
				},
			},
			want: "medium", // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyTestCatalog(t)
			got := GetReasoningEffortLevel(tt.model, tt.cfg)
			if got != tt.want {
				t.Errorf("GetReasoningEffortLevel(%q, cfg) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

// TestModelOverride tests that model-level config overrides global config
func TestModelOverride(t *testing.T) {
	cfg := &config.Config{
		Models: []config.ModelEntry{
			{
				Name:                    "o1-mini",
				ThinkingEnabled:         false,
				SupportsReasoningEffort: true,
			},
		},
		Thinking: config.ThinkingConfig{
			Enabled:    true,
			AutoDetect: true,
		},
	}

	// Model-level override should disable thinking even when global is enabled
	got := ShouldEnableThinkingForModel("o1-mini", cfg)
	if got {
		t.Errorf("ShouldEnableThinkingForModel(o1-mini) with model-level override = true, want false")
	}
}
