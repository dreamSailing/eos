package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatch_EditsFormat_AppliesSuccessfully(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "main.go")
	if err := os.WriteFile(fp, []byte("package main\n\nfunc oldFunc() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "apply",
		"format": "edits",
		"patches": []interface{}{
			map[string]interface{}{
				"path": fp,
				"edits": []interface{}{
					map[string]interface{}{"find": "oldFunc", "replace": "newFunc"},
				},
			},
		},
	})
	if r.Status != "success" {
		t.Fatalf("patch status=%s, error=%s", r.Status, r.Error)
	}
	content, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "newFunc") {
		t.Fatalf("file content %q does not contain newFunc", string(content))
	}
	if strings.Contains(string(content), "oldFunc") {
		t.Fatalf("file content %q still contains oldFunc", string(content))
	}
}

func TestPatch_EditsFormat_OutsideWorkspace_Fails(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "..", "outside_patch_test_dir")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outside)
	fp := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(fp, []byte("secret data"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "apply",
		"format": "edits",
		"patches": []interface{}{
			map[string]interface{}{
				"path": fp,
				"edits": []interface{}{
					map[string]interface{}{"find": "secret", "replace": "leaked"},
				},
			},
		},
	})
	if r.Status != "success" {
		t.Fatalf("expected success with errors in results, got status=%s", r.Status)
	}
	results, _ := r.Data["results"].([]map[string]interface{})
	found := false
	for _, res := range results {
		if errStr, ok := res["error"].(string); ok && strings.Contains(errStr, "outside") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'outside' error in results, got %v", results)
	}
	content, _ := os.ReadFile(fp)
	if string(content) != "secret data" {
		t.Fatalf("file was modified: %q", string(content))
	}
}

func TestPatch_EditsFormat_ReadOnly_Fails(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(fp, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "read-only")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "apply",
		"format": "edits",
		"patches": []interface{}{
			map[string]interface{}{
				"path": fp,
				"edits": []interface{}{
					map[string]interface{}{"find": "hello", "replace": "bye"},
				},
			},
		},
	})
	if r.Status != "success" {
		t.Fatalf("expected success with errors in results, got status=%s", r.Status)
	}
	results, _ := r.Data["results"].([]map[string]interface{})
	found := false
	for _, res := range results {
		if errStr, ok := res["error"].(string); ok && strings.Contains(errStr, "read-only") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'read-only' error in results, got %v", results)
	}
	content, _ := os.ReadFile(fp)
	if string(content) != "hello world" {
		t.Fatalf("file was modified: %q", string(content))
	}
}

func TestPatch_EditsFormat_DryRun_DoesNotWriteDisk(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "file.txt")
	original := []byte("original content\nline2\n")
	if err := os.WriteFile(fp, original, 0644); err != nil {
		t.Fatal(err)
	}

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "dry_run",
		"format": "edits",
		"patches": []interface{}{
			map[string]interface{}{
				"path": fp,
				"edits": []interface{}{
					map[string]interface{}{"find": "original", "replace": "modified"},
				},
			},
		},
	})
	if r.Status != "success" {
		t.Fatalf("dry_run status=%s, error=%s", r.Status, r.Error)
	}

	dryRun, _ := r.Data["dry_run"].(bool)
	if !dryRun {
		t.Fatal("expected dry_run=true in result data")
	}

	content, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(original) {
		t.Fatalf("dry_run modified file: got %q, want %q", string(content), string(original))
	}
}

func TestPatch_UnifiedFormat_AppliesSuccessfully(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc old() {}\n\nfunc main() {}\n"
	if err := os.WriteFile(fp, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	diff := "--- a/main.go\n+++ b/main.go\n@@ -1,5 +1,5 @@\n package main\n \n-func old() {}\n+func new() {}\n \n func main() {}\n"

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "apply",
		"format": "unified",
		"diff":   diff,
	})
	if r.Status != "success" {
		t.Fatalf("patch unified status=%s, error=%s", r.Status, r.Error)
	}
	content, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "func new()") {
		t.Fatalf("file content %q does not contain 'func new()'", string(content))
	}
	if strings.Contains(string(content), "func old()") {
		t.Fatalf("file content %q still contains 'func old()'", string(content))
	}
}

func TestPatch_UnifiedFormat_DryRun_DoesNotWriteDisk(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc old() {}\n"
	if err := os.WriteFile(fp, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	diff := "--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,3 @@\n package main\n \n-func old() {}\n+func new() {}\n"

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "dry_run",
		"format": "unified",
		"diff":   diff,
	})
	if r.Status != "success" {
		t.Fatalf("dry_run status=%s, error=%s", r.Status, r.Error)
	}

	content, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Fatalf("dry_run modified file: got %q, want %q", string(content), original)
	}
}

