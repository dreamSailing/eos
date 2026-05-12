package ui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dreamSailing/eos/internal/bridge"
	"github.com/dreamSailing/eos/internal/pkg/filedialog"
	"github.com/dreamSailing/eos/internal/pkg/settings"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/state"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/internal/ui/features/slash"
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
		command   string
		acceptKey tea.KeyMsg
		wantInput string
	}{
		{
			name:      "enter accepts canonical command",
			command:   "/lang",
			acceptKey: tea.KeyMsg{Type: tea.KeyEnter},
			wantInput: "/lang ",
		},
		{
			name:      "tab accepts canonical command",
			command:   "/workspace",
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

			downs := findVisibleSlashCommandIndex(t, tc.command)
			for i := 0; i < downs; i++ {
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

func findVisibleSlashCommandIndex(t *testing.T, name string) int {
	t.Helper()
	items := slash.VisibleCommands()
	for idx, item := range items {
		if item.Name == name {
			return idx
		}
	}
	t.Fatalf("visible slash command %q not found", name)
	return -1
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

func TestHandlePermissionsSlashSupportsAccessAndApprovalModes(t *testing.T) {
	setTestHome(t)
	app := newTestAppModel(t)

	app.handlePermissionsSlash([]string{"access", "read-only"})
	snap := app.adapter.GetCore().PermissionSnapshot()
	if got := snap.AccessMode; got != "read-only" {
		t.Fatalf("accessMode=%q, want read-only", got)
	}
	if got := snap.SandboxMode; got != "workspace" {
		t.Fatalf("sandboxMode=%q, want workspace", got)
	}

	app.handlePermissionsSlash([]string{"approval", "never"})
	snap = app.adapter.GetCore().PermissionSnapshot()
	if got := snap.ApprovalMode; got != "never" {
		t.Fatalf("approvalMode=%q, want never", got)
	}

	last := app.history[len(app.history)-1].content
	if !strings.Contains(last, "审批模式") || !strings.Contains(last, "never") {
		t.Fatalf("expected permissions output to include approval mode, got %q", last)
	}
}

func TestPromptRequestPermissionStaysInShellWithInlineOverlay(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "https://example.com/v1")
	t.Setenv("EOS_API_KEY", "secret")
	t.Setenv("EOS_MODEL", "demo-model")

	app := newTestAppModel(t)
	next, _ := app.Update(PromptRequestMsg{
		ID:       "perm-1",
		Kind:     "permission",
		Title:    "授权",
		Question: "是否允许执行危险操作？",
		Options:  []string{"allow_once", "allow_session", "deny"},
	})
	updated := next.(*AppModel)

	if updated.activeView != "shell" {
		t.Fatalf("activeView=%q, want shell", updated.activeView)
	}
	if updated.confirmView != nil {
		t.Fatalf("expected permission request to avoid full confirm view")
	}
	if updated.inlinePermissionReq == nil {
		t.Fatalf("expected inline permission state to be stored")
	}
	view := stripANSIAppTest(updated.shell.View())
	if !strings.Contains(view, "是否允许执行危险操作？") || !strings.Contains(view, "仅本次操作，继续执行") {
		t.Fatalf("expected shell to render inline permission overlay, got %q", view)
	}
}

func TestPromptRequestNonPermissionStillUsesConfirmView(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "https://example.com/v1")
	t.Setenv("EOS_API_KEY", "secret")
	t.Setenv("EOS_MODEL", "demo-model")

	app := newTestAppModel(t)
	next, _ := app.Update(PromptRequestMsg{
		ID:       "ws-1",
		Kind:     "workspace_trust",
		Title:    "信任工作区",
		Question: "是否信任当前工作区？",
		Options:  []string{"信任并继续", "退出"},
	})
	updated := next.(*AppModel)

	if updated.activeView != "confirm" {
		t.Fatalf("activeView=%q, want confirm", updated.activeView)
	}
	if updated.confirmView == nil {
		t.Fatalf("expected non-permission prompt to use confirm view")
	}
	if updated.inlinePermissionReq != nil {
		t.Fatalf("did not expect inline permission state for non-permission prompt")
	}
}

func TestHandleStatusSlashShowsAccessAndApprovalModes(t *testing.T) {
	setTestHome(t)
	app := newTestAppModel(t)
	core := app.adapter.GetCore()
	core.SetAccessMode("danger-full-access")
	core.SetApprovalMode("never")

	app.handleStatusSlash()
	last := app.history[len(app.history)-1].content
	for _, part := range []string{"访问模式", "danger-full-access", "审批模式", "never", "沙箱模式", "full_access"} {
		if !strings.Contains(last, part) {
			t.Fatalf("expected status output to contain %q, got %q", part, last)
		}
	}
}

func TestPredictionUpdateMsgShowsPredictionForMatchingDraft(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "https://example.com/v1")
	t.Setenv("EOS_API_KEY", "secret")
	t.Setenv("EOS_MODEL", "demo-model")

	app := newTestAppModel(t)
	app.predictionEnabled = true
	app.predictionSeq = 1
	app.shell.SetInputValue("我")

	next, _ := app.Update(PredictionUpdateMsg{Seq: 1, Draft: "我", Text: "我们继续拆一下这个方案"})
	updated := next.(*AppModel)
	if !updated.shell.HasPrediction() {
		t.Fatalf("expected prediction to be visible")
	}
	if got := updated.shell.GetInputValue(); got != "我" {
		t.Fatalf("input=%q, want 我", got)
	}

	updated = sendAppKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'们'}})
	if !updated.shell.HasPrediction() {
		t.Fatalf("expected prediction to remain when typed input still matches prefix")
	}
	if got := updated.shell.GetInputValue(); got != "我们" {
		t.Fatalf("input=%q, want 我们", got)
	}
}

