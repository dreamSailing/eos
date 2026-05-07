package scheduler

import (
	"fmt"
	"strings"

	"github.com/dreamSailing/eos/internal/tools/bg"
)

func runShellSchedule(item Schedule, workspace string) error {
	command, _ := item.Payload["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("shell command required")
	}
	_, err := bg.Default().Start(command, &bg.StartOptions{WorkingDir: workspace, LogCap: 2000})
	return err
}
