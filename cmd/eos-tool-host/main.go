//go:build legacy

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dreamSailing/eos/pkg/coreapi/sidecar/toolhost"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	host, err := resolveHost()
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolhost: init failed: %v\n", err)
		os.Exit(1)
	}

	srv := toolhost.NewServer(host)
	if err := srv.Serve(ctx, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "toolhost: %v\n", err)
		os.Exit(1)
	}
}

func resolveHost() (toolhost.ToolHost, error) {
	if os.Getenv("EOS_TOOL_HOST_FAKE") == "1" {
		fmt.Fprintf(os.Stderr, "toolhost: using FakeHost (EOS_TOOL_HOST_FAKE=1)\n")
		return &toolhost.FakeHost{}, nil
	}

	workspaceRoot, _ := os.Getwd()
	if v := os.Getenv("EOS_WORKSPACE_ROOT"); v != "" {
		workspaceRoot = v
	}

	runner, err := newManagerRunner(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("initialize tool runner: %w", err)
	}

	fmt.Fprintf(os.Stderr, "toolhost: using LegacyHost (workspace=%s)\n", workspaceRoot)
	return &toolhost.LegacyHost{Runner: runner}, nil
}
