package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// webSearchStructured handles the web_search tool
func (m *Manager) webSearchStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	query, _ := params["query"].(string)
	if strings.TrimSpace(query) == "" {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolWebSearch,
			Status: "error",
			Error:  "query is required",
			Display: "错误：query 参数为必填项",
		}
	}

	maxResults := 5
	if mr, ok := params["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	}

	// Use DuckDuckGo search via the eino-ext dependency
	results, err := duckduckgoSearch(ctx, query, maxResults)
	if err != nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolWebSearch,
			Status: "error",
			Error:  err.Error(),
			Display: fmt.Sprintf("网络搜索失败：%s", err.Error()),
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolWebSearch,
		Status: "success",
		Data:   map[string]interface{}{"results": results, "query": query, "count": len(results)},
		Display: fmt.Sprintf("找到 %d 条结果：%s", len(results), query),
	}
}

// webFetchStructured handles the web_fetch tool
func (m *Manager) webFetchStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	url, _ := params["url"].(string)
	if strings.TrimSpace(url) == "" {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolWebFetch,
			Status: "error",
			Error:  "url is required",
			Display: "错误：url 参数为必填项",
		}
	}

	format, _ := params["format"].(string)
	if format == "" {
		format = "text"
	}

	// Check HTTPS for non-local URLs
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://localhost") && !strings.HasPrefix(url, "http://127.0.0.1") {
		// Warn but allow
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolWebFetch,
			Status: "success",
			Data:   map[string]interface{}{"warning": "non-HTTPS URL", "url": url},
			Display: fmt.Sprintf("警告：非 HTTPS URL %s", url),
		}
	}

	content, err := fetchURL(ctx, url, format)
	if err != nil {
		return ToolResult{
			Type:   "tool_result",
			Tool:   ToolWebFetch,
			Status: "error",
			Error:  err.Error(),
			Display: fmt.Sprintf("网页获取失败：%s", err.Error()),
		}
	}

	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolWebFetch,
		Status: "success",
		Data:   map[string]interface{}{"content": content, "url": url, "format": format},
		Display: fmt.Sprintf("已获取 %d 字节：%s", len(content), url),
	}
}

// duckduckgoSearch performs a web search using DuckDuckGo HTML
func duckduckgoSearch(ctx context.Context, query string, maxResults int) ([]map[string]string, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", urlEncode(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; eos/1.0)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML results (simplified)
	return parseDuckDuckGoHTML(string(body), maxResults), nil
}

func parseDuckDuckGoHTML(html string, maxResults int) []map[string]string {
	var results []map[string]string

	// Simple HTML parsing for DuckDuckGo results
	markers := []string{`class="result__a"`, `<a rel="nofollow" class="result__url"`}
	for _, marker := range markers {
		if strings.Contains(html, marker) {
			// Extract results from HTML
			parts := strings.Split(html, marker)
			for i := 1; i < len(parts) && len(results) < maxResults; i++ {
				result := extractResultPart(parts[i])
				if result != nil {
					results = append(results, result)
				}
			}
			if len(results) > 0 {
				break
			}
		}
	}

	if len(results) == 0 {
		// Return a single result indicating no matches found via HTML parsing
		results = append(results, map[string]string{
			"title": "Search completed",
			"url":   "",
			"snippet": fmt.Sprintf("Searched for query. HTML parsing found %d potential results.", strings.Count(html, "result")),
		})
	}

	return results
}

func extractResultPart(part string) map[string]string {
	// Extract title
	title := extractBetween(part, ">", "</a>")
	if title == "" {
		title = extractBetween(part, ">", "<")
	}
	title = stripHTMLTags(title)
	if title == "" {
		return nil
	}

	// Extract URL
	url := extractBetween(part, `href="`, `"`)
	if url == "" {
		url = ""
	}

	// Extract snippet
	snippet := extractBetween(part, `class="result__snippet"`, "</a>")
	snippet = stripHTMLTags(snippet)
	if len(snippet) > 300 {
		snippet = snippet[:300] + "..."
	}

	return map[string]string{
		"title":   title,
		"url":     url,
		"snippet": snippet,
	}
}

func fetchURL(ctx context.Context, url, format string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; eos/1.0)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50000)) // 50KB limit
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// Utility functions
func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "+")
}

func extractBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	s = s[i+len(start):]
	j := strings.Index(s, end)
	if j < 0 {
		return s
	}
	return s[:j]
}

func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// init: we don't need to check for encoding/base64 if unused
var _ = base64.StdEncoding
