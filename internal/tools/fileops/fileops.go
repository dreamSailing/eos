package fileops

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

type FileOperations struct {
	root string
}

func NewFileOperations() *FileOperations {
	return &FileOperations{}
}

func (f *FileOperations) SetRoot(root string) {
	if f == nil {
		return
	}
	f.root = strings.TrimSpace(root)
}

func (f *FileOperations) Root() string {
	if f == nil {
		return ""
	}
	return strings.TrimSpace(f.root)
}

func (f *FileOperations) ReadFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	if !filepath.IsAbs(path) {
		slog.Error("fileops.read_file.path_invalid", "component", utils.ComponentTool,
			"path", path,
			"reason", "absolute path required",
		)
		return "", fmt.Errorf("invalid path: absolute path required")
	}

	// 验证文件是否适合读取
	valid, size, errMsg := utils.ValidateFileForRead(path, utils.MaxFileSize)
	if !valid {
		slog.Error("fileops.read_file.validation_failed", "component", utils.ComponentTool,
			"path", path,
			"size", size,
			"reason", errMsg,
		)
		return "", fmt.Errorf("cannot read file: %s", errMsg)
	}

	// 读取文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		slog.Error("fileops.read_file.error", "component", utils.ComponentTool,
			"path", path,
			"error", err.Error(),
		)
		return "", err
	}

	slog.Debug("fileops.read_file.success", "component", utils.ComponentTool,
		"path", path,
		"size", size,
	)
	return string(content), nil
}

func (f *FileOperations) WriteFile(path, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("fileops.write_file.mkdir.error", "component", utils.ComponentTool,
			"path", path,
			"dir", dir,
			"error", err.Error(),
		)
		return err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		slog.Error("fileops.write_file.error", "component", utils.ComponentTool,
			"path", path,
			"content_length", len(content),
			"error", err.Error(),
		)
		return err
	}
	return nil
}

func (f *FileOperations) DeleteFile(path string) error {
	if err := os.Remove(path); err != nil {
		slog.Error("fileops.delete_file.error", "component", utils.ComponentTool,
			"path", path,
			"error", err.Error(),
		)
		return err
	}
	return nil
}

func (f *FileOperations) MoveFile(source, dest string) error {
	// Create destination directory if it doesn't exist
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("fileops.move_file.mkdir.error", "component", utils.ComponentTool,
			"source", source,
			"dest", dest,
			"dir", dir,
			"error", err.Error(),
		)
		return err
	}
	if err := os.Rename(source, dest); err != nil {
		slog.Error("fileops.move_file.rename.error", "component", utils.ComponentTool,
			"source", source,
			"dest", dest,
			"error", err.Error(),
		)
		return err
	}
	return nil
}

func (f *FileOperations) DeleteDirectoryRecursive(path string) error {
	if err := os.RemoveAll(path); err != nil {
		slog.Error("fileops.delete_directory.error", "component", utils.ComponentTool,
			"path", path,
			"error", err.Error(),
		)
		return err
	}
	return nil
}

func (f *FileOperations) CopyFile(src, dst string) error {
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		slog.Error("fileops.copy_file.mkdir.error", "component", utils.ComponentTool,
			"src", src,
			"dst", dst,
			"dir", dstDir,
			"error", err.Error(),
		)
		return err
	}
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := sourceFile.Close(); err != nil {
			slog.Error("fileops.copy_file.close_source.error", "component", utils.ComponentTool,
				"src", src,
				"error", err.Error(),
			)
		}
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := destFile.Close(); err != nil {
			slog.Error("fileops.copy_file.close_dest.error", "component", utils.ComponentTool,
				"dst", dst,
				"error", err.Error(),
			)
		}
	}()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		slog.Error("fileops.copy_file.copy.error", "component", utils.ComponentTool,
			"src", src,
			"dst", dst,
			"error", err.Error(),
		)
		return err
	}
	return nil
}

func (f *FileOperations) IsTextFile(path string) bool {
	// Simple heuristic: read first 512 bytes and check for null byte
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Error("fileops.is_text_file.close.error", "component", utils.ComponentTool,
				"path", path,
				"error", err.Error(),
			)
		}
	}()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	buf = buf[:n]

	if strings.Contains(filepath.Base(path), ".") {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".txt", ".md", ".go", ".js", ".ts", ".py", ".html", ".css", ".json", ".yaml", ".yml", ".xml", ".toml", ".sh", ".bat", ".ps1", ".java", ".c", ".cpp", ".h", ".hpp", ".rs", ".rb", ".php", ".cs", ".swift", ".kt", ".lua", ".pl", ".r", ".sql", ".eos", ".vbs", ".ini", ".conf", ".cfg", ".properties", ".env", ".gitignore", ".dockerfile", "dockerfile", "makefile":
			return true
		}
	}

	for _, b := range buf {
		if b == 0 {
			return false
		}
	}
	return true
}

func (f *FileOperations) PathExists(path string) (bool, bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return true, info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, false, nil
	}
	return false, false, err
}

func (f *FileOperations) ListDirectory(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		slog.Error("fileops.list_directory.error", "component", utils.ComponentTool,
			"path", path,
			"error", err.Error(),
		)
		return nil, err
	}
	var result []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		result = append(result, name)
	}
	return result, nil
}

func (f *FileOperations) CreateDirectory(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		slog.Error("fileops.create_directory.error", "component", utils.ComponentTool,
			"path", path,
			"error", err.Error(),
		)
		return err
	}
	return nil
}

func (f *FileOperations) CopyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		return f.CopyFile(path, dstPath)
	})
}
