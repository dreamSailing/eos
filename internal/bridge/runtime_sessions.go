package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/session"
)

type PersistedSession struct {
	ID      string `json:"id"`
	SavedAt int64  `json:"saved_at"`
	Cwd     string `json:"cwd"`
	Model   string `json:"model"`
	Summary string `json:"summary"`
	Preview string `json:"preview,omitempty"`
	Title   string `json:"display_title,omitempty"`
	Rounds  int    `json:"rounds,omitempty"`
	Tokens  int    `json:"tokens,omitempty"`

	Context      session.ContextState       `json:"context"`
	TokenHistory []TokenRecord              `json:"token_history,omitempty"`
	Transcript   []SessionTranscriptMessage `json:"transcript,omitempty"`
}

type PersistedSessionMeta struct {
	ID      string
	SavedAt int64
	Model   string
	Summary string
	Preview string
	Title   string
	Rounds  int
	Tokens  int
}

type SessionWorkspaceState struct {
	CurrentSessionID string `json:"current_session_id,omitempty"`
}

type SessionTranscriptMessage struct {
	Role       string   `json:"role,omitempty"`
	Type       string   `json:"type,omitempty"`
	Content    string   `json:"content"`
	Timestamp  int64    `json:"timestamp,omitempty"`
	ImagePaths []string `json:"image_paths,omitempty"`
}

func (rc *RuntimeCore) AutoSaveSession(ctx context.Context) {
	_, _ = rc.SaveSession(ctx, "")
}

func (rc *RuntimeCore) sessionsDir() string {
	root := rc.workingRoot()
	if strings.TrimSpace(root) == "" {
		return filepath.Join(".eos", "sessions")
	}
	return filepath.Join(root, ".eos", "sessions")
}

func (rc *RuntimeCore) SessionsDir() string {
	return rc.sessionsDir()
}

func (rc *RuntimeCore) sessionStatePath() string {
	root := rc.workingRoot()
	if strings.TrimSpace(root) == "" {
		return filepath.Join(".eos", "session_state.json")
	}
	return filepath.Join(root, ".eos", "session_state.json")
}

func (rc *RuntimeCore) SaveSession(ctx context.Context, id string) (string, error) {
	if rc == nil || rc.cm == nil {
		return "", ErrRuntimeLoopUnavailable
	}

	cwd := rc.workingRoot()
	if strings.TrimSpace(id) == "" {
		id = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	st := rc.cm.ExportState()
	savedModel := st.ModelName
	if savedModel == "" {
		rc.mu.RLock()
		savedModel = rc.modelName
		rc.mu.RUnlock()
	}

	ps := PersistedSession{
		ID:      id,
		SavedAt: time.Now().Unix(),
		Cwd:     cwd,
		Model:   savedModel,
		Summary: bestEffortSessionSummary(rc.cm),
		Preview: bestEffortSessionPreview(rc.cm),
		Context: st,
	}
	if existing, err := rc.loadSessionFromDisk(id); err == nil {
		if len(existing.Transcript) > 0 {
			ps.Transcript = append([]SessionTranscriptMessage{}, existing.Transcript...)
		}
		ps.Title = strings.TrimSpace(existing.Title)
	}

	rc.tokenMu.RLock()
	if len(rc.tokenHistory) > 0 {
		ps.TokenHistory = append([]TokenRecord{}, rc.tokenHistory...)
		ps.Rounds = len(ps.TokenHistory)
		for _, r := range ps.TokenHistory {
			ps.Tokens += r.Total
		}
	}
	rc.tokenMu.RUnlock()

	dir := rc.sessionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, id+".json")
	b, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		return "", err
	}

	_ = cleanupOldSessions(dir, 20)
	rc.setCurrentSessionBestEffort(id)
	return id, nil
}

