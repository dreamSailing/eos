package architecture

// 本文件守护 engineprovider.Select 在 production 模式（AllowFallback=false）
// 下的失败路径：
//   1. eos-core 二进制不存在：直接 error，不回退到 sharedcore。
//   2. eos-core 子进程启动但 initialize 失败：直接 error。
//   3. 子进程 initialize 成功但缺少 required methods：直接 error。
//   4. 协议不匹配（initialize 报协议错误）：直接 error。
//   5. manifest 与 binary checksum 不匹配：直接 error。
//   6. manifest 签名错误：直接 error。
//
// 这 6 条路径在 production 路径里必须 fail-fast；legacy 仅允许
// parity / dev 通过 EOS_CORE_ALLOW_FALLBACK=1 显式打开。
//
// 守护 invariant：production 路径（AllowFallback=false）下，任何 eos-core
// 解析/握手失败都不允许回退到 sharedcore。legacy 永远不会被 select。

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/pkg/coreapi"
	"github.com/dreamSailing/eos/pkg/coreapi/engineprovider"
	coreapijsonrpc "github.com/dreamSailing/eos/pkg/coreapi/jsonrpc"
	"github.com/dreamSailing/eos/pkg/coreapi/sidecar"
	sidecarclient "github.com/dreamSailing/eos/pkg/coreapi/sidecar/client"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

func requiredMethodsForCutover() []string {
	return append([]string(nil), sidecarclient.RequiredMethods...)
}

// TestEngineProviderRejectsMissingRustBinary 验证当 eos-core 二进制不存在时，
// engineprovider.Select 返回 error，且没有 fall back 到 legacy engine。
func TestEngineProviderRejectsMissingRustBinary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EOS_CORE_PATH", filepath.Join(dir, "no-such-binary"))

	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode:            engineprovider.ModeAuto,
		Sidecar:         sidecar.ProcessOptions{},
		RequiredMethods: requiredMethodsForCutover(),
		// 关键：AllowFallback=false 是 production 行为。
		AllowFallback: false,
	})
	if err == nil {
		t.Fatal("Select() succeeded with missing Rust binary; expected error")
	}
	assertRustOnlyError(t, err)
	// 即便显式提供 legacy engine 与 AllowFallback=true，resolution 错误
	// 也不应被静默吞掉。下一条用例覆盖 AllowFallback=true 的兜底矩阵。
}

// TestEngineProviderRejectsMissingRustBinaryWithLegacyLoaded 验证 production
// 模式（AllowFallback=false）下，即使 legacy engine 在 opts 里"就绪"，
// 也会被完全忽略——rust 解析失败直接 fail-fast，绝不会走到 selectLegacy。
//
// 这条 invariant 是 CLI/architecture 的"绝对护栏"：production 代码不能
// 偷偷改 AllowFallback=true 偷偷跑 legacy。
func TestEngineProviderRejectsMissingRustBinaryWithLegacyLoaded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EOS_CORE_PATH", filepath.Join(dir, "no-such-binary"))

	legacy := &fallbackProbeEngine{}
	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode:            engineprovider.ModeAuto,
		Sidecar:         sidecar.ProcessOptions{},
		RequiredMethods: requiredMethodsForCutover(),
		Legacy:          legacy,
		AllowFallback:   false, // production 行为
	})
	if err == nil {
		t.Fatal("Select() succeeded with missing Rust binary; expected error")
	}
	if legacy.touched {
		t.Fatal("legacy engine was touched after resolution failure; production must not touch legacy")
	}
	assertRustOnlyError(t, err)
}

// TestEngineProviderRejectsInitializeFailure 验证当 initialize 失败时，Select 直接
// 返回 error，不会静默回退到 legacy。
func TestEngineProviderRejectsInitializeFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	remote := sidecar.NewRemoteEngine(failureCaller{err: errors.New("fake initialize failure")})
	_, err := engineprovider.Select(ctx, engineprovider.Options{
		Mode:            engineprovider.ModeAuto,
		RequiredMethods: requiredMethodsForCutover(),
		AllowFallback:   false,
		StartRemote:     staticStartRemote(remote, nil),
	})
	if err == nil {
		t.Fatal("Select() succeeded when initialize failed; expected error")
	}
	if !strings.Contains(err.Error(), "fake initialize failure") {
		t.Logf("note: error message is %q", err.Error())
	}
}

