package ai

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"strings"
)

// ModelCatalogEntry 模型目录条目
type ModelCatalogEntry struct {
	ID                      string             // 唯一标识
	Name                    string             // 显示名称
	Provider                ProviderType       // 服务商类型
	ModelName               string             // API 调用时的模型名称
	APIType                 APIType            // API 类型
	ContextWindow           int                // 上下文窗口大小
	ThinkingCap             ThinkingCapability // 思考能力等级
	SupportsVision          bool               // 是否支持视觉
	SupportsTools           bool               // 是否支持工具调用
	SupportsReasoningEffort bool               // 是否支持 ReasoningEffort 参数
	Tags                    []string           // 标签（推荐、免费、推理等）
	Description             string             // 描述
}

// builtinModelsCatalog 内置模型目录
var builtinModelsCatalog = []ModelCatalogEntry{
	// ===== DeepSeek 模型 =====
	{
		ID:                      "deepseek-chat",
		Name:                    "DeepSeek Chat",
		Provider:                ProviderDeepSeek,
		ModelName:               "deepseek-chat",
		APIType:                 APITypeStandard,
		ContextWindow:           64000,
		ThinkingCap:             ThinkingNone,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "通用"},
		Description:             "DeepSeek 官方稳定通用模型 ID。",
	},
	{
		ID:                      "deepseek-reasoner",
		Name:                    "DeepSeek Reasoner",
		Provider:                ProviderDeepSeek,
		ModelName:               "deepseek-reasoner",
		APIType:                 APITypeStandard,
		ContextWindow:           64000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "推理"},
		Description:             "DeepSeek 官方稳定推理模型 ID。",
	},

	// ===== 智谱 GLM 模型 =====
	{
		ID:                      "glm-5",
		Name:                    "GLM-5",
		Provider:                ProviderZhipu,
		ModelName:               "glm-5",
		APIType:                 APITypeStandard,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "旗舰", "编程"},
		Description:             "智谱官方当前主力通用模型。",
	},
	{
		ID:                      "glm-5-turbo",
		Name:                    "GLM-5 Turbo",
		Provider:                ProviderZhipu,
		ModelName:               "glm-5-turbo",
		APIType:                 APITypeStandard,
		ContextWindow:           0,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"快速", "推荐"},
		Description:             "智谱官方轻量通用模型。",
	},
	{
		ID:                      "zhipu-coding-plan-openai",
		Name:                    "智谱 Coding Plan (OpenAI)",
		Provider:                ProviderZhipu,
		ModelName:               "glm-5",
		APIType:                 APITypeCodePlan,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Plan", "套餐", "推荐", "编程", "OpenAI"},
		Description:             "智谱 GLM Coding Plan 的 OpenAI 兼容入口。",
	},
	{
		ID:                      "zhipu-coding-plan-claude",
		Name:                    "智谱 Coding Plan (Claude)",
		Provider:                ProviderZhipu,
		ModelName:               "glm-5",
		APIType:                 APITypeClaude,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Plan", "套餐", "推荐", "编程", "Claude"},
		Description:             "智谱 GLM Coding Plan 的 Claude 兼容入口。",
	},
	{
		ID:                      "glm-4.7",
		Name:                    "GLM-4.7",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4.7",
		APIType:                 APITypeStandard,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "旗舰", "200K", "编程", "推理"},
		Description:             "智谱官方通用模型。",
	},
	{
		ID:                      "glm-4.6",
		Name:                    "GLM-4.6",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4.6",
		APIType:                 APITypeStandard,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "旗舰", "200K"},
		Description:             "智谱官方多模态模型。",
	},
	{
		ID:                      "glm-4.5-air",
		Name:                    "GLM-4.5-Air",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4.5-air",
		APIType:                 APITypeStandard,
		ContextWindow:           0,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"快速", "推荐"},
		Description:             "智谱官方轻量模型。",
	},
	{
		ID:                      "glm-4.5v",
		Name:                    "GLM-4.5V",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4.5v",
		APIType:                 APITypeStandard,
		ContextWindow:           0,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"视觉", "推荐"},
		Description:             "智谱官方视觉模型。",
	},

	// ===== 阿里云通义模型 =====
	{
		ID:                      "qwen3.6-plus",
		Name:                    "Qwen 3.6 Plus",
		Provider:                ProviderDashScope,
		ModelName:               "qwen3.6-plus",
		APIType:                 APITypeStandard,
		ContextWindow:           1000000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "视觉理解", "通用"},
		Description:             "百炼官方稳定通用模型。",
	},
	{
		ID:                      "dashscope-coding-plan-qwen3.6-plus",
		Name:                    "百炼 Coding Plan · Qwen 3.6 Plus",
		Provider:                ProviderDashScope,
		ModelName:               "qwen3.6-plus",
		APIType:                 APITypeCodePlan,
		ContextWindow:           262144,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Plan", "套餐", "推荐", "OpenAI"},
		Description:             "百炼 Coding Plan 官方支持模型。",
	},
	{
		ID:                      "dashscope-coding-plan-glm-5",
		Name:                    "百炼 Coding Plan · GLM-5",
		Provider:                ProviderDashScope,
		ModelName:               "glm-5",
		APIType:                 APITypeCodePlan,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Plan", "套餐", "推荐", "OpenAI"},
		Description:             "百炼 Coding Plan 官方支持模型。",
	},
	{
		ID:                      "dashscope-coding-plan-kimi-k2.5",
		Name:                    "百炼 Coding Plan · Kimi K2.5",
		Provider:                ProviderDashScope,
		ModelName:               "kimi-k2.5",
		APIType:                 APITypeCodePlan,
		ContextWindow:           0,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Plan", "套餐", "视觉", "OpenAI"},
		Description:             "百炼 Coding Plan 官方支持模型。",
	},
	{
		ID:                      "dashscope-coding-plan-minimax-m2.5",
		Name:                    "百炼 Coding Plan · MiniMax M2.5",
		Provider:                ProviderDashScope,
		ModelName:               "MiniMax-M2.5",
		APIType:                 APITypeCodePlan,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Plan", "套餐", "多模态", "OpenAI"},
		Description:             "百炼 Coding Plan 官方支持模型。",
	},

	// ===== 字节豆包模型 =====
	{
		ID:                      "doubao-seed-1.8",
		Name:                    "Doubao Seed 1.8",
		Provider:                ProviderByteDance,
		ModelName:               "doubao-seed-1.8",
		APIType:                 APITypeStandard,
		ContextWindow:           256000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "旗舰", "256K", "多模态"},
		Description:             "Doubao 1.8 旗舰模型，针对多模态 Agent 场景优化，支持复杂工具调用与长视频理解",
	},
	{
		ID:                      "doubao-seed-code",
		Name:                    "Doubao Seed Code",
		Provider:                ProviderByteDance,
		ModelName:               "doubao-seed-code-preview-251028",
		APIType:                 APITypeStandard,
		ContextWindow:           256000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"编程", "旗舰", "256K"},
		Description:             "Doubao 编程旗舰，专为 Agentic Coding 优化，支持 256k 上下文与代码库级任务",
	},
	{
		ID:                      "ark-coding-plan-openai",
		Name:                    "方舟 Coding Plan (OpenAI)",
		Provider:                ProviderByteDance,
		ModelName:               "ark-code-latest",
		APIType:                 APITypeCodePlan,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Plan", "套餐", "推荐", "编程", "OpenAI"},
		Description:             "方舟 Coding Plan 的 OpenAI 兼容入口。",
	},
	{
		ID:                      "ark-coding-plan-claude",
		Name:                    "方舟 Coding Plan (Claude)",
		Provider:                ProviderByteDance,
		ModelName:               "ark-code-latest",
		APIType:                 APITypeClaude,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Plan", "套餐", "推荐", "编程", "Claude"},
		Description:             "方舟 Coding Plan 的 Claude 兼容入口。",
	},

	// ===== Moonshot Kimi 模型 =====
	{
		ID:                      "kimi-k2.5",
		Name:                    "Kimi K2.5",
		Provider:                ProviderMoonshot,
		ModelName:               "kimi-k2.5",
		APIType:                 APITypeStandard,
		ContextWindow:           0,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "多模态", "256K", "推理"},
	},

	// ===== MiniMax 模型 =====
	{
		ID:                      "minimax-token-plan-openai",
		Name:                    "MiniMax Token Plan (OpenAI)",
		Provider:                ProviderMiniMax,
		ModelName:               "MiniMax-M2.7",
		APIType:                 APITypeCodePlan,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Token Plan", "套餐", "推荐", "OpenAI"},
		Description:             "MiniMax Token Plan 的 OpenAI 兼容入口。",
	},
	{
		ID:                      "minimax-token-plan-claude",
		Name:                    "MiniMax Token Plan (Claude)",
		Provider:                ProviderMiniMax,
		ModelName:               "MiniMax-M2.7",
		APIType:                 APITypeClaude,
		ContextWindow:           0,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Token Plan", "套餐", "推荐", "Claude"},
		Description:             "MiniMax Token Plan 的 Claude 兼容入口。",
	},

	// ===== Xiaomi MiMo 模型 =====
	{
		ID:                      "mimo-token-plan-openai-pro",
		Name:                    "MiMo Token Plan · MiMo-V2-Pro (OpenAI)",
		Provider:                ProviderMiMo,
		ModelName:               "mimo-v2-pro",
		APIType:                 APITypeCodePlan,
		ContextWindow:           1048576,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Token Plan", "推荐", "文本", "OpenAI"},
		Description:             "小米 MiMo Token Plan 的 OpenAI 兼容文本模型。",
	},
	{
		ID:                      "mimo-token-plan-openai-omni",
		Name:                    "MiMo Token Plan · MiMo-V2-Omni (OpenAI)",
		Provider:                ProviderMiMo,
		ModelName:               "mimo-v2-omni",
		APIType:                 APITypeCodePlan,
		ContextWindow:           1048576,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Token Plan", "推荐", "视觉", "OpenAI"},
		Description:             "小米 MiMo Token Plan 的 OpenAI 兼容多模态模型。",
	},
	{
		ID:                      "mimo-token-plan-claude-pro",
		Name:                    "MiMo Token Plan · MiMo-V2-Pro (Claude)",
		Provider:                ProviderMiMo,
		ModelName:               "mimo-v2-pro",
		APIType:                 APITypeClaude,
		ContextWindow:           1048576,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Token Plan", "文本", "Claude"},
		Description:             "小米 MiMo Token Plan 的 Anthropic 兼容文本模型。",
	},
	{
		ID:                      "mimo-token-plan-claude-omni",
		Name:                    "MiMo Token Plan · MiMo-V2-Omni (Claude)",
		Provider:                ProviderMiMo,
		ModelName:               "mimo-v2-omni",
		APIType:                 APITypeClaude,
		ContextWindow:           1048576,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Token Plan", "视觉", "Claude"},
		Description:             "小米 MiMo Token Plan 的 Anthropic 兼容多模态模型。",
	},

	// ===== OpenAI 模型 =====
	{
		ID:                      "gpt-4o",
		Name:                    "GPT-4o",
		Provider:                ProviderOpenAI,
		ModelName:               "gpt-4o",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingNone,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"多模态"},
	},
	{
		ID:                      "o3-mini",
		Name:                    "OpenAI o3-mini",
		Provider:                ProviderOpenAI,
		ModelName:               "o3-mini",
		APIType:                 APITypeStandard,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: true,
		Tags:                    []string{"推理", "经济", "推荐"},
	},
	{
		ID:                      "o3",
		Name:                    "OpenAI o3",
		Provider:                ProviderOpenAI,
		ModelName:               "o3",
		APIType:                 APITypeStandard,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: true,
		Tags:                    []string{"推理", "旗舰"},
	},
	{
		ID:                      "gpt-5.3-codex",
		Name:                    "GPT-5.3 Codex",
		Provider:                ProviderOpenAI,
		ModelName:               "gpt-5.3-codex",
		APIType:                 APITypeStandard,
		ContextWindow:           400000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: true,
		Tags:                    []string{"编程", "旗舰", "400K", "推荐"},
		Description:             "OpenAI 最强编程模型，400K 上下文，推理速度提升 25%",
	},

	// ===== Anthropic Claude 模型 =====
	{
		ID:                      "claude-opus-4.6",
		Name:                    "Claude Opus 4.6",
		Provider:                ProviderAnthropic,
		ModelName:               "claude-opus-4.6",
		APIType:                 APITypeStandard,
		ContextWindow:           1000000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"旗舰", "推理", "1M", "推荐"},
		Description:             "Anthropic 最强模型，100万上下文，Adaptive Thinking，GPQA Diamond 77.3%",
	},
	{
		ID:                      "claude-4.5-sonnet",
		Name:                    "Claude Sonnet 4.5",
		Provider:                ProviderAnthropic,
		ModelName:               "claude-4.5-sonnet",
		APIType:                 APITypeStandard,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "通用"},
	},
	{
		ID:                      "claude-4.5-opus",
		Name:                    "Claude Opus 4.5",
		Provider:                ProviderAnthropic,
		ModelName:               "claude-4.5-opus",
		APIType:                 APITypeStandard,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推理", "旗舰"},
	},
	{
		ID:                      "claude-4.5-haiku",
		Name:                    "Claude Haiku 4.5",
		Provider:                ProviderAnthropic,
		ModelName:               "claude-4.5-haiku",
		APIType:                 APITypeStandard,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingLow,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"快速", "经济"},
	},
}

