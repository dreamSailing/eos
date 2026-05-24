package coreapi

import (
	"testing"
)

func TestProjectDiagnosticsFromStrings_Nil(t *testing.T) {
	summary := ProjectDiagnosticsFromStrings(nil)
	if summary.Files != 0 || summary.Errors != 0 || summary.Warnings != 0 || summary.Infos != 0 {
		t.Fatalf("expected zeroed summary, got %+v", summary)
	}
	if len(summary.Items) != 0 {
		t.Fatalf("expected no items, got %d", len(summary.Items))
	}
}

func TestProjectDiagnosticsFromStrings_Empty(t *testing.T) {
	summary := ProjectDiagnosticsFromStrings([]string{})
	if summary.Files != 0 {
		t.Fatalf("expected 0 files, got %d", summary.Files)
	}
}

func TestProjectDiagnosticsFromStrings_Typical(t *testing.T) {
	lines := []string{
		"### main.go",
		"Path: main.go",
		"[Error] Line 3: undefined: foo",
		"[Warning] Line 5: unused var x",
		"### util.go",
		"Path: util.go",
		"[Info] Line 10: consider using strings.Builder",
		"**Summary**: 2 files (1 errors, 1 warnings, 1 infos)",
	}
	summary := ProjectDiagnosticsFromStrings(lines)
	if summary.Files != 2 {
		t.Fatalf("expected 2 files, got %d", summary.Files)
	}
	if summary.Errors != 1 {
		t.Fatalf("expected 1 error, got %d", summary.Errors)
	}
	if summary.Warnings != 1 {
		t.Fatalf("expected 1 warning, got %d", summary.Warnings)
	}
	if summary.Infos != 1 {
		t.Fatalf("expected 1 info, got %d", summary.Infos)
	}
	if len(summary.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(summary.Items))
	}
	if summary.Items[0].File != "main.go" || summary.Items[0].Line != 3 || summary.Items[0].Severity != "Error" {
		t.Fatalf("items[0]=%+v", summary.Items[0])
	}
	if summary.Items[1].File != "main.go" || summary.Items[1].Line != 5 || summary.Items[1].Severity != "Warning" {
		t.Fatalf("items[1]=%+v", summary.Items[1])
	}
	if summary.Items[2].File != "util.go" || summary.Items[2].Line != 10 || summary.Items[2].Severity != "Info" {
		t.Fatalf("items[2]=%+v", summary.Items[2])
	}
}

func TestProjectDiagnosticsFromStrings_WithoutSummaryLine(t *testing.T) {
	lines := []string{
		"### app.py",
		"Path: app.py",
		"[Error] Line 1: syntax error",
	}
	summary := ProjectDiagnosticsFromStrings(lines)
	if summary.Files != 1 {
		t.Fatalf("expected 1 file (inferred), got %d", summary.Files)
	}
	if summary.Errors != 0 {
		t.Fatalf("expected 0 errors (no summary line), got %d", summary.Errors)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(summary.Items))
	}
	if summary.Items[0].Severity != "Error" {
		t.Fatalf("expected Error severity, got %s", summary.Items[0].Severity)
	}
}

func TestProjectDiagnosticsFromStrings_SkipNoise(t *testing.T) {
	lines := []string{
		"### main.go",
		"Path: main.go",
		"[Warning] Line 1: warn",
		"... and 5 more",
		"",
		"**Summary**: 1 files (0 errors, 1 warnings, 0 infos)",
	}
	summary := ProjectDiagnosticsFromStrings(lines)
	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 item (noise skipped), got %d", len(summary.Items))
	}
	if summary.Files != 1 || summary.Warnings != 1 {
		t.Fatalf("expected 1 file 1 warning, got %+v", summary)
	}
}
