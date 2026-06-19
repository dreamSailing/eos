package ai

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"testing"

	"github.com/dreamSailing/eos/pkg/coreapi"
)

func TestResolveProviderAndModelPrefersBaseSpecificPreset(t *testing.T) {
	seedProviderResolverCatalog(t)

	tests := []struct {
		name         string
		baseURL      string
		modelName    string
		wantType     APIType
		wantProvider ProviderType
	}{
		{
			name:         "zhipu claude endpoint",
			baseURL:      "https://open.bigmodel.cn/api/anthropic",
			modelName:    "glm-5",
			wantType:     APITypeClaude,
			wantProvider: ProviderZhipu,
		},
		{
			name:         "zhipu coding endpoint",
			baseURL:      "https://open.bigmodel.cn/api/coding/paas/v4",
			modelName:    "glm-5",
			wantType:     APITypeCodePlan,
			wantProvider: ProviderZhipu,
		},
		{
			name:         "dashscope coding endpoint",
			baseURL:      "https://coding.dashscope.aliyuncs.com/v1",
			modelName:    "qwen3.6-plus",
			wantType:     APITypeCodePlan,
			wantProvider: ProviderDashScope,
		},
		{
			name:         "minimax claude endpoint",
			baseURL:      "https://api.minimaxi.com/anthropic/v1",
			modelName:    "MiniMax-M2.7",
			wantType:     APITypeClaude,
			wantProvider: ProviderMiniMax,
		},
		{
			name:         "mimo claude endpoint",
			baseURL:      "https://token-plan-cn.xiaomimimo.com/anthropic",
			modelName:    "mimo-v2.5-pro",
			wantType:     APITypeClaude,
			wantProvider: ProviderMiMo,
		},
		{
			name:         "mimo openai endpoint",
			baseURL:      "https://token-plan-cn.xiaomimimo.com/v1",
			modelName:    "mimo-v2.5",
			wantType:     APITypeCodePlan,
			wantProvider: ProviderMiMo,
		},
		{
			name:         "gemini openai endpoint",
			baseURL:      "https://generativelanguage.googleapis.com/v1beta/openai",
			modelName:    "gemini-3.1-pro-preview",
			wantType:     APITypeStandard,
			wantProvider: ProviderGemini,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ResolveProviderAndModel(tt.baseURL, tt.modelName)
			if info == nil || info.Model == nil {
				t.Fatalf("ResolveProviderAndModel(%q, %q) returned nil model", tt.baseURL, tt.modelName)
			}
			if info.ProviderType != tt.wantProvider {
				t.Fatalf("provider = %q, want %q", info.ProviderType, tt.wantProvider)
			}
			if info.Model.APIType != tt.wantType {
				t.Fatalf("apiType = %q, want %q", info.Model.APIType, tt.wantType)
			}
		})
	}
}

func TestGetModelContextWindowResolvesOfficialModelNames(t *testing.T) {
	seedProviderResolverCatalog(t)

	tests := []struct {
		model string
		want  int
	}{
		{model: "qwen3.6-plus", want: 1000000},
		{model: "qwen3.6-max-preview", want: 262144},
		{model: "deepseek-v4-pro", want: 1000000},
		{model: "glm-5", want: 200000},
		{model: "kimi-k2.5", want: 256000},
		{model: "MiniMax-M2.7", want: 204800},
		{model: "mimo-v2.5-pro", want: 1000000},
		{model: "gemini-3.1-pro-preview", want: 1048576},
		{model: "gpt-5.5", want: 1050000},
		{model: "gpt-5-codex", want: 400000},
		{model: "claude-sonnet-4-6", want: 1000000},
		{model: "claude-haiku-4-5", want: 200000},
	}

	for _, tt := range tests {
		if got := GetModelContextWindow(tt.model); got != tt.want {
			t.Fatalf("GetModelContextWindow(%q) = %d, want %d", tt.model, got, tt.want)
		}
	}
}

func seedProviderResolverCatalog(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		ApplyCoreModelCatalog(coreapi.ModelCatalogState{})
		globalResolver = NewResolver()
	})

