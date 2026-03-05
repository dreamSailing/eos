package config

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
	base := os.Getenv("VB_API_BASE")
	key := os.Getenv("VB_API_KEY")
	model := os.Getenv("VB_MODEL")
	if base == "" || key == "" || model == "" {
		if entry, ok := m.GetActiveModel(); ok {
			if base == "" {
				base = entry.APIBase
			}
			if key == "" {
				key = entry.APIKey
			}
			if model == "" {
				model = entry.Model
			}
		}
	}
	if model == "" && base != "" {
		if inferred := InferDefaultModel(base); inferred != "" {
			model = inferred
		}
	}
	return base, key, model, m.path
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
	base := os.Getenv("VB_API_BASE")
	key := os.Getenv("VB_API_KEY")
	model := os.Getenv("VB_MODEL")
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
			cfg.Models[i] = ModelEntry{Name: "env", APIBase: base, APIKey: key, Model: model, Source: "env"}
			found = true
			break
		}
	}
	if !found {
		cfg.Models = append(cfg.Models, ModelEntry{Name: "env", APIBase: base, APIKey: key, Model: model, Source: "env"})
	}
	cfg.Active = "env"
	_ = Save(cfg, p)
	return "env", true
}
