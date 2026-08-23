package panels

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/ui/styles"
)

func newTestModelsPanel() *ModelsPanel {
	return NewModelsPanel(styles.NewStyles(styles.DefaultDarkTheme()), "zh")
}

// runKey 向面板发单个 KeyMsg，返回 cmd 执行出的消息与面板（用于断言）。
func runKey(t *testing.T, p *ModelsPanel, key string) (Panel, tea.Msg) {
	t.Helper()
	p2, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	p = p2.(*ModelsPanel)
	if cmd == nil {
		return p, nil
	}
	return p, cmd()
}

func TestModelsPanelShortcutKeys(t *testing.T) {
	panel := newTestModelsPanel()
	panel.SetModels([]config.ModelEntry{
		{Name: "MiniMax M3", Source: "user", APIBase: "https://api.minimaxi.com/v1", Model: "MiniMax-M3"},
		{Name: "GLM-5.2", Source: "user", APIBase: "https://ark.cn-beijing.volces.com", Model: "glm-5.2"},
	}, "MiniMax M3")

	// u：使用实际选中的模型
	_, msg := runKey(t, panel, "u")
	if _, ok := msg.(ModelSelectMsg); !ok {
		t.Fatalf("u -> got %T, want ModelSelectMsg", msg)
	}

	// a：添加
	panel = newTestModelsPanel()
	panel.SetModels([]config.ModelEntry{{Name: "MiniMax M3", Model: "MiniMax-M3"}}, "MiniMax M3")
	_, msg = runKey(t, panel, "a")
	if _, ok := msg.(ModelAddMsg); !ok {
		t.Fatalf("a -> got %T, want ModelAddMsg", msg)
	}

	// d：删除
	panel = newTestModelsPanel()
	panel.SetModels([]config.ModelEntry{{Name: "MiniMax M3", Model: "MiniMax-M3"}}, "MiniMax M3")
	_, msg = runKey(t, panel, "d")
	if _, ok := msg.(ModelDeleteMsg); !ok {
		t.Fatalf("d -> got %T, want ModelDeleteMsg", msg)
	}

	// r：刷新
	panel = newTestModelsPanel()
	panel.SetModels([]config.ModelEntry{{Name: "MiniMax M3", Model: "MiniMax-M3"}}, "MiniMax M3")
	_, msg = runKey(t, panel, "r")
	if _, ok := msg.(ModelRefreshMsg); !ok {
		t.Fatalf("r -> got %T, want ModelRefreshMsg", msg)
	}

	// s：同步（回归：help 声称 S: sync，此前无 case 分支）
	panel = newTestModelsPanel()
	panel.SetModels([]config.ModelEntry{{Name: "MiniMax M3", Model: "MiniMax-M3"}}, "MiniMax M3")
	_, msg = runKey(t, panel, "s")
	if _, ok := msg.(ModelSyncMsg); !ok {
		t.Fatalf("s -> got %T, want ModelSyncMsg", msg)
	}
	if p := panel.GetCurrentAction(); p != "Use" {
		t.Fatalf("actionOps = %q, want first Use with Refresh included", p)
	}
}
