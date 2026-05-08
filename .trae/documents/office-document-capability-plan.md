# Office 文档能力规划

## Summary

目标是把 `Word/DOCX`、`Excel/XLSX`、`PDF` 的读取、生成和转换做成软件内置能力，而不是依赖临时 skill、外部 MCP 或“用户自己写脚本”来完成。

本次规划按“产品内置能力”交付，首要落点是：

- Agent 可直接调用的内置工具能力。
- 软件自带的 CLI 入口，便于用户显式执行文档读取、生成、转换。
- 文档读取与生成为纯 Go 内置实现。
- 复杂格式互转采用“软件内统一封装 + 高保真引擎优先 + 内容级 fallback”的混合方案。

## Current State Analysis

### 已确认的现状

1. 当前仓库只有 `PDF` 的“读取文本”能力，而且还不是完全内置：
   - `internal/tools/fs_tools_read.go` 只对 `.pdf` 做了特殊分支。
   - `internal/tools/reader_pdf.go` 里优先调用 `pdftotext`，失败后再尝试 `python3 + pdfplumber/PyPDF2`。
   - 这说明现有 PDF 读取依赖外部环境，不符合“软件内置能力”的要求。

2. `DOCX/XLSX` 当前没有读写或转换链路：
   - `internal/tools/fs_tools_read.go` 没有 `.docx` / `.xlsx` 分支。
   - 全仓库没有 `excelize`、`gooxml`、`libreoffice`、`soffice` 等文档处理依赖或调用链。
   - `go.mod` 当前也没有 Office 文档处理相关依赖。

3. 文件系统层当前会把 Office/PDF 视为二进制文件，无法自然落入文本读取路径：
   - `internal/pkg/utils/file.go` 已明确把 `.pdf` 视为二进制。
   - `internal/tools/fileops/fileops.go` 的 `IsTextFile()` 没有对 `.docx` / `.xlsx` / `.pdf` 做文本读取兼容。

4. 工具框架本身已经具备很清晰的扩展点：
   - `internal/tools/definitions.go` 维护工具名、参数、描述和风险等级。
   - `internal/tools/manager_types.go` 维护结构化 handler 注册。
   - `internal/tools/fs_tools_read.go` 已经证明“按扩展名接入特殊读取器”的模式可行。

5. CLI 入口也有现成扩展位：
   - `internal/cli/root.go` 通过 `rootCmd.AddCommand(...)` 注册子命令。
   - 现有 CLI 使用 `cobra`，新增 `doc` 或 `office` 子命令成本低。

### 结论

基于当前代码库，可以明确回答用户原问题：

- 现在“生成 Word/Excel/PDF 的能力”还没有。
- 现在“Word/Excel/PDF 互相转换的能力”还没有。
- 现在“读取能力”只有 PDF 文本读取算半具备，且不属于真正的全内置实现。
- 因此，如果目标是“作为软件内置能力做到产品里”，当前确实还未完成，需要新增一整套文档能力层。

## Assumptions & Decisions

由于你没有继续选择转换保真度和入口范围，本计划锁定以下默认决策，执行阶段直接按此实现：

- 能力范围：本次直接规划 `DOCX/XLSX/PDF` 三类格式的读取、生成、转换能力，不做“只读不写”的半成品方案。
- 暴露范围：首期同时提供内置工具能力和 CLI 入口；不在本轮强行做独立 UI 面板。
- 架构策略：读取/生成优先纯 Go；复杂互转走“软件统一封装的转换引擎”。
- 高保真策略：`DOCX <-> PDF`、`XLSX <-> PDF` 优先调用内置封装的 `LibreOffice/soffice` 转换适配器；若环境不可用，则回退到内容级转换，并在结果里显式标注降级。
- 语义边界：`DOCX <-> XLSX` 不承诺任意复杂文档的样式级无损互转；首版限定为“表格/结构化内容优先”的内容级转换。
- 依赖选择：
  - `XLSX` 读写使用 `github.com/xuri/excelize/v2`
  - `DOCX` 读写使用 `baliance.com/gooxml/document`
  - `PDF` 生成使用 `github.com/go-pdf/fpdf`
  - `PDF` 文本提取使用 `github.com/ledongthuc/pdf`
  - 高保真格式互转通过 `soffice` 命令适配层完成

## Proposed Changes

### 1. 新增统一的文档能力内核

