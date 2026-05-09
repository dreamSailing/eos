package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// mcpListResourcesStructured lists available MCP resources from all or a specific server
func (m *Manager) mcpListResourcesStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	if m.mcpManager == nil {
		return ToolResult{Type: "tool_result", Tool: ToolMCPListResources, Status: "error", Error: "MCP manager not initialized"}
	}

	serverName, _ := params["server"].(string)

	if serverName != "" {
		resources := m.mcpManager.GetResourcesByServer(serverName)
		if resources == nil {
			return ToolResult{Type: "tool_result", Tool: ToolMCPListResources, Status: "error", Error: fmt.Sprintf("MCP server not found or no resources: %s", serverName)}
		}
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolMCPListResources,
			Status:  "success",
			Data:    map[string]interface{}{"server": serverName, "resources": resourceListToMaps(resources)},
			Display: formatResourceList(serverName, resources),
		}
	}

	allResources := m.mcpManager.GetAllResources()
	var lines []string
	totalCount := 0
	allData := make(map[string]interface{})
	for name, resources := range allResources {
		lines = append(lines, formatResourceList(name, resources))
		allData[name] = resourceListToMaps(resources)
		totalCount += len(resources)
	}

	if totalCount == 0 {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolMCPListResources,
			Status:  "success",
			Data:    map[string]interface{}{"total": 0},
			Display: "No MCP resources available",
		}
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolMCPListResources,
		Status:  "success",
		Data:    map[string]interface{}{"total": totalCount, "servers": allData},
		Display: strings.Join(lines, "\n"),
	}
}

// mcpReadResourceStructured reads a specific MCP resource by URI
func (m *Manager) mcpReadResourceStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	if m.mcpManager == nil {
		return ToolResult{Type: "tool_result", Tool: ToolMCPReadResource, Status: "error", Error: "MCP manager not initialized"}
	}

	serverName, _ := params["server"].(string)
	uri, _ := params["uri"].(string)

	if serverName == "" {
		return ToolResult{Type: "tool_result", Tool: ToolMCPReadResource, Status: "error", Error: "server parameter is required"}
	}
	if uri == "" {
		return ToolResult{Type: "tool_result", Tool: ToolMCPReadResource, Status: "error", Error: "uri parameter is required"}
	}

	result, err := m.mcpManager.ReadResource(ctx, serverName, uri)
	if err != nil {
		return ToolResult{Type: "tool_result", Tool: ToolMCPReadResource, Status: "error", Error: fmt.Sprintf("Failed to read resource: %v", err)}
	}

	contents := make([]interface{}, 0, len(result.Contents))
	var displayParts []string
	for _, c := range result.Contents {
		switch v := c.(type) {
		case mcp.TextResourceContents:
			entry := map[string]interface{}{
				"uri":      v.URI,
				"mimeType": v.MIMEType,
				"text":     v.Text,
			}
			contents = append(contents, entry)
			displayParts = append(displayParts, v.Text)
		case mcp.BlobResourceContents:
			entry := map[string]interface{}{
				"uri":      v.URI,
				"mimeType": v.MIMEType,
			}
			if v.Blob != "" {
				entry["blob_length"] = len(v.Blob)
				displayParts = append(displayParts, fmt.Sprintf("[binary data: %d bytes]", len(v.Blob)))
			}
			contents = append(contents, entry)
		default:
			contents = append(contents, map[string]interface{}{"unknown": true})
		}
	}

	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolMCPReadResource,
		Status:  "success",
		Data:    map[string]interface{}{"server": serverName, "uri": uri, "contents": contents},
		Display: strings.Join(displayParts, "\n"),
	}
}

func resourceListToMaps(resources []mcp.Resource) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(resources))
	for _, r := range resources {
		entry := map[string]interface{}{
			"uri":  r.URI,
			"name": r.Name,
		}
		if r.Description != "" {
			entry["description"] = r.Description
		}
		if r.MIMEType != "" {
			entry["mime_type"] = r.MIMEType
		}
		result = append(result, entry)
	}
	return result
}

func formatResourceList(serverName string, resources []mcp.Resource) string {
	if len(resources) == 0 {
		return fmt.Sprintf("[%s] no resources", serverName)
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("[%s] %d resource(s):", serverName, len(resources)))
	for _, r := range resources {
		line := fmt.Sprintf("  - %s (%s)", r.Name, r.URI)
		if r.Description != "" {
			line += ": " + r.Description
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
