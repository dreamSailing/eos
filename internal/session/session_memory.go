package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	ai "github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/memory"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

const (
	// Default thresholds for session memory extraction
	defaultMinMessageTokensToInit      = 8000
	defaultMinTokensBetweenUpdate      = 4000
	defaultToolCallsBetweenUpdates     = 15
	defaultMaxSectionLength            = 2000
	defaultMaxTotalSessionMemoryTokens = 12000
)

// SessionMemoryConfig holds configuration for session memory extraction
type SessionMemoryConfig struct {
	MinMessageTokensToInit int
	MinTokensBetweenUpdate int
	ToolCallsBetweenUpdate int
}

// DefaultSessionMemoryConfig returns the default session memory config
func DefaultSessionMemoryConfig() *SessionMemoryConfig {
	return &SessionMemoryConfig{
		MinMessageTokensToInit: defaultMinMessageTokensToInit,
		MinTokensBetweenUpdate: defaultMinTokensBetweenUpdate,
		ToolCallsBetweenUpdate: defaultToolCallsBetweenUpdates,
	}
}

// SessionMemoryManager manages session memory extraction
type SessionMemoryManager struct {
	mu                       sync.RWMutex
	config                   *SessionMemoryConfig
	initialized              bool
	lastExtractionTokenCount int
	lastExtractionTime       time.Time
	lastMemoryMessageID      string
	extractionInProgress     bool
}

// NewSessionMemoryManager creates a new session memory manager
func NewSessionMemoryManager() *SessionMemoryManager {
	return &SessionMemoryManager{
		config:      DefaultSessionMemoryConfig(),
		initialized: false,
	}
}

// MarkInitialized marks session memory as initialized
func (sm *SessionMemoryManager) MarkInitialized() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.initialized = true
}

// IsInitialized returns whether session memory has been initialized
func (sm *SessionMemoryManager) IsInitialized() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.initialized
}

// ShouldExtractMemory determines if it's time to extract session memory
func (sm *SessionMemoryManager) ShouldExtractMemory(messages []ai.Message, currentTokenCount int) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check initialization threshold
	if !sm.initialized {
		if currentTokenCount < sm.config.MinMessageTokensToInit {
			return false
		}
		sm.initialized = true
	}

	// Check if extraction is already in progress
	if sm.extractionInProgress {
		return false
	}

	// Check token threshold (growth since last extraction)
	tokenGrowth := currentTokenCount - sm.lastExtractionTokenCount
	if tokenGrowth < sm.config.MinTokensBetweenUpdate {
		return false
	}

	// Check tool call threshold
	startIndex := 0
	if sm.lastMemoryMessageID != "" {
		// Find the index of last extraction point
		for i, m := range messages {
			// Use content hash as proxy for ID
			if m.Content == sm.lastMemoryMessageID {
				startIndex = i + 1
				break
			}
		}
	}
	toolCallsSinceLastUpdate := sm.countToolCallsSince(messages, startIndex)
	if toolCallsSinceLastUpdate < sm.config.ToolCallsBetweenUpdate {
		return false
	}

	// Check if last assistant turn has no tool calls (safe to extract)
	if sm.hasToolCallsInLastAssistantTurn(messages) {
		// Not safe to extract now, but token threshold is met
		return false
	}

	return true
}

func (sm *SessionMemoryManager) countToolCallsSince(messages []ai.Message, sinceIndex int) int {
	toolCallCount := 0

	for i := sinceIndex; i < len(messages); i++ {
		m := messages[i]
		if m.Role == "assistant" && strings.Contains(m.Content, "tool_use") {
			toolCallCount++
		}
	}

	return toolCallCount
}

func (sm *SessionMemoryManager) hasToolCallsInLastAssistantTurn(messages []ai.Message) bool {
	if len(messages) == 0 {
		return false
	}

	// Find last assistant message
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Role == "assistant" {
			// Check if this message has tool calls
			return strings.Contains(m.Content, "tool_use")
		}
	}
	return false
}

// SetExtractionInProgress sets whether extraction is in progress
func (sm *SessionMemoryManager) SetExtractionInProgress(inProgress bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.extractionInProgress = inProgress
}

// RecordExtraction records an extraction event
func (sm *SessionMemoryManager) RecordExtraction(tokenCount int, lastMessageContent string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastExtractionTokenCount = tokenCount
	sm.lastExtractionTime = time.Now()
	sm.lastMemoryMessageID = lastMessageContent // Use content as ID proxy
}