新增一个专用内部包，承载三类格式的统一模型、解析、生成和转换编排，不把复杂逻辑直接堆在 `internal/tools/`。

涉及文件：

- 新增 `internal/document/model.go`
- 新增 `internal/document/docx.go`
- 新增 `internal/document/xlsx.go`
- 新增 `internal/document/pdf.go`
- 新增 `internal/document/convert.go`
- 修改 `go.mod`

实施内容：

1. 在 `internal/document/model.go` 定义统一中间模型：
   - `DocumentModel`：段落、标题、表格、图片元数据、分页提示。
   - `WorkbookModel`：工作簿、工作表、单元格、合并区、列宽、基础样式。
   - `ConversionResult`：输出路径、是否降级、警告列表、元数据。
2. 在 `docx.go` 实现：
   - `ReadDOCX(path) (DocumentModel, error)`
   - `WriteDOCX(path, model) error`
3. 在 `xlsx.go` 实现：
   - `ReadXLSX(path) (WorkbookModel, error)`
   - `WriteXLSX(path, model) error`
4. 在 `pdf.go` 实现：
   - `ReadPDF(path) (DocumentModel, error)`，替换现有“依赖外部命令”的方案
   - `WritePDF(path, model) error`
5. 在 `convert.go` 实现统一编排：
   - `Convert(path, targetFormat, options) (ConversionResult, error)`
   - 优先判断是否可使用 `soffice`
   - `DOCX <-> PDF`、`XLSX <-> PDF` 使用高保真引擎优先
   - 引擎不可用时回退到中间模型转换

原因：

- 现有 `internal/tools/reader_pdf.go` 只是孤立函数，不适合扩展成完整文档平台。
- 独立包能让工具层、CLI 层和未来 UI 层共用同一套逻辑。

### 2. 扩展现有 `read` 工具，让读取能力真正内置

把 `read` 从“只能特殊处理 PDF”扩展为支持 `DOCX/XLSX/PDF` 的统一只读入口，保证 Agent 在不新增额外命令的情况下就能读取三类文档。

涉及文件：

- 修改 `internal/tools/fs_tools_read.go`
- 删除或重构 `internal/tools/reader_pdf.go`
- 修改 `internal/pkg/utils/file.go`
- 视实现需要，修改 `internal/tools/fileops/fileops.go`
- 修改 `internal/i18n/zh.go`
- 修改 `internal/i18n/en.go`

实施内容：

1. 在 `fs_tools_read.go` 中新增：
   - `.docx` -> 调用 `internal/document.ReadDOCX`
   - `.xlsx` -> 调用 `internal/document.ReadXLSX`
   - `.pdf` -> 调用新的纯 Go `internal/document.ReadPDF`
2. 统一读取结果格式：
   - 仍返回 `tool_result`
   - `Data` 中增加 `format`、`structured`、`warnings`
   - `content` 保留适合模型阅读的摘要文本表示
3. 对 `DOCX/XLSX/PDF` 的失败场景增加更明确错误信息：
   - 文件损坏
   - 加密/受保护
   - 解析降级
4. 更新二进制文件判定：
   - 保持 Office/PDF 不走原始文本读取
   - 但明确列为“支持的结构化二进制文档类型”，而不是统一报“不支持”

原因：

- 读取是最基础能力，必须优先纳入现有 `read` 工具，避免模型需要记忆额外工具名才能访问文档内容。

### 3. 新增生成与转换工具，作为软件内置文档能力入口

在工具层新增两个专用能力：`document_generate` 和 `document_convert`。

涉及文件：

- 修改 `internal/tools/definitions.go`
- 修改 `internal/tools/manager_types.go`
- 新增 `internal/tools/document_tools.go`
- 新增 `internal/tools/document_tools_test.go`

实施内容：

1. 在 `definitions.go` 新增工具定义：
   - `document_generate`
   - `document_convert`
2. 参数设计锁定如下：
   - `document_generate`:
     - `format`: `docx|xlsx|pdf`
     - `path`
     - `title`
     - `content`
     - `structured_content`（表格/工作表/段落 JSON）
   - `document_convert`:
     - `source_path`
     - `target_format`
     - `destination_path`
     - `fidelity`: `high|content`
3. 在 `manager_types.go` 注册对应 handler。
4. 在 `document_tools.go` 中实现：
   - 输入校验
   - 路径解析
   - 调用 `internal/document` 内核
   - 将降级、转换警告和输出路径写回 `ToolResult`
