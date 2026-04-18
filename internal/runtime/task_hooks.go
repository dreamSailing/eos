package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"strings"
)

func (rt *EinoRuntime) EmitTaskCompleted(ctx context.Context, task string, success bool, errorMsg string, meta map[string]any) error {
	if rt == nil || rt.dispatchTools == nil || rt.dispatchTools.hookMgr == nil {
		return nil
	}
	dec, _ := rt.dispatchTools.hookMgr.TaskCompletedDetailed(ctx, task, success, errorMsg, meta)
	if strings.TrimSpace(dec.AdditionalContext) != "" && rt.ctxm != nil {
		rt.ctxm.AddEphemeral(strings.TrimSpace(dec.AdditionalContext))
	}
	return nil
}

