package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/browser"
	"github.com/dreamSailing/eos/internal/mcp"
	"github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools/fileops"
	"github.com/dreamSailing/eos/internal/tools/shell"
	"path/filepath"
	"strings"
)

// ToolCall represents a tool invocation
type ToolCall struct {
	ID         string                 `json:"id"`
	Tool       string                 `json:"tool"`
	Parameters map[string]interface{} `json:"parameters"`
}

// HookRunner defines the interface for executing pre/post tool use hooks
type HookRunner interface {
	PreToolUse(ctx context.Context, toolName string, input map[string]any) (bool, map[string]any, error)
	PostToolUse(ctx context.Context, toolName string, input map[string]any, result map[string]any) error
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
	browserRT    *browser.BuiltinRuntime
	hookRunner   HookRunner // Hook integration for tool execution

	// OnReactiveCompact is called when a tool result exceeds the reactive compaction threshold
	OnReactiveCompact func()
	// AskToolApproval is called when a tool matches an "ask" permission rule.
	// Returns true if the user approves, false to deny.
	AskToolApproval func(toolName string) bool
	// resultBudget tracks aggregate tool result size per turn
	resultBudget *ToolResultBudget
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
		ToolRead:                    m.readStructured,
		ToolFS:                      m.fsStructured,
		ToolEdit:                    m.editStructured,
		ToolHistory:                 m.historyStructured,
		ToolSearch:                  m.searchStructured,
		ToolToolSearch:              m.toolSearchStructured,
		ToolSkill:                   m.skillStructured,
		ToolCreateSkill:             m.createSkillStructured,
		ToolDocumentGenerate:        m.documentGenerateStructured,
		ToolDocumentConvert:         m.documentConvertStructured,
		ToolImageGenerate:           m.imageGenerateStructured,
		ToolVideoGenerate:           m.videoGenerateStructured,
		ToolSpeechSynthesize:        m.speechSynthesizeStructured,
		ToolTimeNow:                 m.timeNowStructured,
		ToolUserConfirm:             m.userConfirmStructured,
		ToolUserInput:               m.userInputStructured,
		ToolUserSelect:              m.userSelectStructured,
		ToolBash:                    m.bashStructured,
		ToolBashSession:             m.bashSessionStructured,
		ToolBGTask:                  m.bgTaskStructured,
		ToolGitStatus:               m.gitStatusStructured,
		ToolGitAdd:                  m.gitAddStructured,
		ToolGitCommit:               m.gitCommitStructured,
		ToolGitBranchList:           m.gitBranchListStructured,
		ToolGitCheckout:             m.gitCheckoutStructured,
		ToolGitInit:                 m.gitInitStructured,
		ToolGitPull:                 m.gitPullStructured,
		ToolGitPush:                 m.gitPushStructured,
		ToolGitDiff:                 m.gitDiffStructured,
		ToolGitLog:                  m.gitLogStructured,
		ToolGitShow:                 m.gitShowStructured,
		ToolGitStash:                m.gitStashStructured,
		ToolGitReset:                m.gitResetStructured,
		ToolGitRevert:               m.gitRevertStructured,
		ToolGitMerge:                m.gitMergeStructured,
		ToolGitRebase:               m.gitRebaseStructured,
		ToolPlanSteps:               m.planStepsStructured,
		ToolTodoRead:                m.todoReadStructured,
		ToolTodoWrite:               m.todoWriteStructured,
		ToolProjectStructure:        m.projectStructureStructured,
		ToolMCPStatus:               m.mcpStatusStructured,
		ToolBrowserStatus:           m.browserStatusStructured,
		ToolBrowserNavigate:         m.browserNavigateStructured,
		ToolBrowserSnapshot:         m.browserSnapshotStructured,
		ToolBrowserBack:             m.browserBackStructured,
		ToolBrowserForward:          m.browserForwardStructured,
		ToolBrowserClick:            m.browserClickStructured,
		ToolBrowserHover:            m.browserHoverStructured,
		ToolBrowserType:             m.browserTypeStructured,
		ToolBrowserPressKey:         m.browserPressKeyStructured,
		ToolBrowserSelect:           m.browserSelectStructured,
		ToolBrowserWait:             m.browserWaitStructured,
		ToolBrowserScroll:           m.browserScrollStructured,
		ToolBrowserScreenshot:       m.browserScreenshotStructured,
		ToolBrowserConsole:          m.browserConsoleStructured,
		ToolBrowserNetwork:          m.browserNetworkStructured,
		ToolSkillsList:              m.skillsListStructured,
		ToolAskUserQuestion:         m.askUserQuestionStructured,
		ToolEnterPlanMode:           m.enterPlanModeStructured,
		ToolExitPlanMode:            m.exitPlanModeStructured,
		ToolAgent:                   m.agentToolStructured,
		ToolSuggestMemory:           m.suggestMemoryStructured,
		ToolWebSearch:               m.webSearchStructured,
		ToolWebFetch:                m.webFetchStructured,
		ToolEnterWorktree:           m.enterWorktreeStructured,
		ToolExitWorktree:            m.exitWorktreeStructured,
		ToolNotebookEdit:            m.notebookEditStructured,
		ToolMCPListResources:        m.mcpListResourcesStructured,
		ToolMCPReadResource:         m.mcpReadResourceStructured,
		ToolMCPListPrompts:          m.mcpListPromptsStructured,
		ToolMCPGetPrompt:            m.mcpGetPromptStructured,
		ToolPowerShell:              m.powerShellStructured,
		ToolStructuredOutput:        m.structuredOutputStructured,
		ToolSnip:                    m.snipStructured,
		ToolTeamCreate:              m.teamCreateStructured,
		ToolTeamDelete:              m.teamDeleteStructured,
		ToolTeamSendMsg:             m.teamSendMessageStructured,
		ToolRemoteRepoConnect:       m.remoteRepoConnectStructured,
		ToolRemoteRepoStatus:        m.remoteRepoStatusStructured,
		ToolRemoteRepoCloneOrOpen:   m.remoteRepoCloneOrOpenStructured,
		ToolRemoteRepoCheckout:      m.remoteRepoCheckoutStructured,
		ToolRemoteRepoCommitAndPush: m.remoteRepoCommitAndPushStructured,
		ToolRemoteRepoCreatePR:      m.remoteRepoCreatePRStructured,
		ToolRemoteRepoCreateMR:      m.remoteRepoCreateMRStructured,
		ToolRemoteRepoDisconnect:    m.remoteRepoDisconnectStructured,
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
