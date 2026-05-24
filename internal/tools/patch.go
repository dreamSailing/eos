package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools/fileops"
	"github.com/pmezard/go-difflib/difflib"
)

func (m *Manager) patchStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	mode, _ := params["mode"].(string)
	if mode == "" {
		mode = "apply"
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "apply" && mode != "dry_run" {
		return ToolResult{Type: "tool_result", Tool: ToolPatch, Status: "error",
			Error:   fmt.Sprintf("invalid mode: %s (valid: apply, dry_run)", mode),
			Display: fmt.Sprintf("错误：无效模式 '%s'", mode)}
	}

	format, _ := params["format"].(string)
	if format == "" {
		format = "edits"
	}
	format = strings.ToLower(strings.TrimSpace(format))

	switch format {
	case "edits":
		return m.patchApplyEdits(ctx, params, mode == "dry_run")
	case "unified":
		return m.patchApplyUnified(ctx, params, mode == "dry_run")
	default:
		return ToolResult{Type: "tool_result", Tool: ToolPatch, Status: "error",
			Error:   fmt.Sprintf("invalid format: %s (valid: edits, unified)", format),
			Display: fmt.Sprintf("错误：无效格式 '%s'", format)}
	}
}

func (m *Manager) patchApplyEdits(ctx context.Context, params map[string]interface{}, dryRun bool) ToolResult {
	patchesRaw, _ := params["patches"].([]interface{})
	if len(patchesRaw) == 0 {
		return ToolResult{Type: "tool_result", Tool: ToolPatch, Status: "error",
			Error: "patches parameter is required for edits format", Display: "错误：patches 为必填项"}
	}

	totalChanges := 0
	results := make([]map[string]interface{}, 0, len(patchesRaw))
	paths := make([]string, 0, len(patchesRaw))

	for _, pRaw := range patchesRaw {
		pm, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		file, _ := pm["path"].(string)
		if strings.TrimSpace(file) == "" {
			continue
		}
		editsRaw, _ := pm["edits"].([]interface{})
		if len(editsRaw) == 0 {
			continue
		}

		file = normalizePathPlaceholder(file)
		res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), file)
		if !res.IsValid {
			results = append(results, map[string]interface{}{"path": filepath.ToSlash(file), "error": "path outside working directory"})
			continue
		}
		ap := res.AbsPath
		rel := res.RelPath

		old, err := m.fileOps.ReadFile(ap)
		if err != nil {
			results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "error": fmt.Sprintf("%v", err)})
			continue
		}

		text := old
		applied := 0
		for _, eRaw := range editsRaw {
			em, _ := eRaw.(map[string]interface{})
			if em == nil {
				continue
			}
			find, _ := em["find"].(string)
			replace, _ := em["replace"].(string)
			if strings.TrimSpace(find) != "" {
				limit := toInt(em["limit"], 0)
				ci := false
				if v, ok := em["caseInsensitive"].(bool); ok {
					ci = v
				}
				isRegex := false
				if v, ok := em["regex"].(bool); ok {
					isRegex = v
				}
				var c int
				text, c = applyReplace(text, find, replace, limit, ci, isRegex)
				applied += c
				continue
			}
			if slf, ok := em["start_line"].(float64); ok {
				elf, _ := em["end_line"].(float64)
				sl := int(slf)
				el := int(elf)
				if sl > 0 && el >= sl {
					lines := strings.Split(text, "\n")
					s := sl - 1
					e := el
					if s < 0 {
						s = 0
					}
					if e > len(lines) {
						e = len(lines)
					}
					before := strings.Join(lines[:s], "\n")
					after := strings.Join(lines[e:], "\n")
					text = strings.TrimSuffix(before+"\n"+replace+"\n"+after, "\n")
					applied++
				}
			}
		}

		diff := ""
		if applied > 0 {
			diffRes := m.generateDiffStructured(context.Background(), map[string]interface{}{"path": filepath.ToSlash(rel), "proposed_content": text})
			if diffRes.Status == "success" {
				if textVal, ok := diffRes.Data["text"].(string); ok {
					diff = textVal
				}
			}
		}

		wrote := false
		if !dryRun && applied > 0 {
			if err := sandboxWriteError(ctx, ap); err != nil {
				results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "error": err.Error()})
				continue
			}
			if m.fileOps.IsTextFile(ap) {
				_, _ = m.fileOps.SaveVersionWithExtra(ap, old, fileops.VersionExtra{
					TraceID:   TraceIDFromContext(ctx),
					Tool:      ToolPatch,
					Operation: "edits",
				})
			}
			if err := m.fileOps.WriteFile(ap, text); err != nil {
				results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "error": fmt.Sprintf("%v", err)})
				continue
			}
			wrote = true
		}

		totalChanges += applied
		if applied > 0 || wrote || dryRun {
			paths = append(paths, filepath.ToSlash(rel))
		}
		results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "applied": applied, "diff": diff, "wrote": wrote})
	}

	display := fmt.Sprintf("patch(edits)：%d 处变更", totalChanges)
	if dryRun {
		display = fmt.Sprintf("patch(edits) dry-run：%d 处变更预览", totalChanges)
	}
	if len(paths) > 0 {
		max := 3
		if len(paths) < max {
			max = len(paths)
		}
		previewPaths := strings.Join(paths[:max], ", ")
		if len(paths) > max {
			previewPaths += fmt.Sprintf(", ...(+%d)", len(paths)-max)
		}
		display = previewPaths + " | " + display
	}
	return ToolResult{Type: "tool_result", Tool: ToolPatch, Status: "success",
		Data: map[string]interface{}{"changes": totalChanges, "results": results, "paths": paths, "dry_run": dryRun},
		Display: display}
}

