# EOS

[中文](./README.md) | [English](./README.en.md)

EOS is a Go-based terminal AI coding assistant built on CloudWeGo Eino. It provides an interactive TUI, tool calling, safety controls, and workspace-aware context management.

- Repository: https://github.com/dreamSailing/eos
- Issues: https://github.com/dreamSailing/eos/issues
- Releases: https://github.com/dreamSailing/eos/releases

## Why EOS Instead of Claude Code?

| Pain Point | EOS |
|---|---|
| Requires a VPN in many regions | Works out of the box |
| Claude-only model support | Supports mainstream models |
| Depends on Node.js for installation | Built in Go with zero Node dependency |
| Complex setup | Start after filling in your key |
| Claude Code lacks MCP + vision support | Already addressed |
| Claude Code has no web search | Built in |

## Core Capabilities

- Interactive TUI: AI/Bash dual mode, panel system, streaming output, and Markdown rendering
- Two execution modes: `plan` / `auto` (switchable in the interface)
- Multi-agent collaboration: planner, developer, tester, reviewer, and other specialized agents
- Tooling system: file read/write/edit, search, Git, Shell, background tasks, MCP calls, and more
- Office document support: built-in `DOCX/XLSX/PDF` reading, generation, and conversion with high-fidelity conversion preferred
- Safety controls: tiered confirmation for high-risk actions with session-level authorization support
- Context indexing: code indexing, file watching, context compression, and session persistence
- Optional LSP support: `without_lsp`, default LSP, and embedded `with_gopls` builds

## Requirements

- Go 1.25+
- An accessible OpenAI-compatible endpoint (`EOS_API_BASE`, `EOS_API_KEY`, `EOS_MODEL`)

## Quick Start

### 1) Build

```bash
git clone https://github.com/dreamSailing/eos.git
cd eos
go mod tidy
go build -o eos
```

Windows:

```powershell
.\eos.exe
```

macOS / Linux:

```bash
./eos
```

### 2) Configure a Model

Option A: environment variables

```bash
export EOS_API_BASE="https://api.openai.com/v1"
export EOS_API_KEY="sk-..."
export EOS_MODEL="gpt-4o-mini"
```

Option B: `~/.eos.json`

```json
{
  "models": [
    {
      "name": "default",
      "api_base": "https://api.openai.com/v1",
      "api_key": "sk-...",
      "model": "gpt-4o-mini"
    }
  ],
  "active_model": "default"
}
```

## Build Variants (LSP)

- Minimal build (without LSP)  
  `go build -tags without_lsp -o eos`
- Default build (with LSP framework enabled)  
  `go build -o eos`
- Go-enhanced build (with embedded gopls)  
  `go build -tags with_gopls -o eos`

The repository includes gopls embedding scripts: `scripts/embed_gopls.sh` and `scripts/embed_gopls.bat`.

## Common Interactions

### Keyboard Shortcuts

- `F2`: switch between AI and Bash mode
- `Alt+M`: switch execution mode (`plan ↔ auto`)
- `Alt+V`: paste an image from the clipboard
- `Alt+H`: expand or collapse thinking content
- `?`: open the help panel
- `Ctrl+O`: switch the real-time info panel style

### Common Slash Commands

- `/help` `/clear` `/exit`
- `/history` (or `/versions`)
- `/models` `/mcp` `/ctx` `/cost` `/tasks`
- `/workspace list|add|remove|use <path>`
- `/settings` `/lsp` `/rules` `/lang` `/compact`
- `/init`: initialize `EOS.md` in the current workspace

## Document Support

- Read: the built-in `read` tool can now open `DOCX`, `XLSX`, and `PDF`
- Generate: available through the `document_generate` tool and `eos doc generate`
- Convert: available through the `document_convert` tool and `eos doc convert`
- High fidelity: `DOCX <-> PDF` and `XLSX <-> PDF` prefer `soffice` for layout-preserving conversion; if unavailable, EOS falls back to content-level conversion with warnings
- Boundary: first-release `DOCX <-> XLSX` conversion focuses on structured content and tables rather than full layout preservation

CLI examples:

```bash
eos doc read ./report.docx
eos doc generate --format pdf --output ./out/report.pdf --title "Weekly Report" --content "Paragraph one\n\nParagraph two"
eos doc convert ./report.docx --to pdf --output ./out/report.pdf --fidelity high
```

## Service Mode API

- Public CLI API (`eos serve`): [internal/docs/serve/API.md](./internal/docs/serve/API.md)
- Minimal IDE bridge integration: first generate a bridge manifest with `eos bridge manifest --workspace "/abs/workspace"`, then see [internal/docs/serve/IDE_BRIDGE.md](./internal/docs/serve/IDE_BRIDGE.md)
- Standard MCP Server: `eos mcp serve --transport stdio --workspace "/abs/workspace"`; see [internal/docs/mcp/SERVER.md](./internal/docs/mcp/SERVER.md)

### MCP Server Examples

stdio:

```bash
eos mcp serve --transport stdio --workspace "/abs/workspace"
```

SSE:

```bash
eos mcp serve --transport sse --listen 127.0.0.1:8765 --workspace "/abs/workspace"
```

### Browser Automation

EOS does not ship an internal browser driver. The recommended production path is to connect a mature Playwright MCP server. Once enabled, the agent can click, type, select, wait for page changes, and capture screenshots in a real browser session.

Minimum working configuration:

```json
[
  {
    "name": "playwright",
    "type": "stdio",
    "command": "npx",
    "args": ["-y", "@playwright/mcp@latest"],
    "envs": {},
    "enabled": true
  }
]
```

How to enable:

- Press `B` in the `/mcp` panel to insert the Playwright preset
- Or edit the `mcp` section in `~/.eos.json` manually

Usage boundaries:

- `web_fetch` is for read-only page fetching
- Browser MCP is for real page interaction and behavior verification
- Use `browser_status`, `/status`, or `/doctor` for diagnostics

## Project Structure (Short Version)

```text
internal/
  cli/       Cobra entrypoints
  ui/        TUI interaction and panels
  bridge/    UI and runtime bridge
  runtime/   Eino orchestration and tool scheduling
  tools/     Tool definitions and execution
  context/   Code indexing and watching
  session/   Session context management
  lsp/       LSP management and embedding support
```

## Development And Testing

```bash
go test ./...
go build ./...
```

## Open-Source Release Notes

- The runtime creates `.eos/` data in the working directory, including sessions, checkpoints, and version snapshots
- Ensure `.eos/`, `.eos.json`, `.env`, logs, and local configuration files are excluded from version control
- Run a sensitive data check before publishing changes, including API keys, private keys, certificates, and absolute paths

## License

This project is released under the EOS Non-Commercial License v1.1. See [LICENSE](./LICENSE):

- Free for personal and non-commercial use, including installer usage
- Non-commercial compilation, modification, and redistribution are allowed
- Derivative works must remain open source under the same license
- Any commercial use is prohibited, including internal enterprise production use, paid services, SaaS, and commercial redistribution
- Commercial use requires separate written authorization from the copyright holder

## Contact

- Issues: https://github.com/dreamSailing/eos/issues
- Commercial licensing: smart-os@qq.com
