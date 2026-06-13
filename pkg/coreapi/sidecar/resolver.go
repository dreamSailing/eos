package sidecar

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	EnvCorePath             = "EOS_CORE_PATH"
	EnvCoreManifest         = "EOS_CORE_MANIFEST"
	EnvCoreBinDir           = "EOS_CORE_BIN_DIR"
	EnvSignaturePublicKey   = "EOS_SIGNATURE_PUBLIC_KEY"
	EnvReleaseArtifactCheck = "EOS_RELEASE_ARTIFACT_CHECK"
)

var ErrCoreBinaryNotFound = errors.New("core sidecar binary not found")

type ResolveOptions struct {
	BinaryPath          string
	ManifestPath        string
	RootDir             string
	GOOS                string
	GOARCH              string
	VerifyChecksum      bool
	RequireSignature    bool
	RequiredFeatures    []string
	PublicKeyPath       string
	AllowDevPlaceholder bool
}

type ResolvedBinary struct {
	Path         string
	ManifestPath string
	Manifest     *Manifest
	Target       string
	Source       string
}

func ResolveBinary(opts ResolveOptions) (ResolvedBinary, error) {
	goos := firstNonEmpty(opts.GOOS, runtime.GOOS)
	goarch := firstNonEmpty(opts.GOARCH, runtime.GOARCH)
	target := TargetTriple(goos, goarch)
	if path := firstNonEmpty(opts.BinaryPath, os.Getenv(EnvCorePath)); strings.TrimSpace(path) != "" {
		resolved, err := requireFile(path)
		if err != nil {
			return ResolvedBinary{}, err
		}
		return ResolvedBinary{Path: resolved, Target: target, Source: EnvCorePath}, nil
	}

	if path := firstNonEmpty(opts.ManifestPath, os.Getenv(EnvCoreManifest)); strings.TrimSpace(path) != "" {
		return resolveManifest(path, opts, EnvCoreManifest, TargetTriples(goos, goarch))
	}

	var roots []string
	if root := firstNonEmpty(opts.RootDir, os.Getenv(EnvCoreBinDir)); strings.TrimSpace(root) != "" {
		roots = append(roots, root)
	}
	roots = append(roots, defaultSearchRoots()...)
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		for _, candidate := range TargetTriples(goos, goarch) {
			path := filepath.Join(root, candidate, DefaultManifestName)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			return resolveManifest(path, opts, "search", []string{candidate})
		}
	}
	return ResolvedBinary{}, fmt.Errorf("%w: target %s", ErrCoreBinaryNotFound, target)
}

func resolveManifest(path string, opts ResolveOptions, source string, targets []string) (ResolvedBinary, error) {
	path = filepath.Clean(path)
	manifest, err := LoadManifest(path)
	if err != nil {
		return ResolvedBinary{}, err
	}
	if err := manifest.RequireTarget(targets); err != nil {
		return ResolvedBinary{}, err
	}
	publicKey, err := resolvePublicKey(opts.PublicKeyPath)
	if err != nil {
		return ResolvedBinary{}, err
	}
	if opts.RequireSignature {
		if err := manifest.CheckSignature(publicKey, opts.AllowDevPlaceholder); err != nil {
			return ResolvedBinary{}, err
		}
	} else if publicKey != nil && strings.TrimSpace(manifest.Signature) != "" && !IsPlaceholderSignature(manifest.Signature) {
		// Verify optional signatures when present; release gates use RequireSignature.
		if err := VerifySignature(manifest, publicKey); err != nil {
			return ResolvedBinary{}, err
		}
	}
	if err := manifest.RequireFeatures(opts.RequiredFeatures); err != nil {
		return ResolvedBinary{}, err
	}
	binaryPath, err := manifest.BinaryPath(path)
	if err != nil {
		return ResolvedBinary{}, err
	}
	binaryPath, err = requireFile(binaryPath)
	if err != nil {
		return ResolvedBinary{}, err
	}
	if opts.VerifyChecksum {
		if err := manifest.VerifyBinary(binaryPath); err != nil {
			return ResolvedBinary{}, err
		}
	}
	return ResolvedBinary{
		Path:         binaryPath,
		ManifestPath: path,
		Manifest:     &manifest,
		Target:       manifest.Target,
		Source:       source,
	}, nil
}

func resolvePublicKey(path string) (ed25519.PublicKey, error) {
	path = firstNonEmpty(path, os.Getenv(EnvSignaturePublicKey))
	if strings.TrimSpace(path) != "" {
		return LoadPublicKeyFromFile(path)
	}
	return DefaultPublicKey(), nil
}

func requireFile(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return "", fmt.Errorf("%w: path is required", ErrCoreBinaryNotFound)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%w: %s is a directory", ErrCoreBinaryNotFound, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func defaultSearchRoots() []string {
	var roots []string
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		exeDir := filepath.Dir(exe)
		roots = append(roots,
			filepath.Join(exeDir, "core"),
			filepath.Join(exeDir, "eos-core"),
		)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		roots = append(roots, filepath.Join(filepath.Dir(file), "binaries"))
	}
	return roots
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
