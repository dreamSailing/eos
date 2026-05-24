package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dreamSailing/eos/pkg/sandbox"
)

func sandboxPolicyForToolContext(ctx context.Context) sandbox.Policy {
	mode := sandbox.ModeDangerFullAccess
	switch normalizeAccessMode(AccessModeFromContext(ctx)) {
	case "read-only":
		mode = sandbox.ModeReadOnly
	case "workspace-write":
		mode = sandbox.ModeWorkspaceWrite
	}
	root := WorkspaceRootFromContext(ctx)
	if mode == sandbox.ModeWorkspaceWrite && strings.TrimSpace(root) == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	return sandbox.Policy{
		Mode:          mode,
		WorkspaceRoot: root,
		WritableRoots: allowedTemporaryDirs(),
		Network:       sandbox.NetworkDeny,
	}.Normalized()
}

func (m *Manager) runSandboxedCommand(ctx context.Context, argv []string, exec func() (string, error)) (string, error) {
	result := sandbox.NewGuardedRunner(func([]string, sandbox.Policy) sandbox.Result {
		out, err := exec()
		if err != nil {
			return sandbox.Result{Stdout: out, Stderr: err.Error(), ExitCode: 1, Err: err}
		}
		return sandbox.Result{Stdout: out}
	}).Run(argv, sandboxPolicyForToolContext(ctx))
	return result.Stdout, result.Err
}

func sandboxWriteError(ctx context.Context, target string) error {
	return sandboxWriteErrorForPolicy(sandboxPolicyForToolContext(ctx), target)
}

func sandboxWorkspaceWriteError(ctx context.Context, target string) error {
	policy := sandboxPolicyForToolContext(ctx)
	policy.WritableRoots = nil
	return sandboxWriteErrorForPolicy(policy.Normalized(), target)
}

func sandboxWriteErrorForPolicy(policy sandbox.Policy, target string) error {
	ok, err := policy.AllowsWrite(target)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	switch policy.Mode {
	case sandbox.ModeReadOnly:
		return fmt.Errorf("sandbox policy read-only blocks writes: %s", filepath.ToSlash(target))
	case sandbox.ModeWorkspaceWrite:
		return fmt.Errorf("sandbox policy workspace-write blocks writes outside workspace: %s", filepath.ToSlash(target))
	default:
		return fmt.Errorf("sandbox policy %s blocks writes: %s", policy.Mode, filepath.ToSlash(target))
	}
}
