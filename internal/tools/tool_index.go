package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。


import (
	"log/slog"
	"sort"
	"strings"
	"sync"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

// ToolIndex 工具索引，用于快速搜索和发现工具
type ToolIndex struct {
	mu           sync.RWMutex
	byCategory   map[string][]string          // 按分类索引的工具
	byRiskLevel  map[ToolRiskLevel][]string   // 按风险等级索引的工具
	byKeyword    map[string][]string          // 按关键词索引的工具
	toolInfo     map[string]*ToolIndexEntry   // 工具详细信息
	searchCache  map[string]*ToolSearchResult // 搜索结果缓存
	maxCacheSize int
}

// ToolIndexEntry 工具索引条目
type ToolIndexEntry struct {
	Name        string // 工具名称
	Description string // 工具描述
	RiskLevel   ToolRiskLevel
	Categories  []string // 分类标签
	Keywords    []string // 搜索关键词
	Params      []string // 参数名称列表
}

// ToolSearchResult 工具搜索结果
type ToolSearchResult struct {
	Query    string           // 搜索查询
	Matches  []ToolMatchEntry // 匹配的工具
	Total    int              // 总匹配数
	Duration int64            // 搜索耗时（毫秒）
}

// ToolMatchEntry 工具匹配条目
type ToolMatchEntry struct {
	Name        string // 工具名称
	Description string // 工具描述
	RiskLevel   ToolRiskLevel
	Score       float64 // 相关性评分 (0-1)
	MatchReason string  // 匹配原因
}

// NewToolIndex 创建工具索引
func NewToolIndex() *ToolIndex {
	idx := &ToolIndex{
		byCategory:   make(map[string][]string),
		byRiskLevel:  make(map[ToolRiskLevel][]string),
		byKeyword:    make(map[string][]string),
		toolInfo:     make(map[string]*ToolIndexEntry),
		searchCache:  make(map[string]*ToolSearchResult),
		maxCacheSize: 100,
	}
	idx.rebuild()
	return idx
}

// rebuild 重建工具索引
func (idx *ToolIndex) rebuild() {
	defs := GetAllToolDefinitions()

	for _, def := range defs {
		entry := &ToolIndexEntry{
			Name:        def.Name,
			Description: def.Description,
			RiskLevel:   def.RiskLevel,
			Categories:  idx.inferCategories(def),
			Keywords:    idx.extractKeywords(def),
			Params:      idx.extractParamNames(def),
		}

		idx.toolInfo[def.Name] = entry

		// 按分类索引
		for _, cat := range entry.Categories {
			idx.byCategory[cat] = append(idx.byCategory[cat], def.Name)
		}

		// 按风险等级索引
		idx.byRiskLevel[entry.RiskLevel] = append(idx.byRiskLevel[entry.RiskLevel], def.Name)

		// 按关键词索引
		for _, kw := range entry.Keywords {
			idx.byKeyword[strings.ToLower(kw)] = append(idx.byKeyword[strings.ToLower(kw)], def.Name)
		}
	}

	// 去重并排序
	idx.deduplicate()
}

// deduplicate 去重并排序索引
func (idx *ToolIndex) deduplicate() {
	for cat, tools := range idx.byCategory {
		idx.byCategory[cat] = uniqueSorted(tools)
	}
	for level, tools := range idx.byRiskLevel {
		idx.byRiskLevel[level] = uniqueSorted(tools)
	}
	for kw, tools := range idx.byKeyword {
		idx.byKeyword[kw] = uniqueSorted(tools)
	}
}

// uniqueSorted 去重并排序字符串切片
func uniqueSorted(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	sort.Strings(result)
	return result
}

// inferCategories 推断工具分类
func (idx *ToolIndex) inferCategories(def ToolDefinition) []string {
	var categories []string

	// 基于工具名称推断
	switch {
	case strings.Contains(def.Name, "git"):
		categories = append(categories, "git", "version_control")
	case strings.Contains(def.Name, "bash") || strings.Contains(def.Name, "shell"):
		categories = append(categories, "shell", "command")
	case strings.Contains(def.Name, "file") || strings.Contains(def.Name, "read") ||
		strings.Contains(def.Name, "fs") || strings.Contains(def.Name, "edit"):
		categories = append(categories, "file", "filesystem")
	case strings.Contains(def.Name, "search") || strings.Contains(def.Name, "grep"):
		categories = append(categories, "search", "query")
	case strings.Contains(def.Name, "plan") || strings.Contains(def.Name, "todo"):
		categories = append(categories, "planning", "organization")
	}

	// 基于风险等级分类
	switch def.RiskLevel {
	case RiskLevelLow:
		categories = append(categories, "read_only", "safe")
	case RiskLevelMedium:
		categories = append(categories, "write")
	case RiskLevelHigh:
		categories = append(categories, "dangerous", "execute")
	}

	return uniqueSorted(categories)
}

// extractKeywords 从工具定义提取关键词
func (idx *ToolIndex) extractKeywords(def ToolDefinition) []string {
	desc := strings.ToLower(def.Description)
	var keywords []string

	// 从描述中提取关键词
	commonWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"can": true, "for": true, "of": true, "with": true, "by": true, "from": true,
		"to": true, "in": true, "on": true, "at": true, "or": true, "and": true,
		"工具": true, "用于": true, "文件": true, "目录": true, "执行": true, "操作": true,
	}

	words := strings.Fields(desc)
	for _, word := range words {
		word = strings.Trim(word, ".,;:!()[]{}\"'")
		if len(word) > 2 && !commonWords[word] {
			keywords = append(keywords, word)
		}
	}

	// 添加工具名作为关键词
	keywords = append(keywords, def.Name)

	return uniqueSorted(keywords)
}