// TestEngineProviderRejectsMissingRequiredMethods 通过 stub 验证当子进程启动
// 成功但 initialize 返回的 methods 不全时，Select 返回 ErrMissingMethods。
func TestEngineProviderRejectsMissingRequiredMethods(t *testing.T) {
	required := requiredMethodsForCutover()
	// 故意从 initialize 结果中删除一个 required method。
	truncated := required[:len(required)-1]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	remote := sidecar.NewRemoteEngine(methodsCaller{methods: truncated})
	_, err := engineprovider.Select(ctx, engineprovider.Options{
		Mode:            engineprovider.ModeAuto,
		RequiredMethods: required,
		AllowFallback:   false,
		StartRemote:     staticStartRemote(remote, nil),
	})
	if err == nil {
		t.Fatal("Select() succeeded with truncated methods; expected ErrMissingMethods")
	}
	if !errors.Is(err, engineprovider.ErrMissingMethods) {
		t.Fatalf("Select() err = %v, want ErrMissingMethods", err)
	}
}

// TestEngineProviderMissingMethodWithLegacyLoaded 验证 production 模式
// (AllowFallback=false) 下，缺方法时 legacy 永远不会被 select。
// production 缺方法等于"eos-core 协议漂移"，必须被 release gate 显式拦下。
func TestEngineProviderMissingMethodWithLegacyLoaded(t *testing.T) {
	required := requiredMethodsForCutover()
	truncated := required[:len(required)-1]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	remote := sidecar.NewRemoteEngine(methodsCaller{methods: truncated})
	legacy := &fallbackProbeEngine{}
	_, err := engineprovider.Select(ctx, engineprovider.Options{
		Mode:            engineprovider.ModeAuto,
		RequiredMethods: required,
		Legacy:          legacy,
		AllowFallback:   false, // production 行为
		StartRemote:     staticStartRemote(remote, nil),
	})
	if err == nil {
		t.Fatal("Select() succeeded with truncated methods; expected error")
	}
	if legacy.touched {
		t.Fatal("legacy engine was touched on missing-method error; production must not touch legacy")
	}
	if !errors.Is(err, engineprovider.ErrMissingMethods) {
		t.Fatalf("Select() err = %v, want ErrMissingMethods", err)
	}
}

// TestEngineProviderRejectsProtocolMismatch 验证 required methods 列表中包含一个
// 不存在的 "eos/protocol/handshake" 时，Select 返回 error。
func TestEngineProviderRejectsProtocolMismatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	required := append([]string(nil), requiredMethodsForCutover()...)
	required = append(required, "eos/protocol/handshake/unknown")

	// 注意：caller 必须声明所有声明过的方法，包括新加的"unknown"。这里只声明 known
	// 列表，所以应该会触发 ErrMissingMethods。
	declared := append([]string(nil), requiredMethodsForCutover()...)
	remote := sidecar.NewRemoteEngine(methodsCaller{methods: declared})
	_, err := engineprovider.Select(ctx, engineprovider.Options{
		Mode:            engineprovider.ModeAuto,
		RequiredMethods: required,
		AllowFallback:   false,
		StartRemote:     staticStartRemote(remote, nil),
	})
	if err == nil {
		t.Fatal("Select() succeeded with protocol mismatch; expected error")
	}
	if !errors.Is(err, engineprovider.ErrMissingMethods) {
		t.Fatalf("Select() err = %v, want ErrMissingMethods", err)
	}
}

// TestEngineProviderModeAutoDefaultsToRustOnly 不传 Mode 时，必须解析为
// ModeAuto = Rust-only。这是 production 行为的根节点。
func TestEngineProviderModeAutoDefaultsToRustOnly(t *testing.T) {
	mode, err := engineprovider.ResolveMode("")
	if err != nil {
		t.Fatalf("ResolveMode(\"\") error = %v", err)
	}
	if mode != engineprovider.ModeAuto {
		t.Fatalf("ResolveMode(\"\") = %q, want %q", mode, engineprovider.ModeAuto)
	}
	// ModeAuto 在 Select 中必须 rust-only，不接受 legacy fallback (AllowFallback=false)。
	dir := t.TempDir()
	t.Setenv("EOS_CORE_PATH", filepath.Join(dir, "no-such-binary"))
	_, err = engineprovider.Select(context.Background(), engineprovider.Options{
		// 不传 Mode → ModeAuto
		Sidecar:         sidecar.ProcessOptions{},
		RequiredMethods: requiredMethodsForCutover(),
		AllowFallback:   false,
	})
	if err == nil {
		t.Fatal("Select(ModeAuto) succeeded with missing binary; expected error")
	}
}