// GetMemoryDir returns the session memory directory path
func GetMemoryDir(rootDir string) string {
	return filepath.Join(rootDir, ".eos", "session-memory")
}

// GetMemoryPath returns the session memory file path
func GetMemoryPath(rootDir string) string {
	return filepath.Join(GetMemoryDir(rootDir), "session.md")
}

// EnsureMemoryDir ensures the session memory directory exists
func EnsureMemoryDir(rootDir string) error {
	dir := GetMemoryDir(rootDir)
	return os.MkdirAll(dir, 0700)
}

// SetupMemoryFile creates the memory file with template if it doesn't exist
func SetupMemoryFile(rootDir string) (string, string, error) {
	if err := EnsureMemoryDir(rootDir); err != nil {
		return "", "", err
	}

	memoryPath := GetMemoryPath(rootDir)

	// Create the file if it doesn't exist
	_, err := os.Stat(memoryPath)
	if os.IsNotExist(err) {
		template := GetDefaultSessionMemoryTemplate()
		if err := os.WriteFile(memoryPath, []byte(template), 0600); err != nil {
			return "", "", err
		}
	}

	// Read current content
	content, err := os.ReadFile(memoryPath)
	if err != nil {
		return "", "", err
	}

	return memoryPath, string(content), nil
}

// GetSessionMemoryUpdatePrompt builds the prompt for updating session memory
func GetSessionMemoryUpdatePrompt(currentNotes string, notesPath string) string {
	prompt := GetDefaultSessionMemoryUpdatePrompt()

	// Substitute variables
	prompt = strings.ReplaceAll(prompt, "{{currentNotes}}", currentNotes)
	prompt = strings.ReplaceAll(prompt, "{{notesPath}}", notesPath)

	// Add section size warnings if needed
	sectionWarnings := analyzeAndWarnSections(currentNotes)
	if sectionWarnings != "" {
		prompt += sectionWarnings
	}

	return prompt
}

// analyzeAndWarnSections checks section sizes and returns warnings if needed (Chinese)
func analyzeAndWarnSections(content string) string {
	sections := analyzeSectionSizes(content)
	totalTokens := estimateTokenCount(content)

	var warnings []string
	isOverBudget := totalTokens > defaultMaxTotalSessionMemoryTokens

	for section, tokens := range sections {
		if tokens > defaultMaxSectionLength {
			warnings = append(warnings, "- \""+section+"\" 约 "+strconv.Itoa(tokens)+" tokens（限制："+strconv.Itoa(defaultMaxSectionLength)+"）")
		}
	}

	if len(warnings) == 0 && !isOverBudget {
		return ""
	}

	var result strings.Builder
	if isOverBudget {
		result.WriteString("\n\n严重：会话记忆文件当前约 ")
		result.WriteString(strconv.Itoa(totalTokens))
		result.WriteString(" tokens，超过了最大限制 ")
		result.WriteString(strconv.Itoa(defaultMaxTotalSessionMemoryTokens))
		result.WriteString(" tokens。你必须压缩文件以符合此预算。")
	}

	if len(warnings) > 0 {
		if isOverBudget {
			result.WriteString("\n需要压缩的超大部分：\n")
		} else {
			result.WriteString("\n重要：以下部分超过每部分限制，必须压缩：\n")
		}
		result.WriteString(strings.Join(warnings, "\n"))
	}

	return result.String()
}

// analyzeSectionSizes returns token counts for each section
func analyzeSectionSizes(content string) map[string]int {
	sections := make(map[string]int)
	lines := strings.Split(content, "\n")

	var currentSection string
	var currentContent []string

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			if currentSection != "" && len(currentContent) > 0 {
				sectionContent := strings.TrimSpace(strings.Join(currentContent, "\n"))
				sections[currentSection] = estimateTokenCount(sectionContent)
			}
			currentSection = line
			currentContent = []string{}
		} else {
			currentContent = append(currentContent, line)
		}
	}

	if currentSection != "" && len(currentContent) > 0 {
		sectionContent := strings.TrimSpace(strings.Join(currentContent, "\n"))
		sections[currentSection] = estimateTokenCount(sectionContent)
	}

	return sections
}

// estimateTokenCount rough estimate of token count
func estimateTokenCount(content string) int {
	// Rough estimation: ~4 characters per token
	return len(content) / 4
}

// ============================================================================
// Templates
// ============================================================================