func TestPredictionUpdateMsgIgnoresStaleDraft(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "https://example.com/v1")
	t.Setenv("EOS_API_KEY", "secret")
	t.Setenv("EOS_MODEL", "demo-model")

	app := newTestAppModel(t)
	app.predictionEnabled = true
	app.predictionSeq = 1
	app.shell.SetInputValue("我")

	next, _ := app.Update(PredictionUpdateMsg{Seq: 1, Draft: "他", Text: "他们继续拆一下这个方案"})
	updated := next.(*AppModel)
	if updated.shell.HasPrediction() {
		t.Fatalf("expected stale draft prediction to be ignored")
	}
	if got := updated.shell.GetInputValue(); got != "我" {
		t.Fatalf("input=%q, want 我", got)
	}
}

func TestSchedulePredictionCreatesDebouncedRequest(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "https://example.com/v1")
	t.Setenv("EOS_API_KEY", "secret")
	t.Setenv("EOS_MODEL", "demo-model")

	app := newTestAppModel(t)
	app.predictionEnabled = true

	cmd := app.schedulePrediction("我想")
	if cmd == nil {
		t.Fatalf("expected debounced prediction cmd")
	}
	msg := cmd()
	debounce, ok := msg.(predictionDebounceMsg)
	if !ok {
		t.Fatalf("msg=%T, want predictionDebounceMsg", msg)
	}
	if debounce.Draft != "我想" {
		t.Fatalf("draft=%q, want 我想", debounce.Draft)
	}
}

func TestHandleSettingsSavePersistsGlobalPredictionFlag(t *testing.T) {
	home := setTestHome(t)
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
	app.predictionEnabled = true
	app.predictionText = "继续帮我拆一下这个方案"
	app.shell.SetPrediction(app.predictionText)

	disabled := false
	app.handleSettingsSave(&settings.Settings{Language: "zh"}, &disabled)

	if app.predictionEnabled {
		t.Fatalf("expected prediction to be disabled")
	}
	if app.shell.HasPrediction() {
		t.Fatalf("expected visible prediction to clear after disabling")
	}

	raw, err := os.ReadFile(filepath.Join(home, ".eos.json"))
	if err != nil {
		t.Fatalf("read global config: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal global config: %v", err)
	}
	if got := doc["next_message_prediction_enabled"]; got != false {
		t.Fatalf("next_message_prediction_enabled=%v, want false", got)
	}
}