// TestEngineProviderModeRustWithoutAllowFallbackFails 验证 ModeRust + AllowFallback=false
// 仍然 fail-fast。ModeRust 与 ModeAuto 在 production 行为上一致。
func TestEngineProviderModeRustWithoutAllowFallbackFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EOS_CORE_PATH", filepath.Join(dir, "no-such-binary"))
	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode:            engineprovider.ModeRust,
		Sidecar:         sidecar.ProcessOptions{},
		RequiredMethods: requiredMethodsForCutover(),
		AllowFallback:   false,
	})
	if err == nil {
		t.Fatal("Select(ModeRust, no fallback) succeeded with missing binary; expected error")
	}
}

// TestEngineProviderAllowFallbackTrueStillRequiresEngineForParity 验证 parity mode
// 即便 AllowFallback=true，仍然需要至少一个可用的 engine（rust 或 legacy）。这是为了
// 防止 parity 测试静默通过一个空 stub。
func TestEngineProviderAllowFallbackTrueStillRequiresEngineForParity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EOS_CORE_PATH", filepath.Join(dir, "no-such-binary"))
	t.Setenv("EOS_CORE_ALLOW_FALLBACK", "1")
	t.Setenv("EOS_CORE_ENGINE", "parity")

	// 模拟 parity 场景：start 失败 + legacy 没传，必须 error。
	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode:            engineprovider.ModeParity,
		Sidecar:         sidecar.ProcessOptions{},
		RequiredMethods: requiredMethodsForCutover(),
		AllowFallback:   true,
		// Legacy 不传 → 走 selectLegacy("legacy core engine is not configured")
		StartRemote: staticStartRemote(nil, errors.New("rust sidecar unavailable")),
	})
	if err == nil {
		t.Fatal("parity mode should error when both rust and legacy are unavailable")
	}
	if !strings.Contains(err.Error(), "parity") {
		t.Logf("note: parity error = %v", err)
	}
}

// TestEngineProviderResolveModeRejectsUnknown 验证 ResolveMode 拒绝未知模式，
// 防止 typo 走错 production 路径。
func TestEngineProviderResolveModeRejectsUnknown(t *testing.T) {
	for _, value := range []string{"foo", "auto-but-also-legacy", "all"} {
		if _, err := engineprovider.ResolveMode(value); err == nil {
			t.Errorf("ResolveMode(%q) succeeded; expected error", value)
		}
	}
}

// TestEngineProviderResolveModeAliasesForDevOnly 验证 legacy / parity 是 dev-only
// 别名，但解析仍然合法。生产路径不应使用这些 alias。
func TestEngineProviderResolveModeAliasesForDevOnly(t *testing.T) {
	for _, value := range []string{"legacy", "parity", "eino", "go"} {
		mode, err := engineprovider.ResolveMode(value)
		if err != nil {
			t.Errorf("ResolveMode(%q) error = %v", value, err)
		}
		if mode == engineprovider.ModeAuto {
			t.Errorf("ResolveMode(%q) resolved to ModeAuto; expected dev-only alias", value)
		}
	}
}

// --- resolution-level smoke tests (real manifest + binary) ---

