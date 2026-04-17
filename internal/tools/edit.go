package tools

import (
	"context"
	"fmt"
	"github.com/dreamSailing/eos/internal/pkg/utils"
	"github.com/dreamSailing/eos/internal/tools/fileops"
	"path/filepath"
	"regexp"
	"strings"
)

// editStructured 统一的编辑工具入口
func (m *Manager) editStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	mode, _ := params["mode"].(string)
	if mode == "" {
		// 自动推断模式，减少首次调用缺参导致的失败
		if editsRaw, ok := params["edits"].([]interface{}); ok && len(editsRaw) > 0 {
			if file, okFile := params["file"].(string); okFile && strings.TrimSpace(file) != "" {
				mode = "multi"
			} else {
				mode = "batch"
			}
		} else if file, okFile := params["file"].(string); okFile && strings.TrimSpace(file) != "" {
			if find, okFind := params["find"].(string); okFind && strings.TrimSpace(find) != "" {
				mode = "single"
			}
		}
	}
	if mode == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    "edit",
			Status:  "error",
			Error:   "mode parameter is required (valid: single, multi, batch)",
			Display: "错误：mode 参数为必填项（可选值：single, multi, batch）",
		}
	}

	switch mode {
	case "single":
		return m.editSingle(ctx, params)
	case "multi":
		return m.editMulti(ctx, params)
	case "batch":
		return m.editBatch(ctx, params)
	default:
		return ToolResult{
			Type:    "tool_result",
			Tool:    "edit",
			Status:  "error",
			Error:   fmt.Sprintf("unknown mode: %s (valid: single, multi, batch)", mode),
			Display: fmt.Sprintf("错误：未知模式 '%s'", mode),
		}
	}
}

