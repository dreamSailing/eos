package ai

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
		ID:                      "deepseek-v3.2",
		Name:                    "DeepSeek V3.2",
		Provider:                ProviderDeepSeek,
		ModelName:               "deepseek-v3.2",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: true,
		Tags:                    []string{"推荐", "旗舰", "混合思考", "工具思考"},
		Description:             "DeepSeek 最新旗舰，支持思考模式下的工具调用，推理能力比肩 GPT-5",
	},
	{
		ID:                      "deepseek-v3.2-speciale",
		Name:                    "DeepSeek V3.2 Speciale",
		Provider:                ProviderDeepSeek,
		ModelName:               "deepseek-v3.2-speciale",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           false, // API 文档注明暂不支持工具
		SupportsReasoningEffort: true,
		Tags:                    []string{"推理", "极致"},
		Description:             "DeepSeek 极致推理版本，专注于复杂数学与逻辑竞赛题",
	},
	{
		ID:                      "deepseek-v3.1",
		Name:                    "DeepSeek V3.1",
		Provider:                ProviderDeepSeek,
		ModelName:               "deepseek-v3.1",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: true,
		Tags:                    []string{"旗舰", "混合思考"},
		Description:             "DeepSeek V3.1，支持混合思考模式",
	},
	{
		ID:                      "deepseek-chat",
		Name:                    "DeepSeek V3",
		Provider:                ProviderDeepSeek,
		ModelName:               "deepseek-chat",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingNone,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"对话", "经典"},
		Description:             "DeepSeek V3 (DeepSeek Chat)，经典通用模型",
	},
	{
		ID:                      "deepseek-reasoner",
		Name:                    "DeepSeek R1",
		Provider:                ProviderDeepSeek,
		ModelName:               "deepseek-reasoner",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推理", "R1"},
		Description:             "DeepSeek R1 推理模型，早期的纯推理版本",
	},

	// ===== 智谱 GLM 模型 =====
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
	},
	{
		ID:                      "zhipu-coding-plan-openai",
		Name:                    "智谱 Coding Plan (OpenAI)",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4.7",
		APIType:                 APITypeCodePlan,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Code Plan", "套餐", "推荐", "编程", "OpenAI"},
		Description:             "兼容 OpenAI 协议的智谱 Coding Plan 接口 (glm-4.7)",
	},
	{
		ID:                      "zhipu-coding-plan-claude",
		Name:                    "智谱 Coding Plan (Claude)",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4.7",
		APIType:                 APITypeClaude,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Code Plan", "套餐", "推荐", "编程", "Claude"},
		Description:             "兼容 Anthropic Claude 协议的智谱 Coding Plan 接口 (glm-4.7)",
	},
	{
		ID:                      "glm-4.7-flash",
		Name:                    "GLM-4.7 Flash",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4.7-flash",
		APIType:                 APITypeStandard,
		ContextWindow:           200000,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"免费", "快速", "推荐"},
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
	},
	{
		ID:                      "glm-4.6v-flash",
		Name:                    "GLM-4.6V Flash",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4.6v-flash",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingNone,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"免费", "快速", "视觉"},
	},
	{
		ID:                      "glm-4-plus",
		Name:                    "GLM-4 Plus",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4-plus",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingNone,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "旗舰"},
	},
	{
		ID:                      "glm-4v-plus",
		Name:                    "GLM-4V Plus",
		Provider:                ProviderZhipu,
		ModelName:               "glm-4v-plus",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingNone,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"视觉", "推荐"},
	},

	// ===== 阿里云通义模型 =====
	{
		ID:                      "qwen3.5-plus",
		Name:                    "Qwen 3.5 Plus",
		Provider:                ProviderDashScope,
		ModelName:               "qwen3.5-plus",
		APIType:                 APITypeStandard,
		ContextWindow:           1000000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "深度思考", "视觉理解"},
		Description:             "Qwen3.5 原生视觉语言 Plus 模型，采用线性注意力与稀疏混合专家架构，在纯文本与多模态任务中具备高推理效率和强性能，等同快照模型 qwen3.5-plus-2026-02-15。",
	},

	// ===== 字节豆包模型 =====
	{
		ID:                      "ark-code-latest",
		Name:                    "Ark Code Latest",
		Provider:                ProviderByteDance,
		ModelName:               "ark-code-latest",
		APIType:                 APITypeCodePlan,
		ContextWindow:           256000,
		ThinkingCap:             ThinkingHigh,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"编程", "256K"},
	},
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
		ID:                      "doubao-seed-1.6",
		Name:                    "Doubao Seed 1.6",
		Provider:                ProviderByteDance,
		ModelName:               "doubao-seed-1.6",
		APIType:                 APITypeStandard,
		ContextWindow:           256000,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "256K"},
	},
	{
		ID:                      "doubao-seed-1.6-lite",
		Name:                    "Doubao Seed 1.6 Lite",
		Provider:                ProviderByteDance,
		ModelName:               "doubao-seed-1.6-lite",
		APIType:                 APITypeStandard,
		ContextWindow:           256000,
		ThinkingCap:             ThinkingLow,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"快速", "经济", "256K"},
		Description:             "Doubao 1.6 轻量版，适合高吞吐场景",
	},
	{
		ID:                      "doubao-seed-1.6-flash",
		Name:                    "Doubao Seed 1.6 Flash",
		Provider:                ProviderByteDance,
		ModelName:               "doubao-seed-1.6-flash",
		APIType:                 APITypeStandard,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingNone,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"极速", "免费", "128K"},
		Description:             "Doubao 1.6 极速版，低延迟响应",
	},
	{
		ID:                      "ark-coding-plan-openai",
		Name:                    "方舟 Coding Plan (OpenAI)",
		Provider:                ProviderByteDance,
		ModelName:               "ark-code-latest",
		APIType:                 APITypeCodePlan,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingLow,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Code Plan", "套餐", "推荐", "编程", "OpenAI"},
		Description:             "兼容 OpenAI 协议的方舟 Coding Plan 接口",
	},
	{
		ID:                      "ark-coding-plan-claude",
		Name:                    "方舟 Coding Plan (Claude)",
		Provider:                ProviderByteDance,
		ModelName:               "ark-code-latest",
		APIType:                 APITypeClaude,
		ContextWindow:           128000,
		ThinkingCap:             ThinkingLow,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"Code Plan", "套餐", "推荐", "编程", "Claude"},
		Description:             "兼容 Anthropic Claude 协议的方舟 Coding Plan 接口",
	},

	// ===== Moonshot Kimi 模型 =====
	{
		ID:                      "kimi-k2-5",
		Name:                    "Kimi K2.5",
		Provider:                ProviderMoonshot,
		ModelName:               "kimi-k2-5",
		APIType:                 APITypeStandard,
		ContextWindow:           256000,
		ThinkingCap:             ThinkingMedium,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsReasoningEffort: false,
		Tags:                    []string{"推荐", "多模态", "256K", "推理"},
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
