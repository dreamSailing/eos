package ai

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func assertPricingEqual(t *testing.T, model string, got, want ModelPricing) {
	t.Helper()
	if got.InputPricePer1M != want.InputPricePer1M || got.OutputPricePer1M != want.OutputPricePer1M {
		t.Fatalf("GetModelPricing(%q) = %+v, want %+v", model, got, want)
	}
	switch {
	case got.CachedInputPricePer1M == nil && want.CachedInputPricePer1M == nil:
	case got.CachedInputPricePer1M == nil || want.CachedInputPricePer1M == nil:
		t.Fatalf("GetModelPricing(%q) cached price mismatch: got=%v want=%v", model, got.CachedInputPricePer1M, want.CachedInputPricePer1M)
	case *got.CachedInputPricePer1M != *want.CachedInputPricePer1M:
		t.Fatalf("GetModelPricing(%q) cached price=%v, want %v", model, *got.CachedInputPricePer1M, *want.CachedInputPricePer1M)
	}
}

func TestGetModelPricingForGeminiCatalogModels(t *testing.T) {
	tests := []struct {
		model string
		want  ModelPricing
	}{
		{
			model: "gemini-3.1-pro-preview",
			want:  ModelPricing{InputPricePer1M: 2.00, OutputPricePer1M: 12.00},
		},
		{
			model: "gemini-3.1-pro-preview-customtools",
			want:  ModelPricing{InputPricePer1M: 2.00, OutputPricePer1M: 12.00},
		},
		{
			model: "gemini-3-flash-preview",
			want:  ModelPricing{InputPricePer1M: 0.50, OutputPricePer1M: 3.00},
		},
		{
			model: "gemini-3.1-flash-lite-preview",
			want:  ModelPricing{InputPricePer1M: 0.25, OutputPricePer1M: 1.50},
		},
	}

	for _, tt := range tests {
		got, ok := GetModelPricing(tt.model)
		if !ok {
			t.Fatalf("GetModelPricing(%q) not found", tt.model)
		}
		assertPricingEqual(t, tt.model, got, tt.want)
	}
}

func TestGetModelPricingForCurrentCatalogModels(t *testing.T) {
	tests := []struct {
		model string
		want  ModelPricing
	}{
		{
			model: "qwen3.6-plus",
			want:  ModelPricing{InputPricePer1M: 2.00, OutputPricePer1M: 12.00},
		},
		{
			model: "qwen3.6-max-preview",
			want:  ModelPricing{InputPricePer1M: 9.00, OutputPricePer1M: 54.00},
		},
		{
			model: "qwen3.6-flash",
			want:  ModelPricing{InputPricePer1M: 1.20, OutputPricePer1M: 7.20},
		},
		{
			model: "MiniMax-M2.7",
			want:  ModelPricing{InputPricePer1M: 0.30, OutputPricePer1M: 1.20},
		},
		{
			model: "mimo-v2.5-pro",
			want:  ModelPricing{InputPricePer1M: 1.00, OutputPricePer1M: 3.00},
		},
	}

	for _, tt := range tests {
		got, ok := GetModelPricing(tt.model)
		if !ok {
			t.Fatalf("GetModelPricing(%q) not found", tt.model)
		}
		assertPricingEqual(t, tt.model, got, tt.want)
	}
}

func TestGetModelPricingForDeepSeekLatestModelsOnly(t *testing.T) {
	tests := []struct {
		model string
		want  ModelPricing
	}{
		{
			model: "deepseek-v4-flash",
			want: ModelPricing{
				InputPricePer1M:       0.14,
				CachedInputPricePer1M: pricePtr(0.028),
				OutputPricePer1M:      0.28,
			},
		},
		{
			model: "deepseek-v4-pro",
			want: ModelPricing{
				InputPricePer1M:       1.68,
				CachedInputPricePer1M: pricePtr(0.14),
				OutputPricePer1M:      3.36,
			},
		},
	}
	for _, tt := range tests {
		got, ok := GetModelPricing(tt.model)
		if !ok {
			t.Fatalf("GetModelPricing(%q) not found", tt.model)
		}
		assertPricingEqual(t, tt.model, got, tt.want)
	}
	if _, ok := GetModelPricing("deepseek-chat"); ok {
		t.Fatalf("deprecated deepseek-chat pricing should not be maintained")
	}
}

func TestEstimateUsageCostRequiresProviderUsage(t *testing.T) {
	qwenUsage := &schema.TokenUsage{
		PromptTokens:     1000000,
		CompletionTokens: 200000,
		TotalTokens:      1200000,
	}
	cost, ok := EstimateUsageCost("qwen3.6-plus", qwenUsage)
	if !ok {
		t.Fatalf("EstimateUsageCost should work for models without cache-tier ambiguity")
	}
	if cost.TotalCost <= 0 {
		t.Fatalf("EstimateUsageCost total cost=%v, want > 0", cost.TotalCost)
	}
	if _, ok := EstimateUsageCost("deepseek-v4-flash", qwenUsage); ok {
		t.Fatalf("EstimateUsageCost should stay unknown for DeepSeek without explicit cached token detail")
	}
	if _, ok := EstimateUsageCost("deepseek-v4-pro", &schema.TokenUsage{
		PromptTokens:     1000000,
		CompletionTokens: 1,
		TotalTokens:      1000001,
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: 10,
		},
	}); !ok {
		t.Fatalf("EstimateUsageCost should use cached token price when detailed usage exists")
	}
}
