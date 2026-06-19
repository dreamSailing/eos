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
		cfg := providerConfigFromEndpoints(&provider)
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
			APIType:                 apiTypeFromPlanFormat(preset.Plan, preset.Format),
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

// providerConfigFromEndpoints derives the legacy flat ProviderConfig fields
// from the new (plan, format) endpoints vector.
func providerConfigFromEndpoints(p *coreapi.ModelProviderOption) *ProviderConfig {
	cfg := &ProviderConfig{
		ID:            strings.TrimSpace(p.ID),
		Name:          strings.TrimSpace(p.Name),
		Type:          ProviderType(strings.TrimSpace(p.ID)),
		APIKeyEnv:     strings.TrimSpace(p.APIKeyEnv),
		Website:       strings.TrimSpace(p.Website),
		DefaultModels: append([]string(nil), p.DefaultModels...),
	}

	for _, ep := range p.Endpoints {
		plan := strings.ToLower(strings.TrimSpace(ep.Plan))
		fmt := strings.ToLower(strings.TrimSpace(ep.Format))
		base := strings.TrimSpace(ep.APIBase)

		switch {
		case fmt == "openai_chat" && (plan == "" || plan == "api"):
			if cfg.DefaultAPIBase == "" {
				cfg.DefaultAPIBase = base
			}
		case fmt == "openai_chat" && plan == "code":
			cfg.CodePlanAPIBase = base
			cfg.HasCodePlan = true
		case fmt == "openai_chat" && plan == "agent":
			cfg.AgentPlanAPIBase = base
			cfg.HasAgentPlan = true
		case fmt == "openai_chat" && plan == "token":
			cfg.TokenPlanAPIBase = base
			cfg.HasTokenPlan = true
		case fmt == "anthropic" && plan == "agent":
			cfg.AgentPlanClaudeAPIBase = base
			cfg.HasAgentPlan = true
		case fmt == "anthropic" && plan == "token":
			cfg.TokenPlanClaudeAPIBase = base
			cfg.HasTokenPlan = true
		case fmt == "anthropic":
			cfg.ClaudeAPIBase = base
			cfg.HasClaudeCode = true
		case fmt == "openai_responses" && cfg.DefaultAPIBase == "":
			// Responses API endpoint also serves as a valid base for the provider
			cfg.DefaultAPIBase = base
		}
	}

	// Fallback: if no explicit default, use first available base
	if cfg.DefaultAPIBase == "" {
		for _, ep := range p.Endpoints {
			if base := strings.TrimSpace(ep.APIBase); base != "" {
				cfg.DefaultAPIBase = base
				break
			}
		}
	}

	// Derive boolean flags from endpoint presence
	if cfg.CodePlanAPIBase != "" {
		cfg.HasCodePlan = true
	}
	if cfg.ClaudeAPIBase != "" {
		cfg.HasClaudeCode = true
	}

	return cfg
}

// apiTypeFromPlanFormat maps (plan, format) to the legacy APIType enum.
func apiTypeFromPlanFormat(plan, format string) APIType {
	plan = strings.ToLower(strings.TrimSpace(plan))
	fmt := strings.ToLower(strings.TrimSpace(format))

	switch fmt {
	case "openai_chat", "openai_responses":
		switch plan {
		case "code":
			return APITypeCodePlan
		case "token":
			return APITypeTokenPlan
		default:
			return APITypeStandard
		}
	case "anthropic":
		switch plan {
		case "token":
			return APITypeTokenPlanClaude
		default:
			return APITypeClaude
		}
	default:
		return APITypeStandard
	}
}

func AllowCustomProviderFromCatalog() bool {
	return allowCustomProvider
}

func AllowCustomModelFromCatalog() bool {
	return allowCustomModel
}

func firstNonEmpty(primary, fallback string) string {
	if value := strings.TrimSpace(primary); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
