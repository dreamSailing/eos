package history

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Manager 输入历史管理器
type Manager struct {
	aiHistory   []string
	bashHistory []string
	maxSize     int
	historyFile string
}

// NewManager 创建新的历史管理器
func NewManager(maxSize int) *Manager {
	if maxSize <= 0 {
		maxSize = 1000
	}

	// 历史文件路径
	homeDir, _ := os.UserHomeDir()
	historyFile := filepath.Join(homeDir, ".vb-coding", "history.txt")

	m := &Manager{
		aiHistory:   make([]string, 0, maxSize),
		bashHistory: make([]string, 0, maxSize),
		maxSize:     maxSize,
		historyFile: historyFile,
	}

	// 加载历史
	m.Load()

	return m
}

// AddAI 添加AI模式历史
func (m *Manager) AddAI(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}

	// 检查重复
	if len(m.aiHistory) > 0 && m.aiHistory[len(m.aiHistory)-1] == entry {
		return
	}

	m.aiHistory = append(m.aiHistory, entry)
	if len(m.aiHistory) > m.maxSize {
		m.aiHistory = m.aiHistory[len(m.aiHistory)-m.maxSize:]
	}

	// 保存到文件
	m.Save()
}

// AddBash 添加Bash模式历史
func (m *Manager) AddBash(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}

	// 检查重复
	if len(m.bashHistory) > 0 && m.bashHistory[len(m.bashHistory)-1] == entry {
		return
	}

	m.bashHistory = append(m.bashHistory, entry)
	if len(m.bashHistory) > m.maxSize {
		m.bashHistory = m.bashHistory[len(m.bashHistory)-m.maxSize:]
	}

	// 保存到文件
	m.Save()
}

// GetAI 获取AI历史
func (m *Manager) GetAI() []string {
	result := make([]string, len(m.aiHistory))
	copy(result, m.aiHistory)
	return result
}

// GetBash 获取Bash历史
func (m *Manager) GetBash() []string {
	result := make([]string, len(m.bashHistory))
	copy(result, m.bashHistory)
	return result
}

// ClearAI 清空AI历史
func (m *Manager) ClearAI() {
	m.aiHistory = m.aiHistory[:0]
	m.Save()
}

// ClearBash 清空Bash历史
func (m *Manager) ClearBash() {
	m.bashHistory = m.bashHistory[:0]
	m.Save()
}

// Save 保存历史到文件
func (m *Manager) Save() error {
	// 确保目录存在
	dir := filepath.Dir(m.historyFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(m.historyFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// 写入AI历史
	writer.WriteString("# AI History\n")
	for _, entry := range m.aiHistory {
		writer.WriteString("AI:" + entry + "\n")
	}

	// 写入Bash历史
	writer.WriteString("# Bash History\n")
	for _, entry := range m.bashHistory {
		writer.WriteString("BASH:" + entry + "\n")
	}

	return writer.Flush()
}

// Load 从文件加载历史
func (m *Manager) Load() error {
	file, err := os.Open(m.historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "AI:") {
			m.aiHistory = append(m.aiHistory, strings.TrimPrefix(line, "AI:"))
		} else if strings.HasPrefix(line, "BASH:") {
			m.bashHistory = append(m.bashHistory, strings.TrimPrefix(line, "BASH:"))
		}
	}

	return scanner.Err()
}
