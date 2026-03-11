package tools

import "github.com/cloudwego/eino/schema"

// ToolExample 工具使用示例
type ToolExample struct {
	Description string                 // 示例描述
	Input       map[string]interface{} // 输入参数示例
}

// ToolDefinition 工具定义，包含工具的名称、描述和参数信息
type ToolDefinition struct {
	Name        string                           // 工具名称
	Description string                           // 工具描述
	Params      map[string]*schema.ParameterInfo // 参数定义
	RiskLevel   ToolRiskLevel                    // 风险等级：low/medium/high
	Examples    []ToolExample                    // 使用示例（提升模型理解复杂参数的准确率）
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
	ToolRead             = "read"
	ToolFS               = "fs"
	ToolEdit             = "edit"
	ToolHistory          = "history"
	ToolSearch           = "search"
	ToolToolSearch       = "tool_search" // 工具搜索工具
	ToolSkill            = "skill"       // Agent Skills meta-tool
	ToolSkillsList       = "skills_list"
	ToolTimeNow          = "time_now"
	ToolUserConfirm      = "user_confirm"
	ToolUserInput        = "user_input"
	ToolUserSelect       = "user_select"
	ToolBash             = "bash"
	ToolBashSession      = "bash_session"
	ToolBGTask           = "bg_task"
	ToolPlanSteps        = "plan_steps"
	ToolTodoRead         = "todo_read"
	ToolTodoWrite        = "todo_write"
	ToolMCPStatus        = "mcp_status"
	ToolGitStatus        = "git_status"
	ToolGitAdd           = "git_add"
	ToolGitCommit        = "git_commit"
	ToolGitBranchList    = "git_branch_list"
	ToolGitCheckout      = "git_checkout"
	ToolGitInit          = "git_init"
	ToolGitPull          = "git_pull"
	ToolGitPush          = "git_push"
	ToolGitDiff          = "git_diff"
	ToolGitLog           = "git_log"
	ToolGitShow          = "git_show"
	ToolGitStash         = "git_stash"
	ToolGitReset         = "git_reset"
	ToolGitRevert        = "git_revert"
	ToolGitMerge         = "git_merge"
	ToolGitRebase        = "git_rebase"
	ToolProjectStructure = "ProjectStructure"
)

// GetAllToolDefinitions 返回所有工具的定义
func GetAllToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        ToolTimeNow,
			Description: "获取本机当前日期时间（本地时区），并返回常用格式（本地/UTC、Unix 时间戳等）。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelLow,
			Examples: []ToolExample{
				{Description: "获取本地日期时间", Input: map[string]any{}},
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
			Name:        ToolSkillsList,
			Description: "查询当前可用 skills 列表与扫描目录（按需排障用）。",
			Params:      map[string]*schema.ParameterInfo{},
			RiskLevel:   RiskLevelLow,
			Examples: []ToolExample{
				{Description: "查询 skills 列表", Input: map[string]any{}},
			},
		},
		{
			Name:        ToolRead,
			Description: "统一读取工具。注意：path 参数必须是有效的文件系统路径，不要包含 '@' 等特殊前缀。",
			Params: map[string]*schema.ParameterInfo{
				"mode": {Type: schema.String, Required: false, Desc: "读取模式: file (默认, 读取文件内容), directory (列出目录条目), exists (检查路径是否存在), resolve (解析路径并返回候选路径与状态)"},
				"path": {Type: schema.String, Required: true, Desc: "要读取的绝对或相对路径 (e.g., 'main.go', 'internal/utils')"},
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
			Name:        ToolFS,
			Description: "统一文件系统操作 (模式: write, create, mkdir, delete, move, copy, diff)。如需检查文件是否存在或读取内容，请使用 'read' 工具。",
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
			Description: "统一搜索工具 (模式: glob, regex, text, code, deps, graph)",
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
			Description: "执行 Shell 命令 (同步)",
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
			Description: "统一编辑工具。必需参数: single模式需(file, find, replace); multi模式需(file, edits); batch模式需(edits)。",
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
				"action":           {Type: schema.String, Required: true, Desc: "动作: save, list, pop, apply, drop"},
				"message":          {Type: schema.String, Required: false, Desc: "save 时可选：stash message"},
				"index":            {Type: schema.Integer, Required: false, Desc: "pop/apply/drop 时可选：stash 序号（默认 0）"},
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
