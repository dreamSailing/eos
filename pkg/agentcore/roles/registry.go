package roles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var roleIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var knownAllowedToolSet = map[string]struct{}{
	"*":                           {},
	"read":                        {},
	"write":                       {},
	"fs":                          {},
	"edit":                        {},
	"history":                     {},
	"search":                      {},
	"glob":                        {},
	"grep":                        {},
	"diff":                        {},
	"tool_search":                 {},
	"skill":                       {},
	"skills_list":                 {},
	"create_skill":                {},
	"time_now":                    {},
	"user_confirm":                {},
	"user_input":                  {},
	"user_select":                 {},
	"bash":                        {},
	"bash_session":                {},
	"bg_task":                     {},
	"plan_steps":                  {},
	"todo_read":                   {},
	"todo_write":                  {},
	"mcp_status":                  {},
	"browser_status":              {},
	"browser_navigate":            {},
	"browser_snapshot":            {},
	"browser_inspect":             {},
	"browser_tabs":                {},
	"browser_back":                {},
	"browser_forward":             {},
	"browser_click":               {},
	"browser_hover":               {},
	"browser_type":                {},
	"browser_press_key":           {},
	"browser_select":              {},
	"browser_wait":                {},
	"browser_scroll":              {},
	"browser_screenshot":          {},
	"browser_console":             {},
	"browser_network":             {},
	"browser_reload":              {},
	"browser_viewport":            {},
	"browser_visibility":          {},
	"browser_clipboard":           {},
	"browser_cua":                 {},
	"browser_dom_cua":             {},
	"browser_locator":             {},
	"browser_dev_logs":            {},
	"browser_downloads":           {},
	"browser_user_tabs":           {},
	"browser_session_name":        {},
	"git_status":                  {},
	"git_add":                     {},
	"git_commit":                  {},
	"git_branch_list":             {},
	"git_checkout":                {},
	"git_init":                    {},
	"git_pull":                    {},
	"git_push":                    {},
	"git_diff":                    {},
	"git_log":                     {},
	"git_show":                    {},
	"git_stash":                   {},
	"git_reset":                   {},
	"git_revert":                  {},
	"git_merge":                   {},
	"git_rebase":                  {},
	"projectstructure":            {},
	"project_structure":           {},
	"ask_user_question":           {},
	"enter_plan_mode":             {},
	"exit_plan_mode":              {},
	"agent":                       {},
	"suggest_memory":              {},
	"web_search":                  {},
	"web_fetch":                   {},
	"enter_worktree":              {},
	"exit_worktree":               {},
	"notebook_edit":               {},
	"document_generate":           {},
	"document_convert":            {},
	"image_generate":              {},
	"video_generate":              {},
	"speech_synthesize":           {},
	"mcp_list_resources":          {},
	"mcp_read_resource":           {},
	"mcp_list_prompts":            {},
	"mcp_get_prompt":              {},
	"powershell":                  {},
	"structured_output":           {},
	"snip":                        {},
	"team_create":                 {},
	"team_delete":                 {},
	"team_send_message":           {},
	"remote_repo_connect":         {},
	"remote_repo_status":          {},
	"remote_repo_clone_or_open":   {},
	"remote_repo_checkout":        {},
	"remote_repo_commit_and_push": {},
	"remote_repo_create_pr":       {},
	"remote_repo_create_mr":       {},
	"remote_repo_disconnect":      {},
}

type Registry struct {
	mu      sync.RWMutex
	roles   map[string]RoleConfig
	aliases map[string]string
}

