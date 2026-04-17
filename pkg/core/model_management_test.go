package core

import (
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/config"
)

func TestRuntimeModelCatalogMatchesCLIProviders(t *testing.T) {
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

	foundDashscopePreset := false
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

	for _, preset := range catalog.Presets {
		if preset.ProviderID == "dashscope" && preset.ID == "qwen3.6-plus" {
			foundDashscopePreset = true
			if preset.ModelName != "qwen3.6-plus" {
				t.Fatalf("preset modelName=%q, want qwen3.6-plus", preset.ModelName)
			}
		}
		if preset.ID == "minimax-token-plan-openai" && preset.ModelName != "MiniMax-M2.7" {
			t.Fatalf("minimax preset modelName=%q, want MiniMax-M2.7", preset.ModelName)
		}
		if preset.ID == "mimo-token-plan-openai-pro" && preset.ModelName != "mimo-v2-pro" {
			t.Fatalf("mimo preset modelName=%q, want mimo-v2-pro", preset.ModelName)
		}
	}
	if !foundDashscopePreset {
		t.Fatal("expected qwen3.6-plus preset in catalog")
	}
}

func TestRuntimeListModelDescriptorsClassifiesEntries(t *testing.T) {
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
