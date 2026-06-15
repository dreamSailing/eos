package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import "github.com/cloudwego/eino/schema"

// ToolExample 工具使用示例
type ToolExample struct {
	Description string                 // 示例描述
	Input       map[string]interface{} // 输入参数示例
}

// ToolDefinition 工具定义，包含工具的名称、描述和参数信息
type ToolDefinition struct {
	Name            string                           // 工具名称
	Description     string                           // 工具描述
	Params          map[string]*schema.ParameterInfo // 参数定义
	RiskLevel       ToolRiskLevel                    // 风险等级：low/medium/high
	Examples        []ToolExample                    // 使用示例（提升模型理解复杂参数的准确率）
	ConcurrencySafe    bool                             // 标记工具是否可安全并行执行
	Category           string                           // 工具分类（如 "Office 文档"、"Git 版本控制"）
	ReadOnly           bool                             // 是否为只读操作
	NeedsSandboxRunner bool                             // 是否需要 sandbox runner 执行
}

// ToolRiskLevel 工具风险等级
type ToolRiskLevel int

const (
	RiskLevelLow    ToolRiskLevel = iota // 只读工具，如读取文件、查看目录
	RiskLevelMedium                      // 写入工具，如创建/修改文件
	RiskLevelHigh                        // 危险工具，如删除、执行命令
)

// ToolNames 所有工具名称的常量定义
const (
	ToolRead                    = "read"
	ToolFS                      = "fs"
	ToolEdit                    = "edit"
	ToolHistory                 = "history"
	ToolSearch                  = "search"
	ToolToolSearch              = "tool_search" // 工具搜索工具
	ToolSkill                   = "skill"       // Agent Skills meta-tool
	ToolSkillsList              = "skills_list"
	ToolCreateSkill             = "create_skill"
	ToolTimeNow                 = "time_now"
	ToolUserConfirm             = "user_confirm"
	ToolUserInput               = "user_input"
	ToolUserSelect              = "user_select"
	ToolBash                    = "bash"
	ToolBashSession             = "bash_session"
	ToolBGTask                  = "bg_task"
	ToolPlanSteps               = "plan_steps"
	ToolTodoRead                = "todo_read"
	ToolTodoWrite               = "todo_write"
	ToolMCPStatus               = "mcp_status"
	ToolBrowserStatus           = "browser_status"
	ToolBrowserNavigate         = "browser_navigate"
	ToolBrowserSnapshot         = "browser_snapshot"
	ToolBrowserInspect          = "browser_inspect"
	ToolBrowserTabs             = "browser_tabs"
	ToolBrowserBack             = "browser_back"
	ToolBrowserForward          = "browser_forward"
	ToolBrowserClick            = "browser_click"
	ToolBrowserHover            = "browser_hover"
	ToolBrowserType             = "browser_type"
	ToolBrowserPressKey         = "browser_press_key"
	ToolBrowserSelect           = "browser_select"
	ToolBrowserWait             = "browser_wait"
	ToolBrowserScroll           = "browser_scroll"
	ToolBrowserScreenshot       = "browser_screenshot"
	ToolBrowserConsole          = "browser_console"
	ToolBrowserNetwork          = "browser_network"
	ToolBrowserReload           = "browser_reload"
	ToolBrowserViewport         = "browser_viewport"
	ToolBrowserVisibility       = "browser_visibility"
	ToolBrowserClipboard        = "browser_clipboard"
	ToolBrowserCUA              = "browser_cua"
	ToolBrowserDOMCUA           = "browser_dom_cua"
	ToolBrowserLocator          = "browser_locator"
	ToolBrowserDevLogs          = "browser_dev_logs"
	ToolBrowserDownloads        = "browser_downloads"
	ToolBrowserUserTabs         = "browser_user_tabs"
	ToolBrowserSessionName      = "browser_session_name"
	ToolGitStatus               = "git_status"
	ToolGitAdd                  = "git_add"
	ToolGitCommit               = "git_commit"
	ToolGitBranchList           = "git_branch_list"
	ToolGitCheckout             = "git_checkout"
	ToolGitInit                 = "git_init"
	ToolGitPull                 = "git_pull"
	ToolGitPush                 = "git_push"
	ToolGitDiff                 = "git_diff"
	ToolGitLog                  = "git_log"
	ToolGitShow                 = "git_show"
	ToolGitStash                = "git_stash"
	ToolGitReset                = "git_reset"
	ToolGitRevert               = "git_revert"
	ToolGitMerge                = "git_merge"
	ToolGitRebase               = "git_rebase"
	ToolProjectStructure        = "ProjectStructure"
	ToolAskUserQuestion         = "ask_user_question"
	ToolEnterPlanMode           = "enter_plan_mode"
	ToolExitPlanMode            = "exit_plan_mode"
	ToolAgent                   = "agent"
	ToolSuggestMemory           = "suggest_memory"
	ToolWebSearch               = "web_search"
	ToolWebFetch                = "web_fetch"
	ToolEnterWorktree           = "enter_worktree"
	ToolExitWorktree            = "exit_worktree"
	ToolNotebookEdit            = "notebook_edit"
	ToolDocumentGenerate        = "document_generate"
	ToolDocumentConvert         = "document_convert"
	ToolImageGenerate           = "image_generate"
	ToolVideoGenerate           = "video_generate"
	ToolSpeechSynthesize        = "speech_synthesize"
	ToolMCPListResources        = "mcp_list_resources"
	ToolMCPReadResource         = "mcp_read_resource"
	ToolMCPListPrompts          = "mcp_list_prompts"
	ToolMCPGetPrompt            = "mcp_get_prompt"
	ToolPowerShell              = "powershell"
	ToolStructuredOutput        = "structured_output"
	ToolSnip                    = "snip"
	ToolTeamCreate              = "team_create"
	ToolTeamDelete              = "team_delete"
	ToolTeamSendMsg             = "team_send_message"
	ToolRemoteRepoConnect       = "remote_repo_connect"
	ToolRemoteRepoStatus        = "remote_repo_status"
	ToolRemoteRepoCloneOrOpen   = "remote_repo_clone_or_open"
	ToolRemoteRepoCheckout      = "remote_repo_checkout"
	ToolRemoteRepoCommitAndPush = "remote_repo_commit_and_push"
	ToolRemoteRepoCreatePR      = "remote_repo_create_pr"
	ToolRemoteRepoCreateMR      = "remote_repo_create_mr"
	ToolRemoteRepoDisconnect    = "remote_repo_disconnect"
	ToolPatch                   = "patch"
)

