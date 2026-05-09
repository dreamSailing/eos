package session

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"github.com/dreamSailing/eos/internal/ai"
)

type ContextState struct {
	Pinned              []ai.Message
	Recent              []ai.Message
	Tools               []string
	ToolObs             []string
	Ephem               []string
	CurrentFull         []ai.Message
	LastPlan            string
	RecentRounds        int
	ToolLimit           int
	MaxChars            int
	ModelName           string
	MaxPromptTokens     int
	ReservedReplyTokens int

	CompressionStrategy CompressionStrategy
	CompressionStats    CompressionStats
	AutoCompressEnabled bool
	CompressThreshold   float64
	MaxSnapshots        int
}

func (c *ContextManager) ExportState() ContextState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return ContextState{
		Pinned:              c.copyMessagesLocked(c.pinned),
		Recent:              c.copyMessagesLocked(c.recent),
		Tools:               append([]string{}, c.tools...),
		ToolObs:             append([]string{}, c.toolObs...),
		Ephem:               append([]string{}, c.ephem...),
		CurrentFull:         c.copyMessagesLocked(c.currentFull),
		LastPlan:            c.lastPlan,
		RecentRounds:        c.recentRounds,
		ToolLimit:           c.toolLimit,
		MaxChars:            c.maxChars,
		ModelName:           c.modelName,
		MaxPromptTokens:     c.maxPromptTokens,
		ReservedReplyTokens: c.reservedReplyTokens,
		CompressionStrategy: c.compressionStrategy,
		CompressionStats:    c.compressionStats,
		AutoCompressEnabled: c.autoCompressEnabled,
		CompressThreshold:   c.compressThreshold,
		MaxSnapshots:        c.maxSnapshots,
	}
}

func (c *ContextManager) ImportState(st ContextState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pinned = c.copyMessagesLocked(st.Pinned)
	c.recent = c.copyMessagesLocked(st.Recent)
	c.tools = append([]string{}, st.Tools...)
	c.toolObs = append([]string{}, st.ToolObs...)
	c.ephem = append([]string{}, st.Ephem...)
	c.currentFull = c.copyMessagesLocked(st.CurrentFull)
	c.lastPlan = st.LastPlan

	if st.RecentRounds > 0 {
		c.recentRounds = st.RecentRounds
	}
	if st.ToolLimit > 0 {
		c.toolLimit = st.ToolLimit
	}
	if st.MaxChars > 0 {
		c.maxChars = st.MaxChars
		c.maxPromptTokens = st.MaxChars / 4
	}
	if st.ModelName != "" {
		c.setModelLocked(st.ModelName)
	} else if st.MaxPromptTokens > 0 {
		c.maxPromptTokens = st.MaxPromptTokens
	}
	if st.ReservedReplyTokens > 0 {
		c.reservedReplyTokens = st.ReservedReplyTokens
	}

	c.compressionStrategy = st.CompressionStrategy
	c.compressionStats = st.CompressionStats
	c.autoCompressEnabled = st.AutoCompressEnabled
	if st.CompressThreshold > 0 {
		c.compressThreshold = st.CompressThreshold
	}
	if st.MaxSnapshots > 0 {
		c.maxSnapshots = st.MaxSnapshots
	}
}
