package server

// transport.go 提供 MCP server 的 stdio / sse transport 启动入口。
// 复用 mark3labs/mcp-go 内置的 ServeStdio / NewSSEServer。

import (
	"context"
	"errors"

	"github.com/mark3labs/mcp-go/server"
)

var errNilServer = errors.New("mcp server: nil MCPServer")

// ServeStdio 在当前进程的 stdin/stdout 上以 stdio transport 运行 MCP server。
// 阻塞直到 stdin 关闭（ServeStdio 内部处理 OS signal 退出）。
//
// ctx 用于取消时通过 WithStdioContextFunc 注入：mcp-go 的 stdio 默认自带
// signal 处理，这里额外把 ctx 作为请求 context 传播，使 ctx 取消能中断请求。
func ServeStdio(ctx context.Context, s *server.MCPServer) error {
	if s == nil {
		return errNilServer
	}
	if ctx == nil {
		ctx = context.Background()
	}
	opts := []server.StdioOption{
		server.WithStdioContextFunc(func(_ context.Context) context.Context { return ctx }),
	}
	return server.ServeStdio(s, opts...)
}

// ServeSSE 在 addr 上以 SSE transport 运行 MCP server。阻塞直到出错。
// endpoint 路径用 MCP 惯例：/sse 长连接 + /message 消息投递。
func ServeSSE(ctx context.Context, s *server.MCPServer, addr string) error {
	if s == nil {
		return errNilServer
	}
	sseServer := server.NewSSEServer(s,
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
	)
	// SSEServer.Start 阻塞监听。在独立 goroutine 监听 ctx 取消以触发退出：
	// 当前 mark3labs 版本 Start 无 ctx 参数，靠进程级 signal 退出；
	// 这里保留 ctx 引用以便后续版本接入更细粒度取消。
	go func() { <-ctx.Done() }()
	return sseServer.Start(addr)
}
