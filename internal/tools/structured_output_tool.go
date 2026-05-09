package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// structuredOutputStructured validates data against a JSON Schema and returns the result
func (m *Manager) structuredOutputStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	schemaStr, _ := params["schema"].(string)
	dataStr, _ := params["data"].(string)

	if schemaStr == "" {
		return ToolResult{Type: "tool_result", Tool: ToolStructuredOutput, Status: "error", Error: "schema parameter is required"}
	}
	if dataStr == "" {
		return ToolResult{Type: "tool_result", Tool: ToolStructuredOutput, Status: "error", Error: "data parameter is required"}
	}

	// Parse the schema
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolStructuredOutput, Status: "error", Error: fmt.Sprintf("invalid schema JSON: %v", err)}
	}

	// Parse the data
	var data interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolStructuredOutput, Status: "error", Error: fmt.Sprintf("invalid data JSON: %v", err)}
	}

	// Basic schema validation
	if errs := validateAgainstSchema(data, schema); len(errs) > 0 {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolStructuredOutput,
			Status:  "error",
			Error:   fmt.Sprintf("schema validation failed: %s", strings.Join(errs, "; ")),
			Data:    map[string]interface{}{"data": data, "errors": errs},
		}
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolStructuredOutput,
		Status:  "success",
		Data:    map[string]interface{}{"data": data, "schema": schema},
		Display: "Structured output validated successfully",
	}
}

// validateAgainstSchema performs basic JSON Schema validation
func validateAgainstSchema(data interface{}, schema map[string]interface{}) []string {
	var errs []string

	// Check type
	if typ, ok := schema["type"].(string); ok {
		if !matchType(data, typ) {
			errs = append(errs, fmt.Sprintf("expected type %s but got %T", typ, data))
		}
	}

	// Check required fields (for objects)
	if required, ok := schema["required"].([]interface{}); ok {
		if obj, ok := data.(map[string]interface{}); ok {
			for _, r := range required {
				if key, ok := r.(string); ok {
					if _, exists := obj[key]; !exists {
						errs = append(errs, fmt.Sprintf("missing required field: %s", key))
					}
				}
			}
		}
	}

	// Check properties (for objects)
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		if obj, ok := data.(map[string]interface{}); ok {
			for key, propSchema := range props {
				if val, exists := obj[key]; exists {
					if ps, ok := propSchema.(map[string]interface{}); ok {
						subErrs := validateAgainstSchema(val, ps)
						for _, e := range subErrs {
							errs = append(errs, fmt.Sprintf("field %q: %s", key, e))
						}
					}
				}
			}
		}
	}

	// Check enum
	if enum, ok := schema["enum"].([]interface{}); ok {
		found := false
		for _, v := range enum {
			if fmt.Sprintf("%v", v) == fmt.Sprintf("%v", data) {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf("value %v not in enum %v", data, enum))
		}
	}

	// Check items (for arrays)
	if items, ok := schema["items"].(map[string]interface{}); ok {
		if arr, ok := data.([]interface{}); ok {
			for i, item := range arr {
				subErrs := validateAgainstSchema(item, items)
				for _, e := range subErrs {
					errs = append(errs, fmt.Sprintf("item[%d]: %s", i, e))
				}
			}
		}
	}

	return errs
}

// matchType checks if a value matches the expected JSON Schema type
func matchType(data interface{}, typ string) bool {
	switch typ {
	case "string":
		_, ok := data.(string)
		return ok
	case "number":
		switch data.(type) {
		case float64, float32, int, int64, int32:
			return true
		default:
			return false
		}
	case "integer":
		switch data.(type) {
		case float64:
			f := data.(float64)
			return f == float64(int(f))
		case int, int64, int32:
			return true
		default:
			return false
		}
	case "boolean":
		_, ok := data.(bool)
		return ok
	case "array":
		_, ok := data.([]interface{})
		return ok
	case "object":
		_, ok := data.(map[string]interface{})
		return ok
	case "null":
		return data == nil
	default:
		return true
	}
}
