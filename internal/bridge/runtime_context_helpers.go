package bridge

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
		return tools.WithWorkspaceRoot(ctx, root)
	}
	return ctx
}
