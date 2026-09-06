package engineprovider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eosaios/eos/pkg/coreapi"
	coreapijsonrpc "github.com/eosaios/eos/pkg/coreapi/jsonrpc"
	"github.com/eosaios/eos/pkg/coreapi/sidecar"
	protocoljsonrpc "github.com/eosaios/eos/pkg/protocol/jsonrpc"
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
		RequiredMethods: []string{protocoljsonrpc.MethodStateSnapshot, protocoljsonrpc.MethodSessionList},
		StartRemote:     staticRemote(remote, nil),
	})
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if selected.Kind != KindRustSidecar {
		t.Fatalf("Kind=%q, want %q", selected.Kind, KindRustSidecar)
	}
}

func TestSelectAutoErrorsWhenRequiredMethodsAreMissing(t *testing.T) {
	remote := sidecar.NewRemoteEngine(fakeProviderCaller{
		init: coreapijsonrpc.InitializeResult{
			ServerName: "rust-core",
			Methods:    []string{protocoljsonrpc.MethodInitialize},
		},
	})

	// 缺失必需方法时直接报 ErrMissingMethods，不再回退。
	_, err := Select(context.Background(), Options{
		RequiredMethods: []string{protocoljsonrpc.MethodWorkspaceList},
		StartRemote:     staticRemote(remote, nil),
	})
	if !errors.Is(err, ErrMissingMethods) {
		t.Fatalf("Select() error = %v, want ErrMissingMethods", err)
	}
}

func TestSelectAutoErrorsWhenSidecarStartFails(t *testing.T) {
	_, err := Select(context.Background(), Options{
		StartRemote: staticRemote(nil, errors.New("sidecar missing")),
	})
	if err == nil {
		t.Fatalf("Select() expected error when sidecar start fails")
	}
	if !strings.Contains(err.Error(), "sidecar missing") {
		t.Fatalf("Select() error = %v, want it to contain sidecar missing", err)
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

func TestResolveModeOnlyAcceptsAutoAndRust(t *testing.T) {
	for _, value := range []string{"", "auto", "rust"} {
		mode, err := ResolveMode(value)
		if err != nil {
			t.Fatalf("ResolveMode(%q) error = %v", value, err)
		}
		if mode != ModeAuto {
			t.Fatalf("ResolveMode(%q) = %q, want %q", value, mode, ModeAuto)
		}
	}

	// 退役的 mode 字符串必须被拒绝，避免静默走老路径。
	for _, value := range []string{"legacy", "go", "eino", "parity", "sidecar"} {
		if _, err := ResolveMode(value); err == nil {
			t.Fatalf("ResolveMode(%q) expected error (retired mode)", value)
		}
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
