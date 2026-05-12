package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildVerifyPromptIncludesVerifierGuidance(t *testing.T) {
	root := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "frontend", "src"), 0o755); err != nil {
		t.Fatalf("mkdir frontend: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "frontend", "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	app := newTestAppModel(t)

	got := buildVerifyPrompt(app, []string{"验证图片上传链路"})
	for _, want := range []string{
		"不要被 80% 的成功欺骗",
		"VERDICT: PASS、FAIL 或 PARTIAL",
		"验收摘要",
		"覆盖到的验证项",
		"未覆盖的风险和空白",
		"关键证据",
		"Playwright",
		"验证图片上传链路",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildVerifyPrompt() missing %q\n%s", want, got)
		}
	}
}
