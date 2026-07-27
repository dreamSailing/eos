package sidecar

// 本文件专测 release gate（EOS_RELEASE_ARTIFACT_CHECK）。
//
// 守护 invariant：release 场景下
//   - 占位签名（unsigned-development-placeholder）无法通过校验，无论调用方是否
//     把 AllowDevPlaceholder 设成 true；
//   - 必须显式注入生产公钥（EOS_SIGNATURE_PUBLIC_KEY 或 PublicKeyPath），否则
//     fail-fast，禁止 defaultPublicKeyPEM 这个 smoke-test key 静默生效；
//   - dev 场景（未设该 env）保持原行为：占位签名受 AllowDevPlaceholder 控制，
//     smoke-test key 仍然可用。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writePlaceholderManifest 在 dir/target/ 下写一份带占位签名的 manifest 与对应二进制，
// 返回 manifest 路径。用于验证 release gate 拦截占位签名。
func writePlaceholderManifest(t *testing.T, dir, target string) (manifestPath, binaryPath string) {
	t.Helper()
	tdir := filepath.Join(dir, target)
	if err := os.MkdirAll(tdir, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	binaryName := "eos-core"
	binaryPath = filepath.Join(tdir, binaryName)
	content := []byte("fake closed core")
	if err := os.WriteFile(binaryPath, content, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	sum := sha256.Sum256(content)
	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CoreVersion:   "0.1.0",
		APIVersion:    DefaultAPIVersion,
		Target:        target,
		Binary:        binaryName,
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		Signature:     SignaturePlaceholder,
		Features:      []string{"initialize"},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestPath = filepath.Join(tdir, DefaultManifestName)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return manifestPath, binaryPath
}

// TestReleaseArtifactCheckDefaultsOff 验证未设置环境变量时判定为 dev 模式。
func TestReleaseArtifactCheckDefaultsOff(t *testing.T) {
	t.Setenv(EnvReleaseArtifactCheck, "")
	if ReleaseArtifactCheck() {
		t.Fatalf("ReleaseArtifactCheck()=true with %s unset, want false", EnvReleaseArtifactCheck)
	}
}

// TestReleaseArtifactCheckTruthy 验证任意非空值都视为 release 模式。
func TestReleaseArtifactCheckTruthy(t *testing.T) {
	for _, value := range []string{"1", "true", "release", "yes"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(EnvReleaseArtifactCheck, value)
			if !ReleaseArtifactCheck() {
				t.Fatalf("ReleaseArtifactCheck()=false with %s=%q, want true", EnvReleaseArtifactCheck, value)
			}
		})
	}
}

// TestEnforceReleaseGateNoOpInDev 验证 dev 模式下 gate 是 no-op：
// AllowDevPlaceholder 与 PublicKeyPath 都保持调用方原值。
func TestEnforceReleaseGateNoOpInDev(t *testing.T) {
	t.Setenv(EnvReleaseArtifactCheck, "")
	opts := ResolveOptions{AllowDevPlaceholder: true}
	out, err := enforceReleaseGate(opts)
	if err != nil {
		t.Fatalf("enforceReleaseGate() dev error = %v", err)
	}
	if !out.AllowDevPlaceholder {
		t.Fatalf("dev mode must preserve AllowDevPlaceholder=true, got false")
	}
}

// TestEnforceReleaseGateForcesPlaceholderOff 验证 release 模式下即使调用方
// 显式请求 AllowDevPlaceholder=true，gate 也会强制改写成 false。
// 这正是"占位签名形同虚设"漏洞的修复点：release 场景无法通过把布尔位设成 true 绕过。
func TestEnforceReleaseGateForcesPlaceholderOff(t *testing.T) {
	t.Setenv(EnvReleaseArtifactCheck, "1")
	t.Setenv(EnvSignaturePublicKey, writeTempPublicKey(t))

	opts := ResolveOptions{AllowDevPlaceholder: true}
	out, err := enforceReleaseGate(opts)
	if err != nil {
		t.Fatalf("enforceReleaseGate() error = %v", err)
	}
	if out.AllowDevPlaceholder {
		t.Fatalf("release mode must force AllowDevPlaceholder=false, got true")
	}
}

// TestEnforceReleaseGateRequiresProductionKey 验证 release 模式下未注入生产公钥
// 时直接 fail-fast，禁止 defaultPublicKeyPEM（smoke-test key）静默生效。
func TestEnforceReleaseGateRequiresProductionKey(t *testing.T) {
	t.Setenv(EnvReleaseArtifactCheck, "1")
	t.Setenv(EnvSignaturePublicKey, "")

	_, err := enforceReleaseGate(ResolveOptions{})
	if err == nil {
		t.Fatalf("enforceReleaseGate() release mode without production key: expected error, got nil")
	}
}

