package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
)

type UserConfirmRequest struct {
	Title     string
	Question  string
	Options   []string
	AllowText bool
	TextHint  string
}

type UserConfirmResponse struct {
	Confirmed   bool
	Option      string
	OptionIndex int
	Text        string
}

var UserConfirmPrompt func(ctx context.Context, req UserConfirmRequest) (UserConfirmResponse, error)

