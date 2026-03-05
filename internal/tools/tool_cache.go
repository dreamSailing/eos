package tools

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/dreamSailing/vb-coding/internal/pkg/utils"
)

// ToolCache 工具输出缓存，基于文件 mtime 的失效机制
type ToolCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration
}

// cacheEntry 缓存条目
type cacheEntry struct {
	result   ToolResult
	mtime    int64     // 文件最后修改时间
	expireAt time.Time // 过期时间
}

// defaultCacheMaxSize 默认最大缓存条目数
const defaultCacheMaxSize = 100

// defaultCacheTTL 默认缓存 TTL
const defaultCacheTTL = 30 * time.Second

// NewToolCache 创建工具输出缓存
func NewToolCache() *ToolCache {
	return &ToolCache{
		entries: make(map[string]*cacheEntry),
		maxSize: defaultCacheMaxSize,
		ttl:     defaultCacheTTL,
	}
}

// cacheKey 生成缓存键
func cacheKey(toolName string, params map[string]interface{}) string {
	bs, _ := json.Marshal(params)
	hash := sha256.Sum256(bs)
	return fmt.Sprintf("%s:%x", toolName, hash[:8])
}

// Get 查询缓存，返回缓存结果和是否命中
func (c *ToolCache) Get(toolName string, params map[string]interface{}) (ToolResult, bool) {
	key := cacheKey(toolName, params)

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return ToolResult{}, false
	}

	// 检查 TTL 是否过期
	if time.Now().After(entry.expireAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		slog.Debug("tool_cache.expired", "component", utils.ComponentTool, "key", key)
		return ToolResult{}, false
	}

	// 检查文件 mtime 是否变化（仅对有路径的条目）
	if entry.mtime > 0 {
		path := extractPathFromParams(params)
		if path != "" {
			currentMtime := getFileMtime(path)
			if currentMtime != entry.mtime {
				c.mu.Lock()
				delete(c.entries, key)
				c.mu.Unlock()
				slog.Debug("tool_cache.mtime_changed",
					"component", utils.ComponentTool,
					"key", key,
					"cached_mtime", entry.mtime,
					"current_mtime", currentMtime)
				return ToolResult{}, false
			}
		}
	}

	slog.Debug("tool_cache.hit", "component", utils.ComponentTool, "key", key)
	return entry.result, true
}

// Put 写入缓存
func (c *ToolCache) Put(toolName string, params map[string]interface{}, result ToolResult) {
	key := cacheKey(toolName, params)

	// 获取文件 mtime（如果有路径）
	var mtime int64
	path := extractPathFromParams(params)
	if path != "" {
		mtime = getFileMtime(path)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 淘汰策略：超过最大容量时删除最旧的条目
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{
		result:   result,
		mtime:    mtime,
		expireAt: time.Now().Add(c.ttl),
	}

	slog.Debug("tool_cache.put", "component", utils.ComponentTool, "key", key, "entries", len(c.entries))
}

// Invalidate 清除指定路径相关的缓存
func (c *ToolCache) Invalidate(path string) {
	if path == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		entryPath, _ := entry.result.Data["path"].(string)
		if entryPath == path {
			delete(c.entries, key)
			slog.Debug("tool_cache.invalidate", "component", utils.ComponentTool, "key", key, "path", path)
		}
	}
}

// evictOldest 淘汰最旧的缓存条目（需持有写锁）
func (c *ToolCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.expireAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expireAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// extractPathFromParams 从参数中提取路径
func extractPathFromParams(params map[string]interface{}) string {
	if p, ok := params["path"].(string); ok {
		return p
	}
	if p, ok := params["file"].(string); ok {
		return p
	}
	return ""
}

// getFileMtime 获取文件最后修改时间的 Unix 时间戳
func getFileMtime(path string) int64 {
	absPath := utils.ResolvePathSimple(path)
	info, err := os.Stat(absPath)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

// IsCacheable 判断工具调用是否可缓存（只读操作）
func IsCacheable(toolName string, params map[string]interface{}) bool {
	if toolName == ToolRead {
		mode, _ := params["mode"].(string)
		// 缓存 file、exists、directory 模式
		return mode == "file" || mode == "" || mode == "exists" || mode == "directory"
	}
	return false
}
