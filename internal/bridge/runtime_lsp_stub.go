//go:build legacy && without_lsp
// +build legacy,without_lsp

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


// lspManagerEntry LSP 管理器条目（stub 版本）

type lspManagerEntry struct{}

// initLSPManager 初始化 LSP 管理器（stub 版本）
func (rc *RuntimeCore) initLSPManager() *lspManagerEntry {
	return nil
}

func (rc *RuntimeCore) refreshLSPManager() {}

// ProcessLSPDiagnostics 处理 LSP 诊断信息（stub 版本）
func (rc *RuntimeCore) ProcessLSPDiagnostics(lspEntry *lspManagerEntry) {
	// LSP 已禁用，不执行任何操作
}

func (rc *RuntimeCore) ProblemsAndDiagnosticsMarkdown() string {
	return "## Problems and Diagnostics\n\nNo diagnostics available (LSP disabled).\n"
}

func (rc *RuntimeCore) LSPServersMarkdown() string {
	return "## LSP\n\nLSP is disabled (built with -tags without_lsp).\n"
}

func (rc *RuntimeCore) LSPStatus() LSPStatus {
	return LSPStatus{
		Enabled:   false,
		Workspace: rc.workingRoot(),
		Message:   "disabled_build_tag",
	}
}

// ShutdownLSPManager 关闭 LSP 管理器（stub 版本）
func (rc *RuntimeCore) ShutdownLSPManager(lspEntry *lspManagerEntry) {
	// LSP 已禁用，不执行任何操作
}