func (rc *RuntimeCore) SaveSessionMessages(ctx context.Context, id string, messages []SessionTranscriptMessage) (string, error) {
	savedID, err := rc.SaveSession(ctx, id)
	if err != nil {
		return "", err
	}
	ps, err := rc.loadSessionFromDisk(savedID)
	if err != nil {
		return "", err
	}
	ps.Transcript = copySessionTranscript(messages)
	if len(sessionPreviewFromState(ps.Context)) == 0 && len(ps.Transcript) > 0 {
		ps.Context = contextStateFromTranscript(ps.Transcript, ps.Model)
	}
	ps.Summary = bestEffortTranscriptSummary(ps.Transcript, ps.Summary)
	ps.Preview = bestEffortTranscriptPreview(ps.Transcript, ps.Preview)
	if ps.Rounds == 0 && len(ps.Transcript) > 0 {
		ps.Rounds = transcriptRoundCount(ps.Transcript)
	}

	dir := rc.sessionsDir()
	path := filepath.Join(dir, savedID+".json")
	b, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		return "", err
	}
	return savedID, nil
}

func (rc *RuntimeCore) ListSessions() ([]PersistedSessionMeta, error) {
	dir := rc.sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	out := make([]PersistedSessionMeta, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		p := filepath.Join(dir, name)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var ps PersistedSession
		if err := json.Unmarshal(b, &ps); err != nil {
			continue
		}
		if strings.TrimSpace(ps.ID) == "" {
			ps.ID = strings.TrimSuffix(name, ".json")
		}
		out = append(out, PersistedSessionMeta{
			ID:      ps.ID,
			SavedAt: ps.SavedAt,
			Model:   ps.Model,
			Summary: strings.TrimSpace(ps.Summary),
			Preview: bestEffortPersistedPreview(ps),
			Title:   strings.TrimSpace(ps.Title),
			Rounds:  ps.Rounds,
			Tokens:  ps.Tokens,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt > out[j].SavedAt })
	return out, nil
}

func (rc *RuntimeCore) LoadSessionWorkspaceState() (SessionWorkspaceState, error) {
	path := rc.sessionStatePath()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionWorkspaceState{}, nil
		}
		return SessionWorkspaceState{}, err
	}
	var state SessionWorkspaceState
	if err := json.Unmarshal(b, &state); err != nil {
		return SessionWorkspaceState{}, err
	}
	state.CurrentSessionID = strings.TrimSpace(state.CurrentSessionID)
	return state, nil
}

func (rc *RuntimeCore) SaveSessionWorkspaceState(state SessionWorkspaceState) error {
	state.CurrentSessionID = strings.TrimSpace(state.CurrentSessionID)
	path := rc.sessionStatePath()
	if state.CurrentSessionID == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func (rc *RuntimeCore) CurrentSessionID() (string, error) {
	state, err := rc.LoadSessionWorkspaceState()
	if err != nil {
		return "", err
	}
	if state.CurrentSessionID == "" {
		return "", nil
	}
	if _, err := rc.loadSessionFromDisk(state.CurrentSessionID); err != nil {
		if os.IsNotExist(err) {
			_ = rc.SaveSessionWorkspaceState(SessionWorkspaceState{})
			return "", nil
		}
		return "", err
	}
	return state.CurrentSessionID, nil
}

func (rc *RuntimeCore) SetCurrentSession(id string) error {
	id = strings.TrimSpace(id)
	if id != "" {
		if _, err := rc.loadSessionFromDisk(id); err != nil {
			return err
		}
	}
	return rc.SaveSessionWorkspaceState(SessionWorkspaceState{CurrentSessionID: id})
}

func (rc *RuntimeCore) resolvePreferredSessionID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id != "" {
		return id, nil
	}
	currentID, err := rc.CurrentSessionID()
	if err != nil {
		return "", err
	}
	if currentID != "" {
		return currentID, nil
	}
	metas, err := rc.ListSessions()
	if err != nil {
		return "", err
	}
	if len(metas) == 0 {
		return "", os.ErrNotExist
	}
	id = metas[0].ID
	rc.setCurrentSessionBestEffort(id)
	return id, nil
}

func (rc *RuntimeCore) setCurrentSessionBestEffort(id string) {
	_ = rc.SetCurrentSession(id)
}

func (rc *RuntimeCore) ResumeSession(ctx context.Context, id string) error {
	if rc == nil || rc.cm == nil {
		return ErrRuntimeLoopUnavailable
	}
	resolvedID, err := rc.resolvePreferredSessionID(id)
	if err != nil {
		return err
	}
	ps, err := rc.loadSessionFromDisk(resolvedID)
	if err != nil {
		return err
	}
	rc.cm.ImportState(ps.Context)
	if strings.TrimSpace(ps.Model) != "" {
		rc.cm.SetModel(ps.Model)
	}
	rc.tokenMu.Lock()
	rc.tokenHistory = append([]TokenRecord{}, ps.TokenHistory...)
	rc.tokenMu.Unlock()
	rc.setCurrentSessionBestEffort(ps.ID)
	return nil
}