func (m *Manager) patchApplyUnified(ctx context.Context, params map[string]interface{}, dryRun bool) ToolResult {
	diffText, _ := params["diff"].(string)
	if strings.TrimSpace(diffText) == "" {
		return ToolResult{Type: "tool_result", Tool: ToolPatch, Status: "error",
			Error: "diff parameter is required for unified format", Display: "错误：diff 为必填项"}
	}

	filePatches := parseUnifiedDiff(diffText)
	if len(filePatches) == 0 {
		return ToolResult{Type: "tool_result", Tool: ToolPatch, Status: "error",
			Error: "no valid patches found in diff", Display: "错误：未找到有效的 patch 块"}
	}

	totalChanges := 0
	results := make([]map[string]interface{}, 0, len(filePatches))
	paths := make([]string, 0, len(filePatches))

	for _, fp := range filePatches {
		targetPath := fp.toFile
		if targetPath == "" || targetPath == "/dev/null" {
			continue
		}
		targetPath = strings.TrimPrefix(targetPath, "b/")

		targetPath = normalizePathPlaceholder(targetPath)
		res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), targetPath)
		if !res.IsValid {
			results = append(results, map[string]interface{}{"path": filepath.ToSlash(targetPath), "error": "path outside working directory"})
			continue
		}
		ap := res.AbsPath
		rel := res.RelPath

		old, err := m.fileOps.ReadFile(ap)
		if err != nil {
			results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "error": fmt.Sprintf("%v", err)})
			continue
		}

		newText, hunksApplied, applyErr := applyUnifiedHunks(old, fp.hunks)
		if applyErr != nil {
			results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "error": applyErr.Error()})
			continue
		}

		diff := ""
		if hunksApplied > 0 {
			diffRes := m.generateDiffStructured(context.Background(), map[string]interface{}{"path": filepath.ToSlash(rel), "proposed_content": newText})
			if diffRes.Status == "success" {
				if textVal, ok := diffRes.Data["text"].(string); ok {
					diff = textVal
				}
			}
		}

		wrote := false
		if !dryRun && hunksApplied > 0 {
			if err := sandboxWriteError(ctx, ap); err != nil {
				results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "error": err.Error()})
				continue
			}
			if m.fileOps.IsTextFile(ap) {
				_, _ = m.fileOps.SaveVersionWithExtra(ap, old, fileops.VersionExtra{
					TraceID:   TraceIDFromContext(ctx),
					Tool:      ToolPatch,
					Operation: "unified",
				})
			}
			if err := m.fileOps.WriteFile(ap, newText); err != nil {
				results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "error": fmt.Sprintf("%v", err)})
				continue
			}
			wrote = true
		}

		totalChanges += hunksApplied
		if hunksApplied > 0 || wrote || dryRun {
			paths = append(paths, filepath.ToSlash(rel))
		}
		results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "applied": hunksApplied, "diff": diff, "wrote": wrote})
	}

	display := fmt.Sprintf("patch(unified)：%d 个 hunk 已应用", totalChanges)
	if dryRun {
		display = fmt.Sprintf("patch(unified) dry-run：%d 个 hunk 预览", totalChanges)
	}
	if len(paths) > 0 {
		max := 3
		if len(paths) < max {
			max = len(paths)
		}
		previewPaths := strings.Join(paths[:max], ", ")
		if len(paths) > max {
			previewPaths += fmt.Sprintf(", ...(+%d)", len(paths)-max)
		}
		display = previewPaths + " | " + display
	}
	return ToolResult{Type: "tool_result", Tool: ToolPatch, Status: "success",
		Data: map[string]interface{}{"changes": totalChanges, "results": results, "paths": paths, "dry_run": dryRun},
		Display: display}
}

