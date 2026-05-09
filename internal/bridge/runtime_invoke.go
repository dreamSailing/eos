package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	codectx "github.com/dreamSailing/eos/internal/context"
	"github.com/dreamSailing/eos/internal/pkg/workspace"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/tools"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// Reload 重新加载运行时环境（如模型配置变更）
func (rc *RuntimeCore) Reload() error {
	resCh := make(chan error, 1)
	rc.reqCh <- reloadReq{resCh: resCh}
	return <-resCh
}

func (rc *RuntimeCore) setForegroundRequest(traceID string, cancel context.CancelFunc) {
	rc.foregroundMu.Lock()
	rc.foregroundTraceID = strings.TrimSpace(traceID)
	rc.foregroundCancel = cancel
	rc.foregroundMu.Unlock()
}

func (rc *RuntimeCore) clearForegroundRequest(traceID string) {
	traceID = strings.TrimSpace(traceID)
	rc.foregroundMu.Lock()
	if traceID == "" || rc.foregroundTraceID == traceID {
		rc.foregroundTraceID = ""
		rc.foregroundCancel = nil
	}
	rc.foregroundMu.Unlock()
}

func (rc *RuntimeCore) CancelForegroundRequest() bool {
	rc.foregroundMu.Lock()
	traceID := strings.TrimSpace(rc.foregroundTraceID)
	cancel := rc.foregroundCancel
	rc.foregroundTraceID = ""
	rc.foregroundCancel = nil
	rc.foregroundMu.Unlock()

	if cancel == nil && traceID == "" {
		return false
	}
	if cancel != nil {
		cancel()
	}
	if traceID == "" {
		return true
	}
	resCh := make(chan bool, 1)
	rc.reqCh <- cancelForegroundReq{traceID: traceID, resCh: resCh}
	return <-resCh
}

// GraphInvoke 执行图形编排调用
func (rc *RuntimeCore) GraphInvoke(ctx context.Context, query string) (*schema.Message, error) {
	return rc.GraphInvokePlan(ctx, query, "")
}

// GraphInvokePlan 按计划调用图
func (rc *RuntimeCore) GraphInvokePlan(ctx context.Context, query, executionMode string) (*schema.Message, error) {
	return rc.GraphInvokePlanWithImages(ctx, query, executionMode, nil)
}

// GraphInvokePlanWithImages 带图片调用图
func (rc *RuntimeCore) GraphInvokePlanWithImages(ctx context.Context, query, executionMode string, imagePaths []string) (*schema.Message, error) {
	traceID := tools.TraceIDFromContext(ctx)
	if traceID == "" {
		traceID = uuid.NewString()[:8]
		ctx = tools.WithTraceID(ctx, traceID)
	}
	ctx = rc.withWorkspaceRoot(ctx)
	ctx, cancel := context.WithCancel(ctx)
	rc.setForegroundRequest(traceID, cancel)
	defer func() {
		cancel()
		rc.clearForegroundRequest(traceID)
	}()

	// Phase 1 集成: Token 预算检查
	if status := rc.CheckTokenBudget(); status == BudgetExceeded {
		return nil, fmt.Errorf("token budget exceeded: session or turn limit reached")
	}

	// Phase 1 集成: 上下文压缩 — 在 invoke 前检查是否需要压缩
	if rc.cm != nil {
		rc.cm.CheckAndCompact(func(text string) (string, error) {
			return rc.Summarize(ctx, text)
		})
	}

	// Phase 1 集成: 重置 turn 级别的 token 计数
	rc.ResetTurnBudget()

	rc.StartRequest(traceID)
	ch := make(chan graphInvokeRes, 1)
	rc.reqCh <- graphInvokeReq{ctx: ctx, query: query, executionMode: executionMode, imagePaths: imagePaths, resCh: ch}
	res := <-ch

	// Phase 1 集成: 记录 token 使用量到预算管理器
	if res.msg != nil {
		inputTokens, replyTokens, totalTokens := rc.EstimateTokens(res.msg.Content)
		if totalTokens > 0 {
			rc.RecordTokenUsage(inputTokens, replyTokens)
		}
		rc.AddTokenRecordWithModel(inputTokens, replyTokens, totalTokens, rc.ModelName())
	}

	if res.err != nil {
		rc.EndRequest(traceID, rc.ModelName())
		rc.FinalizeTask(traceID, query, "", false, res.err.Error())
		return res.msg, res.err
	}
	rc.EndRequest(traceID, rc.ModelName())
	assistant := ""
	if res.msg != nil {
		assistant = res.msg.Content
	}
	rc.FinalizeTask(traceID, query, assistant, true, "")
	return res.msg, res.err
}

