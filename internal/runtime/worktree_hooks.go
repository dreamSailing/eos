package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"strings"
)

func (rt *EinoRuntime) EmitWorktreeCreate(ctx context.Context, path string) {
	if rt == nil || rt.dispatchTools == nil || rt.dispatchTools.hookMgr == nil {
		return
	}
	dec, _ := rt.dispatchTools.hookMgr.WorktreeCreate(ctx, path)
	if strings.TrimSpace(dec.AdditionalContext) != "" && rt.ctxm != nil {
		rt.ctxm.AddEphemeral(strings.TrimSpace(dec.AdditionalContext))
	}
}

func (rt *EinoRuntime) EmitWorktreeRemove(ctx context.Context, path string) {
	if rt == nil || rt.dispatchTools == nil || rt.dispatchTools.hookMgr == nil {
		return
	}
	dec, _ := rt.dispatchTools.hookMgr.WorktreeRemove(ctx, path)
	if strings.TrimSpace(dec.AdditionalContext) != "" && rt.ctxm != nil {
		rt.ctxm.AddEphemeral(strings.TrimSpace(dec.AdditionalContext))
	}
}

