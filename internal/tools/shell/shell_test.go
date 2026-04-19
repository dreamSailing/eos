package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMvdanExecutor_Execute(t *testing.T) {
	executor := NewMvdanExecutor()
	ctx := context.Background()

	tests := []struct {
		name      string
		command   string
		wantErr   bool
		contains  string
		skipOnWin bool
	}{
		{
			name:     "echo simple",
			command:  "echo hello world",
			wantErr:  false,
			contains: "hello world",
		},
		{
			name:     "variable expansion",
			command:  "VAR=hello; echo $VAR",
			wantErr:  false,
			contains: "hello",
		},
		{
			name:     "command substitution",
			command:  "echo $(echo nested)",
			wantErr:  false,
			contains: "nested",
		},
		{
			name:      "ls simple",
			command:   "ls",
			wantErr:   false,
			skipOnWin: true,
		},
		{
			name:      "ls with path",
			command:   "ls .",
			wantErr:   false,
			skipOnWin: true,
		},
		{
			name:      "ls -la",
			command:   "ls -la",
			wantErr:   false,
			skipOnWin: true,
		},
		{
			name:      "cat from stdin",
			command:   "echo test | cat",
			wantErr:   false,
			contains:  "test",
			skipOnWin: true,
		},
		{
			name:     "pipe chain with echo",
			command:  "echo hello",
			wantErr:  false,
			contains: "hello",
		},
		{
			name:      "mkdir and rmdir",
			command:   "mkdir -p /tmp/test_shell_mvdan && rmdir /tmp/test_shell_mvdan",
			wantErr:   false,
			skipOnWin: true,
		},
		{
			name:    "nonexistent command",
			command: "nonexistent_command_xyz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnWin && runtime.GOOS == "windows" {
				t.Skip("skipping on Windows - Unix command not available")
			}

			stdout, stderr, err := executor.Execute(ctx, tt.command, "")

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v, stderr: %s", err, stderr)
				return
			}

			if tt.contains != "" && !strings.Contains(stdout, tt.contains) {
				t.Errorf("stdout = %q, want to contain %q", stdout, tt.contains)
			}
		})
	}
}

func TestShell_ExecuteCtx(t *testing.T) {
	shell := NewShell()
	ctx := context.Background()

	stdout, err := shell.ExecuteCtx(ctx, "echo test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "test") {
		t.Errorf("stdout = %q, want to contain 'test'", stdout)
	}
}

func TestShell_ExecuteWithWorkingDir(t *testing.T) {
	shell := NewShell()

	stdout, err := shell.ExecuteWithWorkingDir("pwd", "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "tmp") {
		t.Errorf("stdout = %q, want to contain 'tmp'", stdout)
	}
}

func TestFallbackExecutor(t *testing.T) {
	primary := NewMvdanExecutor()
	fallback := NewNativeExecutor()
	executor := NewFallbackExecutor(primary, fallback)
	ctx := context.Background()

	stdout, stderr, err := executor.Execute(ctx, "echo fallback_test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v, stderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "fallback_test") {
		t.Errorf("stdout = %q, want to contain 'fallback_test'", stdout)
	}
}

func TestShell_FallbackOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("this test is for Windows only")
	}

	shell := NewShell()
	ctx := context.Background()

	stdout, err := shell.ExecuteCtx(ctx, "ls")
	if err != nil {
		t.Fatalf("expected fallback to work, got error: %v", err)
	}
	if stdout == "" {
		t.Error("expected some output from ls")
	}
}

func TestNewShellWithExecutor(t *testing.T) {
	executor := NewMvdanExecutor()
	shell := NewShellWithExecutor(executor)
	ctx := context.Background()

	stdout, err := shell.ExecuteCtx(ctx, "echo custom_executor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "custom_executor") {
		t.Errorf("stdout = %q, want to contain 'custom_executor'", stdout)
	}
}

func TestSetExecutor(t *testing.T) {
	shell := NewShell()
	ctx := context.Background()

	shell.SetExecutor(NewMvdanExecutor())

	stdout, err := shell.ExecuteCtx(ctx, "echo set_executor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "set_executor") {
		t.Errorf("stdout = %q, want to contain 'set_executor'", stdout)
	}
}

func TestShell_StartAsyncWithWorkingDir(t *testing.T) {
	shell := NewShell()
	dir := t.TempDir()

	id, err := shell.StartAsyncWithWorkingDir("pwd", dir)
	if err != nil {
		t.Fatalf("StartAsyncWithWorkingDir error: %v", err)
	}
	t.Cleanup(func() { _ = shell.Kill(id) })

	var stdout string
	for i := 0; i < 20; i++ {
		var done bool
		stdout, _, done, err = shell.Output(id)
		if err != nil {
			t.Fatalf("Output error: %v", err)
		}
		if done {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if !strings.Contains(strings.ReplaceAll(stdout, "\\", "/"), strings.ReplaceAll(dir, "\\", "/")) {
		t.Fatalf("expected output %q to contain working dir %q", stdout, dir)
	}
}

func TestShell_ExecuteWithWorkingDirIncludesPluginBinInPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	workspace := t.TempDir()
	pluginRoot := filepath.Join(workspace, ".claude", "plugins", "formatter")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"formatter","description":"Format files"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	commandName := "plugin-echo"
	commandPath := filepath.Join(pluginRoot, "bin", commandName)
	commandBody := "#!/bin/sh\necho plugin-bin-ok\n"
	if runtime.GOOS == "windows" {
		commandPath += ".cmd"
		commandBody = "@echo off\r\necho plugin-bin-ok\r\n"
	}
	if err := os.WriteFile(commandPath, []byte(commandBody), 0o755); err != nil {
		t.Fatalf("write plugin command: %v", err)
	}

	shell := NewShell()
	out, err := shell.ExecuteWithWorkingDir(commandName, workspace)
	if err != nil {
		t.Fatalf("ExecuteWithWorkingDir error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "plugin-bin-ok") {
		t.Fatalf("output=%q, want plugin-bin-ok", out)
	}
}

func TestResolveShellCommand_PowerShellUsesRawCommand(t *testing.T) {
	raw := `Get-Content C:\temp\demo.txt`
	name, args, err := resolveShellCommand(ShellTypePowerShell, raw)
	if err != nil {
		t.Fatalf("resolveShellCommand error: %v", err)
	}
	if got := args[len(args)-1]; got != raw {
		t.Fatalf("command arg=%q, want raw %q", got, raw)
	}
	if strings.HasPrefix(args[len(args)-1], "'") || strings.HasSuffix(args[len(args)-1], "'") {
		t.Fatalf("command arg should not be wrapped in single quotes: %q", args[len(args)-1])
	}
	if runtime.GOOS == "windows" && name != "powershell" {
		t.Fatalf("name=%q, want powershell", name)
	}
}

func TestResolveShellCommand_BashUsesLoginShell(t *testing.T) {
	name, args, err := resolveShellCommand(ShellTypeBash, "printf 'ok'")
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("bash not available on PATH: %v", err)
		}
		t.Fatalf("resolveShellCommand error: %v", err)
	}
	if filepath.Base(name) != "bash" && filepath.Base(name) != "bash.exe" {
		t.Fatalf("name=%q, want bash executable", name)
	}
	if len(args) != 2 || args[0] != "-lc" {
		t.Fatalf("args=%v, want [-lc <command>]", args)
	}
	if args[1] != "printf 'ok'" {
		t.Fatalf("command=%q, want original command", args[1])
	}
}
