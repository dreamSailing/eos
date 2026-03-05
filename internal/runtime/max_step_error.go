package runtime

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
		return fmt.Errorf("执行超过最大步骤限制（可在 ~/.vb.json 配置 agent.max_step，建议 40~80）：%w", err)
	}
	return err
}