// ModelCatalog 模型目录
type ModelCatalog struct {
	models     map[string]*ModelCatalogEntry
	byProvider map[ProviderType][]*ModelCatalogEntry
}

// NewModelCatalog 创建模型目录
func NewModelCatalog() *ModelCatalog {
	mc := &ModelCatalog{
		models:     make(map[string]*ModelCatalogEntry),
		byProvider: make(map[ProviderType][]*ModelCatalogEntry),
	}
	for i := range builtinModelsCatalog {
		entry := &builtinModelsCatalog[i]
		mc.models[entry.ID] = entry
		mc.byProvider[entry.Provider] = append(mc.byProvider[entry.Provider], entry)
	}
	return mc
}

// Get 根据 ID 获取模型
func (mc *ModelCatalog) Get(id string) *ModelCatalogEntry {
	return mc.models[id]
}

// GetByProvider 获取指定服务商的所有模型
func (mc *ModelCatalog) GetByProvider(provider ProviderType) []*ModelCatalogEntry {
	return mc.byProvider[provider]
}

// GetAll 获取所有模型
func (mc *ModelCatalog) GetAll() []*ModelCatalogEntry {
	result := make([]*ModelCatalogEntry, 0, len(builtinModelsCatalog))
	for i := range builtinModelsCatalog {
		result = append(result, &builtinModelsCatalog[i])
	}
	return result
}

