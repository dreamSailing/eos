package runtime

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