// extractParamNames 提取参数名称
func (idx *ToolIndex) extractParamNames(def ToolDefinition) []string {
	if def.Params == nil {
		return nil
	}
	var params []string
	for name := range def.Params {
		params = append(params, name)
	}
	sort.Strings(params)
	return params
}

// Search 搜索工具
func (idx *ToolIndex) Search(query string) *ToolSearchResult {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return &ToolSearchResult{
			Query:   query,
			Matches: idx.listAllTools(),
			Total:   len(idx.toolInfo),
		}
	}

	// 检查缓存
	idx.mu.RLock()
	if cached, ok := idx.searchCache[query]; ok {
		idx.mu.RUnlock()
		return cached
	}
	idx.mu.RUnlock()

	idx.mu.Lock()
	defer idx.mu.Unlock()

	result := &ToolSearchResult{
		Query:   query,
		Matches: []ToolMatchEntry{},
	}

	seen := make(map[string]bool)
	matchSet := make(map[string]ToolMatchEntry)

	// 1. 精确名称匹配
	for name, entry := range idx.toolInfo {
		if name == query {
			matchSet[name] = ToolMatchEntry{
				Name:        entry.Name,
				Description: entry.Description,
				RiskLevel:   entry.RiskLevel,
				Score:       1.0,
				MatchReason: "精确名称匹配",
			}
			seen[name] = true
		}
	}

	// 2. 关键词匹配
	for kw, tools := range idx.byKeyword {
		if strings.Contains(kw, query) || strings.Contains(query, kw) {
			for _, name := range tools {
				if !seen[name] {
					if entry, ok := idx.toolInfo[name]; ok {
						score := idx.calculateScore(entry, query, kw)
						if score > 0.3 {
							matchSet[name] = ToolMatchEntry{
								Name:        entry.Name,
								Description: entry.Description,
								RiskLevel:   entry.RiskLevel,
								Score:       score,
								MatchReason: "关键词匹配: " + kw,
							}
							seen[name] = true
						}
					}
				}
			}
		}
	}

	// 3. 描述匹配
	for name, entry := range idx.toolInfo {
		if !seen[name] {
			desc := strings.ToLower(entry.Description)
			if strings.Contains(desc, query) {
				score := 0.6
				if strings.HasPrefix(entry.Description, query) {
					score = 0.8
				}
				matchSet[name] = ToolMatchEntry{
					Name:        entry.Name,
					Description: entry.Description,
					RiskLevel:   entry.RiskLevel,
					Score:       score,
					MatchReason: "描述匹配",
				}
				seen[name] = true
			}
		}
	}

	// 4. 分类匹配
	for cat, tools := range idx.byCategory {
		if strings.Contains(cat, query) || strings.Contains(query, cat) {
			for _, name := range tools {
				if !seen[name] {
					if entry, ok := idx.toolInfo[name]; ok {
						matchSet[name] = ToolMatchEntry{
							Name:        entry.Name,
							Description: entry.Description,
							RiskLevel:   entry.RiskLevel,
							Score:       0.5,
							MatchReason: "分类匹配: " + cat,
						}
						seen[name] = true
					}
				}
			}
		}
	}

	// 按评分排序
	for _, match := range matchSet {
		result.Matches = append(result.Matches, match)
	}
	sort.Slice(result.Matches, func(i, j int) bool {
		if result.Matches[i].Score != result.Matches[j].Score {
			return result.Matches[i].Score > result.Matches[j].Score
		}
		return result.Matches[i].Name < result.Matches[j].Name
	})

	result.Total = len(result.Matches)

	// 缓存结果
	idx.addToCache(query, result)

	slog.Debug("tools.index.search",
		"component", utils.ComponentTool,
		"query", query,
		"matches", result.Total,
		"top_match", func() string {
			if len(result.Matches) > 0 {
				return result.Matches[0].Name
			}
			return ""
		}(),
	)

	return result
}

