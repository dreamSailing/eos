package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectVerifierProjectTypes(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "frontend", "src"))
	mustMkdirAll(t, filepath.Join(root, "internal", "cli"))
	mustMkdirAll(t, filepath.Join(root, "internal", "gateway"))
	mustWriteFile(t, filepath.Join(root, "frontend", "package.json"), `{}`)
	mustWriteFile(t, filepath.Join(root, "main.go"), "package main\n")

	got, err := detectVerifierProjectTypes(root)
	if err != nil {
		t.Fatalf("detectVerifierProjectTypes() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(detectVerifierProjectTypes()) = %d, want 3", len(got))
	}
	if got[0].Type != verifierProjectWeb {
		t.Fatalf("got[0].Type = %q, want %q", got[0].Type, verifierProjectWeb)
	}
	if got[1].Type != verifierProjectCLI {
		t.Fatalf("got[1].Type = %q, want %q", got[1].Type, verifierProjectCLI)
	}
	if got[2].Type != verifierProjectAPI {
		t.Fatalf("got[2].Type = %q, want %q", got[2].Type, verifierProjectAPI)
	}
}

func TestVerifierToolSuggestionsDeduplicatesByTool(t *testing.T) {
	got := verifierToolSuggestions([]verifierProjectDetection{
		{Type: verifierProjectWeb},
		{Type: verifierProjectCLI},
		{Type: verifierProjectWeb},
		{Type: verifierProjectAPI},
	})
	if len(got) != 3 {
		t.Fatalf("len(verifierToolSuggestions()) = %d, want 3", len(got))
	}
	if got[0].Tool != "Playwright" {
		t.Fatalf("got[0].Tool = %q, want Playwright", got[0].Tool)
	}
	if got[1].Tool != "Tmux" {
		t.Fatalf("got[1].Tool = %q, want Tmux", got[1].Tool)
	}
	if got[2].Tool != "HTTP" {
		t.Fatalf("got[2].Tool = %q, want HTTP", got[2].Tool)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
