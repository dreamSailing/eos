package core

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/config"
)

type ModelEditKind string

const (
	ModelEditKindPreset         ModelEditKind = "preset"
	ModelEditKindCustomModel    ModelEditKind = "custom_model"
	ModelEditKindCustomProvider ModelEditKind = "custom_provider"
	ModelEditKindEnv            ModelEditKind = "env"
)

type ModelProviderOption struct {
	ID              string
	Name            string
	Website         string
	APIKeyEnv       string
	DefaultAPIBase  string
	CodePlanAPIBase string
	ClaudeAPIBase   string
	HasCodePlan     bool
	HasClaudeCode   bool
	DefaultModels   []string
}

type ModelPresetOption struct {
	ID                      string
	Name                    string
	ProviderID              string
	ModelName               string
	APIType                 string
	ContextWindow           int
	Tags                    []string
	Description             string
	SupportsReasoningEffort bool
}

type ModelCatalogState struct {
	Providers           []ModelProviderOption
	Presets             []ModelPresetOption
	AllowCustomProvider bool
	AllowCustomModel    bool
}

type ModelDescriptor struct {
	Name                    string
	APIBase                 string
	APIKey                  string
	Model                   string
	Source                  string
	IsActive                bool
	SupportsReasoningEffort bool
	ProviderID              string
	APIType                 string
	PresetID                string
	EditKind                ModelEditKind
	CanEdit                 bool
	CanDelete               bool
}

type ModelSaveRequest struct {
	OriginalName string
	Mode         ModelEditKind
	ProviderID   string
	PresetID     string
	Name         string
	APIKey       string
	APIBase      string
	Model        string
}

func (r *Runtime) ModelCatalog() ModelCatalogState {
	providers := ai.GetAllProviders()
	items := make([]ModelProviderOption, 0, len(providers))
	presets := make([]ModelPresetOption, 0)
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		items = append(items, ModelProviderOption{
			ID:              strings.TrimSpace(provider.ID),
			Name:            strings.TrimSpace(provider.Name),
			Website:         strings.TrimSpace(provider.Website),
			APIKeyEnv:       strings.TrimSpace(provider.APIKeyEnv),
			DefaultAPIBase:  strings.TrimSpace(provider.DefaultAPIBase),
			CodePlanAPIBase: strings.TrimSpace(provider.CodePlanAPIBase),
			ClaudeAPIBase:   strings.TrimSpace(provider.ClaudeAPIBase),
			HasCodePlan:     provider.HasCodePlan,
			HasClaudeCode:   provider.HasClaudeCode,
			DefaultModels:   append([]string(nil), provider.DefaultModels...),
		})
		for _, preset := range ai.GetModelsByProvider(provider.Type) {
			if preset == nil {
				continue
			}
			presets = append(presets, ModelPresetOption{
				ID:                      strings.TrimSpace(preset.ID),
				Name:                    strings.TrimSpace(preset.Name),
				ProviderID:              strings.TrimSpace(provider.ID),
				ModelName:               strings.TrimSpace(preset.ModelName),
				APIType:                 strings.TrimSpace(string(preset.APIType)),
				ContextWindow:           preset.ContextWindow,
				Tags:                    append([]string(nil), preset.Tags...),
				Description:             strings.TrimSpace(preset.Description),
				SupportsReasoningEffort: preset.SupportsReasoningEffort,
			})
		}
	}
	return ModelCatalogState{
		Providers:           items,
		Presets:             presets,
		AllowCustomProvider: true,
		AllowCustomModel:    true,
	}
}

func (r *Runtime) ListModelDescriptors() []ModelDescriptor {
	cfg, _ := r.core.LoadFullModelConfig()
	out := make([]ModelDescriptor, 0, len(cfg.Models))
	for _, item := range cfg.Models {
		out = append(out, describeModelEntry(cfg, item))
	}
	return out
}

func (r *Runtime) GetModelDescriptor(name string) (ModelDescriptor, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ModelDescriptor{}, errors.New("model name required")
	}
	cfg, _ := r.core.LoadFullModelConfig()
	for _, item := range cfg.Models {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return describeModelEntry(cfg, item), nil
		}
	}
	return ModelDescriptor{}, errors.New("model not found")
}

