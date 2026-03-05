package session

import (
	"strings"
	"github.com/dreamSailing/vb-coding/internal/ai"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

// TrimStrategy 裁剪策略
type TrimStrategy int

const (
	// TrimStrategyRecent 保留最近消息
	TrimStrategyRecent TrimStrategy = iota
	// TrimStrategyKeywords 保留包含关键词的消息
	TrimStrategyKeywords
	// TrimStrategySmart 智能裁剪（综合策略）
	TrimStrategySmart
	// TrimStrategyAggressive 激进裁剪（最大化压缩）
	TrimStrategyAggressive
)

func (s TrimStrategy) String() string {
	switch s {
	case TrimStrategyRecent:
		return "recent"
	case TrimStrategyKeywords:
		return "keywords"
	case TrimStrategySmart:
		return "smart"
	case TrimStrategyAggressive:
		return "aggressive"
	default:
		return "unknown"
	}
}

// SmartTrimmer 智能裁剪器
type SmartTrimmer struct {
	maxTokens      int
	strategy       TrimStrategy
	keywords       []string
	preserveSystem bool // 是否保留系统消息
	model          string
	cache          *utils.TokenEstimateCache
}

// NewSmartTrimmer 创建智能裁剪器
func NewSmartTrimmer(maxTokens int) *SmartTrimmer {
	return &SmartTrimmer{
		maxTokens:      maxTokens,
		strategy:       TrimStrategySmart,
		preserveSystem: true,
	}
}

// WithStrategy 设置裁剪策略
func (t *SmartTrimmer) WithStrategy(strategy TrimStrategy) *SmartTrimmer {
	t.strategy = strategy
	return t
}

// WithKeywords 设置关键词
func (t *SmartTrimmer) WithKeywords(keywords ...string) *SmartTrimmer {
	t.keywords = keywords
	return t
}

// WithPreserveSystem 设置是否保留系统消息
func (t *SmartTrimmer) WithPreserveSystem(preserve bool) *SmartTrimmer {
	t.preserveSystem = preserve
	return t
}

func (t *SmartTrimmer) WithModel(model string, cache *utils.TokenEstimateCache) *SmartTrimmer {
	t.model = strings.TrimSpace(model)
	t.cache = cache
	return t
}

// Trim 裁剪消息列表
func (t *SmartTrimmer) Trim(msgs []ai.Message) []ai.Message {
	switch t.strategy {
	case TrimStrategyRecent:
		return t.trimRecent(msgs)
	case TrimStrategyKeywords:
		return t.trimByKeywords(msgs)
	case TrimStrategySmart:
		return t.trimSmart(msgs)
	case TrimStrategyAggressive:
		return t.trimAggressive(msgs)
	default:
		return t.trimSmart(msgs)
	}
}

// trimRecent 保留最近的消息
func (t *SmartTrimmer) trimRecent(msgs []ai.Message) []ai.Message {
	var result []ai.Message
	currentTokens := 0

	// 从后向前遍历，保留最近的非系统消息
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]

		// 保留系统消息
		if t.preserveSystem && msg.Role == "system" {
			result = append([]ai.Message{msg}, result...)
			currentTokens += t.estimateTextTokens(msg.Content)
			continue
		}

		// 跳过元消息
		if msg.IsMeta {
			continue
		}

		msgTokens := t.estimateTextTokens(msg.Content)
		if currentTokens+msgTokens > t.maxTokens {
			break
		}

		result = append([]ai.Message{msg}, result...)
		currentTokens += msgTokens
	}

	return result
}

// trimByKeywords 按关键词裁剪
func (t *SmartTrimmer) trimByKeywords(msgs []ai.Message) []ai.Message {
	if len(t.keywords) == 0 {
		return t.trimRecent(msgs)
	}

	var result []ai.Message
	currentTokens := 0

	for _, msg := range msgs {
		// 保留系统消息
		if t.preserveSystem && msg.Role == "system" {
			result = append(result, msg)
			currentTokens += t.estimateTextTokens(msg.Content)
			continue
		}

		// 跳过元消息
		if msg.IsMeta {
			continue
		}

		// 检查是否包含关键词
		if !t.containsKeyword(msg.Content) {
			continue
		}

		msgTokens := t.estimateTextTokens(msg.Content)
		if currentTokens+msgTokens > t.maxTokens {
			continue
		}

		result = append(result, msg)
		currentTokens += msgTokens
	}

	return result
}

