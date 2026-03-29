package bridge

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

// sessionLockInfo lock 文件中存储的信息
type sessionLockInfo struct {
	PID       int   `json:"pid"`
	StartedAt int64 `json:"started_at"`
}

var (
	sessionLockMu   sync.Mutex
	sessionLockRefs = map[string]int{}
)

// sessionLockPath 返回 lock 文件路径
func sessionLockPath() string {
	return sessionLockPathForRoot("")
}

func sessionLockPathForRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return ""
		}
		root = cwd
	}
	return filepath.Join(root, ".vb", "sessions", ".lock")
}

// AcquireSessionLock 创建 lock 文件，写入 PID + 时间戳
func AcquireSessionLock() {
	acquireSessionLockPath(sessionLockPath())
}

func acquireSessionLockPath(p string) {
	if p == "" {
		return
	}

	sessionLockMu.Lock()
	if n := sessionLockRefs[p]; n > 0 {
		sessionLockRefs[p] = n + 1
		sessionLockMu.Unlock()
		return
	}
	sessionLockMu.Unlock()

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Debug("session_lock.acquire.mkdir.error",
			"component", utils.ComponentSystem,
			"error", err)
		return
	}

	info := sessionLockInfo{
		PID:       os.Getpid(),
		StartedAt: time.Now().Unix(),
	}
	b, err := json.Marshal(info)
	if err != nil {
		return
	}
	if err := os.WriteFile(p, b, 0644); err != nil {
		slog.Debug("session_lock.acquire.write.error",
			"component", utils.ComponentSystem,
			"error", err)
		return
	}
	sessionLockMu.Lock()
	sessionLockRefs[p] = 1
	sessionLockMu.Unlock()
	slog.Debug("session_lock.acquired",
		"component", utils.ComponentSystem,
		"pid", info.PID,
		"path", p)
}

// ReleaseSessionLock 删除 lock 文件
func ReleaseSessionLock() {
	releaseSessionLockPath(sessionLockPath())
}

func releaseSessionLockPath(p string) {
	if p == "" {
		return
	}

	sessionLockMu.Lock()
	if n := sessionLockRefs[p]; n > 1 {
		sessionLockRefs[p] = n - 1
		sessionLockMu.Unlock()
		return
	}
	delete(sessionLockRefs, p)
	sessionLockMu.Unlock()

	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		slog.Debug("session_lock.release.error",
			"component", utils.ComponentSystem,
			"error", err)
	}
}

// CrashRecoveryInfo 非正常退出的恢复信息
type CrashRecoveryInfo struct {
	Detected  bool
	PID       int
	StartedAt time.Time
	SessionID string
}

// DetectCrashRecovery 检测上次是否非正常退出
// 如果 lock 文件存在且进程已不运行，说明非正常退出
func DetectCrashRecovery() CrashRecoveryInfo {
	return detectCrashRecoveryAtPath(sessionLockPath())
}

func detectCrashRecoveryAtPath(p string) CrashRecoveryInfo {
	if p == "" {
		return CrashRecoveryInfo{}
	}

	b, err := os.ReadFile(p)
	if err != nil {
		return CrashRecoveryInfo{}
	}

	var info sessionLockInfo
	if err := json.Unmarshal(b, &info); err != nil {
		// lock 文件损坏，清除
		_ = os.Remove(p)
		return CrashRecoveryInfo{}
	}

	// 检查进程是否仍在运行
	if isProcessRunning(info.PID) {
		return CrashRecoveryInfo{}
	}

	// 查找最近的会话
	dir := filepath.Dir(p)
	metas, err := listSessionsInDir(dir)
	if err != nil || len(metas) == 0 {
		_ = os.Remove(p)
		return CrashRecoveryInfo{}
	}

	// 查找 lock 之后保存的会话
	sessionID := ""
	for _, m := range metas {
		if m.SavedAt >= info.StartedAt {
			sessionID = m.ID
			break
		}
	}
	if sessionID == "" {
		sessionID = metas[0].ID
	}

	// 清除旧 lock
	_ = os.Remove(p)

	slog.Info("session_lock.crash_detected",
		"component", utils.ComponentSystem,
		"pid", info.PID,
		"started_at", info.StartedAt,
		"session_id", sessionID)

	return CrashRecoveryInfo{
		Detected:  true,
		PID:       info.PID,
		StartedAt: time.Unix(info.StartedAt, 0),
		SessionID: sessionID,
	}
}

func (rc *RuntimeCore) syncSessionLock() {
	if rc == nil {
		return
	}

	next := sessionLockPathForRoot(rc.workingRoot())
	rc.mu.Lock()
	prev := rc.sessionLockPath
	if strings.EqualFold(filepath.Clean(prev), filepath.Clean(next)) {
		rc.mu.Unlock()
		return
	}
	rc.sessionLockPath = next
	rc.mu.Unlock()

	acquireSessionLockPath(next)
	releaseSessionLockPath(prev)
}

func (rc *RuntimeCore) releaseHeldSessionLock() {
	if rc == nil {
		return
	}

	rc.mu.Lock()
	p := rc.sessionLockPath
	rc.sessionLockPath = ""
	rc.mu.Unlock()
	releaseSessionLockPath(p)
}

// isProcessRunning 检查指定 PID 的进程是否仍在运行
func isProcessRunning(pid int) bool {
	// os.FindProcess 在 Unix 上总是成功，需要发送信号来检查
	// 在 Windows 上，FindProcess 只有进程存在时才成功
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// 尝试以只读方式查看 /proc/<pid> 是否存在
	// 跨平台：检查 /proc/<pid> 或 tasklist
	if _, err := os.Stat("/proc/" + strconv.Itoa(pid)); err == nil {
		return true
	}
	// Windows 或无 /proc：尝试发送空信号
	if err := proc.Signal(os.Signal(nil)); err == nil {
		return true
	}
	return false
}