func (r *Runtime) SaveModel(req ModelSaveRequest) error {
	cfg, cfgPath := config.Load()
	if strings.TrimSpace(cfgPath) == "" {
		return errors.New("config path empty")
	}

	mode := normalizeModelEditKind(req.Mode)
	if mode == "" || mode == ModelEditKindEnv {
		return errors.New("invalid model save mode")
	}

	originalName := strings.TrimSpace(req.OriginalName)
	entry, err := buildModelEntry(req, cfg)
	if err != nil {
		return err
	}

	if originalName == "" {
		for _, item := range cfg.Models {
			if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(entry.Name)) {
				return errors.New("model already exists")
			}
		}
		cfg.Models = append(cfg.Models, entry)
		cfg.Active = entry.Name
		if err := config.Save(cfg, cfgPath); err != nil {
			return err
		}
		return r.core.Reload()
	}

	index := -1
	var current config.ModelEntry
	for idx, item := range cfg.Models {
		if strings.EqualFold(strings.TrimSpace(item.Name), originalName) {
			index = idx
			current = item
			break
		}
	}
	if index < 0 {
		return errors.New("model not found")
	}
	if strings.EqualFold(strings.TrimSpace(current.Source), "env") {
		return errors.New("environment model cannot be edited")
	}
	for idx, item := range cfg.Models {
		if idx == index {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(entry.Name)) {
			return errors.New("model already exists")
		}
	}

	if strings.TrimSpace(req.APIKey) == "" {
		entry.APIKey = current.APIKey
	}
	entry.Source = fallbackModelSource(current.Source)
	entry.ThinkingEnabled = current.ThinkingEnabled && entry.SupportsReasoningEffort

	cfg.Models[index] = entry
	wasActive := strings.EqualFold(strings.TrimSpace(cfg.Active), originalName)
	if wasActive {
		cfg.Active = entry.Name
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		return err
	}
	if wasActive {
		return r.core.Reload()
	}
	return nil
}

func describeModelEntry(cfg config.Config, entry config.ModelEntry) ModelDescriptor {
	providerID, apiType, presetID, editKind := classifyModelEntry(entry)
	name := strings.TrimSpace(entry.Name)
	source := strings.TrimSpace(entry.Source)
	supportsReasoning := entry.SupportsReasoningEffort || ai.SupportsReasoningEffort(entry.Model)
	isActive := strings.EqualFold(strings.TrimSpace(cfg.Active), name)
	canEdit := editKind != ModelEditKindEnv
	canDelete := editKind != ModelEditKindEnv && !isActive

	return ModelDescriptor{
		Name:                    name,
		APIBase:                 strings.TrimSpace(entry.APIBase),
		APIKey:                  strings.TrimSpace(entry.APIKey),
		Model:                   strings.TrimSpace(entry.Model),
		Source:                  source,
		IsActive:                isActive,
		SupportsReasoningEffort: supportsReasoning,
		ProviderID:              providerID,
		APIType:                 apiType,
		PresetID:                presetID,
		EditKind:                editKind,
		CanEdit:                 canEdit,
		CanDelete:               canDelete,
	}
}