// GetDefaultSessionMemoryTemplate returns the default session memory template (Chinese)
func GetDefaultSessionMemoryTemplate() string {
	return `# 会话标题
_简短且有特色的 5-10 个词描述会话标题。信息密集，无填充词_

# 当前状态
_当前正在积极做什么？未完成的待处理任务。下一步immediate的步骤_

# 任务说明
_用户要求构建什么？任何设计决策或其他解释性上下文_

# 文件和函数
_重要的文件有哪些？简而言之，它们包含什么，为什么相关？_

# 工作流
_通常运行哪些 bash 命令，顺序如何？如果不明显，如何解释它们的输出？_

# 错误与修正
_遇到的错误以及如何修复。用户纠正了什么？哪些方法失败了，不应该再试？_

# 代码库和系统文档
_重要的系统组件有哪些？它们如何工作/组合在一起？_

# 经验总结
_什么效果好？什么不好？应该避免什么？不要与其他部分重复_

# 关键结果
_如果用户要求特定输出（如问题的答案、表格或其他文档），在此重复精确结果_

# 工作日志
_分步骤，尝试了什么，做了什么？每一步非常简洁的总结_
`
}

// GetDefaultSessionMemoryUpdatePrompt returns the default prompt for updating session memory (Chinese)
func GetDefaultSessionMemoryUpdatePrompt() string {
	return `重要提示：此消息和这些说明不是实际用户对话的一部分。不要在笔记内容中包含任何关于"笔记"、"会话笔记提取"或这些更新说明的引用。

基于上面的用户对话（不包括此笔记说明消息以及系统提示、EOS.md 条目或任何过去的会话摘要），更新会话笔记文件。

文件 {{notesPath}} 已为你读取。以下是当前内容：
<current_notes_content>
{{currentNotes}}
</current_notes_content>

你的唯一任务是使用 Edit 工具更新笔记文件，然后停止。你可以进行多次编辑（根据需要更新所有部分）在一条消息中并行进行所有 Edit 工具调用。不要调用任何其他工具。

编辑的关键规则：
- 文件必须保持其确切结构，所有部分、标题和斜体描述完整
-- 永远不要修改、删除或添加部分标题（以 '#' 开头的行，如 # 任务说明）
-- 永远不要修改或删除斜体 _部分描述_ 行（这些是紧接在每个标题后面的斜体行——它们以下划线开头和结尾）
-- 斜体 _部分描述_ 是模板说明，必须完全保持原样——它们指导什么内容属于每个部分
-- 只更新出现在斜体 _部分描述_ 下方的实际内容
-- 不要在现有结构之外添加任何新部分、摘要或信息
- 不要在笔记中引用此笔记过程或说明
- 如果没有实质性新见解，可以跳过更新某个部分。不要添加"No info yet"等填充内容，如果适当的话直接留空/不编辑
- 为每个部分编写详细的、信息密集的内容——包括具体信息如文件路径、函数名、错误消息、确切命令、技术细节等
- 对于"关键结果"，包含用户要求的完整确切输出（例如完整表格、完整答案等）
- 不要包含上下文中已包含的 EOS.md 文件中的信息
- 每个部分保持在 ~` + strconv.Itoa(defaultMaxSectionLength) + ` tokens/词以下——如果某个部分接近此限制，通过循环移除较不重要的细节同时保留最关键的信息来压缩它
- 专注于可操作的、具体的信息，帮助他人理解或重现对话中讨论的工作
- 重要：始终更新"当前状态"以反映最近的工作——这对于压缩后的连续性至关重要

使用 file_path: {{notesPath}} 的 Edit 工具

结构保留提醒：
每个部分有两个必须完全保留的部分：
1. 部分标题（以 # 开头的行）
2. 斜体描述行（紧接在标题后面的 _斜体文本_ ——这是模板说明）

你只更新这两个保留行之后的内容。以下划线开头和结尾的斜体描述行是模板结构的一部分，不是要编辑或删除的内容。

记住：并行使用 Edit 工具然后停止。编辑后不要继续。只包含来自实际用户对话的见解，绝不要来自这些笔记说明。不要删除或更改部分标题或斜体 _部分描述_。`
}

// ============================================================================
// Integration with ContextManager
// ============================================================================

// ExtractSessionMemory triggers session memory extraction
func (c *ContextManager) ExtractSessionMemory(ctx context.Context) error {
	_ = ctx
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}
	memoryPath, _, err := SetupMemoryFile(rootDir)
	if err != nil {
		return err
	}

	msgs := filterMemoryConversation(c.BuildPreview())
	sessionDoc := buildSessionMemoryDocument(msgs)
	if err := os.WriteFile(memoryPath, []byte(sessionDoc), 0o600); err != nil {
		return err
	}

	store := memory.NewStore(rootDir)
	written := 0
	for _, entry := range deriveLongTermMemoryEntries(msgs) {
		if _, err := store.Upsert(entry); err == nil {
			written++
		}
	}

	slog.Info("session.memory.extract.triggered",
		"component", utils.ComponentSystem,
		"messages", len(msgs),
		"memory_path", memoryPath,
		"written_long_term", written,
	)

	return nil
}

