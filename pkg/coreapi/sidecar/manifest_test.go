package sidecar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestLoadAndVerifyBinary(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "eos-core")
	content := []byte("fake closed core")
	if err := os.WriteFile(binaryPath, content, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	sum := sha256.Sum256(content)
	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CoreVersion:   "0.1.0",
		APIVersion:    DefaultAPIVersion,
		Target:        "x86_64-test",
		Binary:        "eos-core",
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		Signature:     "test-signature",
		MinCLIVersion: "v0.3.0",
		Features:      []string{"initialize"},
	}
	manifestPath := filepath.Join(dir, DefaultManifestName)
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	resolved, err := loaded.BinaryPath(manifestPath)
	if err != nil {
		t.Fatalf("BinaryPath() error = %v", err)
	}
	if resolved != filepath.Clean(binaryPath) {
		t.Fatalf("BinaryPath()=%q, want %q", resolved, binaryPath)
	}
	if err := loaded.VerifyBinary(resolved); err != nil {
		t.Fatalf("VerifyBinary() error = %v", err)
	}
}

func TestManifestVerifyBinaryChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "eos-core")
	if err := os.WriteFile(binaryPath, []byte("fake closed core"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	manifest := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CoreVersion:   "0.1.0",
		APIVersion:    DefaultAPIVersion,
		Target:        "x86_64-test",
		Binary:        "eos-core",
		SHA256:        "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}
	err := manifest.VerifyBinary(binaryPath)
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("VerifyBinary() error = %v, want ErrChecksumMismatch", err)
	}
}

func TestManifestRequireSignature(t *testing.T) {
	manifest := Manifest{Signature: "signed"}
	if err := manifest.RequireSignature(); err != nil {
		t.Fatalf("RequireSignature() error = %v", err)
	}
	manifest.Signature = ""
	if err := manifest.RequireSignature(); !errors.Is(err, ErrSignatureMissing) {
		t.Fatalf("RequireSignature() error = %v, want ErrSignatureMissing", err)
	}
	manifest.Signature = "unsigned-development-placeholder"
	if err := manifest.RequireSignature(); !errors.Is(err, ErrSignaturePlaceholder) {
		t.Fatalf("RequireSignature() error = %v, want ErrSignaturePlaceholder", err)
	}
}

func TestManifestRequireTarget(t *testing.T) {
	manifest := Manifest{Target: "x86_64-unknown-linux-musl"}
	if err := manifest.RequireTarget([]string{"x86_64-unknown-linux-musl"}); err != nil {
		t.Fatalf("RequireTarget() error = %v", err)
	}
	if err := manifest.RequireTarget([]string{"aarch64-apple-darwin"}); !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("RequireTarget() error = %v, want ErrTargetMismatch", err)
	}
}

func TestManifestRequireFeatures(t *testing.T) {
	manifest := Manifest{Features: []string{"initialize", "session/list", " turn/start "}}
	if err := manifest.RequireFeatures([]string{"initialize", "session/list", "turn/start"}); err != nil {
		t.Fatalf("RequireFeatures() error = %v", err)
	}
	err := manifest.RequireFeatures([]string{"initialize", "", "tool/execute"})
	if !errors.Is(err, ErrFeatureMissing) {
		t.Fatalf("RequireFeatures() error = %v, want ErrFeatureMissing", err)
	}
	if missing := manifest.MissingFeatures([]string{"tool/execute"}); len(missing) != 1 || missing[0] != "tool/execute" {
		t.Fatalf("MissingFeatures()=%+v, want tool/execute", missing)
	}
}

func TestTargetTriple(t *testing.T) {
	tests := map[string]string{
		"windows/amd64": "x86_64-pc-windows-msvc",
		"windows/arm64": "aarch64-pc-windows-msvc",
		"darwin/amd64":  "x86_64-apple-darwin",
		"darwin/arm64":  "aarch64-apple-darwin",
		"linux/amd64":   "x86_64-unknown-linux-musl",
		"linux/arm64":   "aarch64-unknown-linux-musl",
	}
	for input, want := range tests {
		goos, goarch, _ := strings.Cut(input, "/")
		if got := TargetTriple(goos, goarch); got != want {
			t.Fatalf("TargetTriple(%q,%q)=%q, want %q", goos, goarch, got, want)
		}
	}
}

func TestTargetTriplesIncludesWindowsGNUFallback(t *testing.T) {
	got := TargetTriples("windows", "amd64")
	want := []string{"x86_64-pc-windows-msvc", "x86_64-pc-windows-gnu"}
	if len(got) != len(want) {
		t.Fatalf("TargetTriples()=%+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("TargetTriples()[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

// TestTargetTriplesLinuxMuslPrimaryWithGNUFallback 验证 Linux 主选 musl（对齐
// codex/eos-core-rs CI），回退 gnu（兼容历史 gnu 二进制）。
func TestTargetTriplesLinuxMuslPrimaryWithGNUFallback(t *testing.T) {
	tests := []struct {
		goos, goarch string
		want         []string
	}{
		{"linux", "amd64", []string{"x86_64-unknown-linux-musl", "x86_64-unknown-linux-gnu"}},
		{"linux", "arm64", []string{"aarch64-unknown-linux-musl", "aarch64-unknown-linux-gnu"}},
	}
	for _, tc := range tests {
		got := TargetTriples(tc.goos, tc.goarch)
		if len(got) != len(tc.want) {
			t.Fatalf("TargetTriples(%q,%q)=%+v, want %+v", tc.goos, tc.goarch, got, tc.want)
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("TargetTriples(%q,%q)[%d]=%q, want %q", tc.goos, tc.goarch, i, got[i], tc.want[i])
			}
		}
	}
}