func buildModelEntry(req ModelSaveRequest, cfg config.Config) (config.ModelEntry, error) {
	mode := normalizeModelEditKind(req.Mode)
	switch mode {
	case ModelEditKindPreset:
		provider := ai.GetProviderByID(strings.TrimSpace(req.ProviderID))
		if provider == nil {
			return config.ModelEntry{}, errors.New("provider not found")
		}
		preset := ai.GetModelEntry(strings.TrimSpace(req.PresetID))
		if preset == nil || !strings.EqualFold(provider.ID, providerIDFromType(preset.Provider)) {
			return config.ModelEntry{}, errors.New("preset not found")
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = fallbackText(strings.TrimSpace(preset.Name), strings.TrimSpace(preset.ID))
		}
		return config.ModelEntry{
			Name:                    name,
			APIBase:                 strings.TrimSpace(ai.GetAPIBase(preset.Provider, preset.APIType, "")),
			APIKey:                  strings.TrimSpace(req.APIKey),
			Model:                   strings.TrimSpace(preset.ModelName),
			Source:                  "user",
			Provider:                provider.ID,
			APIType:                 strings.TrimSpace(string(preset.APIType)),
			ThinkingCapability:      strings.TrimSpace(preset.ThinkingCap.String()),
			SupportsReasoningEffort: preset.SupportsReasoningEffort,
		}, nil

	case ModelEditKindCustomModel:
		provider := ai.GetProviderByID(strings.TrimSpace(req.ProviderID))
		if provider == nil || provider.Type == ai.ProviderCustom {
			return config.ModelEntry{}, errors.New("provider not found")
		}
		model := strings.TrimSpace(req.Model)
		if model == "" {
			return config.ModelEntry{}, errors.New("model required")
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = fallbackText(model, fmt.Sprintf("custom-model-%d", time.Now().Unix()%100000))
		}
		apiBase := strings.TrimSpace(req.APIBase)
		if apiBase == "" {
			if current, ok := existingModelByName(cfg, strings.TrimSpace(req.OriginalName)); ok {
				apiBase = strings.TrimSpace(current.APIBase)
			}
		}
		if apiBase == "" {
			apiBase = strings.TrimSpace(provider.DefaultAPIBase)
		}
		return config.ModelEntry{
			Name:                    name,
			APIBase:                 apiBase,
			APIKey:                  strings.TrimSpace(req.APIKey),
			Model:                   model,
			Source:                  "user",
			Provider:                provider.ID,
			APIType:                 inferCustomModelAPIType(provider, apiBase),
			ThinkingCapability:      strings.TrimSpace(ai.DetectThinkingCapability(model).String()),
			SupportsReasoningEffort: ai.SupportsReasoningEffort(model),
		}, nil

	case ModelEditKindCustomProvider:
		name := strings.TrimSpace(req.Name)
		apiBase := strings.TrimSpace(req.APIBase)
		model := strings.TrimSpace(req.Model)
		if apiBase == "" || model == "" {
			return config.ModelEntry{}, errors.New("api base and model required")
		}
		if name == "" {
			name = fmt.Sprintf("model-%d", time.Now().Unix()%100000)
		}
		return config.ModelEntry{
			Name:                    name,
			APIBase:                 apiBase,
			APIKey:                  strings.TrimSpace(req.APIKey),
			Model:                   model,
			Source:                  "user",
			Provider:                ai.GetProviderByID("custom").ID,
			APIType:                 "",
			ThinkingCapability:      strings.TrimSpace(ai.DetectThinkingCapability(model).String()),
			SupportsReasoningEffort: ai.SupportsReasoningEffort(model),
		}, nil
	}
	return config.ModelEntry{}, errors.New("unsupported model save mode")
}

func classifyModelEntry(entry config.ModelEntry) (providerID, apiType, presetID string, editKind ModelEditKind) {
	source := strings.ToLower(strings.TrimSpace(entry.Source))
	base := strings.TrimSpace(entry.APIBase)
	model := strings.TrimSpace(entry.Model)
	storedProviderID := normalizeProviderID(entry.Provider)
	storedAPIType := normalizeAPIType(entry.APIType)

	if source == "env" {
		editKind = ModelEditKindEnv
	}

	if exact := findExactPreset(storedProviderID, storedAPIType, base, model); exact != nil {
		providerID = providerIDFromType(exact.Provider)
		apiType = strings.TrimSpace(string(exact.APIType))
		presetID = strings.TrimSpace(exact.ID)
		if editKind == "" {
			editKind = ModelEditKindPreset
		}
		return
	}

	if storedProviderID == "custom" {
		return storedProviderID, storedAPIType, "", firstModelEditKind(editKind, ModelEditKindCustomProvider)
	}
	if storedProviderID != "" {
		if provider := ai.GetProviderByID(storedProviderID); provider != nil && provider.Type != ai.ProviderCustom {
			return storedProviderID, fallbackText(storedAPIType, inferCustomModelAPIType(provider, base)), "", firstModelEditKind(editKind, ModelEditKindCustomModel)
		}
	}

	info := ai.ResolveProviderAndModel(base, model)
	if info != nil && info.Provider != nil {
		providerID = strings.TrimSpace(info.Provider.ID)
		if exact := findExactPreset(providerID, "", base, model); exact != nil {
			apiType = strings.TrimSpace(string(exact.APIType))
			presetID = strings.TrimSpace(exact.ID)
			return providerID, apiType, presetID, firstModelEditKind(editKind, ModelEditKindPreset)
		}
		if providerID != "" && providerID != "custom" {
			return providerID, inferCustomModelAPIType(info.Provider, base), "", firstModelEditKind(editKind, ModelEditKindCustomModel)
		}
	}

	if providerID == "" {
		providerID = "custom"
	}
	return providerID, storedAPIType, "", firstModelEditKind(editKind, ModelEditKindCustomProvider)
}

