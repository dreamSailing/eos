package fileops

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

type VersionMeta struct {
	ID         string
	PathRel    string
	Timestamp  time.Time
	Size       int
	SHA256     string
	PrevSHA256 string
}

type VersionFileEntry struct {
	PathRel      string
	VersionCount int
	LastModified time.Time
	TotalSize    int
}

type VersionExtra struct {
	TraceID   string
	Tool      string
	Operation string
}

func versionsDirFor(absPath string) (string, string, error) {
	wd, _ := os.Getwd()
	rel, err := filepath.Rel(wd, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", err
	}
	dir := filepath.Join(wd, ".vb", "versions", filepath.FromSlash(rel))
	return dir, rel, nil
}

func (f *FileOperations) SaveVersion(absPath string, oldContent string) (VersionMeta, error) {
	return f.SaveVersionWithExtra(absPath, oldContent, VersionExtra{})
}

func (f *FileOperations) SaveVersionWithExtra(absPath string, oldContent string, extra VersionExtra) (VersionMeta, error) {
	vm := VersionMeta{}
	dir, rel, err := versionsDirFor(absPath)
	if err != nil {
		slog.Error("fileops.save_version.versions_dir.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"error", err.Error(),
		)
		return vm, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("fileops.save_version.mkdir.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"dir", dir,
			"error", err.Error(),
		)
		return vm, err
	}
	now := time.Now().UTC()
	base := now.Format("20060102-150405.000000000")
	ts := base
	sum := sha256.Sum256([]byte(oldContent))
	sha := hex.EncodeToString(sum[:])
	vm = VersionMeta{ID: ts, PathRel: rel, Timestamp: now, Size: len(oldContent), SHA256: sha}
	if b, err := os.ReadFile(absPath); err == nil {
		p := sha256.Sum256(b)
		vm.PrevSHA256 = hex.EncodeToString(p[:])
	} else {
		slog.Warn("fileops.save_version.read_current.warn", "component", utils.ComponentTool,
			"abs_path", absPath,
			"error", err.Error(),
		)
	}
	contentPath := filepath.Join(dir, ts+".content")
	for i := 1; i <= 50; i++ {
		if _, err := os.Stat(contentPath); err != nil {
			break
		}
		ts = fmt.Sprintf("%s-%d", base, i)
		vm.ID = ts
		contentPath = filepath.Join(dir, ts+".content")
	}
	if err := os.WriteFile(contentPath, []byte(oldContent), 0644); err != nil {
		slog.Error("fileops.save_version.write_content.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"content_path", contentPath,
			"size", len(oldContent),
			"error", err.Error(),
		)
		return vm, err
	}
	meta := strings.Builder{}
	meta.WriteString("id=" + ts + "\n")
	meta.WriteString("path=" + filepath.ToSlash(rel) + "\n")
	meta.WriteString("ts_utc=" + now.Format(time.RFC3339Nano) + "\n")
	meta.WriteString("size=" + strconvI(vm.Size) + "\n")
	meta.WriteString("sha256=" + vm.SHA256 + "\n")
	if vm.PrevSHA256 != "" {
		meta.WriteString("prev=" + vm.PrevSHA256 + "\n")
	}
	if strings.TrimSpace(extra.TraceID) != "" {
		meta.WriteString("trace_id=" + strings.TrimSpace(extra.TraceID) + "\n")
	}
	if strings.TrimSpace(extra.Tool) != "" {
		meta.WriteString("tool=" + strings.TrimSpace(extra.Tool) + "\n")
	}
	if strings.TrimSpace(extra.Operation) != "" {
		meta.WriteString("op=" + strings.TrimSpace(extra.Operation) + "\n")
	}
	metaPath := filepath.Join(dir, ts+".meta")
	if err := os.WriteFile(metaPath, []byte(meta.String()), 0644); err != nil {
		slog.Error("fileops.save_version.write_meta.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"meta_path", metaPath,
			"error", err.Error(),
		)
		return vm, err
	}
	writeVersionIndex(rel, vm, extra)
	if strings.TrimSpace(extra.TraceID) != "" {
		_ = recordCheckpoint(strings.TrimSpace(extra.TraceID), filepath.ToSlash(rel), vm.ID)
	}
	_ = f.trimOldVersions(dir, 20)
	return vm, nil
}

