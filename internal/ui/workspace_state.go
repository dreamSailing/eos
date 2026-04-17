package ui

import "github.com/dreamSailing/eos/internal/config"

func rememberKnownWorkspace(path string, foreground bool) {
	cfg, cfgPath := config.Load()
	if !config.RememberWorkspace(&cfg, path, foreground) {
		return
	}
	_ = config.Save(cfg, cfgPath)
}

func forgetKnownWorkspace(path string) {
	cfg, cfgPath := config.Load()
	if !config.ForgetWorkspace(&cfg, path) {
		return
	}
	_ = config.Save(cfg, cfgPath)
}
