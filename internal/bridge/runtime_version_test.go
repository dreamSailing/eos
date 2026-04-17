package bridge

import (
	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/tools/fileops"
	"os"
	"path/filepath"
	"testing"
)

func TestListVersionFiles_Recursive(t *testing.T) {
	tmp := t.TempDir()
	wd, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(wd) }()

	p := filepath.Join(tmp, "dir", "sub", "file.txt")
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	_ = os.WriteFile(p, []byte("new\n"), 0644)

	fo := fileops.NewFileOperations()
	_, err := fo.SaveVersionWithExtra(p, "old\n", fileops.VersionExtra{TraceID: "t1", Tool: "test", Operation: "save"})
	if err != nil {
		t.Fatalf("SaveVersionWithExtra error: %v", err)
	}

	rc := &RuntimeCore{}
	files, err := rc.ListVersionFiles()
	if err != nil {
		t.Fatalf("ListVersionFiles error: %v", err)
	}
	found := false
	for _, f := range files {
		if f.PathRel == "dir/sub/file.txt" && f.VersionCount >= 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected version file entry for dir/sub/file.txt, got: %#v", files)
	}
}

func TestListVersionFiles_UsesActiveRoot(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "dir", "sub", "file.txt")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(p, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	versionDir := filepath.Join(tmp, ".eos", "versions", "dir", "sub", "file.txt")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(versionDir) error = %v", err)
	}
	versionID := "20260328-120000"
	versionPath := filepath.Join(versionDir, versionID+".content")
	if err := os.WriteFile(versionPath, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(versionPath) error = %v", err)
	}

	mgr := codectx.NewMultiEngine()
	mgr.AddRoot(tmp)
	mgr.SetActive(tmp)
	rc := &RuntimeCore{workspaceMgr: mgr}

	files, err := rc.ListVersionFiles()
	if err != nil {
		t.Fatalf("ListVersionFiles error: %v", err)
	}
	if len(files) != 1 || files[0].PathRel != "dir/sub/file.txt" {
		t.Fatalf("expected active-root version file entry, got: %#v", files)
	}

	versions, err := rc.ListVersionsForPath(p)
	if err != nil {
		t.Fatalf("ListVersionsForPath() error = %v", err)
	}
	if len(versions) == 0 || versions[0].PathRel != "dir/sub/file.txt" || versions[0].ID != versionID {
		t.Fatalf("expected versions for active-root file, got: %#v", versions)
	}
}