func TestThinkingStatusIsNotRenderedAsFoldStateWhenIdle(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "https://example.com/v1")
	t.Setenv("EOS_API_KEY", "secret")
	t.Setenv("EOS_MODEL", "demo-model")
	state.SetThinking(true)
	t.Cleanup(func() { state.SetThinking(true) })

	app := newTestAppModel(t)
	view := stripANSIAppTest(app.shell.View())
	if strings.Contains(view, "思考:折叠") || strings.Contains(view, "Thinking:collapsed") {
		t.Fatalf("idle status should not expose thinking fold state, got %q", view)
	}
	if !strings.Contains(view, "思考:开") {
		t.Fatalf("expected idle status to show thinking switch state, got %q", view)
	}
}

func TestThinkingLiveBlockTogglesInContentAndClearsWhenAnswerStarts(t *testing.T) {
	setTestHome(t)
	t.Setenv("EOS_API_BASE", "https://example.com/v1")
	t.Setenv("EOS_API_KEY", "secret")
	t.Setenv("EOS_MODEL", "demo-model")
	state.SetThinking(true)
	t.Cleanup(func() { state.SetThinking(true) })

	app := newTestAppModel(t)
	app.state.Processing = true
	app.shell.SetProcessing(true)
	app.currentAIStartTime = time.Now()

	next, _ := app.Update(ThinkingMsg{Content: "第一步分析\n第二步推理"})
	updated := next.(*AppModel)
	view := stripANSIAppTest(updated.shell.View())
	if !strings.Contains(view, "Thinking") || !strings.Contains(view, "Alt+H") {
		t.Fatalf("expected current thinking to render in content live block, got %q", view)
	}
	if !strings.Contains(view, "第二步推理") {
		t.Fatalf("expected collapsed thinking summary to be visible, got %q", view)
	}
	if strings.Contains(view, "思考:折叠") {
		t.Fatalf("status bar should not render thinking fold state, got %q", view)
	}

	historyLen := len(updated.history)
	updated = sendAppKey(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: true})
	view = stripANSIAppTest(updated.shell.View())
	if !strings.Contains(view, "第一步分析") {
		t.Fatalf("expected Alt+H to expand current thinking content, got %q", view)
	}
	if len(updated.history) != historyLen {
		t.Fatalf("Alt+H should not append a history message, got %d want %d", len(updated.history), historyLen)
	}

	updated.handleAIResponse(AIResponseMsg{Type: "delta", Content: "最终回答"})
	view = stripANSIAppTest(updated.shell.View())
	if strings.Contains(view, "第一步分析") || strings.Contains(view, "第二步推理") || strings.Contains(view, "Thinking") {
		t.Fatalf("thinking live block should clear once assistant output starts, got %q", view)
	}
	if !strings.Contains(view, "最终回答") {
		t.Fatalf("expected assistant live output after thinking clears, got %q", view)
	}
}

func TestRenderHistoryEntryShowsDownloadActionOnlyForPlanMessages(t *testing.T) {
	setTestHome(t)
	app := newTestAppModel(t)

	planRendered := stripANSIAppTest(app.renderHistoryEntry(historyEntry{
		kind:          "ai",
		content:       "## 执行计划",
		rawMarkdown:   "## 执行计划",
		executionMode: "plan",
		timestamp:     time.Now(),
	}))
	if !strings.Contains(planRendered, "复制") {
		t.Fatalf("expected copy action in plan render, got %q", planRendered)
	}
	if !strings.Contains(planRendered, "下载") {
		t.Fatalf("expected download action in plan render, got %q", planRendered)
	}

	autoRendered := stripANSIAppTest(app.renderHistoryEntry(historyEntry{
		kind:          "ai",
		content:       "普通回复",
		rawMarkdown:   "普通回复",
		executionMode: "auto",
		timestamp:     time.Now(),
	}))
	if !strings.Contains(autoRendered, "复制") {
		t.Fatalf("expected copy action in auto render, got %q", autoRendered)
	}
	if strings.Contains(autoRendered, "下载") {
		t.Fatalf("did not expect download action in auto render, got %q", autoRendered)
	}
}

