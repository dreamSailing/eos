package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"path/filepath"
	"strings"
)

func gitMutationSandboxResult(ctx context.Context, toolName, repoRoot string) (ToolResult, bool) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return ToolResult{}, false
	}
	if err := sandboxWorkspaceWriteError(ctx, repoRoot); err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    toolName,
			Status:  "error",
			Error:   err.Error(),
			Display: "错误：" + err.Error(),
			Data: map[string]interface{}{
				"repo_root": filepath.ToSlash(repoRoot),
			},
		}, true
	}
	return ToolResult{}, false
}
