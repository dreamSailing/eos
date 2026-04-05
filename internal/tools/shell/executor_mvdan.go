package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

type MvdanExecutor struct {
	parser *syntax.Parser
}

func NewMvdanExecutor() Executor {
	return &MvdanExecutor{
		parser: syntax.NewParser(),
	}
}

func (e *MvdanExecutor) Execute(ctx context.Context, command string, workingDir string) (stdout, stderr string, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	runner, err := e.createRunner(ctx, &stdoutBuf, &stderrBuf, workingDir)
	if err != nil {
		slog.Error("shell.mvdan.create_runner.error", "component", utils.ComponentTool,
			"command", command,
			"working_dir", workingDir,
			"error", err.Error(),
		)
		return "", "", err
	}

	file, err := e.parser.Parse(strings.NewReader(command), "")
	if err != nil {
		slog.Error("shell.mvdan.parse.error", "component", utils.ComponentTool,
			"command", command,
			"error", err.Error(),
		)
		return "", "", fmt.Errorf("parse error: %w", err)
	}

	err = runner.Run(ctx, file)
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if err != nil {
		if exitStatus, ok := interp.IsExitStatus(err); ok {
			slog.Debug("shell.mvdan.execute.exit_status", "component", utils.ComponentTool,
				"command", command,
				"working_dir", workingDir,
				"exit_status", exitStatus,
				"stdout_length", len(stdoutStr),
				"stderr_length", len(stderrStr),
			)
			return stdoutStr, stderrStr, fmt.Errorf("exit status %d: %s", exitStatus, stderrStr)
		}

		slog.Error("shell.mvdan.execute.error", "component", utils.ComponentTool,
			"command", command,
			"working_dir", workingDir,
			"error", err.Error(),
			"stdout_length", len(stdoutStr),
			"stderr_length", len(stderrStr),
		)
		return stdoutStr, stderrStr, err
	}

	slog.Debug("shell.mvdan.execute.success", "component", utils.ComponentTool,
		"command", command,
		"working_dir", workingDir,
		"stdout_length", len(stdoutStr),
		"stderr_length", len(stderrStr),
	)

	return stdoutStr, stderrStr, nil
}

func (e *MvdanExecutor) createRunner(ctx context.Context, stdout, stderr io.Writer, workingDir string) (*interp.Runner, error) {
	opts := []interp.RunnerOption{
		interp.StdIO(nil, stdout, stderr),
		interp.Env(expand.ListEnviron(envFromContext(ctx)...)),
		interp.ExecHandlers(e.execHandler()),
		interp.OpenHandler(e.openHandler()),
	}

	if workingDir != "" {
		opts = append(opts, interp.Dir(workingDir))
	}

	return interp.New(opts...)
}

func (e *MvdanExecutor) execHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			switch args[0] {
			case "sudo":
				if len(args) > 1 {
					return fmt.Errorf("sudo is not supported, please run the command without sudo or with appropriate permissions")
				}
			case "apt", "apt-get", "yum", "dnf", "pacman", "brew", "choco", "winget":
				slog.Warn("shell.mvdan.package_manager", "component", utils.ComponentTool,
					"command", args[0],
					"hint", "package manager commands may require system shell",
				)
			}

			return interp.DefaultExecHandler(2*60*1e9)(ctx, args)
		}
	}
}

func (e *MvdanExecutor) openHandler() interp.OpenHandlerFunc {
	return interp.DefaultOpenHandler()
}

func (e *MvdanExecutor) ExecuteDirect(ctx context.Context, name string, args []string, opts *ExecuteOptions) (stdout, stderr string, err error) {
	var cmd string
	if len(args) > 0 {
		cmd = name + " " + strings.Join(args, " ")
	} else {
		cmd = name
	}

	workingDir := ""
	if opts != nil {
		workingDir = opts.WorkingDir
	}

	return e.Execute(ctx, cmd, workingDir)
}
