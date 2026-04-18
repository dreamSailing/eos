package ui

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


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
