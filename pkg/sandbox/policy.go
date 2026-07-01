// Package sandbox 提供 EOS 壳层（eos-cli / eos-app）与 Rust 内核之间传递
// 沙箱策略所需的 DTO。
//
// 命令/写入/网络的实际裁决发生在 Rust 内核（eos-core-sandbox 的
// EnforcementSandboxRunner.check_command 等），壳层不做本地裁决，只通过
// sidecar RPC（sandbox/policy、sandbox/set_policy、sandbox/backend_status）
// 透传策略配置与后端状态。历史上壳层曾有一份本地裁决实现（GuardedRunner /
// CommandViolation / AllowsCommand 等），已作为死代码删除——裁决必须与工具
// 执行在同进程，避免 TOCTOU。
package sandbox

import (
	"runtime"
	"strings"
)

// Mode 是沙箱策略的三档模式。仅作为 DTO 在壳层与内核间传递；
// 裁决语义由内核实现。
type Mode string

const (
	ModeReadOnly         Mode = "read-only"
	ModeWorkspaceWrite   Mode = "workspace-write"
	ModeDangerFullAccess Mode = "danger-full-access"
)

// NetworkPolicy 控制网络访问的总开关。仅 DTO。
type NetworkPolicy string

const (
	NetworkDeny  NetworkPolicy = "deny"
	NetworkAllow NetworkPolicy = "allow"
)

// Policy 是经 sidecar 透传给内核的沙箱策略。字段语义由内核消费，
// 壳层不基于这些字段做本地裁决。
type Policy struct {
	Mode                   Mode          `json:"mode"`
	WorkspaceRoot          string        `json:"workspace_root,omitempty"`
	WritableRoots          []string      `json:"writable_roots,omitempty"`
	Network                NetworkPolicy `json:"network"`
	AllowedCommandPrefixes []string      `json:"allowed_command_prefixes,omitempty"`
}

// BackendStatus 描述内核沙箱后端的当前能力与降级状态，由内核经 RPC 返回壳层。
type BackendStatus struct {
	GOOS                    string   `json:"goos"`
	Backend                 string   `json:"backend"`
	Enforced                bool     `json:"enforced"`
	Degraded                bool     `json:"degraded"`
	Reason                  string   `json:"reason,omitempty"`
	UnsupportedCapabilities []string `json:"unsupported_capabilities,omitempty"`
}

// NormalizeMode 把大小写/下划线变体归一化为标准 Mode 字符串。
// 用于壳层解析用户输入（CLI flag / 配置）后传给内核之前。
func NormalizeMode(mode string) Mode {
	key := strings.ToLower(strings.TrimSpace(mode))
	key = strings.ReplaceAll(key, "_", "-")
	switch key {
	case "read-only", "readonly":
		return ModeReadOnly
	case "danger-full-access", "dangerfullaccess", "full-access", "fullaccess", "full-access-mode":
		return ModeDangerFullAccess
	default:
		return ModeWorkspaceWrite
	}
}

// DetectBackendForOS 返回给定 OS 的后端能力快照，用于壳层在不经内核往返时
// 给出降级提示（例如初始化阶段）。最终裁决仍以内核 BackendStatus 为准。
func DetectBackendForOS(goos string) BackendStatus {
	switch goos {
	case "linux":
		return BackendStatus{
			GOOS:                    goos,
			Backend:                 "bubblewrap-or-landlock",
			Enforced:                false,
			Degraded:                true,
			Reason:                  "backend probing not wired yet",
			UnsupportedCapabilities: []string{"seccomp-filter", "namespace-isolation"},
		}
	case "darwin":
		return BackendStatus{
			GOOS:                    goos,
			Backend:                 "seatbelt",
			Enforced:                false,
			Degraded:                true,
			Reason:                  "backend probing not wired yet",
			UnsupportedCapabilities: []string{"seatbelt-profile", "filesystem-tampering-detection"},
		}
	case "windows":
		return BackendStatus{
			GOOS:                    goos,
			Backend:                 "path-broker",
			Enforced:                false,
			Degraded:                true,
			Reason:                  "restricted token/job object backend not wired yet",
			UnsupportedCapabilities: []string{"restricted-token", "job-object", "path-broker-enforcement"},
		}
	default:
		return BackendStatus{
			GOOS:                    goos,
			Backend:                 "none",
			Enforced:                false,
			Degraded:                true,
			Reason:                  "unsupported OS",
			UnsupportedCapabilities: []string{"all-sandbox-capabilities"},
		}
	}
}

// DetectBackend 是 DetectBackendForOS 的便捷封装，按当前运行平台探测。
func DetectBackend() BackendStatus {
	return DetectBackendForOS(runtime.GOOS)
}
