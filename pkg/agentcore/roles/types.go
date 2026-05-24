package roles

import "strings"

type ContextStrategy string

const (
	ContextShared      ContextStrategy = "shared"
	ContextIndependent ContextStrategy = "independent"
	ContextHybrid      ContextStrategy = "hybrid"
)

type RoleConfig struct {
	ID              string          `json:"id"`
	Description     string          `json:"description,omitempty"`
	SystemPrompt    string          `json:"system_prompt,omitempty"`
	PromptFile      string          `json:"prompt_file,omitempty"`
	AllowedTools    []string        `json:"allowed_tools,omitempty"`
	ContextStrategy ContextStrategy `json:"context_strategy,omitempty"`
	Model           string          `json:"model,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	LegacyNames     []string        `json:"legacy_names,omitempty"`
	LegacyAliases   []string        `json:"legacy_aliases,omitempty"`
}

type Document struct {
	Roles []RoleConfig `json:"roles"`
}

type ConfigPaths struct {
	UserPath    string
	ProjectPath string
}

func (p ConfigPaths) Ordered() []string {
	return compactStrings([]string{p.UserPath, p.ProjectPath})
}

func NormalizeRoleID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.ReplaceAll(id, "_", "-")
	return id
}
