package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

type TurnWrapUpState struct {
	Active         bool
	PromptInjected bool
	Reason         string
	ToolName       string
	Level          string
	Alternatives   string
	BlockedCalls   int
}

func (rt *EinoRuntime) resetTurnWrapUpState() {
	if rt == nil {
		return
	}
	rt.turnWrapUp = TurnWrapUpState{}
	if rt.loopDetector != nil {
		rt.loopDetector.Reset()
	}
}

func (rt *EinoRuntime) activateTurnWrapUp(result *LoopCheckResult) {
	if rt == nil || result == nil {
		return
	}
	rt.turnWrapUp.Active = true
	rt.turnWrapUp.Reason = strings.TrimSpace(result.Reason)
	rt.turnWrapUp.ToolName = strings.TrimSpace(result.ToolName)
	rt.turnWrapUp.Level = strings.TrimSpace(result.Level)
	rt.turnWrapUp.Alternatives = strings.TrimSpace(result.Alternatives)

	rt.emitLoopBlockEvent(result)
	if rt.turnWrapUp.PromptInjected {
		return
	}

	rt.turnWrapUp.PromptInjected = true
	prompt := buildTurnWrapUpPrompt(result)
	if rt.ctxm != nil && prompt != "" {
		rt.ctxm.AddEphemeral(prompt)
		rt.ctxm.AddToolObservation(map[string]any{
			"type":         "turn_wrap_up",
			"tool":         result.ToolName,
			"level":        result.Level,
			"reason":       result.Reason,
			"alternatives": result.Alternatives,
			"message":      "检测到重复步骤，停止继续试探工具，改为基于已有信息总结本轮结果。",
		})
	}
	rt.emitTurnWrapUpEvent(result)
}

func (rt *EinoRuntime) handleBlockedToolDuringWrapUp(toolName string) {
	if rt == nil {
		return
	}
	rt.turnWrapUp.BlockedCalls++
	result := &LoopCheckResult{
		Level:          LoopLevelForceBreak,
		ToolName:       strings.TrimSpace(toolName),
		Reason:         "wrap_up_active",
		Alternatives:   rt.turnWrapUp.Alternatives,
		WarnCount:      rt.turnWrapUp.BlockedCalls,
		WrapUpRequired: true,
	}
	rt.emitLoopBlockEvent(result)
}

func (rt *EinoRuntime) buildWrapUpToolResult(toolName string) map[string]any {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		toolName = rt.turnWrapUp.ToolName
	}
	return map[string]any{
		"type":         "turn_wrap_up_blocked",
		"tool":         toolName,
		"level":        firstNonEmpty(rt.turnWrapUp.Level, LoopLevelForceBreak),
		"reason":       firstNonEmpty(rt.turnWrapUp.Reason, "wrap_up_active"),
		"alternatives": rt.turnWrapUp.Alternatives,
		"message":      "当前轮已进入总结收束模式，请停止继续调用工具，直接基于已有信息总结已完成内容、阻塞点和下一步建议。",
	}
}

func buildTurnWrapUpPrompt(result *LoopCheckResult) string {
	if result == nil {
		return ""
	}
	toolName := strings.TrimSpace(result.ToolName)
	if toolName == "" {
		toolName = "当前工具链路"
	}
	return fmt.Sprintf(
		"TURN_WRAP_UP: 检测到重复步骤，已停止继续调用工具。\n原因: %s\n触发工具: %s\n要求:\n1. 不要再调用任何工具\n2. 基于已有信息总结已完成内容\n3. 明确说明当前阻塞或缺失信息\n4. 给出用户下一步建议\n可选替代方案: %s",
		strings.TrimSpace(result.Reason),
		toolName,
		strings.TrimSpace(result.Alternatives),
	)
}

func (rt *EinoRuntime) emitLoopBlockEvent(result *LoopCheckResult) {
	if rt == nil || rt.onMeta == nil || result == nil {
		return
	}
	payload := map[string]any{
		"type":         EventLoopBlock,
		"tool":         strings.TrimSpace(result.ToolName),
		"level":        strings.TrimSpace(result.Level),
		"reason":       strings.TrimSpace(result.Reason),
		"alternatives": strings.TrimSpace(result.Alternatives),
		"message":      "检测到重复步骤，已阻止继续调用并准备总结现有结果",
	}
	if result.SameSignatureCount > 0 {
		payload["same_signature_count"] = result.SameSignatureCount
	}
	if result.PatternLength > 0 {
		payload["pattern_length"] = result.PatternLength
	}
	if result.WarnCount > 0 {
		payload["warn_count"] = result.WarnCount
	}
	if result.MaxWarns > 0 {
		payload["max_warns"] = result.MaxWarns
	}
	rt.emitStructuredMeta(payload)
}

func (rt *EinoRuntime) emitTurnWrapUpEvent(result *LoopCheckResult) {
	if rt == nil || rt.onMeta == nil || result == nil {
		return
	}
	rt.emitStructuredMeta(map[string]any{
		"type":         EventTurnWrapUp,
		"tool":         strings.TrimSpace(result.ToolName),
		"level":        strings.TrimSpace(result.Level),
		"reason":       strings.TrimSpace(result.Reason),
		"alternatives": strings.TrimSpace(result.Alternatives),
		"message":      "检测到重复步骤，停止继续试探工具，正在基于已有结果总结",
	})
}

func (rt *EinoRuntime) emitStructuredMeta(payload map[string]any) {
	if rt == nil || rt.onMeta == nil || len(payload) == 0 {
		return
	}
	bs, err := json.Marshal(payload)
	if err != nil {
		slog.Error("runtime.emit_structured_meta.marshal_error", "error", err)
		return
	}
	rt.onMeta(string(bs))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