// ToolsNode 工具节点
func (rc *RuntimeCore) ToolsNode(ctx context.Context, payload string) ([]string, bool, bool) {
	ctx = rc.withWorkspaceRoot(ctx)
	ch := make(chan toolsNodeRes, 1)
	rc.reqCh <- toolsNodeReq{ctx: ctx, text: payload, resCh: ch}
	res := <-ch
	return res.results, res.executed, res.cont
}

// Summarize 摘要
func (rc *RuntimeCore) Summarize(ctx context.Context, text string) (string, error) {
	ctx = rc.withWorkspaceRoot(ctx)
	ch := make(chan summarizeRes, 1)
	rc.reqCh <- summarizeReq{ctx: ctx, text: text, resCh: ch}
	res := <-ch
	return res.text, res.err
}

func (rc *RuntimeCore) PredictNextUserMessage(ctx context.Context, draft string) (string, error) {
	ctx = rc.withWorkspaceRoot(ctx)
	transcript := buildPredictionTranscript(rc.cm, draft)
	if transcript == "" {
		return "", nil
	}
	ch := make(chan predictNextRes, 1)
	rc.reqCh <- predictNextReq{ctx: ctx, text: transcript, resCh: ch}
	res := <-ch
	if res.err != nil {
		return "", res.err
	}
	return cleanPredictionText(res.text), nil
}

func (rc *RuntimeCore) FinalizeTask(traceID string, userText string, assistantText string, success bool, errorMsg string) {
	ch := make(chan struct{}, 1)
	rc.reqCh <- finalizeTaskReq{traceID: traceID, userText: userText, assistantText: assistantText, success: success, errorMsg: errorMsg, resCh: ch}
	<-ch
}

// WithOnMeta 设置元数据回调
func (rc *RuntimeCore) WithOnMeta(cb func(string)) *RuntimeCore {
	rc.mu.Lock()
	rc.onMeta = cb
	rc.mu.Unlock()
	return rc
}

// WithOnDelta 设置增量回调
func (rc *RuntimeCore) WithOnDelta(cb func(string)) *RuntimeCore {
	rc.mu.Lock()
	rc.onDelta = cb
	rc.mu.Unlock()
	return rc
}

// WithOnReasoning 设置思考回调
func (rc *RuntimeCore) WithOnReasoning(cb func(string)) *RuntimeCore {
	rc.mu.Lock()
	rc.onReasoning = cb
	rc.mu.Unlock()
	return rc
}

// ModelName 获取模型名称
func (rc *RuntimeCore) ModelName() string {
	rc.mu.RLock()
	v := rc.modelName
	rc.mu.RUnlock()
	return v
}

// GetContext 获取会话上下文管理器
func (rc *RuntimeCore) GetContext() *session.ContextManager {
	return rc.cm
}

// GetTools 获取工具管理器
func (rc *RuntimeCore) GetTools() *tools.Manager {
	return rc.tm
}

// GetWorkspace 获取工作区管理器
func (rc *RuntimeCore) GetWorkspace() *workspace.Manager {
	return rc.wsMgr
}

// ModelBase 获取模型基础
func (rc *RuntimeCore) ModelBase() string {
	rc.mu.RLock()
	v := rc.modelBase
	rc.mu.RUnlock()
	return v
}

