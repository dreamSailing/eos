package config

import "testing"

func TestRememberWorkspaceKeepsDefaultAndUpdatesLast(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	var cfg Config
	if !RememberWorkspace(&cfg, "C:/repo-b", true) {
		t.Fatal("expected remember workspace to report change")
	}
	if len(cfg.KnownWorkspaces) != 2 {
		t.Fatalf("KnownWorkspaces=%v, want default + repo", cfg.KnownWorkspaces)
	}
	if got, want := NormalizeWorkspacePath(cfg.LastWorkspace), NormalizeWorkspacePath("C:/repo-b"); got != want {
		t.Fatalf("LastWorkspace=%q, want %q", got, want)
	}
	if got, want := NormalizeWorkspacePath(cfg.KnownWorkspaces[0]), NormalizeWorkspacePath(DefaultWorkspacePath()); got != want {
		t.Fatalf("KnownWorkspaces[0]=%q, want %q", got, want)
	}
}

func TestForgetWorkspacePreservesDefaultFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	cfg := Config{}
	RememberWorkspace(&cfg, "C:/repo-a", false)
	RememberWorkspace(&cfg, "C:/repo-b", true)

	if !ForgetWorkspace(&cfg, "C:/repo-b") {
		t.Fatal("expected forget workspace to report change")
	}
	if got, want := NormalizeWorkspacePath(cfg.LastWorkspace), NormalizeWorkspacePath(DefaultWorkspacePath()); got != want {
		t.Fatalf("LastWorkspace=%q, want %q", got, want)
	}
	if len(cfg.KnownWorkspaces) != 2 {
		t.Fatalf("KnownWorkspaces=%v, want default + repo-a", cfg.KnownWorkspaces)
	}
}
