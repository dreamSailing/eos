package sidecar

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func generateTestKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	return pub, priv
}

func exportPublicKeyPEM(t *testing.T, pub ed25519.PublicKey) []byte {
	t.Helper()
	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
}

func exportPrivateKeyPEM(t *testing.T, priv ed25519.PrivateKey) []byte {
	t.Helper()
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
}

func newSignedTestManifest(t *testing.T, priv ed25519.PrivateKey) Manifest {
	t.Helper()
	m := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CoreVersion:   "0.1.0",
		APIVersion:    DefaultAPIVersion,
		Target:        "x86_64-unknown-linux-gnu",
		Binary:        "eos-core",
		SHA256:        "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		MinCLIVersion: "v0.3.0",
		Features:      []string{"initialize", "shutdown", "session/list"},
	}
	SignManifest(&m, priv)
	return m
}

func TestBuildSignedPayloadDeterministic(t *testing.T) {
	m := Manifest{
		CoreVersion:   "0.1.0",
		APIVersion:    "v1",
		Target:        "x86_64-unknown-linux-gnu",
		Binary:        "eos-core",
		SHA256:        "sha256:abcdef",
		Features:      []string{"shutdown", "initialize", "session/list"},
		MinCLIVersion: "v0.3.0",
	}
	p1 := BuildSignedPayload(m)
	p2 := BuildSignedPayload(m)
	if p1 != p2 {
		t.Fatalf("BuildSignedPayload not deterministic: %q != %q", p1, p2)
	}
	want := "0.1.0|v1|x86_64-unknown-linux-gnu|eos-core|sha256:abcdef|initialize,session/list,shutdown|v0.3.0"
	if p1 != want {
		t.Fatalf("BuildSignedPayload() = %q, want %q", p1, want)
	}
}

func TestBuildSignedPayloadEmptyFeatures(t *testing.T) {
	m := Manifest{
		CoreVersion: "0.1.0",
		APIVersion:  "v1",
		Target:      "x86_64-test",
		Binary:      "eos-core",
		SHA256:      "sha256:0000",
	}
	payload := BuildSignedPayload(m)
	want := "0.1.0|v1|x86_64-test|eos-core|sha256:0000||"
	if payload != want {
		t.Fatalf("BuildSignedPayload() = %q, want %q", payload, want)
	}
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	pub, priv := generateTestKeypair(t)
	m := newSignedTestManifest(t, priv)

	if err := VerifySignature(m, pub); err != nil {
		t.Fatalf("VerifySignature() error = %v", err)
	}
}

func TestVerifySignatureModifiedBinaryHash(t *testing.T) {
	pub, priv := generateTestKeypair(t)
	m := newSignedTestManifest(t, priv)

	m.SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	err := VerifySignature(m, pub)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("VerifySignature() error = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifySignatureModifiedFeatures(t *testing.T) {
	pub, priv := generateTestKeypair(t)
	m := newSignedTestManifest(t, priv)

	m.Features = []string{"initialize", "shutdown", "tool/execute"}
	err := VerifySignature(m, pub)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("VerifySignature() error = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifySignatureMissingSignature(t *testing.T) {
	pub, _ := generateTestKeypair(t)
	m := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CoreVersion:   "0.1.0",
		APIVersion:    DefaultAPIVersion,
		Target:        "x86_64-test",
		Binary:        "eos-core",
		SHA256:        "sha256:abcdef",
	}

	err := VerifySignature(m, pub)
	if !errors.Is(err, ErrSignatureMissing) {
		t.Fatalf("VerifySignature() error = %v, want ErrSignatureMissing", err)
	}
}

func TestVerifySignaturePlaceholderRejected(t *testing.T) {
	pub, _ := generateTestKeypair(t)
	m := Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CoreVersion:   "0.1.0",
		APIVersion:    DefaultAPIVersion,
		Target:        "x86_64-test",
		Binary:        "eos-core",
		SHA256:        "sha256:abcdef",
		Signature:     SignaturePlaceholder,
	}

	err := VerifySignature(m, pub)
	if !errors.Is(err, ErrSignaturePlaceholder) {
		t.Fatalf("VerifySignature() error = %v, want ErrSignaturePlaceholder", err)
	}
}

func TestVerifySignatureWrongPublicKey(t *testing.T) {
	_, priv := generateTestKeypair(t)
	m := newSignedTestManifest(t, priv)

	wrongPub, _ := generateTestKeypair(t)
	err := VerifySignature(m, wrongPub)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("VerifySignature() error = %v, want ErrSignatureInvalid", err)
	}
}

func TestCheckSignatureWithPublicKey(t *testing.T) {
	pub, priv := generateTestKeypair(t)
	m := newSignedTestManifest(t, priv)

	if err := m.CheckSignature(pub, false); err != nil {
		t.Fatalf("CheckSignature() error = %v", err)
	}
}

func TestCheckSignatureWithoutPublicKey(t *testing.T) {
	m := Manifest{Signature: "some-non-placeholder-sig"}
	if err := m.CheckSignature(nil, false); err != nil {
		t.Fatalf("CheckSignature(nil) error = %v", err)
	}
}

func TestCheckSignaturePlaceholderNotAllowed(t *testing.T) {
	m := Manifest{Signature: SignaturePlaceholder}
	err := m.CheckSignature(nil, false)
	if !errors.Is(err, ErrSignaturePlaceholder) {
		t.Fatalf("CheckSignature() error = %v, want ErrSignaturePlaceholder", err)
	}
}

