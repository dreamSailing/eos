package engineprovider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/pkg/coreapi"
	coreapijsonrpc "github.com/dreamSailing/eos/pkg/coreapi/jsonrpc"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

func TestSelectAutoUsesRustWhenRequiredMethodsArePresent(t *testing.T) {
	remote := sidecar.NewRemoteEngine(fakeProviderCaller{
		init: coreapijsonrpc.InitializeResult{
			ServerName: "rust-core",
			Methods: []string{
				protocoljsonrpc.MethodInitialize,
				protocoljsonrpc.MethodStateSnapshot,
				protocoljsonrpc.MethodSessionList,
			},
		},
	})

	selected, err := Select(context.Background(), Options{
		Legacy:          sidecar.NewRemoteEngine(nil),
		RequiredMethods: []string{protocoljsonrpc.MethodStateSnapshot, protocoljsonrpc.MethodSessionList},
		StartRemote:     staticRemote(remote, nil),
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selected.Kind != KindRustSidecar {
		t.Fatalf("Kind=%q, want %q", selected.Kind, KindRustSidecar)
	}
	if selected.FallbackUsed {
		t.Fatalf("FallbackUsed=true, want false")
	}
}

func TestSelectAutoPrefersRustAndDoesNotFallBackWithoutDevToggle(t *testing.T) {
	remote := sidecar.NewRemoteEngine(fakeProviderCaller{
		init: coreapijsonrpc.InitializeResult{
			ServerName: "rust-core",
			Methods:    []string{protocoljsonrpc.MethodInitialize},
		},
	})

	// 默认 auto 模式不再静默回退，必须 AllowFallback 才能 dev-only fallback。
	_, err := Select(context.Background(), Options{
		Legacy:          sidecar.NewRemoteEngine(nil),
		RequiredMethods: []string{protocoljsonrpc.MethodWorkspaceList},
		StartRemote:     staticRemote(remote, nil),
	})
	if err == nil {
		t.Fatalf("Select() expected error: auto mode should not silently fall back")
	}
	if !errors.Is(err, ErrMissingMethods) {
		t.Fatalf("Select() error = %v, want ErrMissingMethods", err)
	}
}

// TestSelectAutoWithDevToggleFallsBack 验证 dev-only 显式开启 AllowFallback 后，
// 才能走 legacy 回退路径。production 代码不应该传 AllowFallback=true。
func TestSelectAutoWithDevToggleFallsBack(t *testing.T) {
	legacy := sidecar.NewRemoteEngine(nil)
	remote := sidecar.NewRemoteEngine(fakeProviderCaller{
		init: coreapijsonrpc.InitializeResult{
			ServerName: "rust-core",
			Methods:    []string{protocoljsonrpc.MethodInitialize},
		},
	})

	selected, err := Select(context.Background(), Options{
		Legacy:          legacy,
		RequiredMethods: []string{protocoljsonrpc.MethodWorkspaceList},
		StartRemote:     staticRemote(remote, nil),
		AllowFallback:   true, // dev-only
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selected.Kind != KindLegacyGo || !selected.FallbackUsed {
		t.Fatalf("Selection=%+v, want legacy fallback", selected)
	}
	if selected.Engine != legacy {
		t.Fatalf("Engine did not use legacy fallback")
	}
	if selected.FallbackReason == "" {
		t.Fatalf("FallbackReason is empty")
	}
	if len(selected.Missing) != 1 || selected.Missing[0] != protocoljsonrpc.MethodWorkspaceList {
		t.Fatalf("Missing=%+v, want workspace/list", selected.Missing)
	}
	if selected.Initialize.ServerName != "rust-core" {
		t.Fatalf("Initialize.ServerName=%q, want rust-core", selected.Initialize.ServerName)
	}
}

func TestSelectAutoFailsWhenSidecarStartFailsWithoutDevToggle(t *testing.T) {
	_, err := Select(context.Background(), Options{
		Legacy:      sidecar.NewRemoteEngine(nil),
		StartRemote: staticRemote(nil, errors.New("sidecar missing")),
	})
	if err == nil {
		t.Fatalf("Select() expected error: auto mode should fail when sidecar start fails")
	}
	if !strings.Contains(err.Error(), "auto mode") || !strings.Contains(err.Error(), "sidecar missing") {
		t.Fatalf("Select() error = %v, want auto mode + sidecar missing", err)
	}
}

func TestSelectRustErrorsWhenRequiredMethodsAreMissing(t *testing.T) {
	remote := sidecar.NewRemoteEngine(fakeProviderCaller{
		init: coreapijsonrpc.InitializeResult{
			ServerName: "rust-core",
			Methods:    []string{protocoljsonrpc.MethodInitialize},
		},
	})

	_, err := Select(context.Background(), Options{
		Mode:            ModeRust,
		Legacy:          sidecar.NewRemoteEngine(nil),
		RequiredMethods: []string{protocoljsonrpc.MethodWorkspaceList},
		StartRemote:     staticRemote(remote, nil),
	})
	if !errors.Is(err, ErrMissingMethods) {
		t.Fatalf("Select() error = %v, want ErrMissingMethods", err)
	}
}

func TestSelectRustPassesRequiredMethodsToSidecarResolver(t *testing.T) {
	remote := sidecar.NewRemoteEngine(fakeProviderCaller{
		init: coreapijsonrpc.InitializeResult{
			ServerName: "rust-core",
			Methods: []string{
				protocoljsonrpc.MethodInitialize,
				protocoljsonrpc.MethodWorkspaceList,
			},
		},
	})
	var got sidecar.ProcessOptions

	_, err := Select(context.Background(), Options{
		Mode:            ModeRust,
		Legacy:          sidecar.NewRemoteEngine(nil),
		RequiredMethods: []string{protocoljsonrpc.MethodWorkspaceList},
		StartRemote: func(_ context.Context, opts sidecar.ProcessOptions) (RemoteEngine, error) {
			got = opts
			return remote, nil
		},
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if len(got.RequiredFeatures) != 1 || got.RequiredFeatures[0] != protocoljsonrpc.MethodWorkspaceList {
		t.Fatalf("RequiredFeatures=%+v, want workspace/list", got.RequiredFeatures)
	}
}

func TestSelectLegacyRequiresDevToggle(t *testing.T) {
	legacy := sidecar.NewRemoteEngine(nil)

	// 显式 ModeLegacy 但 AllowFallback=false 时必须拒绝：legacy 是 dev-only。
	_, err := Select(context.Background(), Options{
		Mode:   ModeLegacy,
		Legacy: legacy,
	})
	if err == nil {
		t.Fatalf("Select() expected error: legacy mode without dev toggle should fail")
	}
	if !errors.Is(err, ErrRustRequired) {
		t.Fatalf("Select() error = %v, want ErrRustRequired", err)
	}
}

func TestSelectLegacyWithDevToggleDoesNotStartRemote(t *testing.T) {
	legacy := sidecar.NewRemoteEngine(nil)
	var started bool

	selected, err := Select(context.Background(), Options{
		Mode:         ModeLegacy,
		Legacy:       legacy,
		AllowFallback: true, // dev-only
		StartRemote: func(context.Context, sidecar.ProcessOptions) (RemoteEngine, error) {
			started = true
			return nil, errors.New("should not start")
		},
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if started {
		t.Fatalf("remote was started in legacy mode")
	}
	if selected.Kind != KindLegacyGo || selected.Engine != legacy {
		t.Fatalf("Selection=%+v, want legacy", selected)
	}
}

func TestResolveModeUsesEnvironmentAliases(t *testing.T) {
	t.Setenv(EnvCoreEngine, "sidecar")
	mode, err := ResolveMode("")
	if err != nil {
		t.Fatalf("ResolveMode() error = %v", err)
	}
	if mode != ModeRust {
		t.Fatalf("mode=%q, want %q", mode, ModeRust)
	}

	mode, err = ResolveMode("eino")
	if err != nil {
		t.Fatalf("ResolveMode(eino) error = %v", err)
	}
	if mode != ModeLegacy {
		t.Fatalf("mode=%q, want %q", mode, ModeLegacy)
	}
}

func TestMissingMethodsTrimsAndIgnoresEmptyRequired(t *testing.T) {
	missing := MissingMethods(
		[]string{" initialize ", protocoljsonrpc.MethodStateSnapshot},
		[]string{"", protocoljsonrpc.MethodInitialize, protocoljsonrpc.MethodSessionList},
	)
	if len(missing) != 1 || missing[0] != protocoljsonrpc.MethodSessionList {
		t.Fatalf("MissingMethods()=%+v, want session/list", missing)
	}
}

func staticRemote(remote RemoteEngine, err error) StartRemoteFunc {
	return func(context.Context, sidecar.ProcessOptions) (RemoteEngine, error) {
		return remote, err
	}
}

type fakeProviderCaller struct {
	init coreapijsonrpc.InitializeResult
}

func (f fakeProviderCaller) Call(_ context.Context, method string, _ any, out any) error {
	switch method {
	case protocoljsonrpc.MethodInitialize:
		if target, ok := out.(*coreapijsonrpc.InitializeResult); ok {
			*target = f.init
		}
		return nil
	default:
		return coreapi.ErrUnsupported
	}
}
