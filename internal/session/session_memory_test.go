package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractSessionMemoryWritesSessionAndLongTermMemory(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	root := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cm := NewContextManager()
	cm.AddUser("默认使用中文回答，优先给出最小改动方案。")
	cm.AddAssistant("已记录用户偏好。")
	cm.AddAssistant("项目约定：长期记忆统一写入 .eos/memory/project.md。")

	if err := cm.ExtractSessionMemory(context.Background()); err != nil {
		t.Fatalf("ExtractSessionMemory failed: %v", err)
	}

	sessionPath := filepath.Join(root, ".eos", "session-memory", "session.md")
	sessionContent, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read session memory failed: %v", err)
	}
	if !strings.Contains(string(sessionContent), "默认使用中文回答") {
		t.Fatalf("session memory missing recent conversation summary")
	}

	globalPath := filepath.Join(home, ".eos", "memory", "user.md")
	globalContent, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("read global memory failed: %v", err)
	}
	if !strings.Contains(string(globalContent), "默认使用中文回答") {
		t.Fatalf("global memory missing extracted preference")
	}

	projectPath := filepath.Join(root, ".eos", "memory", "project.md")
	projectContent, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatalf("read project memory failed: %v", err)
	}
	if !strings.Contains(string(projectContent), "项目约定：长期记忆统一写入 .eos/memory/project.md") {
		t.Fatalf("project memory missing extracted convention")
	}
}