// ep is a shorthand for creating ProviderEndpoint literals in tests.
func ep(plan, format, base string) coreapi.ProviderEndpoint {
	return coreapi.ProviderEndpoint{Plan: plan, Format: format, APIBase: base}
}

func setupCatalogFixture(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		ApplyCoreModelCatalog(coreapi.ModelCatalogState{})
		globalResolver = NewResolver()
	})

	ApplyCoreModelCatalog(coreapi.ModelCatalogState{
		Providers: []coreapi.ModelProviderOption{
			{ID: "deepseek", Name: "DeepSeek", Endpoints: []coreapi.ProviderEndpoint{
				ep("api", "openai_chat", "https://api.deepseek.com"),
			}, DefaultModels: []string{"deepseek-v4-pro"}},
			{ID: "dashscope", Name: "阿里云通义", Endpoints: []coreapi.ProviderEndpoint{
				ep("api", "openai_chat", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
				ep("code", "openai_chat", "https://coding.dashscope.aliyuncs.com/v1"),
			}, DefaultModels: []string{"qwen3.6-plus", "dashscope-coding-plan-qwen3.6-plus"}},
			{ID: "zhipu", Name: "智谱 GLM", Endpoints: []coreapi.ProviderEndpoint{
				ep("api", "openai_chat", "https://open.bigmodel.cn/api/paas/v4"),
				ep("code", "openai_chat", "https://open.bigmodel.cn/api/coding/paas/v4"),
				ep("code", "anthropic", "https://open.bigmodel.cn/api/anthropic"),
			}, DefaultModels: []string{"glm-5", "zhipu-coding-plan-openai", "zhipu-coding-plan-claude"}},
			{ID: "moonshot", Name: "Moonshot", Endpoints: []coreapi.ProviderEndpoint{
				ep("api", "openai_chat", "https://api.moonshot.cn/v1"),
			}, DefaultModels: []string{"kimi-k2.6", "kimi-k2.5"}},
			{ID: "minimax", Name: "MiniMax", Endpoints: []coreapi.ProviderEndpoint{
				ep("api", "openai_chat", "https://api.minimaxi.com/v1"),
				ep("code", "openai_chat", "https://api.minimaxi.com/v1"),
				ep("code", "anthropic", "https://api.minimaxi.com/anthropic/v1"),
			}, DefaultModels: []string{"minimax-token-plan-openai", "minimax-token-plan-claude"}},
			{ID: "mimo", Name: "小米 MiMo", Endpoints: []coreapi.ProviderEndpoint{
				ep("token", "openai_chat", "https://token-plan-cn.xiaomimimo.com/v1"),
				ep("token", "anthropic", "https://token-plan-cn.xiaomimimo.com/anthropic"),
			}, DefaultModels: []string{"mimo-token-plan-openai-pro", "mimo-token-plan-openai-omni", "mimo-token-plan-claude-pro"}},
			{ID: "gemini", Name: "Google Gemini", Endpoints: []coreapi.ProviderEndpoint{
				ep("api", "openai_chat", "https://generativelanguage.googleapis.com/v1beta/openai"),
			}, DefaultModels: []string{"gemini-3.1-pro-preview"}},
			{ID: "openai", Name: "OpenAI", Endpoints: []coreapi.ProviderEndpoint{
				ep("api", "openai_chat", "https://api.openai.com/v1"),
				ep("api", "openai_responses", "https://api.openai.com/v1"),
			}, DefaultModels: []string{"gpt-5.5", "gpt-5-codex"}},
			{ID: "anthropic", Name: "Anthropic", Endpoints: []coreapi.ProviderEndpoint{
				ep("api", "anthropic", "https://api.anthropic.com/v1"),
			}, DefaultModels: []string{"claude-sonnet-4-6", "claude-haiku-4-5"}},
		},
		Presets: []coreapi.ModelPresetOption{
			{ID: "deepseek-v4-pro", ProviderID: "deepseek", ModelName: "deepseek-v4-pro", Plan: "api", Format: "openai_chat", ContextWindow: 1000000, SupportsTools: true},
			{ID: "qwen3.6-plus", ProviderID: "dashscope", ModelName: "qwen3.6-plus", Plan: "api", Format: "openai_chat", ContextWindow: 1000000, SupportsVision: true, SupportsTools: true},
			{ID: "qwen3.6-max-preview", ProviderID: "dashscope", ModelName: "qwen3.6-max-preview", Plan: "api", Format: "openai_chat", ContextWindow: 262144, SupportsTools: true},
			{ID: "dashscope-coding-plan-qwen3.6-plus", ProviderID: "dashscope", ModelName: "qwen3.6-plus", Plan: "code", Format: "openai_chat", ContextWindow: 1000000, SupportsVision: true, SupportsTools: true},
			{ID: "glm-5", ProviderID: "zhipu", ModelName: "glm-5", Plan: "api", Format: "openai_chat", ContextWindow: 200000, SupportsTools: true},
			{ID: "zhipu-coding-plan-openai", ProviderID: "zhipu", ModelName: "glm-5", Plan: "code", Format: "openai_chat", ContextWindow: 200000, SupportsTools: true},
			{ID: "zhipu-coding-plan-claude", ProviderID: "zhipu", ModelName: "glm-5", Plan: "code", Format: "anthropic", ContextWindow: 200000, SupportsTools: true},
			{ID: "kimi-k2.6", ProviderID: "moonshot", ModelName: "kimi-k2.6", Plan: "api", Format: "openai_chat", ContextWindow: 0, SupportsTools: true},
			{ID: "kimi-k2.5", ProviderID: "moonshot", ModelName: "kimi-k2.5", Plan: "api", Format: "openai_chat", ContextWindow: 256000, SupportsVision: true, SupportsTools: true},
			{ID: "minimax-token-plan-openai", ProviderID: "minimax", ModelName: "MiniMax-M2.7", Plan: "code", Format: "openai_chat", ContextWindow: 204800, SupportsVision: true, SupportsTools: true},
			{ID: "minimax-token-plan-claude", ProviderID: "minimax", ModelName: "MiniMax-M2.7", Plan: "code", Format: "anthropic", ContextWindow: 204800, SupportsVision: true, SupportsTools: true},
			{ID: "mimo-token-plan-openai-pro", ProviderID: "mimo", ModelName: "mimo-v2.5-pro", Plan: "token", Format: "openai_chat", ContextWindow: 1000000, SupportsTools: true},
			{ID: "mimo-token-plan-openai-omni", ProviderID: "mimo", ModelName: "mimo-v2.5", Plan: "token", Format: "openai_chat", ContextWindow: 1000000, SupportsVision: true, SupportsTools: true},
			{ID: "mimo-token-plan-claude-pro", ProviderID: "mimo", ModelName: "mimo-v2.5-pro", Plan: "token", Format: "anthropic", ContextWindow: 1000000, SupportsTools: true},
			{ID: "gemini-3.1-pro-preview", ProviderID: "gemini", ModelName: "gemini-3.1-pro-preview", Plan: "api", Format: "openai_chat", ContextWindow: 1048576, SupportsVision: true, SupportsTools: true},
			{ID: "gpt-5.5", ProviderID: "openai", ModelName: "gpt-5.5", Plan: "api", Format: "openai_chat", ContextWindow: 1050000, SupportsTools: true},
			{ID: "gpt-5-codex", ProviderID: "openai", ModelName: "gpt-5-codex", Plan: "api", Format: "openai_chat", ContextWindow: 400000, SupportsTools: true},
			{ID: "claude-sonnet-4-6", ProviderID: "anthropic", ModelName: "claude-sonnet-4-6", Plan: "api", Format: "anthropic", ContextWindow: 1000000, SupportsTools: true},
			{ID: "claude-haiku-4-5", ProviderID: "anthropic", ModelName: "claude-haiku-4-5", Plan: "api", Format: "anthropic", ContextWindow: 200000, SupportsTools: true},
		},
		AllowCustomProvider: true,
		AllowCustomModel:    true,
	})
	globalResolver = NewResolver()
}