// GetAllToolDefinitions 返回所有工具的定义
func GetAllToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        ToolAskUserQuestion,
			Description: "向用户提问并获取用户的选择或文本回答。可提供选项列表。",
			Params: map[string]*schema.ParameterInfo{
				"question": {Type: schema.String, Required: true, Desc: "要询问用户的问题"},
				"options":  {Type: schema.Array, Required: false, Desc: "可选：提供给用户的选项列表（字符串数组）"},
			},
			RiskLevel:       RiskLevelLow,
			ConcurrencySafe: true,
			Examples: []ToolExample{
				{Description: "询问用户一个开放性问题", Input: map[string]any{"question": "你需要什么帮助？"}},
				{Description: "询问用户并提供选项", Input: map[string]any{"question": "选择操作模式", "options": []string{"自动", "手动"}}},
			},
		},
		{
			Name:        ToolTimeNow,
			Description: "获取权威日期时间（NTP 校准，国内外通用）。返回本地/UTC时间、Unix时间戳、星期、ISO周数、季度等丰富信息。time_source 标识时间来源(ntp/system)。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelLow,
			Examples: []ToolExample{
				{Description: "获取权威日期时间（NTP 校准）", Input: map[string]any{}},
			},
		},
		{
			Name:        ToolUserConfirm,
			Description: "向用户发起确认请求，可提供选项并允许用户输入补充意见；返回用户选择与文本。",
			Params: map[string]*schema.ParameterInfo{
				"title":      {Type: schema.String, Required: false, Desc: "可选：标题"},
				"question":   {Type: schema.String, Required: true, Desc: "要用户确认的问题"},
				"options":    {Type: schema.Array, Required: false, Desc: "可选：建议选项列表（字符串数组）"},
				"allow_text": {Type: schema.Boolean, Required: false, Desc: "是否允许用户输入补充意见（默认 true）"},
				"text_hint":  {Type: schema.String, Required: false, Desc: "可选：用户输入提示"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{Description: "让用户在几条方案中选择并给出补充意见", Input: map[string]any{"question": "选择一个发布策略", "options": []any{"方案A：灰度发布", "方案B：直接全量", "方案C：先预发布验证"}, "allow_text": true, "text_hint": "补充你的约束/偏好"}},
			},
		},
		{
			Name:        ToolUserInput,
			Description: "让用户输入文本/数字（底层复用确认弹窗的输入框），并返回结构化结果。",
			Params: map[string]*schema.ParameterInfo{
				"title":       {Type: schema.String, Required: false, Desc: "可选：标题"},
				"question":    {Type: schema.String, Required: true, Desc: "提示用户输入的内容"},
				"input_type":  {Type: schema.String, Required: false, Desc: "输入类型: text(默认), integer, number"},
				"allow_empty": {Type: schema.Boolean, Required: false, Desc: "是否允许空输入（默认 false）"},
				"text_hint":   {Type: schema.String, Required: false, Desc: "可选：输入框提示"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{Description: "让用户输入一段文本", Input: map[string]any{"question": "请输入你的需求补充", "input_type": "text"}},
				{Description: "让用户输入整数", Input: map[string]any{"question": "请输入端口号", "input_type": "integer"}},
			},
		},
		{
			Name:        ToolUserSelect,
			Description: "让用户从候选项中单选/多选（底层复用确认弹窗的选项列表与输入框）。",
			Params: map[string]*schema.ParameterInfo{
				"title":      {Type: schema.String, Required: false, Desc: "可选：标题"},
				"question":   {Type: schema.String, Required: true, Desc: "要用户选择的问题"},
				"options":    {Type: schema.Array, Required: true, Desc: "候选项列表（字符串数组）"},
				"multi":      {Type: schema.Boolean, Required: false, Desc: "是否多选（默认 false）"},
				"allow_text": {Type: schema.Boolean, Required: false, Desc: "是否允许用户输入补充意见（默认 false；多选时会强制为 true）"},
				"text_hint":  {Type: schema.String, Required: false, Desc: "可选：输入框提示"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{Description: "单选", Input: map[string]any{"question": "选择一个方案", "options": []any{"方案A", "方案B"}, "multi": false}},
				{Description: "多选（用输入框填 1,3）", Input: map[string]any{"question": "选择需要启用的功能", "options": []any{"功能1", "功能2", "功能3"}, "multi": true}},
			},
		},
		{
			Name:        ToolMCPStatus,
			Description: "查询 MCP 服务器加载状态与最近一次错误（按需排障用）。",
			Params: map[string]*schema.ParameterInfo{
				"name": {Type: schema.String, Required: false, Desc: "可选：仅查询指定 MCP server 名称"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{Description: "查询所有 MCP 状态", Input: map[string]any{}},
				{Description: "查询单个 MCP 状态", Input: map[string]any{"name": "http-proxy"}},
			},
		},
		{
			Name:        ToolBrowserStatus,
			Description: "查询浏览器 MCP（推荐 Playwright）是否已配置、启用和成功加载。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelLow,
			Examples: []ToolExample{
				{Description: "检查浏览器能力是否可用", Input: map[string]any{}},
			},
		},
		{
			Name:        ToolBrowserNavigate,
			Description: "使用内置浏览器后端导航到目标 URL，并在当前会话中记录页面内容与网络请求。",
			Params: map[string]*schema.ParameterInfo{
				"url": {Type: schema.String, Required: true, Desc: "完整目标 URL"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "打开一个网页", Input: map[string]any{"url": "https://example.com"}},
			},
		},
		{
			Name:        ToolBrowserSnapshot,
			Description: "读取当前浏览器会话中的页面快照摘要，并返回当前 active tab 元信息、稳定元素 ref 列表以及轻量层级 outline。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "获取当前页面快照", Input: map[string]any{}},
			},
		},
		{
			Name:        ToolBrowserInspect,
			Description: "读取某个页面元素的细粒度详情。优先推荐使用 browser_snapshot 返回的 ref，也兼容 selector。",
			Params: map[string]*schema.ParameterInfo{
				"ref":      {Type: schema.String, Required: false, Desc: "来自 browser_snapshot 的稳定元素引用"},
				"selector": {Type: schema.String, Required: false, Desc: "兼容模式：目标元素选择器"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "查看快照元素详情", Input: map[string]any{"ref": "e1"}},
				{Description: "查看按钮详情", Input: map[string]any{"selector": "button[type=submit]"}},
			},
		},
		{
			Name:        ToolBrowserTabs,
			Description: "管理当前浏览器会话的标签页。支持 list、current、new、switch、activate_last、close、close_others、close_right 八种 action，切换或关闭时可用 id、index 或 match 定位目标标签页。",
			Params: map[string]*schema.ParameterInfo{
				"action":   {Type: schema.String, Required: true, Desc: "操作类型：list、current、new、switch、activate_last、close、close_others 或 close_right"},
				"id":       {Type: schema.String, Required: false, Desc: "可选：目标标签页 ID"},
				"index":    {Type: schema.Integer, Required: false, Desc: "可选：目标标签页索引（从 0 开始）"},
				"match":    {Type: schema.String, Required: false, Desc: "可选：按标签页标题、URL 或 ID 做模糊匹配"},
				"url":      {Type: schema.String, Required: false, Desc: "action=new 时可选：新标签页打开的 URL"},
				"activate": {Type: schema.Boolean, Required: false, Desc: "action=new 时可选：是否立即切换到新标签页，默认 true"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "列出当前标签页", Input: map[string]any{"action": "list"}},
				{Description: "读取当前 active 标签页", Input: map[string]any{"action": "current"}},
				{Description: "打开新标签页", Input: map[string]any{"action": "new", "url": "https://example.com"}},
				{Description: "后台打开新标签页", Input: map[string]any{"action": "new", "url": "https://example.com/docs", "activate": false}},
				{Description: "切换到第二个标签页", Input: map[string]any{"action": "switch", "index": 1}},
				{Description: "按标题或 URL 匹配切换标签页", Input: map[string]any{"action": "switch", "match": "docs"}},
				{Description: "切回最近活跃的标签页", Input: map[string]any{"action": "activate_last"}},
				{Description: "关闭其他标签页，仅保留当前匹配页", Input: map[string]any{"action": "close_others", "match": "docs"}},
				{Description: "关闭目标标签页右侧的所有标签页", Input: map[string]any{"action": "close_right", "index": 0}},
				{Description: "关闭指定标签页", Input: map[string]any{"action": "close", "id": "tab-2"}},
			},
		},
		{
			Name:        ToolBrowserBack,
			Description: "让当前浏览器会话回退到上一页。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "回到上一页", Input: map[string]any{}},
			},
		},
		{
			Name:        ToolBrowserForward,
			Description: "让当前浏览器会话前进到下一页。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "前进到下一页", Input: map[string]any{}},
			},
		},
		{
			Name:        ToolBrowserClick,
			Description: "在当前页面点击目标元素，优先推荐使用 browser_snapshot 返回的 ref，也兼容 selector。",
			Params: map[string]*schema.ParameterInfo{
				"ref":      {Type: schema.String, Required: false, Desc: "来自 browser_snapshot 的稳定元素引用"},
				"selector": {Type: schema.String, Required: false, Desc: "兼容模式：目标元素选择器"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "点击快照元素", Input: map[string]any{"ref": "e1"}},
				{Description: "点击登录按钮", Input: map[string]any{"selector": "button[type=submit]"}},
			},
		},
		{
			Name:        ToolBrowserHover,
			Description: "在当前页面悬停到指定元素上，优先推荐使用 ref，也兼容 selector。",
			Params: map[string]*schema.ParameterInfo{
				"ref":      {Type: schema.String, Required: false, Desc: "来自 browser_snapshot 的稳定元素引用"},
				"selector": {Type: schema.String, Required: false, Desc: "兼容模式：目标元素选择器"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "悬停到快照元素", Input: map[string]any{"ref": "e2"}},
				{Description: "悬停到菜单按钮", Input: map[string]any{"selector": ".menu-trigger"}},
			},
		},
		{
			Name:        ToolBrowserType,
			Description: "在当前页面向输入框填写文本，优先推荐使用 ref，也兼容 selector。",
			Params: map[string]*schema.ParameterInfo{
				"ref":      {Type: schema.String, Required: false, Desc: "来自 browser_snapshot 的稳定元素引用"},
				"selector": {Type: schema.String, Required: false, Desc: "兼容模式：输入框选择器"},
				"text":     {Type: schema.String, Required: true, Desc: "要输入的文本"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "向快照输入框填值", Input: map[string]any{"ref": "e3", "text": "admin@example.com"}},
				{Description: "填写邮箱", Input: map[string]any{"selector": "input[type=email]", "text": "admin@example.com"}},
			},
		},
		{
			Name:        ToolBrowserPressKey,
			Description: "向当前页面或指定元素发送键盘按键。",
			Params: map[string]*schema.ParameterInfo{
				"keys":     {Type: schema.String, Required: true, Desc: "要发送的键值，如 Enter、Tab 或普通文本"},
				"ref":      {Type: schema.String, Required: false, Desc: "可选：来自 browser_snapshot 的稳定元素引用"},
				"selector": {Type: schema.String, Required: false, Desc: "可选：先聚焦的目标元素"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "向输入框发送回车键", Input: map[string]any{"selector": "#search", "keys": "\n"}},
			},
		},
		{
			Name:        ToolBrowserSelect,
			Description: "在当前页面选择下拉选项，优先推荐使用 ref，也兼容 selector。",
			Params: map[string]*schema.ParameterInfo{
				"ref":      {Type: schema.String, Required: false, Desc: "来自 browser_snapshot 的稳定元素引用"},
				"selector": {Type: schema.String, Required: false, Desc: "兼容模式：下拉框选择器"},
				"values":   {Type: schema.Array, Required: true, Desc: "要选择的值列表"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "选择快照下拉框中的值", Input: map[string]any{"ref": "e4", "values": []string{"cn"}}},
				{Description: "选择一个地区", Input: map[string]any{"selector": "select#region", "values": []string{"cn"}}},
			},
		},
		{
			Name:        ToolBrowserWait,
			Description: "等待页面变化或超时。可选传入选择器和超时时间（毫秒）。",
			Params: map[string]*schema.ParameterInfo{
				"ref":      {Type: schema.String, Required: false, Desc: "可选：来自 browser_snapshot 的稳定元素引用"},
				"selector": {Type: schema.String, Required: false, Desc: "可选：等待的选择器"},
				"timeout":  {Type: schema.Integer, Required: false, Desc: "可选：等待毫秒数"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "等待 1 秒", Input: map[string]any{"timeout": 1000}},
			},
		},
		{
			Name:        ToolBrowserScroll,
			Description: "滚动窗口或将指定元素滚动到视口内。",
			Params: map[string]*schema.ParameterInfo{
				"ref":      {Type: schema.String, Required: false, Desc: "可选：来自 browser_snapshot 的稳定元素引用"},
				"selector": {Type: schema.String, Required: false, Desc: "可选：要滚动到视口内的元素"},
				"x":        {Type: schema.Integer, Required: false, Desc: "可选：窗口横向滚动偏移"},
				"y":        {Type: schema.Integer, Required: false, Desc: "可选：窗口纵向滚动偏移"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "窗口向下滚动", Input: map[string]any{"y": 600}},
				{Description: "将元素滚动到视口内", Input: map[string]any{"selector": "#footer"}},
			},
		},
		{
			Name:        ToolBrowserScreenshot,
			Description: "获取当前页面截图。path 可选；未传时返回可渲染图片数据，传入时保存到工作区内路径。",
			Params: map[string]*schema.ParameterInfo{
				"path":      {Type: schema.String, Required: false, Desc: "可选：输出文件路径"},
				"full_page": {Type: schema.Boolean, Required: false, Desc: "是否截取完整页面，默认视口截图"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "保存页面快照", Input: map[string]any{"path": "artifacts/page.html"}},
			},
		},
		{
			Name:        ToolBrowserConsole,
			Description: "读取当前浏览器会话的控制台信息。",
			Params: map[string]*schema.ParameterInfo{
				"limit": {Type: schema.Integer, Required: false, Desc: "可选：返回条数限制"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "读取最近控制台消息", Input: map[string]any{"limit": 20}},
			},
		},
		{
			Name:        ToolBrowserNetwork,
			Description: "读取当前浏览器会话记录到的网络请求。",
			Params: map[string]*schema.ParameterInfo{
				"limit": {Type: schema.Integer, Required: false, Desc: "可选：返回条数限制"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "读取最近网络请求", Input: map[string]any{"limit": 20}},
			},
		},
		{
			Name:        ToolBrowserReload,
			Description: "重新加载当前浏览器标签页。",
			Params: map[string]*schema.ParameterInfo{
				"ignore_cache": {Type: schema.Boolean, Required: false, Desc: "可选：是否绕过缓存刷新"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserViewport,
			Description: "读取、设置或重置当前浏览器视口。action 支持 get、set、reset。",
			Params: map[string]*schema.ParameterInfo{
				"action": {Type: schema.String, Required: false, Desc: "get、set 或 reset，默认 get"},
				"width":  {Type: schema.Integer, Required: false, Desc: "set 时的宽度"},
				"height": {Type: schema.Integer, Required: false, Desc: "set 时的高度"},
				"mobile": {Type: schema.Boolean, Required: false, Desc: "是否模拟移动设备视口"},
				"reset":  {Type: schema.Boolean, Required: false, Desc: "兼容字段：true 等价于 action=reset"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserVisibility,
			Description: "读取或设置浏览器可见性状态。headless 后端会记录状态；桌面后端可映射为显示/隐藏面板。",
			Params: map[string]*schema.ParameterInfo{
				"action":  {Type: schema.String, Required: false, Desc: "get、show 或 hide，默认 get"},
				"visible": {Type: schema.Boolean, Required: false, Desc: "兼容字段：true 显示，false 隐藏"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserClipboard,
			Description: "读取或写入当前页面上下文中的剪贴板文本。action 支持 read、write。",
			Params: map[string]*schema.ParameterInfo{
				"action": {Type: schema.String, Required: true, Desc: "read 或 write"},
				"text":   {Type: schema.String, Required: false, Desc: "write 时要写入的文本"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserCUA,
			Description: "执行浏览器范围内的坐标 CUA 动作。action 支持 click、double_click、move、scroll、type、keypress。",
			Params: map[string]*schema.ParameterInfo{
				"action":   {Type: schema.String, Required: true, Desc: "click、double_click、move、scroll、type 或 keypress"},
				"x":        {Type: schema.Integer, Required: false, Desc: "坐标 X"},
				"y":        {Type: schema.Integer, Required: false, Desc: "坐标 Y"},
				"scroll_x": {Type: schema.Integer, Required: false, Desc: "横向滚动量"},
				"scroll_y": {Type: schema.Integer, Required: false, Desc: "纵向滚动量"},
				"text":     {Type: schema.String, Required: false, Desc: "type 时要输入的文本"},
				"keys":     {Type: schema.Array, Required: false, Desc: "keypress 时的按键列表"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserDOMCUA,
			Description: "执行浏览器 DOM CUA 动作。action 支持 get_visible_dom、click、double_click、type、keypress、scroll。",
			Params: map[string]*schema.ParameterInfo{
				"action":  {Type: schema.String, Required: true, Desc: "get_visible_dom、click、double_click、type、keypress 或 scroll"},
				"node_id": {Type: schema.String, Required: false, Desc: "来自 visible DOM/snapshot 的节点 ref"},
				"text":    {Type: schema.String, Required: false, Desc: "type 时要输入的文本"},
				"keys":    {Type: schema.Array, Required: false, Desc: "keypress 时的按键列表"},
				"x":       {Type: schema.Integer, Required: false, Desc: "scroll 横向偏移"},
				"y":       {Type: schema.Integer, Required: false, Desc: "scroll 纵向偏移"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserLocator,
			Description: "Playwright locator 子集。action 支持 count、click、fill、type、press、check、uncheck、set_checked、select、text、attribute、state、wait。",
			Params: map[string]*schema.ParameterInfo{
				"selector":  {Type: schema.String, Required: true, Desc: "CSS selector"},
				"action":    {Type: schema.String, Required: false, Desc: "locator 动作，默认 count"},
				"text":      {Type: schema.String, Required: false, Desc: "fill/type/press 用文本或按键"},
				"value":     {Type: schema.String, Required: false, Desc: "select 用值"},
				"attribute": {Type: schema.String, Required: false, Desc: "attribute 动作的属性名"},
				"state":     {Type: schema.String, Required: false, Desc: "state 动作的状态名，如 visible、enabled、checked"},
				"checked":   {Type: schema.Boolean, Required: false, Desc: "set_checked 目标值"},
				"timeout":   {Type: schema.Integer, Required: false, Desc: "wait 超时毫秒"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserDevLogs,
			Description: "读取当前浏览器会话的开发日志，目前包含 console 日志并为后续浏览器后端保留扩展字段。",
			Params: map[string]*schema.ParameterInfo{
				"limit": {Type: schema.Integer, Required: false, Desc: "可选：返回条数限制"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserDownloads,
			Description: "读取当前浏览器标签页记录到的下载事件。",
			Params: map[string]*schema.ParameterInfo{
				"limit": {Type: schema.Integer, Required: false, Desc: "可选：返回条数限制"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserUserTabs,
			Description: "列出用户/当前会话标签页，兼容 Codex user tabs 能力。",
			Params: map[string]*schema.ParameterInfo{
				"include_background": {Type: schema.Boolean, Required: false, Desc: "是否包含后台标签页，当前内置后端会返回全部标签页"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolBrowserSessionName,
			Description: "设置当前浏览器 session 的展示名称。",
			Params: map[string]*schema.ParameterInfo{
				"name": {Type: schema.String, Required: true, Desc: "session 名称"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolSkillsList,
			Description: "查询当前可用 skills 列表与扫描目录（按需排障用）。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelLow,
			Examples: []ToolExample{
				{Description: "查询 skills 列表", Input: map[string]any{}},
			},
		},
		{
			Name:        ToolCreateSkill,
			Description: "根据用户需求创建一个新的 skill。调用前必须先判断该 skill 更适合工作区还是全局；如果用户未说明创建位置，应先用 ask_user_question 询问，再带上明确的 scope 调用本工具。",
			Params: map[string]*schema.ParameterInfo{
				"name":               {Type: schema.String, Required: false, Desc: "可选：skill 名称；为空时会尝试从需求或生成结果中推导"},
				"request":            {Type: schema.String, Required: true, Desc: "用户的自然语言需求，用于生成完整 SKILL.md"},
				"scope":              {Type: schema.String, Required: true, Desc: "创建范围：workspace 或 user"},
				"description":        {Type: schema.String, Required: false, Desc: "可选：覆盖或补充 skill 描述"},
				"allowed_tools":      {Type: schema.Array, Required: false, Desc: "可选：写入 allowed-tools 的工具名称列表"},
				"model":              {Type: schema.String, Required: false, Desc: "可选：写入 skill frontmatter 的 model"},
				"argument_hint":      {Type: schema.String, Required: false, Desc: "可选：写入 argument-hint"},
				"user_invocable":     {Type: schema.Boolean, Required: false, Desc: "可选：写入 user-invocable"},
				"context":            {Type: schema.String, Required: false, Desc: "可选：写入 context，例如 fork"},
				"agent":              {Type: schema.String, Required: false, Desc: "可选：写入 agent"},
				"keywords":           {Type: schema.Array, Required: false, Desc: "可选：写入 keywords"},
				"include_scripts":    {Type: schema.Boolean, Required: false, Desc: "是否创建 scripts/ 目录"},
				"include_references": {Type: schema.Boolean, Required: false, Desc: "是否创建 references/ 目录"},
				"include_assets":     {Type: schema.Boolean, Required: false, Desc: "是否创建 assets/ 目录"},
				"overwrite":          {Type: schema.Boolean, Required: false, Desc: "若目标 skill 已存在，是否允许覆盖"},
				"activate":           {Type: schema.Boolean, Required: false, Desc: "创建后是否重新加载 skills（默认 true）"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{
					Description: "在当前工作区创建一个项目专属 skill",
					Input: map[string]any{
						"name":    "repo-review",
						"request": "创建一个用于本仓库代码审查的 skill，重点检查 API 兼容性、数据库迁移和测试缺失。",
						"scope":   "workspace",
					},
				},
				{
					Description: "创建一个可跨项目复用的全局 skill",
					Input: map[string]any{
						"request": "创建一个通用的 release notes 生成 skill，可复用于多个项目。",
						"scope":   "user",
						"keywords": []any{
							"release",
							"notes",
						},
					},
				},
			},
		},
		{
			Name:        ToolRead,
			Description: "读取文件系统。通过 mode 选择行为：file(默认) 读取文件内容并返回带行号的文本；directory 列出目录条目；exists 返回路径是否存在；resolve 解析路径并返回候选路径与状态。path 必填，必须是有效的文件系统路径（不要含 '@' 等前缀）。不知道确切路径时，先用 search(mode:glob/text) 或 read(mode:directory) 定位，拿到真实路径再调用本工具。PDF 文件须用 pages 指定页码范围。",
			Params: map[string]*schema.ParameterInfo{
				"mode":  {Type: schema.String, Required: false, Desc: "读取模式: file (默认, 读取文件内容), directory (列出目录条目), exists (检查路径是否存在), resolve (解析路径并返回候选路径与状态)"},
				"path":  {Type: schema.String, Required: true, Desc: "要读取的绝对或相对路径 (e.g., 'main.go', 'internal/utils')"},
				"pages": {Type: schema.String, Required: false, Desc: "PDF 页面范围 (如 '1-5', '10-20')，仅 PDF 格式必填"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{
					Description: "读取文件内容 (正确示例)",
					Input:       map[string]any{"mode": "file", "path": "main.go"},
				},
				{
					Description: "读取文件 (错误示例 - 不要使用 @ 前缀)",
					Input:       map[string]any{"mode": "file", "path": "@main.go"}, // This is just for doc, maybe confusing? Better remove it or explicit say DON'T.
				},
				{
					Description: "列出目录内容",
					Input:       map[string]any{"mode": "directory", "path": "."},
				},
				{
					Description: "检查文件是否存在",
					Input:       map[string]any{"mode": "exists", "path": "README.md"},
				},
			},
		},
		{
			Name:        ToolDocumentGenerate,
			Description: "生成 DOCX/XLSX/PDF 文档。支持纯文本内容和结构化内容输入。",
			Params: map[string]*schema.ParameterInfo{
				"format":             {Type: schema.String, Required: true, Desc: "输出格式：docx、xlsx 或 pdf"},
				"path":               {Type: schema.String, Required: true, Desc: "输出文件路径"},
				"title":              {Type: schema.String, Required: false, Desc: "文档标题"},
				"content":            {Type: schema.String, Required: false, Desc: "纯文本正文"},
				"structured_content": {Type: schema.Object, Required: false, Desc: "结构化内容，可传入文档块/工作表 JSON"},
			},
			RiskLevel:          RiskLevelMedium,
			Category:           "Office 文档",
			ReadOnly:           false,
			NeedsSandboxRunner: true,
			Examples: []ToolExample{
				{Description: "生成 DOCX", Input: map[string]any{"format": "docx", "path": "out/report.docx", "title": "周报", "content": "第一段\n\n第二段"}},
				{Description: "生成 XLSX", Input: map[string]any{"format": "xlsx", "path": "out/table.xlsx", "structured_content": map[string]any{"sheets": []map[string]any{{"name": "Sheet1", "rows": [][]string{{"A", "B"}, {"1", "2"}}}}}}},
			},
		},
		{
			Name:        ToolDocumentConvert,
			Description: "在 DOCX/XLSX/PDF 之间进行文档转换。默认优先高保真转换，必要时回退为内容级转换并附带告警。",
			Params: map[string]*schema.ParameterInfo{
				"source_path":      {Type: schema.String, Required: true, Desc: "源文件路径"},
				"target_format":    {Type: schema.String, Required: true, Desc: "目标格式：docx、xlsx 或 pdf"},
				"destination_path": {Type: schema.String, Required: false, Desc: "输出文件路径；为空时自动推导"},
				"fidelity":         {Type: schema.String, Required: false, Desc: "转换保真度：high（默认）或 content"},
			},
			RiskLevel:          RiskLevelMedium,
			Category:           "Office 文档",
			ReadOnly:           false,
			NeedsSandboxRunner: true,
			Examples: []ToolExample{
				{Description: "高保真转换为 PDF", Input: map[string]any{"source_path": "report.docx", "target_format": "pdf", "fidelity": "high"}},
				{Description: "内容级转换为 XLSX", Input: map[string]any{"source_path": "notes.pdf", "target_format": "xlsx", "fidelity": "content"}},
			},
		},
		{
			Name:        ToolImageGenerate,
			Description: "调用当前主模型或专项图片模型生成图片，并将结果保存到工作区。",
			Params: map[string]*schema.ParameterInfo{
				"prompt":      {Type: schema.String, Required: true, Desc: "图片生成提示词"},
				"output_path": {Type: schema.String, Required: false, Desc: "可选：输出路径；为空时自动生成"},
				"size":        {Type: schema.String, Required: false, Desc: "可选：图片尺寸，如 1024x1024"},
				"count":       {Type: schema.Integer, Required: false, Desc: "可选：生成数量，首版默认 1"},
				"model":       {Type: schema.String, Required: false, Desc: "可选：覆盖自动能力路由，指定使用的模型名"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "生成一张图片", Input: map[string]any{"prompt": "一只坐在键盘前写代码的橘猫", "output_path": "outputs/cat.png", "size": "1024x1024"}},
			},
		},
		{
			Name:        ToolVideoGenerate,
			Description: "调用当前主模型或专项视频模型生成视频，并将结果保存到工作区。",
			Params: map[string]*schema.ParameterInfo{
				"prompt":           {Type: schema.String, Required: true, Desc: "视频生成提示词"},
				"output_path":      {Type: schema.String, Required: false, Desc: "可选：输出路径；为空时自动生成"},
				"duration_seconds": {Type: schema.Integer, Required: false, Desc: "可选：视频时长（秒）"},
				"aspect_ratio":     {Type: schema.String, Required: false, Desc: "可选：画面比例，如 16:9"},
				"image_input":      {Type: schema.String, Required: false, Desc: "可选：图生视频的输入图片路径"},
				"model":            {Type: schema.String, Required: false, Desc: "可选：覆盖自动能力路由，指定使用的模型名"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "生成短视频", Input: map[string]any{"prompt": "夜晚城市街道的延时摄影", "output_path": "outputs/city.mp4", "duration_seconds": 5, "aspect_ratio": "16:9"}},
			},
		},
		{
			Name:        ToolSpeechSynthesize,
			Description: "调用当前主模型或专项语音模型合成语音，并将结果保存到工作区。",
			Params: map[string]*schema.ParameterInfo{
				"text":        {Type: schema.String, Required: true, Desc: "要合成的文本"},
				"output_path": {Type: schema.String, Required: false, Desc: "可选：输出路径；为空时自动生成"},
				"voice":       {Type: schema.String, Required: false, Desc: "可选：音色/发音人"},
				"format":      {Type: schema.String, Required: false, Desc: "可选：输出格式，如 mp3 或 wav"},
				"model":       {Type: schema.String, Required: false, Desc: "可选：覆盖自动能力路由，指定使用的模型名"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{Description: "合成中文语音", Input: map[string]any{"text": "欢迎使用 EOS。", "output_path": "outputs/welcome.mp3", "voice": "alloy", "format": "mp3"}},
			},
		},
		{
			Name:        ToolFS,
			Description: "文件系统写操作（非读取）。通过 mode 选择行为：write 覆盖写入文件、create 创建文件或目录、mkdir 建目录、delete 删除文件或目录、move/copy 移动或复制、diff 生成当前内容与给定 content 的差异。各模式所需参数不同：write/create/mkdir/delete/diff 需 path（delete/diff 还需 content），move/copy 需 source+destination。检查文件是否存在或读取内容请用 read 工具。path 必须是确切路径；不确定时先用 search 定位。",
			Params: map[string]*schema.ParameterInfo{
				"mode":        {Type: schema.String, Required: false, Desc: "操作模式: write (写入/覆盖文件), create (创建文件/目录), mkdir (创建目录), delete (删除文件/目录), move (移动文件/目录), copy (复制文件/目录), diff (生成差异)"},
				"path":        {Type: schema.String, Required: false, Desc: "文件/目录路径 (用于 write, create, mkdir, delete, diff)"},
				"source":      {Type: schema.String, Required: false, Desc: "源路径 (用于 move, copy)"},
				"destination": {Type: schema.String, Required: false, Desc: "目标路径 (用于 move, copy)"},
				"content":     {Type: schema.String, Required: false, Desc: "文件内容 (用于 write, create, diff)"},
				"type":        {Type: schema.String, Required: false, Desc: "类型: 'file' 或 'dir' (用于 create, copy; 如省略则自动检测)"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{
					Description: "创建新文件",
					Input:       map[string]any{"mode": "create", "path": "main.go", "content": "package main\n\nfunc main() {}\n"},
				},
				{
					Description: "覆盖写入文件",
					Input:       map[string]any{"mode": "write", "path": "config.json", "content": "{\"key\": \"value\"}"},
				},
				{
					Description: "创建目录",
					Input:       map[string]any{"mode": "mkdir", "path": "internal/utils"},
				},
				{
					Description: "删除文件",
					Input:       map[string]any{"mode": "delete", "path": "old_file.go"},
				},
				{
					Description: "移动文件",
					Input:       map[string]any{"mode": "move", "source": "old.txt", "destination": "new.txt"},
				},
				{
					Description: "复制文件",
					Input:       map[string]any{"mode": "copy", "source": "template.yaml", "destination": "config.yaml"},
				},
				{
					Description: "生成文件差异预览",
					Input:       map[string]any{"mode": "diff", "path": "main.go", "content": "new content"},
				},
			},
		},
		{
			Name:        ToolSearch,
			Description: "在项目中搜索文件或代码。pattern 必填，必须来自用户问题、错误日志或已读代码中的真实关键词，不要用空 pattern 或泛泛猜测词。通过 mode 选择行为：glob 按文件名模式匹配（如 **/*.go）、regex 按正则搜内容、text 按纯文本搜内容、code 做代码语义搜索。root 限定搜索根目录，limit 限制结果数。建议先搜内容定位文件，再 read 相关片段；多次搜索关键词不同时可在同一轮并行调用。",
			Params: map[string]*schema.ParameterInfo{
				"mode":             {Type: schema.String, Required: false, Desc: "搜索模式: glob (文件名匹配), regex (正则内容搜索), text (文本内容搜索), code (代码语义搜索)"},
				"pattern":          {Type: schema.String, Required: true, Desc: "搜索模式或查询字符串"},
				"root":             {Type: schema.String, Required: false, Desc: "搜索根目录"},
				"max_depth":        {Type: schema.Integer, Required: false, Desc: "最大搜索深度"},
				"exclude":          {Type: schema.Array, Required: false, Desc: "排除模式列表"},
				"min_size":         {Type: schema.Integer, Required: false},
				"max_size":         {Type: schema.Integer, Required: false},
				"limit":            {Type: schema.Integer, Required: false, Desc: "最大结果数"},
				"context":          {Type: schema.Integer, Required: false},
				"case_insensitive": {Type: schema.Boolean, Required: false, Desc: "忽略大小写"},
				"flags":            {Type: schema.String, Required: false},
				"k":                {Type: schema.Integer, Required: false},
				"depth":            {Type: schema.Integer, Required: false},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{
					Description: "按文件名模式搜索",
					Input:       map[string]any{"mode": "glob", "pattern": "**/*.go", "root": "."},
				},
				{
					Description: "正则表达式内容搜索",
					Input:       map[string]any{"mode": "regex", "pattern": `func\s+(\w+).*error`, "root": "."},
				},
				{
					Description: "文本内容搜索",
					Input:       map[string]any{"mode": "text", "pattern": "TODO", "root": ".", "caseInsensitive": true},
				},
			},
		},
		{
			Name:        ToolBash,
			Description: "在用户默认 shell 中同步执行命令并返回 stdout/stderr 与退出码。command 必填。仅用于专用工具（read/search/edit/fs）覆盖不到的系统命令，如构建、测试、git、进程管理；不要用它替代 cat/sed/grep/ls。运行前必须知道当前工作目录和命令目的；不确定命令是否存在时先查项目脚本或文档。命令失败先分析原因，不要直接换一个猜测命令。",
			Params: map[string]*schema.ParameterInfo{
				"command": {Type: schema.String, Required: true, Desc: "要执行的命令"},
			},
			RiskLevel: RiskLevelHigh,
		},
		{
			Name:        ToolBashSession,
			Description: "统一 Bash 会话工具 (模式: start, output, kill)",
			Params: map[string]*schema.ParameterInfo{
				"mode":    {Type: schema.String, Required: true, Desc: "会话模式: start (启动后台会话), output (获取输出), kill (终止会话)"},
				"id":      {Type: schema.String, Required: false, Desc: "会话 ID (用于 output, kill)"},
				"command": {Type: schema.String, Required: false, Desc: "要执行的命令 (用于 start)"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{
					Description: "启动后台命令",
					Input:       map[string]any{"mode": "start", "command": "npm run dev"},
				},
				{
					Description: "获取会话输出",
					Input:       map[string]any{"mode": "output", "id": "session_123"},
				},
				{
					Description: "终止会话",
					Input:       map[string]any{"mode": "kill", "id": "session_123"},
				},
			},
		},
		{
			Name:        ToolBGTask,
			Description: "后台任务管理 (start/list/info/tail/kill/cleanup)。用于长期运行程序、查看日志、终止任务。",
			Params: map[string]*schema.ParameterInfo{
				"action":      {Type: schema.String, Required: true, Desc: "操作: start, list, info, tail, kill, cleanup"},
				"id":          {Type: schema.String, Required: false, Desc: "任务 ID (用于 info/tail/kill)"},
				"command":     {Type: schema.String, Required: false, Desc: "要启动的命令 (用于 start)"},
				"working_dir": {Type: schema.String, Required: false, Desc: "工作目录 (用于 start)"},
				"env":         {Type: schema.Array, Required: false, Desc: "环境变量列表 (如 KEY=VALUE) (用于 start)"},
				"log_cap":     {Type: schema.Integer, Required: false, Desc: "保留日志行数上限 (用于 start)"},
				"from_seq":    {Type: schema.Integer, Required: false, Desc: "从该序号之后开始拉取 (用于 tail)"},
				"limit":       {Type: schema.Integer, Required: false, Desc: "最多返回条数 (用于 tail)"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{Description: "启动后台任务", Input: map[string]any{"action": "start", "command": "npm run dev", "working_dir": ".", "log_cap": 2000}},
				{Description: "列出任务", Input: map[string]any{"action": "list"}},
				{Description: "查看任务信息", Input: map[string]any{"action": "info", "id": "123"}},
				{Description: "拉取增量日志", Input: map[string]any{"action": "tail", "id": "123", "from_seq": 0, "limit": 200}},
				{Description: "终止任务", Input: map[string]any{"action": "kill", "id": "123"}},
			},
		},
		{
			Name:        ToolEdit,
			Description: "在文件中做精确文本替换，不整文件重写。mode 必填：single 单次替换（需 file+find，replace 可选留空即删除匹配）、multi 同文件多处替换（需 file+edits）、batch 跨文件编辑（需 edits，每项含 file/find/replace）。find 必须是文件中真实存在的文本（或 regex:true 时为正则），不要凭记忆猜 find 内容——编辑前先 read 目标片段拿到准确文本。limit 限制单次替换次数；caseInsensitive 忽略大小写。匹配不到时工具会报错，应回头读文件确认 find 文本而非盲目重试。",
			Params: map[string]*schema.ParameterInfo{
				"mode":            {Type: schema.String, Required: true, Desc: "编辑模式: single (单次替换), multi (同文件多次编辑), batch (跨文件编辑)"},
				"file":            {Type: schema.String, Required: false, Desc: "文件路径 (single/multi 模式必填)"},
				"find":            {Type: schema.String, Required: false, Desc: "查找文本 (single 模式必填)"},
				"replace":         {Type: schema.String, Required: false, Desc: "替换文本 (single 模式选填)"},
				"limit":           {Type: schema.Integer, Required: false, Desc: "最大替换次数 (用于 single, multi)"},
				"regex":           {Type: schema.Boolean, Required: false, Desc: "使用正则 (用于 single, multi)"},
				"caseInsensitive": {Type: schema.Boolean, Required: false, Desc: "忽略大小写 (用于 single, multi)"},
				"edits":           {Type: schema.Array, Required: false, Desc: "编辑列表 (multi/batch 模式必填)"},
				"previewOnly":     {Type: schema.Boolean, Required: false, Desc: "仅预览 (用于 batch)"},
				"format":          {Type: schema.Boolean, Required: false, Desc: "编辑后格式化 (用于 batch)"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{
					Description: "单次字符串替换",
					Input:       map[string]any{"mode": "single", "file": "main.go", "find": "oldFunc", "replace": "newFunc"},
				},
				{
					Description: "正则替换",
					Input:       map[string]any{"mode": "single", "file": "main.go", "find": `func\s+(\w+)`, "replace": "func $1_new", "regex": true},
				},
				{
					Description: "同文件多次编辑",
					Input: map[string]any{
						"mode": "multi",
						"file": "config.go",
						"edits": []map[string]any{
							{"find": "port = 8080", "replace": "port = 3000"},
							{"find": "host = \"localhost\"", "replace": "host = \"0.0.0.0\""},
						},
					},
				},
				{
					Description: "跨文件批量编辑",
					Input: map[string]any{
						"mode": "batch",
						"edits": []map[string]any{
							{"file": "file1.go", "find": "TODO", "replace": "FIXME"},
							{"file": "file2.go", "find": "TODO", "replace": "FIXME"},
						},
					},
				},
			},
		},
		{
			Name:        ToolHistory,
			Description: "文件历史快照（非 Git）。支持列出有快照的文件、列出/读取版本、按版本回滚，以及按 trace_id 恢复到某次对话开始前的状态。",
			Params: map[string]*schema.ParameterInfo{
				"mode":       {Type: schema.String, Required: true, Desc: "模式: list_files, list_versions, read_version, rollback, list_checkpoints, restore_checkpoint"},
				"path":       {Type: schema.String, Required: false, Desc: "文件路径（用于 list_versions/read_version/rollback）"},
				"version_id": {Type: schema.String, Required: false, Desc: "版本 ID（用于 read_version/rollback）"},
				"trace_id":   {Type: schema.String, Required: false, Desc: "对话 trace_id（用于 restore_checkpoint）"},
				"limit":      {Type: schema.Integer, Required: false, Desc: "返回条数上限（用于 list_checkpoints）"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{Description: "列出有快照的文件", Input: map[string]any{"mode": "list_files"}},
				{Description: "列出某文件的版本", Input: map[string]any{"mode": "list_versions", "path": "main.go"}},
				{Description: "读取某版本内容", Input: map[string]any{"mode": "read_version", "path": "main.go", "version_id": "20260101-120000.000000000"}},
				{Description: "回滚文件到指定版本", Input: map[string]any{"mode": "rollback", "path": "main.go", "version_id": "20260101-120000.000000000"}},
				{Description: "列出对话级 checkpoint", Input: map[string]any{"mode": "list_checkpoints", "limit": 20}},
				{Description: "恢复到某次对话前状态", Input: map[string]any{"mode": "restore_checkpoint", "trace_id": "abcd1234"}},
			},
		},
		{
			Name:        ToolPlanSteps,
			Description: "根据模糊的用户请求规划可执行步骤",
			Params: map[string]*schema.ParameterInfo{
				"user_request":        {Type: schema.String, Required: true, Desc: "用户请求"},
				"constraints":         {Type: schema.Array, Required: false, Desc: "约束条件"},
				"preferred_languages": {Type: schema.Array, Required: false, Desc: "偏好语言"},
				"max_steps":           {Type: schema.Integer, Required: false, Desc: "最大步骤数"},
				"context_k":           {Type: schema.Integer, Required: false},
				"neighbors_depth":     {Type: schema.Integer, Required: false},
				"attach_snippets":     {Type: schema.Boolean, Required: false},
				"snippet_bytes_limit": {Type: schema.Integer, Required: false},
				"symbols_limit":       {Type: schema.Integer, Required: false},
			},
			RiskLevel: RiskLevelLow,
		},
		{
			Name:        ToolTodoRead,
			Description: "读取当前运行时的待办事项列表",
			Params:      nil,
			RiskLevel:   RiskLevelLow,
		},
		{
			Name:        ToolTodoWrite,
			Description: "替换运行时待办事项列表。items 为数组；每个元素必须包含 content（也兼容 text/title）。status 取 pending/in_progress/completed。",
			Params: map[string]*schema.ParameterInfo{
				"items": {Type: schema.Array, Required: true, Desc: "待办事项列表（元素字段: content/text/title, status, priority, id）"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{
					Description: "设置待办列表",
					Input: map[string]any{"items": []map[string]any{
						{"id": "1", "content": "实现登录页", "status": "in_progress", "priority": "high"},
						{"id": "2", "content": "补充单元测试", "status": "pending", "priority": "medium"},
					}},
				},
			},
		},
		{
			Name:        ToolGitStatus,
			Description: "显示 Git 工作区状态",
			Params:      nil,
			RiskLevel:   RiskLevelLow,
		},
		{
			Name:        ToolGitAdd,
			Description: "暂存文件以提交",
			Params: map[string]*schema.ParameterInfo{
				"paths": {Type: schema.Array, Required: false, Desc: "文件路径列表"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{
					Description: "暂存单个文件",
					Input:       map[string]any{"paths": []string{"main.go"}},
				},
				{
					Description: "暂存多个文件",
					Input:       map[string]any{"paths": []string{"file1.go", "file2.go", "config.yaml"}},
				},
			},
		},
		{
			Name:        ToolGitCommit,
			Description: "提交暂存的更改",
			Params: map[string]*schema.ParameterInfo{
				"message":      {Type: schema.String, Required: true, Desc: "提交信息"},
				"author_name":  {Type: schema.String, Required: false, Desc: "作者姓名"},
				"author_email": {Type: schema.String, Required: false, Desc: "作者邮箱"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{
					Description: "简单提交",
					Input:       map[string]any{"message": "feat: add new feature"},
				},
				{
					Description: "带作者信息的提交",
					Input:       map[string]any{"message": "fix: resolve bug", "author_name": "Developer", "author_email": "dev@example.com"},
				},
			},
		},
		{
			Name:        ToolGitBranchList,
			Description: "列出本地分支",
			Params:      nil,
			RiskLevel:   RiskLevelLow,
		},
		{
			Name:        ToolGitCheckout,
			Description: "检出分支",
			Params: map[string]*schema.ParameterInfo{
				"name":   {Type: schema.String, Required: true, Desc: "分支名称"},
				"create": {Type: schema.Boolean, Required: false, Desc: "如果不存在则创建"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{
					Description: "切换到已有分支",
					Input:       map[string]any{"name": "develop"},
				},
				{
					Description: "创建并切换到新分支",
					Input:       map[string]any{"name": "feature/new-api", "create": true},
				},
			},
		},
		{
			Name:        ToolGitInit,
			Description: "初始化 Git 仓库",
			Params:      nil,
			RiskLevel:   RiskLevelMedium,
		},
		{
			Name:        ToolGitPull,
			Description: "从远程拉取",
			Params: map[string]*schema.ParameterInfo{
				"remote":   {Type: schema.String, Required: false, Desc: "远程仓库"},
				"branch":   {Type: schema.String, Required: false, Desc: "分支"},
				"username": {Type: schema.String, Required: false, Desc: "用户名"},
				"password": {Type: schema.String, Required: false, Desc: "密码/Token"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolGitPush,
			Description: "推送到远程",
			Params: map[string]*schema.ParameterInfo{
				"remote":   {Type: schema.String, Required: false, Desc: "远程仓库"},
				"branch":   {Type: schema.String, Required: false, Desc: "分支"},
				"username": {Type: schema.String, Required: false, Desc: "用户名"},
				"password": {Type: schema.String, Required: false, Desc: "密码/Token"},
			},
			RiskLevel: RiskLevelHigh,
		},
		{
			Name:        ToolRemoteRepoConnect,
			Description: "连接远程 GitHub 或 Gitee 账号，优先复用已保存授权；若未授权，则返回 OAuth 授权信息。",
			Params: map[string]*schema.ParameterInfo{
				"platform": {Type: schema.String, Required: true, Desc: "平台: github 或 gitee"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolRemoteRepoStatus,
			Description: "查看当前会话绑定的远程仓库上下文、授权账号和本地目录。",
			Params:      nil,
			RiskLevel:   RiskLevelLow,
		},
		{
			Name:        ToolRemoteRepoCloneOrOpen,
			Description: "克隆远程 GitHub/Gitee 仓库到隔离目录，或复用本地副本，并将后续工具切换到远程仓库上下文。",
			Params: map[string]*schema.ParameterInfo{
				"platform":    {Type: schema.String, Required: true, Desc: "平台: github 或 gitee"},
				"repo_url":    {Type: schema.String, Required: true, Desc: "远程仓库地址"},
				"base_branch": {Type: schema.String, Required: false, Desc: "可选：要切换/克隆的基线分支"},
			},
			RiskLevel: RiskLevelHigh,
		},
		{
			Name:        ToolRemoteRepoCheckout,
			Description: "在当前远程仓库上下文中切换或创建分支。",
			Params: map[string]*schema.ParameterInfo{
				"branch": {Type: schema.String, Required: true, Desc: "目标分支名"},
				"create": {Type: schema.Boolean, Required: false, Desc: "是否创建新分支"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolRemoteRepoCommitAndPush,
			Description: "在当前远程仓库上下文中执行 add/commit/push，并返回分支与提交信息。",
			Params: map[string]*schema.ParameterInfo{
				"message":      {Type: schema.String, Required: true, Desc: "提交信息"},
				"branch":       {Type: schema.String, Required: false, Desc: "可选：推送分支；为空时使用当前分支"},
				"author_name":  {Type: schema.String, Required: false, Desc: "可选：提交作者名"},
				"author_email": {Type: schema.String, Required: false, Desc: "可选：提交作者邮箱"},
			},
			RiskLevel: RiskLevelHigh,
		},
		{
			Name:        ToolRemoteRepoCreatePR,
			Description: "为当前 GitHub 远程仓库创建 Pull Request。",
			Params: map[string]*schema.ParameterInfo{
				"title": {Type: schema.String, Required: true, Desc: "PR 标题"},
				"body":  {Type: schema.String, Required: false, Desc: "PR 描述"},
				"base":  {Type: schema.String, Required: false, Desc: "目标基线分支"},
				"head":  {Type: schema.String, Required: false, Desc: "源分支"},
			},
			RiskLevel: RiskLevelHigh,
		},
		{
			Name:        ToolRemoteRepoCreateMR,
			Description: "为当前 Gitee 远程仓库创建 Pull Request / Merge Request。",
			Params: map[string]*schema.ParameterInfo{
				"title": {Type: schema.String, Required: true, Desc: "标题"},
				"body":  {Type: schema.String, Required: false, Desc: "描述"},
				"base":  {Type: schema.String, Required: false, Desc: "目标基线分支"},
				"head":  {Type: schema.String, Required: false, Desc: "源分支"},
			},
			RiskLevel: RiskLevelHigh,
		},
		{
			Name:        ToolRemoteRepoDisconnect,
			Description: "断开当前会话的远程仓库上下文，可选清理本地克隆目录。",
			Params: map[string]*schema.ParameterInfo{
				"cleanup_local": {Type: schema.Boolean, Required: false, Desc: "是否删除本地副本"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolGitDiff,
			Description: "生成与 HEAD 的统一差异",
			Params: map[string]*schema.ParameterInfo{
				"path": {Type: schema.String, Required: true, Desc: "文件路径"},
			},
			RiskLevel: RiskLevelLow,
		},
		{
			Name:        ToolGitLog,
			Description: "查看提交历史（支持 oneline/graph/all、限定条数、指定路径）",
			Params: map[string]*schema.ParameterInfo{
				"limit":   {Type: schema.Integer, Required: false, Desc: "最多返回条数（默认 20）"},
				"oneline": {Type: schema.Boolean, Required: false, Desc: "是否使用单行格式（默认 true）"},
				"graph":   {Type: schema.Boolean, Required: false, Desc: "是否显示图形化分支线（默认 false）"},
				"all":     {Type: schema.Boolean, Required: false, Desc: "是否包含所有引用（默认 false）"},
				"path":    {Type: schema.String, Required: false, Desc: "可选：仅查看影响该路径的提交"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{Description: "查看最近 20 条提交（单行）", Input: map[string]any{"limit": 20, "oneline": true}},
				{Description: "查看全引用提交图（前 50 条）", Input: map[string]any{"limit": 50, "oneline": true, "graph": true, "all": true}},
				{Description: "查看某文件的提交历史", Input: map[string]any{"limit": 30, "path": "internal/tools/manager_execute.go"}},
			},
		},
		{
			Name:        ToolGitShow,
			Description: "查看指定提交的详情（默认 HEAD），可选限定路径",
			Params: map[string]*schema.ParameterInfo{
				"revision": {Type: schema.String, Required: false, Desc: "提交引用（默认 HEAD，如 HEAD~1、<hash>）"},
				"path":     {Type: schema.String, Required: false, Desc: "可选：仅显示该路径在该提交中的变化"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{Description: "查看 HEAD 提交详情", Input: map[string]any{}},
				{Description: "查看指定提交详情", Input: map[string]any{"revision": "HEAD~1"}},
				{Description: "查看提交中某文件变化", Input: map[string]any{"revision": "abc1234", "path": "internal/tools/manager_types.go"}},
			},
		},
		{
			Name:        ToolGitStash,
			Description: "Git stash 操作（save/list/pop/apply/drop）",
			Params: map[string]*schema.ParameterInfo{
				"action":            {Type: schema.String, Required: true, Desc: "动作: save, list, pop, apply, drop"},
				"message":           {Type: schema.String, Required: false, Desc: "save 时可选：stash message"},
				"index":             {Type: schema.Integer, Required: false, Desc: "pop/apply/drop 时可选：stash 序号（默认 0）"},
				"include_untracked": {Type: schema.Boolean, Required: false, Desc: "save 时可选：是否包含未跟踪文件（默认 false）"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{Description: "列出 stash", Input: map[string]any{"action": "list"}},
				{Description: "保存 stash（含未跟踪文件）", Input: map[string]any{"action": "save", "message": "WIP: refactor tools", "include_untracked": true}},
				{Description: "应用 stash@{0}", Input: map[string]any{"action": "apply", "index": 0}},
			},
		},
		{
			Name:        ToolGitReset,
			Description: "Git reset（soft/mixed/hard）",
			Params: map[string]*schema.ParameterInfo{
				"mode":   {Type: schema.String, Required: false, Desc: "模式: soft, mixed, hard（默认 mixed）"},
				"target": {Type: schema.String, Required: true, Desc: "目标提交/引用（如 HEAD~1, <hash>）"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{Description: "mixed 回退到上一个提交", Input: map[string]any{"mode": "mixed", "target": "HEAD~1"}},
				{Description: "hard 回退到指定 hash", Input: map[string]any{"mode": "hard", "target": "abc1234"}},
			},
		},
		{
			Name:        ToolGitRevert,
			Description: "Git revert 指定提交（生成反向提交）",
			Params: map[string]*schema.ParameterInfo{
				"commit":   {Type: schema.String, Required: true, Desc: "要 revert 的提交 hash"},
				"no_edit":  {Type: schema.Boolean, Required: false, Desc: "是否不打开编辑器（默认 true）"},
				"mainline": {Type: schema.Integer, Required: false, Desc: "可选：revert merge commit 时指定主线序号（如 1）"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{Description: "revert 一个提交（不打开编辑器）", Input: map[string]any{"commit": "abc1234", "no_edit": true}},
				{Description: "revert 一个 merge 提交（mainline=1）", Input: map[string]any{"commit": "abc1234", "mainline": 1}},
			},
		},
		{
			Name:        ToolGitMerge,
			Description: "Git merge 指定分支到当前分支",
			Params: map[string]*schema.ParameterInfo{
				"branch":  {Type: schema.String, Required: true, Desc: "要合并的分支名"},
				"no_edit": {Type: schema.Boolean, Required: false, Desc: "是否不打开编辑器（默认 true）"},
				"no_ff":   {Type: schema.Boolean, Required: false, Desc: "是否强制生成 merge commit（--no-ff，默认 false）"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{Description: "合并 develop 到当前分支", Input: map[string]any{"branch": "develop", "no_edit": true}},
				{Description: "强制 no-ff 合并", Input: map[string]any{"branch": "feature/x", "no_ff": true}},
			},
		},
		{
			Name:        ToolGitRebase,
			Description: "Git rebase（start/continue/abort/skip）",
			Params: map[string]*schema.ParameterInfo{
				"action":   {Type: schema.String, Required: false, Desc: "动作: start(默认), continue, abort, skip"},
				"upstream": {Type: schema.String, Required: false, Desc: "start 时必填：上游分支/引用（如 origin/main）"},
				"onto":     {Type: schema.String, Required: false, Desc: "start 时可选：--onto 目标"},
				"branch":   {Type: schema.String, Required: false, Desc: "start 时可选：要 rebase 的分支（默认当前分支）"},
			},
			RiskLevel: RiskLevelHigh,
			Examples: []ToolExample{
				{Description: "将当前分支 rebase 到 main", Input: map[string]any{"action": "start", "upstream": "main"}},
				{Description: "继续 rebase", Input: map[string]any{"action": "continue"}},
				{Description: "终止 rebase", Input: map[string]any{"action": "abort"}},
			},
		},
		{
			Name:        ToolProjectStructure,
			Description: "获取项目目录结构视图，帮助了解工程文件组织",
			Params: map[string]*schema.ParameterInfo{
				"path": {Type: schema.String, Required: false, Desc: "根目录路径 (默认为当前目录)"},
			},
			RiskLevel: RiskLevelLow,
		},
		{
			Name:        ToolToolSearch,
			Description: "搜索可用工具，支持按名称、关键词、分类、风险等级查找",
			Params: map[string]*schema.ParameterInfo{
				"query":      {Type: schema.String, Required: false, Desc: "搜索查询（工具名称或关键词）"},
				"category":   {Type: schema.String, Required: false, Desc: "按分类搜索 (如: file, git, shell, search)"},
				"risk_level": {Type: schema.String, Required: false, Desc: "按风险等级搜索 (low/medium/high)"},
				"limit":      {Type: schema.Integer, Required: false, Desc: "最大返回数量"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{
					Description: "搜索文件相关工具",
					Input:       map[string]any{"category": "file"},
				},
				{
					Description: "搜索包含 read 关键词的工具",
					Input:       map[string]any{"query": "read"},
				},
				{
					Description: "列出所有安全的只读工具",
					Input:       map[string]any{"risk_level": "low"},
				},
			},
		},
		{
			Name:        ToolSkill,
			Description: "启用一个技能以获得专门能力与领域知识。\n\n<skills_instructions>\n- 当用户请求的任务与某个 skill 的 description 高度相关时，可调用本工具启用该 skill\n- 只能使用 <available_skills> 中列出的 skill 名称\n- 如果不确定有哪些 skill，可调用 skills_list 查询（仅用于排障/确认）\n</skills_instructions>",
			Params: map[string]*schema.ParameterInfo{
				"command": {
					Type:     schema.String,
					Required: true,
					Desc:     "要调用的技能名称（例如：pdf、pptx、docx、code-review）",
				},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{
					Description: "调用 PDF 处理技能",
					Input:       map[string]any{"command": "pdf"},
				},
				{
					Description: "调用代码审查技能",
					Input:       map[string]any{"command": "code-review"},
				},
			},
		},
		{
			Name:        ToolEnterPlanMode,
			Description: "进入计划模式。在计划模式下，所有写操作和危险操作将被拒绝，你只能读取文件、搜索和规划。当你的计划准备好后，使用 exit_plan_mode 退出。",
			Params: map[string]*schema.ParameterInfo{
				"reason": {Type: schema.String, Required: false, Desc: "进入计划模式的原因"},
			},
			RiskLevel:       RiskLevelLow,
			ConcurrencySafe: true,
		},
		{
			Name:        ToolExitPlanMode,
			Description: "退出计划模式，恢复到之前的执行模式。提供 plan_summary 以总结你在计划模式中制定的计划。",
			Params: map[string]*schema.ParameterInfo{
				"plan_summary": {Type: schema.String, Required: false, Desc: "计划摘要"},
			},
			RiskLevel:       RiskLevelLow,
			ConcurrencySafe: true,
		},
		{
			Name:        ToolAgent,
			Description: "委派任务给子代理执行。子代理可以在隔离的上下文中独立运行，支持同步和异步两种模式。\n\n可用子代理类型:\n- explore: 只读探索代理，用于搜索和阅读代码\n- general-purpose: 通用代理，拥有完整工具集\n- plan: 规划专用代理\n- verification: 验证专用代理",
			Params: map[string]*schema.ParameterInfo{
				"prompt":            {Type: schema.String, Required: true, Desc: "子代理任务描述"},
				"subagent_type":     {Type: schema.String, Required: false, Desc: "子代理类型: explore, general-purpose, plan, verification (默认 general-purpose)"},
				"run_in_background": {Type: schema.Boolean, Required: false, Desc: "是否异步执行 (默认 false)"},
				"description":       {Type: schema.String, Required: false, Desc: "任务简短描述 (用于显示)"},
				"model":             {Type: schema.String, Required: false, Desc: "可选：指定模型"},
			},
			RiskLevel: RiskLevelLow,
			Examples: []ToolExample{
				{Description: "同步探索代码库", Input: map[string]any{"prompt": "查找所有处理用户认证的文件", "subagent_type": "explore"}},
				{Description: "异步执行通用任务", Input: map[string]any{"prompt": "为 auth 模块编写单元测试", "subagent_type": "general-purpose", "run_in_background": true}},
			},
		},
		{
			Name:        ToolSuggestMemory,
			Description: "将长期有效的信息写入独立记忆体系。优先写项目记忆；只有跨项目适用的用户偏好才写全局记忆。避免写入临时任务噪声、一次性状态或短期上下文。",
			Params: map[string]*schema.ParameterInfo{
				"file":    {Type: schema.String, Required: false, Desc: "可选：遗留兼容字段。留空时自动路由到全局或项目记忆文件"},
				"type":    {Type: schema.String, Required: false, Desc: "记忆类型：global 或 project；若省略则默认 project"},
				"content": {Type: schema.String, Required: true, Desc: "要沉淀的长期记忆内容"},
				"section": {Type: schema.String, Required: false, Desc: "可选：逻辑分组标题，如“用户偏好”“任务结论”"},
			},
			RiskLevel:       RiskLevelMedium,
			ConcurrencySafe: true,
			Examples: []ToolExample{
				{Description: "沉淀项目约定", Input: map[string]any{"type": "project", "content": "变更前先查看相关测试与现有配置约束", "section": "项目约定"}},
				{Description: "沉淀全局用户偏好", Input: map[string]any{"type": "global", "content": "默认使用中文回答，优先给出最小改动方案", "section": "用户偏好"}},
			},
		},
		{
			Name:        ToolWebSearch,
			Description: "Search the web using DuckDuckGo. Returns search results with titles, URLs, and snippets.",
			Params: map[string]*schema.ParameterInfo{
				"query":       {Type: schema.String, Required: true, Desc: "Search query"},
				"max_results": {Type: schema.Integer, Required: false, Desc: "Maximum number of results (default 5)"},
			},
			RiskLevel: RiskLevelLow,
		},
		{
			Name:        ToolWebFetch,
			Description: "Fetch public content from a URL using the built-in HTTP client. Supports lightweight text/markdown extraction but does not execute page JavaScript, and cross-host redirects must be fetched explicitly.",
			Params: map[string]*schema.ParameterInfo{
				"url":    {Type: schema.String, Required: true, Desc: "URL to fetch"},
				"format": {Type: schema.String, Required: false, Desc: "Output format: text (default) or markdown with lightweight HTML extraction"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolEnterWorktree,
			Description: "Create an isolated git worktree for working on changes without affecting the main working directory.",
			Params: map[string]*schema.ParameterInfo{
				"name": {Type: schema.String, Required: false, Desc: "Worktree name (auto-generated if empty)"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolExitWorktree,
			Description: "Remove or prune a git worktree.",
			Params: map[string]*schema.ParameterInfo{
				"path":   {Type: schema.String, Required: true, Desc: "Worktree path to remove"},
				"remove": {Type: schema.Boolean, Required: false, Desc: "Whether to remove the worktree directory (default true)"},
			},
			RiskLevel: RiskLevelHigh,
		},
		{
			Name:        ToolNotebookEdit,
			Description: "Edit a Jupyter notebook (.ipynb) file. Supports replace, insert, and delete cell operations.",
			Params: map[string]*schema.ParameterInfo{
				"path":         {Type: schema.String, Required: true, Desc: "Path to .ipynb file"},
				"cell_id":      {Type: schema.String, Required: false, Desc: "Cell ID to edit (required for replace/delete)"},
				"cell_type":    {Type: schema.String, Required: false, Desc: "Cell type: code or markdown (for insert)"},
				"source":       {Type: schema.String, Required: false, Desc: "New cell content"},
				"edit_mode":    {Type: schema.String, Required: false, Desc: "Edit mode: replace (default), insert, delete"},
				"insert_after": {Type: schema.String, Required: false, Desc: "Cell ID after which to insert (for insert mode)"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolMCPListResources,
			Description: "列出 MCP 服务器暴露的资源（模板、静态数据等）。可按服务器名筛选。",
			Params: map[string]*schema.ParameterInfo{
				"server": {Type: schema.String, Required: false, Desc: "可选：指定 MCP 服务器名称"},
			},
			RiskLevel:       RiskLevelLow,
			ConcurrencySafe: true,
		},
		{
			Name:        ToolMCPReadResource,
			Description: "读取指定 MCP 服务器的指定资源 URI 内容。",
			Params: map[string]*schema.ParameterInfo{
				"server": {Type: schema.String, Required: true, Desc: "MCP 服务器名称"},
				"uri":    {Type: schema.String, Required: true, Desc: "资源 URI"},
			},
			RiskLevel: RiskLevelLow,
		},
		{
			Name:        ToolMCPListPrompts,
			Description: "列出所有 MCP 服务器提供的 prompt 模板。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelLow,
		},
		{
			Name:        ToolMCPGetPrompt,
			Description: "获取指定 MCP 服务器的指定 prompt 模板内容。",
			Params: map[string]*schema.ParameterInfo{
				"server":    {Type: schema.String, Required: true, Desc: "MCP 服务器名称"},
				"name":      {Type: schema.String, Required: true, Desc: "prompt 名称"},
				"arguments": {Type: schema.String, Required: false, Desc: "可选的 prompt 参数（JSON 格式）"},
			},
			RiskLevel: RiskLevelLow,
		},
		{
			Name:        ToolPowerShell,
			Description: "执行 PowerShell 命令（Windows 上使用 powershell.exe，跨平台使用 pwsh）。",
			Params: map[string]*schema.ParameterInfo{
				"command": {Type: schema.String, Required: true, Desc: "要执行的 PowerShell 命令"},
			},
			RiskLevel: RiskLevelHigh,
		},
		{
			Name:        ToolStructuredOutput,
			Description: "生成符合 JSON Schema 的结构化输出。接收 schema 和 data 参数，验证 data 符合 schema 后返回。",
			Params: map[string]*schema.ParameterInfo{
				"schema": {Type: schema.String, Required: true, Desc: "JSON Schema 字符串"},
				"data":   {Type: schema.String, Required: true, Desc: "要验证的 JSON 数据字符串"},
			},
			RiskLevel:       RiskLevelLow,
			ConcurrencySafe: true,
		},
		{
			Name:        ToolSnip,
			Description: "标记上下文中可裁剪的消息。Agent 主动标记不再需要的上下文内容以释放 token 空间。",
			Params: map[string]*schema.ParameterInfo{
				"message_id": {Type: schema.String, Required: true, Desc: "要裁剪的消息 ID"},
				"reason":     {Type: schema.String, Required: false, Desc: "裁剪原因"},
			},
			RiskLevel:       RiskLevelLow,
			ConcurrencySafe: true,
		},
		{
			Name:        ToolTeamCreate,
			Description: "创建命名 team，配置多个 agent 角色并行协作执行任务。",
			Params: map[string]*schema.ParameterInfo{
				"name":   {Type: schema.String, Required: true, Desc: "Team 名称"},
				"agents": {Type: schema.Array, Required: true, Desc: "Agent 配置列表（每个元素包含 role, prompt, subagent_type）"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolTeamDelete,
			Description: "停止并清理指定 team。",
			Params: map[string]*schema.ParameterInfo{
				"name": {Type: schema.String, Required: true, Desc: "Team 名称"},
			},
			RiskLevel: RiskLevelMedium,
		},
		{
			Name:        ToolTeamSendMsg,
			Description: "在 team 内的 agent 之间发送消息。",
			Params: map[string]*schema.ParameterInfo{
				"team":       {Type: schema.String, Required: true, Desc: "Team 名称"},
				"from_agent": {Type: schema.String, Required: true, Desc: "发送方 agent 角色"},
				"to_agent":   {Type: schema.String, Required: true, Desc: "接收方 agent 角色"},
				"message":    {Type: schema.String, Required: true, Desc: "消息内容"},
			},
			RiskLevel: RiskLevelLow,
		},
		{
			Name:        ToolPatch,
			Description: "[实验性] 结构化 patch 工具。支持两种格式：edits（结构化编辑列表）和 unified（unified diff）。支持 dry_run 预览和 apply 实际写入。",
			Params: map[string]*schema.ParameterInfo{
				"mode":   {Type: schema.String, Required: false, Desc: "执行模式: apply（默认，实际写入）, dry_run（仅预览不落盘）"},
				"format": {Type: schema.String, Required: false, Desc: "patch 格式: edits（默认，结构化编辑）, unified（unified diff 文本）"},
				"patches": {Type: schema.Array, Required: false, Desc: "edits 格式的 patch 列表，每项含 path 和 edits 数组"},
				"diff":   {Type: schema.String, Required: false, Desc: "unified 格式的 diff 文本"},
			},
			RiskLevel: RiskLevelMedium,
			Examples: []ToolExample{
				{
					Description: "结构化编辑（dry_run 预览）",
					Input: map[string]any{
						"mode":   "dry_run",
						"format": "edits",
						"patches": []map[string]any{
							{"path": "main.go", "edits": []map[string]any{{"find": "oldFunc", "replace": "newFunc"}}},
						},
					},
				},
				{
					Description: "应用 unified diff",
					Input: map[string]any{
						"format": "unified",
						"diff":   "--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,3 @@\n package main\n-func old()\n+func new()\n func main() {}",
					},
				},
			},
		},
	}
}

// GetToolDefinition 根据工具名称获取工具定义
func GetToolDefinition(name string) (ToolDefinition, bool) {
	for _, def := range GetAllToolDefinitions() {
		if def.Name == name {
			return def, true
		}
	}
	return ToolDefinition{}, false
}

// GetToolRiskLevel 获取工具的风险等级
func GetToolRiskLevel(name string) ToolRiskLevel {
	if def, ok := GetToolDefinition(name); ok {
		return def.RiskLevel
	}
	return RiskLevelHigh // 默认为高风险
}

// GetToolsByCategory 按分类获取工具定义列表
func GetToolsByCategory(category string) []ToolDefinition {
	var result []ToolDefinition
	for _, def := range GetAllToolDefinitions() {
		if def.Category == category {
			result = append(result, def)
		}
	}
	return result
}

// IsReadOnlyTool 检查工具是否为只读操作
func IsReadOnlyTool(name string) bool {
	if def, ok := GetToolDefinition(name); ok {
		return def.ReadOnly
	}
	return false
}

// NeedsSandbox 检查工具是否需要 sandbox runner
func NeedsSandbox(name string) bool {
	if def, ok := GetToolDefinition(name); ok {
		return def.NeedsSandboxRunner
	}
	return false
}

// GetOfficeTools 返回所有 Office 文档相关工具定义
func GetOfficeTools() []ToolDefinition {
	return GetToolsByCategory("Office 文档")
}
