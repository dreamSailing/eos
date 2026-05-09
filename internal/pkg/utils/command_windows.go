//go:build windows

package utils

import (
	"os/exec"
	"syscall"
)

func applyNoWindow(cmd *exec.Cmd) {
	if isGUIMode() {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.HideWindow = true
	}
}
