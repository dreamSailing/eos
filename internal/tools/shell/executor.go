package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"io"
	"time"
)

type ShellType string

const (
	ShellTypeDefault    ShellType = "default"
	ShellTypeBash       ShellType = "bash"
	ShellTypePowerShell ShellType = "powershell"
)

type Executor interface {
	Execute(ctx context.Context, command string, workingDir string) (stdout, stderr string, err error)
}

type ExecutorFactory func() Executor

var defaultExecutor ExecutorFactory = NewMvdanExecutor

func SetDefaultExecutor(fn ExecutorFactory) {
	defaultExecutor = fn
}

func GetDefaultExecutor() Executor {
	return defaultExecutor()
}

type ExecuteOptions struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Env        []string
	WorkingDir string
	Timeout    time.Duration
}

type AsyncSession interface {
	ID() string
	Output() (stdout, stderr string, done bool, err error)
	Kill() error
	Wait(ctx context.Context) error
}

type AsyncExecutor interface {
	StartAsync(ctx context.Context, command string, opts *ExecuteOptions) (AsyncSession, error)
}

type DirectExecutor interface {
	ExecuteDirect(ctx context.Context, name string, args []string, opts *ExecuteOptions) (stdout, stderr string, err error)
}

type CompositeExecutor interface {
	Executor
	AsyncExecutor
	DirectExecutor
}

func Execute(ctx context.Context, command string, workingDir string) (stdout, stderr string, err error) {
	return GetDefaultExecutor().Execute(ctx, command, workingDir)
}

func ExecuteWithTimeout(command string, workingDir string, timeout time.Duration) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Execute(ctx, command, workingDir)
}

func ExecuteDirect(ctx context.Context, name string, args []string, opts *ExecuteOptions) (stdout, stderr string, err error) {
	if executor, ok := GetDefaultExecutor().(DirectExecutor); ok {
		return executor.ExecuteDirect(ctx, name, args, opts)
	}

	var cmd string
	if len(args) > 0 {
		cmd = name + " " + joinArgs(args)
	} else {
		cmd = name
	}

	workingDir := ""
	if opts != nil {
		workingDir = opts.WorkingDir
	}
	return Execute(ctx, cmd, workingDir)
}

func joinArgs(args []string) string {
	var result string
	for _, arg := range args {
		if containsSpace(arg) {
			result += `"` + arg + `" `
		} else {
			result += arg + " "
		}
	}
	return result[:len(result)-1]
}

func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '"' || c == '\'' {
			return true
		}
	}
	return false
}