func TestHandlePlanDownloadActionFallsBackToManualPath(t *testing.T) {
	setTestHome(t)
	app := newTestAppModel(t)
	app.history = []historyEntry{{
		kind:          "ai",
		content:       "## 执行计划",
		rawMarkdown:   "## 执行计划",
		executionMode: "plan",
		timestamp:     time.Now(),
	}}

	origChooser := choosePlanDownloadDirectory
	choosePlanDownloadDirectory = func(string) (string, error) {
		return "", filedialog.ErrUnavailable
	}
	t.Cleanup(func() {
		choosePlanDownloadDirectory = origChooser
	})

	cmd := app.handlePlanDownloadAction(0)
	if cmd == nil {
		t.Fatalf("expected non-nil command to mark the mouse action as handled")
	}
	if app.confirmView == nil {
		t.Fatalf("expected fallback confirm view to open")
	}
	if app.pendingPlanDownload == nil || app.pendingPlanDownload.HistoryIndex != 0 {
		t.Fatalf("expected pending plan download state to be recorded, got %+v", app.pendingPlanDownload)
	}
	if app.activeView != "confirm" {
		t.Fatalf("activeView=%q, want confirm", app.activeView)
	}
}

func TestSavePlanHistoryEntryToDirWritesMarkdownAndDeduplicatesName(t *testing.T) {
	setTestHome(t)
	app := newTestAppModel(t)
	dir := t.TempDir()
	ts := time.Date(2026, 5, 9, 12, 30, 0, 0, time.UTC)
	app.history = []historyEntry{{
		kind:          "ai",
		content:       "## 执行计划",
		rawMarkdown:   "# Plan\n\n- step 1\n",
		executionMode: "plan",
		timestamp:     ts,
	}}

	first, err := app.savePlanHistoryEntryToDir(0, dir)
	if err != nil {
		t.Fatalf("first save error: %v", err)
	}
	second, err := app.savePlanHistoryEntryToDir(0, dir)
	if err != nil {
		t.Fatalf("second save error: %v", err)
	}
	if first == second {
		t.Fatalf("expected unique file path on second save, got %q", first)
	}
	raw1, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first file: %v", err)
	}
	raw2, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second file: %v", err)
	}
	if string(raw1) != "# Plan\n\n- step 1\n" || string(raw2) != "# Plan\n\n- step 1\n" {
		t.Fatalf("saved markdown mismatch: %q / %q", string(raw1), string(raw2))
	}
}

func TestSavePlanHistoryEntryToDirRejectsNonPlanMessage(t *testing.T) {
	setTestHome(t)
	app := newTestAppModel(t)
	app.history = []historyEntry{{
		kind:          "ai",
		content:       "普通回复",
		rawMarkdown:   "普通回复",
		executionMode: "auto",
		timestamp:     time.Now(),
	}}

	_, err := app.savePlanHistoryEntryToDir(0, t.TempDir())
	if err == nil {
		t.Fatalf("expected non-plan message to be rejected")
	}
	if !strings.Contains(err.Error(), "计划") && !strings.Contains(strings.ToLower(err.Error()), "downloadable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandlePlanDownloadActionReportsChooserFailure(t *testing.T) {
	setTestHome(t)
	app := newTestAppModel(t)
	app.history = []historyEntry{{
		kind:          "ai",
		content:       "## 执行计划",
		rawMarkdown:   "## 执行计划",
		executionMode: "plan",
		timestamp:     time.Now(),
	}}

	origChooser := choosePlanDownloadDirectory
	choosePlanDownloadDirectory = func(string) (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() {
		choosePlanDownloadDirectory = origChooser
	})

	_ = app.handlePlanDownloadAction(0)
	if len(app.history) == 0 {
		t.Fatalf("expected an error message in history")
	}
	last := app.history[len(app.history)-1]
	if last.kind != "system" || last.level != "error" {
		t.Fatalf("expected system error entry, got %+v", last)
	}
}