func TestPatch_UnifiedFormat_OutsideWorkspace_Fails(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "..", "outside_unified_test_dir")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outside)
	fp := filepath.Join(outside, "main.go")
	if err := os.WriteFile(fp, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	outsideSlash := filepath.ToSlash(outside)
	diff := "--- a/" + outsideSlash + "/main.go\n+++ b/" + outsideSlash + "/main.go\n@@ -1,1 +1,1 @@\n-package main\n+package other\n"

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "apply",
		"format": "unified",
		"diff":   diff,
	})
	if r.Status != "success" {
		t.Fatalf("expected success with errors in results, got status=%s", r.Status)
	}
	results, _ := r.Data["results"].([]map[string]interface{})
	found := false
	for _, res := range results {
		if errStr, ok := res["error"].(string); ok && (strings.Contains(errStr, "outside") || strings.Contains(errStr, "not found")) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'outside' or 'not found' error in results, got %v", results)
	}
	content, _ := os.ReadFile(fp)
	if string(content) != "package main\n" {
		t.Fatalf("file was modified: %q", string(content))
	}
}

func TestPatch_ClassifyToolDanger_Apply(t *testing.T) {
	call := ToolCall{Tool: ToolPatch, Parameters: map[string]interface{}{"mode": "apply"}}
	cat, lvl, _, dangerous := ClassifyToolDanger(call)
	if !dangerous {
		t.Fatal("expected apply mode to be dangerous")
	}
	if cat != "overwrite_file" {
		t.Fatalf("category=%q, want overwrite_file", cat)
	}
	if lvl != "medium" {
		t.Fatalf("level=%q, want medium", lvl)
	}
}

func TestPatch_ClassifyToolDanger_DryRun(t *testing.T) {
	call := ToolCall{Tool: ToolPatch, Parameters: map[string]interface{}{"mode": "dry_run"}}
	cat, lvl, _, dangerous := ClassifyToolDanger(call)
	if dangerous {
		t.Fatal("expected dry_run mode to not be dangerous")
	}
	if lvl != "low" {
		t.Fatalf("level=%q, want low", lvl)
	}
	_ = cat
}

func TestPatch_ReadOnlyToolCall_ApplyIsNotReadOnly(t *testing.T) {
	call := ToolCall{Tool: ToolPatch, Parameters: map[string]interface{}{"mode": "apply"}}
	if isReadOnlyToolCall(call) {
		t.Fatal("expected apply mode to not be read-only")
	}
}

func TestPatch_ReadOnlyToolCall_DryRunIsReadOnly(t *testing.T) {
	call := ToolCall{Tool: ToolPatch, Parameters: map[string]interface{}{"mode": "dry_run"}}
	if !isReadOnlyToolCall(call) {
		t.Fatal("expected dry_run mode to be read-only")
	}
}

func TestPatch_AccessModeAllowsToolCall_ReadOnly_BlocksApply(t *testing.T) {
	ctx := WithAccessMode(context.Background(), "read-only")
	call := ToolCall{Tool: ToolPatch, Parameters: map[string]interface{}{"mode": "apply"}}
	ok, reason := AccessModeAllowsToolCall(ctx, call)
	if ok {
		t.Fatal("expected read-only to block apply")
	}
	if !strings.Contains(reason, "read-only") {
		t.Fatalf("reason %q does not mention read-only", reason)
	}
}

func TestPatch_AccessModeAllowsToolCall_ReadOnly_AllowsDryRun(t *testing.T) {
	ctx := WithAccessMode(context.Background(), "read-only")
	call := ToolCall{Tool: ToolPatch, Parameters: map[string]interface{}{"mode": "dry_run"}}
	ok, _ := AccessModeAllowsToolCall(ctx, call)
	if !ok {
		t.Fatal("expected read-only to allow dry_run")
	}
}

func TestPatch_AccessModeAllowsToolCall_WorkspaceWrite_BlocksOutsidePath(t *testing.T) {
	dir, err := os.MkdirTemp(".", "patchws_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	outside, err := os.MkdirTemp(".", "patchout_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outside)

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	call := ToolCall{Tool: ToolPatch, Parameters: map[string]interface{}{
		"mode": "apply",
		"patches": []interface{}{
			map[string]interface{}{"path": filepath.Join(outside, "f.txt"), "edits": []interface{}{}},
		},
	}}
	ok, reason := AccessModeAllowsToolCall(ctx, call)
	if ok {
		t.Fatal("expected workspace-write to block outside path")
	}
	if !strings.Contains(reason, "outside") {
		t.Fatalf("reason %q does not mention 'outside'", reason)
	}
}