// TestEngineProviderRejectsBadChecksum 验证当 manifest 声明的 sha256 与实际
// binary 不一致时，engineprovider 直接返回 ErrChecksumMismatch，
// 不允许回退 legacy。这条路径在 release build 永远 fail-fast。
func TestEngineProviderRejectsBadChecksum(t *testing.T) {
	dir := t.TempDir()
	binaryName := eosCoreBinaryName()
	binaryPath := filepath.Join(dir, binaryName)
	content := []byte("fake closed core binary")
	if err := os.WriteFile(binaryPath, content, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	manifestPath := writeTestManifest(t, dir, sidecarManifestOptions{
		Binary:   binaryName,
		Target:   currentTargetTriple(t),
		Features: fullFeatures(t), // 覆盖全部 required methods，让 feature check 跳过
		// 故意写错 sha256：使用全零 hash 触发 checksum mismatch。
		OverrideSHA256: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		// 跳过签名校验：让 checksum 真正成为第一个 fail-fast 锚点。
		AllowDevPlaceholder: true,
	})

	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode: engineprovider.ModeAuto,
		Sidecar: sidecar.ProcessOptions{
			Resolve: sidecar.ResolveOptions{
				ManifestPath:        manifestPath,
				AllowDevPlaceholder: true,
			},
			// 注意：VerifyChecksum 在 ProcessOptions 顶层，由 StartProcess
			// 透传到 ResolveOptions，Resolve.VerifyChecksum 会被覆盖。
			VerifyChecksum:      true,
			AllowDevPlaceholder: true,
		},
		RequiredMethods: requiredMethodsForCutover(),
		AllowFallback:   false,
	})
	if err == nil {
		t.Fatal("Select() succeeded with bad checksum; expected ErrChecksumMismatch")
	}
	if !errors.Is(err, sidecar.ErrChecksumMismatch) {
		t.Fatalf("Select() err = %v, want sidecar.ErrChecksumMismatch", err)
	}
}

// TestEngineProviderRejectsBadChecksumWithLegacyLoaded 验证 production 模式
// (AllowFallback=false) 下，bad checksum 永远不触发 legacy。
func TestEngineProviderRejectsBadChecksumWithLegacyLoaded(t *testing.T) {
	dir := t.TempDir()
	binaryName := eosCoreBinaryName()
	binaryPath := filepath.Join(dir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("fake closed core binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	manifestPath := writeTestManifest(t, dir, sidecarManifestOptions{
		Binary:              binaryName,
		Target:              currentTargetTriple(t),
		Features:            fullFeatures(t),
		OverrideSHA256:      "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AllowDevPlaceholder: true,
	})

	legacy := &fallbackProbeEngine{}
	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode: engineprovider.ModeAuto,
		Sidecar: sidecar.ProcessOptions{
			Resolve: sidecar.ResolveOptions{
				ManifestPath:        manifestPath,
				AllowDevPlaceholder: true,
			},
			VerifyChecksum:      true,
			AllowDevPlaceholder: true,
		},
		RequiredMethods: requiredMethodsForCutover(),
		Legacy:          legacy,
		AllowFallback:   false, // production 行为
	})
	if err == nil {
		t.Fatal("Select() succeeded with bad checksum; expected error")
	}
	if legacy.touched {
		t.Fatal("legacy engine was touched on checksum error; production must not touch legacy")
	}
	if !errors.Is(err, sidecar.ErrChecksumMismatch) {
		t.Fatalf("Select() err = %v, want sidecar.ErrChecksumMismatch", err)
	}
}

// TestEngineProviderRejectsBadSignature 验证当 manifest 签名与 embedded
// public key 不匹配时，engineprovider 直接返回 ErrSignatureInvalid，
// 不允许回退 legacy。production release gate 必须在签名层先 fail-fast。
func TestEngineProviderRejectsBadSignature(t *testing.T) {
	dir := t.TempDir()
	binaryName := eosCoreBinaryName()
	binaryPath := filepath.Join(dir, binaryName)
	content := []byte("fake closed core binary")
	if err := os.WriteFile(binaryPath, content, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	manifestPath := writeTestManifest(t, dir, sidecarManifestOptions{
		Binary:   binaryName,
		Target:   currentTargetTriple(t),
		Features: []string{"initialize", "session/list"},
		// 注入一组长度合法但随机生成的 64 字节作为 "ed25519" 签名：
		// 长度正确 → 能过 base64 解码；值随机 → 必然 verify 失败。
		OverrideSignature: "ed25519:" + randomBase64Signature(t),
		RequireSignature:  true,
	})

	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode: engineprovider.ModeAuto,
		Sidecar: sidecar.ProcessOptions{
			Resolve: sidecar.ResolveOptions{
				ManifestPath: manifestPath,
			},
			// RequireSignature 顶层字段：StartProcess 透传到 ResolveOptions。
			RequireSignature: true,
		},
		RequiredMethods: requiredMethodsForCutover(),
		AllowFallback:   false,
	})
	if err == nil {
		t.Fatal("Select() succeeded with bad signature; expected ErrSignatureInvalid")
	}
	if !errors.Is(err, sidecar.ErrSignatureInvalid) {
		t.Fatalf("Select() err = %v, want sidecar.ErrSignatureInvalid", err)
	}
}

