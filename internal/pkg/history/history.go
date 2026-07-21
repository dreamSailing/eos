package history

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"strings"
)

type Manager struct {
	historyAI   []string
	historyBash []string
	limit       int
	index       int
	shadow      string
}

func NewManager(limit int) *Manager {
	if limit <= 0 {
		limit = 100
	}
	return &Manager{
		limit: limit,
		index: -1,
	}
}

func (m *Manager) Add(text string, isBash bool) {
	s := strings.TrimSpace(text)
	if s == "" {
		return
	}

	hs := &m.historyAI
	if isBash {
		hs = &m.historyBash
	}

	// 避免重复添加相同的最后一条记录
	if len(*hs) > 0 && (*hs)[len(*hs)-1] == s {
		return
	}

	// 添加新记录
	*hs = append(*hs, s)

	// 限制历史记录长度
	if len(*hs) > m.limit {
		*hs = (*hs)[len(*hs)-m.limit:]
	}

	// 重置历史导航状态
	m.index = -1
	m.shadow = ""
}

func (m *Manager) Navigate(dir int, currentText string, isBash bool) (string, bool) {
	hs := m.historyAI
	if isBash {
		hs = m.historyBash
	}

	if len(hs) == 0 {
		return "", false
	}

	if dir < 0 {
		// 向上导航（更旧的记录）
		if m.index == -1 {
			// 第一次向上导航，保存当前输入
			m.shadow = currentText
			m.index = len(hs) - 1
		} else if m.index > 0 {
			// 继续向上
			m.index--
		}
		return hs[m.index], true
	}

	// 向下导航（更新的记录）
	if m.index == -1 {
		return "", false
	}

	if m.index < len(hs)-1 {
		// 还有更新的记录
		m.index++
		return hs[m.index], true
	}

	// 到达最新记录，恢复用户原始输入
	m.index = -1
	res := m.shadow
	m.shadow = ""
	return res, true
}

func (m *Manager) ResetIndex() {
	m.index = -1
	m.shadow = ""
}
