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

func TestExtractVerificationVerdict(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "pass", text: "VERDICT: PASS\n- 覆盖关键路径", want: "PASS"},
		{name: "fail", text: "VERDICT: FAIL\n- 首页打不开", want: "FAIL"},
		{name: "partial with full width colon", text: "VERDICT： PARTIAL\n- 缺少边界验证", want: "PARTIAL"},
		{name: "missing", text: "未给出 verdict", want: ""},
	}
	for _, tc := range cases {
		if got := extractVerificationVerdict(tc.text); got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseVerificationDetails(t *testing.T) {
	text := "VERDICT: PARTIAL\n覆盖到的验证项:\n- 登录成功\n- 首页可打开\n未覆盖的风险和空白:\n- 弱网重试未覆盖\n关键证据:\n- npm test\n- curl /health"
	got := parseVerificationDetails(text)
	if got.Summary != "VERDICT: PARTIAL" {
		t.Fatalf("summary = %q, want VERDICT: PARTIAL", got.Summary)
	}
	if len(got.CoveredChecks) != 2 || got.CoveredChecks[0] != "登录成功" || got.CoveredChecks[1] != "首页可打开" {
		t.Fatalf("covered = %#v", got.CoveredChecks)
	}
	if len(got.OpenRisks) != 1 || got.OpenRisks[0] != "弱网重试未覆盖" {
		t.Fatalf("risks = %#v", got.OpenRisks)
	}
	if len(got.Evidence) != 2 || got.Evidence[0] != "npm test" || got.Evidence[1] != "curl /health" {
		t.Fatalf("evidence = %#v", got.Evidence)
	}
}

func TestParseVerificationDetailsStructuredSummaryHeading(t *testing.T) {
	text := "VERDICT: FAIL\n验收摘要:\n- 首屏仍然白屏\n覆盖到的验证项:\n- 登录接口可达\n未覆盖的风险和空白:\n- 回退链路未验证"
	got := parseVerificationDetails(text)
	if got.Summary != "VERDICT: FAIL" {
		t.Fatalf("summary = %q, want VERDICT: FAIL", got.Summary)
	}
	if len(got.CoveredChecks) != 1 || got.CoveredChecks[0] != "登录接口可达" {
		t.Fatalf("covered = %#v", got.CoveredChecks)
	}
	if len(got.OpenRisks) != 1 || got.OpenRisks[0] != "回退链路未验证" {
		t.Fatalf("risks = %#v", got.OpenRisks)
	}
}