// Search 搜索模型（按名称或标签）
func (mc *ModelCatalog) Search(query string) []*ModelCatalogEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return mc.GetAll()
	}

	var result []*ModelCatalogEntry
	for i := range builtinModelsCatalog {
		entry := &builtinModelsCatalog[i]
		// 搜索名称
		if strings.Contains(strings.ToLower(entry.Name), q) ||
			strings.Contains(strings.ToLower(entry.ID), q) {
			result = append(result, entry)
			continue
		}
		// 搜索标签
		for _, tag := range entry.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				result = append(result, entry)
				break
			}
		}
	}
	return result
}

// FilterByTags 按标签筛选模型
func (mc *ModelCatalog) FilterByTags(tags []string) []*ModelCatalogEntry {
	if len(tags) == 0 {
		return mc.GetAll()
	}

	var result []*ModelCatalogEntry
	for i := range builtinModelsCatalog {
		entry := &builtinModelsCatalog[i]
		for _, tag := range entry.Tags {
			for _, filterTag := range tags {
				if strings.EqualFold(tag, filterTag) {
					result = append(result, entry)
					break
				}
			}
		}
	}
	return result
}

// GetRecommended 获取推荐模型
func (mc *ModelCatalog) GetRecommended() []*ModelCatalogEntry {
	return mc.FilterByTags([]string{"推荐"})
}

