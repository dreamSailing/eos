package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	doccap "github.com/dreamSailing/eos/internal/document"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func (m *Manager) documentGenerateStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	format := doccap.NormalizeFormat(asString(params["format"]))
	if format == "" {
		return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: "format is required", Display: "错误：format 参数为必填项，且必须是 docx/xlsx/pdf"}
	}
	path := asString(params["path"])
	if strings.TrimSpace(path) == "" {
		return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: "path is required", Display: "错误：path 参数为必填项"}
	}
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), path)
	if !res.IsValid {
		return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: res.ErrMsg, Display: "错误：输出路径超出工作区范围"}
	}
	if err := sandboxWriteError(ctx, res.AbsPath); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
	}

	title := asString(params["title"])
	content := asString(params["content"])
	structured := params["structured_content"]

	var warnings []string
	switch format {
	case "docx", "pdf":
		model, err := parseDocumentPayload(title, content, structured)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: err.Error(), Display: "错误：文档内容解析失败：" + err.Error()}
		}
		warnings = append(warnings, model.Warnings...)
		if format == "docx" {
			if err := doccap.WriteDOCX(res.AbsPath, model); err != nil {
				return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: err.Error(), Display: "错误：生成 DOCX 失败：" + err.Error()}
			}
		} else {
			if err := doccap.WritePDF(res.AbsPath, model); err != nil {
				return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: err.Error(), Display: "错误：生成 PDF 失败：" + err.Error()}
			}
		}
	case "xlsx":
		model, err := parseWorkbookPayload(title, content, structured)
		if err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: err.Error(), Display: "错误：工作簿内容解析失败：" + err.Error()}
		}
		warnings = append(warnings, model.Warnings...)
		if err := doccap.WriteXLSX(res.AbsPath, model); err != nil {
			return ToolResult{Type: "tool_result", Tool: ToolDocumentGenerate, Status: "error", Error: err.Error(), Display: "错误：生成 XLSX 失败：" + err.Error()}
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolDocumentGenerate,
		Status: "success",
		Data: map[string]interface{}{
			"path":     res.RelPath,
			"format":   format,
			"warnings": warnings,
		},
		Display: fmt.Sprintf("已生成 %s：%s", strings.ToUpper(format), res.RelPath),
	}
}

func (m *Manager) documentConvertStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	source := asString(params["source_path"])
	target := doccap.NormalizeFormat(asString(params["target_format"]))
	destination := asString(params["destination_path"])
	fidelity := strings.ToLower(strings.TrimSpace(asString(params["fidelity"])))
	if fidelity == "" {
		fidelity = "high"
	}
	if strings.TrimSpace(source) == "" || target == "" {
		return ToolResult{Type: "tool_result", Tool: ToolDocumentConvert, Status: "error", Error: "source_path and target_format are required", Display: "错误：source_path 和 target_format 为必填项"}
	}
	srcRes := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), source)
	if !srcRes.IsValid {
		return ToolResult{Type: "tool_result", Tool: ToolDocumentConvert, Status: "error", Error: srcRes.ErrMsg, Display: "错误：源文件路径超出工作区范围"}
	}
	if strings.TrimSpace(destination) != "" {
		dstRes := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), destination)
		if !dstRes.IsValid {
			return ToolResult{Type: "tool_result", Tool: ToolDocumentConvert, Status: "error", Error: dstRes.ErrMsg, Display: "错误：目标文件路径超出工作区范围"}
		}
		destination = dstRes.AbsPath
	}
	destinationForPolicy := destination
	if strings.TrimSpace(destinationForPolicy) == "" {
		destinationForPolicy = strings.TrimSuffix(srcRes.AbsPath, filepath.Ext(srcRes.AbsPath)) + "." + target
	}
	if err := sandboxWriteError(ctx, destinationForPolicy); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolDocumentConvert, Status: "error", Error: err.Error(), Display: "错误：" + err.Error()}
	}
	result, err := doccap.Convert(srcRes.AbsPath, doccap.ConversionOptions{DestinationPath: destination, TargetFormat: target, Fidelity: fidelity})
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolDocumentConvert, Status: "error", Error: err.Error(), Display: "错误：文档转换失败：" + err.Error()}
	}
	relDest := result.DestinationPath
	if ws := WorkspaceRootFromContext(ctx); strings.TrimSpace(ws) != "" {
		if rp, err := filepath.Rel(ws, result.DestinationPath); err == nil && !strings.HasPrefix(rp, "..") {
			relDest = rp
		}
	}
	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolDocumentConvert,
		Status: "success",
		Data: map[string]interface{}{
			"source_path":      srcRes.RelPath,
			"destination_path": relDest,
			"source_format":    result.SourceFormat,
			"target_format":    result.TargetFormat,
			"used_engine":      result.UsedEngine,
			"degraded":         result.Degraded,
			"warnings":         result.Warnings,
		},
		Display: fmt.Sprintf("已转换为 %s：%s", strings.ToUpper(result.TargetFormat), relDest),
	}
}

func parseDocumentPayload(title, content string, structured any) (doccap.DocumentModel, error) {
	if structured != nil {
		var model doccap.DocumentModel
		if err := decodeStructured(structured, &model); err == nil && (len(model.Blocks) > 0 || strings.TrimSpace(model.Title) != "") {
			if strings.TrimSpace(model.Title) == "" {
				model.Title = strings.TrimSpace(title)
			}
			return model, nil
		}
		var book doccap.WorkbookModel
		if err := decodeStructured(structured, &book); err == nil && len(book.Sheets) > 0 {
			return doccap.ToDocumentModelFromWorkbook(book), nil
		}
	}
	return doccap.DocumentFromText(title, content), nil
}

func parseWorkbookPayload(title, content string, structured any) (doccap.WorkbookModel, error) {
	if structured != nil {
		var model doccap.WorkbookModel
		if err := decodeStructured(structured, &model); err == nil && len(model.Sheets) > 0 {
			if strings.TrimSpace(model.Title) == "" {
				model.Title = strings.TrimSpace(title)
			}
			return model, nil
		}
		var doc doccap.DocumentModel
		if err := decodeStructured(structured, &doc); err == nil && (len(doc.Blocks) > 0 || strings.TrimSpace(doc.Title) != "") {
			return doccap.ToWorkbookModelFromDocument(doc), nil
		}
	}
	return doccap.WorkbookFromText(title, content), nil
}

func decodeStructured(in any, out any) error {
	switch v := in.(type) {
	case string:
		return json.Unmarshal([]byte(v), out)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, out)
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