// TestEngineProviderRejectsBadSignatureWithLegacyLoaded 验证 production 模式
// (AllowFallback=false) 下，bad signature 永远不触发 legacy。
func TestEngineProviderRejectsBadSignatureWithLegacyLoaded(t *testing.T) {
	dir := t.TempDir()
	binaryName := eosCoreBinaryName()
	binaryPath := filepath.Join(dir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("fake closed core binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	manifestPath := writeTestManifest(t, dir, sidecarManifestOptions{
		Binary:            binaryName,
		Target:            currentTargetTriple(t),
		Features:          []string{"initialize", "session/list"},
		OverrideSignature: "ed25519:" + randomBase64Signature(t),
		RequireSignature:  true,
	})

	legacy := &fallbackProbeEngine{}
	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode: engineprovider.ModeAuto,
		Sidecar: sidecar.ProcessOptions{
			Resolve: sidecar.ResolveOptions{
				ManifestPath: manifestPath,
			},
			RequireSignature: true,
		},
		RequiredMethods: requiredMethodsForCutover(),
		Legacy:          legacy,
		AllowFallback:   false, // production 行为
	})
	if err == nil {
		t.Fatal("Select() succeeded with bad signature; expected error")
	}
	if legacy.touched {
		t.Fatal("legacy engine was touched on signature error; production must not touch legacy")
	}
	if !errors.Is(err, sidecar.ErrSignatureInvalid) {
		t.Fatalf("Select() err = %v, want sidecar.ErrSignatureInvalid", err)
	}
}

// TestEngineProviderRejectsManifestTargetMismatch 验证当 manifest 的 target
// 与运行平台不一致时，engineprovider 直接返回 ErrTargetMismatch，不允许
// 回退 legacy。target 漂移在 release 必须 fail-fast。
func TestEngineProviderRejectsManifestTargetMismatch(t *testing.T) {
	dir := t.TempDir()
	binaryName := eosCoreBinaryName()
	binaryPath := filepath.Join(dir, binaryName)
	content := []byte("fake closed core binary")
	if err := os.WriteFile(binaryPath, content, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	manifestPath := writeTestManifest(t, dir, sidecarManifestOptions{
		Binary:              binaryName,
		Target:              "aarch64-apple-darwin", // 与当前平台不匹配
		Features:            []string{"initialize", "session/list"},
		AllowDevPlaceholder: true,
	})

	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode: engineprovider.ModeAuto,
		Sidecar: sidecar.ProcessOptions{
			Resolve: sidecar.ResolveOptions{
				ManifestPath: manifestPath,
			},
			AllowDevPlaceholder: true,
		},
		RequiredMethods: requiredMethodsForCutover(),
		AllowFallback:   false,
	})
	if err == nil {
		t.Fatal("Select() succeeded with target mismatch; expected ErrTargetMismatch")
	}
	if !errors.Is(err, sidecar.ErrTargetMismatch) {
		t.Fatalf("Select() err = %v, want sidecar.ErrTargetMismatch", err)
	}
}

// TestEngineProviderRejectsManifestFeatureMissing 验证 manifest 缺少 required
// feature 时，engineprovider 直接返回 ErrFeatureMissing，不允许回退 legacy。
func TestEngineProviderRejectsManifestFeatureMissing(t *testing.T) {
	dir := t.TempDir()
	binaryName := eosCoreBinaryName()
	binaryPath := filepath.Join(dir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("fake closed core binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	// 故意只声明 initialize，缺少 session/list → 触发 ErrFeatureMissing。
	manifestPath := writeTestManifest(t, dir, sidecarManifestOptions{
		Binary:              binaryName,
		Target:              currentTargetTriple(t),
		Features:            []string{"initialize"},
		AllowDevPlaceholder: true,
	})

	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode: engineprovider.ModeAuto,
		Sidecar: sidecar.ProcessOptions{
			Resolve: sidecar.ResolveOptions{
				ManifestPath:        manifestPath,
				AllowDevPlaceholder: true,
			},
			// RequiredFeatures 顶层字段：StartProcess 透传到 ResolveOptions。
			RequiredFeatures:    []string{"initialize", "session/list"},
			AllowDevPlaceholder: true,
		},
		RequiredMethods: requiredMethodsForCutover(),
		AllowFallback:   false,
	})
	if err == nil {
		t.Fatal("Select() succeeded with feature missing; expected ErrFeatureMissing")
	}
	if !errors.Is(err, sidecar.ErrFeatureMissing) {
		t.Fatalf("Select() err = %v, want sidecar.ErrFeatureMissing", err)
	}
}