// IsSessionMemoryEnabled returns whether session memory feature is enabled
// This could be controlled by a config flag or remote config
func (c *ContextManager) IsSessionMemoryEnabled() bool {
	// Could be controlled by config
	return true
}

func filterMemoryConversation(messages []ai.Message) []ai.Message {
	filtered := make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if role != "user" && role != "assistant" {
			continue
		}
		if strings.HasPrefix(content, "DOC:") || strings.HasPrefix(content, "CONVERSATION_SUMMARY_AI:") {
			continue
		}
		filtered = append(filtered, ai.Message{Role: role, Content: content})
	}
	if len(filtered) > 12 {
		filtered = filtered[len(filtered)-12:]
	}
	return filtered
}

func buildSessionMemoryDocument(messages []ai.Message) string {
	title := fallbackString(summarizeUserIntent(messages), "会话记忆更新")
	currentState := fallbackString(summarizeCurrentState(messages), "正在根据最近对话更新会话记忆，并同步长期记忆候选。")
	task := fallbackString(summarizeUserIntent(messages), "最近的用户请求主要围绕当前工作区中的开发任务。")
	files := fallbackString(extractFileRefs(messages), "未识别到稳定的文件/函数引用。")
	workflow := fallbackString(extractCommandRefs(messages), "未识别到稳定工作流命令。")
	errors := fallbackString(extractErrorRefs(messages), "最近对话中未出现明确错误，或错误尚未稳定复现。")
	systemDocs := fallbackString(summarizeSystemDocs(messages), "长期记忆与规则由 EOS.md、Rules.md、独立 memory 文件共同提供。")
	lessons := fallbackString(extractLessons(messages), "优先保留长期有效的约定，避免把一次性任务状态写入长期记忆。")
	keyResults := fallbackString(summarizeAssistantResult(messages), "最近暂无可稳定复用的最终结果。")
	workLog := fallbackString(buildWorkLog(messages), "最近暂无可记录的步骤。")

	var sb strings.Builder
	writeSection := func(title string, hint string, body string) {
		sb.WriteString("# ")
		sb.WriteString(title)
		sb.WriteString("\n")
		sb.WriteString(hint)
		sb.WriteString("\n")
		if strings.TrimSpace(body) != "" {
			sb.WriteString(trimToChars(body, defaultMaxSectionLength*4))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	writeSection("会话标题", "_简短且有特色的 5-10 个词描述会话标题。信息密集，无填充词_", title)
	writeSection("当前状态", "_当前正在积极做什么？未完成的待处理任务。下一步immediate的步骤_", currentState)
	writeSection("任务说明", "_用户要求构建什么？任何设计决策或其他解释性上下文_", task)
	writeSection("文件和函数", "_重要的文件有哪些？简而言之，它们包含什么，为什么相关？_", files)
	writeSection("工作流", "_通常运行哪些 bash 命令，顺序如何？如果不明显，如何解释它们的输出？_", workflow)
	writeSection("错误与修正", "_遇到的错误以及如何修复。用户纠正了什么？哪些方法失败了，不应该再试？_", errors)
	writeSection("代码库和系统文档", "_重要的系统组件有哪些？它们如何工作/组合在一起？_", systemDocs)
	writeSection("经验总结", "_什么效果好？什么不好？应该避免什么？不要与其他部分重复_", lessons)
	writeSection("关键结果", "_如果用户要求特定输出（如问题的答案、表格或其他文档），在此重复精确结果_", keyResults)
	writeSection("工作日志", "_分步骤，尝试了什么，做了什么？每一步非常简洁的总结_", workLog)
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

func deriveLongTermMemoryEntries(messages []ai.Message) []memory.MemoryEntry {
	entries := make([]memory.MemoryEntry, 0, 4)
	seen := make(map[string]struct{})
	appendEntry := func(memType memory.MemoryType, section string, content string, source string) {
		content = memory.NormalizeContent(content)
		if content == "" {
			return
		}
		key := string(memType) + "\n" + section + "\n" + content
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		entries = append(entries, memory.MemoryEntry{
			Type:    memType,
			Section: section,
			Content: content,
			Source:  source,
		})
	}

	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		if msg.Role == "user" && looksLikeGlobalPreference(text) {
			appendEntry(memory.MemoryTypeGlobal, "用户偏好", text, "session_extract:user")
		}
		if looksLikeProjectConvention(text) {
			appendEntry(memory.MemoryTypeProject, "项目约定", text, "session_extract:"+msg.Role)
		}
		if looksLikeProjectConclusion(text) {
			appendEntry(memory.MemoryTypeProject, "任务结论", text, "session_extract:"+msg.Role)
		}
	}
	return entries
}

func summarizeUserIntent(messages []ai.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return trimToChars(messages[i].Content, 120)
		}
	}
	return ""
}

