package runtime

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestDispatchToolsResolveVerificationAgentRuntime(t *testing.T) {
	dt := &DispatchTools{}
	role, agent := dt.resolveAgentRuntime(SubAgentTypeVerification)
	if role != "verification" {
		t.Fatalf("role = %q, want verification", role)
	}
	if agent != nil {
		t.Fatalf("agent = %v, want nil when verification agent is not wired", agent)
	}
}

func TestInvokeVerificationDirectRequiresVerificationAgent(t *testing.T) {
	dt := &DispatchTools{}
	_, err := dt.InvokeVerificationDirect("检查关键路径", []*schema.Message{schema.AssistantMessage("实现完成", nil)})
	if err == nil {
		t.Fatal("expected error when verification agent is not initialized")
	}
	if err.Error() != "verification agent not initialized" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShouldAutoVerifySeniorDevTask(t *testing.T) {
	cases := []struct {
		name string
		task string
		want bool
	}{
		{name: "fix task", task: "修复附件文件名截断，完成标准：显示完整", want: true},
		{name: "implement task", task: "实现 /verify 命令并补测试", want: true},
		{name: "analysis task", task: "分析代码质量并给出改进建议", want: false},
		{name: "summary task", task: "梳理当前 verifier 架构并总结差异", want: false},
	}

	for _, tc := range cases {
		if got := shouldAutoVerifySeniorDevTask(tc.task); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCombineImplementationAndVerificationResult(t *testing.T) {
	got := combineImplementationAndVerificationResult("实现已完成", "VERDICT: PASS")
	if got != "IMPLEMENTATION_RESULT:\n实现已完成\n\nVERIFICATION_RESULT:\nVERDICT: PASS" {
		t.Fatalf("unexpected combined result: %q", got)
	}
}