// TestEngineProviderRejectsManifestFeatureMissingWithLegacyLoaded 验证
// production 模式 (AllowFallback=false) 下，manifest 缺 feature 永远
// 不触发 legacy。feature drift 与 ErrMissingMethods 用例对称。
func TestEngineProviderRejectsManifestFeatureMissingWithLegacyLoaded(t *testing.T) {
	dir := t.TempDir()
	binaryName := eosCoreBinaryName()
	binaryPath := filepath.Join(dir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("fake closed core binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	manifestPath := writeTestManifest(t, dir, sidecarManifestOptions{
		Binary:              binaryName,
		Target:              currentTargetTriple(t),
		Features:            []string{"initialize"},
		AllowDevPlaceholder: true,
	})

	legacy := &fallbackProbeEngine{}
	_, err := engineprovider.Select(context.Background(), engineprovider.Options{
		Mode: engineprovider.ModeAuto,
		Sidecar: sidecar.ProcessOptions{
			Resolve: sidecar.ResolveOptions{
				ManifestPath:        manifestPath,
				AllowDevPlaceholder: true,
			},
			RequiredFeatures:    []string{"initialize", "session/list"},
			AllowDevPlaceholder: true,
		},
		RequiredMethods: requiredMethodsForCutover(),
		Legacy:          legacy,
		AllowFallback:   false, // production 行为
	})
	if err == nil {
		t.Fatal("Select() succeeded with feature missing; expected error")
	}
	if legacy.touched {
		t.Fatal("legacy engine was touched on feature-missing error; production must not touch legacy")
	}
	if !errors.Is(err, sidecar.ErrFeatureMissing) {
		t.Fatalf("Select() err = %v, want sidecar.ErrFeatureMissing", err)
	}
}

// --- helpers ---

func staticStartRemote(remote engineprovider.RemoteEngine, err error) engineprovider.StartRemoteFunc {
	return func(context.Context, sidecar.ProcessOptions) (engineprovider.RemoteEngine, error) {
		return remote, err
	}
}

// assertRustOnlyError 断言 err 是从 rust 解析/启动路径冒出来的（不允许
// 在回退 legacy 之后才返回 error），并显式检查 error message 不含
// "fallback" / "legacy" 等暗示回退的字样。这是 release gate 的"绝对
// invariant"——production 路径下不能掩盖 rust 错误。
//
// 该函数不强制要求 errors.Is 命中具体 sentinel（os.Stat 错误不会被
// 显式 wrap），而是断言两条线：
//   1. error message 含 engineprovider 注入的 rust-only 模式前缀
//      （"auto mode (rust-only)" / "rust mode" / "parity mode"）。
//   2. error message 不含 "fallback" / "legacy" 等回退提示。
func assertRustOnlyError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	low := strings.ToLower(err.Error())
	rustModeHints := []string{
		"auto mode (rust-only)",
		"rust mode:",
		"rust-only",
		"parity: rust",
	}
	hasRustModeHint := false
	for _, hint := range rustModeHints {
		if strings.Contains(low, hint) {
			hasRustModeHint = true
			break
		}
	}
	if !hasRustModeHint {
		t.Fatalf("err = %q; expected engineprovider rust-only fail-fast prefix, but got neither %q; production must surface the rust-side error directly",
			err.Error(), strings.Join(rustModeHints, " | "))
	}
	for _, hint := range []string{"fallback", "legacy engine", "sharedcore"} {
		if idx := strings.Index(low, hint); idx >= 0 {
			// Print context window for debugging the false-positive.
			from := idx - 8
			if from < 0 {
				from = 0
			}
			to := idx + len(hint) + 8
			if to > len(low) {
				to = len(low)
			}
			t.Fatalf("err = %q contains fallback hint %q at offset %d (...%q...); production must not advertise fallback on rust failure",
				err.Error(), hint, idx, low[from:to])
		}
	}
}

