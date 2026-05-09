package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"log/slog"
	"runtime"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func ExecuteWithStdin(ctx context.Context, command string, workingDir string, stdin string) (stdout, stderr string, exitCode int, err error) {
	return ExecuteWithStdinEnv(ctx, command, workingDir, stdin, nil)
}

func ExecuteWithStdinEnv(ctx context.Context, command string, workingDir string, stdin string, env []string) (stdout, stderr string, exitCode int, err error) {
	stdoutStr, stderrStr, exitCode, err := executeNativeShellCommand(ctx, ShellTypeDefault, command, workingDir, stdin, env)

	if err != nil {
		slog.Debug("shell.execute_with_stdin.error", "component", utils.ComponentTool,
			"command", command,
			"working_dir", workingDir,
			"os", runtime.GOOS,
			"error", err.Error(),
			"stdout_length", len(stdoutStr),
			"stderr_length", len(stderrStr),
		)
		return stdoutStr, stderrStr, exitCode, err
	}
	return stdoutStr, stderrStr, exitCode, nil
}
