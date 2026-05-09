package git

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Checkpoint Git 快照
type Checkpoint struct {
	TreeHash    string    // Git tree hash
	CommitHash  string    // 可选的提交哈希
	Timestamp   time.Time // 创建时间
	Description string    // 描述
	Branch      string    // 当前分支
	Files       []string  // 受影响的文件列表
}

// CheckpointManager 快照管理器
type CheckpointManager struct {
	rootDir        string
	checkpoints    []Checkpoint
	maxCheckpoints int
	mu             sync.RWMutex
	enabled        bool
}

// NewCheckpointManager 创建快照管理器
func NewCheckpointManager(rootDir string) *CheckpointManager {
	return &CheckpointManager{
		rootDir:        rootDir,
		checkpoints:    make([]Checkpoint, 0, 50),
		maxCheckpoints: 50,
		enabled:        true,
	}
}

// SetEnabled 设置是否启用快照
func (m *CheckpointManager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// SetMaxCheckpoints 设置最大快照数量
func (m *CheckpointManager) SetMaxCheckpoints(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxCheckpoints = max
}

// IsEnabled 检查是否启用快照
func (m *CheckpointManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Create 创建快照
func (m *CheckpointManager) Create(description string, affectedFiles []string) (*Checkpoint, error) {
	if !m.IsEnabled() {
		return nil, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否在 Git 仓库中
	if !m.isGitRepo() {
		slog.Debug("checkpoint.skip.not_git_repo", "root_dir", m.rootDir)
		return nil, nil
	}

	// 获取当前分支
	branch, err := m.getCurrentBranch()
	if err != nil {
		branch = "unknown"
	}

	// 创建 Git tree hash
	treeHash, err := m.writeTree()
	if err != nil {
		slog.Warn("checkpoint.create.write_tree.error", "error", err)
		return nil, fmt.Errorf("failed to write tree: %w", err)
	}

	checkpoint := Checkpoint{
		TreeHash:    treeHash,
		Timestamp:   time.Now(),
		Description: description,
		Branch:      branch,
		Files:       affectedFiles,
	}

	m.checkpoints = append(m.checkpoints, checkpoint)

	// 限制快照数量
	if len(m.checkpoints) > m.maxCheckpoints {
		m.checkpoints = m.checkpoints[len(m.checkpoints)-m.maxCheckpoints:]
	}

	slog.Debug("checkpoint.created",
		"tree_hash", treeHash,
		"description", description,
		"branch", branch,
		"files", len(affectedFiles),
	)

	return &checkpoint, nil
}

// Restore 恢复到指定快照
func (m *CheckpointManager) Restore(treeHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cp := range m.checkpoints {
		if cp.TreeHash == treeHash {
			return m.restoreTree(&cp)
		}
	}
	return fmt.Errorf("checkpoint not found: %s", treeHash)
}

// RestoreLatest 恢复到最新快照
func (m *CheckpointManager) RestoreLatest() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.checkpoints) == 0 {
		return fmt.Errorf("no checkpoints available")
	}

	latest := m.checkpoints[len(m.checkpoints)-1]
	return m.restoreTree(&latest)
}

// List 列出所有快照
func (m *CheckpointManager) List() []Checkpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Checkpoint, len(m.checkpoints))
	copy(result, m.checkpoints)
	return result
}

// GetLatest 获取最新快照
func (m *CheckpointManager) GetLatest() *Checkpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.checkpoints) == 0 {
		return nil
	}
	return &m.checkpoints[len(m.checkpoints)-1]
}

// Clear 清除所有快照
func (m *CheckpointManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints = make([]Checkpoint, 0)
}

// isGitRepo 检查是否在 Git 仓库中
func (m *CheckpointManager) isGitRepo() bool {
	gitDir := filepath.Join(m.rootDir, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

// writeTree 创建 Git tree 对象
func (m *CheckpointManager) writeTree() (string, error) {
	cmd := utils.Command("git", "write-tree")
	cmd.Dir = m.rootDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderrBuffer{}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git write-tree failed: %w", err)
	}

	treeHash := strings.TrimSpace(stdout.String())
	return treeHash, nil
}

// getCurrentBranch 获取当前分支
func (m *CheckpointManager) getCurrentBranch() (string, error) {
	cmd := utils.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = m.rootDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderrBuffer{}

	if err := cmd.Run(); err != nil {
		return "", err
	}

	branch := strings.TrimSpace(stdout.String())
	return branch, nil
}

// restoreTree 恢复到指定 tree
func (m *CheckpointManager) restoreTree(cp *Checkpoint) error {
	// 读取 tree 内容到索引
	cmd := utils.Command("git", "read-tree", cp.TreeHash)
	cmd.Dir = m.rootDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git read-tree failed: %w", err)
	}

	// 检出索引内容
	cmd = utils.Command("git", "checkout-index", "-f", "-a")
	cmd.Dir = m.rootDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git checkout-index failed: %w", err)
	}

	slog.Info("checkpoint.restored",
		"tree_hash", cp.TreeHash,
		"description", cp.Description,
		"timestamp", cp.Timestamp.Format(time.RFC3339),
	)

	return nil
}

// CreateCheckpointBeforeOperation 在危险操作前创建快照
func CreateCheckpointBeforeOperation(rootDir, operation string, affectedFiles []string) (*Checkpoint, error) {
	manager := NewCheckpointManager(rootDir)
	return manager.Create(fmt.Sprintf("Before %s", operation), affectedFiles)
}

// IsDangerousOperation 检查是否为危险操作
func IsDangerousOperation(operation string) bool {
	dangerousOps := []string{
		"delete_file",
		"delete_dir",
		"overwrite_file",
		"recursive_delete",
		"batch_delete",
		"force_push",
		"reset_hard",
		"clean",
	}

	op := strings.ToLower(operation)
	for _, dangerous := range dangerousOps {
		if strings.Contains(op, dangerous) {
			return true
		}
	}
	return false
}

// CheckpointConfig 快照配置
type CheckpointConfig struct {
	AutoCreate       bool     // 是否在危险操作前自动创建
	MaxCheckpoints   int      // 最大快照数量
	ExcludedPaths    []string // 排除的路径
	IncludeUntracked bool     // 是否包含未跟踪的文件
}

// DefaultCheckpointConfig 默认快照配置
func DefaultCheckpointConfig() *CheckpointConfig {
	return &CheckpointConfig{
		AutoCreate:       true,
		MaxCheckpoints:   50,
		ExcludedPaths:    []string{".eos", "node_modules", ".git"},
		IncludeUntracked: false,
	}
}

// stderrBuffer 用于捕获 stderr
type stderrBuffer struct {
	bytes.Buffer
}

func (s *stderrBuffer) Write(p []byte) (int, error) {
	// 静默处理 stderr
	return len(p), nil
}