// editSingle 单次查找替换
func (m *Manager) editSingle(ctx context.Context, params map[string]interface{}) ToolResult {
	file, _ := params["file"].(string)
	find, _ := params["find"].(string)
	replace, _ := params["replace"].(string)
	limit := toInt(params["limit"], 0)
	preview := false
	if v, ok := params["previewOnly"].(bool); ok {
		preview = v
	}
	ci := false
	if v, ok := params["caseInsensitive"].(bool); ok {
		ci = v
	}
	isRegex := false
	if v, ok := params["regex"].(bool); ok {
		isRegex = v
	}

	if strings.TrimSpace(file) == "" {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: "file parameter is required for single mode", Display: "错误：file 参数为必填项"}
	}
	if strings.TrimSpace(find) == "" {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: "find parameter is required for single mode", Display: "错误：find 参数为必填项"}
	}
	file = normalizePathPlaceholder(file)
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), file)
	if !res.IsValid {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: "path outside working directory", Display: "错误：路径超出工作目录"}
	}
	ap := res.AbsPath
	rel := res.RelPath
	old, err := m.fileOps.ReadFile(ap)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: fmt.Sprintf("%v", err), Display: fmt.Sprintf("错误：%v", err)}
	}
	newText, count := applyReplace(old, find, replace, limit, ci, isRegex)
	if count == 0 {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "success", Data: map[string]interface{}{"path": filepath.ToSlash(rel), "changes": 0}, Display: "无变更"}
	}
	if preview {
		diffRes := m.generateDiffStructured(context.Background(), map[string]interface{}{"path": filepath.ToSlash(rel), "proposed_content": newText})
		diff := ""
		if diffRes.Status == "success" {
			if textVal, ok := diffRes.Data["text"].(string); ok {
				diff = textVal
			}
		}
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "success", Data: map[string]interface{}{"path": filepath.ToSlash(rel), "changes": count, "diff": diff}, Display: fmt.Sprintf("预览：%d 处匹配", count)}
	}
	if m.fileOps.IsTextFile(ap) {
		_, _ = m.fileOps.SaveVersionWithExtra(ap, old, fileops.VersionExtra{
			TraceID:   TraceIDFromContext(ctx),
			Tool:      ToolEdit,
			Operation: "single",
		})
	}
	if err := m.fileOps.WriteFile(ap, newText); err != nil {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: fmt.Sprintf("%v", err), Display: fmt.Sprintf("错误：%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "edit", Status: "success", Data: map[string]interface{}{"path": filepath.ToSlash(rel), "changes": count}, Display: fmt.Sprintf("已编辑 %d 处匹配", count)}
}

// editMulti 单文件多次编辑
func (m *Manager) editMulti(ctx context.Context, params map[string]interface{}) ToolResult {
	file, _ := params["file"].(string)
	editsRaw, _ := params["edits"].([]interface{})
	preview := false
	if v, ok := params["previewOnly"].(bool); ok {
		preview = v
	}
	if strings.TrimSpace(file) == "" {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: "file parameter is required for multi mode", Display: "错误：file 参数为必填项"}
	}
	if len(editsRaw) == 0 {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: "edits parameter is required for multi mode", Display: "错误：edits 参数为必填项"}
	}
	file = normalizePathPlaceholder(file)
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), file)
	if !res.IsValid {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: "path outside working directory", Display: "错误：路径超出工作目录"}
	}
	ap := res.AbsPath
	rel := res.RelPath
	old, err := m.fileOps.ReadFile(ap)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: fmt.Sprintf("%v", err), Display: fmt.Sprintf("错误：%v", err)}
	}
	text := old
	total := 0
	for _, e := range editsRaw {
		em, _ := e.(map[string]interface{})
		if em == nil {
			continue
		}
		// find/replace path
		find, _ := em["find"].(string)
		replace, _ := em["replace"].(string)
		limit := toInt(em["limit"], 0)
		ci := false
		if v, ok := em["caseInsensitive"].(bool); ok {
			ci = v
		}
		isRegex := false
		if v, ok := em["regex"].(bool); ok {
			isRegex = v
		}
		if strings.TrimSpace(find) != "" {
			var c int
			text, c = applyReplace(text, find, replace, limit, ci, isRegex)
			total += c
			continue
		}
		// range path
		if sl, ok := em["start_line"].(float64); ok {
			el, _ := em["end_line"].(float64)
			if int(sl) > 0 && int(el) >= int(sl) {
				lines := strings.Split(text, "\n")
				s := int(sl) - 1
				e := int(el)
				if s < 0 {
					s = 0
				}
				if e > len(lines) {
					e = len(lines)
				}
				before := strings.Join(lines[:s], "\n")
				after := strings.Join(lines[e:], "\n")
				text = strings.TrimSuffix(before+"\n"+replace+"\n"+after, "\n")
				total++
			}
		}
	}
	if total == 0 {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "success", Data: map[string]interface{}{"path": filepath.ToSlash(rel), "changes": 0}, Display: "无变更"}
	}
	if preview {
		diffRes := m.generateDiffStructured(context.Background(), map[string]interface{}{"path": filepath.ToSlash(rel), "proposed_content": text})
		diff := ""
		if diffRes.Status == "success" {
			if textVal, ok := diffRes.Data["text"].(string); ok {
				diff = textVal
			}
		}
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "success", Data: map[string]interface{}{"path": filepath.ToSlash(rel), "changes": total, "diff": diff}, Display: fmt.Sprintf("预览：%d 处编辑", total)}
	}
	if m.fileOps.IsTextFile(ap) {
		_, _ = m.fileOps.SaveVersionWithExtra(ap, old, fileops.VersionExtra{
			TraceID:   TraceIDFromContext(ctx),
			Tool:      ToolEdit,
			Operation: "multi",
		})
	}
	if err := m.fileOps.WriteFile(ap, text); err != nil {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: fmt.Sprintf("%v", err), Display: fmt.Sprintf("错误：%v", err)}
	}
	return ToolResult{Type: "tool_result", Tool: "edit", Status: "success", Data: map[string]interface{}{"path": filepath.ToSlash(rel), "changes": total}, Display: fmt.Sprintf("已应用 %d 处编辑", total)}
}

