package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/notify"
	pluginpkg "github.com/dreamSailing/eos/internal/pkg/plugins"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	einoruntime "github.com/dreamSailing/eos/internal/runtime"
	"github.com/dreamSailing/eos/internal/session"
	"github.com/dreamSailing/eos/internal/skills"
	"github.com/dreamSailing/eos/internal/state"
	"github.com/dreamSailing/eos/internal/tools"
	"github.com/dreamSailing/eos/internal/tools/fileops"
	"github.com/google/uuid"
	"log/slog"
	"regexp"
	stdruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var thinkBlockPattern = regexp.MustCompile(`(?is)<think>.*?</think>`)

// loop 运行时事件循环
func (rc *RuntimeCore) loop() {
	defer rc.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("runtime.loop.panic.recovered",
				"component", utils.ComponentSystem,
				"panic", r,
				"stack_trace", getStackTrace(),
			)
			// 尝试优雅关闭：通知所有等待的通道
			rc.cleanupPendingRequests()
			rc.restartLoopAsync()
		}
	}()
	var rt *einoruntime.EinoRuntime
	ctx := context.Background()

	initRuntime := func() error {
		_, _, modelName := ai.ResolveAPISettings()

		cfg, _ := config.Load()
		reasoningLevel := ""

		if modelName != "" {
			// 综合全局配置、模型能力以及 UI 层的实时开关
			uiThinking := state.Thinking()
			shouldEnable := uiThinking && ai.ShouldEnableThinkingForModel(modelName, &cfg)

			if shouldEnable {
				reasoningLevel = ai.GetReasoningEffortLevel(modelName, &cfg)
				slog.Info("bridge.init_runtime.thinking_enabled", "component", utils.ComponentSystem,
					"model", modelName,
					"reasoning_level", reasoningLevel,
					"ui_toggle", uiThinking,
				)
			} else {
				slog.Debug("bridge.init_runtime.thinking_disabled", "component", utils.ComponentSystem,
					"model", modelName,
					"reason", "ui_disabled_or_model_unsupported",
					"ui_toggle", uiThinking,
				)
			}
		}

		slog.Debug("bridge.init_runtime.create_model_start", "component", utils.ComponentSystem)
		em, errM := einoruntime.NewChatModelWithReasoning(ctx, reasoningLevel)
		if errM != nil {
			slog.Error("bridge.init_runtime.chat_model_failed", "component", utils.ComponentSystem, "error", errM.Error())
			return errM
		}
		slog.Debug("bridge.init_runtime.create_model_success", "component", utils.ComponentSystem)

		slog.Debug("bridge.init_runtime.load_mcp_start", "component", utils.ComponentSystem)
		mcpCfg := cfg
		mcpCfg.MCP = pluginpkg.MergeMCPEntries(&cfg, rc.workingRoot())
		if err := rc.mcpMgr.Reload(ctx, &mcpCfg); err != nil {
			slog.Warn("bridge.init_runtime.load_mcp_failed", "component", utils.ComponentSystem, "error", err.Error())
		}
		mcpTools := rc.mcpMgr.GetAllTools()
		slog.Debug("bridge.init_runtime.load_mcp_success", "component", utils.ComponentSystem, "tools_count", len(mcpTools))

		slog.Debug("bridge.init_runtime.load_skills_start", "component", utils.ComponentSystem)
		skillsDirs := skills.ResolveScanDirs(rc.workingRoot(), &cfg)

		if len(skillsDirs) > 0 {
			rc.skillsLoader.SetSkillsDirs(skillsDirs)
			if err := rc.skillsLoader.Scan(); err != nil {
				slog.Warn("bridge.init_runtime.load_skills_failed", "component", utils.ComponentSystem, "error", err.Error())
			} else {
				slog.Info("bridge.init_runtime.load_skills_success", "component", utils.ComponentSystem,
					"skills_dirs", skillsDirs,
					"skills_count", len(rc.skillsLoader.List()))
			}
		}
		if sm := rc.tm.GetSkillManager(); sm != nil {
			sm.SetDisabledSkills(cfg.DisabledSkills)
		}
		for _, plugin := range pluginpkg.DefaultRegistry().List() {
			if plugin == nil {
				continue
			}
			pluginpkg.DefaultRegistry().SetEnabled(plugin.Name(), true)
		}
		for _, entry := range cfg.Plugins {
			pluginpkg.DefaultRegistry().SetEnabled(entry.Name, entry.Enabled)
		}

		slog.Debug("bridge.init_runtime.create_runtime_start", "component", utils.ComponentSystem)
		nrt, err := einoruntime.NewEinoRuntimeWithMCP(ctx, rc.cm, rc.tm, em, mcpTools)
		if err != nil {
			slog.Error("bridge.init_runtime.eino_runtime_failed", "component", utils.ComponentSystem, "error", err.Error())
			return err
		}
		nrt.WithOnPlanUpdate(func(plan string) {
			rc.HandlePlanUpdate(plan)
		})
		slog.Debug("bridge.init_runtime.create_runtime_success", "component", utils.ComponentSystem)

		var agentMu sync.Mutex
		lastAgentName := ""
		nrt.WithOnMeta(func(line string) {
			rc.mu.RLock()
			cbMeta := rc.onMeta
			rc.mu.RUnlock()
			if cbMeta != nil {
				if len(line) < 200 {
					slog.Debug("bridge.on_meta", "component", utils.ComponentSystem, "content", line)
				}
				cbMeta(line)
			}

			rawLine := strings.TrimSpace(line)
			if rawLine == "" {
				return
			}
			if strings.HasPrefix(rawLine, "{") {
				var m map[string]any
				if err := json.Unmarshal([]byte(rawLine), &m); err == nil {
					if t, ok := m["type"].(string); ok {
						switch t {
						case einoruntime.EventToolCall:
							name, _ := m["tool"].(string)
							id, _ := m["id"].(string)
							var params map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								params = d
							}
							rc.eventsCh <- bridgeToolCallEvent(id, name, params)
							return
						case einoruntime.EventToolResult:
							id, _ := m["id"].(string)
							status, _ := m["status"].(string)
							display, _ := m["display"].(string)
							errMsg, _ := m["error"].(string)
							toolName, _ := m["tool"].(string)
							out := strings.TrimSpace(display)
							if out == "" {
								out = strings.TrimSpace(errMsg)
							}
							var data map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								data = d
							}
							rc.eventsCh <- bridgeToolResultEvent(id, toolName, status, out, errMsg, data)
							return
						case einoruntime.EventLoopBlock:
							name, _ := m["tool"].(string)
							level, _ := m["level"].(string)
							reason, _ := m["reason"].(string)
							message, _ := m["message"].(string)
							var data map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								data = d
							}
							rc.eventsCh <- bridgeLoopBlockedEvent(name, level, reason, message, mergeBridgeEventData(data, m))
							return
						case einoruntime.EventTurnWrapUp:
							name, _ := m["tool"].(string)
							reason, _ := m["reason"].(string)
							message, _ := m["message"].(string)
							var data map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								data = d
							}
							rc.eventsCh <- bridgeTurnWrapUpEvent(name, reason, message, mergeBridgeEventData(data, m))
							return
						case einoruntime.EventAssistantDelta:
							content, _ := m["content"].(string)
							rc.eventsCh <- bridgeTextDeltaEvent(content)
							return
						case einoruntime.EventPhaseNote:
							note, _ := m["content"].(string)
							rc.eventsCh <- bridgeReasoningEvent(note)
							return
						case einoruntime.EventAgentStarted:
							id, _ := m["id"].(string)
							content, _ := m["content"].(string)
							var data map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								data = d
							}
							agentName, _ := data["agent_name"].(string)
							task := strings.TrimSpace(content)
							if task == "" {
								task, _ = data["task"].(string)
							}
							rc.eventsCh <- bridgeAgentStartedEvent(id, agentName, task, data)
							return
						case einoruntime.EventAgentProgress:
							id, _ := m["id"].(string)
							content, _ := m["content"].(string)
							var data map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								data = d
							}
							agentName, _ := data["agent_name"].(string)
							task := strings.TrimSpace(content)
							if task == "" {
								task, _ = data["task"].(string)
							}
							rc.eventsCh <- bridgeAgentProgressEvent(id, agentName, task, data)
							return
						case einoruntime.EventAgentCompleted:
							id, _ := m["id"].(string)
							content, _ := m["content"].(string)
							var data map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								data = d
							}
							agentName, _ := data["agent_name"].(string)
							rc.eventsCh <- bridgeAgentCompletedEvent(id, agentName, content, data)
							return
						case einoruntime.EventAgentFailed:
							id, _ := m["id"].(string)
							content, _ := m["content"].(string)
							var data map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								data = d
							}
							agentName, _ := data["agent_name"].(string)
							errMsg := strings.TrimSpace(content)
							if errMsg == "" {
								errMsg, _ = data["error"].(string)
							}
							rc.eventsCh <- bridgeAgentFailedEvent(id, agentName, errMsg, data)
							return
						case einoruntime.EventAgentCancelled:
							id, _ := m["id"].(string)
							content, _ := m["content"].(string)
							var data map[string]any
							if d, ok := m["data"].(map[string]any); ok {
								data = d
							}
							agentName, _ := data["agent_name"].(string)
							reason := strings.TrimSpace(content)
							if reason == "" {
								reason, _ = data["reason"].(string)
							}
							rc.eventsCh <- bridgeAgentCancelledEvent(id, agentName, reason, data)
							return
						}
					}
				}
			}

			if strings.HasPrefix(line, "agent.task:") {
				raw := strings.TrimSpace(strings.TrimPrefix(line, "agent.task:"))
				if raw == "" {
					return
				}
				parts := strings.SplitN(raw, " ", 2)
				name := strings.TrimSpace(parts[0])
				task := ""
				if len(parts) > 1 {
					task = strings.TrimSpace(parts[1])
				}
				agentMu.Lock()
				lastAgentName = name
				agentMu.Unlock()
				rc.eventsCh <- bridgeAgentProgressEvent("", name, task, nil)
				return
			}

			if strings.HasPrefix(line, "agent.final:") {
				content := strings.TrimSpace(strings.TrimPrefix(line, "agent.final:"))
				agentMu.Lock()
				name := lastAgentName
				agentMu.Unlock()
				rc.eventsCh <- bridgeAgentCompletedEvent("", name, content, nil)
				return
			}

			if strings.HasPrefix(line, einoruntime.EventToolCall+":") {
				raw := strings.TrimSpace(strings.TrimPrefix(line, einoruntime.EventToolCall+":"))
				if raw == "" {
					return
				}
				parts := strings.SplitN(raw, ":", 2)
				if len(parts) == 2 {
					rc.eventsCh <- bridgeToolCallEvent(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil)
				} else {
					rc.eventsCh <- bridgeToolCallEvent("", strings.TrimSpace(parts[0]), nil)
				}
				return
			}

			if strings.HasPrefix(line, einoruntime.EventToolResult+":") {
				raw := strings.TrimSpace(strings.TrimPrefix(line, einoruntime.EventToolResult+":"))
				if raw == "" {
					return
				}
				parts := strings.SplitN(raw, ":", 2)
				if len(parts) == 2 {
					rc.eventsCh <- bridgeToolResultEvent(strings.TrimSpace(parts[0]), "", "success", strings.TrimSpace(parts[1]), "", nil)
				} else {
					rc.eventsCh <- bridgeToolResultEvent("", "", "success", strings.TrimSpace(parts[0]), "", nil)
				}
				return
			}

			if strings.HasPrefix(line, einoruntime.EventLoopBlock+":") {
				raw := strings.TrimSpace(strings.TrimPrefix(line, einoruntime.EventLoopBlock+":"))
				if raw != "" {
					rc.eventsCh <- bridgeLoopBlockedEvent("", "", "", raw, map[string]any{"message": raw})
				}
				return
			}

			if strings.HasPrefix(line, einoruntime.EventAssistantDelta+":") {
				raw := strings.TrimSpace(strings.TrimPrefix(line, einoruntime.EventAssistantDelta+":"))
				if raw != "" {
					rc.eventsCh <- bridgeTextDeltaEvent(raw)
				}
				return
			}

			if strings.HasPrefix(line, einoruntime.EventPhaseNote+":") {
				raw := strings.TrimSpace(strings.TrimPrefix(line, einoruntime.EventPhaseNote+":"))
				if raw != "" {
					rc.eventsCh <- bridgeReasoningEvent(raw)
				}
				return
			}
		})
		nrt.WithOnDelta(func(delta string) {
			// 使用流式解析器处理
			if rc.parser == nil {
				rc.parser = NewStreamParser(rc.eventsCh)
			}
			rc.parser.Process(delta)
		})
		nrt.WithOnReasoning(func(reasoning string) {
			rc.mu.RLock()
			cbReasoning := rc.onReasoning
			rc.mu.RUnlock()
			if cbReasoning != nil {
				cbReasoning(reasoning)
			}
			// 发送事件到 eventsCh
			rc.eventsCh <- bridgeReasoningEvent(reasoning)
		})

		nrt = nrt.WithSafety(rc.hooks)

		// Phase 1 集成: Wire AgentTool executor to use the runtime
		tools.AgentToolExecutor = func(agentCtx context.Context, prompt, subagentType, description, model string) (string, error) {
			agentCtx = tools.WithTraceID(agentCtx, "agent-"+uuid.New().String()[:8])
			result, err := nrt.GraphInvoke(agentCtx, prompt, "")
			if err != nil {
				return "", err
			}
			if result != nil {
				return result.Content, nil
			}
			return "", nil
		}

		rt = nrt
		rc.mu.Lock()
		rc.modelName = rt.ModelName()
		rc.modelBase = rt.ModelBase()
		rc.mu.Unlock()
		if rc.cm != nil {
			base, key, _, _ := rc.ResolveAPIConfig()
			ai.PrimeContextWindowFromProvider(ctx, base, key, rc.modelName)
			rc.cm.SetModel(rc.modelName)
			window := ai.ContextWindowTokens(rc.modelName)
			rc.mu.Lock()
			if rc.settings.MaxInjectKB <= 0 {
				switch {
				case window >= 128000:
					rc.settings.MaxInjectKB = 96
				case window >= 32000:
					rc.settings.MaxInjectKB = 48
				default:
					rc.settings.MaxInjectKB = 24
				}
			}
			rc.mu.Unlock()
			switch {
			case window >= 128000:
				rc.cm.SetCompressionStrategy(session.CompressionConservative)
			case window >= 32000:
				rc.cm.SetCompressionStrategy(session.CompressionBalanced)
			default:
				rc.cm.SetCompressionStrategy(session.CompressionAggressive)
			}
		}

		// Initialize fast model runtime for summaries if FastModel is configured
		if cfg.FastModel != "" && rc.fastRT == nil {
			var fastModelEntry config.ModelEntry
			fastOk := false
			for _, m := range cfg.Models {
				if m.Name == cfg.FastModel || m.Model == cfg.FastModel {
					fastModelEntry = m
					fastOk = true
					break
				}
			}
			if fastOk {
				fastEm, errF := einoruntime.NewChatModelWithSettings(ctx, fastModelEntry.APIKey, fastModelEntry.APIBase, fastModelEntry.Model, "")
				if errF == nil {
					fastRT, errF2 := einoruntime.NewEinoRuntimeWithMCP(ctx, rc.cm, rc.tm, fastEm, nil)
					if errF2 == nil {
						fastRT.WithOnPlanUpdate(func(plan string) {
							rc.HandlePlanUpdate(plan)
						})
						rc.fastRT = fastRT
						slog.Info("bridge.init_runtime.fast_model_created", "component", utils.ComponentSystem, "fast_model", fastModelEntry.Model)
					} else {
						slog.Warn("bridge.init_runtime.fast_model_runtime_failed", "component", utils.ComponentSystem, "error", errF2.Error())
					}
				} else {
					slog.Warn("bridge.init_runtime.fast_model_failed", "component", utils.ComponentSystem, "error", errF.Error())
				}
			} else {
				slog.Warn("bridge.init_runtime.fast_model_not_found", "component", utils.ComponentSystem, "fast_model", cfg.FastModel)
			}
		}

		return nil
	}

	for {
		select {
		case req := <-rc.reqCh:
			switch r := req.(type) {
			case reloadReq:
				rc.addPendingReload(r.resCh)
				rc.ShutdownLSPManager(rc.lspManager)
				rc.lspManager = rc.initLSPManager()
				err := initRuntime()
				rc.removePendingReload(r.resCh)
				r.resCh <- err
			case graphInvokeReq:
				rc.addPendingGraph(r.resCh)
				if rt == nil {
					if err := initRuntime(); err != nil {
						rc.removePendingGraph(r.resCh)
						r.resCh <- graphInvokeRes{msg: nil, err: err}
						continue
					}
				}
				currentRT := rt
				rc.wg.Add(1)
				go func(tRT *einoruntime.EinoRuntime, tReq graphInvokeReq) {
					defer rc.wg.Done()
					defer rc.removePendingGraph(tReq.resCh)

					// 重置解析器并确保在结束时刷新
					if rc.parser == nil {
						rc.parser = NewStreamParser(rc.eventsCh)
					}
					rc.parser.Reset()
					defer rc.parser.Flush()

					defer func() {
						if rr := recover(); rr != nil {
							tReq.resCh <- graphInvokeRes{msg: nil, err: ErrRuntimeLoopUnavailable}
						}
					}()

					// Reset tool result budget at start of each turn
					rc.tm.ResetResultBudget()

					// Proactive micro-compact: compress large tool outputs before they accumulate
					if rc.cm != nil && rc.cm.ShouldMicroCompact() {
						slog.Debug("runtime.micro_compact.proactive", "component", utils.ComponentSystem)
						rc.cm.MicroCompact()
					}

					// Fix 4.4: Auto-compact when token threshold exceeded
					if rc.cm != nil && rc.cm.GetAutoCompressEnabled() {
						rc.cm.AutoCompactIfNeeded()
					}

					msg, err := rc.graphInvokeWithRetry(tReq.ctx, tRT, tReq.query, tReq.executionMode, tReq.imagePaths)

					// Gap D: Check stop hooks after graph invocation
					if err == nil && msg != nil && rc.hookManager != nil {
						assistantContent := ""
						if msg.Content != "" {
							assistantContent = msg.Content
						}
						stopDec, stopErr := rc.hookManager.Stop(tReq.ctx, assistantContent, true)
						if stopErr == nil && (stopDec.Decision == "block" || stopDec.Decision == "deny") {
							slog.Info("runtime.stop_hook.blocked", "component", utils.ComponentSystem, "reason", stopDec.Reason)
						}
					}

					// Fix 4.2: Check session memory extraction after graph invoke
					if err == nil && rc.cm != nil {
						if smMgr := rc.cm.GetSessionMemoryManager(); smMgr != nil && rc.cm.IsSessionMemoryEnabled() {
							messages := rc.cm.Build()
							tokenCount := rc.cm.EstimateCurrentTokens()
							if smMgr.ShouldExtractMemory(messages, tokenCount) {
								lastMessageContent := ""
								if len(messages) > 0 {
									lastMessageContent = messages[len(messages)-1].Content
								}
								go func() {
									smMgr.SetExtractionInProgress(true)
									defer smMgr.SetExtractionInProgress(false)
									if extractErr := rc.cm.ExtractSessionMemory(context.Background()); extractErr != nil {
										slog.Warn("runtime.session_memory.extract_failed", "component", utils.ComponentSystem, "error", extractErr.Error())
									} else {
										smMgr.RecordExtraction(tokenCount, lastMessageContent)
									}
								}()
							}
						}
					}

					tReq.resCh <- graphInvokeRes{msg: msg, err: err}
				}(currentRT, r)
			case toolsNodeReq:
				rc.addPendingTools(r.resCh)
				if rt == nil {
					if err := initRuntime(); err != nil {
						rc.removePendingTools(r.resCh)
						r.resCh <- toolsNodeRes{}
						continue
					}
				}
				currentRT := rt
				rc.wg.Add(1)
				go func(tRT *einoruntime.EinoRuntime, tReq toolsNodeReq) {
					defer rc.wg.Done()
					defer rc.removePendingTools(tReq.resCh)
					defer func() {
						if rr := recover(); rr != nil {
							tReq.resCh <- toolsNodeRes{results: nil, executed: false, cont: false}
						}
					}()
					rs, ok, cont := tRT.ToolsNode(tReq.ctx, tReq.text)
					tReq.resCh <- toolsNodeRes{results: rs, executed: ok, cont: cont}
				}(currentRT, r)
			case summarizeReq:
				rc.addPendingSummarize(r.resCh)
				if rt == nil {
					if err := initRuntime(); err != nil {
						rc.removePendingSummarize(r.resCh)
						r.resCh <- summarizeRes{text: "", err: err}
						continue
					}
				}
				// Use fast model for summaries if available
				currentRT := rt
				if rc.fastRT != nil {
					currentRT = rc.fastRT
				}
				rc.wg.Add(1)
				go func(tRT *einoruntime.EinoRuntime, tReq summarizeReq) {
					defer rc.wg.Done()
					defer rc.removePendingSummarize(tReq.resCh)
					defer func() {
						if rr := recover(); rr != nil {
							tReq.resCh <- summarizeRes{text: "", err: ErrRuntimeLoopUnavailable}
						}
					}()
					out, err := tRT.Summarize(tReq.ctx, tReq.text)
					tReq.resCh <- summarizeRes{text: out, err: err}
				}(currentRT, r)
			case predictNextReq:
				rc.addPendingPredict(r.resCh)
				if rt == nil {
					if err := initRuntime(); err != nil {
						rc.removePendingPredict(r.resCh)
						r.resCh <- predictNextRes{text: "", err: err}
						continue
					}
				}
				currentRT := rt
				if rc.fastRT != nil {
					currentRT = rc.fastRT
				}
				rc.wg.Add(1)
				go func(tRT *einoruntime.EinoRuntime, tReq predictNextReq) {
					defer rc.wg.Done()
					defer rc.removePendingPredict(tReq.resCh)
					defer func() {
						if rr := recover(); rr != nil {
							tReq.resCh <- predictNextRes{text: "", err: ErrRuntimeLoopUnavailable}
						}
					}()
					out, err := tRT.PredictNextUserMessage(tReq.ctx, tReq.text)
					tReq.resCh <- predictNextRes{text: out, err: err}
				}(currentRT, r)
			case finalizeTaskReq:
				rc.finalizeTask(rt, r.traceID, r.userText, r.assistantText, r.success, r.errorMsg)
				r.resCh <- struct{}{}
			case cancelForegroundReq:
				cancelled := false
				if rt != nil && strings.TrimSpace(r.traceID) != "" {
					rt.ClearRequestContexts(strings.TrimSpace(r.traceID))
					cancelled = true
				}
				r.resCh <- cancelled
			case hookEventReq:
				if rt == nil {
					continue
				}
				switch strings.TrimSpace(r.event) {
				case "WorktreeCreate":
					rt.EmitWorktreeCreate(r.ctx, r.path)
				case "WorktreeRemove":
					rt.EmitWorktreeRemove(r.ctx, r.path)
				}
			}
		case <-rc.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (rc *RuntimeCore) finalizeTask(rt *einoruntime.EinoRuntime, traceID string, userText string, assistantText string, success bool, errorMsg string) {
	traceID = strings.TrimSpace(traceID)
	userText = strings.TrimSpace(userText)
	assistantText = strings.TrimSpace(assistantText)
	errorMsg = strings.TrimSpace(errorMsg)

	if traceID != "" && rt != nil {
		rt.ClearRequestContexts(traceID)
	}

	toolObs, toolSummaries, _ := rc.cm.DrainAndClearToolContext()

	durationMs := 0
	toolCalls := map[string]int{}
	toolErrors := map[string]int{}
	model := ""

	if traceID != "" {
		rc.metricsMu.Lock()
		if rc.inflightReq != nil {
			if m := rc.inflightReq[traceID]; m != nil {
				if !m.StartedAt.IsZero() {
					durationMs = int(time.Since(m.StartedAt).Milliseconds())
				}
				model = strings.TrimSpace(m.Model)
				for k, v := range m.ToolCalls {
					toolCalls[k] = v
				}
				for k, v := range m.ToolCallsError {
					toolErrors[k] = v
				}
			}
		}
		if len(toolCalls) == 0 {
			for i := len(rc.reqHistory) - 1; i >= 0; i-- {
				if rc.reqHistory[i].ID != traceID {
					continue
				}
				if rc.reqHistory[i].Duration > 0 {
					durationMs = int(rc.reqHistory[i].Duration.Milliseconds())
				}
				model = strings.TrimSpace(rc.reqHistory[i].Model)
				for k, v := range rc.reqHistory[i].ToolCalls {
					toolCalls[k] = v
				}
				for k, v := range rc.reqHistory[i].ToolCallsError {
					toolErrors[k] = v
				}
				break
			}
		}
		rc.metricsMu.Unlock()
	}

	paths := extractSummaryPaths(toolObs, toolSummaries)

	var toolParts []string
	if len(toolCalls) > 0 {
		keys := make([]string, 0, len(toolCalls))
		for k := range toolCalls {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			toolParts = append(toolParts, k+"="+strconv.Itoa(toolCalls[k]))
			if len(toolParts) >= 8 {
				break
			}
		}
	}

	userShort := userText
	if len(userShort) > 180 {
		userShort = userShort[:180] + "…"
	}
	assistantShort := sanitizeTaskSummaryAssistantText(assistantText)
	if len(assistantShort) > 380 {
		assistantShort = assistantShort[:380] + "…"
	}

	ts := time.Now().Format("2006-01-02 15:04:05")
	var b strings.Builder
	b.WriteString("- " + ts)
	if traceID != "" {
		b.WriteString(" (trace_id=" + traceID + ")")
	}
	b.WriteString("\n  - 用户: " + userShort)
	if assistantShort != "" {
		b.WriteString("\n  - 结果: " + assistantShort)
	}
	if durationMs > 0 {
		b.WriteString("\n  - 耗时: " + strconv.Itoa(durationMs) + "ms")
	}
	if len(toolParts) > 0 {
		b.WriteString("\n  - 工具: " + strings.Join(toolParts, ", "))
	}
	if len(paths) > 0 {
		max := 10
		if len(paths) < max {
			max = len(paths)
		}
		b.WriteString("\n  - 关键路径: " + strings.Join(paths[:max], ", "))
	}

	if rc != nil && rc.cm != nil {
		rc.cm.AppendTaskSummary(b.String())
	}

	if rt != nil {
		meta := map[string]any{
			"trace_id":    traceID,
			"duration_ms": durationMs,
			"model":       model,
			"tool_calls":  toolCalls,
			"tool_errors": toolErrors,
			"paths":       paths,
		}
		_ = rt.EmitTaskCompleted(tools.WithTraceID(context.Background(), traceID), userText, success, errorMsg, meta)
	}
	if traceID != "" {
		_ = fileops.UpdateCheckpointSummary(traceID, userText, success)
	}

	mode := ""
	if rc != nil && rc.securityMgr != nil {
		mode = rc.securityMgr.ExecutionMode()
	}
	desktopEnabled := true
	if rc != nil {
		s := rc.GetSettings()
		if s.DesktopNotifications != nil {
			desktopEnabled = *s.DesktopNotifications
		}
	}
	st := "完成"
	if !success {
		st = "失败"
	}
	msg := st
	if strings.TrimSpace(mode) != "" {
		msg = fmt.Sprintf("%s（%s）", msg, strings.TrimSpace(mode))
	}
	if strings.TrimSpace(userShort) != "" {
		msg = msg + "\n" + userShort
	}
	if desktopEnabled {
		notify.NotifyAsync("EOS", msg)
	}
}

func sanitizeTaskSummaryAssistantText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = thinkBlockPattern.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text
}

func extractSummaryPaths(toolObs []string, toolSummaries []string) []string {
	seen := map[string]struct{}{}
	var out []string

	add := func(p string) {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"")
		if p == "" {
			return
		}
		if len(p) > 260 {
			p = p[:260] + "…"
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	scan := func(s string) {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		for _, line := range strings.Split(s, "\n") {
			t := strings.TrimSpace(line)
			for _, k := range []string{"path:", "file:", "candidate:"} {
				if strings.HasPrefix(t, k) {
					add(strings.TrimSpace(strings.TrimPrefix(t, k)))
				}
			}
			if strings.HasPrefix(t, "@") && len(t) > 1 {
				add(strings.TrimSpace(strings.TrimPrefix(t, "@")))
			}
		}
	}

	for _, s := range toolObs {
		scan(s)
	}
	for _, s := range toolSummaries {
		scan(s)
	}
	sort.Strings(out)
	return out
}

// getStackTrace 获取 panic 堆栈信息
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := stdruntime.Stack(buf, false)
	return string(buf[:n])
}

// cleanupPendingRequests 清理待处理的请求，用于 panic 恢复时
func (rc *RuntimeCore) cleanupPendingRequests() {
	slog.Warn("runtime.loop.cleanup_pending_requests",
		"component", utils.ComponentSystem,
	)

	rc.pendingMu.Lock()
	reloadChs := make([]chan error, 0, len(rc.pendingReload))
	for ch := range rc.pendingReload {
		reloadChs = append(reloadChs, ch)
	}
	graphChs := make([]chan graphInvokeRes, 0, len(rc.pendingGraph))
	for ch := range rc.pendingGraph {
		graphChs = append(graphChs, ch)
	}
	toolsChs := make([]chan toolsNodeRes, 0, len(rc.pendingTools))
	for ch := range rc.pendingTools {
		toolsChs = append(toolsChs, ch)
	}
	sumChs := make([]chan summarizeRes, 0, len(rc.pendingSumm))
	for ch := range rc.pendingSumm {
		sumChs = append(sumChs, ch)
	}
	predChs := make([]chan predictNextRes, 0, len(rc.pendingPred))
	for ch := range rc.pendingPred {
		predChs = append(predChs, ch)
	}
	rc.pendingReload = make(map[chan error]struct{})
	rc.pendingGraph = make(map[chan graphInvokeRes]struct{})
	rc.pendingTools = make(map[chan toolsNodeRes]struct{})
	rc.pendingSumm = make(map[chan summarizeRes]struct{})
	rc.pendingPred = make(map[chan predictNextRes]struct{})
	rc.pendingMu.Unlock()

	for _, ch := range reloadChs {
		select {
		case ch <- ErrRuntimeLoopUnavailable:
		default:
		}
	}
	for _, ch := range graphChs {
		select {
		case ch <- graphInvokeRes{msg: nil, err: ErrRuntimeLoopUnavailable}:
		default:
		}
	}
	for _, ch := range toolsChs {
		select {
		case ch <- toolsNodeRes{results: nil, executed: false, cont: false}:
		default:
		}
	}
	for _, ch := range sumChs {
		select {
		case ch <- summarizeRes{text: "", err: ErrRuntimeLoopUnavailable}:
		default:
		}
	}
	for _, ch := range predChs {
		select {
		case ch <- predictNextRes{text: "", err: ErrRuntimeLoopUnavailable}:
		default:
		}
	}
}

// Shutdown 优雅关闭运行时，等待所有 goroutine 完成
func (rc *RuntimeCore) Shutdown() {
	// 释放会话锁（正常退出时清除 lock 文件）
	rc.releaseHeldSessionLock()

	// 关闭 LSP 管理器
	rc.ShutdownLSPManager(rc.lspManager)

	// 关闭 done 通道，通知 loop 退出
	select {
	case <-rc.done:
		// 已经关闭
	default:
		close(rc.done)
	}
	// 等待所有 goroutine 完成，包括主 loop
	rc.wg.Wait()
	slog.Info("runtime.shutdown.completed",
		"component", utils.ComponentSystem,
	)
}