// calculateScore 计算匹配评分
func (idx *ToolIndex) calculateScore(entry *ToolIndexEntry, query, matchedKeyword string) float64 {
	score := 0.5

	// 精确关键词匹配
	if matchedKeyword == query {
		score = 0.9
	} else if strings.HasPrefix(matchedKeyword, query) {
		score = 0.8
	} else if strings.HasSuffix(matchedKeyword, query) {
		score = 0.7
	}

	// 根据关键词位置调整
	if strings.Contains(entry.Name, query) {
		score += 0.1
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// listAllTools 列出所有工具
func (idx *ToolIndex) listAllTools() []ToolMatchEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]ToolMatchEntry, 0, len(idx.toolInfo))
	for _, entry := range idx.toolInfo {
		result = append(result, ToolMatchEntry{
			Name:        entry.Name,
			Description: entry.Description,
			RiskLevel:   entry.RiskLevel,
			Score:       0.5,
			MatchReason: "全部工具",
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// GetByCategory 按分类获取工具
func (idx *ToolIndex) GetByCategory(category string) []ToolMatchEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	tools, ok := idx.byCategory[strings.ToLower(category)]
	if !ok {
		return []ToolMatchEntry{}
	}

	result := make([]ToolMatchEntry, 0, len(tools))
	for _, name := range tools {
		if entry, ok := idx.toolInfo[name]; ok {
			result = append(result, ToolMatchEntry{
				Name:        entry.Name,
				Description: entry.Description,
				RiskLevel:   entry.RiskLevel,
				Score:       1.0,
				MatchReason: "分类: " + category,
			})
		}
	}
	return result
}

// GetByRiskLevel 按风险等级获取工具
func (idx *ToolIndex) GetByRiskLevel(level ToolRiskLevel) []ToolMatchEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	tools, ok := idx.byRiskLevel[level]
	if !ok {
		return []ToolMatchEntry{}
	}

	result := make([]ToolMatchEntry, 0, len(tools))
	for _, name := range tools {
		if entry, ok := idx.toolInfo[name]; ok {
			result = append(result, ToolMatchEntry{
				Name:        entry.Name,
				Description: entry.Description,
				RiskLevel:   entry.RiskLevel,
				Score:       1.0,
				MatchReason: "风险等级",
			})
		}
	}
	return result
}

// GetCategories 获取所有分类
func (idx *ToolIndex) GetCategories() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	cats := make([]string, 0, len(idx.byCategory))
	for cat := range idx.byCategory {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	return cats
}

// addToCache 添加搜索结果到缓存
func (idx *ToolIndex) addToCache(query string, result *ToolSearchResult) {
	if len(idx.searchCache) >= idx.maxCacheSize {
		// 简单清理：删除一半缓存
		count := 0
		for k := range idx.searchCache {
			delete(idx.searchCache, k)
			count++
			if count >= idx.maxCacheSize/2 {
				break
			}
		}
	}
	idx.searchCache[query] = result
}

// ClearCache 清除搜索缓存
func (idx *ToolIndex) ClearCache() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.searchCache = make(map[string]*ToolSearchResult)
}

// GetToolInfo 获取工具详细信息
func (idx *ToolIndex) GetToolInfo(name string) (*ToolIndexEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	entry, ok := idx.toolInfo[name]
	return entry, ok
}

// GetStats 获取索引统计信息
func (idx *ToolIndex) GetStats() map[string]any {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return map[string]any{
		"total_tools":      len(idx.toolInfo),
		"total_categories": len(idx.byCategory),
		"total_keywords":   len(idx.byKeyword),
		"cached_searches":  len(idx.searchCache),
		"categories":       idx.GetCategories(),
	}
}
