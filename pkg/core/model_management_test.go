//go:build legacy

package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/pkg/coreapi"
)

func TestRuntimeModelCatalogMatchesCLIProviders(t *testing.T) {
	seedLegacyRuntimeCatalog(t)
	configureCoreWorkspaceTestEnv(t)
	rt := NewRuntime()

	catalog := rt.ModelCatalog()
	if len(catalog.Providers) == 0 {
		t.Fatal("expected builtin providers")
	}
	if catalog.Providers[0].ID != "deepseek" {
		t.Fatalf("first provider=%q, want deepseek", catalog.Providers[0].ID)
	}
	if !catalog.AllowCustomProvider || !catalog.AllowCustomModel {
		t.Fatalf("catalog custom flags = provider:%v model:%v, want true/true", catalog.AllowCustomProvider, catalog.AllowCustomModel)
	}

	foundMiniMax := false
	foundMiMo := false
	foundDashscopeCodePlan := false
	foundMoonshotDefault := false
	foundOpenAIDefault := false
	foundAnthropicDefault := false
	foundDeepSeekDefault := false
	foundGeminiProvider := false

	foundDashscopePreset := false
	foundDashscopeCodePlanPreset := false
	for _, provider := range catalog.Providers {
		if provider.ID == "minimax" {
			foundMiniMax = true
			if provider.CodePlanAPIBase != "https://api.minimaxi.com/v1" {
				t.Fatalf("minimax codePlanApiBase=%q, want official /v1 base", provider.CodePlanAPIBase)
			}
		}
		if provider.ID == "dashscope" && provider.CodePlanAPIBase == "https://coding.dashscope.aliyuncs.com/v1" && provider.HasCodePlan {
			foundDashscopeCodePlan = true
		}
		if provider.ID == "mimo" {
			foundMiMo = true
			if provider.ClaudeAPIBase != "https://token-plan-cn.xiaomimimo.com/anthropic" {
				t.Fatalf("mimo claudeApiBase=%q, want official anthropic path", provider.ClaudeAPIBase)
			}
		}
		if provider.ID == "gemini" {
			foundGeminiProvider = true
			if provider.DefaultAPIBase != "https://generativelanguage.googleapis.com/v1beta/openai" {
				t.Fatalf("gemini defaultApiBase=%q, want official OpenAI-compatible base", provider.DefaultAPIBase)
			}
		}
		if provider.ID == "deepseek" && len(provider.DefaultModels) > 0 && provider.DefaultModels[0] == "deepseek-v4-pro" {
			foundDeepSeekDefault = true
		}
		if provider.ID == "moonshot" && len(provider.DefaultModels) > 0 && provider.DefaultModels[0] == "kimi-k2.6" {
			foundMoonshotDefault = true
		}
		if provider.ID == "openai" && len(provider.DefaultModels) > 0 && provider.DefaultModels[0] == "gpt-5.5" {
			foundOpenAIDefault = true
		}
		if provider.ID == "anthropic" && len(provider.DefaultModels) > 0 && provider.DefaultModels[0] == "claude-opus-4-7" {
			foundAnthropicDefault = true
		}
	}
	if !foundMiniMax {
		t.Fatal("expected minimax provider in catalog")
	}
	if !foundMiMo {
		t.Fatal("expected mimo provider in catalog")
	}
	if !foundDashscopeCodePlan {
		t.Fatal("expected dashscope coding plan base in catalog")
	}
	if !foundGeminiProvider {
		t.Fatal("expected gemini provider in catalog")
	}
	if !foundDeepSeekDefault {
		t.Fatal("expected deepseek default model to prefer deepseek-v4-pro")
	}
	if !foundMoonshotDefault {
		t.Fatal("expected moonshot default model to prefer kimi-k2.6")
	}
	if !foundOpenAIDefault {
		t.Fatal("expected openai default model to prefer gpt-5.5")
	}
	if !foundAnthropicDefault {
		t.Fatal("expected anthropic default model to prefer claude-opus-4-7")
	}

	for _, preset := range catalog.Presets {
		if preset.ProviderID == "dashscope" && preset.ID == "qwen3.6-plus" {
			foundDashscopePreset = true
			if preset.ModelName != "qwen3.6-plus" {
				t.Fatalf("preset modelName=%q, want qwen3.6-plus", preset.ModelName)
			}
			if preset.ContextWindow != 1000000 {
				t.Fatalf("preset contextWindow=%d, want 1000000", preset.ContextWindow)
			}
		}
		if preset.ProviderID == "dashscope" && preset.ID == "dashscope-coding-plan-qwen3.6-plus" {
			foundDashscopeCodePlanPreset = true
			if preset.ContextWindow != 1000000 {
				t.Fatalf("coding plan preset contextWindow=%d, want 1000000", preset.ContextWindow)
			}
		}
		if preset.ID == "minimax-token-plan-openai" && preset.ModelName != "MiniMax-M2.7" {
			t.Fatalf("minimax preset modelName=%q, want MiniMax-M2.7", preset.ModelName)
		}
		if preset.ID == "mimo-token-plan-openai-pro" && preset.ModelName != "mimo-v2.5-pro" {
			t.Fatalf("mimo preset modelName=%q, want mimo-v2.5-pro", preset.ModelName)
		}
	}
	if !foundDashscopePreset {
		t.Fatal("expected qwen3.6-plus preset in catalog")
	}
	if !foundDashscopeCodePlanPreset {
		t.Fatal("expected dashscope coding plan qwen3.6-plus preset in catalog")
	}
}

