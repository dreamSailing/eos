package ai

import "strings"

// ModelPricing holds pricing information for a model
type ModelPricing struct {
	InputPricePer1M  float64 // USD per 1M input tokens
	OutputPricePer1M float64 // USD per 1M output tokens
}

// CostEstimate holds the result of a cost estimation
type CostEstimate struct {
	InputCost  float64
	OutputCost float64
	TotalCost  float64
	Model      string
}

// knownPricing is a hardcoded table of model prices (USD per 1M tokens)
var knownPricing = map[string]ModelPricing{
	// DeepSeek
	"deepseek-chat":       {InputPricePer1M: 0.14, OutputPricePer1M: 0.28},
	"deepseek-reasoner":   {InputPricePer1M: 0.55, OutputPricePer1M: 2.19},
	// Qwen
	"qwen-max":            {InputPricePer1M: 2.00, OutputPricePer1M: 6.00},
	"qwen-plus":           {InputPricePer1M: 0.80, OutputPricePer1M: 2.00},
	"qwen-turbo":          {InputPricePer1M: 0.30, OutputPricePer1M: 0.60},
	// OpenAI
	"gpt-4o":              {InputPricePer1M: 2.50, OutputPricePer1M: 10.00},
	"gpt-4o-mini":         {InputPricePer1M: 0.15, OutputPricePer1M: 0.60},
	"gpt-4-turbo":         {InputPricePer1M: 10.00, OutputPricePer1M: 30.00},
	"o1":                  {InputPricePer1M: 15.00, OutputPricePer1M: 60.00},
	"o1-mini":             {InputPricePer1M: 3.00, OutputPricePer1M: 12.00},
	// Claude
	"claude-sonnet-4-6":   {InputPricePer1M: 3.00, OutputPricePer1M: 15.00},
	"claude-opus-4-6":     {InputPricePer1M: 15.00, OutputPricePer1M: 75.00},
	"claude-haiku-4-5":    {InputPricePer1M: 0.80, OutputPricePer1M: 4.00},
	// GLM
	"glm-4-plus":          {InputPricePer1M: 8.00, OutputPricePer1M: 8.00},
	"glm-4":               {InputPricePer1M: 5.00, OutputPricePer1M: 5.00},
	"glm-4-flash":         {InputPricePer1M: 0.10, OutputPricePer1M: 0.10},
	// Doubao
	"doubao-pro-32k":      {InputPricePer1M: 0.80, OutputPricePer1M: 2.00},
	"doubao-pro-128k":     {InputPricePer1M: 5.00, OutputPricePer1M: 9.00},
	// Kimi
	"moonshot-v1-8k":      {InputPricePer1M: 1.67, OutputPricePer1M: 1.67},
	"moonshot-v1-32k":     {InputPricePer1M: 3.33, OutputPricePer1M: 3.33},
	// MiniMax
	"abab6.5-chat":        {InputPricePer1M: 1.00, OutputPricePer1M: 1.00},
}

// EstimateCost estimates the USD cost for a model invocation
func EstimateCost(model string, inputTokens, outputTokens int) CostEstimate {
	pricing := lookupPricing(model)

	inputCost := pricing.InputPricePer1M * float64(inputTokens) / 1_000_000
	outputCost := pricing.OutputPricePer1M * float64(outputTokens) / 1_000_000

	return CostEstimate{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Model:      model,
	}
}

// lookupPricing finds the pricing for a model, with prefix matching
func lookupPricing(model string) ModelPricing {
	if model == "" {
		return ModelPricing{}
	}

	modelLower := strings.ToLower(model)

	// Exact match first
	if p, ok := knownPricing[modelLower]; ok {
		return p
	}

	// Prefix matching (e.g., "deepseek-chat-v3" matches "deepseek-chat")
	for key, pricing := range knownPricing {
		if strings.HasPrefix(modelLower, key) {
			return pricing
		}
	}

	// Default: no pricing info
	return ModelPricing{}
}

// GetModelPricing returns the pricing for a specific model
func GetModelPricing(model string) (ModelPricing, bool) {
	modelLower := strings.ToLower(model)
	p, ok := knownPricing[modelLower]
	if !ok {
		// Try prefix match
		for key, pricing := range knownPricing {
			if strings.HasPrefix(modelLower, key) {
				return pricing, true
			}
		}
	}
	return p, ok
}