func TestPatch_EditsFormat_LineRangeReplace(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(fp, []byte("line1\nline2\nline3\nline4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "apply",
		"format": "edits",
		"patches": []interface{}{
			map[string]interface{}{
				"path": fp,
				"edits": []interface{}{
					map[string]interface{}{"start_line": 2.0, "end_line": 3.0, "replace": "replaced"},
				},
			},
		},
	})
	if r.Status != "success" {
		t.Fatalf("patch status=%s, error=%s", r.Status, r.Error)
	}
	content, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "replaced") {
		t.Fatalf("file content %q does not contain 'replaced'", string(content))
	}
	if strings.Contains(string(content), "line2") {
		t.Fatalf("file content %q still contains 'line2'", string(content))
	}
}

func TestPatch_EditsFormat_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	fp1 := filepath.Join(dir, "a.txt")
	fp2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(fp1, []byte("alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp2, []byte("beta"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "apply",
		"format": "edits",
		"patches": []interface{}{
			map[string]interface{}{
				"path":  fp1,
				"edits": []interface{}{map[string]interface{}{"find": "alpha", "replace": "ALPHA"}},
			},
			map[string]interface{}{
				"path":  fp2,
				"edits": []interface{}{map[string]interface{}{"find": "beta", "replace": "BETA"}},
			},
		},
	})
	if r.Status != "success" {
		t.Fatalf("patch status=%s, error=%s", r.Status, r.Error)
	}

	c1, _ := os.ReadFile(fp1)
	c2, _ := os.ReadFile(fp2)
	if !strings.Contains(string(c1), "ALPHA") {
		t.Fatalf("a.txt content %q does not contain ALPHA", string(c1))
	}
	if !strings.Contains(string(c2), "BETA") {
		t.Fatalf("b.txt content %q does not contain BETA", string(c2))
	}
}

func TestPatch_EditsFormat_NoChanges(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(fp, []byte("no match here"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := WithWorkspaceRoot(context.Background(), dir)
	ctx = WithAccessMode(ctx, "workspace-write")
	m := NewManager()

	r := m.patchStructured(ctx, map[string]interface{}{
		"mode":   "apply",
		"format": "edits",
		"patches": []interface{}{
			map[string]interface{}{
				"path":  fp,
				"edits": []interface{}{map[string]interface{}{"find": "nonexistent", "replace": "replaced"}},
			},
		},
	})
	if r.Status != "success" {
		t.Fatalf("patch status=%s, error=%s", r.Status, r.Error)
	}
	changes, _ := r.Data["changes"].(int)
	if changes != 0 {
		t.Fatalf("expected 0 changes, got %d", changes)
	}
}

func TestPatch_InvalidMode(t *testing.T) {
	m := NewManager()
	r := m.patchStructured(context.Background(), map[string]interface{}{
		"mode": "invalid",
	})
	if r.Status != "error" {
		t.Fatalf("expected error for invalid mode, got %s", r.Status)
	}
}

func TestPatch_InvalidFormat(t *testing.T) {
	m := NewManager()
	r := m.patchStructured(context.Background(), map[string]interface{}{
		"format": "unknown",
	})
	if r.Status != "error" {
		t.Fatalf("expected error for invalid format, got %s", r.Status)
	}
}

func TestPatch_EditsFormat_EmptyPatches(t *testing.T) {
	m := NewManager()
	r := m.patchStructured(context.Background(), map[string]interface{}{
		"format":  "edits",
		"patches": []interface{}{},
	})
	if r.Status != "error" {
		t.Fatalf("expected error for empty patches, got %s", r.Status)
	}
}

func TestPatch_UnifiedFormat_EmptyDiff(t *testing.T) {
	m := NewManager()
	r := m.patchStructured(context.Background(), map[string]interface{}{
		"format": "unified",
		"diff":   "",
	})
	if r.Status != "error" {
		t.Fatalf("expected error for empty diff, got %s", r.Status)
	}
}

func TestPatch_RegisteredInGetAllToolDefinitions(t *testing.T) {
	defs := GetAllToolDefinitions()
	found := false
	for _, d := range defs {
		if d.Name == ToolPatch {
			found = true
			if d.RiskLevel != RiskLevelMedium {
				t.Fatalf("expected RiskLevelMedium, got %d", d.RiskLevel)
			}
			break
		}
	}
	if !found {
		t.Fatal("ToolPatch not found in GetAllToolDefinitions()")
	}
}

func TestParseUnifiedDiff(t *testing.T) {
	diff := "--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,3 @@\n package main\n-func old()\n+func new()\n func main() {}\n"
	patches := parseUnifiedDiff(diff)
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	if patches[0].toFile != "b/main.go" {
		t.Fatalf("toFile=%q, want b/main.go", patches[0].toFile)
	}
	if len(patches[0].hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(patches[0].hunks))
	}
	h := patches[0].hunks[0]
	if h.oldStart != 1 || h.oldCount != 3 {
		t.Fatalf("hunk old range: start=%d count=%d, want 1,3", h.oldStart, h.oldCount)
	}
	if h.newStart != 1 || h.newCount != 3 {
		t.Fatalf("hunk new range: start=%d count=%d, want 1,3", h.newStart, h.newCount)
	}
}
