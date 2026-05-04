package utils

import (
	"context"
	"os"
	"os/exec"
)

func isGUIMode() bool {
	return os.Getenv("EOS_GUI_MODE") != ""
}

// Command is like exec.Command, but on Windows in GUI mode, it prevents
// console windows from being created for the child process.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	applyNoWindow(cmd)
	return cmd
}

// CommandContext is like exec.CommandContext, but on Windows in GUI mode, it prevents
// console windows from being created for the child process.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	applyNoWindow(cmd)
	return cmd
}
