package bridge

import (
	"context"
	"github.com/dreamSailing/vb-coding/internal/ai"
	"github.com/dreamSailing/vb-coding/internal/config"
	codectx "github.com/dreamSailing/vb-coding/internal/context"
	"github.com/dreamSailing/vb-coding/internal/mcp"
	"github.com/dreamSailing/vb-coding/internal/pkg/git"
	"github.com/dreamSailing/vb-coding/internal/pkg/settings"
	"github.com/dreamSailing/vb-coding/internal/pkg/workspace"
	"github.com/dreamSailing/vb-coding/internal/runtime"
	"github.com/dreamSailing/vb-coding/internal/session"
	"github.com/dreamSailing/vb-coding/internal/skills"
	"github.com/dreamSailing/vb-coding/internal/tools"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
)

// CoreUI UI 核心接口
type CoreUI interface {
	ClearContent()
	T(key string) string
	WriteLine(color, text string)
	PromptPermission(category, summary string) string
	ClearReviewText()
}

// Logger 日志接口
type Logger interface {
	LogPrimary(msg string)
	LogMuted(msg string)
	ShowHint(msg string)
}

// Attachment 附件信息
type Attachment struct {
	ID          int
	Path        string
	Mime        string
	Placeholder string
}

// UserInputAttachments 用户输入附件
type UserInputAttachments struct {
	Text        string
	ImagePaths  []string
	Attachments []Attachment
}

var pendingAttachments UserInputAttachments

// SetPendingAttachments 设置待处理的附件
func SetPendingAttachments(text string, attachments []Attachment) {
	pendingAttachments.Text = text
	pendingAttachments.Attachments = attachments
	pendingAttachments.ImagePaths = nil
	for _, a := range attachments {
		pendingAttachments.ImagePaths = append(pendingAttachments.ImagePaths, a.Path)
	}
}

// GetPendingAttachments 获取待处理的附件
func GetPendingAttachments() UserInputAttachments {
	return pendingAttachments
}

// ClearPendingAttachments 清除待处理的附件
func ClearPendingAttachments() {
	pendingAttachments = UserInputAttachments{}
}

// Event 运行时事件
type Event struct {
	Type    string
	RID     string
	Content string
	Data    map[string]any
}

// RuntimeCore 运行时核心结构
type RuntimeCore struct {
	cm           *session.ContextManager
	tm           *tools.Manager
	mcpMgr       *mcp.Manager
	skillsLoader *skills.Loader
	ctxEngine    *codectx.Engine
	workspaceMgr *codectx.MultiEngine
	gitMgr       *git.Manager
	wsMgr        *workspace.Manager
	settingsMgr  *settings.Manager
	hooks        runtime.SafetyGate
	reqCh        chan any
	eventsCh     chan Event // 新增：事件通道
	mu           sync.RWMutex
	onMeta       func(string) // 已弃用
	onDelta      func(string) // 已弃用
	onReasoning  func(string) // 已弃用
	modelName    string
	modelBase    string
	tokenHistory []TokenRecord
	tokenMu      sync.RWMutex

	metricsMu   sync.Mutex
	inflightReq map[string]*RequestMetric
	reqHistory  []RequestMetric

	// 安全与授权管理
	securityMgr *SecurityManager
	promptMu    sync.Mutex
	prompts     map[string]chan PromptResponse

	// 设置状态
	settings settings.Settings

	// LSP 支持（条件编译）
	lspManager      *lspManagerEntry
	sessionLockPath string

	// Goroutine 追踪
	wg   sync.WaitGroup
	done chan struct{}

	summaryMu                    sync.Mutex
	lastConversationSummaryRound int
	conversationSummaryEvery     int
	conversationSummaryRunning   bool

	pendingMu     sync.Mutex
	pendingReload map[chan error]struct{}
	pendingGraph  map[chan graphInvokeRes]struct{}
	pendingTools  map[chan toolsNodeRes]struct{}
	pendingSumm   map[chan summarizeRes]struct{}

	panicMu   sync.Mutex
	panicAt   time.Time
	panicHits int

	// 流式解析器
	parser *StreamParser
}

type PromptResponse struct {
	Decision    string
	Option      string
	OptionIndex int
	Text        string
}

// StreamParser 用于解析 AI 输出中的协议标记
type StreamParser struct {
	eventsCh  chan Event
	buffer    strings.Builder
	mode      string // "normal", "task", "final"
	agentName string
	mu        sync.Mutex
}

// NewStreamParser 创建新的流式解析器
func NewStreamParser(eventsCh chan Event) *StreamParser {
	return &StreamParser{
		eventsCh: eventsCh,
		mode:     "normal",
	}
}

// Reset 重置解析器状态
func (p *StreamParser) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buffer.Reset()
	p.mode = "normal"
	p.agentName = ""
}