// editBatch 跨文件批量编辑
func (m *Manager) editBatch(ctx context.Context, params map[string]interface{}) ToolResult {
	raw, _ := params["edits"].([]interface{})
	preview := false
	if v, ok := params["previewOnly"].(bool); ok {
		preview = v
	}
	doFormat := false
	if v, ok := params["format"].(bool); ok {
		doFormat = v
	}
	if len(raw) == 0 {
		return ToolResult{Type: "tool_result", Tool: "edit", Status: "error", Error: "edits required", Display: "错误：edits 为必填项"}
	}
	total := 0
	results := make([]map[string]interface{}, 0, len(raw))
	paths := make([]string, 0, len(raw))
	for _, it := range raw {
		fm, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		file, _ := fm["file"].(string)
		opsRaw, _ := fm["operations"].([]interface{})
		if strings.TrimSpace(file) == "" || len(opsRaw) == 0 {
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
		for _, or := range opsRaw {
			op, _ := or.(map[string]interface{})
			if op == nil {
				continue
			}
			// find/replace
			if find, ok := op["find"].(string); ok && strings.TrimSpace(find) != "" {
				replace, _ := op["replace"].(string)
				limit := toInt(op["limit"], 0)
				ci := false
				if v, ok2 := op["caseInsensitive"].(bool); ok2 {
					ci = v
				}
				regex := false
				if v, ok2 := op["regex"].(bool); ok2 {
					regex = v
				}
				var c int
				text, c = applyReplace(text, find, replace, limit, ci, regex)
				applied += c
				continue
			}
			// range replace
			if slf, ok := op["start_line"].(float64); ok {
				elf, _ := op["end_line"].(float64)
				repl, _ := op["replace"].(string)
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
					text = strings.TrimSuffix(before+"\n"+repl+"\n"+after, "\n")
					applied++
				}
			}
		}
		// generate diff
		diffRes := m.generateDiffStructured(context.Background(), map[string]interface{}{"path": rel, "proposed_content": text})
		diff := ""
		if diffRes.Status == "success" {
			if textVal, ok := diffRes.Data["text"].(string); ok {
				diff = textVal
			}
		}
		wrote := false
		if !preview && applied > 0 {
			if m.fileOps.IsTextFile(ap) {
				_, _ = m.fileOps.SaveVersionWithExtra(ap, old, fileops.VersionExtra{
					TraceID:   TraceIDFromContext(ctx),
					Tool:      ToolEdit,
					Operation: "batch",
				})
			}
			if err := m.fileOps.WriteFile(ap, text); err != nil {
				results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "error": fmt.Sprintf("%v", err)})
				continue
			}
			wrote = true
			if doFormat {
				_ = m.shellExecuteFormat(ctx, ap)
			}
		}
		total += applied
		if applied > 0 || wrote || preview {
			paths = append(paths, filepath.ToSlash(rel))
		}
		results = append(results, map[string]interface{}{"path": filepath.ToSlash(rel), "applied": applied, "diff": diff, "wrote": wrote})
	}
	display := fmt.Sprintf("批量编辑：%d 处变更", total)
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
	return ToolResult{Type: "tool_result", Tool: "edit", Status: "success", Data: map[string]interface{}{"changes": total, "results": results, "paths": paths}, Display: display}
}

// applyReplace 执行字符串替换
func applyReplace(s, find, replace string, limit int, ci, regex bool) (string, int) {
	if regex {
		flags := ""
		if ci {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + find)
		if err != nil {
			return s, 0
		}
		count := 0
		out := re.ReplaceAllStringFunc(s, func(orig string) string {
			if limit > 0 && count >= limit {
				return orig
			}
			count++
			return replace
		})
		return out, count
	}
	src := s
	tgt := find
	if ci {
		// simple case-insensitive replace
		lowerSrc := strings.ToLower(src)
		lowerTgt := strings.ToLower(tgt)
		b := strings.Builder{}
		i := 0
		c := 0
		for {
			j := strings.Index(lowerSrc[i:], lowerTgt)
			if j < 0 {
				b.WriteString(src[i:])
				break
			}
			j = i + j
			b.WriteString(src[i:j])
			if limit > 0 && c >= limit {
				b.WriteString(src[j : j+len(tgt)])
			} else {
				b.WriteString(replace)
				c++
			}
			i = j + len(tgt)
		}
		return b.String(), c
	}
	if limit <= 0 {
		return strings.ReplaceAll(s, find, replace), strings.Count(s, find)
	}
	out := s
	c := 0
	for c < limit {
		idx := strings.Index(out, find)
		if idx < 0 {
			break
		}
		out = out[:idx] + replace + out[idx+len(find):]
		c++
	}
	return out, c
}

// shellExecuteFormat 格式化文件（Go 文件使用 gofmt）
func (m *Manager) shellExecuteFormat(ctx context.Context, path string) error {
	// best-effort: go files use gofmt -w
	if strings.HasSuffix(strings.ToLower(path), ".go") {
		_, err := m.shell.ExecuteWithWorkingDirCtx(ctx, fmt.Sprintf("gofmt -w \"%s\"", path), WorkspaceRootFromContext(ctx))
		return err
	}
	return nil
}
