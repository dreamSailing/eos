package ai

import (
	"strings"
)

// ModelProviderInfo 模型和服务商的组合信息
type ModelProviderInfo struct {
	Provider         *ProviderConfig    // 服务商配置
	Model            *ModelCatalogEntry // 模型目录条目
	APIBase          string             // 实际使用的 API Base URL
	APIKey           string             // API Key
	ProviderType     ProviderType       // 服务商类型
	RequiresCodePlan bool               // 是否需要使用 Code Plan API
}

// Resolver 服务商和模型解析器
type Resolver struct {
	registry *ProviderRegistry
	catalog  *ModelCatalog
}

// NewResolver 创建解析器
func NewResolver() *Resolver {
	return &Resolver{
		registry: globalRegistry,
		catalog:  globalCatalog,
	}
}

// ResolveByAPIBase 根据 API Base URL 解析服务商
func (r *Resolver) ResolveByAPIBase(baseURL string) *ProviderConfig {
	return r.registry.DetectProvider(baseURL)
}

// ResolveByModelName 根据模型名称解析服务商和模型信息
func (r *Resolver) ResolveByModelName(modelName string) *ModelProviderInfo {
	name := strings.ToLower(strings.TrimSpace(modelName))

	// 从模型目录查找
	for _, entry := range builtinModelsCatalog {
		if strings.ToLower(entry.ModelName) == name || strings.ToLower(entry.ID) == name {
			provider := r.registry.Get(entry.Provider)
			if provider == nil {
				continue
			}

			info := &ModelProviderInfo{
				Provider:         provider,
				Model:            &entry,
				APIBase:          r.getAPIBaseForEntry(&entry, provider),
				ProviderType:     entry.Provider,
				RequiresCodePlan: entry.APIType == APITypeCodePlan,
			}
			return info
		}
	}

	return nil
}

// Resolve 根据 API Base 和模型名称解析完整信息
func (r *Resolver) Resolve(baseURL, modelName string) *ModelProviderInfo {
	info := r.ResolveByModelName(modelName)
	if info != nil {
		// 如果提供了自定义 baseURL，使用它
		if baseURL != "" {
			info.APIBase = baseURL
		}
		return info
	}

	// 没找到模型信息，尝试通过 API Base 推断
	provider := r.ResolveByAPIBase(baseURL)
	if provider != nil {
		return &ModelProviderInfo{
			Provider:         provider,
			Model:            nil,
			APIBase:          baseURL,
			ProviderType:     provider.Type,
			RequiresCodePlan: false,
		}
	}

	// 完全未知，返回自定义配置
	return &ModelProviderInfo{
		Provider:         r.registry.Get(ProviderCustom),
		Model:            nil,
		APIBase:          baseURL,
		ProviderType:     ProviderCustom,
		RequiresCodePlan: false,
	}
}

// getAPIBaseForEntry 根据模型条目获取正确的 API Base URL
func (r *Resolver) getAPIBaseForEntry(entry *ModelCatalogEntry, provider *ProviderConfig) string {
	switch entry.APIType {
	case APITypeCodePlan:
		if provider.CodePlanAPIBase != "" {
			return provider.CodePlanAPIBase
		}
	case APITypeClaude:
		if provider.ClaudeAPIBase != "" {
			return provider.ClaudeAPIBase
		}
		// 如果没有专门的 Claude Base，尝试返回 CodePlan
		if provider.CodePlanAPIBase != "" {
			return provider.CodePlanAPIBase
		}
	}
	return provider.DefaultAPIBase
}

// GetDefaultModelForProvider 获取服务商的默认模型
func (r *Resolver) GetDefaultModelForProvider(providerType ProviderType) string {
	provider := r.registry.Get(providerType)
	if provider == nil || len(provider.DefaultModels) == 0 {
		return ""
	}
	return provider.DefaultModels[0]
}

// GetAvailableProvidersForModel 根据模型名称获取可用的服务商列表
func (r *Resolver) GetAvailableProvidersForModel(modelName string) []*ProviderConfig {
	name := strings.ToLower(strings.TrimSpace(modelName))
	var providers []*ProviderConfig

	for _, entry := range builtinModelsCatalog {
		if strings.ToLower(entry.ModelName) == name || strings.ToLower(entry.ID) == name {
			provider := r.registry.Get(entry.Provider)
			if provider != nil {
				providers = append(providers, provider)
			}
		}
	}

	return providers
}

// IsCompatible 检查 API Base 和模型是否兼容
func (r *Resolver) IsCompatible(baseURL, modelName string) bool {
	info := r.ResolveByModelName(modelName)
	if info == nil {
		return true // 未知模型，不进行限制
	}

	// 检查 API Base 是否匹配服务商
	provider := r.ResolveByAPIBase(baseURL)
	if provider == nil {
		return true // 未知 API Base，不进行限制
	}

	return provider.Type == info.ProviderType
}