// ExecuteBash 执行 Bash 命令
func (rc *RuntimeCore) ExecuteBash(ctx context.Context, cmd string) (string, error) {
	ctx = rc.withWorkspaceRoot(ctx)
	cat, _, sum, dang := tools.ClassifyBashDanger(cmd)
	if dang && (rc.hooks.SessionAllowed == nil || !rc.hooks.SessionAllowed(cat)) {
		dec := rc.hooks.Prompt(ctx, cat, sum)
		if dec == "deny" {
			return "", os.ErrPermission
		}
		if dec == "session" && rc.hooks.AllowSession != nil {
			rc.hooks.AllowSession(cat)
		}
	}

	return rc.tm.ExecuteBashDirect(ctx, cmd)
}

// DemoteFullWithAISummary 使用 AI 摘要降级完整消息
func (rc *RuntimeCore) DemoteFullWithAISummary(ctx context.Context) {
	full := rc.cm.CurrentFull()
	if len(full) == 0 {
		return
	}

	success := 0
	for _, m := range full {
		out, err := rc.Summarize(ctx, m.Content)
		if err != nil {
			slog.Error("bridge.demote_full.summarize.error",
				"content_length", len(m.Content),
				"error", err)
			continue
		}
		if strings.TrimSpace(out) == "" {
			slog.Warn("bridge.demote_full.empty_summary",
				"content_length", len(m.Content))
			continue
		}
		rc.cm.AddToolSummary(out)
		success++
	}
	if success == len(full) {
		rc.cm.ClearCurrentFull()
		slog.Debug("bridge.demote_full.success",
			"summarized_count", success)
	} else {
		slog.Debug("bridge.demote_full.partial",
			"success_count", success,
			"total_count", len(full))
	}
}

