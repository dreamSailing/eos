package ai

import (
	"strings"

	"github.com/dreamSailing/eos/pkg/coreapi"
)

var (
	allowCustomProvider = true
	allowCustomModel    = true
)

// ApplyCoreModelCatalog replaces the production provider/model catalog with the
// snapshot returned by Rust model/catalog. Empty snapshots stay empty so the UI
// surfaces real core failures instead of falling back to local builtins.
func ApplyCoreModelCatalog(catalog coreapi.ModelCatalogState) {
	allowCustomProvider = catalog.AllowCustomProvider
	allowCustomModel = catalog.AllowCustomModel

	providers := make([]*ProviderConfig, 0, len(catalog.Providers)+1)
	for _, provider := range catalog.Providers {
		cfg := &ProviderConfig{
			ID:                      strings.TrimSpace(provider.ID),
			Name:                    strings.TrimSpace(provider.Name),
			Type:                    ProviderType(strings.TrimSpace(provider.ID)),
			DefaultAPIBase:          strings.TrimSpace(provider.DefaultAPIBase),
			CodePlanAPIBase:         strings.TrimSpace(provider.CodePlanAPIBase),
			ClaudeAPIBase:           strings.TrimSpace(provider.ClaudeAPIBase),
			AgentPlanAPIBase:        strings.TrimSpace(provider.AgentPlanAPIBase),
			AgentPlanClaudeAPIBase:  strings.TrimSpace(provider.AgentPlanClaudeAPIBase),
			APIKeyEnv:               strings.TrimSpace(provider.APIKeyEnv),
			Website:                 strings.TrimSpace(provider.Website),
			HasCodePlan:             provider.HasCodePlan,
			HasClaudeCode:           provider.HasClaudeCode,
			HasAgentPlan:            provider.HasAgentPlan,
			DefaultModels:           append([]string(nil), provider.DefaultModels...),
		}
		cfg.EinoComponent = defaultEinoComponent(cfg.Type)
		providers = append(providers, cfg)
	}
	if catalog.AllowCustomProvider {
		providers = append(providers, &ProviderConfig{
			ID:   "custom",
			Name: "自定义",
			Type: ProviderCustom,
		})
	}
	globalRegistry.replaceAll(providers)

	entries := make([]*ModelCatalogEntry, 0, len(catalog.Presets))
	for _, preset := range catalog.Presets {
		entry := &ModelCatalogEntry{
			ID:                      strings.TrimSpace(preset.ID),
			Name:                    strings.TrimSpace(preset.Name),
			Provider:                ProviderType(strings.TrimSpace(preset.ProviderID)),
			ModelName:               strings.TrimSpace(preset.ModelName),
			APIType:                 APIType(strings.TrimSpace(preset.APIType)),
			ContextWindow:           preset.ContextWindow,
			ThinkingCap:             DetectThinkingCapability(firstNonEmpty(preset.ModelName, preset.ID)),
			SupportsVision:          preset.SupportsVision,
			SupportsImageGeneration: preset.SupportsImageGeneration,
			SupportsVideoGeneration: preset.SupportsVideoGeneration,
			SupportsSpeechSynthesis: preset.SupportsSpeechSynthesis,
			SupportsTools:           preset.SupportsTools || !preset.SupportsImageGeneration && !preset.SupportsVideoGeneration && !preset.SupportsSpeechSynthesis,
			Tags:                    append([]string(nil), preset.Tags...),
			Description:             strings.TrimSpace(preset.Description),
			SupportsReasoningEffort: preset.SupportsReasoningEffort,
		}
		entries = append(entries, entry)
	}
	globalCatalog.replaceAll(entries)
}

func AllowCustomProviderFromCatalog() bool {
	return allowCustomProvider
}

func AllowCustomModelFromCatalog() bool {
	return allowCustomModel
}

func defaultEinoComponent(provider ProviderType) string {
	switch provider {
	case ProviderDeepSeek:
		return "deepseek"
	case ProviderDashScope:
		return "dashscope"
	case ProviderByteDance:
		return "volcengine"
	case ProviderZhipu:
		return "zhipuai"
	case ProviderOpenAI:
		return "openai"
	case ProviderAnthropic:
		return "anthropic"
	default:
		return ""
	}
}

func firstNonEmpty(primary, fallback string) string {
	if value := strings.TrimSpace(primary); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