var checkpointMu sync.Mutex

type Checkpoint struct {
	TraceID   string            `json:"trace_id"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at,omitempty"`
	UserText  string            `json:"user_text,omitempty"`
	Success   *bool             `json:"success,omitempty"`
	Files     map[string]string `json:"files"`
}

func checkpointPath(traceID string) (string, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return "", fmt.Errorf("trace_id required")
	}
	wd, _ := os.Getwd()
	dir := filepath.Join(wd, ".vb", "versions", "_checkpoints")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, traceID+".json"), nil
}

func LoadCheckpoint(traceID string) (*Checkpoint, error) {
	p, err := checkpointPath(traceID)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(b, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

func UpdateCheckpointSummary(traceID string, userText string, success bool) error {
	p, err := checkpointPath(traceID)
	if err != nil {
		return err
	}
	checkpointMu.Lock()
	defer checkpointMu.Unlock()
	var cp Checkpoint
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &cp)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if cp.TraceID == "" {
		cp.TraceID = strings.TrimSpace(traceID)
		cp.CreatedAt = now
	}
	cp.UpdatedAt = now
	cp.UserText = strings.TrimSpace(userText)
	s := success
	cp.Success = &s
	if cp.Files == nil {
		cp.Files = map[string]string{}
	}
	out, err := json.MarshalIndent(&cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, out, 0644)
}

func recordCheckpoint(traceID string, pathRel string, versionID string) error {
	p, err := checkpointPath(traceID)
	if err != nil {
		return err
	}
	checkpointMu.Lock()
	defer checkpointMu.Unlock()
	var cp Checkpoint
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &cp)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if cp.TraceID == "" {
		cp.TraceID = strings.TrimSpace(traceID)
		cp.CreatedAt = now
	}
	cp.UpdatedAt = now
	if cp.Files == nil {
		cp.Files = map[string]string{}
	}
	if _, exists := cp.Files[pathRel]; exists {
		return nil
	}
	cp.Files[pathRel] = strings.TrimSpace(versionID)
	out, err := json.MarshalIndent(&cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, out, 0644)
}

func writeVersionIndex(pathRel string, vm VersionMeta, extra VersionExtra) {
	wd, _ := os.Getwd()
	p := filepath.Join(wd, ".vb", "versions", "_index.jsonl")
	dir := filepath.Dir(p)
	_ = os.MkdirAll(dir, 0755)
	type row struct {
		TraceID   string `json:"trace_id,omitempty"`
		PathRel   string `json:"path_rel"`
		VersionID string `json:"version_id"`
		TsUTC     string `json:"ts_utc"`
		Tool      string `json:"tool,omitempty"`
		Operation string `json:"op,omitempty"`
		Size      int    `json:"size"`
		SHA256    string `json:"sha256"`
	}
	r := row{
		TraceID:   strings.TrimSpace(extra.TraceID),
		PathRel:   filepath.ToSlash(strings.TrimSpace(pathRel)),
		VersionID: strings.TrimSpace(vm.ID),
		TsUTC:     vm.Timestamp.UTC().Format(time.RFC3339Nano),
		Tool:      strings.TrimSpace(extra.Tool),
		Operation: strings.TrimSpace(extra.Operation),
		Size:      vm.Size,
		SHA256:    vm.SHA256,
	}
	b, err := json.Marshal(&r)
	if err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

type CheckpointSummary struct {
	TraceID   string
	CreatedAt string
	UpdatedAt string
	UserText  string
	Success   *bool
	FileCount int
}

func ListCheckpoints(limit int) ([]CheckpointSummary, error) {
	wd, _ := os.Getwd()
	dir := filepath.Join(wd, ".vb", "versions", "_checkpoints")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []CheckpointSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(b, &cp); err != nil {
			continue
		}
		cnt := 0
		if cp.Files != nil {
			cnt = len(cp.Files)
		}
		out = append(out, CheckpointSummary{
			TraceID:   strings.TrimSpace(cp.TraceID),
			CreatedAt: strings.TrimSpace(cp.CreatedAt),
			UpdatedAt: strings.TrimSpace(cp.UpdatedAt),
			UserText:  strings.TrimSpace(cp.UserText),
			Success:   cp.Success,
			FileCount: cnt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a := out[i].UpdatedAt
		if a == "" {
			a = out[i].CreatedAt
		}
		b := out[j].UpdatedAt
		if b == "" {
			b = out[j].CreatedAt
		}
		return a > b
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *FileOperations) trimOldVersions(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("fileops.trim_versions.readdir.error", "component", utils.ComponentTool,
			"dir", dir,
			"keep", keep,
			"error", err.Error(),
		)
		return nil
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".content") {
			ids = append(ids, strings.TrimSuffix(name, ".content"))
		}
	}
	sort.Strings(ids)
	if len(ids) <= keep {
		return nil
	}
	drop := ids[:len(ids)-keep]
	for _, id := range drop {
		_ = os.Remove(filepath.Join(dir, id+".content"))
		_ = os.Remove(filepath.Join(dir, id+".meta"))
	}
	return nil
}

func (f *FileOperations) ListVersions(absPath string) ([]VersionMeta, error) {
	dir, rel, err := versionsDirFor(absPath)
	if err != nil {
		slog.Error("fileops.list_versions.versions_dir.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"error", err.Error(),
		)
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("fileops.list_versions.readdir.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"dir", dir,
			"error", err.Error(),
		)
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".content") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".content"))
		}
	}
	sort.Strings(ids)
	var out []VersionMeta
	for _, id := range ids {
		p := filepath.Join(dir, id+".content")
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(b)
		out = append(out, VersionMeta{ID: id, PathRel: rel, Size: len(b), SHA256: hex.EncodeToString(sum[:])})
	}
	return out, nil
}

