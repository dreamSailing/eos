package mcp

import (
	"testing"

	"github.com/dreamSailing/eos/internal/config"
)

func TestRecommendedBrowserPreset(t *testing.T) {
	entry := RecommendedBrowserPreset()
	if entry.Name != DefaultBrowserServerName {
		t.Fatalf("name = %q, want %q", entry.Name, DefaultBrowserServerName)
	}
	if entry.Type != config.MCPTypeStdio {
		t.Fatalf("type = %q, want %q", entry.Type, config.MCPTypeStdio)
	}
	if entry.Command != "npx" {
		t.Fatalf("command = %q, want npx", entry.Command)
	}
	if len(entry.Args) < 2 || entry.Args[1] != "@playwright/mcp@latest" {
		t.Fatalf("unexpected args: %#v", entry.Args)
	}
	if !entry.Enabled {
		t.Fatal("preset should be enabled")
	}
}

func TestDetectBrowserStatusWithManager(t *testing.T) {
	cfg := &config.Config{
		MCP: []config.MCPEntry{
			RecommendedBrowserPreset(),
		},
	}
	mgr := &Manager{
		status: map[string]ServerStatus{
			DefaultBrowserServerName: {
				Name:    DefaultBrowserServerName,
				Enabled: true,
				Loaded:  true,
				Tools:   7,
			},
		},
	}

	status := DetectBrowserStatus(cfg, mgr)
	if !status.Configured || !status.Enabled || !status.Loaded {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.Tools != 7 {
		t.Fatalf("tools = %d, want 7", status.Tools)
	}
}

func TestDetectBrowserStatusMissing(t *testing.T) {
	status := DetectBrowserStatus(&config.Config{}, nil)
	if status.Configured {
		t.Fatalf("expected unconfigured status, got %+v", status)
	}
	if status.ServerName != DefaultBrowserServerName {
		t.Fatalf("server name = %q, want %q", status.ServerName, DefaultBrowserServerName)
	}
}
