package config

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"os"
	"strings"
)

// Manager 负责模型配置的高层管理
type Manager struct {
	path string
}

func NewManager(path string) *Manager {
	if path == "" {
		path = Path()
	}
	return &Manager{path: path}
}

func (m *Manager) ResolveAPIConfig() (string, string, string, string) {
	cfg, p := Load()
	if entry, ok := ActiveModel(cfg); ok {
		return strings.TrimSpace(entry.APIBase),
			strings.TrimSpace(entry.APIKey),
			strings.TrimSpace(entry.Model),
			p
	}
	return "", "", "", p
}

func (m *Manager) ResolveCapabilityModelConfig(capability string) (ModelEntry, string, bool) {
	cfg, _ := Load()
	if active, ok := ActiveModel(cfg); ok && SupportsCapability(active, capability) {
		return active, "primary", true
	}
	if entry, ok := ResolveCapabilityModel(cfg, capability); ok {
		return entry, "capability_model", true
	}
	return ModelEntry{}, "", false
}

func (m *Manager) Load() (Config, string) {
	return Load()
}

func (m *Manager) Save(cfg Config) error {
	return Save(cfg, m.path)
}

func (m *Manager) ListModels() ([]ModelEntry, string, string) {
	cfg, p := Load()
	return cfg.Models, cfg.Active, p
}

func (m *Manager) SetActiveModel(name string) bool {
	cfg, p := Load()
	if !SetActive(&cfg, name) {
		return false
	}
	_ = Save(cfg, p)
	return true
}

func (m *Manager) AddModel(entry ModelEntry) bool {
	cfg, p := Load()
	if !AddModel(&cfg, entry) {
		return false
	}
	_ = Save(cfg, p)
	return true
}

func (m *Manager) UpdateModel(entry ModelEntry) bool {
	cfg, p := Load()
	if !UpdateModel(&cfg, entry) {
		return false
	}
	_ = Save(cfg, p)
	return true
}

func (m *Manager) DeleteModel(name string) bool {
	cfg, p := Load()
	if !DeleteModel(&cfg, name) {
		return false
	}
	_ = Save(cfg, p)
	return true
}

func (m *Manager) GetActiveModel() (ModelEntry, bool) {
	cfg, _ := Load()
	return ActiveModel(cfg)
}

func (m *Manager) SyncEnvModel() (string, bool) {
	base := os.Getenv("EOS_API_BASE")
	key := os.Getenv("EOS_API_KEY")
	model := os.Getenv("EOS_MODEL")
	if strings.TrimSpace(base) == "" || strings.TrimSpace(key) == "" {
		return "", false
	}
	if strings.TrimSpace(model) == "" {
		if inferred := InferDefaultModel(base); inferred != "" {
			model = inferred
		}
	}
	cfg, p := Load()
	found := false
	for i := range cfg.Models {
		if cfg.Models[i].Source == "env" {
			cfg.Models[i] = ModelEntry{Name: "env", APIBase: base, APIKey: key, Model: model, Source: "env", Role: ModelRolePrimary, Enabled: boolPtr(true)}
			found = true
			break
		}
	}
	if !found {
		cfg.Models = append(cfg.Models, ModelEntry{Name: "env", APIBase: base, APIKey: key, Model: model, Source: "env", Role: ModelRolePrimary, Enabled: boolPtr(true)})
	}
	cfg.Active = "env"
	_ = Save(cfg, p)
	return "env", true
}
