package mcp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

func (s *Server) RunStdio(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if s == nil || s.mcp == nil {
		return fmt.Errorf("server not initialized")
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	stdio := mcpserver.NewStdioServer(s.mcp)
	stdio.SetErrorLogger(log.New(stderr, "", log.LstdFlags))
	return stdio.Listen(ctx, stdin, stdout)
}
