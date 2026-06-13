//go:build legacy

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/pkg/coreapi/engineprovider"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

func TestAppServerRequiredMethodsGuardFullCoreCutover(t *testing.T) {
	methods := appServerRequiredMethods()
	required := []string{
		protocoljsonrpc.MethodStateSnapshot,
		protocoljsonrpc.MethodWorkspaceList,
		protocoljsonrpc.MethodWorkspaceRemember,
		protocoljsonrpc.MethodWorkspaceSetForeground,
		protocoljsonrpc.MethodSessionList,
		protocoljsonrpc.MethodSessionMessagesLoad,
		protocoljsonrpc.MethodSessionCreate,
		protocoljsonrpc.MethodSessionResume,
		protocoljsonrpc.MethodSessionCurrent,
		protocoljsonrpc.MethodSessionMessagesSave,
		protocoljsonrpc.MethodTurnStart,
		protocoljsonrpc.MethodTurnInterrupt,
		protocoljsonrpc.MethodApprovalRespond,
		protocoljsonrpc.MethodInquiryRespond,
		protocoljsonrpc.MethodToolCatalog,
		protocoljsonrpc.MethodToolExecute,
		protocoljsonrpc.MethodEventSubscribe,
		protocoljsonrpc.MethodEventUnsubscribe,
		protocoljsonrpc.MethodConfigReload,
		protocoljsonrpc.MethodAgentControl,
		protocoljsonrpc.MethodAgentInput,
		protocoljsonrpc.MethodAgentRun,
		protocoljsonrpc.MethodSandboxBackend,
	}
	if len(methods) != len(required) {
		t.Fatalf("appServerRequiredMethods() len=%d, want %d: %+v", len(methods), len(required), methods)
	}
	for _, method := range required {
		if !containsString(methods, method) {
			t.Fatalf("appServerRequiredMethods() missing %s", method)
		}
	}
}

func TestAppServerRequiredMethodsCoveredByVendoredManifest(t *testing.T) {
	_, err := sidecar.ResolveBinary(sidecar.ResolveOptions{
		VerifyChecksum:   true,
		RequiredFeatures: appServerRequiredMethods(),
	})
	if err != nil {
		if errors.Is(err, sidecar.ErrCoreBinaryNotFound) {
			t.Skipf("no vendored sidecar binary for this target: %v", err)
		}
		t.Fatalf("vendored sidecar manifest does not cover app-server cutover: %v", err)
	}
}

func TestWriteAppServerEngineSelection(t *testing.T) {
	var out bytes.Buffer
	writeAppServerEngineSelection(&out, engineprovider.Selection{
		Kind:           engineprovider.KindLegacyGo,
		FallbackUsed:   true,
		FallbackReason: "missing method",
	})
	got := out.String()
	if !strings.Contains(got, "legacy-go") || !strings.Contains(got, "missing method") {
		t.Fatalf("selection output=%q, want kind and fallback reason", got)
	}

	out.Reset()
	writeAppServerEngineSelection(&out, engineprovider.Selection{Kind: engineprovider.KindRustSidecar})
	got = out.String()
	if !strings.Contains(got, "rust-sidecar") || strings.Contains(got, "fallback") {
		t.Fatalf("selection output=%q, want rust without fallback", got)
	}
}

func TestAppServerSidecarOptionsSetsDefaultStoreDir(t *testing.T) {
	t.Setenv(envRustCoreStoreDir, "")
	opts := appServerSidecarOptions()
	if opts.Env[envRustCoreStoreDir] == "" {
		t.Fatalf("appServerSidecarOptions() did not provide %s", envRustCoreStoreDir)
	}
	if !opts.VerifyChecksum || !opts.RequireSignature {
		t.Fatalf("appServerSidecarOptions() must verify checksum and signature: %#v", opts)
	}
}

func TestAppServerSidecarOptionsKeepsExplicitStoreDirInParentEnv(t *testing.T) {
	t.Setenv(envRustCoreStoreDir, "C:/custom-store")
	opts := appServerSidecarOptions()
	if _, ok := opts.Env[envRustCoreStoreDir]; ok {
		t.Fatalf("appServerSidecarOptions() should not override explicit parent %s", envRustCoreStoreDir)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
