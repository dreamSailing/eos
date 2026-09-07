package hooks

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

type Config struct {
	Hooks              map[string][]MatcherGroup `json:"hooks" yaml:"hooks"`
	DisableAllHooks    bool                      `json:"disableAllHooks" yaml:"disableAllHooks"`
	ManagedHooksOnly   bool                      `json:"managedHooksOnly" yaml:"managedHooksOnly"`
	EnabledHookSources []string                  `json:"enabledHookSources" yaml:"enabledHookSources"`
}

type MatcherGroup struct {
	Matcher    string    `json:"matcher" yaml:"matcher"`
	Hooks      []Handler `json:"hooks" yaml:"hooks"`
	Source     string    `json:"-" yaml:"-"`
	BaseDir    string    `json:"-" yaml:"-"`
	SourcePath string    `json:"-" yaml:"-"`
}

type Handler struct {
	Type          string `json:"type" yaml:"type"`
	Command       string `json:"command" yaml:"command"`
	Prompt        string `json:"prompt" yaml:"prompt"`
	Model         string `json:"model" yaml:"model"`
	Timeout       int    `json:"timeout" yaml:"timeout"`
	Async         bool   `json:"async" yaml:"async"`
	StatusMessage string `json:"statusMessage" yaml:"statusMessage"`
	Once          bool   `json:"once" yaml:"once"`
}

type Decision struct {
	Decision          string
	Reason            string
	AdditionalContext string
	UpdatedInput      map[string]any
	AllowSession      bool
}
