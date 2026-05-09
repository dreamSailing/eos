package ai

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import "testing"

func TestResolveProviderAndModelPrefersBaseSpecificPreset(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		modelName    string
		wantType     APIType
		wantProvider ProviderType
	}{
		{
			name:         "zhipu claude endpoint",
			baseURL:      "https://open.bigmodel.cn/api/anthropic",
			modelName:    "glm-5",
			wantType:     APITypeClaude,
			wantProvider: ProviderZhipu,
		},
		{
			name:         "zhipu coding endpoint",
			baseURL:      "https://open.bigmodel.cn/api/coding/paas/v4",
			modelName:    "glm-5",
			wantType:     APITypeCodePlan,
			wantProvider: ProviderZhipu,
		},
		{
			name:         "dashscope coding endpoint",
			baseURL:      "https://coding.dashscope.aliyuncs.com/v1",
			modelName:    "qwen3.6-plus",
			wantType:     APITypeCodePlan,
			wantProvider: ProviderDashScope,
		},
		{
			name:         "minimax claude endpoint",
			baseURL:      "https://api.minimaxi.com/anthropic/v1",
			modelName:    "MiniMax-M2.7",
			wantType:     APITypeClaude,
			wantProvider: ProviderMiniMax,
		},
		{
			name:         "mimo claude endpoint",
			baseURL:      "https://token-plan-cn.xiaomimimo.com/anthropic",
			modelName:    "mimo-v2-pro",
			wantType:     APITypeClaude,
			wantProvider: ProviderMiMo,
		},
		{
			name:         "mimo openai endpoint",
			baseURL:      "https://token-plan-cn.xiaomimimo.com/v1",
			modelName:    "mimo-v2-omni",
			wantType:     APITypeCodePlan,
			wantProvider: ProviderMiMo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ResolveProviderAndModel(tt.baseURL, tt.modelName)
			if info == nil || info.Model == nil {
				t.Fatalf("ResolveProviderAndModel(%q, %q) returned nil model", tt.baseURL, tt.modelName)
			}
			if info.ProviderType != tt.wantProvider {
				t.Fatalf("provider = %q, want %q", info.ProviderType, tt.wantProvider)
			}
			if info.Model.APIType != tt.wantType {
				t.Fatalf("apiType = %q, want %q", info.Model.APIType, tt.wantType)
			}
		})
	}
}
