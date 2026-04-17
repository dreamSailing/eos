package runtime

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