func (rc *RuntimeCore) SessionPreview(id string) ([]ai.Message, error) {
	ps, err := rc.loadSessionFromDisk(id)
	if err != nil {
		return nil, err
	}
	return sessionPreviewFromState(ps.Context), nil
}

func (rc *RuntimeCore) LoadSessionMessages(id string) ([]SessionTranscriptMessage, error) {
	ps, err := rc.loadSessionFromDisk(id)
	if err != nil {
		return nil, err
	}
	if len(ps.Transcript) > 0 {
		return copySessionTranscript(ps.Transcript), nil
	}
	preview := sessionPreviewFromState(ps.Context)
	out := make([]SessionTranscriptMessage, 0, len(preview))
	for _, msg := range preview {
		role := normalizedTranscriptRole(msg.Role)
		out = append(out, SessionTranscriptMessage{
			Role:       role,
			Type:       role,
			Content:    strings.TrimSpace(msg.Content),
			ImagePaths: append([]string{}, msg.ImagePaths...),
		})
	}
	return out, nil
}

func (rc *RuntimeCore) UpdateSessionTitle(id, title string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("session id required")
	}
	ps, err := rc.loadSessionFromDisk(id)
	if err != nil {
		return err
	}
	ps.Title = strings.TrimSpace(title)
	path := filepath.Join(rc.sessionsDir(), id+".json")
	b, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func (rc *RuntimeCore) DeleteSession(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("session id required")
	}
	state, _ := rc.LoadSessionWorkspaceState()
	path := filepath.Join(rc.sessionsDir(), id+".json")
	if err := os.Remove(path); err != nil {
		return err
	}
	if state.CurrentSessionID == id {
		nextID := ""
		if metas, err := rc.ListSessions(); err == nil && len(metas) > 0 {
			nextID = metas[0].ID
		}
		rc.setCurrentSessionBestEffort(nextID)
	}
	return nil
}

func copySessionTranscript(messages []SessionTranscriptMessage) []SessionTranscriptMessage {
	if len(messages) == 0 {
		return nil
	}
	out := make([]SessionTranscriptMessage, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		out = append(out, SessionTranscriptMessage{
			Role:       normalizedTranscriptRole(msg.Role),
			Type:       normalizedTranscriptType(msg.Type, msg.Role),
			Content:    content,
			Timestamp:  msg.Timestamp,
			ImagePaths: append([]string{}, msg.ImagePaths...),
		})
	}
	return out
}

func sessionPreviewFromState(st session.ContextState) []ai.Message {
	cm := session.NewContextManager()
	cm.ImportState(st)
	return cm.BuildPreview()
}

func contextStateFromTranscript(messages []SessionTranscriptMessage, model string) session.ContextState {
	cm := session.NewContextManager()
	if strings.TrimSpace(model) != "" {
		cm.SetModel(model)
	}
	st := cm.ExportState()
	st.ModelName = strings.TrimSpace(model)
	st.Recent = make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		st.Recent = append(st.Recent, ai.Message{
			Role:       normalizedTranscriptRole(msg.Role),
			Content:    content,
			ImagePaths: append([]string{}, msg.ImagePaths...),
		})
	}
	return st
}

func normalizedTranscriptRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "ai":
		return "assistant"
	case "tool", "error", "system":
		return "system"
	default:
		return "user"
	}
}

func normalizedTranscriptType(kind string, role string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "" {
		return kind
	}
	return normalizedTranscriptRole(role)
}

func bestEffortTranscriptSummary(messages []SessionTranscriptMessage, fallback string) string {
	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		kind := strings.ToLower(strings.TrimSpace(msg.Type))
		if role != "user" && kind != "user" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if len(content) > 80 {
			content = content[:80] + "…"
		}
		return content
	}
	return strings.TrimSpace(fallback)
}

func bestEffortTranscriptPreview(messages []SessionTranscriptMessage, fallback string) string {
	candidate := bestEffortPreviewTextFromTranscript(messages)
	if candidate != "" {
		return candidate
	}
	return normalizeSessionPreview(fallback)
}

