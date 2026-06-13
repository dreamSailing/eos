//go:build legacy

package bridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/pkg/sandbox"
)

func (rc *RuntimeCore) buildGitContextHint(startRoot string) string {
	root := findGitRoot(startRoot)
	if strings.TrimSpace(root) == "" {
		return ""
	}
	policy := rc.sandboxPolicy(context.Background())
	branch := rc.runGitGuarded(policy, root, "rev-parse", "--abbrev-ref", "HEAD")
	head := rc.runGitGuarded(policy, root, "rev-parse", "--short", "HEAD")
	subject := rc.runGitGuarded(policy, root, "log", "-1", "--pretty=%s")
	status := rc.runGitGuarded(policy, root, "status", "--porcelain")

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

func (rc *RuntimeCore) runGitGuarded(policy sandbox.Policy, dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	result := rc.guardedGitCmd(ctx, policy, dir, args...)
	if result.Err != nil {
		slog.Warn("bridge.git_context.sandbox_blocked",
			"component", utils.ComponentSystem,
			"error", result.Err,
			"args", args,
		)
		return ""
	}
	return result.Stdout
}