func TestCheckSignaturePlaceholderAllowed(t *testing.T) {
	m := Manifest{Signature: SignaturePlaceholder}
	if err := m.CheckSignature(nil, true); err != nil {
		t.Fatalf("CheckSignature(allowDevPlaceholder=true) error = %v", err)
	}
}

func TestCheckSignatureEmptySignature(t *testing.T) {
	m := Manifest{}
	err := m.CheckSignature(nil, false)
	if !errors.Is(err, ErrSignatureMissing) {
		t.Fatalf("CheckSignature() error = %v, want ErrSignatureMissing", err)
	}
}

func TestLoadPublicKeyFromPEMRoundTrip(t *testing.T) {
	pub, _ := generateTestKeypair(t)
	pubPEM := exportPublicKeyPEM(t, pub)

	loaded, err := LoadPublicKeyFromPEM(pubPEM)
	if err != nil {
		t.Fatalf("LoadPublicKeyFromPEM() error = %v", err)
	}
	if !loaded.Equal(pub) {
		t.Fatalf("loaded key does not match original")
	}
}

func TestLoadPrivateKeyFromPEMRoundTrip(t *testing.T) {
	_, priv := generateTestKeypair(t)
	privPEM := exportPrivateKeyPEM(t, priv)

	loaded, err := LoadPrivateKeyFromPEM(privPEM)
	if err != nil {
		t.Fatalf("LoadPrivateKeyFromPEM() error = %v", err)
	}
	if !loaded.Equal(priv) {
		t.Fatalf("loaded key does not match original")
	}
}

func TestLoadPublicKeyFromPEMInvalidData(t *testing.T) {
	_, err := LoadPublicKeyFromPEM([]byte("not a PEM file"))
	if err == nil {
		t.Fatal("LoadPublicKeyFromPEM() expected error for invalid data")
	}
}

func TestLoadPublicKeyFromFile(t *testing.T) {
	pub, _ := generateTestKeypair(t)
	pubPEM := exportPublicKeyPEM(t, pub)

	dir := t.TempDir()
	path := filepath.Join(dir, "test-key.pem")
	if err := os.WriteFile(path, pubPEM, 0o644); err != nil {
		t.Fatalf("write key: %v", err)
	}

	loaded, err := LoadPublicKeyFromFile(path)
	if err != nil {
		t.Fatalf("LoadPublicKeyFromFile() error = %v", err)
	}
	if !loaded.Equal(pub) {
		t.Fatalf("loaded key does not match original")
	}
}

func TestIsPlaceholderSignature(t *testing.T) {
	if !IsPlaceholderSignature("unsigned-development-placeholder") {
		t.Fatal("IsPlaceholderSignature should return true for placeholder")
	}
	if IsPlaceholderSignature("ed25519:abc123") {
		t.Fatal("IsPlaceholderSignature should return false for real signature")
	}
	if IsPlaceholderSignature("") {
		t.Fatal("IsPlaceholderSignature should return false for empty")
	}
}

func TestVerifySignatureUnsupportedAlgorithm(t *testing.T) {
	pub, priv := generateTestKeypair(t)
	m := newSignedTestManifest(t, priv)
	m.SignatureAlgorithm = "rsa-pss"

	err := VerifySignature(m, pub)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("VerifySignature() error = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifySignatureBadBase64(t *testing.T) {
	pub, _ := generateTestKeypair(t)
	m := Manifest{
		Signature: "ed25519:!!!not-base64!!!",
	}
	err := VerifySignature(m, pub)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("VerifySignature() error = %v, want ErrSignatureInvalid", err)
	}
}

func TestVerifySignatureWrongLength(t *testing.T) {
	pub, _ := generateTestKeypair(t)
	m := Manifest{
		Signature: "ed25519:AAECAwQF",
	}
	err := VerifySignature(m, pub)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("VerifySignature() error = %v, want ErrSignatureInvalid", err)
	}
}

func TestEnvReleaseArtifactCheckConstant(t *testing.T) {
	if EnvReleaseArtifactCheck != "EOS_RELEASE_ARTIFACT_CHECK" {
		t.Fatalf("EnvReleaseArtifactCheck=%q, want %q", EnvReleaseArtifactCheck, "EOS_RELEASE_ARTIFACT_CHECK")
	}
}

func TestCheckSignatureReleaseGateRejectsPlaceholder(t *testing.T) {
	pub, priv := generateTestKeypair(t)

	devManifest := Manifest{Signature: SignaturePlaceholder}
	if err := devManifest.CheckSignature(nil, true); err != nil {
		t.Fatalf("dev mode (allowDevPlaceholder=true) should accept placeholder: %v", err)
	}

	releaseManifest := Manifest{Signature: SignaturePlaceholder}
	if err := releaseManifest.CheckSignature(nil, false); !errors.Is(err, ErrSignaturePlaceholder) {
		t.Fatalf("release mode (allowDevPlaceholder=false) should reject placeholder: %v", err)
	}

	signed := newSignedTestManifest(t, priv)
	if err := signed.CheckSignature(pub, false); err != nil {
		t.Fatalf("signed manifest should pass in release mode: %v", err)
	}
}
