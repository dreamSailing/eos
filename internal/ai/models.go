package ai

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "strings"

// ThinkingCapability 定义模型支持思考的能力等级
type ThinkingCapability int

const (
	ThinkingNone ThinkingCapability = iota
	ThinkingLow
	ThinkingMedium
	ThinkingHigh
)

// String 返回思考能力的字符串表示
func (tc ThinkingCapability) String() string {
	switch tc {
	case ThinkingLow:
		return "low"
	case ThinkingMedium:
		return "medium"
	case ThinkingHigh:
		return "high"
	default:
		return "none"
	}
}

// ParseThinkingCapability 从字符串解析思考能力
func ParseThinkingCapability(s string) ThinkingCapability {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return ThinkingLow
	case "medium":
		return ThinkingMedium
	case "high":
		return ThinkingHigh
	default:
		return ThinkingNone
	}
}

// ModelInfo 包含模型能力的元数据
type ModelInfo struct {
	Name                    string             // 模型名称
	Aliases                 []string           // 别名列表
	Thinking                ThinkingCapability // 思考能力等级
	SupportsReasoningEffort bool               // 是否支持 ReasoningEffort 参数
	Provider                string             // 提供商 (openai, anthropic, etc.)
}

// builtinModels 内置模型字典
// 包含常见模型的思考能力信息
var builtinModels = []ModelInfo{
	// OpenAI o1 系列 - 支持推理
	{
		Name:                    "o1",
		Aliases:                 []string{"o1-2024-12-17"},
		Thinking:                ThinkingHigh,
		SupportsReasoningEffort: true,
		Provider:                "openai",
	},
	{
		Name:                    "o1-mini",
		Aliases:                 []string{"o1-mini-2024-09-12"},
		Thinking:                ThinkingMedium,
		SupportsReasoningEffort: true,
		Provider:                "openai",
	},
	{
		Name:                    "o1-preview",
		Aliases:                 []string{"o1-preview-2024-09-12"},
		Thinking:                ThinkingMedium,
		SupportsReasoningEffort: true,
		Provider:                "openai",
	},

	// OpenAI GPT 系列 - 不支持原生思考
	{
		Name:     "gpt-4o",
		Aliases:  []string{"gpt-4o-2024-08-06", "gpt-4o-2024-05-13"},
		Thinking: ThinkingNone,
		Provider: "openai",
	},
	{
		Name:     "gpt-4o-mini",
		Aliases:  []string{"gpt-4o-mini-2024-07-18"},
		Thinking: ThinkingNone,
		Provider: "openai",
	},
	{
		Name:     "gpt-4-turbo",
		Aliases:  []string{"gpt-4-turbo-2024-04-09", "gpt-4-0125-preview", "gpt-4-1106-preview"},
		Thinking: ThinkingNone,
		Provider: "openai",
	},
	{
		Name:     "gpt-4",
		Aliases:  []string{"gpt-4-0613"},
		Thinking: ThinkingNone,
		Provider: "openai",
	},

	// Claude 系列 - 不支持原生思考（截至 2024）
	{
		Name:     "claude-3-5-sonnet",
		Aliases:  []string{"claude-3-5-sonnet-20241022", "claude-3.5-sonnet"},
		Thinking: ThinkingNone,
		Provider: "anthropic",
	},
	{
		Name:     "claude-3-opus",
		Aliases:  []string{"claude-3-opus-20240229"},
		Thinking: ThinkingNone,
		Provider: "anthropic",
	},
	{
		Name:     "claude-3-sonnet",
		Aliases:  []string{"claude-3-sonnet-20240229"},
		Thinking: ThinkingNone,
		Provider: "anthropic",
	},

	// Moonshot Kimi 系列
	{
		Name:     "kimi-k2.5",
		Aliases:  []string{"kimi-k2-5"},
		Thinking: ThinkingMedium,
		Provider: "moonshot",
	},

	// Zhipu GLM 系列
	{
		Name:     "glm-5",
		Aliases:  []string{"glm-5-turbo"},
		Thinking: ThinkingHigh,
		Provider: "zhipu",
	},
	{
		Name:     "glm-4.7",
		Aliases:  []string{"glm-4.7-flash"},
		Thinking: ThinkingHigh,
		Provider: "zhipu",
	},
	{
		Name:     "glm-4.6",
		Aliases:  []string{"glm-4.6v"},
		Thinking: ThinkingMedium,
		Provider: "zhipu",
	},

	// DeepSeek Reasoning 系列
	{
		Name:                    "deepseek-reasoner",
		Aliases:                 []string{"deepseek-r1"},
		Thinking:                ThinkingHigh,
		SupportsReasoningEffort: false, // DeepSeek 使用自己的推理格式
		Provider:                "deepseek",
	},

	// Qwen 3.5 系列
	{
		Name:                    "qwen3.6-plus",
		Aliases:                 []string{"qwen3.6", "qwen3.6-plus-2026-02-15", "qwen3.5-plus", "qwen3.5"},
		Thinking:                ThinkingHigh,
		SupportsReasoningEffort: false,
		Provider:                "dashscope",
	},

	// MiniMax M 系列
	{
		Name:     "minimax-m2.7",
		Aliases:  []string{"codex-minimax-m2.7", "minimax-m2.5", "codex-minimax-m2.5"},
		Thinking: ThinkingHigh,
		Provider: "minimax",
	},

	// Xiaomi MiMo 系列
	{
		Name:     "mimo-v2-pro",
		Aliases:  []string{"MiMo-V2-Pro"},
		Thinking: ThinkingHigh,
		Provider: "mimo",
	},
	{
		Name:     "mimo-v2-omni",
		Aliases:  []string{"MiMo-V2-Omni"},
		Thinking: ThinkingHigh,
		Provider: "mimo",
	},
}

// GetModelInfo 根据模型名称获取模型信息（不区分大小写）
func GetModelInfo(modelName string) (ModelInfo, bool) {
	name := strings.ToLower(strings.TrimSpace(modelName))
	for _, m := range builtinModels {
		if strings.ToLower(m.Name) == name {
			return m, true
		}
		for _, alias := range m.Aliases {
			if strings.ToLower(alias) == name {
				return m, true
			}
		}
	}
	return ModelInfo{}, false
}

// SupportsThinking 返回模型是否支持思考模式
func SupportsThinking(modelName string) bool {
	// 优先使用新模型目录
	if BuiltinSupportsThinking(modelName) {
		return true
	}
	// 回退到旧列表
	if info, ok := GetModelInfo(modelName); ok {
		return info.Thinking > ThinkingNone
	}
	return false
}

// SupportsReasoningEffort 返回模型是否支持 ReasoningEffort 参数
func SupportsReasoningEffort(modelName string) bool {
	// 优先使用新模型目录
	if BuiltinSupportsReasoningEffort(modelName) {
		return true
	}
	// 回退到旧列表
	if info, ok := GetModelInfo(modelName); ok {
		return info.SupportsReasoningEffort
	}
	return false
}

// GetThinkingCapability 获取模型的思考能力等级
func GetThinkingCapability(modelName string) ThinkingCapability {
	// 优先使用新模型目录
	if cap := BuiltinGetThinkingCapability(modelName); cap != ThinkingNone {
		return cap
	}
	// 回退到旧列表
	if info, ok := GetModelInfo(modelName); ok {
		return info.Thinking
	}
	return ThinkingNone
}
