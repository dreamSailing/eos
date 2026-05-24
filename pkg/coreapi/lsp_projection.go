package coreapi

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reDiagnosticItem = regexp.MustCompile(`^\[(Error|Warning|Info|Hint)\]\s+Line\s+(\d+):\s+(.+)$`)
	reSummary        = regexp.MustCompile(`^\*\*Summary\*\*:\s+(\d+)\s+files?\s+\((\d+)\s+errors?,\s+(\d+)\s+warnings?,\s+(\d+)\s+infos?\)`)
)

func ProjectDiagnosticsFromStrings(lines []string) LSPDiagnosticsSummary {
	var out LSPDiagnosticsSummary
	if len(lines) == 0 {
		return out
	}

	currentFile := ""
	fileSet := make(map[string]struct{})

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "### ") {
			currentFile = strings.TrimSpace(strings.TrimPrefix(line, "### "))
			continue
		}

		if strings.HasPrefix(line, "Path: ") {
			continue
		}

		if strings.HasPrefix(line, "... and ") {
			continue
		}

		if m := reSummary.FindStringSubmatch(line); len(m) == 5 {
			out.Files, _ = strconv.Atoi(m[1])
			out.Errors, _ = strconv.Atoi(m[2])
			out.Warnings, _ = strconv.Atoi(m[3])
			out.Infos, _ = strconv.Atoi(m[4])
			continue
		}

		if m := reDiagnosticItem.FindStringSubmatch(line); len(m) == 4 {
			lineNum, _ := strconv.Atoi(m[2])
			item := LSPDiagnosticItem{
				File:     currentFile,
				Line:     lineNum,
				Severity: m[1],
				Message:  m[3],
			}
			out.Items = append(out.Items, item)
			if currentFile != "" {
				fileSet[currentFile] = struct{}{}
			}
		}
	}

	if out.Files == 0 {
		out.Files = len(fileSet)
	}
	return out
}