// Process 处理流式数据
func (p *StreamParser) Process(delta string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buffer.WriteString(delta)

	for {
		fullText := p.buffer.String()
		if fullText == "" {
			return
		}

		if p.mode == "normal" {
			// 查找标记
			idxTask := strings.Index(fullText, "agent.task:")
			idxFinal := strings.Index(fullText, "agent.final:")

			// 找到最早的标记
			idx := -1
			nextMode := ""
			tagLen := 0

			if idxTask >= 0 && (idxFinal == -1 || idxTask < idxFinal) {
				idx = idxTask
				nextMode = "task"
				tagLen = 11
			} else if idxFinal >= 0 {
				idx = idxFinal
				nextMode = "final"
				tagLen = 12
			}

			if idx >= 0 {
				// 发送标记前的内容
				if idx > 0 {
					p.eventsCh <- Event{Type: "delta", Content: fullText[:idx]}
				}
				// 切换模式
				p.mode = nextMode
				// 移除已处理部分（包括标记）
				p.buffer.Reset()
				p.buffer.WriteString(fullText[idx+tagLen:])
				continue // 继续处理剩余部分
			}

			// 没找到标记，检查是否有部分匹配
			matchLen := 0
			checkStr := "agent.final:" // 包含 "agent.task:" 的大部分

			// 从末尾开始匹配
			l := len(fullText)
			maxCheck := min(l, len(checkStr))

			for i := 1; i <= maxCheck; i++ {
				suffix := fullText[l-i:]
				if strings.HasPrefix(checkStr, suffix) || strings.HasPrefix("agent.task:", suffix) {
					matchLen = i
				}
			}

			if matchLen > 0 {
				// 发送安全部分
				safeLen := l - matchLen
				if safeLen > 0 {
					p.eventsCh <- Event{Type: "delta", Content: fullText[:safeLen]}
					p.buffer.Reset()
					p.buffer.WriteString(fullText[safeLen:])
				}
				// 剩余部分保留在 buffer 中
				return
			} else {
				// 没有匹配，全部发送
				p.eventsCh <- Event{Type: "delta", Content: fullText}
				p.buffer.Reset()
				return
			}

		} else if p.mode == "task" {
			// 在 task 模式下，我们要找 agent.final:
			content, rest, found := strings.Cut(fullText, "agent.final:")
			if found {
				// 找到了 final，结束 task
				// 提取 name 和 content
				name := p.agentName
				taskContent := content
				if name == "" {
					content = strings.TrimSpace(content)
					parts := strings.SplitN(content, " ", 2)
					if len(parts) > 0 {
						name = parts[0]
						p.agentName = name
					}
					if len(parts) > 1 {
						taskContent = parts[1]
					} else {
						taskContent = ""
					}
				}
				p.eventsCh <- Event{Type: "agent.task", RID: name, Content: strings.TrimSpace(taskContent)}

				p.mode = "final"
				p.buffer.Reset()
				p.buffer.WriteString(rest)
				continue
			}

			// 没找到，继续缓冲
			return

		} else if p.mode == "final" {
			// 在 final 模式下，检测 agent.task:
			content, rest, found := strings.Cut(fullText, "agent.task:")
			if found {
				// 结束 final
				// 提取 name 和 content
				name := p.agentName
				finalContent := content
				p.eventsCh <- Event{Type: "agent.final", RID: name, Content: strings.TrimSpace(finalContent)}

				p.mode = "task"
				p.buffer.Reset()
				p.buffer.WriteString(rest)
				continue
			}

			return
		}
	}
}

// Flush 刷新缓冲区
func (p *StreamParser) Flush() {
	p.mu.Lock()
	defer p.mu.Unlock()

	content := p.buffer.String()
	if content == "" {
		return
	}

	switch p.mode {
	case "task":
		// task 结束（流结束）
		name := p.agentName
		taskContent := content
		if name == "" {
			parts := strings.SplitN(content, " ", 2)
			if len(parts) > 0 {
				name = parts[0]
			}
			if len(parts) > 1 {
				taskContent = parts[1]
			} else {
				taskContent = ""
			}
		}
		p.eventsCh <- Event{Type: "agent.task", RID: name, Content: strings.TrimSpace(taskContent)}
	case "final":
		// final 结束
		// 尝试提取 name (agent.final:name content)
		name := p.agentName // 沿用 task 的 name
		finalContent := content

		// 检查是否有新的 name
		// 假设 final 内容不包含 agent.final:
		// 如果 content 开头是 name? 很难判断
		// 我们假设 agent.final 后面直接是内容，或者 name space content
		// 如果 name 已经设置，且 content 看起来不像以 name 开头...

		p.eventsCh <- Event{Type: "agent.final", RID: name, Content: strings.TrimSpace(finalContent)}
	default:
		// normal
		p.eventsCh <- Event{Type: "delta", Content: content}
	}

	p.buffer.Reset()
	p.mode = "normal"
	p.agentName = ""
}

// TokenRecord 每轮 Token 使用记录
type TokenRecord struct {
	Timestamp time.Time
	Model     string
	Input     int
	Reply     int
	Total     int
}

