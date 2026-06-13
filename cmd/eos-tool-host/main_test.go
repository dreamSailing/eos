//go:build legacy

package main

import (
	"testing"

	"github.com/dreamSailing/eos/pkg/coreapi/sidecar/toolhost"
)

// TestResolveHostFakeFlagShortcut verifies the test-compat path: setting
// `EOS_TOOL_HOST_FAKE=1` short-circuits resolveHost to a FakeHost so unit
// tests and dev loops don't need a real `LegacyHost` runner.
//
// The Go tool-host process is intentionally not part of the default
// EOS startup path. It only runs as a test-compat shim when
// `EOS_TOOL_HOST` is set on the Rust sidecar, and this test guarantees
// the FakeHost opt-in keeps working in isolation.
func TestResolveHostFakeFlagShortcut(t *testing.T) {
	t.Setenv("EOS_TOOL_HOST_FAKE", "1")
	host, err := resolveHost()
	if err != nil {
		t.Fatalf("resolveHost error: %v", err)
	}
	if _, ok := host.(*toolhost.FakeHost); !ok {
		t.Fatalf("resolveHost returned %T, want *toolhost.FakeHost", host)
	}
}

// TestResolveHostDoesNotRequireEOS_TOOL_HOST asserts that the Go tool-host
// binary itself never reads EOS_TOOL_HOST. That env var is only consulted
// by the Rust sidecar's runtime_deps; the Go process is launched *by* the
// sidecar after the host decision is already made.
func TestResolveHostDoesNotRequireEOS_TOOL_HOST(t *testing.T) {
	t.Setenv("EOS_TOOL_HOST_FAKE", "1")
	// Force a sentinel EOS_TOOL_HOST value in the environment; resolveHost
	// must still pick the FakeHost and never inspect EOS_TOOL_HOST itself.
	t.Setenv("EOS_TOOL_HOST", "/nonexistent/eos-tool-host-binary")
	host, err := resolveHost()
	if err != nil {
		t.Fatalf("resolveHost error: %v", err)
	}
	if _, ok := host.(*toolhost.FakeHost); !ok {
		t.Fatalf("resolveHost returned %T, want *toolhost.FakeHost", host)
	}
}

// TestResolveHostFakeFlagWinsOverLegacy confirms the FakeHost shortcut
// always wins when EOS_TOOL_HOST_FAKE=1 is set, even if the LegacyHost
// would otherwise be selected. This protects the test-compat surface
// from environment drift.
func TestResolveHostFakeFlagWinsOverLegacy(t *testing.T) {
	t.Setenv("EOS_TOOL_HOST_FAKE", "1")
	t.Setenv("EOS_WORKSPACE_ROOT", "/tmp/should-not-be-used")
	host, err := resolveHost()
	if err != nil {
		t.Fatalf("resolveHost error: %v", err)
	}
	if _, ok := host.(*toolhost.FakeHost); !ok {
		t.Fatalf("resolveHost returned %T, want *toolhost.FakeHost", host)
	}
}
