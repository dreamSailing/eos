//go:build !windows

package sidecar

import "os/exec"

// hideConsole 仅 Windows 需要：Unix 平台无控制台窗口概念，no-op。
func hideConsole(*exec.Cmd) {}
