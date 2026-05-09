//go:build !windows

package utils

import "os/exec"

func applyNoWindow(cmd *exec.Cmd) {}
