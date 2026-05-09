package filedialog

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestNormalizeDirectoryTrimsWhitespace(t *testing.T) {
	got, err := normalizeDirectory("  ./testdata  \n")
	if err != nil {
		t.Fatalf("normalizeDirectory error: %v", err)
	}
	if filepath.Base(got) != "testdata" {
		t.Fatalf("normalizeDirectory=%q, want path ending in testdata", got)
	}
}

func TestNormalizeDirectoryEmptyCancels(t *testing.T) {
	_, err := normalizeDirectory(" \n ")
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("expected ErrCanceled, got %v", err)
	}
}

func TestErrorHelpers(t *testing.T) {
	if !IsUnavailable(ErrUnavailable) {
		t.Fatalf("expected IsUnavailable to recognize ErrUnavailable")
	}
	if !IsCanceled(ErrCanceled) {
		t.Fatalf("expected IsCanceled to recognize ErrCanceled")
	}
}
