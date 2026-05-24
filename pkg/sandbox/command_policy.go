package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (p Policy) CommandViolation(argv []string) string {
	p = p.Normalized()
	if p.Mode == ModeDangerFullAccess {
		return ""
	}
	if !p.AllowsCommand(argv) {
		return fmt.Sprintf("sandbox policy %s blocks command: %s", p.Mode, strings.Join(argv, " "))
	}
	if p.Mode != ModeWorkspaceWrite {
		return ""
	}
	command := shellCommandString(argv)
	if command == "" {
		return ""
	}
	if blocksGlobalSystemChange(command) {
		return "sandbox policy workspace-write blocks global system changes"
	}
	for _, target := range commandWriteTargets(command) {
		resolved := resolveCommandPath(p.WorkspaceRoot, target)
		ok, err := p.AllowsWrite(resolved)
		if err != nil {
			return err.Error()
		}
		if !ok {
			return fmt.Sprintf("sandbox policy workspace-write blocks writes outside workspace: %s", filepath.ToSlash(resolved))
		}
	}
	return ""
}

func shellCommandString(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	name := strings.ToLower(filepath.Base(strings.TrimSpace(argv[0])))
	if (name == "bash" || name == "bash.exe" || name == "sh" || name == "sh.exe") && len(argv) >= 3 {
		for i := 1; i < len(argv)-1; i++ {
			arg := strings.ToLower(strings.TrimSpace(argv[i]))
			if arg == "-c" || arg == "-lc" {
				return strings.TrimSpace(argv[i+1])
			}
		}
	}
	if (name == "powershell" || name == "powershell.exe" || name == "pwsh" || name == "pwsh.exe") && len(argv) >= 3 {
		for i := 1; i < len(argv)-1; i++ {
			arg := strings.ToLower(strings.TrimSpace(argv[i]))
			if arg == "-command" || arg == "-c" {
				return strings.TrimSpace(argv[i+1])
			}
		}
	}
	return strings.TrimSpace(strings.Join(argv, " "))
}

func blocksGlobalSystemChange(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	if lower == "" {
		return false
	}
	blocked := []string{
		"npm install -g",
		"pnpm add -g",
		"yarn global add",
		"pip install --user",
		"sudo ",
		"apt install",
		"apt-get install",
		"brew install",
		"yum install",
		"dnf install",
		"pacman -s",
	}
	for _, pattern := range blocked {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func commandWriteTargets(command string) []string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, 0, 2)
	for i := 0; i < len(fields); i++ {
		token := cleanCommandToken(fields[i])
		if token == "" {
			continue
		}
		if token == ">" || token == ">>" || token == "1>" || token == "1>>" || token == "2>" || token == "2>>" {
			if i+1 < len(fields) {
				if target := cleanCommandToken(fields[i+1]); target != "" {
					out = append(out, target)
				}
			}
			continue
		}
		for _, prefix := range []string{">>", ">", "1>>", "1>", "2>>", "2>"} {
			if strings.HasPrefix(token, prefix) && len(token) > len(prefix) {
				out = append(out, cleanCommandToken(strings.TrimPrefix(token, prefix)))
				break
			}
		}
	}
	segments := splitShellSegments(fields)
	for _, segment := range segments {
		out = append(out, commandSegmentWriteTargets(segment)...)
	}
	return compactStrings(out)
}

func commandSegmentWriteTargets(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}
	cmd := strings.ToLower(cleanCommandToken(fields[0]))
	switch cmd {
	case "touch", "mkdir", "rm", "del", "rmdir", "new-item", "ni", "remove-item", "ri", "erase", "rd":
		return commandOperands(fields[1:])
	case "cp", "copy", "mv", "move", "copy-item", "move-item":
		operands := commandOperands(fields[1:])
		if len(operands) == 0 {
			return nil
		}
		return []string{operands[len(operands)-1]}
	case "set-content", "add-content", "out-file":
		return commandOperands(fields[1:])
	case "tee":
		return commandOperands(fields[1:])
	case "sed":
		if commandHasOption(fields[1:], "-i") || commandHasOption(fields[1:], "--in-place") {
			return commandOperands(fields[1:])
		}
	case "gofmt":
		if commandHasOption(fields[1:], "-w") {
			return commandOperands(fields[1:])
		}
	case "perl":
		if commandHasOptionPrefix(fields[1:], "-pi") {
			return commandOperands(fields[1:])
		}
	}
	return nil
}

func splitShellSegments(fields []string) [][]string {
	var segments [][]string
	var current []string
	for _, field := range fields {
		token := cleanCommandToken(field)
		switch token {
		case "&&", "||", ";", "|":
			if len(current) > 0 {
				segments = append(segments, current)
				current = nil
			}
		default:
			current = append(current, field)
		}
	}
	if len(current) > 0 {
		segments = append(segments, current)
	}
	return segments
}

func commandOperands(fields []string) []string {
	out := make([]string, 0, len(fields))
	skipNext := false
	for _, field := range fields {
		if skipNext {
			skipNext = false
			continue
		}
		token := cleanCommandToken(field)
		if token == "" {
			continue
		}
		if token == ">" || token == ">>" || token == "1>" || token == "1>>" || token == "2>" || token == "2>>" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		if strings.Contains(token, "=") && !strings.ContainsAny(token, `/\`) {
			continue
		}
		out = append(out, token)
	}
	return out
}

func commandHasOption(fields []string, want string) bool {
	for _, field := range fields {
		if cleanCommandToken(field) == want {
			return true
		}
	}
	return false
}

func commandHasOptionPrefix(fields []string, want string) bool {
	for _, field := range fields {
		if strings.HasPrefix(cleanCommandToken(field), want) {
			return true
		}
	}
	return false
}

func cleanCommandToken(token string) string {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, "\"'`,()[]{}")
	token = strings.TrimRight(token, ",")
	return token
}

func resolveCommandPath(workspaceRoot, target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	if workspaceRoot == "" {
		return filepath.Clean(target)
	}
	return filepath.Clean(filepath.Join(workspaceRoot, filepath.FromSlash(target)))
}