func bestEffortPersistedPreview(ps PersistedSession) string {
	if preview := normalizeSessionPreview(ps.Preview); preview != "" {
		return preview
	}
	if preview := bestEffortPreviewTextFromTranscript(ps.Transcript); preview != "" {
		return preview
	}
	return bestEffortPreviewTextFromMessages(sessionPreviewFromState(ps.Context))
}

func transcriptRoundCount(messages []SessionTranscriptMessage) int {
	rounds := 0
	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		kind := strings.ToLower(strings.TrimSpace(msg.Type))
		if role == "user" || kind == "user" {
			rounds++
		}
	}
	if rounds == 0 && len(messages) > 0 {
		return len(messages)
	}
	return rounds
}

func (rc *RuntimeCore) ExecuteSessions(ui CoreUI, args []string) bool {
	if len(args) >= 2 {
		sub := strings.ToLower(args[1])
		switch sub {
		case "save":
			id, err := rc.SaveSession(context.Background(), "")
			if err != nil {
				ui.WriteLine("red", "Error: "+err.Error())
				return true
			}
			ui.WriteLine("green", "Saved session: "+id)
			return true
		case "export":
			id := ""
			if len(args) >= 3 {
				id = args[2]
			}
			path := ""
			if len(args) >= 4 {
				path = args[3]
			} else if strings.TrimSpace(id) != "" {
				path = filepath.Join(rc.sessionsDir(), id+".md")
			}
			if strings.TrimSpace(path) == "" {
				ui.WriteLine("yellow", "Usage: /sessions export <id> [outputPath]")
				return true
			}
			if err := rc.SaveSessionMarkdown(id, path); err != nil {
				ui.WriteLine("red", "Error: "+err.Error())
				return true
			}
			ui.WriteLine("green", "Exported: "+path)
			return true
		}
	}

	metas, err := rc.ListSessions()
	if err != nil {
		ui.WriteLine("red", "Error: "+err.Error())
		return true
	}
	if len(metas) == 0 {
		ui.WriteLine("yellow", "No saved sessions.")
		return true
	}
	ui.WriteLine("white", "Saved sessions:")
	limit := 20
	if len(metas) < limit {
		limit = len(metas)
	}
	for i := 0; i < limit; i++ {
		m := metas[i]
		ts := time.Unix(m.SavedAt, 0).Format("2006-01-02 15:04:05")
		line := fmt.Sprintf("- %s  %s  %s  rounds=%d tokens=%d", m.ID, ts, strings.TrimSpace(m.Model), m.Rounds, m.Tokens)
		label := strings.TrimSpace(m.Title)
		if label == "" {
			label = strings.TrimSpace(m.Summary)
		}
		if label != "" {
			line += "  " + label
		}
		ui.WriteLine("white", line)
	}
	ui.WriteLine("white", "Usage: /resume [id]")
	ui.WriteLine("white", "Usage: /sessions save | /sessions export <id> [outputPath]")
	return true
}

func (rc *RuntimeCore) ExecuteResume(ui CoreUI, args []string) bool {
	id := ""
	if len(args) >= 2 {
		id = args[1]
	}
	resolvedID, err := rc.resolvePreferredSessionID(id)
	if err != nil {
		if os.IsNotExist(err) {
			ui.WriteLine("yellow", "No saved sessions.")
			return true
		}
		ui.WriteLine("red", "Error: "+err.Error())
		return true
	}
	if err := rc.ResumeSession(context.Background(), resolvedID); err != nil {
		ui.WriteLine("red", "Error: "+err.Error())
		return true
	}
	ui.WriteLine("green", "Resumed session: "+resolvedID)
	return true
}

func bestEffortSessionSummary(cm *session.ContextManager) string {
	msgs := cm.BuildPreview()
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != "user" {
			continue
		}
		s := strings.TrimSpace(m.Content)
		if s == "" {
			continue
		}
		if len(s) > 80 {
			s = s[:80] + "…"
		}
		return s
	}
	return ""
}

func bestEffortSessionPreview(cm *session.ContextManager) string {
	return bestEffortPreviewTextFromMessages(cm.BuildPreview())
}

