package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/skills"

	"gopkg.in/yaml.v3"
)

type createSkillFrontmatter struct {
	Name                   string   `yaml:"name"`
	Description            string   `yaml:"description,omitempty"`
	AllowedTools           []string `yaml:"allowed-tools,omitempty"`
	Model                  string   `yaml:"model,omitempty"`
	ArgumentHint           string   `yaml:"argument-hint,omitempty"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty"`
	UserInvocable          *bool    `yaml:"user-invocable,omitempty"`
	Context                string   `yaml:"context,omitempty"`
	Agent                  string   `yaml:"agent,omitempty"`
	Keywords               []string `yaml:"keywords,omitempty"`
	License                string   `yaml:"license,omitempty"`
	Version                string   `yaml:"version,omitempty"`
}

func (m *Manager) createSkillStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	request, _ := params["request"].(string)
	request = strings.TrimSpace(request)
	if request == "" {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: "request parameter is required"}
	}

	scope, _ := params["scope"].(string)
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolCreateSkill,
			Status: "error",
			Error:  "scope parameter is required; analyze whether this should be a workspace or user skill, ask the user where to create it, then retry with scope=workspace|user",
		}
	}
	if scope != "workspace" && scope != "user" {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: "scope must be workspace or user"}
	}

	root, err := resolveCreateSkillRoot(ctx, scope)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: err.Error()}
	}
	rootRes := utils.ResolvePathUnder(root, filepath.ToSlash(filepath.Join(".eos", "skills")))
	if !rootRes.IsValid {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: rootRes.ErrMsg}
	}

	overwrite := boolParam(params, "overwrite", false)
	activate := boolParam(params, "activate", true)
	includeScripts := boolParam(params, "include_scripts", false)
	includeReferences := boolParam(params, "include_references", false)
	includeAssets := boolParam(params, "include_assets", false)

	rawName, _ := params["name"].(string)
	rawName = strings.TrimSpace(rawName)
	descriptionOverride, _ := params["description"].(string)
	descriptionOverride = strings.TrimSpace(descriptionOverride)

	doc, usedAI := generateSkillDocumentWithFallback(request, rawName)
	parsed, body := parseSkillDocument(doc)
	if strings.TrimSpace(body) == "" {
		body = fallbackSkillBody(request)
	}

	resolvedName := sanitizeSkillDirectoryName(rawName)
	if resolvedName == "" {
		resolvedName = sanitizeSkillDirectoryName(parsed.Name)
	}
	if resolvedName == "" {
		resolvedName = suggestSkillName(request)
	}
	if resolvedName == "" {
		resolvedName = "custom-skill"
	}

	normalized := normalizeGeneratedSkillDocument(parsed, params, request, resolvedName, descriptionOverride)
	skillDirRes := utils.ResolvePathUnder(root, filepath.ToSlash(filepath.Join(".eos", "skills", resolvedName)))
	if !skillDirRes.IsValid {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: skillDirRes.ErrMsg}
	}
	skillPathRes := utils.ResolvePathUnder(root, filepath.ToSlash(filepath.Join(".eos", "skills", resolvedName, "SKILL.md")))
	if !skillPathRes.IsValid {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: skillPathRes.ErrMsg}
	}

	if exists, _, _ := m.fileOps.PathExists(skillPathRes.AbsPath); exists && !overwrite {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolCreateSkill,
			Status: "error",
			Error:  fmt.Sprintf("skill already exists: %s (set overwrite=true to replace it)", filepath.ToSlash(skillPathRes.RelPath)),
		}
	}

	rendered, err := renderSkillDocument(normalized, body)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: err.Error()}
	}
	if err := m.fileOps.CreateDirectory(skillDirRes.AbsPath); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: err.Error()}
	}
	if err := m.fileOps.WriteFile(skillPathRes.AbsPath, rendered); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: err.Error()}
	}

	created := []string{filepath.ToSlash(skillPathRes.RelPath)}
	if includeScripts {
		if rel, e := createSkillSubdir(m, root, resolvedName, "scripts"); e != nil {
			return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: e.Error()}
		} else {
			created = append(created, rel)
		}
	}
	if includeReferences {
		if rel, e := createSkillSubdir(m, root, resolvedName, "references"); e != nil {
			return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: e.Error()}
		} else {
			created = append(created, rel)
		}
	}
	if includeAssets {
		if rel, e := createSkillSubdir(m, root, resolvedName, "assets"); e != nil {
			return ToolResult{Type: "tool_result", Tool: ToolCreateSkill, Status: "error", Error: e.Error()}
		} else {
			created = append(created, rel)
		}
	}

	reloaded := false
	if activate && m.skillManager != nil {
		if err := m.skillManager.ReloadPreserveActive(); err != nil {
			return ToolResult{
				Type:   "tool_result",
				Tool:   ToolCreateSkill,
				Status: "error",
				Error:  fmt.Sprintf("skill created but reload failed: %v", err),
				Data: map[string]interface{}{
					"name":          normalized.Name,
					"scope":         scope,
					"path":          filepath.ToSlash(skillDirRes.RelPath),
					"skill_md_path": filepath.ToSlash(skillPathRes.RelPath),
					"created_files": created,
				},
			}
		}
		reloaded = true
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolCreateSkill,
		Status:  "success",
		Display: fmt.Sprintf("Created skill %q at %s", normalized.Name, filepath.ToSlash(skillPathRes.RelPath)),
		Data: map[string]interface{}{
			"name":               normalized.Name,
			"scope":              scope,
			"path":               filepath.ToSlash(skillDirRes.RelPath),
			"skill_md_path":      filepath.ToSlash(skillPathRes.RelPath),
			"created_files":      created,
			"reloaded":           reloaded,
			"used_ai_generation": usedAI,
		},
	}
}