func firstModelEditKind(current, fallback ModelEditKind) ModelEditKind {
	if current != "" {
		return current
	}
	return fallback
}

func normalizeModelEditKind(kind ModelEditKind) ModelEditKind {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case string(ModelEditKindPreset):
		return ModelEditKindPreset
	case string(ModelEditKindCustomModel):
		return ModelEditKindCustomModel
	case string(ModelEditKindCustomProvider):
		return ModelEditKindCustomProvider
	case string(ModelEditKindEnv):
		return ModelEditKindEnv
	default:
		return ""
	}
}

func normalizeProviderID(providerID string) string {
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	if providerID == "" {
		return ""
	}
	if provider := ai.GetProviderByID(providerID); provider != nil {
		return strings.TrimSpace(provider.ID)
	}
	return providerID
}

func normalizeAPIType(apiType string) string {
	switch strings.ToLower(strings.TrimSpace(apiType)) {
	case string(ai.APITypeCodePlan):
		return string(ai.APITypeCodePlan)
	case string(ai.APITypeClaude):
		return string(ai.APITypeClaude)
	case string(ai.APITypeStandard):
		return string(ai.APITypeStandard)
	default:
		return ""
	}
}

func findExactPreset(providerID, apiType, apiBase, model string) *ai.ModelCatalogEntry {
	providerID = normalizeProviderID(providerID)
	apiType = normalizeAPIType(apiType)
	apiBase = strings.TrimSpace(apiBase)
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}

	for _, preset := range ai.GetAllModels() {
		if preset == nil {
			continue
		}
		currentProviderID := providerIDFromType(preset.Provider)
		if providerID != "" && !strings.EqualFold(providerID, currentProviderID) {
			continue
		}
		if !modelMatchesPreset(preset, model) {
			continue
		}
		expectedAPIType := strings.TrimSpace(string(preset.APIType))
		expectedBase := strings.TrimSpace(ai.GetAPIBase(preset.Provider, preset.APIType, ""))
		if apiType != "" {
			if !strings.EqualFold(apiType, expectedAPIType) {
				continue
			}
			if apiBase != "" && expectedBase != "" && !sameAPIBase(apiBase, expectedBase) {
				continue
			}
			return preset
		}
		if apiBase != "" && expectedBase != "" && sameAPIBase(apiBase, expectedBase) {
			return preset
		}
	}
	return nil
}

func modelMatchesPreset(preset *ai.ModelCatalogEntry, model string) bool {
	if preset == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(preset.ModelName), model) || strings.EqualFold(strings.TrimSpace(preset.ID), model)
}

func sameAPIBase(left, right string) bool {
	return normalizeAPIBase(left) == normalizeAPIBase(right)
}

func normalizeAPIBase(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, "/")
	return value
}

func providerIDFromType(providerType ai.ProviderType) string {
	if provider := ai.GetProvider(providerType); provider != nil {
		return strings.TrimSpace(provider.ID)
	}
	return strings.TrimSpace(string(providerType))
}

func inferCustomModelAPIType(provider *ai.ProviderConfig, apiBase string) string {
	if provider == nil {
		return ""
	}
	apiBase = strings.TrimSpace(apiBase)
	if apiBase != "" {
		if provider.ClaudeAPIBase != "" && sameAPIBase(apiBase, provider.ClaudeAPIBase) {
			return string(ai.APITypeClaude)
		}
		if provider.CodePlanAPIBase != "" && sameAPIBase(apiBase, provider.CodePlanAPIBase) {
			return string(ai.APITypeCodePlan)
		}
	}
	return string(ai.APITypeStandard)
}

func existingModelByName(cfg config.Config, name string) (config.ModelEntry, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return config.ModelEntry{}, false
	}
	for _, item := range cfg.Models {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return item, true
		}
	}
	return config.ModelEntry{}, false
}

func fallbackModelSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "user"
	}
	return source
}
