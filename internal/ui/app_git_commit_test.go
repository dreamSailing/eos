package ui

// app_git_commit_test.go — 提交提醒文案组装与直派守卫的单元测试。

import "testing"

func TestGitCommitHintText(t *testing.T) {
	cases := []struct {
		name  string
		lang  string
		dirty int
		ahead int
		want  string
	}{
		{name: "dirty only zh", lang: "zh", dirty: 3, ahead: 0, want: "工作区有 3 个未提交文件，输入 /commit 让我提交推送"},
		{name: "ahead only zh", lang: "zh", dirty: 0, ahead: 2, want: "2 个提交未推送，输入 /commit 让我提交推送"},
		{name: "both zh", lang: "zh", dirty: 3, ahead: 2, want: "工作区有 3 个未提交文件、2 个提交未推送，输入 /commit 让我提交推送"},
		{name: "both en", lang: "en", dirty: 1, ahead: 1, want: "1 uncommitted file(s) in the workspace, 1 commit(s) not pushed — type /commit and I will commit and push"},
	}
	for _, tc := range cases {
		if got := gitCommitHintText(tc.lang, tc.dirty, tc.ahead); got != tc.want {
			t.Errorf("%s: gitCommitHintText() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDispatchGitCommitRequestBlockedWhileProcessing(t *testing.T) {
	app := newTestAppModel(t)
	app.state.Processing = true
	if cmd := app.dispatchGitCommitRequest(); cmd != nil {
		t.Fatal("dispatch must be a no-op while a turn is processing")
	}
}

func TestSendMessageTextIgnoresBlankDraft(t *testing.T) {
	app := newTestAppModel(t)
	if cmd := app.sendMessageText("   ", false); cmd != nil {
		t.Fatal("blank programmatic draft must not dispatch")
	}
}