func (f *FileOperations) ReadVersion(absPath, id string) (string, error) {
	dir, _, err := versionsDirFor(absPath)
	if err != nil {
		slog.Error("fileops.read_version.versions_dir.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"id", id,
			"error", err.Error(),
		)
		return "", err
	}
	p := filepath.Join(dir, id+".content")
	b, err := os.ReadFile(p)
	if err != nil {
		slog.Error("fileops.read_version.read_file.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"id", id,
			"content_path", p,
			"error", err.Error(),
		)
		return "", err
	}
	return string(b), nil
}

// DeleteVersion 删除指定文件的单个版本
// 返回删除后剩余的版本数量和可能的错误
func (f *FileOperations) DeleteVersion(absPath, versionID string) (int, error) {
	dir, _, err := versionsDirFor(absPath)
	if err != nil {
		slog.Error("fileops.delete_version.versions_dir.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"version_id", versionID,
			"error", err.Error(),
		)
		return 0, err
	}

	// 删除版本文件
	contentPath := filepath.Join(dir, versionID+".content")
	metaPath := filepath.Join(dir, versionID+".meta")

	contentErr := os.Remove(contentPath)
	metaErr := os.Remove(metaPath)

	// 如果两个文件都不存在，返回错误
	if contentErr != nil && metaErr != nil {
		slog.Error("fileops.delete_version.remove.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"version_id", versionID,
			"content_error", contentErr.Error(),
			"meta_error", metaErr.Error(),
		)
		return 0, fmt.Errorf("version not found: %s", versionID)
	}

	// 统计剩余版本数量
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil
	}
	remaining := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".content") {
			remaining++
		}
	}

	slog.Info("fileops.delete_version.success", "component", utils.ComponentTool,
		"abs_path", absPath,
		"version_id", versionID,
		"remaining", remaining,
	)

	return remaining, nil
}

// DeleteAllVersions 删除指定文件的所有版本（整个版本目录）
func (f *FileOperations) DeleteAllVersions(absPath string) error {
	dir, _, err := versionsDirFor(absPath)
	if err != nil {
		slog.Error("fileops.delete_all_versions.versions_dir.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"error", err.Error(),
		)
		return err
	}

	// 删除整个版本目录
	err = os.RemoveAll(dir)
	if err != nil {
		slog.Error("fileops.delete_all_versions.remove.error", "component", utils.ComponentTool,
			"abs_path", absPath,
			"dir", dir,
			"error", err.Error(),
		)
		return err
	}

	slog.Info("fileops.delete_all_versions.success", "component", utils.ComponentTool,
		"abs_path", absPath,
		"dir", dir,
	)

	return nil
}

func strconvI(n int) string     { return strconvI64(int64(n)) }
func strconvI64(n int64) string { return strings.TrimSpace(fmt.Sprintf("%d", n)) }
