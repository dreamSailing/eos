package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionLock_AcquireAndRelease(t *testing.T) {
	// 使用临时目录模拟
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// 创建 .vb/sessions 目录
	sessDir := filepath.Join(tmpDir, ".vb", "sessions")
	_ = os.MkdirAll(sessDir, 0755)

	lockPath := filepath.Join(sessDir, ".lock")

	// 获取锁
	AcquireSessionLock()
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatalf("expected lock file to exist after acquire")
	}

	// 验证 lock 文件内容
	b, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}
	var info sessionLockInfo
	if err := json.Unmarshal(b, &info); err != nil {
		t.Fatalf("failed to unmarshal lock info: %v", err)
	}
	if info.PID != os.Getpid() {
		t.Fatalf("expected PID %d, got %d", os.Getpid(), info.PID)
	}
	if info.StartedAt == 0 {
		t.Fatalf("expected non-zero started_at")
	}

	// 释放锁
	ReleaseSessionLock()
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file to be removed after release")
	}
}

func TestDetectCrashRecovery_NoLock(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	result := DetectCrashRecovery()
	if result.Detected {
		t.Fatalf("expected no crash detected without lock file")
	}
}

func TestDetectCrashRecovery_StaleLock(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	sessDir := filepath.Join(tmpDir, ".vb", "sessions")
	_ = os.MkdirAll(sessDir, 0755)

	// 写入一个假的 lock 文件（使用不存在的 PID）
	lockInfo := sessionLockInfo{PID: 99999999, StartedAt: 1700000000}
	b, _ := json.Marshal(lockInfo)
	lockPath := filepath.Join(sessDir, ".lock")
	_ = os.WriteFile(lockPath, b, 0644)

	// 写入一个假的会话文件
	session := PersistedSession{ID: "test-session-123", SavedAt: 1700000001}
	sb, _ := json.Marshal(session)
	_ = os.WriteFile(filepath.Join(sessDir, "test-session-123.json"), sb, 0644)

	result := DetectCrashRecovery()
	if !result.Detected {
		t.Fatalf("expected crash detected with stale lock file")
	}
	if result.SessionID != "test-session-123" {
		t.Fatalf("expected session ID 'test-session-123', got %q", result.SessionID)
	}

	// lock 文件应该被清除
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file to be cleaned up after detection")
	}
}
