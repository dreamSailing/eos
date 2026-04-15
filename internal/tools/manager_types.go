package tools

import (
	"context"
	"fmt"
	"github.com/dreamSailing/vb-coding/internal/mcp"
	"github.com/dreamSailing/vb-coding/internal/pkg/plugins"
	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
	"github.com/dreamSailing/vb-coding/internal/tools/fileops"
	"github.com/dreamSailing/vb-coding/internal/tools/shell"
	"path/filepath"
	"strings"
)

// ToolCall represents a tool invocation
type ToolCall struct {
	ID         string                 `json:"id"`
	Tool       string                 `json:"tool"`
	Parameters map[string]interface{} `json:"parameters"`
}

// Manager is the central orchestrator for all tool operations
type Manager struct {
	fileOps      *fileops.FileOperations
	shell        *shell.Shell
	structured   map[string]func(context.Context, map[string]interface{}) ToolResult
	executor     *ToolCallExecutor
	toolIndex    *ToolIndex
	skillManager *SkillManager
	cache        *ToolCache // 工具输出缓存
	mcpManager   *mcp.Manager
}

// NewManager creates a new tool manager with all handlers registered
func NewManager() *Manager {
	utils.NewLogger("")
	m := &Manager{
		fileOps:   fileops.NewFileOperations(),
		shell:     shell.NewShell(),
		executor:  NewToolCallExecutor(),
		toolIndex: NewToolIndex(),
		cache:     NewToolCache(),
	}
	m.structured = map[string]func(context.Context, map[string]interface{}) ToolResult{
		ToolRead:             m.readStructured,
		ToolFS:               m.fsStructured,
		ToolEdit:             m.editStructured,
		ToolHistory:          m.historyStructured,
		ToolSearch:           m.searchStructured,
		ToolToolSearch:       m.toolSearchStructured,
		ToolSkill:            m.skillStructured,
		ToolTimeNow:          m.timeNowStructured,
		ToolUserConfirm:      m.userConfirmStructured,
		ToolUserInput:        m.userInputStructured,
		ToolUserSelect:       m.userSelectStructured,
		ToolBash:             m.bashStructured,
		ToolBashSession:      m.bashSessionStructured,
		ToolBGTask:           m.bgTaskStructured,
		ToolGitStatus:        m.gitStatusStructured,
		ToolGitAdd:           m.gitAddStructured,
		ToolGitCommit:        m.gitCommitStructured,
		ToolGitBranchList:    m.gitBranchListStructured,
		ToolGitCheckout:      m.gitCheckoutStructured,
		ToolGitInit:          m.gitInitStructured,
		ToolGitPull:          m.gitPullStructured,
		ToolGitPush:          m.gitPushStructured,
		ToolGitDiff:          m.gitDiffStructured,
		ToolGitLog:           m.gitLogStructured,
		ToolGitShow:          m.gitShowStructured,
		ToolGitStash:         m.gitStashStructured,
		ToolGitReset:         m.gitResetStructured,
		ToolGitRevert:        m.gitRevertStructured,
		ToolGitMerge:         m.gitMergeStructured,
		ToolGitRebase:        m.gitRebaseStructured,
		ToolPlanSteps:        m.planStepsStructured,
		ToolTodoRead:         m.todoReadStructured,
		ToolTodoWrite:        m.todoWriteStructured,
		ToolProjectStructure: m.projectStructureStructured,
		ToolMCPStatus:        m.mcpStatusStructured,
		ToolSkillsList:       m.skillsListStructured,
		ToolAskUserQuestion:  m.askUserQuestionStructured,
		ToolEnterPlanMode:    m.enterPlanModeStructured,
		ToolExitPlanMode:     m.exitPlanModeStructured,
		ToolAgent:            m.agentToolStructured,
	}
	m.LoadPluginsFromRegistry(plugins.DefaultRegistry())
	return m
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Tool    string                 `json:"tool"`
	Status  string                 `json:"status"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Display string                 `json:"display,omitempty"`
	Ts      int64                  `json:"ts,omitempty"`
}

func summarizeDisplay(r ToolResult) string {
	if strings.TrimSpace(r.Display) != "" {
		return r.Display
	}
	if r.Error != "" {
		return "Error: " + r.Error
	}
	switch r.Tool {
	case "read":
		p, _ := r.Data["path"].(string)
		switch v := r.Data["bytes"].(type) {
		case int:
			return fmt.Sprintf("Read file: %s (%d bytes)", filepath.ToSlash(p), v)
		case float64:
			return fmt.Sprintf("Read file: %s (%d bytes)", filepath.ToSlash(p), int(v))
		default:
			return fmt.Sprintf("Read: %s", filepath.ToSlash(p))
		}
	default:
		return fmt.Sprintf("Tool: %s, Status: %s", r.Tool, r.Status)
	}
}
