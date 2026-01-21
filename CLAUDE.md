# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ZAP is an AI-powered API debugging assistant that runs in the terminal. It combines API testing with codebase awareness - when an API returns an error, ZAP can search your code to find the cause and suggest fixes. Uses local LLMs (Ollama) or cloud providers.

## Build & Run Commands

```bash
# Build the application
go build -o zap.exe ./cmd/zap

# Run the application
./zap.exe

# Run with custom config
./zap --config path/to/config.json
```

## Architecture

### Package Structure

- **cmd/zap/** - Application entry point using Cobra CLI framework
- **pkg/core/** - Agent logic, event system, and initialization
- **pkg/core/tools/** - Agent tools (HTTP, file, search, persistence)
- **pkg/llm/** - LLM client implementations (Ollama)
- **pkg/storage/** - Request persistence (YAML save/load, environments)
- **pkg/tui/** - Minimal terminal UI using Bubble Tea

### Core Components

**Agent (pkg/core/agent.go)**: Implements ReAct (Reason+Act) loop with event system:
- `ProcessMessage(input)` - Blocking, returns final answer
- `ProcessMessageWithEvents(input, callback)` - Emits events for real-time UI updates
- Events: `thinking`, `tool_call`, `observation`, `answer`, `error`, `streaming`
- Max 5 iterations to prevent infinite loops
- Enhanced system prompt teaches:
  - Natural language to HTTP request conversion
  - Error diagnosis workflow (analyze → search → read → diagnose)
  - Common error patterns and framework hints

**Tool Interface**:
```go
type Tool interface {
    Name() string
    Description() string
    Parameters() string
    Execute(args string) (string, error)
}
```

**TUI (pkg/tui/app.go)**: Minimal Claude Code-style interface:
- `bubbles/viewport` - Scrollable log area (pgup/pgdown, mouse wheel)
- `bubbles/textinput` - Single-line input with `> ` prompt
- `bubbles/spinner` - Loading indicator
- `glamour` - Markdown rendering for responses
- Streaming display (text appears as it arrives)
- Status line showing current state (thinking/streaming/executing tool)
- Input history navigation (↑/↓ arrows)

**Styling (pkg/tui/styles.go)**: Minimal 7-color palette with log prefixes:
- `> ` user input
- `  thinking ` agent reasoning
- `  tool ` tool calls
- `  result ` observations
- `  error ` errors
- `───` conversation separator

**Keyboard Shortcuts**:
- `enter` - Send message
- `↑` / `↓` - Navigate input history
- `pgup` / `pgdown` - Scroll viewport
- `ctrl+l` - Clear screen
- `ctrl+u` - Clear input line
- `ctrl+c` / `esc` - Quit

### Configuration

On first run, creates `.zap/` folder containing:
- `config.json` - Ollama URL, model settings
- `history.jsonl` - Conversation log
- `memory.json` - Agent memory
- `requests/` - Saved API requests (YAML files)
- `environments/` - Environment configs (dev.yaml, prod.yaml, etc.)

Environment: `OLLAMA_API_KEY` loaded from `.env` file.

### Request Persistence

Requests are saved as YAML files with variable substitution:
```yaml
# .zap/requests/get-users.yaml
name: Get Users
method: GET
url: "{{BASE_URL}}/api/users"
headers:
  Authorization: "Bearer {{API_TOKEN}}"
```

Environments define variables:
```yaml
# .zap/environments/dev.yaml
BASE_URL: http://localhost:3000
API_TOKEN: dev-token-123
```

### Adding New Tools

1. Create a new file in `pkg/core/tools/`
2. Implement the `core.Tool` interface
3. Register in `pkg/tui/app.go` via `agent.RegisterTool()`

### Message Flow

```
User Input → TUI captures Enter
           → runAgentAsync() starts goroutine
           → Agent.ProcessMessageWithEvents() runs
           → Callback sends AgentEvent via program.Send()
           → TUI Update() receives agentEventMsg
           → Appends to logs[], updates viewport
           → agentDoneMsg signals completion
```

## Key Files

| File | Purpose |
|------|---------|
| `pkg/core/agent.go` | ReAct loop + event system + error diagnosis prompt |
| `pkg/core/analysis.go` | Error context extraction, stack trace parsing |
| `pkg/tui/app.go` | Minimal TUI with viewport, textinput, spinner, status line, history |
| `pkg/tui/styles.go` | 7-color palette, log prefixes, keyboard shortcut styles |
| `pkg/llm/ollama.go` | Ollama Cloud client with Bearer auth + streaming |
| `pkg/core/tools/http.go` | HTTP request tool + status code meanings/hints |
| `pkg/core/tools/file.go` | `read_file` and `list_files` tools |
| `pkg/core/tools/search.go` | `search_code` tool (ripgrep with native fallback) |
| `pkg/core/tools/persistence.go` | Save/load requests, environment management |
| `pkg/storage/schema.go` | YAML request/environment schema definitions |
| `pkg/storage/yaml.go` | YAML file read/write operations |
| `pkg/storage/env.go` | Environment variable substitution |

## Available Tools

| Tool | Description |
|------|-------------|
| `http_request` | Make HTTP requests (GET/POST/PUT/DELETE); includes status code meanings and error hints |
| `read_file` | Read file contents (100KB limit, security bounded) |
| `list_files` | List files with glob patterns (`**/*.go`, recursive) |
| `search_code` | Search patterns in codebase (ripgrep with native fallback) |
| `save_request` | Save API request to YAML file with {{VAR}} placeholders |
| `load_request` | Load saved request from YAML (substitutes environment variables) |
| `list_requests` | List all saved requests in `.zap/requests/` |
| `set_environment` | Set active environment (dev, prod, etc.) |
| `list_environments` | List available environments in `.zap/environments/` |

## Error Analysis Features

**Error Context Extraction (`pkg/core/analysis.go`)**:
- `ParseStackTrace()` - Extracts file:line from Python/Go/JS tracebacks
- `ExtractErrorContext()` - Parses error messages from JSON responses
- Handles FastAPI/Pydantic validation errors, common error fields
- `FormatErrorContext()` - Human-readable error summaries

**HTTP Response Enhancement (`pkg/core/tools/http.go`)**:
- `StatusCodeMeaning()` - Human-readable status code explanations
- `getErrorHints()` - Context-aware debugging hints (422, 500, etc.)
- Shows validation error fields when detected

## Current Capabilities

**What ZAP Can Do Now**:
1. ✓ Test APIs with natural language ("GET users from localhost:8000")
2. ✓ Diagnose API errors (find broken code, explain cause)
3. ✓ Search codebase for endpoints and error patterns
4. ✓ Read source code with context
5. ✓ Parse stack traces from error responses
6. ✓ Suggest fixes with code examples
7. ✓ Save/load requests to YAML files
8. ✓ Environment variable substitution ({{VAR}})
9. ✓ Switch between dev/prod environments

**What's Coming Next**:
- Sprint 4: Polish (JSON syntax highlighting, response diffing)
- Sprint 5: Launch prep (Postman import, installation script)
