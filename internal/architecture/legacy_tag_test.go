package architecture

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var legacyTaggedDirectories = []string{
	"pkg/core",
	"cmd/eos-tool-host",
	"pkg/coreapi/parity",
	"internal/bridge",
}

var legacyTaggedFiles = []string{
	"pkg/coreapi/sidecar/toolhost/legacy_host.go",
	"pkg/coreapi/sidecar/toolhost/legacy_host_test.go",
	"internal/cli/app_server.go",
	"internal/cli/app_server_test.go",
	"internal/ui/adapter/runtime_legacy_test.go",
	"internal/ui/adapter/runtime_jsonrpc_test.go",
}

func TestLegacyPackagesHaveBuildTags(t *testing.T) {
	root := moduleRoot(t)
	var missing []string

	for _, dir := range legacyTaggedDirectories {
		absDir := filepath.Join(root, filepath.FromSlash(dir))
		entries, err := os.ReadDir(absDir)
		if err != nil {
			t.Logf("skip directory %s: %v", dir, err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			path := filepath.Join(absDir, entry.Name())
			if !hasLegacyBuildTag(path) {
				rel := filepath.Join(dir, entry.Name())
				missing = append(missing, rel)
			}
		}
	}

	for _, file := range legacyTaggedFiles {
		path := filepath.Join(root, filepath.FromSlash(file))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		if !hasLegacyBuildTag(path) {
			missing = append(missing, file)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("legacy files missing //go:build legacy tag:\n  %s\n\n"+
			"All files in legacy-tagged directories and explicit legacy files must have the tag.",
			strings.Join(missing, "\n  "))
	}
}

func TestLegacyBuildTagImportConsistency(t *testing.T) {
	root := moduleRoot(t)
	legacyImportPaths := []string{
		"github.com/dreamSailing/eos/pkg/core",
		"github.com/dreamSailing/eos/internal/bridge",
	}
	var violations []string

	for _, dir := range legacyTaggedDirectories {
		absDir := filepath.Join(root, filepath.FromSlash(dir))
		entries, err := os.ReadDir(absDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(absDir, entry.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			for _, legacyPkg := range legacyImportPaths {
				if strings.Contains(string(src), `"`+legacyPkg+`"`) {
					if !hasLegacyBuildTag(path) {
						rel := filepath.Join(dir, entry.Name())
						violations = append(violations, fmt.Sprintf("%s imports %s but lacks //go:build legacy", rel, legacyPkg))
					}
				}
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("files importing legacy packages must have //go:build legacy:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

func hasLegacyBuildTag(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			if line == "//go:build legacy" || strings.HasPrefix(line, "//go:build legacy ") {
				return true
			}
			continue
		}
		return false
	}
	return false
}
