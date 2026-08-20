package sidecar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ManifestSchemaVersion = "1.0"
	DefaultAPIVersion     = "v1"
	DefaultManifestName   = "manifest.json"
)

var (
	ErrManifestInvalid  = errors.New("core sidecar manifest invalid")
	ErrChecksumMismatch = errors.New("core sidecar checksum mismatch")
	ErrSignatureMissing = errors.New("core sidecar signature missing")
	ErrFeatureMissing   = errors.New("core sidecar manifest missing required feature")
	ErrTargetMismatch   = errors.New("core sidecar manifest target mismatch")
)

type Manifest struct {
	SchemaVersion      string   `json:"schema_version"`
	CoreVersion        string   `json:"core_version"`
	APIVersion         string   `json:"api_version"`
	Target             string   `json:"target"`
	Binary             string   `json:"binary"`
	SHA256             string   `json:"sha256"`
	Signature          string   `json:"signature,omitempty"`
	SignatureAlgorithm string   `json:"signature_algorithm,omitempty"`
	MinCLIVersion      string   `json:"min_cli_version,omitempty"`
	Features           []string `json:"features,omitempty"`
}

func LoadManifest(path string) (Manifest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Manifest{}, fmt.Errorf("%w: manifest path is required", ErrManifestInvalid)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return LoadManifestBytes(data)
}

// LoadManifestBytes 从字节内容解析 manifest（内嵌分发通道使用：内核产物
// 以 go:embed 内嵌进 CLI，落盘前需先在内存中校验）。
func LoadManifestBytes(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.SchemaVersion) == "" {
		return fmt.Errorf("%w: schema_version is required", ErrManifestInvalid)
	}
	if m.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("%w: unsupported schema_version %q", ErrManifestInvalid, m.SchemaVersion)
	}
	if strings.TrimSpace(m.CoreVersion) == "" {
		return fmt.Errorf("%w: core_version is required", ErrManifestInvalid)
	}
	if strings.TrimSpace(m.APIVersion) == "" {
		return fmt.Errorf("%w: api_version is required", ErrManifestInvalid)
	}
	if strings.TrimSpace(m.Target) == "" {
		return fmt.Errorf("%w: target is required", ErrManifestInvalid)
	}
	if strings.TrimSpace(m.Binary) == "" {
		return fmt.Errorf("%w: binary is required", ErrManifestInvalid)
	}
	if strings.TrimSpace(m.SHA256) == "" {
		return fmt.Errorf("%w: sha256 is required", ErrManifestInvalid)
	}
	if _, err := parseSHA256(m.SHA256); err != nil {
		return fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	return nil
}

func (m Manifest) BinaryPath(manifestPath string) (string, error) {
	binary := strings.TrimSpace(m.Binary)
	if binary == "" {
		return "", fmt.Errorf("%w: binary is required", ErrManifestInvalid)
	}
	if filepath.IsAbs(binary) {
		return filepath.Clean(binary), nil
	}
	base := filepath.Dir(strings.TrimSpace(manifestPath))
	if base == "." || base == "" {
		base = "."
	}
	return filepath.Clean(filepath.Join(base, binary)), nil
}

func (m Manifest) RequireSignature() error {
	sig := strings.TrimSpace(m.Signature)
	if sig == "" {
		return ErrSignatureMissing
	}
	if IsPlaceholderSignature(sig) {
		return ErrSignaturePlaceholder
	}
	return nil
}

func (m Manifest) RequireTarget(allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}
	target := strings.TrimSpace(m.Target)
	for _, candidate := range allowed {
		if target != "" && target == strings.TrimSpace(candidate) {
			return nil
		}
	}
	return fmt.Errorf("%w: got %q want one of %s", ErrTargetMismatch, target, strings.Join(trimmedNonEmpty(allowed), ", "))
}

func (m Manifest) RequireFeatures(required []string) error {
	missing := m.MissingFeatures(required)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrFeatureMissing, strings.Join(missing, ", "))
}

func (m Manifest) MissingFeatures(required []string) []string {
	if len(required) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(m.Features))
	for _, feature := range m.Features {
		feature = strings.TrimSpace(feature)
		if feature != "" {
			seen[feature] = struct{}{}
		}
	}
	var missing []string
	for _, feature := range required {
		feature = strings.TrimSpace(feature)
		if feature == "" {
			continue
		}
		if _, ok := seen[feature]; !ok {
			missing = append(missing, feature)
		}
	}
	return missing
}

func (m Manifest) VerifyBinary(path string) error {
	want, err := parseSHA256(m.SHA256)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	got, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%w: got sha256:%s want sha256:%s", ErrChecksumMismatch, got, want)
	}
	return nil
}

// VerifyBinaryBytes 校验字节内容与 manifest 的 sha256 一致（内嵌分发通道：
// 释放到磁盘前先验内容，防止内嵌产物与 manifest 串包）。
func (m Manifest) VerifyBinaryBytes(bin []byte) error {
	want, err := parseSHA256(m.SHA256)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	got := sha256.Sum256(bin)
	if !strings.EqualFold(hex.EncodeToString(got[:]), want) {
		return fmt.Errorf("%w: embedded binary sha256 mismatch with manifest", ErrChecksumMismatch)
	}
	return nil
}

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func CurrentTarget() string {
	return TargetTriple(runtime.GOOS, runtime.GOARCH)
}

func TargetTriple(goos, goarch string) string {
	goos = strings.TrimSpace(strings.ToLower(goos))
	goarch = strings.TrimSpace(strings.ToLower(goarch))
	switch goos + "/" + goarch {
	case "windows/amd64":
		return "x86_64-pc-windows-msvc"
	case "windows/arm64":
		return "aarch64-pc-windows-msvc"
	case "darwin/amd64":
		return "x86_64-apple-darwin"
	case "darwin/arm64":
		return "aarch64-apple-darwin"
	case "linux/amd64":
		return "x86_64-unknown-linux-gnu"
	case "linux/arm64":
		return "aarch64-unknown-linux-gnu"
	default:
		if goos == "" || goarch == "" {
			return ""
		}
		return goarch + "-" + goos
	}
}

func TargetTriples(goos, goarch string) []string {
	primary := TargetTriple(goos, goarch)
	out := make([]string, 0, 3)
	if primary != "" {
		out = append(out, primary)
	}
	goos = strings.TrimSpace(strings.ToLower(goos))
	goarch = strings.TrimSpace(strings.ToLower(goarch))
	// Windows amd64：主选 msvc，回退 gnu（mingw 工具链产物）。
	if goos == "windows" && goarch == "amd64" && primary != "x86_64-pc-windows-gnu" {
		out = append(out, "x86_64-pc-windows-gnu")
	}
	// Linux：主选 gnu（对齐 release.yml 与 vendored 产物——xcap 截图链的
	// -sys 系统库探测在 musl 交叉编译下必败），回退 musl（兼容历史安装
	// 布局里的 musl 二进制：旧版发版/手动放置）。
	if goos == "linux" {
		switch goarch {
		case "amd64":
			if primary != "x86_64-unknown-linux-musl" {
				out = append(out, "x86_64-unknown-linux-musl")
			}
		case "arm64":
			if primary != "aarch64-unknown-linux-musl" {
				out = append(out, "aarch64-unknown-linux-musl")
			}
		}
	}
	return out
}

func parseSHA256(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != sha256.Size*2 {
		return "", fmt.Errorf("sha256 must be %d hex chars", sha256.Size*2)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("sha256 must be hex: %w", err)
	}
	return value, nil
}

func trimmedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