func resolveCreateSkillRoot(ctx context.Context, scope string) (string, error) {
	switch scope {
	case "workspace":
		root := strings.TrimSpace(WorkspaceRootFromContext(ctx))
		if root == "" {
			return "", fmt.Errorf("workspace root is unavailable; ask the user where to create the skill and retry with an active workspace")
		}
		return root, nil
	case "user":
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			if err == nil {
				err = fmt.Errorf("user home directory is unavailable")
			}
			return "", err
		}
		return strings.TrimSpace(home), nil
	default:
		return "", fmt.Errorf("scope must be workspace or user")
	}
}

func createSkillSubdir(m *Manager, root, skillName, subdir string) (string, error) {
	res := utils.ResolvePathUnder(root, filepath.ToSlash(filepath.Join(".eos", "skills", skillName, subdir)))
	if !res.IsValid {
		return "", fmt.Errorf("%s", res.ErrMsg)
	}
	if err := m.fileOps.CreateDirectory(res.AbsPath); err != nil {
		return "", err
	}
	return filepath.ToSlash(res.RelPath), nil
}

func normalizeGeneratedSkillDocument(parsed skills.Skill, params map[string]interface{}, request, resolvedName, descriptionOverride string) createSkillFrontmatter {
	doc := createSkillFrontmatter{
		Name:                   resolvedName,
		Description:            strings.TrimSpace(parsed.Description),
		AllowedTools:           uniqueNonEmptyStrings(parsed.AllowedTools.Values()),
		Model:                  strings.TrimSpace(parsed.Model),
		ArgumentHint:           strings.TrimSpace(parsed.ArgumentHint),
		DisableModelInvocation: parsed.DisableModelInvocation,
		UserInvocable:          parsed.UserInvocable,
		Context:                strings.TrimSpace(parsed.Context),
		Agent:                  strings.TrimSpace(parsed.Agent),
		Keywords:               uniqueNonEmptyStrings(parsed.Keywords),
		License:                strings.TrimSpace(parsed.License),
		Version:                strings.TrimSpace(parsed.Version),
	}

	if doc.Description == "" {
		doc.Description = strings.TrimSpace(descriptionOverride)
	}
	if doc.Description == "" {
		doc.Description = buildFallbackDescription(request)
	}

	if v, ok := params["allowed_tools"]; ok {
		doc.AllowedTools = uniqueNonEmptyStrings(toStringSlice(v))
	}
	if s, ok := params["model"].(string); ok && strings.TrimSpace(s) != "" {
		doc.Model = strings.TrimSpace(s)
	}
	if s, ok := params["argument_hint"].(string); ok && strings.TrimSpace(s) != "" {
		doc.ArgumentHint = strings.TrimSpace(s)
	}
	if v, ok := params["user_invocable"].(bool); ok {
		b := v
		doc.UserInvocable = &b
	}
	if s, ok := params["context"].(string); ok && strings.TrimSpace(s) != "" {
		doc.Context = strings.TrimSpace(s)
	}
	if s, ok := params["agent"].(string); ok && strings.TrimSpace(s) != "" {
		doc.Agent = strings.TrimSpace(s)
	}
	if v, ok := params["keywords"]; ok {
		doc.Keywords = uniqueNonEmptyStrings(toStringSlice(v))
	}
	if strings.TrimSpace(descriptionOverride) != "" {
		doc.Description = strings.TrimSpace(descriptionOverride)
	}
	return doc
}

func renderSkillDocument(frontmatter createSkillFrontmatter, body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("skill body is empty")
	}
	raw, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", fmt.Errorf("failed to render skill frontmatter: %w", err)
	}
	return "---\n" + strings.TrimSpace(string(raw)) + "\n---\n\n" + body + "\n", nil
}

