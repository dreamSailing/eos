package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	codectx "github.com/dreamSailing/vb-coding/internal/context"
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

func TestSessionLock_ReferenceCount(t *testing.T) {
	root := t.TempDir()
	lockPath := sessionLockPathForRoot(root)

	acquireSessionLockPath(lockPath)
	acquireSessionLockPath(lockPath)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file after acquire: %v", err)
	}

	releaseSessionLockPath(lockPath)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file to remain until final release: %v", err)
	}

	releaseSessionLockPath(lockPath)
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file removed after final release, err=%v", err)
	}
}

func TestRuntimeCore_SyncSessionLock_FollowsActiveRoot(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()

	mgr := codectx.NewMultiEngine()
	mgr.AddRoot(first)
	mgr.AddRoot(second)
	mgr.SetActive(first)

	rc := &RuntimeCore{workspaceMgr: mgr}
	rc.syncSessionLock()

	firstLock := sessionLockPathForRoot(first)
	secondLock := sessionLockPathForRoot(second)
	if _, err := os.Stat(firstLock); err != nil {
		t.Fatalf("expected first lock file: %v", err)
	}

	if rc.SetActiveWorkspaceRoot(second) == nil {
		t.Fatalf("expected second root to become active")
	}
	if _, err := os.Stat(secondLock); err != nil {
		t.Fatalf("expected second lock file after switch: %v", err)
	}
	if _, err := os.Stat(firstLock); !os.IsNotExist(err) {
		t.Fatalf("expected first lock file removed after switch, err=%v", err)
	}

	rc.releaseHeldSessionLock()
	if _, err := os.Stat(secondLock); !os.IsNotExist(err) {
		t.Fatalf("expected second lock file removed after release, err=%v", err)
	}
}
