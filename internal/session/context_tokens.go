package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


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