// fallbackProbeEngine 在被 selectLegacy 使用时记录 touched=true，
// 用于断言 release gate 不会把它接进 production 路径。
type fallbackProbeEngine struct {
	touched bool
}

func (f *fallbackProbeEngine) State() coreapi.StateService           { return nil }
func (f *fallbackProbeEngine) Workspaces() coreapi.WorkspaceService   { return nil }
func (f *fallbackProbeEngine) Sessions() coreapi.SessionService       { return nil }
func (f *fallbackProbeEngine) MCP() coreapi.MCPService               { return nil }
func (f *fallbackProbeEngine) LSP() coreapi.LSPService               { return nil }
func (f *fallbackProbeEngine) Config() coreapi.ConfigService         { return nil }
func (f *fallbackProbeEngine) Permissions() coreapi.PermissionService { return nil }
func (f *fallbackProbeEngine) Extensions() coreapi.ExtensionService  { return nil }
func (f *fallbackProbeEngine) Context() coreapi.ContextService       { return nil }
func (f *fallbackProbeEngine) Usage() coreapi.UsageService           { return nil }
func (f *fallbackProbeEngine) Versions() coreapi.VersionService     { return nil }
func (f *fallbackProbeEngine) Tasks() coreapi.TaskService           { return nil }
func (f *fallbackProbeEngine) Modes() coreapi.ModeService           { return nil }
func (f *fallbackProbeEngine) Models() coreapi.ModelService         { return nil }
func (f *fallbackProbeEngine) RemoteWorkspaces() coreapi.RemoteWorkspaceService {
	return nil
}
func (f *fallbackProbeEngine) Git() coreapi.GitService         { return nil }
func (f *fallbackProbeEngine) Insights() coreapi.InsightService { return nil }
func (f *fallbackProbeEngine) Memory() coreapi.MemoryService   { return nil }
func (f *fallbackProbeEngine) Roles() coreapi.RoleService      { return nil }
func (f *fallbackProbeEngine) Turns() coreapi.TurnService      { return nil }
func (f *fallbackProbeEngine) Approvals() coreapi.ApprovalService {
	return nil
}
func (f *fallbackProbeEngine) Inquiries() coreapi.InquiryService { return nil }
func (f *fallbackProbeEngine) Agents() coreapi.AgentService     { return nil }
func (f *fallbackProbeEngine) Tools() coreapi.ToolExecutor      { return nil }
func (f *fallbackProbeEngine) ToolCatalog() coreapi.ToolCatalogService {
	return nil
}
func (f *fallbackProbeEngine) ToolTelemetry() coreapi.ToolTelemetryService {
	return nil
}
func (f *fallbackProbeEngine) Events() coreapi.EventSubscriber   { return nil }
func (f *fallbackProbeEngine) Sandbox() coreapi.SandboxService   { return nil }
func (f *fallbackProbeEngine) Diagnostics() coreapi.DiagnosticsService {
	return nil
}

func (f *fallbackProbeEngine) MarkTouched() { f.touched = true }

// fullFeatures 覆盖整个 required method set，作为 happy-path 用的 baseline。
// checksum / target / signature 这些错误锚点出现得更早，所以 manifest features
// 必须先把 feature check 放过去，让真正的 error 锚点有机会"说话"。
func fullFeatures(t *testing.T) []string {
	t.Helper()
	return append([]string(nil), requiredMethodsForCutover()...)
}

// eosCoreBinaryName 返回当前平台下可被 exec.Cmd 接受的 eos-core binary 名。
// Windows 要求 .exe 扩展名，否则 exec.LookPath / Cmd.Start 会因为
// "executable file not found in %PATH%" 失败。
func eosCoreBinaryName() string {
	if runtime.GOOS == "windows" {
		return "eos-core.exe"
	}
	return "eos-core"
}

