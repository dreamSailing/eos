package sandbox

import (
	"errors"
	"strings"
)

type ExecFunc func(command []string, policy Policy) Result

type GuardedRunner struct {
	Backend BackendStatus
	Exec    ExecFunc
}

func NewGuardedRunner(exec ExecFunc) GuardedRunner {
	return GuardedRunner{
		Backend: DetectBackend(),
		Exec:    exec,
	}
}

func (r GuardedRunner) Run(command []string, policy Policy) Result {
	policy = policy.Normalized()
	backend := r.Backend
	if strings.TrimSpace(backend.GOOS) == "" {
		backend = DetectBackend()
	}
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return Result{ExitCode: 2, Err: errors.New("sandbox command required"), Backend: backend}
	}
	if reason := policy.CommandViolation(command); reason != "" {
		return Result{
			Stderr:   reason,
			ExitCode: 126,
			Err:      errors.New(reason),
			Backend:  backend,
		}
	}
	if r.Exec == nil {
		return Result{ExitCode: 0, Backend: backend}
	}
	result := r.Exec(command, policy)
	result.Backend = backend
	return result
}
