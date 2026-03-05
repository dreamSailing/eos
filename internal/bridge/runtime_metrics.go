package bridge

import (
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

type RequestMetric struct {
	ID        string
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
	Model     string

	ToolCalls      map[string]int
	ToolCallsError map[string]int
}

type PerfSummary struct {
	Requests int
	AvgMs    int
	P50Ms    int
	P95Ms    int

	TopTools []ToolCount
}

type ToolCount struct {
	Tool  string
	Calls int
}

func (rc *RuntimeCore) StartRequest(id string) {
	if rc == nil {
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	rc.metricsMu.Lock()
	defer rc.metricsMu.Unlock()
	if rc.inflightReq == nil {
		rc.inflightReq = make(map[string]*RequestMetric)
	}
	rc.inflightReq[id] = &RequestMetric{
		ID:        id,
		StartedAt: time.Now(),
		ToolCalls: make(map[string]int),
		ToolCallsError: make(map[string]int),
	}
	slog.Debug("perf.request.start", "component", utils.ComponentUI, "operation", "ai_request", "trace_id", id)
}

func (rc *RuntimeCore) EndRequest(id string, model string) {
	if rc == nil {
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	rc.metricsMu.Lock()
	defer rc.metricsMu.Unlock()
	if rc.inflightReq == nil {
		return
	}
	m := rc.inflightReq[id]
	if m == nil {
		return
	}
	delete(rc.inflightReq, id)
	m.EndedAt = time.Now()
	m.Duration = m.EndedAt.Sub(m.StartedAt)
	m.Model = strings.TrimSpace(model)
	rc.reqHistory = append(rc.reqHistory, *m)
	if len(rc.reqHistory) > 200 {
		rc.reqHistory = rc.reqHistory[len(rc.reqHistory)-200:]
	}
	toolTotal := 0
	for _, v := range m.ToolCalls {
		toolTotal += v
	}
	toolErr := 0
	for _, v := range m.ToolCallsError {
		toolErr += v
	}
	slog.Info("perf.request.completed",
		"component", utils.ComponentUI,
		"operation", "ai_request",
		"trace_id", id,
		"duration_ms", m.Duration.Milliseconds(),
		"model", m.Model,
		"tool_calls", toolTotal,
		"tool_errors", toolErr,
	)
}

func (rc *RuntimeCore) RecordToolCall(id string, toolName string) {
	if rc == nil {
		return
	}
	id = strings.TrimSpace(id)
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	if id == "" || toolName == "" {
		return
	}
	rc.metricsMu.Lock()
	defer rc.metricsMu.Unlock()
	if rc.inflightReq == nil {
		return
	}
	m := rc.inflightReq[id]
	if m == nil {
		return
	}
	if m.ToolCalls == nil {
		m.ToolCalls = make(map[string]int)
	}
	m.ToolCalls[toolName]++
}

func (rc *RuntimeCore) RecordToolResult(id string, toolName string, success bool) {
	if rc == nil {
		return
	}
	id = strings.TrimSpace(id)
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	if id == "" || toolName == "" || success {
		return
	}
	rc.metricsMu.Lock()
	defer rc.metricsMu.Unlock()
	if rc.inflightReq == nil {
		return
	}
	m := rc.inflightReq[id]
	if m == nil {
		return
	}
	if m.ToolCallsError == nil {
		m.ToolCallsError = make(map[string]int)
	}
	m.ToolCallsError[toolName]++
}

func (rc *RuntimeCore) GetPerfSummary() PerfSummary {
	rc.metricsMu.Lock()
	history := append([]RequestMetric{}, rc.reqHistory...)
	rc.metricsMu.Unlock()

	ds := make([]int, 0, len(history))
	toolAgg := map[string]int{}
	for _, h := range history {
		if h.Duration > 0 {
			ds = append(ds, int(h.Duration.Milliseconds()))
		}
		for k, v := range h.ToolCalls {
			toolAgg[k] += v
		}
	}
	sort.Ints(ds)

	out := PerfSummary{Requests: len(history)}
	if len(ds) > 0 {
		sum := 0
		for _, v := range ds {
			sum += v
		}
		out.AvgMs = sum / len(ds)
		out.P50Ms = percentileInts(ds, 0.50)
		out.P95Ms = percentileInts(ds, 0.95)
	}
	out.TopTools = topTools(toolAgg, 10)
	return out
}

func percentileInts(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	i := int(float64(len(sorted)-1) * p)
	if i < 0 {
		i = 0
	}
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

func topTools(m map[string]int, n int) []ToolCount {
	if n <= 0 {
		n = 10
	}
	out := make([]ToolCount, 0, len(m))
	for k, v := range m {
		out = append(out, ToolCount{Tool: k, Calls: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Calls == out[j].Calls {
			return out[i].Tool < out[j].Tool
		}
		return out[i].Calls > out[j].Calls
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
