package sidecar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBinaryFromEnvCorePath(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "eos-core")
	if err := os.WriteFile(binaryPath, []byte("fake"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	t.Setenv(EnvCorePath, binaryPath)

	resolved, err := ResolveBinary(ResolveOptions{GOOS: "linux", GOARCH: "amd64"})
	if err != nil {
		t.Fatalf("ResolveBinary() error = %v", err)
	}
	if resolved.Path != filepath.Clean(binaryPath) {
		t.Fatalf("Path=%q, want %q", resolved.Path, binaryPath)
	}
	if resolved.Source != EnvCorePath {
		t.Fatalf("Source=%q, want %q", resolved.Source, EnvCorePath)
	}
	if resolved.Target != "x86_64-unknown-linux-gnu" {
		t.Fatalf("Target=%q", resolved.Target)
	}
}

func TestResolveBinaryFromManifestRoot(t *testing.T) {
	t.Setenv(EnvCorePath, "")

	root := t.TempDir()
	target := "x86_64-unknown-linux-gnu"
	dir := filepath.Join(root, target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
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
		Target:        target,
		Binary:        "eos-core",
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		Signature:     "unsigned-development-placeholder",
		Features:      []string{"initialize", "session/list"},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, DefaultManifestName), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	resolved, err := ResolveBinary(ResolveOptions{
		RootDir:            root,
		GOOS:               "linux",
		GOARCH:             "amd64",
		VerifyChecksum:     true,
		RequireSignature:   true,
		AllowDevPlaceholder: true,
		RequiredFeatures:   []string{"initialize", "session/list"},
	})
	if err != nil {
		t.Fatalf("ResolveBinary() error = %v", err)
	}
	if resolved.Path != filepath.Clean(binaryPath) {
		t.Fatalf("Path=%q, want %q", resolved.Path, binaryPath)
	}
	if resolved.Manifest == nil || resolved.Manifest.CoreVersion != "0.1.0" {
		t.Fatalf("Manifest=%+v, want loaded manifest", resolved.Manifest)
	}
}

func TestResolveBinaryRejectsManifestMissingRequiredFeature(t *testing.T) {
	t.Setenv(EnvCorePath, "")

	root := t.TempDir()
	target := "x86_64-unknown-linux-gnu"
	dir := filepath.Join(root, target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
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
		Target:        target,
		Binary:        "eos-core",
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		Signature:     "unsigned-development-placeholder",
		Features:      []string{"initialize"},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, DefaultManifestName), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err = ResolveBinary(ResolveOptions{
		RootDir:          root,
		GOOS:             "linux",
		GOARCH:           "amd64",
		RequiredFeatures: []string{"initialize", "turn/start"},
	})
	if !errors.Is(err, ErrFeatureMissing) {
		t.Fatalf("ResolveBinary() error = %v, want ErrFeatureMissing", err)
	}
}

func TestResolveBinaryRejectsExplicitManifestTargetMismatch(t *testing.T) {
	t.Setenv(EnvCorePath, "")

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
		Target:        "aarch64-apple-darwin",
		Binary:        "eos-core",
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestPath := filepath.Join(dir, DefaultManifestName)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err = ResolveBinary(ResolveOptions{
		ManifestPath: manifestPath,
		GOOS:         "linux",
		GOARCH:       "amd64",
	})
	if !errors.Is(err, ErrTargetMismatch) {
		t.Fatalf("ResolveBinary() error = %v, want ErrTargetMismatch", err)
	}
}

func TestResolveBinaryUsesWindowsGNUFallback(t *testing.T) {
	t.Setenv(EnvCorePath, "")

	root := t.TempDir()
	target := "x86_64-pc-windows-gnu"
	dir := filepath.Join(root, target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	binaryPath := filepath.Join(dir, "eos-core.exe")
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
		Binary:        "eos-core.exe",
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, DefaultManifestName), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	resolved, err := ResolveBinary(ResolveOptions{
		RootDir:        root,
		GOOS:           "windows",
		GOARCH:         "amd64",
		VerifyChecksum: true,
	})
	if err != nil {
		t.Fatalf("ResolveBinary() error = %v", err)
	}
	if resolved.Target != target {
		t.Fatalf("Target=%q, want %q", resolved.Target, target)
	}
	if resolved.Path != filepath.Clean(binaryPath) {
		t.Fatalf("Path=%q, want %q", resolved.Path, binaryPath)
	}
}

func TestResolveBinaryDevPlaceholderAllowed(t *testing.T) {
	t.Setenv(EnvCorePath, "")

	root := t.TempDir()
	target := "x86_64-unknown-linux-gnu"
	dir := filepath.Join(root, target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
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
		Target:        target,
		Binary:        "eos-core",
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		Signature:     SignaturePlaceholder,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, DefaultManifestName), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	resolved, err := ResolveBinary(ResolveOptions{
		RootDir:             root,
		GOOS:                "linux",
		GOARCH:              "amd64",
		RequireSignature:    true,
		AllowDevPlaceholder: true,
	})
	if err != nil {
		t.Fatalf("ResolveBinary(AllowDevPlaceholder=true) error = %v", err)
	}
	if resolved.Path != filepath.Clean(binaryPath) {
		t.Fatalf("Path=%q, want %q", resolved.Path, binaryPath)
	}
}

func TestResolveBinaryDevPlaceholderRejectedInReleaseMode(t *testing.T) {
	t.Setenv(EnvCorePath, "")

	root := t.TempDir()
	target := "x86_64-unknown-linux-gnu"
	dir := filepath.Join(root, target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
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
		Target:        target,
		Binary:        "eos-core",
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		Signature:     SignaturePlaceholder,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, DefaultManifestName), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err = ResolveBinary(ResolveOptions{
		RootDir:             root,
		GOOS:                "linux",
		GOARCH:              "amd64",
		RequireSignature:    true,
		AllowDevPlaceholder: false,
	})
	if !errors.Is(err, ErrSignaturePlaceholder) {
		t.Fatalf("ResolveBinary(AllowDevPlaceholder=false) error = %v, want ErrSignaturePlaceholder", err)
	}
}
