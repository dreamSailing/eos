package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreUpsertRoutesDedupesAndBuildsIndex(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	root := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	setMemoryTestHome(t, home)

	store := NewStore(root)
	globalRes, err := store.Upsert(MemoryEntry{
		Type:    MemoryTypeGlobal,
		Section: "用户偏好",
		Content: "默认使用中文回答",
	})
	if err != nil {
		t.Fatalf("global upsert failed: %v", err)
	}
	if want := filepath.Join(home, ".eos", "memory", "user.md"); globalRes.Path != want {
		t.Fatalf("global path=%q, want %q", globalRes.Path, want)
	}

	dupRes, err := store.Upsert(MemoryEntry{
		Type:    MemoryTypeGlobal,
		Section: "用户偏好",
		Content: "默认使用中文回答",
	})
	if err != nil {
		t.Fatalf("duplicate upsert failed: %v", err)
	}
	if !dupRes.Deduped {
		t.Fatalf("expected duplicate write to be deduped")
	}

	projectRes, err := store.Upsert(MemoryEntry{
		Type:    MemoryTypeProject,
		Section: "任务结论",
		Content: "为当前仓库新增独立 memory 文件体系",
	})
	if err != nil {
		t.Fatalf("project upsert failed: %v", err)
	}
	if want := filepath.Join(root, ".eos", "memory", "project.md"); projectRes.Path != want {
		t.Fatalf("project path=%q, want %q", projectRes.Path, want)
	}

	globalContent, err := os.ReadFile(globalRes.Path)
	if err != nil {
		t.Fatalf("read global memory failed: %v", err)
	}
	if !strings.Contains(string(globalContent), "默认使用中文回答") {
		t.Fatalf("global memory content missing expected entry")
	}

	projectContent, err := os.ReadFile(projectRes.Path)
	if err != nil {
		t.Fatalf("read project memory failed: %v", err)
	}
	if !strings.Contains(string(projectContent), "独立 memory 文件体系") {
		t.Fatalf("project memory content missing expected entry")
	}

	indexContent, err := os.ReadFile(ProjectMemoryIndexPath(root))
	if err != nil {
		t.Fatalf("read memory index failed: %v", err)
	}
	indexText := string(indexContent)
	if !strings.Contains(indexText, "## global") || !strings.Contains(indexText, "## project") {
		t.Fatalf("memory index missing expected sections: %s", indexText)
	}
}

func setMemoryTestHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if volume := filepath.VolumeName(home); volume != "" {
		t.Setenv("HOMEDRIVE", volume)
		t.Setenv("HOMEPATH", strings.TrimPrefix(home, volume))
	}
}
