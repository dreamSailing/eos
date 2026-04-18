package bg

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusRunning TaskStatus = "running"
	StatusExited  TaskStatus = "exited"
	StatusKilled  TaskStatus = "killed"
	StatusError   TaskStatus = "error"
)

type LogEntry struct {
	Seq    int64     `json:"seq"`
	At     time.Time `json:"at"`
	Stream string    `json:"stream"`
	Line   string    `json:"line"`
}

type TaskInfo struct {
	ID         string     `json:"id"`
	Command    string     `json:"command"`
	WorkingDir string     `json:"working_dir,omitempty"`
	PID        int        `json:"pid"`
	Status     TaskStatus `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	ExitedAt   time.Time  `json:"exited_at,omitempty"`
	ExitCode   int        `json:"exit_code,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type tailResult struct {
	Entries []LogEntry `json:"entries"`
	NextSeq int64      `json:"next_seq"`
	Done    bool       `json:"done"`
	Info    TaskInfo   `json:"info"`
}

type task struct {
	mu sync.Mutex

	info TaskInfo

	cmd    *exec.Cmd
	cancel context.CancelFunc
	doneCh chan struct{}

	nextSeq int64
	logs    []LogEntry
	logCap  int
}

func (t *task) appendLog(stream string, line string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	line = strings.TrimRight(line, "\r\n")
	t.logs = append(t.logs, LogEntry{
		Seq:    t.nextSeq,
		At:     time.Now(),
		Stream: stream,
		Line:   line,
	})
	t.nextSeq++
	if t.logCap > 0 && len(t.logs) > t.logCap {
		drop := len(t.logs) - t.logCap
		t.logs = append([]LogEntry(nil), t.logs[drop:]...)
	}
}

func (t *task) done() bool {
	select {
	case <-t.doneCh:
		return true
	default:
		return false
	}
}

type StartOptions struct {
	WorkingDir string
	Env        []string
	LogCap     int
}

type TailOptions struct {
	FromSeq int64
	Limit   int
}

type Manager struct {
	mu    sync.Mutex
	tasks map[string]*task
}

var (
	defaultOnce sync.Once
	defaultMgr  *Manager
)

func Default() *Manager {
	defaultOnce.Do(func() {
		defaultMgr = NewManager()
	})
	return defaultMgr
}

func NewManager() *Manager {
	return &Manager{tasks: map[string]*task{}}
}

func (m *Manager) Start(command string, opts *StartOptions) (TaskInfo, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return TaskInfo{}, fmt.Errorf("command required")
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())

	execName, execArgs := shellCommand(command)
	cmd := exec.CommandContext(ctx, execName, execArgs...)
	if opts != nil {
		if strings.TrimSpace(opts.WorkingDir) != "" {
			cmd.Dir = opts.WorkingDir
		}
		if len(opts.Env) > 0 {
			cmd.Env = append(os.Environ(), opts.Env...)
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return TaskInfo{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return TaskInfo{}, err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return TaskInfo{}, err
	}

	ti := TaskInfo{
		ID:         id,
		Command:    command,
		WorkingDir: cmd.Dir,
		PID:        cmd.Process.Pid,
		Status:     StatusRunning,
		StartedAt:  time.Now(),
	}

	logCap := 2000
	if opts != nil && opts.LogCap > 0 {
		logCap = opts.LogCap
	}
	t := &task{
		info:   ti,
		cmd:    cmd,
		cancel: cancel,
		doneCh: make(chan struct{}),
		logCap: logCap,
	}

	m.mu.Lock()
	m.tasks[id] = t
	m.mu.Unlock()

	go t.consumeStream(stdout, "stdout")
	go t.consumeStream(stderr, "stderr")
	go t.wait()

	return ti, nil
}

func (t *task) consumeStream(rdr io.Reader, stream string) {
	sc := bufio.NewScanner(rdr)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		t.appendLog(stream, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.appendLog(stream, "scanner error: "+err.Error())
	}
}

func exitCodeFromError(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), true
	}
	return 0, false
}

func (t *task) wait() {
	err := t.cmd.Wait()

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.info.Status == StatusKilled {
		t.info.ExitedAt = time.Now()
		close(t.doneCh)
		return
	}

	t.info.ExitedAt = time.Now()
	if code, ok := exitCodeFromError(err); ok {
		t.info.ExitCode = code
	}
	if err != nil {
		t.info.Status = StatusError
		t.info.Error = err.Error()
	} else {
		t.info.Status = StatusExited
	}
	close(t.doneCh)
}

func (m *Manager) Kill(id string) (TaskInfo, error) {
	t := m.get(id)
	if t == nil {
		return TaskInfo{}, fmt.Errorf("task not found")
	}

	t.mu.Lock()
	if t.info.Status != StatusRunning {
		inf := t.info
		t.mu.Unlock()
		return inf, nil
	}
	t.info.Status = StatusKilled
	inf := t.info
	cancel := t.cancel
	proc := t.cmd.Process
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if proc != nil {
		_ = proc.Kill()
	}

	select {
	case <-t.doneCh:
	case <-time.After(3 * time.Second):
	}
	return inf, nil
}

func (m *Manager) CleanupFinished() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, t := range m.tasks {
		if t.done() {
			delete(m.tasks, id)
			n++
		}
	}
	return n
}

func (m *Manager) List() []TaskInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TaskInfo, 0, len(m.tasks))
	for _, t := range m.tasks {
		t.mu.Lock()
		out = append(out, t.info)
		t.mu.Unlock()
	}
	sortTasks(out)
	return out
}

func sortTasks(items []TaskInfo) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].StartedAt.After(items[i].StartedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func (m *Manager) Info(id string) (TaskInfo, error) {
	t := m.get(id)
	if t == nil {
		return TaskInfo{}, fmt.Errorf("task not found")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.info, nil
}

func (m *Manager) Tail(id string, opts *TailOptions) (tailResult, error) {
	t := m.get(id)
	if t == nil {
		return tailResult{}, fmt.Errorf("task not found")
	}

	from := int64(0)
	limit := 200
	if opts != nil {
		if opts.FromSeq > 0 {
			from = opts.FromSeq
		}
		if opts.Limit > 0 {
			limit = opts.Limit
		}
	}
	if limit > 2000 {
		limit = 2000
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	var out []LogEntry
	for _, e := range t.logs {
		if e.Seq < from {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	nextSeq := t.nextSeq
	if len(out) > 0 {
		nextSeq = out[len(out)-1].Seq + 1
	}

	return tailResult{
		Entries: out,
		NextSeq: nextSeq,
		Done:    t.done(),
		Info:    t.info,
	}, nil
}

func (m *Manager) get(id string) *task {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tasks[id]
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-NonInteractive", "-Command", command}
	}
	return "bash", []string{"-lc", command}
}