// GetRecommendedModelForBase 根据 API Base 推荐模型
func (r *Resolver) GetRecommendedModelForBase(baseURL string) string {
	provider := r.ResolveByAPIBase(baseURL)
	if provider == nil || len(provider.DefaultModels) == 0 {
		return ""
	}

	// 优先返回推荐模型
	models := provider.DefaultModels
	for _, modelID := range models {
		entry := globalCatalog.Get(modelID)
		if entry != nil {
			for _, tag := range entry.Tags {
				if tag == "推荐" {
					return entry.ModelName
				}
			}
		}
	}

	// 没有标记推荐的，返回第一个
	if entry := globalCatalog.Get(models[0]); entry != nil {
		return entry.ModelName
	}

	return models[0]
}

// globalResolver 全局解析器
var globalResolver = NewResolver()

// ResolveProviderByBase 根据 API Base URL 解析服务商（使用全局解析器）
func ResolveProviderByBase(baseURL string) *ProviderConfig {
	return globalResolver.ResolveByAPIBase(baseURL)
}

// ResolveModelInfo 根据模型名称解析信息（使用全局解析器）
func ResolveModelInfo(modelName string) *ModelProviderInfo {
	return globalResolver.ResolveByModelName(modelName)
}

// ResolveProviderAndModel 根据 API Base 和模型名称解析（使用全局解析器）
func ResolveProviderAndModel(baseURL, modelName string) *ModelProviderInfo {
	return globalResolver.Resolve(baseURL, modelName)
}

// GetDefaultModel 获取服务商的默认模型（使用全局解析器）
func GetDefaultModel(providerType ProviderType) string {
	return globalResolver.GetDefaultModelForProvider(providerType)
}

// IsModelAndBaseCompatible 检查 API Base 和模型是否兼容（使用全局解析器）
func IsModelAndBaseCompatible(baseURL, modelName string) bool {
	return globalResolver.IsCompatible(baseURL, modelName)
}

// GetRecommendedModel 根据 API Base 推荐模型（使用全局解析器）
func GetRecommendedModel(baseURL string) string {
	return globalResolver.GetRecommendedModelForBase(baseURL)
}

// GetEinoComponentName 获取服务商的 Eino 组件名称
func GetEinoComponentName(providerType ProviderType) string {
	provider := GetProvider(providerType)
	if provider == nil {
		return "" // 使用默认 OpenAI 兼容
	}
	return provider.EinoComponent
}

// SupportsVision 检查模型是否支持视觉（优先使用目录）
func SupportsVisionFromCatalog(modelName string) bool {
	id := strings.ToLower(strings.TrimSpace(modelName))
	if entry := GetModelEntry(id); entry != nil {
		return entry.SupportsVision
	}
	// 回退到原来的检测方式
	return SupportsVision(modelName)
}

func SupportsToolsFromCatalog(modelName string) bool {
	id := strings.ToLower(strings.TrimSpace(modelName))
	if entry := GetModelEntry(id); entry != nil {
		return entry.SupportsTools
	}
	return true
}

// GetModelContextWindow 获取模型的上下文窗口大小
func GetModelContextWindow(modelName string) int {
	id := strings.ToLower(strings.TrimSpace(modelName))
	if entry := GetModelEntry(id); entry != nil {
		return entry.ContextWindow
	}
	return 0
}

// GetModelAPIType 获取模型的 API 类型
func GetModelAPIType(modelName string) APIType {
	id := strings.ToLower(strings.TrimSpace(modelName))
	if entry := GetModelEntry(id); entry != nil {
		return entry.APIType
	}
	return APITypeStandard
}

// ShouldUseCodePlanAPI 检查模型是否应该使用 Code Plan API
func ShouldUseCodePlanAPI(baseURL, modelName string) bool {
	info := ResolveProviderAndModel(baseURL, modelName)
	if info == nil {
		return IsCodePlanModel(modelName)
	}
	return info.RequiresCodePlan || IsCodePlanModel(modelName)
}

// InferDefaultModelFromBase 从 API Base URL 推断默认模型
// 这是 config.InferDefaultModel 的增强版本
func InferDefaultModelFromBase(baseURL string) string {
	if recommended := GetRecommendedModel(baseURL); recommended != "" {
		return recommended
	}

	// 回退到原来的逻辑
	b := strings.ToLower(strings.TrimSpace(baseURL))
	if strings.Contains(b, "dashscope.aliyuncs.com") && strings.Contains(b, "compatible-mode") {
		return "qwen3.5-plus"
	}
	if strings.Contains(b, "api.kimi.com") && strings.Contains(b, "/coding/") {
		return "kimi-for-coding"
	}
	return ""
}
