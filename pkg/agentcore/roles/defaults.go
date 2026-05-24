package roles

func BuiltinRoles() []RoleConfig {
	return []RoleConfig{
		{
			ID:              "planner",
			Description:     "Plan complex work before execution.",
			SystemPrompt:    BuiltinPlannerPrompt,
			ContextStrategy: ContextShared,
			LegacyAliases:   []string{"plan", "architect-plan"},
		},
		{
			ID:              "senior-dev",
			Description:     "Implement production code changes.",
			SystemPrompt:    BuiltinSeniorDevPrompt,
			ContextStrategy: ContextShared,
			LegacyAliases:   []string{"developer", "senior_dev"},
		},
		{
			ID:              "tester",
			Description:     "Create and run focused tests.",
			SystemPrompt:    BuiltinTesterPrompt,
			AllowedTools:    []string{"read", "write", "glob", "grep"},
			ContextStrategy: ContextIndependent,
		},
		{
			ID:              "verification",
			Description:     "Independently verify implementation results.",
			SystemPrompt:    BuiltinVerificationPrompt,
			AllowedTools:    []string{"read", "glob", "grep", "search", "todo_read", "project_structure"},
			ContextStrategy: ContextIndependent,
			LegacyAliases:   []string{"verify"},
		},
		{
			ID:              "reviewer",
			Description:     "Review code quality and regressions.",
			SystemPrompt:    BuiltinReviewerPrompt,
			AllowedTools:    []string{"read", "diff", "glob", "grep", "search", "todo_read", "project_structure"},
			ContextStrategy: ContextHybrid,
			LegacyAliases:   []string{"review"},
		},
		{
			ID:              "explore",
			Description:     "Explore codebase structure read-only.",
			SystemPrompt:    BuiltinExplorePrompt,
			AllowedTools:    []string{"glob", "grep", "read", "search", "todo_read", "project_structure"},
			ContextStrategy: ContextIndependent,
			LegacyAliases:   []string{"explorer"},
		},
		{
			ID:              "security",
			Description:     "Audit security-sensitive behavior.",
			SystemPrompt:    BuiltinSecurityPrompt,
			AllowedTools:    []string{"glob", "grep", "read", "search", "todo_read", "project_structure"},
			ContextStrategy: ContextHybrid,
		},
		{
			ID:              "architect",
			Description:     "Design architecture and module boundaries.",
			SystemPrompt:    BuiltinArchitectPrompt,
			AllowedTools:    []string{"glob", "grep", "read", "search", "diff", "todo_read", "project_structure"},
			ContextStrategy: ContextHybrid,
		},
	}
}