func buildPredictionTranscript(cm *session.ContextManager, draft string) string {
	if cm == nil {
		return ""
	}
	st := cm.ExportState()
	msgs := st.Recent
	if len(msgs) == 0 {
		msgs = st.CurrentFull
	}
	if len(msgs) == 0 {
		return ""
	}
	relevant := make([]string, 0, 6)
	hasUser := false
	hasAssistant := false
	for i := len(msgs) - 1; i >= 0 && len(relevant) < 6; i-- {
		msg := msgs[i]
		role := strings.TrimSpace(strings.ToLower(msg.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		switch role {
		case "user":
			hasUser = true
			relevant = append(relevant, "用户: "+content)
		case "assistant":
			hasAssistant = true
			relevant = append(relevant, "助手: "+content)
		}
	}
	if !hasUser || !hasAssistant || len(relevant) < 2 {
		return ""
	}
	for i, j := 0, len(relevant)-1; i < j; i, j = i+1, j-1 {
		relevant[i], relevant[j] = relevant[j], relevant[i]
	}
	var sb strings.Builder
	sb.WriteString("请基于以下最近对话，预测用户接下来最可能发送的一句话。")
	if strings.TrimSpace(draft) == "" {
		sb.WriteString("当前用户还没有开始输入，请输出一条完整的下一句消息。")
	} else {
		sb.WriteString("当前用户已经输入了以下前缀，请保持此前缀不变，并补全成最自然的一整句用户消息。")
	}
	sb.WriteString("\n\n")
	sb.WriteString(strings.Join(relevant, "\n\n"))
	if strings.TrimSpace(draft) != "" {
		sb.WriteString("\n\n")
		sb.WriteString("当前输入前缀: ")
		sb.WriteString(strings.TrimSpace(draft))
	}
	return sb.String()
}

func cleanPredictionText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' })
	text = strings.TrimSpace(strings.Join(lines, " "))
	for _, prefix := range []string{
		"用户可能会说：",
		"用户可能会发送：",
		"预测：",
		"下一句：",
		"Next:",
		"Prediction:",
	} {
		if strings.HasPrefix(text, prefix) {
			text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	}
	text = strings.Trim(text, `"'“”‘’`)
	if text == "" {
		return ""
	}
	rs := []rune(text)
	if len(rs) > 120 {
		text = strings.TrimSpace(string(rs[:120]))
	}
	return text
}

// ProcessContextHints 处理上下文提示
func (rc *RuntimeCore) ProcessContextHints(text string, autoContext bool, maxInjectKB int, logger Logger) {
	settings := rc.GetSettings()
	if !autoContext {
		autoContext = settings.AutoContext
	}
	if maxInjectKB <= 0 {
		maxInjectKB = settings.MaxInjectKB
	}
	if rc.ctxEngine == nil || !autoContext {
		return
	}

	hardLimitBytes := computeInjectBudgetBytes(rc, maxInjectKB)
	if hardLimitBytes <= 0 {
		return
	}

	limitFiles := clampInt(hardLimitBytes/4096, 2, 12)
	sugg := rc.ctxEngine.Suggest(text, limitFiles*3)
	candidates := buildInjectCandidates(rc, text, sugg, limitFiles)
	if len(candidates) == 0 {
		return
	}

	// 显示 UI 提示
	hint := rc.buildContextHint(candidates)
	rc.cm.AddEphemeral(hint)
	if logger != nil {
		logger.ShowHint(hint)
	}

	// 注入文件内容
	rc.injectContextFiles(text, candidates, hardLimitBytes)

	if git := buildGitContextHint(rc.workingRoot()); strings.TrimSpace(git) != "" {
		rc.cm.AddEphemeral(git)
	}
}

// buildContextHint 构建上下文提示字符串
func (rc *RuntimeCore) buildContextHint(sugg []codectx.Suggestion) string {
	var parts []string
	maxSuggestions := 8
	if len(sugg) < maxSuggestions {
		maxSuggestions = len(sugg)
	}

	for i := 0; i < maxSuggestions; i++ {
		s := sugg[i]
		items := s.Symbols
		if len(items) > 6 {
			items = items[:6]
		}
		preview := s.Path
		if len(items) > 0 {
			preview += "(" + strings.Join(items, ", ") + ")"
		}
		parts = append(parts, preview)
	}

	return "AutoContext: " + strconv.Itoa(len(parts)) + " files"
}

// injectContextFiles 注入上下文文件内容到当前会话
func (rc *RuntimeCore) injectContextFiles(query string, sugg []codectx.Suggestion, maxBytes int) {
	used := 0
	if maxBytes <= 0 {
		return
	}
	perFileMax := clampInt(maxBytes/clampInt(len(sugg), 1, 12), 2048, 16384)

	for _, s := range sugg {
		if used >= maxBytes {
			break
		}
		budget := perFileMax
		if left := maxBytes - used; left < budget {
			budget = left
		}

		content, err := rc.readContextFile(query, s, budget)
		if err != nil || strings.TrimSpace(content) == "" {
			continue
		}

		rc.cm.AddToolFull("@" + s.Path + "\n" + content)
		used += len(content)
	}
}

// readContextFile 读取上下文文件内容，限制大小
func (rc *RuntimeCore) readContextFile(query string, sugg codectx.Suggestion, maxBytes int) (string, error) {
	ap := rc.resolveWithinRoot(sugg.Path)
	bs, err := os.ReadFile(ap)
	if err != nil {
		return "", err
	}

	content := string(bs)
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	content = extractRelevantSnippet(content, query, sugg.Symbols, maxBytes)
	return content, nil
}

// ProcessAttachments 处理附件
func (rc *RuntimeCore) ProcessAttachments(attachments []Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("此消息包含图片占位符，请调用 vision_parse 工具解析；若工具返回 VISION_UNAVAILABLE，请明确告知用户当前模型不具备视觉能力。\n")
	b.WriteString("attachments=[")
	for i, a := range attachments {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("{id=")
		b.WriteString(strconv.Itoa(a.ID))
		b.WriteString(",path=")
		rel := a.Path
		if filepath.IsAbs(rel) {
			if r, e := filepath.Rel(rc.workingRoot(), rel); e == nil && !strings.HasPrefix(r, "..") {
				rel = filepath.ToSlash(r)
			}
		}
		b.WriteString(rel)
		b.WriteString(",mime=")
		b.WriteString(a.Mime)
		b.WriteString(",placeholder=")
		b.WriteString(a.Placeholder)
		b.WriteString("}")
	}
	b.WriteString("]")
	return b.String()
}

// ProcessLSPContext 处理 LSP 上下文信息
func (rc *RuntimeCore) ProcessLSPContext() {
	rc.ProcessLSPDiagnostics(rc.lspManager)
}
