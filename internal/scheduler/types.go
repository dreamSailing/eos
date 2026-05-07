package scheduler

import "time"

type TaskKind string

const (
	TaskKindEOSCall TaskKind = "eos_call"
	TaskKindShell   TaskKind = "shell"
)

type Schedule struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Enabled    bool           `json:"enabled"`
	Cron       string         `json:"cron"`
	Timezone   string         `json:"timezone,omitempty"`
	Kind       TaskKind       `json:"kind"`
	Workspace  string         `json:"workspace,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
	LastRunAt  time.Time      `json:"last_run_at,omitempty"`
	NextRunAt  time.Time      `json:"next_run_at,omitempty"`
	LastStatus string         `json:"last_status,omitempty"`
	LastError  string         `json:"last_error,omitempty"`
}

type StoreData struct {
	Schedules []Schedule `json:"schedules,omitempty"`
}
