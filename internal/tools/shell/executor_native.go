package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/dreamSailing/eos/internal/pkg/utils"
	"mvdan.cc/sh/v3/interp"
)

type NativeExecutor struct{}

func NewNativeExecutor() Executor {
	return &NativeExecutor{}
}

func (e *NativeExecutor) Execute(ctx context.Context, command string, workingDir string) (stdout, stderr string, err error) {
	stdoutStr, stderrStr, _, err := executeNativeShellCommand(ctx, ShellTypeDefault, command, workingDir, "", nil)

	if err != nil {
		slog.Error("shell.native.execute.error", "component", utils.ComponentTool,
			"command", command,
			"working_dir", workingDir,
			"os", runtime.GOOS,
			"error", err.Error(),
			"stdout_length", len(stdoutStr),
			"stderr_length", len(stderrStr),
		)
		return stdoutStr, stderrStr, err
	}

	slog.Debug("shell.native.execute.success", "component", utils.ComponentTool,
		"command", command,
		"working_dir", workingDir,
		"os", runtime.GOOS,
		"stdout_length", len(stdoutStr),
		"stderr_length", len(stderrStr),
	)

	return stdoutStr, stderrStr, nil
}

func (e *NativeExecutor) ExecuteDirect(ctx context.Context, name string, args []string, opts *ExecuteOptions) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)

	workingDir := ""
	if opts != nil {
		workingDir = opts.WorkingDir
		if opts.Env != nil {
			cmd.Env = opts.Env
		}
	}

	if workingDir != "" {
		cmd.Dir = workingDir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func executeNativeShellCommand(ctx context.Context, shellType ShellType, command string, workingDir string, stdin string, extraEnv []string) (stdout, stderr string, exitCode int, err error) {
	cmd, err := buildNativeShellCommand(ctx, shellType, command, workingDir)
	if err != nil {
		return "", "", 1, err
	}

	cmd.Env = mergeEnv(envFromContext(ctx), extraEnv)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()
	exitCode = 0
	if err != nil {
		exitCode = 1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		}
	}
	if st := cmd.ProcessState; st != nil && exitCode == 0 {
		exitCode = st.ExitCode()
	}
	if exitCode < 0 {
		exitCode = 0
	}
	return stdoutStr, stderrStr, exitCode, err
}

func buildNativeShellCommand(ctx context.Context, shellType ShellType, command string, workingDir string) (*exec.Cmd, error) {
	name, args, err := resolveShellCommand(shellType, command)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if workingDir != "" {
		cmd.Dir = workingDir
	}
	return cmd, nil
}

func resolveShellCommand(shellType ShellType, command string) (string, []string, error) {
	switch shellType {
	case ShellTypeBash:
		bashPath, err := exec.LookPath("bash")
		if err != nil {
			if runtime.GOOS == "windows" {
				return "", nil, fmt.Errorf("bash.exe not found; please install Git Bash, WSL bash, or add bash.exe to PATH")
			}
			return "", nil, fmt.Errorf("bash not found in PATH")
		}
		return bashPath, []string{"-lc", command}, nil
	case ShellTypePowerShell:
		name, args := resolvePowerShellCommand(command)
		return name, args, nil
	default:
		if runtime.GOOS == "windows" {
			name, args := resolvePowerShellCommand(withPowerShellUTF8Preamble(command))
			return name, args, nil
		}
		return "sh", []string{"-c", command}, nil
	}
}

func resolvePowerShellCommand(command string) (string, []string) {
	psCmd := "pwsh"
	args := []string{"-NoLogo", "-NoProfile", "-NonInteractive"}
	if runtime.GOOS == "windows" {
		psCmd = "powershell"
		args = append(args, "-ExecutionPolicy", "Bypass")
	}
	args = append(args, "-Command", command)
	return psCmd, args
}

func withPowerShellUTF8Preamble(command string) string {
	return "[Console]::InputEncoding=[System.Text.UTF8Encoding]::new();" +
		" [Console]::OutputEncoding=[System.Text.UTF8Encoding]::new();" +
		" $OutputEncoding=[System.Text.UTF8Encoding]::new();" +
		" chcp 65001 > $null;" +
		" $ErrorActionPreference='SilentlyContinue'; " + command
}

type FallbackExecutor struct {
	primary   Executor
	fallback  Executor
	shouldUse func(command string, err error) bool
}

func NewFallbackExecutor(primary, fallback Executor) Executor {
	return &FallbackExecutor{
		primary:  primary,
		fallback: fallback,
		shouldUse: func(command string, err error) bool {
			if err == nil {
				return false
			}

			var exitStatus interp.ExitStatus
			if errors.As(err, &exitStatus) {
				return false
			}

			errStr := strings.ToLower(err.Error())
			unsupported := []string{
				"not implemented",
				"not supported",
				"unknown command",
				"not found",
				"no such file",
			}
			for _, u := range unsupported {
				if strings.Contains(errStr, u) {
					return true
				}
			}
			return false
		},
	}
}

func (e *FallbackExecutor) Execute(ctx context.Context, command string, workingDir string) (stdout, stderr string, err error) {
	stdout, stderr, err = e.primary.Execute(ctx, command, workingDir)
	if err != nil && e.shouldUse(command, err) {
		slog.Info("shell.fallback.using_native", "component", utils.ComponentTool,
			"command", command,
			"primary_error", err.Error(),
		)
		return e.fallback.Execute(ctx, command, workingDir)
	}
	return stdout, stderr, err
}

func (e *FallbackExecutor) ExecuteDirect(ctx context.Context, name string, args []string, opts *ExecuteOptions) (stdout, stderr string, err error) {
	if primary, ok := e.primary.(DirectExecutor); ok {
		stdout, stderr, err = primary.ExecuteDirect(ctx, name, args, opts)
		if err != nil && e.shouldUse(name, err) {
			slog.Info("shell.fallback.using_native_direct", "component", utils.ComponentTool,
				"command", name,
				"primary_error", err.Error(),
			)
			if fallback, ok := e.fallback.(DirectExecutor); ok {
				return fallback.ExecuteDirect(ctx, name, args, opts)
			}
		}
		return stdout, stderr, err
	}

	if fallback, ok := e.fallback.(DirectExecutor); ok {
		return fallback.ExecuteDirect(ctx, name, args, opts)
	}

	cmd := name
	if len(args) > 0 {
		cmd = name + " " + strings.Join(args, " ")
	}
	workingDir := ""
	if opts != nil {
		workingDir = opts.WorkingDir
	}
	return e.Execute(ctx, cmd, workingDir)
}

func init() {
	fallback := NewNativeExecutor()
	primary := NewMvdanExecutor()
	SetDefaultExecutor(func() Executor {
		return NewFallbackExecutor(primary, fallback)
	})
}

func SetupExecutor(useMvdanOnly bool) {
	if useMvdanOnly {
		SetDefaultExecutor(func() Executor {
			return NewMvdanExecutor()
		})
	} else {
		SetDefaultExecutor(func() Executor {
			return NewFallbackExecutor(NewMvdanExecutor(), NewNativeExecutor())
		})
	}
}

func GetEnvFromOS() []string {
	return os.Environ()
}
