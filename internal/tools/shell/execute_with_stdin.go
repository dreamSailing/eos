package shell

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"

	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func ExecuteWithStdin(ctx context.Context, command string, workingDir string, stdin string) (stdout, stderr string, exitCode int, err error) {
	return ExecuteWithStdinEnv(ctx, command, workingDir, stdin, nil)
}

func ExecuteWithStdinEnv(ctx context.Context, command string, workingDir string, stdin string, env []string) (stdout, stderr string, exitCode int, err error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		psCmd := "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; $ErrorActionPreference='SilentlyContinue'; " + command
		cmd = exec.CommandContext(ctx, "powershell", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	if workingDir != "" {
		cmd.Dir = workingDir
	}
	if len(env) > 0 {
		cmd.Env = env
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()
	exitCode = 0

	if err != nil {
		exitCode = 1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		}
		slog.Debug("shell.execute_with_stdin.error", "component", utils.ComponentTool,
			"command", command,
			"working_dir", workingDir,
			"os", runtime.GOOS,
			"error", err.Error(),
			"stdout_length", len(stdoutStr),
			"stderr_length", len(stderrStr),
		)
		return stdoutStr, stderrStr, exitCode, err
	}
	if st := cmd.ProcessState; st != nil {
		exitCode = st.ExitCode()
	}
	if exitCode < 0 {
		exitCode = 0
	}
	return stdoutStr, stderrStr, exitCode, nil
}