// trimSmart 智能裁剪
func (t *SmartTrimmer) trimSmart(msgs []ai.Message) []ai.Message {
	// 策略：
	// 1. 保留所有系统消息
	// 2. 保留最近的 N 条用户/助手消息
	// 3. 保留包含关键词的消息
	// 4. 如果超出 token 限制，截断过长的消息

	var systemMsgs []ai.Message
	var relevantMsgs []ai.Message
	currentTokens := 0

	// 首先分离系统消息
	for _, msg := range msgs {
		if t.preserveSystem && msg.Role == "system" {
			systemMsgs = append(systemMsgs, msg)
			currentTokens += t.estimateTextTokens(msg.Content)
		}
	}

	// 计算剩余 token 预算
	remainingTokens := t.maxTokens - currentTokens
	if remainingTokens <= 0 {
		return systemMsgs
	}

	// 从后向前收集相关消息
	recentCount := 0
	maxRecent := 8 // 保留最近 8 条消息

	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]

		// 跳过系统消息（已处理）
		if t.preserveSystem && msg.Role == "system" {
			continue
		}

		// 跳过元消息
		if msg.IsMeta {
			continue
		}

		// 检查是否相关（最近消息或包含关键词）
		isRelevant := recentCount < maxRecent
		if !isRelevant && len(t.keywords) > 0 {
			isRelevant = t.containsKeyword(msg.Content)
		}

		if !isRelevant {
			continue
		}

		// 处理过长的消息
		content := msg.Content
		msgTokens := t.estimateTextTokens(content)
		if msgTokens > remainingTokens && msgTokens > 2000 {
			maxLen := remainingTokens * 4
			if maxLen < 200 {
				maxLen = 200
			}
			if maxLen > 4000 {
				maxLen = 4000
			}
			if len(content) > maxLen {
				content = content[:maxLen] + "\n…[trimmed]"
			}
			msgTokens = t.estimateTextTokens(content)
		}

		if currentTokens+msgTokens > t.maxTokens {
			break
		}

		truncatedMsg := ai.Message{
			Role:       msg.Role,
			Content:    content,
			ImagePaths: msg.ImagePaths,
			IsMeta:     msg.IsMeta,
		}

		relevantMsgs = append([]ai.Message{truncatedMsg}, relevantMsgs...)
		currentTokens += msgTokens
		recentCount++
	}

	// 合并系统消息和相关消息
	result := append(systemMsgs, relevantMsgs...)
	return result
}

// trimAggressive 激进裁剪（最大化压缩）
func (t *SmartTrimmer) trimAggressive(msgs []ai.Message) []ai.Message {
	var result []ai.Message
	currentTokens := 0

	// 只保留系统消息和最近 3 条消息
	recentCount := 0
	maxRecent := 3

	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]

		// 保留系统消息
		if t.preserveSystem && msg.Role == "system" {
			result = append([]ai.Message{msg}, result...)
			currentTokens += t.estimateTextTokens(msg.Content)
			continue
		}

		// 跳过元消息
		if msg.IsMeta {
			continue
		}

		if recentCount >= maxRecent {
			continue
		}

		// 截断消息到最大 500 字符
		content := msg.Content
		if len(content) > 500 {
			content = content[:500] + "…"
		}

		msgTokens := t.estimateTextTokens(content)
		if currentTokens+msgTokens > t.maxTokens {
			break
		}

		truncatedMsg := ai.Message{
			Role:       msg.Role,
			Content:    content,
			ImagePaths: nil, // 激进裁剪时移除图片
			IsMeta:     msg.IsMeta,
		}

		result = append([]ai.Message{truncatedMsg}, result...)
		currentTokens += msgTokens
		recentCount++
	}

	return result
}

// containsKeyword 检查内容是否包含关键词
func (t *SmartTrimmer) containsKeyword(content string) bool {
	if len(t.keywords) == 0 {
		return true
	}
	lowerContent := strings.ToLower(content)
	for _, kw := range t.keywords {
		if strings.Contains(lowerContent, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// EstimateTokens 估算消息列表的 token 数
func (t *SmartTrimmer) EstimateTokens(msgs []ai.Message) int {
	total := 0
	for _, msg := range msgs {
		total += t.estimateTextTokens(msg.Content)
	}
	return total
}

// TrimToTarget 裁剪到目标 token 数
func (t *SmartTrimmer) TrimToTarget(msgs []ai.Message, targetTokens int) []ai.Message {
	t.maxTokens = targetTokens
	return t.Trim(msgs)
}

// AutoTrim 自动裁剪消息列表以适应 token 限制
func AutoTrim(msgs []ai.Message, maxTokens int) []ai.Message {
	trimmer := NewSmartTrimmer(maxTokens)
	return trimmer.Trim(msgs)
}

// AutoTrimWithStrategy 使用指定策略自动裁剪
func AutoTrimWithStrategy(msgs []ai.Message, maxTokens int, strategy TrimStrategy, keywords []string) []ai.Message {
	trimmer := NewSmartTrimmer(maxTokens).
		WithStrategy(strategy).
		WithKeywords(keywords...)
	return trimmer.Trim(msgs)
}

func (t *SmartTrimmer) estimateTextTokens(text string) int {
	return estimateTextTokens(t.model, t.cache, text)
}
