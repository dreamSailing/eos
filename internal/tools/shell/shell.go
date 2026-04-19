package shell

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"bytes"
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"log/slog"
	"sync"
	"time"
)

type asyncSession struct {
	mu       sync.Mutex
	id       string
	cancel   context.CancelFunc
	out      bytes.Buffer
	err      bytes.Buffer
	done     chan struct{}
	executor Executor
}

type Shell struct {
	mu       sync.Mutex
	sessions map[string]*asyncSession
	executor Executor
}

func NewShell() *Shell {
	return &Shell{
		sessions: map[string]*asyncSession{},
		executor: GetDefaultExecutor(),
	}
}

func NewShellWithExecutor(executor Executor) *Shell {
	return &Shell{
		sessions: map[string]*asyncSession{},
		executor: executor,
	}
}

func (s *Shell) SetExecutor(executor Executor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executor = executor
}

func (s *Shell) ExecuteCtx(ctx context.Context, command string) (string, error) {
	stdout, stderr, err := s.executor.Execute(ctx, command, "")
	if err != nil {
		if stderr != "" {
			return stdout, fmt.Errorf("%v: %s", err, stderr)
		}
		return stdout, err
	}
	return stdout, nil
}

func (s *Shell) ExecuteTypedCtx(ctx context.Context, shellType ShellType, command string) (string, error) {
	return s.ExecuteTypedWithWorkingDirCtx(ctx, shellType, command, "")
}

func (s *Shell) StartAsyncWithWorkingDir(command, workingDir string) (string, error) {
	sid := fmt.Sprintf("%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	ctx = withPluginEnv(ctx, workingDir)

	sess := &asyncSession{
		id:       sid,
		cancel:   cancel,
		done:     make(chan struct{}, 1),
		executor: s.executor,
	}

	s.mu.Lock()
	s.sessions[sid] = sess
	s.mu.Unlock()

	go func() {
		defer close(sess.done)
		stdout, stderr, err := sess.executor.Execute(ctx, command, workingDir)

		sess.mu.Lock()
		defer sess.mu.Unlock()

		if stdout != "" {
			sess.out.WriteString(stdout)
		}
		if stderr != "" {
			sess.err.WriteString(stderr)
		}
		if err != nil {
			sess.err.WriteString(err.Error())
		}
	}()

	slog.Debug("shell.start_async.success", "component", utils.ComponentTool,
		"command", command,
		"working_dir", workingDir,
		"session_id", sid,
	)

	return sid, nil
}

func (s *Shell) ExecuteWithWorkingDir(command, workingDir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.ExecuteWithWorkingDirCtx(ctx, command, workingDir)
}

func (s *Shell) ExecuteWithWorkingDirCtx(ctx context.Context, command, workingDir string) (string, error) {
	ctx = withPluginEnv(ctx, workingDir)
	stdout, stderr, err := s.executor.Execute(ctx, command, workingDir)
	if err != nil {
		if stderr != "" {
			return stdout, fmt.Errorf("%v: %s", err, stderr)
		}
		return stdout, err
	}
	return stdout, nil
}

func (s *Shell) ExecuteTypedWithWorkingDirCtx(ctx context.Context, shellType ShellType, command, workingDir string) (string, error) {
	ctx = withPluginEnv(ctx, workingDir)
	stdout, stderr, _, err := executeNativeShellCommand(ctx, shellType, command, workingDir, "", nil)
	if err != nil {
		if stderr != "" {
			return stdout, fmt.Errorf("%v: %s", err, stderr)
		}
		return stdout, err
	}
	return stdout, nil
}

func (s *Shell) StartAsync(command string) (string, error) {
	return s.StartAsyncWithWorkingDir(command, "")
}

func (s *Shell) Output(id string) (string, string, bool, error) {
	s.mu.Lock()
	sess := s.sessions[id]
	s.mu.Unlock()

	if sess == nil {
		return "", "", true, fmt.Errorf("session not found")
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	done := false
	select {
	case <-sess.done:
		done = true
	default:
	}

	return sess.out.String(), sess.err.String(), done, nil
}

func (s *Shell) Kill(id string) error {
	s.mu.Lock()
	sess := s.sessions[id]
	s.mu.Unlock()

	if sess == nil {
		slog.Error("shell.kill.session_not_found", "component", utils.ComponentTool,
			"session_id", id,
		)
		return fmt.Errorf("session not found")
	}

	select {
	case <-sess.done:
		s.Remove(id)
		return nil
	default:
	}

	sess.cancel()

	select {
	case <-sess.done:
	case <-time.After(5 * time.Second):
	}

	slog.Info("shell.kill.success", "component", utils.ComponentTool,
		"session_id", id,
	)

	s.Remove(id)
	return nil
}

func (s *Shell) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[id]; ok {
		select {
		case <-sess.done:
		default:
			sess.cancel()
		}
		delete(s.sessions, id)
		slog.Debug("shell.session.removed", "component", utils.ComponentTool,
			"session_id", id,
		)
	}
}

func (s *Shell) CleanupFinished() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, sess := range s.sessions {
		select {
		case <-sess.done:
			delete(s.sessions, id)
			count++
		default:
		}
	}

	if count > 0 {
		slog.Debug("shell.cleanup_finished", "component", utils.ComponentTool,
			"cleaned_count", count,
			"remaining_count", len(s.sessions),
		)
	}
	return count
}

func (s *Shell) CleanupAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.sessions)
	for id, sess := range s.sessions {
		sess.cancel()
		select {
		case <-sess.done:
		default:
			close(sess.done)
		}
		delete(s.sessions, id)
	}

	if count > 0 {
		slog.Debug("shell.cleanup_all", "component", utils.ComponentTool,
			"cleaned_count", count,
		)
	}
}