// TokenStats Token 累计统计
type TokenStats struct {
	Rounds int
	Input  int
	Reply  int
	Total  int
}

// ModelTokenStats 单个模型的 Token 统计
type ModelTokenStats struct {
	Model  string
	Rounds int
	Input  int
	Reply  int
	Total  int
}

type graphInvokeReq struct {
	ctx           context.Context
	query         string
	executionMode string
	imagePaths    []string
	resCh         chan graphInvokeRes
}

type graphInvokeRes struct {
	msg *schema.Message
	err error
}

type toolsNodeReq struct {
	ctx   context.Context
	text  string
	resCh chan toolsNodeRes
}

type toolsNodeRes struct {
	results  []string
	executed bool
	cont     bool
}

type summarizeReq struct {
	ctx   context.Context
	text  string
	resCh chan summarizeRes
}

type summarizeRes struct {
	text string
	err  error
}

type finalizeTaskReq struct {
	traceID       string
	userText      string
	assistantText string
	success       bool
	errorMsg      string
	resCh         chan struct{}
}

type hookEventReq struct {
	ctx   context.Context
	event string
	path  string
}

type reloadReq struct {
	resCh chan error
}

// NewRuntimeCore 创建运行时核心实例
// Events 返回运行时事件通道
func (rc *RuntimeCore) Events() <-chan Event {
	return rc.eventsCh
}

func NewRuntimeCore(cm *session.ContextManager, tm *tools.Manager, ui CoreUI) *RuntimeCore {
	skillsLoader := skills.NewLoader()

	// 初始化时就获取并设置模型信息
	_, _, modelName := ai.ResolveAPISettings()
	cfg, _ := config.Load()
	var modelBase string
	if cfg.Active != "" {
		if m, ok := config.ActiveModel(cfg); ok {
			modelBase = m.APIBase
			if modelName == "" {
				modelName = m.Model
			}
		}
	}

	rc := &RuntimeCore{
		cm:                       cm,
		tm:                       tm,
		mcpMgr:                   mcp.NewManager(),
		skillsLoader:             skillsLoader,
		gitMgr:                   git.NewManager(tm, cm),
		settingsMgr:              settings.NewManager(""),
		settings:                 settings.Settings{AutoContext: true},
		securityMgr:              NewSecurityManager(),
		prompts:                  make(map[string]chan PromptResponse),
		reqCh:                    make(chan any),
		eventsCh:                 make(chan Event, 100), // 新增：事件通道，带缓冲
		done:                     make(chan struct{}),
		conversationSummaryEvery: 6,
		pendingReload:            make(map[chan error]struct{}),
		pendingGraph:             make(map[chan graphInvokeRes]struct{}),
		pendingTools:             make(map[chan toolsNodeRes]struct{}),
		pendingSumm:              make(map[chan summarizeRes]struct{}),
		modelName:                modelName,
		modelBase:                modelBase,
	}
	rc.parser = NewStreamParser(rc.eventsCh)

	skillManager := tools.NewSkillManager(skillsLoader, tm)
	tm.SetSkillManager(skillManager)
	tm.SetMCPManager(rc.mcpMgr)

	hooks := runtime.SafetyGate{
		Classify: func(call tools.ToolCall) (string, string, string, bool) {
			return tools.ClassifyToolDanger(call)
		},
		Prompt: func(ctx context.Context, category, summary string) string {
			return rc.promptPermission(ctx, category, summary)
		},
		SessionAllowed: func(category string) bool {
			return rc.securityMgr.IsAllowed(category)
		},
		AllowSession: func(category string) {
			rc.securityMgr.AllowSession(category)
		},
		SetPendingDiff: func(diff string) {
			rc.securityMgr.SetPendingDiff(diff)
		},
		ClearReviewText: func() {
			if ui != nil {
				ui.ClearReviewText()
			}
		},
	}
	rc.hooks = hooks
	rc.securityMgr.SetHooks(hooks)

	tools.SafetyGatePrompt = hooks.Prompt
	tools.SafetyGateSessionAllowed = hooks.SessionAllowed
	tools.SafetyGateAllowSession = hooks.AllowSession
	tools.SafetyGateClassify = hooks.Classify
	tools.SetPendingDiff = hooks.SetPendingDiff
	tools.ClearReviewText = hooks.ClearReviewText
	tools.UserConfirmPrompt = rc.userConfirmPrompt
	tools.AskUserQuestionPrompt = rc.askUserQuestionPrompt
	tools.OnToolCall = func(id string, toolName string) {
		rc.RecordToolCall(id, toolName)
	}
	tools.OnToolResult = func(id string, toolName string, success bool) {
		rc.RecordToolResult(id, toolName, success)
	}

	// 初始化 LSP 管理器（可选功能）
	rc.lspManager = rc.initLSPManager()

	// 创建会话锁，用于检测非正常退出
	rc.syncSessionLock()

	rc.wg.Add(1)
	go rc.loop()
	return rc
}
