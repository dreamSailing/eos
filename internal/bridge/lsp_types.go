//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


type LSPServerInfo struct {
	Language string
	Command  string
	Found    bool
}

type LSPStatus struct {
	Enabled          bool
	AutoDetect       bool
	ConfigFile       string
	Workspace        string
	DetectedLanguage string
	ActiveLanguage   string
	ActiveServer     string
	ActiveRoot       string
	Servers          []LSPServerInfo
	Message          string
}

