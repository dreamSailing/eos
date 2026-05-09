package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"log/slog"
	"strings"
)

func (m *Manager) skillStructured(ctx context.Context, params map[string]any) ToolResult {
	if m.skillManager == nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSkill,
			Status: "error",
			Error:  "skill manager not initialized",
		}
	}

	command, _ := params["command"].(string)
	if command == "" {
		available := m.skillManager.FormatSkillsForPrompt()
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSkill,
			Status: "error",
			Error:  "missing required parameter: command. Available skills:\n" + available,
		}
	}

	s, ok := m.skillManager.Get(command)
	if !ok || s == nil {
		available := m.skillManager.FormatSkillsForPrompt()
		errorMsg := fmt.Sprintf("skill not found: %s\n\nAvailable skills:\n%s", command, available)
		if m.skillManager.IsDisabled(command) {
			errorMsg = fmt.Sprintf("skill disabled: %s\n\nAvailable skills:\n%s", command, available)
		}

		suggestion := m.suggestSkill(command)
		if suggestion != "" {
			errorMsg += fmt.Sprintf("\n\nDid you mean: %s?", suggestion)
		}

		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSkill,
			Status: "error",
			Error:  errorMsg,
		}
	}

	if s.DisableModelInvocation {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSkill,
			Status: "error",
			Error:  "skill is not model-invocable: " + command,
		}
	}

	if strings.EqualFold(strings.TrimSpace(s.Context), "fork") {
		ctxMod := s.GetContextModifier()
		data := map[string]any{
			"skill_name": command,
			"fork":       true,
			"context":    "fork",
			"agent":      strings.TrimSpace(s.Agent),
			"prompt":     s.RenderPrompt(""),
			"continue":   true,
		}
		if ctxMod != nil {
			if len(ctxMod.AllowedTools) > 0 {
				data["allowed_tools"] = ctxMod.AllowedTools
			}
			if ctxMod.ModelOverride != "" {
				data["model_override"] = ctxMod.ModelOverride
			}
		}
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolSkill,
			Status:  "success",
			Data:    data,
			Display: fmt.Sprintf("Skill '%s' running (fork)", command),
		}
	}

	messages, ctxMod, err := m.skillManager.InjectSkillWithArguments(ctx, command, "")
	if err != nil {
		slog.Error("tools.skill.inject.error",
			"component", utils.ComponentTool,
			"skill", command,
			"error", err,
		)

		available := m.skillManager.FormatSkillsForPrompt()
		errorMsg := fmt.Sprintf("skill not found: %s\n\nAvailable skills:\n%s", command, available)
		if m.skillManager.IsDisabled(command) || strings.Contains(strings.ToLower(err.Error()), "skill disabled") {
			errorMsg = fmt.Sprintf("skill disabled: %s\n\nAvailable skills:\n%s", command, available)
		}

		suggestion := m.suggestSkill(command)
		if suggestion != "" {
			errorMsg += fmt.Sprintf("\n\nDid you mean: %s?", suggestion)
		}

		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolSkill,
			Status: "error",
			Error:  errorMsg,
		}
	}

	data := map[string]any{
		"skill_name": command,
		"injected":   true,
	}

	if ctxMod != nil {
		if len(ctxMod.AllowedTools) > 0 {
			data["allowed_tools"] = ctxMod.AllowedTools
		}
		if ctxMod.ModelOverride != "" {
			data["model_override"] = ctxMod.ModelOverride
		}
	}

	if len(messages) > 0 {
		data["messages"] = messages
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolSkill,
		Status:  "success",
		Data:    data,
		Display: fmt.Sprintf("Skill '%s' activated", command),
	}
}

func (m *Manager) suggestSkill(name string) string {
	skills := m.skillManager.List()
	if len(skills) == 0 {
		return ""
	}

	nameLower := strings.ToLower(name)
	for _, skill := range skills {
		if strings.Contains(strings.ToLower(skill.Name), nameLower) ||
			strings.Contains(nameLower, strings.ToLower(skill.Name)) {
			return skill.Name
		}
	}

	return ""
}
