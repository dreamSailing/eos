package bridge

import (
	"context"
	"strings"
)

func (rc *RuntimeCore) MaybeSummarizeConversation(ctx context.Context) {
	if rc == nil || ctx == nil || rc.cm == nil {
		return
	}

	rc.summaryMu.Lock()
	if rc.conversationSummaryRunning {
		rc.summaryMu.Unlock()
		return
	}
	if rc.conversationSummaryEvery <= 0 {
		rc.summaryMu.Unlock()
		return
	}
	rounds := len(rc.GetTokenHistory())
	if rounds == 0 || rounds-rc.lastConversationSummaryRound < rc.conversationSummaryEvery {
		rc.summaryMu.Unlock()
		return
	}
	rc.conversationSummaryRunning = true
	rc.summaryMu.Unlock()

	defer func() {
		rc.summaryMu.Lock()
		rc.conversationSummaryRunning = false
		rc.lastConversationSummaryRound = rounds
		rc.summaryMu.Unlock()
	}()

	msgs := rc.cm.BuildPreview()
	var lines []string
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		s := strings.TrimSpace(m.Content)
		if s == "" {
			continue
		}
		if len(s) > 600 {
			s = s[:600] + "…"
		}
		lines = append([]string{"[" + m.Role + "] " + s}, lines...)
		if len(lines) >= 24 {
			break
		}
	}
	if len(lines) == 0 {
		return
	}

	var b strings.Builder
	b.WriteString("请把下面对话片段压缩成可复用的全局摘要，要求：\n")
	b.WriteString("- 中文输出\n")
	b.WriteString("- 以要点列表给出：目标、已完成工作、关键约束、未解决问题\n")
	b.WriteString("- 保留文件名/函数名/关键命令\n\n")
	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n")

	out, err := rc.Summarize(ctx, b.String())
	if err != nil {
		return
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return
	}
	rc.cm.SetConversationSummary(out)
}