5. 风险等级设为：
   - `document_generate`: `medium`
   - `document_convert`: `medium`

原因：

- 生成和转换不适合硬塞进现有 `fs` 或 `read` 工具；单独命名更清晰，也便于 Agent 正确理解使用场景。

### 4. 补齐 CLI 入口，让能力真正属于“软件内置”

除了 Agent 工具，还要提供直接可见的产品命令行入口。

涉及文件：

- 新增 `internal/cli/document.go`
- 修改 `internal/cli/root.go`

实施内容：

1. 新增 `eos doc` 子命令，包含：
   - `eos doc read <path>`
   - `eos doc generate`
   - `eos doc convert <source>`
2. CLI 参数与工具层保持同构，避免两套协议：
   - `--format`
   - `--output`
   - `--fidelity`
   - `--json`
3. CLI 内部直接复用 `internal/document` 或工具层 handler，不重复实现业务逻辑。
4. 对转换降级场景输出明确提示：
   - 是否使用了高保真引擎
   - 是否回退为内容级转换
   - 哪些元素可能丢失

原因：

- 只做 Agent 内置工具还不够“产品化”；CLI 是最轻量也最直接的软件能力落点。

### 5. 文档、帮助文本与能力声明更新

让模型提示、用户文档和帮助信息都知道这套能力已经内置。

涉及文件：

- 修改 `README.md`
- 修改 `README.en.md`
- 视需要修改 `internal/runtime/prompt_capabilities.go`
- 视需要修改 `internal/ui/views/help/help.go`

实施内容：

1. 在 README 中新增文档能力说明：
   - 支持的格式
   - 读取/生成/转换边界
   - 高保真与内容级转换差异
2. 如果运行时能力提示没有自动覆盖到新工具，则补充一段显式能力说明。
3. 如果帮助页当前只展示通用能力，则补充 `eos doc` 相关命令。

原因：

- 没有对外文档，内置能力会继续处于“代码存在但用户不知道”的状态。

### 6. 增加测试基线和样例文件

文档读写和转换很容易出现回归，必须从第一版开始有最小但有效的测试基线。

涉及文件：

- 新增 `internal/document/docx_test.go`
- 新增 `internal/document/xlsx_test.go`
- 新增 `internal/document/pdf_test.go`
- 新增 `internal/tools/document_tools_test.go`
- 新增 `internal/document/testdata/`

实施内容：

1. 为读取能力增加 fixture 测试：
   - 简单文本文档
   - 包含表格的 DOCX
   - 多 sheet 的 XLSX
   - 基础文本型 PDF
2. 为生成能力增加 golden/结构断言：
   - 生成后再次读取，断言核心内容一致
3. 为转换能力增加最小矩阵测试：
   - `docx -> pdf`
   - `xlsx -> pdf`
   - `pdf -> docx`
   - `docx -> xlsx`（表格型样例）
4. 对 `soffice` 相关测试做条件化处理：
   - 有命令时跑高保真路径
   - 无命令时跑 fallback 路径并断言 warning

原因：

- 这类能力如果没有 fixture 和 round-trip 测试，后续改一点解析逻辑就会静默退化。

## Verification Steps

实现完成后按以下顺序验证：

1. 运行单元测试：
   - `go test ./internal/document/... ./internal/tools/... ./internal/cli/...`
2. 验证读取：
   - 用 `read` 工具读取 `.docx` / `.xlsx` / `.pdf`
   - 确认 `content`、`structured`、`warnings` 字段符合预期
3. 验证生成：
   - 使用 `document_generate` 生成三种格式
   - 再次读取生成结果，确认关键内容一致
4. 验证转换：
   - `docx -> pdf`
   - `xlsx -> pdf`
   - `pdf -> docx`
   - `docx -> xlsx`（表格样例）
5. 验证 CLI：
   - `eos doc read`
   - `eos doc generate`
   - `eos doc convert`
6. 验证降级提示：
   - 在无 `soffice` 环境下执行转换
   - 确认结果明确提示“已回退到内容级转换”

## Implementation Order

按以下顺序执行，避免中途出现“工具接口已暴露但能力还没打通”的半成品状态：

1. 先加 `internal/document` 内核与依赖。
2. 再接 `read` 的三格式内置读取。
3. 再加 `document_generate` / `document_convert` 工具。
4. 再补 CLI 入口。
5. 最后补 README、帮助文本和测试基线。