// globalCatalog 全局模型目录
var globalCatalog = NewModelCatalog()

// GetModelEntry 根据 ID 获取模型（使用全局目录）
func GetModelEntry(id string) *ModelCatalogEntry {
	return globalCatalog.Get(id)
}

// GetAllModels 获取所有模型（使用全局目录）
func GetAllModels() []*ModelCatalogEntry {
	return globalCatalog.GetAll()
}

// GetModelsByProvider 获取指定服务商的所有模型（使用全局目录）
func GetModelsByProvider(provider ProviderType) []*ModelCatalogEntry {
	return globalCatalog.GetByProvider(provider)
}

// SearchModels 搜索模型（使用全局目录）
func SearchModels(query string) []*ModelCatalogEntry {
	return globalCatalog.Search(query)
}

// FilterModelsByTags 按标签筛选模型（使用全局目录）
func FilterModelsByTags(tags []string) []*ModelCatalogEntry {
	return globalCatalog.FilterByTags(tags)
}

// GetRecommendedModels 获取推荐模型（使用全局目录）
func GetRecommendedModels() []*ModelCatalogEntry {
	return globalCatalog.GetRecommended()
}

// CatalogEntryToModelInfo 将目录条目转换为 ModelInfo
func CatalogEntryToModelInfo(entry ModelCatalogEntry) ModelInfo {
	return ModelInfo{
		Name:                    entry.ID,
		Aliases:                 []string{entry.ModelName},
		Thinking:                entry.ThinkingCap,
		SupportsReasoningEffort: entry.SupportsReasoningEffort,
		Provider:                string(entry.Provider),
	}
}

