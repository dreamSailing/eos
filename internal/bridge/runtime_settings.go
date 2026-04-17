package bridge

import (
	"github.com/dreamSailing/eos/internal/pkg/settings"
)

// LoadSettings 加载设置
func (rc *RuntimeCore) LoadSettings(path string) (*settings.Settings, error) {
	rc.settingsMgr.SetPath(path)
	s, err := rc.settingsMgr.Load()
	if err == nil && s != nil {
		rc.mu.Lock()
		rc.settings = *s
		rc.mu.Unlock()
	}
	return s, err
}

// SaveSettings 保存设置
func (rc *RuntimeCore) SaveSettings(path string, s *settings.Settings) error {
	rc.settingsMgr.SetPath(path)
	if s != nil {
		rc.mu.Lock()
		rc.settings = *s
		rc.mu.Unlock()
	}
	return rc.settingsMgr.Save(s)
}

// GetSettings 获取当前设置
func (rc *RuntimeCore) GetSettings() settings.Settings {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.settings
}

// GetSettingsManager 获取设置管理器
func (rc *RuntimeCore) GetSettingsManager() *settings.Manager {
	return rc.settingsMgr
}
