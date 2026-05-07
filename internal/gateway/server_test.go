package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	eosmcp "github.com/dreamSailing/eos/internal/mcp"
	"github.com/dreamSailing/eos/internal/scheduler"
	toolapiimpl "github.com/dreamSailing/eos/internal/toolapi/impl"
)

func TestGatewayHealthAndStatus(t *testing.T) {
	services := toolapiimpl.NewServices()
	mcpServer, err := eosmcp.NewServer(eosmcp.ServerOptions{
		Transport:            "sse",
		DefaultWorkspacePath: t.TempDir(),
		DefaultSandboxMode:   "workspace",
		ListenAddr:           "127.0.0.1:0",
	}, services)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	schedulerSvc := scheduler.NewService(filepath.Join(t.TempDir(), "schedules.json"), services, t.TempDir())
	if err := schedulerSvc.Start(context.Background()); err != nil {
		t.Fatalf("scheduler.Start() error = %v", err)
	}
	defer schedulerSvc.Close()
	server := NewServer(Options{ListenAddr: "127.0.0.1:0", MCPBasePath: "/mcp"}, &Context{
		MCP:       mcpServer,
		Scheduler: schedulerSvc,
		Services:  services,
		StatusSnapshot: func() map[string]any {
			return map[string]any{"ok": true}
		},
	})
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Shutdown(context.Background())

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(server.BaseURL() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d", resp.StatusCode)
	}

	resp, err = client.Get(server.BaseURL() + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status error = %v", err)
	}
	defer resp.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode status payload: %v", err)
	}
	if payload["ok"] != true {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
