package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Manager struct {
	opts Options
}

func NewManager(opts Options) *Manager {
	return &Manager{opts: opts}
}

func (m *Manager) StartBackground() (State, error) {
	opts, err := normalizeOptions(m.opts)
	if err != nil {
		return State{}, err
	}
	if state, ok, _ := Status(opts.StateFile); ok {
		return state, fmt.Errorf("daemon already running")
	}
	if err := os.MkdirAll(DefaultDir(), 0o755); err != nil {
		return State{}, err
	}
	logFile, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return State{}, err
	}
	defer logFile.Close()
	exe, err := os.Executable()
	if err != nil {
		return State{}, err
	}
	args := []string{"daemon", "run", "--workspace", opts.Workspace, "--listen", opts.ListenAddr, "--state-file", opts.StateFile, "--schedule-file", opts.ScheduleFile, "--session-store", opts.SessionStorePath, "--sandbox-mode", opts.SandboxMode, "--mcp-base-path", opts.MCPBasePath, "--log-file", opts.LogFile}
	if strings.TrimSpace(opts.PolicyPath) != "" {
		args = append(args, "--policy", opts.PolicyPath)
	}
	if strings.TrimSpace(opts.BaseURL) != "" {
		args = append(args, "--base-url", opts.BaseURL)
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(opts.AllowedTools, ","))
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return State{}, err
	}
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		state, err := LoadState(opts.StateFile)
		if err == nil && state.PID > 0 {
			if healthy(state.WebBaseURL) {
				return state, nil
			}
		}
	}
	return State{}, fmt.Errorf("daemon did not become ready")
}

func (m *Manager) RunForeground(ctx context.Context) error {
	svc, err := NewService(m.opts)
	if err != nil {
		return err
	}
	return svc.Start(ctx)
}

func Status(stateFile string) (State, bool, error) {
	state, err := LoadState(stateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, false, nil
		}
		return State{}, false, err
	}
	if state.PID <= 0 || !processAlive(state.PID) {
		return state, false, nil
	}
	return state, true, nil
}

func Stop(stateFile string) error {
	state, ok, err := Status(stateFile)
	if err != nil {
		return err
	}
	if !ok {
		_ = RemoveState(stateFile)
		return fmt.Errorf("daemon not running")
	}
	proc, err := os.FindProcess(state.PID)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !processAlive(state.PID) {
			_ = RemoveState(stateFile)
			return nil
		}
	}
	return fmt.Errorf("daemon stop timeout")
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func healthy(baseURL string) bool {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