// sidecarManifestOptions 描述一个 fake manifest 的字段；测试通过显式覆盖
// 字段来构造 good / tampered 变体。
type sidecarManifestOptions struct {
	Binary              string
	Target              string
	Features            []string
	OverrideSHA256      string
	OverrideSignature   string
	AllowDevPlaceholder bool
	RequireSignature    bool
}

func writeTestManifest(t *testing.T, dir string, opts sidecarManifestOptions) string {
	t.Helper()
	binaryPath := filepath.Join(dir, opts.Binary)
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	sum := sha256.Sum256(content)
	sha := "sha256:" + hex.EncodeToString(sum[:])
	if strings.TrimSpace(opts.OverrideSHA256) != "" {
		sha = opts.OverrideSHA256
	}
	signature := sidecar.SignaturePlaceholder
	if strings.TrimSpace(opts.OverrideSignature) != "" {
		signature = opts.OverrideSignature
	}
	manifest := sidecar.Manifest{
		SchemaVersion: sidecar.ManifestSchemaVersion,
		CoreVersion:   "0.1.0",
		APIVersion:    sidecar.DefaultAPIVersion,
		Target:        opts.Target,
		Binary:        opts.Binary,
		SHA256:        sha,
		Signature:     signature,
		MinCLIVersion: "v0.3.0",
		Features:      append([]string(nil), opts.Features...),
	}
	if opts.RequireSignature {
		// 任何"ed25519:" 形式 + 随机 64 字节会走到 verify 失败分支。
		// 保留 placeholder 给 AllowDevPlaceholder=true 场景。
		if !strings.HasPrefix(signature, "ed25519:") {
			manifest.Signature = "ed25519:" + randomBase64Signature(t)
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	path := filepath.Join(dir, sidecar.DefaultManifestName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

// randomBase64Signature 返回一组 base64(std) 编码的 64 字节确定性数据。
// 它会通过 base64 解码与 ed25519 长度校验，但 verify 必然失败——这是
// "长度合法但签名错" 的最干净的构造方式。
func randomBase64Signature(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 64)
	for i := range raw {
		raw[i] = byte(i + 1) // 非全 0 即可，verify 必然失败
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	// 自检：base64 解码后必须仍然得到 64 字节。
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err != nil || len(decoded) != 64 {
		t.Fatalf("randomBase64Signature sanity: decoded=%d err=%v", len(decoded), err)
	}
	return encoded
}

// currentTargetTriple 返回当前运行平台的 manifest target triple。
// 用作 happy-path manifest 的 target 字段。
func currentTargetTriple(t *testing.T) string {
	t.Helper()
	// 直接通过 sidecar.TargetTriple 计算，不假设具体 GOOS/GOARCH。
	out := sidecar.TargetTriple(goEnv(t, "GOOS"), goEnv(t, "GOARCH"))
	if out == "" {
		// fallback 到 runtime 值（测试运行在 host 上时与预期一致）。
		out = sidecar.TargetTriple(runtime.GOOS, runtime.GOARCH)
	}
	if out == "" {
		t.Skip("unable to determine current target triple")
	}
	return out
}

func goEnv(t *testing.T, key string) string {
	t.Helper()
	if v := os.Getenv(key); v != "" {
		return v
	}
	return os.Getenv(strings.ToUpper(key))
}

// 防 止 format 包被自动移除（_ = fmt.Sprintf）。
var _ = fmt.Sprintf

// methodsCaller 返回 initialize 响应时带 methods 列表，标志 initialize 成功。
type methodsCaller struct {
	methods []string
}

func (m methodsCaller) Call(_ context.Context, method string, _ any, out any) error {
	switch method {
	case protocoljsonrpc.MethodInitialize:
		if target, ok := out.(*coreapijsonrpc.InitializeResult); ok {
			*target = coreapijsonrpc.InitializeResult{
				ServerName:      "eos-core-fake",
				ProtocolVersion: "v1",
				Methods:         append([]string(nil), m.methods...),
			}
		}
		return nil
	default:
		return coreapi.ErrUnsupported
	}
}

// failureCaller 在 initialize 阶段直接返回 error。
type failureCaller struct {
	err error
}

func (f failureCaller) Call(_ context.Context, method string, _ any, _ any) error {
	if method == protocoljsonrpc.MethodInitialize {
		return f.err
	}
	return coreapi.ErrUnsupported
}

// 防止误删导入的副作用
var (
	_ = os.Getenv
)
