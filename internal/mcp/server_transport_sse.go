package mcp

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

func (s *Server) RunSSE(ctx context.Context) error {
	if s == nil || s.mcp == nil {
		return fmt.Errorf("server not initialized")
	}
	addr := strings.TrimSpace(s.opts.ListenAddr)
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	baseURL := strings.TrimSpace(s.opts.BaseURL)
	if baseURL == "" {
		baseURL = "http://" + addr
	}
	httpServer := &http.Server{Addr: addr}
	sse := mcpserver.NewSSEServer(
		s.mcp,
		mcpserver.WithBaseURL(baseURL),
		mcpserver.WithHTTPServer(httpServer),
		mcpserver.WithKeepAlive(true),
		mcpserver.WithKeepAliveInterval(10*time.Second),
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- sse.Start(addr)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sse.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
