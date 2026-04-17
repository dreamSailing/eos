package session

import (
	"strings"
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func estimateTextTokens(model string, cache *utils.TokenEstimateCache, text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	key := utils.TokenEstimateKey(model, text)
	if cache != nil {
		if v, ok := cache.Get(key); ok {
			return v
		}
	}
	tokens := utils.EstimateTokensWeighted(model, text)
	if cache != nil {
		cache.Put(key, tokens)
	}
	return tokens
}

func (c *ContextManager) estimateTextTokensLocked(text string) int {
	return estimateTextTokens(c.modelName, c.tokenCache, text)
}

func (c *ContextManager) estimateMessagesTokensLocked(msgs []ai.Message) int {
	total := 0
	for _, m := range msgs {
		total += estimateTextTokens(c.modelName, c.tokenCache, m.Content)
		if len(m.ImagePaths) > 0 {
			total += len(m.ImagePaths) * 256
		}
	}
	return total
}
