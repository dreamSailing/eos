package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// NotebookCell represents a cell in a Jupyter notebook
type NotebookCell struct {
	CellType string `json:"cell_type"`
	Source   any    `json:"source"`
	Outputs  any    `json:"outputs"`
}

// Notebook represents a Jupyter notebook structure
type Notebook struct {
	Cells []NotebookCell `json:"cells"`
}

// ReadNotebook reads and formats a Jupyter notebook (.ipynb) file
func ReadNotebook(path string, maxLines int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read notebook: %w", err)
	}

	var nb Notebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return "", fmt.Errorf("failed to parse notebook JSON: %w", err)
	}

	if maxLines <= 0 {
		maxLines = 2000
	}

	var sb strings.Builder
	lineCount := 0

	for i, cell := range nb.Cells {
		if lineCount >= maxLines {
			sb.WriteString(fmt.Sprintf("\n... (truncated at %d lines, %d more cells)\n", maxLines, len(nb.Cells)-i))
			break
		}

		// Cell header
		switch cell.CellType {
		case "code":
			sb.WriteString(fmt.Sprintf("\n## Cell %d [code]\n", i+1))
		case "markdown":
			sb.WriteString(fmt.Sprintf("\n## Cell %d [markdown]\n", i+1))
		default:
			sb.WriteString(fmt.Sprintf("\n## Cell %d [%s]\n", i+1, cell.CellType))
		}

		// Cell source
		source := extractCellSource(cell.Source)
		sb.WriteString(source)
		sb.WriteString("\n")
		lineCount += strings.Count(source, "\n") + 1

		// Cell outputs (for code cells)
		if cell.CellType == "code" && cell.Outputs != nil {
			outputStr := extractCellOutputs(cell.Outputs)
			if outputStr != "" {
				sb.WriteString("### Output:\n")
				sb.WriteString(outputStr)
				sb.WriteString("\n")
				lineCount += strings.Count(outputStr, "\n") + 2
			}
		}

		if lineCount >= maxLines {
			sb.WriteString(fmt.Sprintf("\n... (truncated at %d lines)\n", maxLines))
			break
		}
	}

	return sb.String(), nil
}

// extractCellSource extracts the source content from a cell
func extractCellSource(source any) string {
	switch v := source.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, "")
	default:
		return fmt.Sprintf("%v", source)
	}
}

// extractCellOutputs extracts and formats cell outputs
func extractCellOutputs(outputs any) string {
	outputList, ok := outputs.([]interface{})
	if !ok {
		return ""
	}

	var parts []string
	for _, out := range outputList {
		outputMap, ok := out.(map[string]interface{})
		if !ok {
			continue
		}

		outputType, _ := outputMap["output_type"].(string)

		switch outputType {
		case "stream":
			if text, ok := outputMap["text"]; ok {
				parts = append(parts, extractCellSource(text))
			}
		case "execute_result", "display_data":
			if data, ok := outputMap["data"].(map[string]interface{}); ok {
				if text, ok := data["text/plain"]; ok {
					parts = append(parts, extractCellSource(text))
				}
			}
		case "error":
			if traceback, ok := outputMap["traceback"]; ok {
				tb, _ := traceback.([]interface{})
				for _, line := range tb {
					parts = append(parts, fmt.Sprintf("%v", line))
				}
			}
		}
	}

	result := strings.Join(parts, "\n")
	if len(result) > 2000 {
		result = result[:2000] + "\n... (output truncated)"
	}
	return result
}
