package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"strings"

	"github.com/dreamSailing/eos/internal/tools"
)

func (rc *RuntimeCore) withWorkspaceRoot(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if root := strings.TrimSpace(rc.workingRoot()); root != "" {
		ctx = tools.WithWorkspaceRoot(ctx, root)
	}
	if rc != nil && rc.securityMgr != nil {
		ctx = tools.WithAccessMode(ctx, rc.securityMgr.AccessMode())
		ctx = tools.WithApprovalMode(ctx, rc.securityMgr.ApprovalMode())
	}
	return ctx
}