func TestRuntimeListModelDescriptorsClassifiesEntries(t *testing.T) {
	seedLegacyRuntimeCatalog(t)
	configureCoreWorkspaceTestEnv(t)
	writeCoreModelConfig(t, config.Config{
		Active: "env-qwen",
		Models: []config.ModelEntry{
			{
				Name:    "preset-qwen",
				APIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
				Model:   "qwen3.6-plus",
				Source:  "user",
			},
			{
				Name:     "custom-on-provider",
				APIBase:  "https://dashscope.aliyuncs.com/compatible-mode/v1/custom",
				Model:    "qwen-my-team",
				Provider: "dashscope",
				Source:   "user",
			},
			{
				Name:    "custom-provider",
				APIBase: "https://api.example.com/v1",
				Model:   "my-model",
				Source:  "user",
			},
			{
				Name:    "env-qwen",
				APIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
				Model:   "qwen3.6-plus",
				Source:  "env",
			},
		},
	})

	rt := NewRuntime()
	items := rt.ListModelDescriptors()
	if len(items) != 4 {
		t.Fatalf("descriptor count=%d, want 4", len(items))
	}

	preset := descriptorByName(items, "preset-qwen")
	if preset.EditKind != ModelEditKindPreset || preset.PresetID != "qwen3.6-plus" {
		t.Fatalf("preset descriptor=%+v, want preset qwen3.6-plus", preset)
	}

	customModel := descriptorByName(items, "custom-on-provider")
	if customModel.EditKind != ModelEditKindCustomModel || customModel.ProviderID != "dashscope" {
		t.Fatalf("customModel descriptor=%+v, want custom_model on dashscope", customModel)
	}

	customProvider := descriptorByName(items, "custom-provider")
	if customProvider.EditKind != ModelEditKindCustomProvider || customProvider.ProviderID != "custom" {
		t.Fatalf("customProvider descriptor=%+v, want custom_provider/custom", customProvider)
	}

	env := descriptorByName(items, "env-qwen")
	if env.EditKind != ModelEditKindEnv || env.CanEdit || env.CanDelete {
		t.Fatalf("env descriptor=%+v, want non-editable env model", env)
	}
}

func TestRuntimeSaveModelAddsPresetAndActivatesIt(t *testing.T) {
	seedLegacyRuntimeCatalog(t)
	configureCoreWorkspaceTestEnv(t)
	writeCoreModelConfig(t, config.Config{})
	rt := NewRuntime()

	if err := rt.SaveModel(ModelSaveRequest{
		Mode:       ModelEditKindPreset,
		ProviderID: "dashscope",
		PresetID:   "qwen3.6-plus",
		Name:       "main-qwen",
		APIKey:     "secret-token",
	}); err != nil {
		t.Fatalf("SaveModel(add preset) error = %v", err)
	}

	cfg, _ := config.Load()
	if cfg.Active != "main-qwen" {
		t.Fatalf("active=%q, want main-qwen", cfg.Active)
	}
	if len(cfg.Models) != 1 {
		t.Fatalf("models=%d, want 1", len(cfg.Models))
	}
	got := cfg.Models[0]
	if got.Provider != "dashscope" || got.APIType != "standard" {
		t.Fatalf("saved model provider/apiType = %q/%q, want dashscope/standard", got.Provider, got.APIType)
	}
	if got.Model != "qwen3.6-plus" {
		t.Fatalf("saved model=%q, want qwen3.6-plus", got.Model)
	}
	if got.APIBase != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("apiBase=%q, want dashscope default base", got.APIBase)
	}

	desc, err := rt.GetModelDescriptor("main-qwen")
	if err != nil {
		t.Fatalf("GetModelDescriptor() error = %v", err)
	}
	if desc.EditKind != ModelEditKindPreset || desc.PresetID != "qwen3.6-plus" || !desc.IsActive {
		t.Fatalf("descriptor=%+v, want active preset model", desc)
	}
}

