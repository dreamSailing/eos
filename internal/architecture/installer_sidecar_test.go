package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerIncludesRustSidecarArtifacts(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "installer.iss"))
	if err != nil {
		t.Fatalf("read installer.iss: %v", err)
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	required := []string{
		`Source: "pkg\coreapi\sidecar\core\x86_64-pc-windows-gnu\eos-core.exe"; DestDir: "{app}\core\x86_64-pc-windows-gnu";`,
		`Source: "pkg\coreapi\sidecar\core\x86_64-pc-windows-gnu\manifest.json"; DestDir: "{app}\core\x86_64-pc-windows-gnu";`,
	}
	for _, line := range required {
		if !strings.Contains(content, line) {
			t.Fatalf("installer.iss must include Rust sidecar artifact line:\n%s", line)
		}
	}
}