// TestResolveBinaryReleaseRejectsPlaceholderEvenWhenAllowed 端到端验证：
// release 模式 + 调用方传 AllowDevPlaceholder=true + 占位签名 manifest，
// ResolveBinary 必须拒绝（不能因为布尔位为 true 放行）。
func TestResolveBinaryReleaseRejectsPlaceholderEvenWhenAllowed(t *testing.T) {
	t.Setenv(EnvCorePath, "")
	t.Setenv(EnvReleaseArtifactCheck, "1")
	t.Setenv(EnvSignaturePublicKey, writeTempPublicKey(t))

	root := t.TempDir()
	manifestPath, _ := writePlaceholderManifest(t, root, "x86_64-unknown-linux-gnu")

	_, err := ResolveBinary(ResolveOptions{
		ManifestPath:        manifestPath,
		GOOS:                "linux",
		GOARCH:              "amd64",
		RequireSignature:    true,
		AllowDevPlaceholder: true, // 调用方试图打开 dev 后门
	})
	if !errors.Is(err, ErrSignaturePlaceholder) {
		t.Fatalf("ResolveBinary() release + AllowDevPlaceholder=true + placeholder: err = %v, want ErrSignaturePlaceholder", err)
	}
}

// TestResolveBinaryReleaseRejectsPlaceholderWhenForbidden 同样场景但调用方没设
// AllowDevPlaceholder；release gate 必须依然拒绝。
func TestResolveBinaryReleaseRejectsPlaceholderWhenForbidden(t *testing.T) {
	t.Setenv(EnvCorePath, "")
	t.Setenv(EnvReleaseArtifactCheck, "1")
	t.Setenv(EnvSignaturePublicKey, writeTempPublicKey(t))

	root := t.TempDir()
	manifestPath, _ := writePlaceholderManifest(t, root, "x86_64-unknown-linux-gnu")

	_, err := ResolveBinary(ResolveOptions{
		ManifestPath:     manifestPath,
		GOOS:             "linux",
		GOARCH:           "amd64",
		RequireSignature: true,
	})
	if !errors.Is(err, ErrSignaturePlaceholder) {
		t.Fatalf("ResolveBinary() release + placeholder: err = %v, want ErrSignaturePlaceholder", err)
	}
}

// TestResolveBinaryReleaseFailsWithoutProductionKey 验证 release 模式下未注入
// 生产公钥时，即使 manifest 用了真实签名，ResolveBinary 也直接 fail-fast，
// 不静默回退到 defaultPublicKeyPEM。
func TestResolveBinaryReleaseFailsWithoutProductionKey(t *testing.T) {
	t.Setenv(EnvCorePath, "")
	t.Setenv(EnvReleaseArtifactCheck, "1")
	t.Setenv(EnvSignaturePublicKey, "")

	root := t.TempDir()
	manifestPath, _ := writePlaceholderManifest(t, root, "x86_64-unknown-linux-gnu")

	_, err := ResolveBinary(ResolveOptions{
		ManifestPath:     manifestPath,
		GOOS:             "linux",
		GOARCH:           "amd64",
		RequireSignature: true,
	})
	if err == nil {
		t.Fatalf("ResolveBinary() release without production key: expected fail-fast, got nil")
	}
}

// TestResolveBinaryReleaseRejectsPlaceholderWithoutRequireSignature 验证 release
// 场景下即便调用方忘记把 RequireSignature 设为 true，占位签名也照样被拒——
// 不能因为走"可选签名校验"分支而放行未签名 manifest。
func TestResolveBinaryReleaseRejectsPlaceholderWithoutRequireSignature(t *testing.T) {
	t.Setenv(EnvCorePath, "")
	t.Setenv(EnvReleaseArtifactCheck, "1")
	t.Setenv(EnvSignaturePublicKey, writeTempPublicKey(t))

	root := t.TempDir()
	manifestPath, _ := writePlaceholderManifest(t, root, "x86_64-unknown-linux-gnu")

	_, err := ResolveBinary(ResolveOptions{
		ManifestPath:   manifestPath,
		GOOS:           "linux",
		GOARCH:         "amd64",
		VerifyChecksum: true,
		// 故意不设 RequireSignature，走可选签名分支。
	})
	if !errors.Is(err, ErrSignaturePlaceholder) {
		t.Fatalf("ResolveBinary() release + RequireSignature=false + placeholder: err = %v, want ErrSignaturePlaceholder", err)
	}
}

// TestResolveBinaryDevStillAllowsPlaceholder 验证修复没有破坏 dev 工作流：
// 未设 release env 时，AllowDevPlaceholder=true 仍然放行占位签名，方便本地开发。
func TestResolveBinaryDevStillAllowsPlaceholder(t *testing.T) {
	t.Setenv(EnvCorePath, "")
	t.Setenv(EnvReleaseArtifactCheck, "")

	root := t.TempDir()
	manifestPath, _ := writePlaceholderManifest(t, root, "x86_64-unknown-linux-gnu")

	resolved, err := ResolveBinary(ResolveOptions{
		ManifestPath:        manifestPath,
		GOOS:                "linux",
		GOARCH:              "amd64",
		RequireSignature:    true,
		AllowDevPlaceholder: true,
	})
	if err != nil {
		t.Fatalf("ResolveBinary() dev + AllowDevPlaceholder=true: error = %v", err)
	}
	if resolved.Manifest == nil {
		t.Fatalf("resolved.Manifest is nil")
	}
}

// writeTempPublicKey 写一份临时 ed25519 公钥 PEM 文件，模拟生产环境注入的公钥。
func writeTempPublicKey(t *testing.T) string {
	t.Helper()
	pub, _ := generateTestKeypair(t)
	pubPEM := exportPublicKeyPEM(t, pub)
	dir := t.TempDir()
	path := filepath.Join(dir, "release-public-key.pem")
	if err := os.WriteFile(path, pubPEM, 0o644); err != nil {
		t.Fatalf("write release public key: %v", err)
	}
	return path
}
