package gateway

import (
	"github.com/dreamSailing/eos/internal/mcp"
	"github.com/dreamSailing/eos/internal/scheduler"
	"github.com/dreamSailing/eos/internal/toolapi"
)

type Options struct {
	ListenAddr string
	BaseURL    string
	MCPBasePath string
}

type Context struct {
	MCP           *mcp.Server
	Scheduler     *scheduler.Service
	Services      toolapi.Services
	StatusSnapshot func() map[string]any
}
