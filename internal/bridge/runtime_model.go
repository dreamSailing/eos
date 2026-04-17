package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/config"
)

// SaveModelConfig 保存模型配置
func (rc *RuntimeCore) SaveModelConfig(cfg config.Config, path string) error {
	return config.Save(cfg, path)
}

// LoadFullModelConfig 加载完整模型配置
func (rc *RuntimeCore) LoadFullModelConfig() (config.Config, string) {
	return config.Load()
}

// SetActiveModel 设置活动模型
func (rc *RuntimeCore) SetActiveModel(name string) bool {
	cfg, p := config.Load()
	if !config.SetActive(&cfg, name) {
		return false
	}
	_ = config.Save(cfg, p)
	return true
}

// AddModel 添加模型
func (rc *RuntimeCore) AddModel(entry config.ModelEntry) bool {
	cfg, p := config.Load()
	if !config.AddModel(&cfg, entry) {
		return false
	}
	_ = config.Save(cfg, p)
	return true
}

// UpdateModel 更新模型
func (rc *RuntimeCore) UpdateModel(entry config.ModelEntry) bool {
	cfg, p := config.Load()
	if !config.UpdateModel(&cfg, entry) {
		return false
	}
	_ = config.Save(cfg, p)
	return true
}

// DeleteModel 删除模型
func (rc *RuntimeCore) DeleteModel(name string) bool {
	cfg, p := config.Load()
	if !config.DeleteModel(&cfg, name) {
		return false
	}
	_ = config.Save(cfg, p)
	return true
}

// GetActiveModel 获取活动模型
func (rc *RuntimeCore) GetActiveModel() (config.ModelEntry, bool) {
	cfg, _ := config.Load()
	return config.ActiveModel(cfg)
}

// SyncEnvModel 同步环境变量模型
func (rc *RuntimeCore) SyncEnvModel() bool {
	base := os.Getenv("EOS_API_BASE")
	key := os.Getenv("EOS_API_KEY")
	model := os.Getenv("EOS_MODEL")
	if strings.TrimSpace(base) == "" || strings.TrimSpace(key) == "" {
		return false
	}
	if strings.TrimSpace(model) == "" {
		if m := config.InferDefaultModel(base); m != "" {
			model = m
		}
	}
	cfg, p := config.Load()
	found := false
	for i := range cfg.Models {
		if cfg.Models[i].Source == "env" {
			cfg.Models[i] = config.ModelEntry{Name: "env", APIBase: base, APIKey: key, Model: model, Source: "env"}
			found = true
			break
		}
	}
	if !found {
		cfg.Models = append(cfg.Models, config.ModelEntry{Name: "env", APIBase: base, APIKey: key, Model: model, Source: "env"})
	}
	cfg.Active = "env"
	_ = config.Save(cfg, p)
	return true
}

// GetContextWindowTokens 获取上下文窗口 Token 数
func (rc *RuntimeCore) GetContextWindowTokens() int {
	return ai.ContextWindowTokens(rc.ModelName())
}

// ResolveAPIConfig 解析 API 配置
func (rc *RuntimeCore) ResolveAPIConfig() (string, string, string, string) {
	base := os.Getenv("EOS_API_BASE")
	key := os.Getenv("EOS_API_KEY")
	model := os.Getenv("EOS_MODEL")
	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".eos.json")
	if base == "" || key == "" || model == "" {
		if m, ok := rc.GetActiveModel(); ok {
			if base == "" {
				base = m.APIBase
			}
			if key == "" {
				key = m.APIKey
			}
			if model == "" {
				model = m.Model
			}
		}
	}
	if model == "" && base != "" {
		if m := config.InferDefaultModel(base); m != "" {
			model = m
		}
	}
	return base, key, model, cfgPath
}
