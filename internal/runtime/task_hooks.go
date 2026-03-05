package runtime

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