func TestRuntimeSaveModelEditsCustomModelPreservesKeyAndRenamesActiveModel(t *testing.T) {
	seedLegacyRuntimeCatalog(t)
	configureCoreWorkspaceTestEnv(t)
	writeCoreModelConfig(t, config.Config{
		Active: "old-name",
		Models: []config.ModelEntry{
			{
				Name:                    "old-name",
				APIBase:                 "https://dashscope.aliyuncs.com/compatible-mode/v1/custom",
				APIKey:                  "keep-me",
				Model:                   "qwen-old",
				Source:                  "user",
				Provider:                "dashscope",
				APIType:                 "standard",
				ThinkingEnabled:         true,
				SupportsReasoningEffort: true,
			},
		},
	})
	rt := NewRuntime()

	if err := rt.SaveModel(ModelSaveRequest{
		OriginalName: "old-name",
		Mode:         ModelEditKindCustomModel,
		ProviderID:   "dashscope",
		Name:         "new-name",
		Model:        "qwen-new",
	}); err != nil {
		t.Fatalf("SaveModel(edit custom model) error = %v", err)
	}

	cfg, _ := config.Load()
	if cfg.Active != "new-name" {
		t.Fatalf("active=%q, want new-name", cfg.Active)
	}
	if len(cfg.Models) != 1 {
		t.Fatalf("models=%d, want 1", len(cfg.Models))
	}
	got := cfg.Models[0]
	if got.Name != "new-name" {
		t.Fatalf("name=%q, want new-name", got.Name)
	}
	if got.APIKey != "keep-me" {
		t.Fatalf("apiKey=%q, want preserved key", got.APIKey)
	}
	if got.APIBase != "https://dashscope.aliyuncs.com/compatible-mode/v1/custom" {
		t.Fatalf("apiBase=%q, want preserved custom base", got.APIBase)
	}
	if got.Model != "qwen-new" {
		t.Fatalf("model=%q, want qwen-new", got.Model)
	}
}

func TestRuntimeSaveModelAddsCodePlanPresetWithOfficialBase(t *testing.T) {
	seedLegacyRuntimeCatalog(t)
	configureCoreWorkspaceTestEnv(t)
	writeCoreModelConfig(t, config.Config{})
	rt := NewRuntime()

	if err := rt.SaveModel(ModelSaveRequest{
		Mode:       ModelEditKindPreset,
		ProviderID: "dashscope",
		PresetID:   "dashscope-coding-plan-qwen3.6-plus",
		Name:       "plan-qwen",
		APIKey:     "secret-token",
	}); err != nil {
		t.Fatalf("SaveModel(add code plan preset) error = %v", err)
	}

	cfg, _ := config.Load()
	if len(cfg.Models) != 1 {
		t.Fatalf("models=%d, want 1", len(cfg.Models))
	}
	got := cfg.Models[0]
	if got.APIType != "code-plan" {
		t.Fatalf("apiType=%q, want code-plan", got.APIType)
	}
	if got.APIBase != "https://coding.dashscope.aliyuncs.com/v1" {
		t.Fatalf("apiBase=%q, want official coding plan base", got.APIBase)
	}
	if got.Model != "qwen3.6-plus" {
		t.Fatalf("model=%q, want qwen3.6-plus", got.Model)
	}
}

