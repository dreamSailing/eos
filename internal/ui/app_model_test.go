package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/internal/ui/views/setup"

	tea "github.com/charmbracelet/bubbletea"
)

var ansiPatternAppTest = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSIAppTest(s string) string {
	return ansiPatternAppTest.ReplaceAllString(s, "")
}

func setTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	volume := filepath.VolumeName(home)
	if volume != "" {
		t.Setenv("HOMEDRIVE", volume)
		t.Setenv("HOMEPATH", strings.TrimPrefix(home, volume))
	}
	return home
}

func newTestAppModel(t *testing.T) *AppModel {
	t.Helper()
	core := bridge.NewRuntimeCore(session.NewContextManager(), tools.NewManager(), nil)
	t.Cleanup(core.Shutdown)
	return NewAppModel(core)
}

func sendAppKey(t *testing.T, app *AppModel, msg tea.KeyMsg) *AppModel {
	t.Helper()
	next, _ := app.Update(msg)
	updated, ok := next.(*AppModel)
	if !ok {
		t.Fatalf("expected *AppModel, got %T", next)
	}
	return updated
}

func TestInitialSetupKeepsWelcomeAfterFirstModelAdded(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "")
	t.Setenv("EOS_API_KEY", "")
	t.Setenv("EOS_MODEL", "")

	app := newTestAppModel(t)
	if app.activeView != "setup" {
		t.Fatalf("expected setup view on first launch, got %q", app.activeView)
	}
	if !app.initialSetupFlow {
		t.Fatalf("expected initial setup flow to be enabled")
	}

	app.handleModelFormComplete(setup.ModelFormCompleteMsg{
		Config: setup.SetupConfig{
			Name:    "first-model",
			APIBase: "https://example.com/v1",
			APIKey:  "secret",
			Model:   "demo-model",
		},
	})

	if app.activeView != "shell" {
		t.Fatalf("expected shell view after setup, got %q", app.activeView)
	}
	if len(app.history) != 0 {
		t.Fatalf("expected initial setup success to keep history empty, got %d entries", len(app.history))
	}

	view := stripANSIAppTest(app.shell.View())
	if strings.Contains(view, "Added and switched to model: first-model") {
		t.Fatalf("expected welcome screen without success history message, got %q", view)
	}
	if !strings.Contains(view, "AI Powered Development Assistant") {
		t.Fatalf("expected welcome card to remain visible, got %q", view)
	}
	if !strings.Contains(view, "demo-model") {
		t.Fatalf("expected welcome card to show new model info, got %q", view)
	}
	if !strings.Contains(view, "https://example.com/v1") {
		t.Fatalf("expected welcome card to show updated API info, got %q", view)
	}
}

func TestSlashHintsNavigationKeepsPanelAndInsertsCanonicalCommand(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "https://example.com/v1")
	t.Setenv("EOS_API_KEY", "secret")
	t.Setenv("EOS_MODEL", "demo-model")

	testCases := []struct {
		name      string
		downs     int
		acceptKey tea.KeyMsg
		wantInput string
	}{
		{
			name:      "enter accepts canonical command",
			downs:     4,
			acceptKey: tea.KeyMsg{Type: tea.KeyEnter},
			wantInput: "/lang ",
		},
		{
			name:      "tab accepts canonical command",
			downs:     7,
			acceptKey: tea.KeyMsg{Type: tea.KeyTab},
			wantInput: "/workspace ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppModel(t)
			if app.activeView != "shell" {
				t.Fatalf("expected shell view, got %q", app.activeView)
			}

			app = sendAppKey(t, app, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
			if got := app.shell.GetInputValue(); got != "/" {
				t.Fatalf("expected input to contain slash trigger, got %q", got)
			}
			if !app.shell.IsHintsVisible() {
				t.Fatalf("expected slash hints to be visible after typing slash")
			}

			for i := 0; i < tc.downs; i++ {
				app = sendAppKey(t, app, tea.KeyMsg{Type: tea.KeyDown})
				if !app.shell.IsHintsVisible() {
					t.Fatalf("expected slash hints to remain visible after down navigation %d", i+1)
				}
			}

			app = sendAppKey(t, app, tc.acceptKey)
			if app.shell.IsHintsVisible() {
				t.Fatalf("expected slash hints to hide after accepting a command")
			}
			if got := app.shell.GetInputValue(); got != tc.wantInput {
				t.Fatalf("expected accepted command %q, got %q", tc.wantInput, got)
			}
		})
	}
}

func TestHandlePlanStyleSlashShowsCurrentAndSavesWorkspaceSetting(t *testing.T) {
	setTestHome(t)
	workspace := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("chdir workspace: %v", err)
	}
	app := newTestAppModel(t)

	app.handlePlanStyleSlash(nil)
	if len(app.history) == 0 || !strings.Contains(app.history[len(app.history)-1].content, "concise") {
		t.Fatalf("expected current plan style message to mention concise, history=%+v", app.history)
	}

	app.handlePlanStyleSlash([]string{"detailed"})
	if got := app.adapter.GetCore().GetSettings().PlanPromptStyle; got != "detailed" {
		t.Fatalf("runtime PlanPromptStyle=%q, want detailed", got)
	}

	raw, err := os.ReadFile(filepath.Join(workspace, ".eos", "settings.json"))
	if err != nil {
		t.Fatalf("read workspace settings: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal workspace settings: %v", err)
	}
	if got := doc["plan_prompt_style"]; got != "detailed" {
		t.Fatalf("plan_prompt_style=%v, want detailed", got)
	}
}
