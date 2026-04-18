package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"strings"

	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools"
)

func envInfoForContext(ctx context.Context) utils.EnvInfo {
	info := utils.GetEnvInfo()
	if root := strings.TrimSpace(tools.WorkspaceRootFromContext(ctx)); root != "" {
		info.CWD = root
	}
	return info
}