func TestRuntimeSaveModelRejectsEnvironmentModelEdit(t *testing.T) {
	seedLegacyRuntimeCatalog(t)
	configureCoreWorkspaceTestEnv(t)
	writeCoreModelConfig(t, config.Config{
		Models: []config.ModelEntry{
			{
				Name:    "env-model",
				APIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
				APIKey:  "secret",
				Model:   "qwen3.6-plus",
				Source:  "env",
			},
		},
	})
	rt := NewRuntime()

	err := rt.SaveModel(ModelSaveRequest{
		OriginalName: "env-model",
		Mode:         ModelEditKindPreset,
		ProviderID:   "dashscope",
		PresetID:     "qwen3.6-plus",
		Name:         "env-model-renamed",
	})
	if err == nil || !strings.Contains(err.Error(), "environment model") {
		t.Fatalf("SaveModel(env edit) error = %v, want environment model rejection", err)
	}
}

func writeCoreModelConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	_, path := config.Load()
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
}

func descriptorByName(items []ModelDescriptor, name string) ModelDescriptor {
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	return ModelDescriptor{}
}

func seedLegacyRuntimeCatalog(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		ai.ApplyCoreModelCatalog(coreapi.ModelCatalogState{})
	})

	ai.ApplyCoreModelCatalog(coreapi.ModelCatalogState{
		Providers: []coreapi.ModelProviderOption{
			{ID: "deepseek", Name: "DeepSeek", DefaultAPIBase: "https://api.deepseek.com", DefaultModels: []string{"deepseek-v4-pro"}},
			{ID: "dashscope", Name: "阿里云通义", DefaultAPIBase: "https://dashscope.aliyuncs.com/compatible-mode/v1", CodePlanAPIBase: "https://coding.dashscope.aliyuncs.com/v1", HasCodePlan: true, DefaultModels: []string{"qwen3.6-plus", "dashscope-coding-plan-qwen3.6-plus"}},
			{ID: "zhipu", Name: "智谱 GLM", DefaultAPIBase: "https://open.bigmodel.cn/api/paas/v4", CodePlanAPIBase: "https://open.bigmodel.cn/api/coding/paas/v4", ClaudeAPIBase: "https://open.bigmodel.cn/api/anthropic", HasCodePlan: true, HasClaudeCode: true, DefaultModels: []string{"glm-5", "zhipu-coding-plan-openai", "zhipu-coding-plan-claude"}},
			{ID: "moonshot", Name: "Moonshot", DefaultAPIBase: "https://api.moonshot.cn/v1", DefaultModels: []string{"kimi-k2.6", "kimi-k2.5"}},
			{ID: "minimax", Name: "MiniMax", DefaultAPIBase: "https://api.minimaxi.com/v1", CodePlanAPIBase: "https://api.minimaxi.com/v1", ClaudeAPIBase: "https://api.minimaxi.com/anthropic/v1", HasCodePlan: true, HasClaudeCode: true, DefaultModels: []string{"minimax-token-plan-openai", "minimax-token-plan-claude"}},
			{ID: "mimo", Name: "小米 MiMo", DefaultAPIBase: "https://token-plan-cn.xiaomimimo.com/v1", CodePlanAPIBase: "https://token-plan-cn.xiaomimimo.com/v1", ClaudeAPIBase: "https://token-plan-cn.xiaomimimo.com/anthropic", HasCodePlan: true, HasClaudeCode: true, DefaultModels: []string{"mimo-token-plan-openai-pro", "mimo-token-plan-openai-omni", "mimo-token-plan-claude-pro"}},
			{ID: "gemini", Name: "Google Gemini", DefaultAPIBase: "https://generativelanguage.googleapis.com/v1beta/openai", DefaultModels: []string{"gemini-3.1-pro-preview"}},
			{ID: "openai", Name: "OpenAI", DefaultAPIBase: "https://api.openai.com/v1", DefaultModels: []string{"gpt-5.5", "gpt-5-codex"}},
			{ID: "anthropic", Name: "Anthropic", DefaultAPIBase: "https://api.anthropic.com/v1", DefaultModels: []string{"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4-5"}},
		},
		Presets: []coreapi.ModelPresetOption{
			{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", ProviderID: "deepseek", ModelName: "deepseek-v4-pro", APIType: "standard", ContextWindow: 1000000, SupportsTools: true},
			{ID: "qwen3.6-plus", Name: "Qwen 3.6 Plus", ProviderID: "dashscope", ModelName: "qwen3.6-plus", APIType: "standard", ContextWindow: 1000000, SupportsVision: true, SupportsTools: true},
			{ID: "dashscope-coding-plan-qwen3.6-plus", Name: "百炼 Coding Plan · Qwen 3.6 Plus", ProviderID: "dashscope", ModelName: "qwen3.6-plus", APIType: "code-plan", ContextWindow: 1000000, SupportsVision: true, SupportsTools: true},
			{ID: "glm-5", Name: "GLM-5", ProviderID: "zhipu", ModelName: "glm-5", APIType: "standard", ContextWindow: 200000, SupportsTools: true},
			{ID: "zhipu-coding-plan-openai", Name: "智谱 Coding Plan (OpenAI)", ProviderID: "zhipu", ModelName: "glm-5", APIType: "code-plan", ContextWindow: 200000, SupportsTools: true},
			{ID: "zhipu-coding-plan-claude", Name: "智谱 Coding Plan (Claude)", ProviderID: "zhipu", ModelName: "glm-5", APIType: "claude", ContextWindow: 200000, SupportsTools: true},
			{ID: "kimi-k2.6", Name: "Kimi K2.6", ProviderID: "moonshot", ModelName: "kimi-k2.6", APIType: "standard", ContextWindow: 0, SupportsTools: true},
			{ID: "kimi-k2.5", Name: "Kimi K2.5", ProviderID: "moonshot", ModelName: "kimi-k2.5", APIType: "standard", ContextWindow: 256000, SupportsVision: true, SupportsTools: true},
			{ID: "minimax-token-plan-openai", Name: "MiniMax Token Plan (OpenAI)", ProviderID: "minimax", ModelName: "MiniMax-M2.7", APIType: "code-plan", ContextWindow: 204800, SupportsVision: true, SupportsTools: true},
			{ID: "minimax-token-plan-claude", Name: "MiniMax Token Plan (Claude)", ProviderID: "minimax", ModelName: "MiniMax-M2.7", APIType: "claude", ContextWindow: 204800, SupportsVision: true, SupportsTools: true},
			{ID: "mimo-token-plan-openai-pro", Name: "MiMo Token Plan · MiMo-V2.5-Pro (OpenAI)", ProviderID: "mimo", ModelName: "mimo-v2.5-pro", APIType: "code-plan", ContextWindow: 1000000, SupportsTools: true},
			{ID: "mimo-token-plan-openai-omni", Name: "MiMo Token Plan · MiMo-V2.5 (OpenAI)", ProviderID: "mimo", ModelName: "mimo-v2.5", APIType: "code-plan", ContextWindow: 1000000, SupportsVision: true, SupportsTools: true},
			{ID: "mimo-token-plan-claude-pro", Name: "MiMo Token Plan · MiMo-V2.5-Pro (Claude)", ProviderID: "mimo", ModelName: "mimo-v2.5-pro", APIType: "claude", ContextWindow: 1000000, SupportsTools: true},
			{ID: "gemini-3.1-pro-preview", Name: "Gemini 3.1 Pro Preview", ProviderID: "gemini", ModelName: "gemini-3.1-pro-preview", APIType: "standard", ContextWindow: 1048576, SupportsVision: true, SupportsTools: true},
			{ID: "gpt-5.5", Name: "GPT-5.5", ProviderID: "openai", ModelName: "gpt-5.5", APIType: "standard", ContextWindow: 1050000, SupportsTools: true},
			{ID: "gpt-5-codex", Name: "GPT-5-Codex", ProviderID: "openai", ModelName: "gpt-5-codex", APIType: "standard", ContextWindow: 400000, SupportsTools: true},
			{ID: "claude-opus-4-7", Name: "Claude Opus 4.7", ProviderID: "anthropic", ModelName: "claude-opus-4-7", APIType: "standard", ContextWindow: 1000000, SupportsTools: true},
			{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", ProviderID: "anthropic", ModelName: "claude-sonnet-4-6", APIType: "standard", ContextWindow: 1000000, SupportsTools: true},
			{ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", ProviderID: "anthropic", ModelName: "claude-haiku-4-5", APIType: "standard", ContextWindow: 200000, SupportsTools: true},
		},
		AllowCustomProvider: true,
		AllowCustomModel:    true,
	})
}