func generateSkillDocumentWithFallback(request, name string) (string, bool) {
	doc, err := generateSkillDocumentWithModel(request, name)
	if err == nil && strings.TrimSpace(doc) != "" {
		return doc, true
	}
	return buildFallbackSkillDocument(request, name), false
}

func generateSkillDocumentWithModel(request, name string) (string, error) {
	apiKey, base, model := ai.ResolveAPISettings()
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(base) == "" || strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("model generation unavailable")
	}

	promptName := strings.TrimSpace(name)
	if promptName == "" {
		promptName = "从需求中自行推导"
	}
	systemPrompt := `你是 skill 生成器。请输出一个完整、可落盘的 SKILL.md 文档，不要添加解释。

要求：
- 输出必须是原始 Markdown，不要使用代码块包裹
- 必须包含 YAML frontmatter，以 --- 开始和结束
- frontmatter 至少包含 name 和 description
- 可选字段只能使用这些键：allowed-tools, model, argument-hint, user-invocable, context, agent, keywords
- 正文必须清晰说明：何时使用、工作步骤、输入/输出、边界与注意事项
- 内容应适合作为可执行 agent skill，而不是产品宣传文案`
	userPrompt := fmt.Sprintf("名称偏好：%s\n用户需求：%s", promptName, strings.TrimSpace(request))

	body := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	bs, _ := json.Marshal(body)
	reqCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, "POST", strings.TrimRight(base, "/")+"/v1/chat/completions", bytes.NewReader(bs))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	retryPolicy := utils.RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2,
	}
	result := utils.DoHTTPRetryWithClient(reqCtx, http.DefaultClient, req, retryPolicy)
	if result.Error != nil {
		return "", result.Error
	}
	if result.Response == nil {
		return "", fmt.Errorf("empty model response")
	}
	defer func() { _ = result.Response.Body.Close() }()

	rb, _ := io.ReadAll(result.Response.Body)
	if result.Response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("model request failed: %s", strings.TrimSpace(string(rb)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("model returned no choices")
	}
	content := cleanupModelSkillDocument(out.Choices[0].Message.Content)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("model returned empty content")
	}
	return content, nil
}

func buildFallbackSkillDocument(request, name string) string {
	resolvedName := sanitizeSkillDirectoryName(name)
	if resolvedName == "" {
		resolvedName = suggestSkillName(request)
	}
	if resolvedName == "" {
		resolvedName = "custom-skill"
	}
	frontmatter := createSkillFrontmatter{
		Name:        resolvedName,
		Description: buildFallbackDescription(request),
	}
	body, _ := renderSkillDocument(frontmatter, fallbackSkillBody(request))
	return body
}

func buildFallbackDescription(request string) string {
	summary := strings.Join(strings.Fields(strings.TrimSpace(request)), " ")
	if len(summary) > 96 {
		summary = strings.TrimSpace(summary[:96]) + "..."
	}
	if summary == "" {
		return "Generated skill"
	}
	return summary
}

func fallbackSkillBody(request string) string {
	return strings.TrimSpace(fmt.Sprintf(`# When To Use

Use this skill when the user needs help with the following goal:

%s

# Workflow

1. Clarify the user's exact intent and success criteria.
2. Inspect the relevant repository context before making changes.
3. Produce the requested result with minimal, targeted edits.
4. Verify the outcome and report concrete results.

# Output Requirements

- Focus on the requested deliverable.
- Keep the work grounded in the repository context.
- Call out risks, assumptions, and any verification that was performed.`, strings.TrimSpace(request)))
}

func parseSkillDocument(raw string) (skills.Skill, string) {
	text := strings.ReplaceAll(strings.TrimSpace(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return skills.Skill{}, text
	}
	rest := text[len("---\n"):]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return skills.Skill{}, text
	}
	frontmatter := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(strings.TrimPrefix(rest[idx+len("\n---"):], "\n"))
	var skill skills.Skill
	_ = yaml.Unmarshal([]byte(frontmatter), &skill)
	return skill, body
}

func cleanupModelSkillDocument(raw string) string {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```markdown")
	text = strings.TrimPrefix(text, "```md")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}

var nonNameChars = regexp.MustCompile(`[^a-z0-9]+`)

func suggestSkillName(request string) string {
	text := strings.ToLower(strings.TrimSpace(request))
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "_", "-")
	text = strings.ReplaceAll(text, "/", "-")
	text = nonNameChars.ReplaceAllString(text, "-")
	text = strings.Trim(text, "-")
	if text == "" {
		return ""
	}
	parts := strings.Split(text, "-")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
		if len(filtered) >= 6 {
			break
		}
	}
	return strings.Join(filtered, "-")
}

func sanitizeSkillDirectoryName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = nonNameChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func boolParam(params map[string]interface{}, key string, def bool) bool {
	if params == nil {
		return def
	}
	if v, ok := params[key].(bool); ok {
		return v
	}
	return def
}
