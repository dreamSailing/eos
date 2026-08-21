package setup

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"testing"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/ui/styles"
	"github.com/dreamSailing/eos/pkg/coreapi"

	tea "github.com/charmbracelet/bubbletea"
)

// newPlanConfigWizard 构造处于配置步骤、选中套餐类 preset（两个可选模型）的向导。
func newPlanConfigWizard() *ModelSetupView {
	v := NewModelSetupWizard(styles.NewStyles(styles.DefaultDarkTheme()), "zh")
	model := &ai.ModelCatalogEntry{
		ID:        "test-plan",
		Name:      "Test Plan",
		ModelName: "model-a",
		PlanModels: []coreapi.PlanModel{
			{ModelID: "model-a", Label: "A"},
			{ModelID: "model-b", Label: "B"},
		},
	}
	v.step = ModelSetupStepConfig
	v.selectedModel = model
	v.planIndex = 0
	v.modelReadOnly = false
	v.apiBaseReadOnly = true
	v.inputs[3].SetValue("model-a")
	v.focusInput(0)
	return v
}

// assertTabSequence 逐个按键后校验 focusIndex 轨迹。
func assertTabSequence(t *testing.T, v *ModelSetupView, keyType tea.KeyType, want []int) {
	t.Helper()
	for _, w := range want {
		key := tea.KeyMsg{Type: keyType}
		v.Update(key)
		if v.focusIndex != w {
			t.Fatalf("after %s, focusIndex = %d, want %d", key.String(), v.focusIndex, w)
		}
	}
}

// 套餐类配置步骤：Tab 显示名(0)→API Key(2)→模型选择(3)后回绕到显示名(0)，
// Shift+Tab 反向同样回绕。回归：旧实现在模型选择处吞掉 Tab 导致焦点卡死。
func TestPlanConfigFocusCycles(t *testing.T) {
	v := newPlanConfigWizard()
	assertTabSequence(t, v, tea.KeyTab, []int{2, 3, 0})
	assertTabSequence(t, v, tea.KeyShiftTab, []int{3, 2, 0})
}

// 套餐内模型选择器：←/→ 切换模型并同步到表单值（handleSave 读 inputs[3]）。
func TestPlanPickerCyclesPlanModels(t *testing.T) {
	v := newPlanConfigWizard()
	v.Update(tea.KeyMsg{Type: tea.KeyTab})
	v.Update(tea.KeyMsg{Type: tea.KeyTab})
	if v.focusIndex != 3 {
		t.Fatalf("focusIndex = %d, want 3 (plan picker)", v.focusIndex)
	}

	v.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := v.inputs[3].Value(); got != "model-b" {
		t.Fatalf("after right, model = %q, want model-b", got)
	}
	v.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if got := v.inputs[3].Value(); got != "model-a" {
		t.Fatalf("after left, model = %q, want model-a", got)
	}
}

// 自定义服务商：四个字段全部参与循环（含 API Base）。
func TestCustomProviderFocusCycles(t *testing.T) {
	v := NewModelSetupWizard(styles.NewStyles(styles.DefaultDarkTheme()), "zh")
	v.step = ModelSetupStepCustom
	v.customProvider = true
	v.focusInput(0)

	assertTabSequence(t, v, tea.KeyTab, []int{1, 2, 3, 0})
	assertTabSequence(t, v, tea.KeyShiftTab, []int{3, 2, 1, 0})
}

// 自定义模型：API Base 只读，焦点在 显示名(0)→API Key(2)→模型名(3) 循环。
func TestCustomModelFocusSkipsAPIBase(t *testing.T) {
	v := NewModelSetupWizard(styles.NewStyles(styles.DefaultDarkTheme()), "zh")
	v.step = ModelSetupStepConfig
	v.customModel = true
	v.modelReadOnly = false
	v.apiBaseReadOnly = true
	v.focusInput(0)

	assertTabSequence(t, v, tea.KeyTab, []int{2, 3, 0})
	assertTabSequence(t, v, tea.KeyShiftTab, []int{3, 2, 0})
}

// 普通 preset：模型只读，焦点在 显示名(0)↔API Key(2) 循环。
func TestFixedPresetFocusCycles(t *testing.T) {
	v := NewModelSetupWizard(styles.NewStyles(styles.DefaultDarkTheme()), "zh")
	v.step = ModelSetupStepConfig
	v.selectedModel = &ai.ModelCatalogEntry{ID: "fixed", Name: "Fixed", ModelName: "fixed-model"}
	v.modelReadOnly = true
	v.apiBaseReadOnly = true
	v.focusInput(0)

	assertTabSequence(t, v, tea.KeyTab, []int{2, 0})
	assertTabSequence(t, v, tea.KeyShiftTab, []int{2, 0})
}
