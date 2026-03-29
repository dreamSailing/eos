package bridge

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func buildGitContextHint(startRoot string) string {
	root := findGitRoot(startRoot)
	if strings.TrimSpace(root) == "" {
		return ""
	}
	branch := runGit(root, "rev-parse", "--abbrev-ref", "HEAD")
	head := runGit(root, "rev-parse", "--short", "HEAD")
	subject := runGit(root, "log", "-1", "--pretty=%s")
	status := runGit(root, "status", "--porcelain")

	branch = strings.TrimSpace(branch)
	head = strings.TrimSpace(head)
	subject = strings.TrimSpace(subject)
	status = strings.TrimSpace(status)

	if branch == "" && head == "" && subject == "" && status == "" {
		return ""
	}

	lines := []string{}
	if status != "" {
		for _, l := range strings.Split(status, "\n") {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			lines = append(lines, l)
			if len(lines) >= 12 {
				break
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("GitStatus:\n")
	if branch != "" {
		sb.WriteString("- branch: " + branch + "\n")
	}
	if head != "" {
		sb.WriteString("- head: " + head + "\n")
	}
	if subject != "" {
		sb.WriteString("- last_commit: " + subject + "\n")
	}
	if len(lines) > 0 {
		sb.WriteString("- changes:\n")
		for _, l := range lines {
			sb.WriteString("  - " + l + "\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func findGitRoot(startRoot string) string {
	cur := strings.TrimSpace(startRoot)
	if cur == "" {
		return ""
	}
	for {
		if fi, err := os.Stat(filepath.Join(cur, ".git")); err == nil && fi.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

func runGit(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()
	return out.String()
}
