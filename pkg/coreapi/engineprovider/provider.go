package engineprovider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dreamSailing/eos/pkg/coreapi"
	coreapijsonrpc "github.com/dreamSailing/eos/pkg/coreapi/jsonrpc"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar"
)

const EnvCoreEngine = "EOS_CORE_ENGINE"

type Mode string

const (
	// ModeAuto 现在的语义是 "默认 Rust-only"：尝试启动 eos-core sidecar，
	// 失败时直接返回 error，不做静默回退。legacy/parity 必须显式声明。
	ModeAuto Mode = "auto"
	ModeRust Mode = "rust"
	// ModeLegacy 显式选择 Go legacy core。仅允许 dev/test 场景使用。
	ModeLegacy Mode = "legacy"
	// ModeParity 走 parity harness，对比 legacy 与 rust 的等价性。
	// 仅 parity_test / dev 显式开启，production 不使用。
	ModeParity Mode = "parity"
)

type Kind string

const (
	KindRustSidecar Kind = "rust-sidecar"
	KindLegacyGo    Kind = "legacy-go"
)

var (
	ErrMissingMethods  = errors.New("core engine is missing required methods")
	ErrRustRequired    = errors.New("eos-core sidecar is required; legacy fallback is dev-only")
	ErrParityDevOnly   = errors.New("parity mode is dev-only; production must use rust")
)

type RemoteEngine interface {
	coreapi.Engine
	Initialize(context.Context) (coreapijsonrpc.InitializeResult, error)
	Close() error
}

type StartRemoteFunc func(context.Context, sidecar.ProcessOptions) (RemoteEngine, error)

type Options struct {
	Mode            Mode
	Legacy          coreapi.Engine
	Sidecar         sidecar.ProcessOptions
	RequiredMethods []string
	StartRemote     StartRemoteFunc
	AllowFallback   bool
}

type Selection struct {
	Kind           Kind
	Engine         coreapi.Engine
	Initialize     coreapijsonrpc.InitializeResult
	Missing        []string
	FallbackUsed   bool
	FallbackReason string
	close          func() error
}

func Select(ctx context.Context, opts Options) (Selection, error) {
	mode, err := ResolveMode(string(opts.Mode))
	if err != nil {
		return Selection{}, err
	}
	switch mode {
	case ModeLegacy:
		// 显式 legacy：仅在 AllowFallback=true 或显式声明 dev 时允许。
		if !opts.AllowFallback {
			return Selection{}, fmt.Errorf("legacy mode requires AllowFallback=true (dev only): %w", ErrRustRequired)
		}
		return selectLegacy(opts.Legacy, false)
	case ModeParity:
		// parity 必须 AllowFallback。
		if !opts.AllowFallback {
			return Selection{}, ErrParityDevOnly
		}
		selected, err := selectRust(ctx, opts)
		if err != nil {
			return Selection{}, fmt.Errorf("parity: rust init failed: %w", err)
		}
		legacy, legacyErr := selectLegacy(opts.Legacy, true)
		if legacyErr != nil {
			return Selection{}, fmt.Errorf("parity: legacy init failed: %v", legacyErr)
		}
		legacy.Initialize = selected.Initialize
		legacy.Missing = append([]string(nil), selected.Missing...)
		legacy.FallbackReason = "parity harness: " + err.Error()
		return legacy, nil
	case ModeRust:
		selected, err := selectRust(ctx, opts)
		if err == nil {
			return selected, nil
		}
		if !opts.AllowFallback {
			return Selection{}, fmt.Errorf("rust mode: %w", err)
		}
		legacy, legacyErr := selectLegacy(opts.Legacy, true)
		if legacyErr != nil {
			return Selection{}, fmt.Errorf("%w; legacy fallback failed: %v", err, legacyErr)
		}
		legacy.Initialize = selected.Initialize
		legacy.Missing = append([]string(nil), selected.Missing...)
		legacy.FallbackReason = err.Error()
		return legacy, nil
	default: // ModeAuto
		// 默认 Rust-only；不静默回退到 legacy。
		selected, err := selectRust(ctx, opts)
		if err == nil {
			return selected, nil
		}
		if !opts.AllowFallback {
			return Selection{}, fmt.Errorf("auto mode (rust-only): %w", err)
		}
		legacy, legacyErr := selectLegacy(opts.Legacy, true)
		if legacyErr != nil {
			return Selection{}, fmt.Errorf("%w; legacy fallback failed: %v", err, legacyErr)
		}
		legacy.Initialize = selected.Initialize
		legacy.Missing = append([]string(nil), selected.Missing...)
		legacy.FallbackReason = err.Error()
		return legacy, nil
	}
}

func ResolveMode(value string) (Mode, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = strings.TrimSpace(os.Getenv(EnvCoreEngine))
	}
	switch strings.ToLower(value) {
	case "", "auto", "rust":
		// 空 / auto / rust 都视为 Rust-only；与 production 默认行为一致。
		return ModeAuto, nil
	case "sidecar", "rust-sidecar":
		return ModeRust, nil
	case "legacy", "go", "eino", "legacy-go":
		return ModeLegacy, nil
	case "parity":
		return ModeParity, nil
	default:
		return "", fmt.Errorf("unsupported core engine mode %q", value)
	}
}

func MissingMethods(available []string, required []string) []string {
	if len(required) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(available))
	for _, method := range available {
		method = strings.TrimSpace(method)
		if method != "" {
			seen[method] = struct{}{}
		}
	}
	var missing []string
	for _, method := range required {
		method = strings.TrimSpace(method)
		if method == "" {
			continue
		}
		if _, ok := seen[method]; !ok {
			missing = append(missing, method)
		}
	}
	return missing
}

func (s Selection) Close() error {
	if s.close == nil {
		return nil
	}
	return s.close()
}

func selectRust(ctx context.Context, opts Options) (Selection, error) {
	start := opts.StartRemote
	if start == nil {
		start = func(ctx context.Context, processOpts sidecar.ProcessOptions) (RemoteEngine, error) {
			return sidecar.StartRemoteEngine(ctx, processOpts)
		}
	}
	processOpts := opts.Sidecar
	if len(processOpts.RequiredFeatures) == 0 && len(opts.RequiredMethods) > 0 {
		processOpts.RequiredFeatures = append([]string(nil), opts.RequiredMethods...)
	}
	remote, err := start(ctx, processOpts)
	if err != nil {
		return Selection{}, err
	}
	closeRemote := func() error { return remote.Close() }
	initResult, err := remote.Initialize(ctx)
	if err != nil {
		_ = closeRemote()
		return Selection{}, err
	}
	missing := MissingMethods(initResult.Methods, opts.RequiredMethods)
	if len(missing) > 0 {
		_ = closeRemote()
		return Selection{Kind: KindRustSidecar, Initialize: initResult, Missing: missing}, fmt.Errorf("%w: %s", ErrMissingMethods, strings.Join(missing, ", "))
	}
	return Selection{
		Kind:       KindRustSidecar,
		Engine:     remote,
		Initialize: initResult,
		close:      closeRemote,
	}, nil
}

func selectLegacy(engine coreapi.Engine, fallback bool) (Selection, error) {
	if engine == nil {
		return Selection{}, errors.New("legacy core engine is not configured")
	}
	return Selection{
		Kind:         KindLegacyGo,
		Engine:       engine,
		FallbackUsed: fallback,
	}, nil
}
