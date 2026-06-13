package ui

import "testing"

func TestTUISidecarClientOptionsRequiresVerifiedArtifact(t *testing.T) {
	opts := tuiSidecarClientOptions(TUIOptions{
		ModelOverride: "test-model",
		AccessMode:    "workspace-write",
		ApprovalMode:  "on-request",
	})
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
	if opts.Env["EOS_ACCESS_MODE"] != "workspace-write" {
		t.Fatalf("EOS_ACCESS_MODE=%q, want workspace-write", opts.Env["EOS_ACCESS_MODE"])
	}
	if opts.Env["EOS_APPROVAL_MODE"] != "on-request" {
		t.Fatalf("EOS_APPROVAL_MODE=%q, want on-request", opts.Env["EOS_APPROVAL_MODE"])
	}
	if opts.Env["EOS_CORE_STORE_DIR"] == "" {
		t.Fatal("tuiSidecarClientOptions() must provide EOS_CORE_STORE_DIR")
	}
}