type parsedFilePatch struct {
	toFile string
	hunks  []unifiedHunk
}

type unifiedHunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []hunkLine
}

type hunkLine struct {
	kind byte
	text string
}

func parseUnifiedDiff(diff string) []parsedFilePatch {
	var patches []parsedFilePatch
	var current *parsedFilePatch
	var currentHunk *unifiedHunk

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "--- ") {
			cp := parsedFilePatch{}
			patches = append(patches, cp)
			current = &patches[len(patches)-1]
			currentHunk = nil
			continue
		}
		if strings.HasPrefix(line, "+++ ") && current != nil {
			current.toFile = strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			continue
		}
		if strings.HasPrefix(line, "@@") && current != nil {
			h := parseHunkHeader(line)
			current.hunks = append(current.hunks, h)
			currentHunk = &current.hunks[len(current.hunks)-1]
			continue
		}
		if currentHunk != nil && len(line) > 0 {
			kind := line[0]
			text := line[1:]
			switch kind {
			case '+':
				currentHunk.lines = append(currentHunk.lines, hunkLine{kind: '+', text: text})
			case '-':
				currentHunk.lines = append(currentHunk.lines, hunkLine{kind: '-', text: text})
			case ' ':
				currentHunk.lines = append(currentHunk.lines, hunkLine{kind: ' ', text: text})
			}
		}
	}
	return patches
}

func parseHunkHeader(line string) unifiedHunk {
	h := unifiedHunk{}
	parts := strings.SplitN(line, "@@", 3)
	if len(parts) < 3 {
		return h
	}
	ranges := strings.TrimSpace(parts[1])
	segments := strings.SplitN(ranges, " ", 2)

	oldPart := segments[0]
	oldPart = strings.TrimPrefix(oldPart, "-")
	oldRange := strings.SplitN(oldPart, ",", 2)
	h.oldStart, _ = strconv.Atoi(oldRange[0])
	if len(oldRange) > 1 {
		h.oldCount, _ = strconv.Atoi(oldRange[1])
	} else {
		h.oldCount = 1
	}

	if len(segments) > 1 {
		newPart := strings.TrimPrefix(segments[1], "+")
		newRange := strings.SplitN(newPart, ",", 2)
		h.newStart, _ = strconv.Atoi(newRange[0])
		if len(newRange) > 1 {
			h.newCount, _ = strconv.Atoi(newRange[1])
		} else {
			h.newCount = 1
		}
	}
	return h
}

func applyUnifiedHunks(original string, hunks []unifiedHunk) (string, int, error) {
	originalLines := difflib.SplitLines(original)
	applied := 0
	offset := 0

	for _, h := range hunks {
		startIdx := h.oldStart - 1 + offset
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(originalLines) {
			return original, applied, fmt.Errorf("hunk at line %d exceeds file length %d", h.oldStart, len(originalLines))
		}

		endIdx := startIdx + h.oldCount
		if endIdx > len(originalLines) {
			endIdx = len(originalLines)
		}

		match := true
		if h.oldCount > 0 {
			oldBlock := originalLines[startIdx:endIdx]
			var expectedOld []string
			for _, l := range h.lines {
				if l.kind == '-' || l.kind == ' ' {
					expectedOld = append(expectedOld, l.text)
				}
			}
			if len(oldBlock) != len(expectedOld) {
				match = false
			} else {
				for i := range oldBlock {
					a := strings.TrimSuffix(oldBlock[i], "\n")
					b := strings.TrimSuffix(expectedOld[i], "\n")
					if a != b {
						match = false
						break
					}
				}
			}
		}

		if !match {
			return original, applied, fmt.Errorf("hunk context mismatch at line %d", h.oldStart)
		}

		var newBlock []string
		for _, l := range h.lines {
			if l.kind == '+' || l.kind == ' ' {
				newBlock = append(newBlock, l.text)
			}
		}

		var result []string
		result = append(result, originalLines[:startIdx]...)
		result = append(result, newBlock...)
		result = append(result, originalLines[endIdx:]...)

		delta := len(newBlock) - (endIdx - startIdx)
		offset += delta
		originalLines = result
		applied++
	}

	return strings.Join(originalLines, ""), applied, nil
}