func NewRegistry(defaults []RoleConfig) (*Registry, error) {
	r := &Registry{
		roles:   make(map[string]RoleConfig),
		aliases: make(map[string]string),
	}
	for _, role := range defaults {
		if err := r.Register(role); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func NewDefaultRegistry() *Registry {
	r, err := NewRegistry(BuiltinRoles())
	if err != nil {
		panic(err)
	}
	return r
}

func LoadRegistry(paths ...string) (*Registry, error) {
	registry := NewDefaultRegistry()
	for _, path := range compactStrings(paths) {
		if err := registry.ApplyJSONFile(path); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func LoadRegistryWithPaths(paths ConfigPaths) (*Registry, error) {
	return LoadRegistry(paths.Ordered()...)
}

func DefaultConfigPaths(workspaceRoot string) ConfigPaths {
	var paths ConfigPaths
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		paths.UserPath = filepath.Join(home, ".eos", "roles.json")
	}
	if root := strings.TrimSpace(workspaceRoot); root != "" {
		paths.ProjectPath = filepath.Join(root, ".eos", "roles.json")
	}
	return paths
}

func (r *Registry) Register(role RoleConfig) error {
	role, err := normalizeRole(role)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.roles[role.ID]; ok {
		role.LegacyAliases = compactStrings(append(existing.LegacyAliases, role.LegacyAliases...))
		for _, alias := range existing.LegacyAliases {
			alias = NormalizeRoleID(alias)
			if r.aliases[alias] == role.ID {
				delete(r.aliases, alias)
			}
		}
	}

	for _, alias := range role.LegacyAliases {
		alias = NormalizeRoleID(alias)
		if alias == "" || alias == role.ID {
			continue
		}
		if _, ok := r.roles[alias]; ok && alias != role.ID {
			return fmt.Errorf("role %q legacy name %q conflicts with role id", role.ID, alias)
		}
		if current, ok := r.aliases[alias]; ok && current != role.ID {
			return fmt.Errorf("role %q legacy name %q conflicts with role %q", role.ID, alias, current)
		}
	}

	r.roles[role.ID] = role
	for _, alias := range role.LegacyAliases {
		alias = NormalizeRoleID(alias)
		if alias != "" && alias != role.ID {
			r.aliases[alias] = role.ID
		}
	}
	return nil
}

func (r *Registry) Resolve(id string) (RoleConfig, bool) {
	key := NormalizeRoleID(id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if target, ok := r.aliases[key]; ok {
		key = target
	}
	role, ok := r.roles[key]
	if !ok {
		return RoleConfig{}, false
	}
	return cloneRole(role), true
}

func (r *Registry) List() []RoleConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RoleConfig, 0, len(r.roles))
	for _, role := range r.roles {
		out = append(out, cloneRole(role))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) ApplyJSON(data []byte) error {
	return r.applyJSON(data, "")
}

func (r *Registry) ApplyJSONFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil
	}
	if err := r.applyJSON(data, filepath.Dir(path)); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (r *Registry) applyJSON(data []byte, baseDir string) error {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	if len(doc.Roles) == 0 {
		return errors.New("role config must contain at least one role")
	}
	if err := validateDocument(doc); err != nil {
		return err
	}
	for _, role := range doc.Roles {
		var err error
		role, err = loadPromptFile(role, baseDir)
		if err != nil {
			return err
		}
		if err := r.Register(role); err != nil {
			return err
		}
	}
	return nil
}

func loadPromptFile(role RoleConfig, baseDir string) (RoleConfig, error) {
	if strings.TrimSpace(role.SystemPrompt) != "" || strings.TrimSpace(role.PromptFile) == "" {
		return role, nil
	}
	path := strings.TrimSpace(role.PromptFile)
	if !filepath.IsAbs(path) && strings.TrimSpace(baseDir) != "" {
		path = filepath.Join(baseDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return RoleConfig{}, fmt.Errorf("role %q prompt_file %q: %w", role.ID, role.PromptFile, err)
	}
	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return RoleConfig{}, fmt.Errorf("role %q prompt_file %q is empty", role.ID, role.PromptFile)
	}
	role.SystemPrompt = prompt
	return role, nil
}

func validateDocument(doc Document) error {
	seenIDs := make(map[string]struct{}, len(doc.Roles))
	seenLegacy := make(map[string]string)
	for _, role := range doc.Roles {
		roleID := NormalizeRoleID(role.ID)
		if roleID == "" {
			return errors.New("role id is required")
		}
		if _, ok := seenIDs[roleID]; ok {
			return fmt.Errorf("duplicate role id %q", roleID)
		}
		seenIDs[roleID] = struct{}{}
		for _, alias := range collectLegacyNames(role) {
			if alias == "" || alias == roleID {
				continue
			}
			if owner, ok := seenLegacy[alias]; ok && owner != roleID {
				return fmt.Errorf("duplicate legacy role name %q", alias)
			}
			seenLegacy[alias] = roleID
		}
	}
	return nil
}

func normalizeRole(role RoleConfig) (RoleConfig, error) {
	role.ID = NormalizeRoleID(role.ID)
	role.Description = strings.TrimSpace(role.Description)
	role.SystemPrompt = strings.TrimSpace(role.SystemPrompt)
	role.PromptFile = strings.TrimSpace(role.PromptFile)
	role.Model = strings.TrimSpace(role.Model)
	role.ReasoningEffort = strings.TrimSpace(role.ReasoningEffort)
	role.LegacyAliases = compactStrings(collectLegacyNames(role))
	role.LegacyNames = nil
	role.AllowedTools = normalizeAllowedTools(role.AllowedTools)
	if role.ContextStrategy == "" {
		role.ContextStrategy = ContextShared
	}
	if err := validateRole(role); err != nil {
		return RoleConfig{}, err
	}
	return role, nil
}

func validateRole(role RoleConfig) error {
	if role.ID == "" {
		return errors.New("role id is required")
	}
	if !roleIDPattern.MatchString(role.ID) {
		return fmt.Errorf("role %q has invalid id", role.ID)
	}
	if role.SystemPrompt == "" && role.PromptFile == "" {
		return fmt.Errorf("role %q needs system_prompt or prompt_file", role.ID)
	}
	switch role.ContextStrategy {
	case "", ContextShared, ContextIndependent, ContextHybrid:
	default:
		return fmt.Errorf("role %q has unknown context_strategy %q", role.ID, role.ContextStrategy)
	}
	for _, tool := range role.AllowedTools {
		if err := validateAllowedTool(tool); err != nil {
			return fmt.Errorf("role %q: %w", role.ID, err)
		}
	}
	for _, alias := range role.LegacyAliases {
		if alias == "" {
			continue
		}
		if !roleIDPattern.MatchString(alias) {
			return fmt.Errorf("role %q has invalid legacy name %q", role.ID, alias)
		}
	}
	return nil
}

func cloneRole(role RoleConfig) RoleConfig {
	role.AllowedTools = append([]string(nil), role.AllowedTools...)
	role.LegacyNames = append([]string(nil), role.LegacyNames...)
	role.LegacyAliases = append([]string(nil), role.LegacyAliases...)
	return role
}

func collectLegacyNames(role RoleConfig) []string {
	items := make([]string, 0, len(role.LegacyAliases)+len(role.LegacyNames))
	items = append(items, role.LegacyAliases...)
	items = append(items, role.LegacyNames...)
	for i := range items {
		items[i] = NormalizeRoleID(items[i])
	}
	return items
}

func compactStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func normalizeAllowedTools(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func validateAllowedTool(value string) error {
	value = normalizeToolToken(value)
	if value == "" {
		return errors.New("allowed_tools contains an empty entry")
	}
	if value == "*" {
		return nil
	}
	base := value
	for _, suffix := range []string{"/*", ".*", "/", "."} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	if base == "" || !isKnownAllowedTool(base) {
		return fmt.Errorf("unsupported allowed_tools entry %q", value)
	}
	return nil
}

func normalizeToolToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "\\", "/")
	return value
}

func isKnownAllowedTool(value string) bool {
	value = normalizeToolToken(value)
	if value == "" {
		return false
	}
	if _, ok := knownAllowedToolSet[value]; ok {
		return true
	}
	return false
}
