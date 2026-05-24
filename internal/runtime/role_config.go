package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"log/slog"
	"strings"

	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/pkg/agentcore"
)

const runtimePlannerPromptMarker = "__eos_runtime_planner_prompt__"

func runtimeBuiltinRoles() []agentcore.Role {
	roles := agentcore.BuiltinRoles()
	out := make([]agentcore.Role, len(roles))
	copy(out, roles)
	for i := range out {
		switch out[i].ID {
		case "planner":
			out[i].Description = "规划复杂任务的执行步骤"
			out[i].SystemPrompt = runtimePlannerPromptMarker
		case "senior-dev":
			out[i].Description = "编写和实现代码功能"
		case "tester":
			out[i].Description = "生成和验证测试用例"
		case "verification":
			out[i].Description = "独立验证实现结果并寻找遗漏风险"
		case "reviewer":
			out[i].Description = "审查代码质量和最佳实践"
		case "explore":
			out[i].Description = "探索和理解代码库结构"
		case "security":
			out[i].Description = "审计代码安全漏洞和风险"
		case "architect":
			out[i].Description = "设计系统架构和技术方案"
			out[i].SystemPrompt = RoleArchitectPrompt
		}
	}
	return out
}

func loadRuntimeRoleRegistry(ctx context.Context) (*agentcore.RoleRegistry, error) {
	registry, err := agentcore.NewRoleRegistry(runtimeBuiltinRoles())
	if err != nil {
		return nil, err
	}
	paths := agentcore.DefaultRoleConfigPaths(runtimeRoleWorkspaceRoot(ctx))
	for _, path := range paths.Ordered() {
		if err := registry.ApplyJSONFile(path); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func runtimeRoleRegistryOrDefault(ctx context.Context) *agentcore.RoleRegistry {
	registry, err := loadRuntimeRoleRegistry(ctx)
	if err == nil {
		return registry
	}
	slog.Warn("runtime.role_config.load_failed", "error", err)
	registry, err = agentcore.NewRoleRegistry(runtimeBuiltinRoles())
	if err != nil {
		slog.Error("runtime.role_config.default_failed", "error", err)
		return agentcore.NewDefaultRoleRegistry()
	}
	return registry
}

func runtimeRoleWorkspaceRoot(ctx context.Context) string {
	return strings.TrimSpace(tools.WorkspaceRootFromContext(ctx))
}

func resolveRuntimeRole(ctx context.Context, roleRef string) (agentcore.Role, bool) {
	return runtimeRoleRegistryOrDefault(ctx).Resolve(roleRef)
}

func resolveRuntimeDefaultRole(roleRef string) (agentcore.Role, bool) {
	registry, err := agentcore.NewRoleRegistry(runtimeBuiltinRoles())
	if err != nil {
		slog.Error("runtime.role_config.default_failed", "error", err)
		return agentcore.Role{}, false
	}
	return registry.Resolve(roleRef)
}

func runtimeCanonicalRoleID(ctx context.Context, roleRef string, fallback string) string {
	if role, ok := resolveRuntimeRole(ctx, roleRef); ok && strings.TrimSpace(role.ID) != "" {
		return role.ID
	}
	if normalized := agentcore.NormalizeRoleID(fallback); normalized != "" {
		return normalized
	}
	return agentcore.NormalizeRoleID(roleRef)
}

func runtimeRoleDescription(roleRef string, fallback string) string {
	if role, ok := resolveRuntimeDefaultRole(roleRef); ok && strings.TrimSpace(role.Description) != "" {
		return role.Description
	}
	return strings.TrimSpace(fallback)
}

func runtimeRoleBasePrompt(ctx context.Context, roleRef string) (string, string) {
	if role, ok := resolveRuntimeRole(ctx, roleRef); ok {
		prompt := strings.TrimSpace(role.SystemPrompt)
		if prompt == runtimePlannerPromptMarker {
			return BuildPlanPromptForStyle(planPromptStyleFromContext(ctx)), role.ID
		}
		if prompt != "" {
			return prompt, role.ID
		}
		return RoleDefaultPrompt, role.ID
	}

	canonical := agentcore.NormalizeRoleID(roleRef)
	switch canonical {
	case "architect":
		return RoleArchitectPrompt, canonical
	case "planner":
		return BuildPlanPromptForStyle(planPromptStyleFromContext(ctx)), canonical
	case "senior-dev":
		return RoleSeniorDevPrompt, canonical
	case "reviewer":
		return RoleReviewerPrompt, canonical
	case "tester":
		return RoleTesterPrompt, canonical
	case "verification":
		return RoleVerificationPrompt, canonical
	default:
		return RoleDefaultPrompt, canonical
	}
}

func runtimeRoleContextStrategy(ctx context.Context, roleRef string, fallback ContextStrategy) ContextStrategy {
	if role, ok := resolveRuntimeRole(ctx, roleRef); ok {
		return roleConfigContextStrategy(role.ContextStrategy, fallback)
	}
	return fallback
}

func runtimeDefaultRoleContextStrategy(roleRef string, fallback ContextStrategy) ContextStrategy {
	if role, ok := resolveRuntimeDefaultRole(roleRef); ok {
		return roleConfigContextStrategy(role.ContextStrategy, fallback)
	}
	return fallback
}

func roleConfigContextStrategy(strategy agentcore.ContextStrategy, fallback ContextStrategy) ContextStrategy {
	switch strategy {
	case agentcore.ContextIndependent:
		return ContextStrategyIndependent
	case agentcore.ContextShared:
		return ContextStrategyShared
	case agentcore.ContextHybrid:
		return ContextStrategyHybrid
	default:
		return fallback
	}
}

func runtimeRoleAllowedTools(ctx context.Context, roleRef string, fallback []string) []string {
	if role, ok := resolveRuntimeRole(ctx, roleRef); ok && len(role.AllowedTools) > 0 {
		return cloneStringSlice(role.AllowedTools)
	}
	return cloneStringSlice(fallback)
}

func runtimeDefaultRoleAllowedTools(roleRef string, fallback []string) []string {
	if role, ok := resolveRuntimeDefaultRole(roleRef); ok && len(role.AllowedTools) > 0 {
		return cloneStringSlice(role.AllowedTools)
	}
	return cloneStringSlice(fallback)
}

func runtimeRoleIncludesMCPToolsInfo(roleID string) bool {
	switch agentcore.NormalizeRoleID(roleID) {
	case "planner", "senior-dev":
		return true
	default:
		return false
	}
}

func cloneStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	copy(out, items)
	return out
}
