//go:build windows

package sidecar

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// HideConsole 防止 GUI 壳层拉起控制台 sidecar 时，Windows 为子进程单独弹出
// 一个控制台窗口。eos-core.exe 是控制台程序：GUI 父进程（eos-app 桌面端）
// spawn 它时若不带 CREATE_NO_WINDOW，系统会为其新建可见控制台——表现为
// 「桌面软件一启动旁边还有个命令行窗口」。管道 stdio 不受该标志影响。
// 终端 CLI（eos）自身有控制台，此标志对它无副作用。
// 导出供两个壳层共用：eos-cli 的 process_client 与 eos-app 的 stdio_client
// 都必须走这里，勿在壳层另写裸 exec.Command（2026-08-20 桌面端曾因自建
// 启动器丢了该标志而复现弹窗）。
func HideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
