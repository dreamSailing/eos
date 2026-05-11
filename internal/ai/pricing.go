package ai

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

// ModelPricing holds pricing information for a model
type ModelPricing struct {
	InputPricePer1M       float64  // USD per 1M uncached input tokens
	CachedInputPricePer1M *float64 // USD per 1M cached input tokens, when provider bills it separately
	OutputPricePer1M      float64  // USD per 1M output tokens
	RequiresCacheBreakdown bool    // true when cost should stay unknown without explicit cached token detail
}

// CostEstimate holds the result of a cost estimation
type CostEstimate struct {
	InputCost  float64
	OutputCost float64
	TotalCost  float64
	Model      string
}

func pricePtr(v float64) *float64 {
	return &v
}

// knownPricing is a hardcoded table of official model prices.
// It is only used when the provider returned real token usage. If billing
// requires finer-grained usage than the provider exposes, cost stays unknown.
var knownPricing = map[string]ModelPricing{
	// DeepSeek
	"deepseek-v4-flash": {
		InputPricePer1M:         0.14,
		CachedInputPricePer1M:   pricePtr(0.028),
		OutputPricePer1M:        0.28,
		RequiresCacheBreakdown:  true,
	},
	"deepseek-v4-pro": {
		InputPricePer1M:         1.68,
		CachedInputPricePer1M:   pricePtr(0.14),
		OutputPricePer1M:        3.36,
		RequiresCacheBreakdown:  true,
	},
	// Qwen
	"qwen3.6-max-preview": {InputPricePer1M: 9.00, OutputPricePer1M: 54.00},
	"qwen3.6-plus":        {InputPricePer1M: 2.00, OutputPricePer1M: 12.00},
	"qwen3.6-flash":       {InputPricePer1M: 1.20, OutputPricePer1M: 7.20},
	"qwen-max":            {InputPricePer1M: 2.00, OutputPricePer1M: 6.00},
	"qwen-plus":           {InputPricePer1M: 0.80, OutputPricePer1M: 2.00},
	"qwen-turbo":          {InputPricePer1M: 0.30, OutputPricePer1M: 0.60},
	// OpenAI
	"gpt-5.5":     {InputPricePer1M: 5.00, OutputPricePer1M: 30.00},
	"gpt-4o":      {InputPricePer1M: 2.50, OutputPricePer1M: 10.00},
	"gpt-4o-mini": {InputPricePer1M: 0.15, OutputPricePer1M: 0.60},
	"gpt-4-turbo": {InputPricePer1M: 10.00, OutputPricePer1M: 30.00},
	"gpt-5-codex": {InputPricePer1M: 1.25, OutputPricePer1M: 10.00},
	"o1":          {InputPricePer1M: 15.00, OutputPricePer1M: 60.00},
	"o1-mini":     {InputPricePer1M: 3.00, OutputPricePer1M: 12.00},
	// Gemini
	// Google Gemini 3 pricing is tiered by request size. The estimator stores the
	// standard paid-tier short-context baseline (<= 200k prompt tokens) because
	// the current pricing model cannot express per-request price bands.
	"gemini-3.1-pro-preview": {InputPricePer1M: 2.00, OutputPricePer1M: 12.00},
	"gemini-3.1-pro-preview-customtools": {
		InputPricePer1M:  2.00,
		OutputPricePer1M: 12.00,
	},
	"gemini-3-flash-preview":        {InputPricePer1M: 0.50, OutputPricePer1M: 3.00},
	"gemini-3.1-flash-lite-preview": {InputPricePer1M: 0.25, OutputPricePer1M: 1.50},
	// Claude
	"claude-sonnet-4-6": {InputPricePer1M: 3.00, OutputPricePer1M: 15.00},
	"claude-opus-4-7":   {InputPricePer1M: 5.00, OutputPricePer1M: 25.00},
	"claude-opus-4-6":   {InputPricePer1M: 15.00, OutputPricePer1M: 75.00},
	"claude-haiku-4-5":  {InputPricePer1M: 1.00, OutputPricePer1M: 5.00},
	// GLM
	"glm-4-plus":  {InputPricePer1M: 8.00, OutputPricePer1M: 8.00},
	"glm-4":       {InputPricePer1M: 5.00, OutputPricePer1M: 5.00},
	"glm-4-flash": {InputPricePer1M: 0.10, OutputPricePer1M: 0.10},
	// Doubao
	"doubao-pro-32k":  {InputPricePer1M: 0.80, OutputPricePer1M: 2.00},
	"doubao-pro-128k": {InputPricePer1M: 5.00, OutputPricePer1M: 9.00},
	// Kimi
	"moonshot-v1-8k":  {InputPricePer1M: 1.67, OutputPricePer1M: 1.67},
	"moonshot-v1-32k": {InputPricePer1M: 3.33, OutputPricePer1M: 3.33},
	// MiniMax
	"minimax-m2.7": {InputPricePer1M: 0.30, OutputPricePer1M: 1.20},
	"abab6.5-chat": {InputPricePer1M: 1.00, OutputPricePer1M: 1.00},
	// MiMo
	"mimo-v2.5-pro": {InputPricePer1M: 1.00, OutputPricePer1M: 3.00},
	"mimo-v2.5":     {InputPricePer1M: 0.40, OutputPricePer1M: 2.00},
}

// EstimateUsageCost estimates USD cost using provider-returned usage only.
func EstimateUsageCost(model string, usage *schema.TokenUsage) (CostEstimate, bool) {
	if usage == nil {
		return CostEstimate{Model: model}, false
	}
	pricing, ok := GetModelPricing(model)
	if !ok {
		return CostEstimate{Model: model}, false
	}

	promptTokens := usage.PromptTokens
	cachedTokens := usage.PromptTokenDetails.CachedTokens
	if cachedTokens < 0 || cachedTokens > promptTokens {
		return CostEstimate{Model: model}, false
	}
	if pricing.RequiresCacheBreakdown && promptTokens > 0 && cachedTokens == 0 {
		return CostEstimate{Model: model}, false
	}
	uncachedTokens := promptTokens - cachedTokens

	inputCost := pricing.InputPricePer1M * float64(promptTokens) / 1_000_000
	if cachedTokens > 0 {
		if pricing.CachedInputPricePer1M == nil {
			return CostEstimate{Model: model}, false
		}
		inputCost = pricing.InputPricePer1M*float64(uncachedTokens)/1_000_000 +
			(*pricing.CachedInputPricePer1M)*float64(cachedTokens)/1_000_000
	}
	outputCost := pricing.OutputPricePer1M * float64(usage.CompletionTokens) / 1_000_000

	return CostEstimate{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Model:      model,
	}, true
}

// lookupPricing finds the pricing for a model, with prefix matching
func lookupPricing(model string) (ModelPricing, bool) {
	if model == "" {
		return ModelPricing{}, false
	}

	modelLower := strings.ToLower(model)

	// Exact match first
	if p, ok := knownPricing[modelLower]; ok {
		return p, true
	}

	// Prefix matching (e.g., "deepseek-chat-v3" matches "deepseek-chat")
	for key, pricing := range knownPricing {
		if strings.HasPrefix(modelLower, key) {
			return pricing, true
		}
	}

	// Default: no pricing info
	return ModelPricing{}, false
}

// GetModelPricing returns the pricing for a specific model
func GetModelPricing(model string) (ModelPricing, bool) {
	return lookupPricing(model)
}
