package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

// GetPendingDiff 获取待处理的差异
func (rc *RuntimeCore) GetPendingDiff() string {
	return rc.securityMgr.GetPendingDiff()
}

// ClearPendingDiff 清除待处理的差异
func (rc *RuntimeCore) ClearPendingDiff() {
	rc.securityMgr.ClearPendingDiff()
}

// SetPendingDiffPath 设置待处理差异的路径
func (rc *RuntimeCore) SetPendingDiffPath(p string) {
	rc.securityMgr.SetPendingDiffPath(p)
}

// GetPendingDiffPath 获取待处理差异的路径
func (rc *RuntimeCore) GetPendingDiffPath() string {
	return rc.securityMgr.GetPendingDiffPath()
}

// AllowSession 允许会话级别的权限
func (rc *RuntimeCore) AllowSession(category string) {
	rc.securityMgr.AllowSession(category)
}

// DenySession 拒绝会话级别的权限
func (rc *RuntimeCore) DenySession(category string) {
	rc.securityMgr.DenySession(category)
}

// IsAllowed 检查是否允许某类别操作
func (rc *RuntimeCore) IsAllowed(category string) bool {
	return rc.securityMgr.IsAllowed(category)
}

func (rc *RuntimeCore) SetExecutionMode(mode string) {
	if rc == nil || rc.securityMgr == nil {
		return
	}
	rc.securityMgr.SetExecutionMode(mode)
}

func (rc *RuntimeCore) ExecutionMode() string {
	if rc == nil || rc.securityMgr == nil {
		return ""
	}
	return rc.securityMgr.ExecutionMode()
}

func (rc *RuntimeCore) SetSandboxMode(mode string) {
	if rc == nil || rc.securityMgr == nil {
		return
	}
	rc.securityMgr.SetSandboxMode(mode)
}

func (rc *RuntimeCore) SandboxMode() string {
	if rc == nil || rc.securityMgr == nil {
		return ""
	}
	return rc.securityMgr.SandboxMode()
}

func (rc *RuntimeCore) PermissionSnapshot() PermissionSnapshot {
	if rc == nil || rc.securityMgr == nil {
		return PermissionSnapshot{}
	}
	return rc.securityMgr.Snapshot()
}
