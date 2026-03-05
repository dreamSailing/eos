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

	"github.com/dreamSailing/vb-coding/internal/session"
)

type PersistedSession struct {
	ID      string `json:"id"`
	SavedAt int64  `json:"saved_at"`
	Cwd     string `json:"cwd"`
	Model   string `json:"model"`
	Summary string `json:"summary"`
	Rounds  int    `json:"rounds,omitempty"`
	Tokens  int    `json:"tokens,omitempty"`

	Context      session.ContextState `json:"context"`
	TokenHistory []TokenRecord        `json:"token_history,omitempty"`
}

type PersistedSessionMeta struct {
	ID      string
	SavedAt int64
	Model   string
	Summary string
	Rounds  int
	Tokens  int
}

func (rc *RuntimeCore) AutoSaveSession(ctx context.Context) {
	_, _ = rc.SaveSession(ctx, "")
}

func (rc *RuntimeCore) SaveSession(ctx context.Context, id string) (string, error) {
	if rc == nil || rc.cm == nil {
		return "", ErrRuntimeLoopUnavailable
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
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
		Context: st,
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

	dir := filepath.Join(cwd, ".vb", "sessions")
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
	return id, nil
}

func (rc *RuntimeCore) ListSessions() ([]PersistedSessionMeta, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cwd, ".vb", "sessions")
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
		if strings.TrimSpace(meta.ID) == "" {
			meta.ID = strings.TrimSuffix(name, ".json")
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

func (rc *RuntimeCore) ResumeSession(ctx context.Context, id string) error {
	if rc == nil || rc.cm == nil {
		return ErrRuntimeLoopUnavailable
	}
	ps, err := loadSessionFromDisk(id)
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
	return nil
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
			cwd, _ := os.Getwd()
			path := ""
			if len(args) >= 4 {
				path = args[3]
			} else if strings.TrimSpace(id) != "" {
				path = filepath.Join(cwd, ".vb", "sessions", id+".md")
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
		if strings.TrimSpace(m.Summary) != "" {
			line += "  " + strings.TrimSpace(m.Summary)
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
	if strings.TrimSpace(id) == "" {
		metas, err := rc.ListSessions()
		if err != nil {
			ui.WriteLine("red", "Error: "+err.Error())
			return true
		}
		if len(metas) == 0 {
			ui.WriteLine("yellow", "No saved sessions.")
			return true
		}
		id = metas[0].ID
	}
	if err := rc.ResumeSession(context.Background(), id); err != nil {
		ui.WriteLine("red", "Error: "+err.Error())
		return true
	}
	ui.WriteLine("green", "Resumed session: "+id)
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

func loadSessionFromDisk(id string) (PersistedSession, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return PersistedSession{}, err
	}
	dir := filepath.Join(cwd, ".vb", "sessions")
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
	ps, err := loadSessionFromDisk(id)
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
