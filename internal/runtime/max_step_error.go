package runtime

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"fmt"
	"strings"
)

func wrapMaxStepError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "exceeds max steps") || strings.Contains(msg, "exceed max steps") || (strings.Contains(msg, "max steps") && strings.Contains(msg, "exceed")) {
		return fmt.Errorf("执行超过最大步骤限制（可在 ~/.eos.json 配置 agent.max_step；当前默认 160，复杂任务可适当提高）：%w", err)
	}
	return err
}
