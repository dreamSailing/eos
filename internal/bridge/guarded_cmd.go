package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"

	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/pkg/sandbox"
)

// guardedGitCmd is the single process-execution entry point for all bridge
// git/worktree operations. Every git command issued by the bridge package MUST
// flow through this helper so that sandbox.GuardedRunner pre-checks are applied
// consistently and the architecture boundary test only sees one call site.
func (rc *RuntimeCore) guardedGitCmd(ctx context.Context, policy sandbox.Policy, dir string, args ...string) sandbox.Result {
	argv := append([]string{"git"}, args...)
	return sandbox.NewGuardedRunner(func(_ []string, _ sandbox.Policy) sandbox.Result {
		cmd := utils.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return sandbox.Result{Stderr: string(output), ExitCode: 1, Err: err}
		}
		return sandbox.Result{Stdout: string(output)}
	}).Run(argv, policy)
}
