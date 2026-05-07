package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type Server struct {
	opts     Options
	ctx      *Context
	mux      *http.ServeMux
	http     *http.Server
	listener net.Listener
}

func NewServer(opts Options, ctx *Context) *Server {
	if strings.TrimSpace(opts.ListenAddr) == "" {
		opts.ListenAddr = "127.0.0.1:8765"
	}
	if strings.TrimSpace(opts.MCPBasePath) == "" {
		opts.MCPBasePath = "/mcp"
	}
	mux := http.NewServeMux()
	s := &Server{opts: opts, ctx: ctx, mux: mux}
	s.routes()
	s.http = &http.Server{Addr: opts.ListenAddr, Handler: mux}
	return s
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.opts.ListenAddr)
	if err != nil {
		return err
	}
	s.listener = ln
	go func() {
		_ = s.http.Serve(ln)
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) BaseURL() string {
	if s == nil {
		return ""
	}
	if strings.TrimSpace(s.opts.BaseURL) != "" {
		return strings.TrimRight(s.opts.BaseURL, "/")
	}
	addr := s.opts.ListenAddr
	if s.listener != nil {
		addr = s.listener.Addr().String()
	}
	return fmt.Sprintf("http://%s", addr)
}
