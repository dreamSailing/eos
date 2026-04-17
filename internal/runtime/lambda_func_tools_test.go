package runtime

import (
	"testing"
	"github.com/dreamSailing/eos/internal/tools"
)

func TestAllowedTools_IncludeGitShow(t *testing.T) {
	roles := []string{"architect", "planner", "reviewer", "tester", "senior-dev"}
	for _, role := range roles {
		allowed := AllowedTools(role)
		if !allowed[tools.ToolGitShow] {
			t.Fatalf("role %s missing %s", role, tools.ToolGitShow)
		}
	}
}
