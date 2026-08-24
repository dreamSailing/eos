package ui

import (
	"io"
	"testing"
)

func TestTUISidecarClientOptionsRequiresVerifiedArtifact(t *testing.T) {
	opts := tuiSidecarClientOptions(TUIOptions{
		ModelOverride: "test-model",
		AccessMode:    "workspace-write",
		ApprovalMode:  "on-request",
	}, io.Discard)
	if !opts.VerifyChecksum {
		t.Fatal("tuiSidecarClientOptions() must enable checksum verification")
	}
	if !opts.RequireSignature {
		t.Fatal("tuiSidecarClientOptions() must require a signed manifest")
	}
	if opts.AllowDevPlaceholder {
		t.Fatal("tuiSidecarClientOptions() must not allow development placeholder signatures")
	}
	if opts.Env["EOS_MODEL_OVERRIDE"] != "test-model" {
		t.Fatalf("EOS_MODEL_OVERRIDE=%q, want test-model", opts.Env["EOS_MODEL_OVERRIDE"])
	}
	// 沙箱轴只经 EOS_SANDBOX_MODE 单通道下发（内核不读 EOS_ACCESS_MODE）。
	if _, ok := opts.Env["EOS_ACCESS_MODE"]; ok {
		t.Fatalf("EOS_ACCESS_MODE=%q leaked; kernel does not read it", opts.Env["EOS_ACCESS_MODE"])
	}
	if opts.Env["EOS_SANDBOX_MODE"] != "workspace-write" {
		t.Fatalf("EOS_SANDBOX_MODE=%q, want workspace-write (falls back to AccessMode)", opts.Env["EOS_SANDBOX_MODE"])
	}
	if opts.Env["EOS_APPROVAL_MODE"] != "on-request" {
		t.Fatalf("EOS_APPROVAL_MODE=%q, want on-request", opts.Env["EOS_APPROVAL_MODE"])
	}
	if opts.Env["EOS_CORE_STORE_DIR"] == "" {
		t.Fatal("tuiSidecarClientOptions() must provide EOS_CORE_STORE_DIR")
	}
}

// 日志级别默认 info（debug 会把敏感内容落盘），用户显式设置时跟随。
func TestTUISidecarClientOptionsLogLevel(t *testing.T) {
	t.Setenv("EOS_LOG_LEVEL", "")
	opts := tuiSidecarClientOptions(TUIOptions{}, io.Discard)
	if got := opts.Env["EOS_LOG_LEVEL"]; got != "info" {
		t.Fatalf("EOS_LOG_LEVEL=%q, want info (default)", got)
	}

	t.Setenv("EOS_LOG_LEVEL", "debug")
	opts = tuiSidecarClientOptions(TUIOptions{}, io.Discard)
	if got := opts.Env["EOS_LOG_LEVEL"]; got != "debug" {
		t.Fatalf("EOS_LOG_LEVEL=%q, want debug (user override)", got)
	}
}
