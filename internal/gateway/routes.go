package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dreamSailing/eos/internal/scheduler"
	"github.com/dreamSailing/eos/internal/tools/bg"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/sessions", s.handleSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.handleSessionDetail)
	s.mux.HandleFunc("GET /api/prompts", s.handlePrompts)
	s.mux.HandleFunc("POST /api/approvals/{id}/resolve", s.handleResolveApproval)
	s.mux.HandleFunc("POST /api/inquiries/{id}/resolve", s.handleResolveInquiry)
	s.mux.HandleFunc("GET /api/tasks", s.handleTasks)
	s.mux.HandleFunc("GET /api/tasks/{id}/logs", s.handleTaskLogs)
	s.mux.HandleFunc("POST /api/tasks/{id}/kill", s.handleTaskKill)
	s.mux.HandleFunc("GET /api/schedules", s.handleSchedules)
	s.mux.HandleFunc("POST /api/schedules", s.handleScheduleCreate)
	s.mux.HandleFunc("PUT /api/schedules/{id}", s.handleScheduleUpdate)
	s.mux.HandleFunc("DELETE /api/schedules/{id}", s.handleScheduleDelete)
	s.mux.HandleFunc("POST /api/schedules/{id}/trigger", s.handleScheduleTrigger)
	if s.ctx != nil && s.ctx.MCP != nil {
		mounted := s.ctx.MCP.NewMountedSSE(s.BaseURL(), s.opts.MCPBasePath)
		mounted.Attach(s.mux)
	}
	s.mountWeb()
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	payload := map[string]any{}
	if s.ctx != nil && s.ctx.StatusSnapshot != nil {
		payload = s.ctx.StatusSnapshot()
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.MCP == nil {
		writeError(w, http.StatusServiceUnavailable, "mcp unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.ctx.MCP.SessionSnapshots()})
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.MCP == nil {
		writeError(w, http.StatusServiceUnavailable, "mcp unavailable")
		return
	}
	payload, ok := s.ctx.MCP.SessionDetail(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.MCP == nil {
		writeError(w, http.StatusServiceUnavailable, "mcp unavailable")
		return
	}
	writeJSON(w, http.StatusOK, s.ctx.MCP.PromptSnapshots())
}

func (s *Server) handleResolveApproval(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.MCP == nil {
		writeError(w, http.StatusServiceUnavailable, "mcp unavailable")
		return
	}
	var body struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
		PolicyID string `json:"policy_id"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	payload, err := s.ctx.MCP.ResolveApproval(r.PathValue("id"), body.Decision, body.Reason, body.PolicyID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleResolveInquiry(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.MCP == nil {
		writeError(w, http.StatusServiceUnavailable, "mcp unavailable")
		return
	}
	var body struct {
		Option string `json:"option"`
		Text   string `json:"text"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	payload, err := s.ctx.MCP.ResolveInquiry(r.PathValue("id"), body.Option, body.Text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.Services == nil {
		writeError(w, http.StatusServiceUnavailable, "services unavailable")
		return
	}
	items, err := s.ctx.Services.Tasks().List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": items})
}

func (s *Server) handleTaskLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	fromSeq, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("from_seq")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	res, err := bg.Default().Tail(id, &bg.TailOptions{FromSeq: fromSeq, Limit: limit})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tail": res})
}

func (s *Server) handleTaskKill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if s.ctx == nil || s.ctx.Services == nil {
		writeError(w, http.StatusServiceUnavailable, "services unavailable")
		return
	}
	if err := s.ctx.Services.Tasks().Kill(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.Scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": s.ctx.Scheduler.List()})
}

func (s *Server) handleScheduleCreate(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.Scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler unavailable")
		return
	}
	var body scheduleRequest
	if !decodeBody(w, r, &body) {
		return
	}
	item, err := s.ctx.Scheduler.Create(body.toSchedule())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleScheduleUpdate(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.Scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler unavailable")
		return
	}
	var body scheduleRequest
	if !decodeBody(w, r, &body) {
		return
	}
	item, err := s.ctx.Scheduler.Update(r.PathValue("id"), body.toSchedule())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleScheduleDelete(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.Scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler unavailable")
		return
	}
	if err := s.ctx.Scheduler.Delete(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleScheduleTrigger(w http.ResponseWriter, r *http.Request) {
	if s.ctx == nil || s.ctx.Scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "scheduler unavailable")
		return
	}
	item, err := s.ctx.Scheduler.TriggerNow(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type scheduleRequest struct {
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	Cron      string         `json:"cron"`
	Timezone  string         `json:"timezone"`
	Kind      string         `json:"kind"`
	Workspace string         `json:"workspace"`
	Payload   map[string]any `json:"payload"`
}

func (s scheduleRequest) toSchedule() scheduler.Schedule {
	return scheduler.Schedule{
		Name:      strings.TrimSpace(s.Name),
		Enabled:   s.Enabled,
		Cron:      strings.TrimSpace(s.Cron),
		Timezone:  strings.TrimSpace(s.Timezone),
		Kind:      scheduler.TaskKind(strings.TrimSpace(s.Kind)),
		Workspace: strings.TrimSpace(s.Workspace),
		Payload:   s.Payload,
	}
}

func decodeBody(w http.ResponseWriter, r *http.Request, out any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]any{"error": strings.TrimSpace(message)})
}