func bestEffortPreviewTextFromTranscript(messages []SessionTranscriptMessage) string {
	return bestEffortPreviewText(len(messages), func(i int) (string, string) {
		msg := messages[i]
		return normalizedTranscriptType(msg.Type, msg.Role), msg.Content
	})
}

func bestEffortPreviewTextFromMessages(messages []ai.Message) string {
	return bestEffortPreviewText(len(messages), func(i int) (string, string) {
		return normalizedTranscriptRole(messages[i].Role), messages[i].Content
	})
}

func bestEffortPreviewText(count int, at func(i int) (string, string)) string {
	var userFallback string
	var systemFallback string
	for i := count - 1; i >= 0; i-- {
		kind, content := at(i)
		text := normalizeSessionPreview(content)
		if text == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(kind)) {
		case "assistant", "ai":
			return text
		case "user":
			if userFallback == "" {
				userFallback = text
			}
		default:
			if systemFallback == "" {
				systemFallback = text
			}
		}
	}
	if userFallback != "" {
		return userFallback
	}
	return systemFallback
}

func normalizeSessionPreview(content string) string {
	content = strings.TrimSpace(strings.Join(strings.Fields(strings.TrimSpace(content)), " "))
	if content == "" {
		return ""
	}
	if len(content) > 96 {
		content = content[:96] + "…"
	}
	return content
}

func loadSessionFromDiskInDir(dir, id string) (PersistedSession, error) {
	if strings.TrimSpace(id) == "" {
		metas, err := listSessionsInDir(dir)
		if err != nil {
			return PersistedSession{}, err
		}
		if len(metas) == 0 {
			return PersistedSession{}, os.ErrNotExist
		}
		id = metas[0].ID
	}
	path := filepath.Join(dir, id+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return PersistedSession{}, err
	}
	var ps PersistedSession
	if err := json.Unmarshal(b, &ps); err != nil {
		return PersistedSession{}, err
	}
	if ps.ID == "" {
		ps.ID = id
	}
	return ps, nil
}

func (rc *RuntimeCore) loadSessionFromDisk(id string) (PersistedSession, error) {
	return loadSessionFromDiskInDir(rc.sessionsDir(), id)
}

func listSessionsInDir(dir string) ([]PersistedSessionMeta, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]PersistedSessionMeta, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var meta struct {
			ID      string `json:"id"`
			SavedAt int64  `json:"saved_at"`
			Model   string `json:"model"`
			Summary string `json:"summary"`
			Rounds  int    `json:"rounds"`
			Tokens  int    `json:"tokens"`
		}
		if err := json.Unmarshal(b, &meta); err != nil {
			continue
		}
		if meta.ID == "" {
			meta.ID = strings.TrimSuffix(e.Name(), ".json")
		}
		out = append(out, PersistedSessionMeta{
			ID:      meta.ID,
			SavedAt: meta.SavedAt,
			Model:   meta.Model,
			Summary: meta.Summary,
			Rounds:  meta.Rounds,
			Tokens:  meta.Tokens,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt > out[j].SavedAt })
	return out, nil
}

func cleanupOldSessions(dir string, keep int) error {
	if keep <= 0 {
		keep = 20
	}
	metas, err := listSessionsInDir(dir)
	if err != nil {
		return err
	}
	if len(metas) <= keep {
		return nil
	}
	toDelete := metas[keep:]
	for _, m := range toDelete {
		_ = os.Remove(filepath.Join(dir, m.ID+".json"))
	}
	return nil
}

func buildMarkdownExport(cm *session.ContextManager) string {
	msgs := cm.BuildPreview()
	var sb strings.Builder
	sb.WriteString("# Session Export\n\n")
	for _, m := range msgs {
		role := strings.ToUpper(m.Role)
		sb.WriteString("## " + role + "\n\n")
		sb.WriteString(strings.TrimSpace(m.Content))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func (rc *RuntimeCore) ExportSessionMarkdown(id string) (string, error) {
	ps, err := rc.loadSessionFromDisk(id)
	if err != nil {
		return "", err
	}
	cm := session.NewContextManager()
	cm.ImportState(ps.Context)
	return buildMarkdownExport(cm), nil
}

func (rc *RuntimeCore) SaveSessionMarkdown(id string, outputPath string) error {
	md, err := rc.ExportSessionMarkdown(id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("outputPath is empty")
	}
	return os.WriteFile(outputPath, []byte(md), 0644)
}