func summarizeCurrentState(messages []ai.Message) string {
	if len(messages) == 0 {
		return ""
	}
	last := messages[len(messages)-1]
	return trimToChars(last.Role+": "+last.Content, 220)
}

func summarizeAssistantResult(messages []ai.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			return trimToChars(messages[i].Content, 240)
		}
	}
	return ""
}

func extractFileRefs(messages []ai.Message) string {
	seen := make(map[string]struct{})
	refs := make([]string, 0, 8)
	for _, msg := range messages {
		for _, token := range strings.Fields(strings.ReplaceAll(msg.Content, "\n", " ")) {
			token = strings.Trim(token, "`'\",.:;()[]{}")
			if token == "" {
				continue
			}
			if strings.Contains(token, "/") || strings.Contains(token, ".go") || strings.Contains(token, ".md") || strings.Contains(token, ".json") {
				if _, ok := seen[token]; ok {
					continue
				}
				seen[token] = struct{}{}
				refs = append(refs, token)
				if len(refs) >= 8 {
					return strings.Join(refs, "\n")
				}
			}
		}
	}
	return strings.Join(refs, "\n")
}

func extractCommandRefs(messages []ai.Message) string {
	lines := make([]string, 0, 6)
	seen := make(map[string]struct{})
	for _, msg := range messages {
		for _, line := range strings.Split(msg.Content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "/") || strings.HasPrefix(line, "go ") || strings.HasPrefix(line, "npm ") || strings.HasPrefix(line, "pnpm ") || strings.HasPrefix(line, "git ") {
				if _, ok := seen[line]; ok {
					continue
				}
				seen[line] = struct{}{}
				lines = append(lines, line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func extractErrorRefs(messages []ai.Message) string {
	lines := make([]string, 0, 4)
	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		if strings.Contains(lower, "error") || strings.Contains(text, "报错") || strings.Contains(lower, "failed") || strings.Contains(lower, "panic") {
			lines = append(lines, trimToChars(text, 240))
		}
		if len(lines) >= 4 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func summarizeSystemDocs(messages []ai.Message) string {
	parts := []string{
		"- 项目指导：EOS.md",
		"- 项目规则：.eos/Rules.md",
		"- 全局规则：~/.eos/Rules.md",
		"- 项目记忆：.eos/memory/project.md",
		"- 全局用户记忆：~/.eos/memory/user.md",
	}
	if refs := extractFileRefs(messages); strings.TrimSpace(refs) != "" {
		parts = append(parts, "- 最近提到的相关文件：\n"+refs)
	}
	return strings.Join(parts, "\n")
}

func extractLessons(messages []ai.Message) string {
	lessons := make([]string, 0, 4)
	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content)
		if looksLikeGlobalPreference(text) || looksLikeProjectConvention(text) {
			lessons = append(lessons, trimToChars(text, 200))
		}
		if len(lessons) >= 4 {
			break
		}
	}
	return strings.Join(lessons, "\n")
}

func buildWorkLog(messages []ai.Message) string {
	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		prefix := "用户"
		if msg.Role == "assistant" {
			prefix = "助手"
		}
		lines = append(lines, "- "+prefix+"："+trimToChars(msg.Content, 120))
	}
	return strings.Join(lines, "\n")
}

func looksLikeGlobalPreference(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	keywords := []string{"默认", "优先", "请用", "使用中文", "不要", "尽量", "习惯", "偏好"}
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func looksLikeProjectConvention(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	keywords := []string{"约定", "规范", "固定", "必须", "统一", "工作流", "命令", "目录结构"}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func looksLikeProjectConclusion(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	keywords := []string{"结论", "最终", "支持", "实现", "新增", "修复", "方案"}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) && (strings.Contains(text, "项目") || strings.Contains(text, "记忆") || strings.Contains(text, "功能")) {
			return true
		}
	}
	return false
}

func trimToChars(text string, max int) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "…"
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
