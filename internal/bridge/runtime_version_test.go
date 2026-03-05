package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"github.com/dreamSailing/vb-coding/internal/tools/fileops"
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

