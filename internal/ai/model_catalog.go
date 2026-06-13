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
	SupportsImageGeneration bool               // 是否支持图片生成
	SupportsVideoGeneration bool               // 是否支持视频生成
	SupportsSpeechSynthesis bool               // 是否支持语音合成
	SupportsTools           bool               // 是否支持工具调用
	SupportsReasoningEffort bool               // 是否支持 ReasoningEffort 参数
	Tags                    []string           // 标签（推荐、免费、推理等）
	Description             string             // 描述
}

type ModelCatalog struct {
	models     map[string]*ModelCatalogEntry
	byProvider map[ProviderType][]*ModelCatalogEntry
	entries    []*ModelCatalogEntry
}

// NewModelCatalog 创建模型目录
func NewModelCatalog() *ModelCatalog {
	return &ModelCatalog{
		models:     make(map[string]*ModelCatalogEntry),
		byProvider: make(map[ProviderType][]*ModelCatalogEntry),
		entries:    make([]*ModelCatalogEntry, 0),
	}
}

// Get 根据 ID 获取模型
func (mc *ModelCatalog) Get(id string) *ModelCatalogEntry {
	return mc.models[strings.ToLower(strings.TrimSpace(id))]
}

// GetByProvider 获取指定服务商的所有模型
func (mc *ModelCatalog) GetByProvider(provider ProviderType) []*ModelCatalogEntry {
	return mc.byProvider[provider]
}

// GetAll 获取所有模型
func (mc *ModelCatalog) GetAll() []*ModelCatalogEntry {
	result := make([]*ModelCatalogEntry, 0, len(mc.entries))
	result = append(result, mc.entries...)
	return result
}

// Search 搜索模型（按名称或标签）
func (mc *ModelCatalog) Search(query string) []*ModelCatalogEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return mc.GetAll()
	}

	var result []*ModelCatalogEntry
	for _, entry := range mc.entries {
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
	for _, entry := range mc.entries {
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

func (mc *ModelCatalog) replaceAll(entries []*ModelCatalogEntry) {
	mc.models = make(map[string]*ModelCatalogEntry, len(entries))
	mc.byProvider = make(map[ProviderType][]*ModelCatalogEntry)
	mc.entries = make([]*ModelCatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		mc.models[strings.ToLower(strings.TrimSpace(entry.ID))] = entry
		mc.byProvider[entry.Provider] = append(mc.byProvider[entry.Provider], entry)
		mc.entries = append(mc.entries, entry)
	}
}

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
