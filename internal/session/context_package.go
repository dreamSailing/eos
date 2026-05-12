package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strconv"
	"strings"

	"github.com/dreamSailing/eos/internal/ai"
)

const (
	auxContextBudgetSharePct = 45
	minAuxContextTokens      = 192
	minTrimEntryTokens       = 64
)

type budgetedContextEntry struct {
	label   string
	content string
}

func (c *ContextManager) appendBudgetedAuxContextLocked(msgs []ai.Message) []ai.Message {
	entries := c.buildBudgetedAuxEntriesLocked()
	if len(entries) == 0 {
		return msgs
	}

	remaining := c.maxPromptTokens - c.estimateMessagesTokensLocked(msgs)
	if remaining <= 0 {
		return msgs
	}

	auxBudget := remaining
	if capTokens := c.maxPromptTokens * auxContextBudgetSharePct / 100; capTokens > 0 && auxBudget > capTokens {
		auxBudget = capTokens
	}
	if auxBudget < minAuxContextTokens {
		if remaining < minAuxContextTokens {
			auxBudget = remaining
		} else {
			auxBudget = minAuxContextTokens
		}
	}
	if auxBudget <= 0 {
		return msgs
	}

	summaryReserve := auxBudget / 8
	if summaryReserve < 32 {
		summaryReserve = 32
	}
	if summaryReserve > 128 {
		summaryReserve = 128
	}

	included := 0
	trimmed := 0
	omitted := 0
	omittedLabels := make([]string, 0, 4)

	for idx, entry := range entries {
		if auxBudget <= 0 {
			omitted++
			omittedLabels = appendOmittedLabel(omittedLabels, entry.label)
			continue
		}

		entryBudget := auxBudget
		if idx < len(entries)-1 && entryBudget > summaryReserve {
			entryBudget -= summaryReserve
		}
		if entryBudget < minTrimEntryTokens {
			omitted++
			omittedLabels = appendOmittedLabel(omittedLabels, entry.label)
			continue
		}

		entryTokens := c.estimateTextTokensLocked(entry.content)
		content := entry.content
		if entryTokens > entryBudget {
			var ok bool
			content, ok = c.trimContextEntryToBudgetLocked(entry.content, entryBudget)
			if !ok {
				omitted++
				omittedLabels = appendOmittedLabel(omittedLabels, entry.label)
				continue
			}
			entryTokens = c.estimateTextTokensLocked(content)
			trimmed++
		}

		if entryTokens > auxBudget {
			omitted++
			omittedLabels = appendOmittedLabel(omittedLabels, entry.label)
			continue
		}

		msgs = append(msgs, ai.Message{Role: "system", Content: content})
		auxBudget -= entryTokens
		included++
	}

	summary := formatContextPackageSummary(included, trimmed, omitted, omittedLabels)
	if strings.TrimSpace(summary) == "" || auxBudget <= 0 {
		return msgs
	}

	summaryTokens := c.estimateTextTokensLocked(summary)
	if summaryTokens <= auxBudget {
		return append(msgs, ai.Message{Role: "system", Content: summary})
	}

	if trimmedSummary, ok := c.trimContextEntryToBudgetLocked(summary, auxBudget); ok {
		return append(msgs, ai.Message{Role: "system", Content: trimmedSummary})
	}
	return msgs
}

func (c *ContextManager) buildBudgetedAuxEntriesLocked() []budgetedContextEntry {
	entries := make([]budgetedContextEntry, 0, len(c.currentFull)+len(c.toolObs)+len(c.tools))
	for idx, msg := range c.currentFull {
		content := strings.TrimSpace(msg.Content)
		if content == "" || c.shouldSnip(msg) {
			continue
		}
		entries = append(entries, budgetedContextEntry{
			label:   contextEntryLabel(content, "context file", idx+1),
			content: content,
		})
	}
	for idx, raw := range c.toolObs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		entries = append(entries, budgetedContextEntry{
			label:   "tool observation " + strconv.Itoa(idx+1),
			content: raw,
		})
	}
	for idx, raw := range c.tools {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		entries = append(entries, budgetedContextEntry{
			label:   "tool summary " + strconv.Itoa(idx+1),
			content: raw,
		})
	}
	return entries
}

func (c *ContextManager) trimContextEntryToBudgetLocked(content string, budgetTokens int) (string, bool) {
	content = strings.TrimSpace(content)
	if content == "" || budgetTokens < minTrimEntryTokens {
		return "", false
	}
	if c.estimateTextTokensLocked(content) <= budgetTokens {
		return content, true
	}

	const marker = "\n...[trimmed to fit prompt budget]"

	prefix := ""
	body := content
	if strings.HasPrefix(content, "@") {
		if cut := strings.Index(content, "\n"); cut > 0 {
			prefix = content[:cut+1]
			body = content[cut+1:]
		}
	}

	minCandidate := strings.TrimSpace(prefix + marker)
	if c.estimateTextTokensLocked(minCandidate) > budgetTokens {
		return "", false
	}

	bodyRunes := []rune(body)
	best := minCandidate
	lo := 0
	hi := len(bodyRunes)
	for lo <= hi {
		mid := (lo + hi) / 2
		candidate := strings.TrimSpace(prefix + string(bodyRunes[:mid]) + marker)
		if c.estimateTextTokensLocked(candidate) <= budgetTokens {
			best = candidate
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best, true
}

func contextEntryLabel(content, fallback string, index int) string {
	firstLine := strings.TrimSpace(content)
	if cut := strings.Index(firstLine, "\n"); cut >= 0 {
		firstLine = strings.TrimSpace(firstLine[:cut])
	}
	if strings.HasPrefix(firstLine, "@") {
		firstLine = strings.TrimSpace(strings.TrimPrefix(firstLine, "@"))
	}
	if firstLine == "" {
		return fallback + " " + strconv.Itoa(index)
	}
	if len(firstLine) > 64 {
		firstLine = firstLine[:61] + "..."
	}
	return firstLine
}

func appendOmittedLabel(labels []string, label string) []string {
	if strings.TrimSpace(label) == "" || len(labels) >= 4 {
		return labels
	}
	return append(labels, label)
}

func formatContextPackageSummary(included, trimmed, omitted int, omittedLabels []string) string {
	if trimmed == 0 && omitted == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Context package: included ")
	sb.WriteString(strconv.Itoa(included))
	sb.WriteString(" item(s)")
	if trimmed > 0 {
		sb.WriteString(", trimmed ")
		sb.WriteString(strconv.Itoa(trimmed))
	}
	if omitted > 0 {
		sb.WriteString(", omitted ")
		sb.WriteString(strconv.Itoa(omitted))
	}
	if len(omittedLabels) > 0 {
		sb.WriteString(" [")
		sb.WriteString(strings.Join(omittedLabels, ", "))
		if omitted > len(omittedLabels) {
			sb.WriteString(", +")
			sb.WriteString(strconv.Itoa(omitted - len(omittedLabels)))
		}
		sb.WriteString("]")
	}
	return sb.String()
}