// GetBuiltinModelInfo 返回合并后的内置模型信息
// 首先从目录中查找，如果没有则从旧的 builtinModels 中查找
func GetBuiltinModelInfo(modelName string) (ModelInfo, bool) {
	// 先从新目录查找
	if entry := GetModelEntry(modelName); entry != nil {
		return CatalogEntryToModelInfo(*entry), true
	}
	// 再从旧列表查找
	return GetModelInfo(modelName)
}

// BuiltinSupportsThinking 返回模型是否支持思考模式（优先使用目录）
func BuiltinSupportsThinking(modelName string) bool {
	if info, ok := GetBuiltinModelInfo(modelName); ok {
		return info.Thinking > ThinkingNone
	}
	return false
}

// BuiltinSupportsReasoningEffort 返回模型是否支持 ReasoningEffort（优先使用目录）
func BuiltinSupportsReasoningEffort(modelName string) bool {
	if info, ok := GetBuiltinModelInfo(modelName); ok {
		return info.SupportsReasoningEffort
	}
	return false
}

// BuiltinGetThinkingCapability 获取模型的思考能力等级（优先使用目录）
func BuiltinGetThinkingCapability(modelName string) ThinkingCapability {
	if info, ok := GetBuiltinModelInfo(modelName); ok {
		return info.Thinking
	}
	return ThinkingNone
}
