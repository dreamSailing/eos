package sidecar

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withEmbeddedCacheDir 把内嵌释放目录指向临时目录，测试后还原。
func withEmbeddedCacheDir(t *testing.T, dir string) {
	t.Helper()
	orig := embeddedCacheDir
	embeddedCacheDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { embeddedCacheDir = orig })
}

// 内嵌通道全流程：释放 → manifest/binary 落盘 → 幂等复用 → 校验一致。
func TestMaterializeEmbeddedReleasesAndIsIdempotent(t *testing.T) {
	if embeddedCoreSidecar == nil {
		t.Skip("current platform has no embedded sidecar")
	}
	cache := t.TempDir()
	withEmbeddedCacheDir(t, cache)

	manifestPath, err := materializeEmbedded(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("materializeEmbedded: %v", err)
	}
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("load released manifest: %v", err)
	}
	if err := manifest.VerifyBinary(filepath.Join(filepath.Dir(manifestPath), manifest.Binary)); err != nil {
		t.Fatalf("released binary failed checksum: %v", err)
	}

	// 二次调用幂等：不报错、路径不变。
	again, err := materializeEmbedded(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("second materializeEmbedded: %v", err)
	}
	if again != manifestPath {
		t.Fatalf("expected stable manifest path %s, got %s", manifestPath, again)
	}
}

// 释放目录按内容寻址：伪造同 triple 下的陈旧目录应被清理。
func TestMaterializeEmbeddedCleansStaleDirs(t *testing.T) {
	if embeddedCoreSidecar == nil {
		t.Skip("current platform has no embedded sidecar")
	}
	cache := t.TempDir()
	withEmbeddedCacheDir(t, cache)

	// 陈旧目录挂在产物实际 triple 下（manifest.Target，而非宿主主选 triple）。
	_, mf, _ := embeddedCoreSidecar()
	manifest, err := LoadManifestBytes(mf)
	if err != nil {
		t.Fatalf("parse embedded manifest: %v", err)
	}
	stale := filepath.Join(cache, manifest.Target, "000000000000")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "junk"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := materializeEmbedded(runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatalf("materializeEmbedded: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale dir should be cleaned, stat err=%v", err)
	}
}

// 内存校验：内嵌二进制与 manifest 的 sha256 不一致（串包/损坏）必须报错。
func TestMaterializeEmbeddedRejectsTamperedBinary(t *testing.T) {
	orig := embeddedCoreSidecar
	bin, mf, ok := orig()
	if !ok {
		t.Skip("current platform has no embedded sidecar")
	}
	tampered := append([]byte(nil), bin...)
	tampered[0] ^= 0xFF
	embeddedCoreSidecar = func() ([]byte, []byte, bool) { return tampered, mf, true }
	t.Cleanup(func() { embeddedCoreSidecar = orig })

	withEmbeddedCacheDir(t, t.TempDir())
	if _, err := materializeEmbedded(runtime.GOOS, runtime.GOARCH); err == nil {
		t.Fatal("tampered embedded binary must be rejected")
	}
}
