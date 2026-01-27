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
- Events: `thinking`, `tool_call`, `observation`, `answer`, `error`, `streaming`, `confirmation_required`
- Per-tool call limits to prevent runaway execution
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
- `ctrl+y` - Copy last response to clipboard
- `ctrl+c` / `esc` - Quit

**File Write Confirmation Mode** (shown when agent wants to modify files):
- `y` / `Y` - Approve file change
- `n` / `N` - Reject file change
- `pgup` / `pgdown` - Scroll diff
- `esc` - Reject and continue

### Configuration

On first run, creates `.zap/` folder containing:
- `config.json` - Ollama URL, model settings, tool limits, framework
- `history.jsonl` - Conversation log
- `memory.json` - Agent memory
- `requests/` - Saved API requests (YAML files)
- `environments/` - Environment configs (dev.yaml, prod.yaml, etc.)

Environment: `OLLAMA_API_KEY` loaded from `.env` file.

#### Framework Configuration

ZAP supports framework-aware assistance. The agent provides framework-specific hints for searching code and diagnosing errors.

**First-time setup**: If no framework is specified via flag, ZAP prompts you to select one:
```bash
# With flag (no prompt)
./zap.exe --framework gin

# Without flag (interactive prompt)
./zap.exe
# Select your API framework:
# 1. gin
# 2. echo
# ...
```

**Update existing config**:
```bash
./zap.exe --framework fastapi
# Updated framework to: fastapi
```

**Supported frameworks**:
- **Go**: gin, echo, chi, fiber
- **Python**: fastapi, flask, django
- **Node.js**: express, nestjs, hono
- **Java**: spring
- **PHP**: laravel
- **Ruby**: rails
- **Rust**: actix, axum
- **Other**: for custom/unlisted frameworks

#### Tool Limits Configuration

The agent uses per-tool call limits instead of a global iteration limit. This allows complex workflows while preventing runaway execution. Configure in `config.json`:

```json
{
  "tool_limits": {
    "default_limit": 50,
    "total_limit": 200,
    "per_tool": {
      "http_request": 25,
      "read_file": 50,
      "search_code": 30,
      "variable": 100
    }
  }
}
```

- `default_limit` - Fallback limit for tools without specific limit (default: 50)
- `total_limit` - Safety cap on total tool calls per session (default: 200)
- `per_tool` - Per-tool limits by name (overrides defaults)

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
| `pkg/core/tools/http.go` | HTTP request tool + status code meanings/hints + variable substitution |
| `pkg/core/tools/file.go` | `read_file` and `list_files` tools |
| `pkg/core/tools/write.go` | `write_file` tool with human-in-the-loop confirmation |
| `pkg/core/tools/confirm.go` | ConfirmationManager for file write approval flow |
| `pkg/core/tools/search.go` | `search_code` tool (ripgrep with native fallback) |
| `pkg/core/tools/persistence.go` | Save/load requests, environment management |
| `pkg/core/tools/assert.go` | Response validation tool (status, headers, body, timing) |
| `pkg/core/tools/extract.go` | Value extraction tool (JSON path, headers, cookies, regex) |
| `pkg/core/tools/variables.go` | Variable management (session/global with persistence) |
| `pkg/core/tools/timing.go` | Wait and retry tools (delays, backoff strategies) |
| `pkg/core/tools/manager.go` | Response manager for sharing HTTP responses between tools |
| `pkg/core/tools/schema.go` | JSON Schema validation tool (draft-07, draft-2020-12) |
| `pkg/core/tools/auth.go` | Authentication tools (Bearer, Basic, OAuth2, JWT parsing) |
| `pkg/core/tools/suite.go` | Test suite execution with pass/fail reporting |
| `pkg/core/tools/diff.go` | Response comparison for regression testing |
| `pkg/core/tools/perf.go` | Performance/load testing with latency metrics |
| `pkg/core/tools/webhook.go` | Webhook listener (temporary HTTP server) |
| `pkg/storage/schema.go` | YAML request/environment schema definitions |
| `pkg/storage/yaml.go` | YAML file read/write operations |
| `pkg/storage/env.go` | Environment variable substitution |

## Available Tools

### Core API Tools
| Tool | Description |
|------|-------------|
| `http_request` | Make HTTP requests (GET/POST/PUT/DELETE); includes status code meanings, error hints, and variable substitution |
| `save_request` | Save API request to YAML file with {{VAR}} placeholders |
| `load_request` | Load saved request from YAML (substitutes environment variables) |
| `list_requests` | List all saved requests in `.zap/requests/` |
| `set_environment` | Set active environment (dev, prod, etc.) |
| `list_environments` | List available environments in `.zap/environments/` |

### Testing & Validation Tools (Sprint 1)
| Tool | Description |
|------|-------------|
| `assert_response` | Validate API responses (status codes, headers, body content, JSON path, performance) |
| `extract_value` | Extract values from responses (JSON path, headers, cookies, regex) for request chaining |
| `variable` | Manage session/global variables (set, get, delete, list) with disk persistence |
| `wait` | Add delays for async operations (webhooks, polling, rate limiting) |
| `retry` | Retry tool execution with configurable attempts, delay, and exponential backoff |

### Advanced Testing Tools (Sprint 2)
| Tool | Description |
|------|-------------|
| `validate_json_schema` | Validate response bodies against JSON Schema specs (draft-07, draft-2020-12) |
| `auth_bearer` | Create Bearer token authorization headers (JWT, API tokens) |
| `auth_basic` | Create HTTP Basic authentication headers (base64 encoded) |
| `auth_helper` | Parse JWT tokens, decode Basic auth, show claims and metadata |
| `test_suite` | Run organized test suites with multiple tests, assertions, and value extraction |
| `compare_responses` | Compare API responses for regression testing with baseline management |

### Performance & OAuth Tools (Sprint 3 - MVP)
| Tool | Description |
|------|-------------|
| `performance_test` | Run load tests with concurrent users, measure latency (p50/p95/p99), throughput, error rate |
| `webhook_listener` | Start temporary HTTP server to capture webhook callbacks (start/stop/get_requests) |
| `auth_oauth2` | Perform OAuth2 authentication (client_credentials, password flows) |

### Codebase Analysis Tools
| Tool | Description |
|------|-------------|
| `read_file` | Read file contents (100KB limit, security bounded) |
| `write_file` | Write/modify files with human-in-the-loop confirmation (shows diff, requires y/n approval) |
| `list_files` | List files with glob patterns (`**/*.go`, recursive) |
| `search_code` | Search patterns in codebase (ripgrep with native fallback) |

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
10. ✓ JSON syntax highlighting in responses
11. ✓ Display request timing (ms) and response size (KB)
12. ✓ CLI mode for scripting (`--request`, `--env` flags)
13. ✓ Copy responses to clipboard (`ctrl+y`)
14. ✓ **Validate responses** (status, headers, body, JSON path, performance)
15. ✓ **Chain requests** (extract values → use in next request)
16. ✓ **Manage variables** (session/global with persistence)
17. ✓ **Retry failed requests** (exponential backoff, conditional retry)
18. ✓ **Wait for async operations** (webhooks, polling)
19. ✓ **JSON Schema validation** (contract testing, type validation)
20. ✓ **JWT/Bearer token auth** (automatic header creation)
21. ✓ **HTTP Basic auth** (base64 encoding)
22. ✓ **JWT token parsing** (decode claims, expiration, subject)
23. ✓ **Test suites** (organized multi-test execution)
24. ✓ **Regression testing** (baseline comparison, diff detection)
25. ✓ **Load testing** (concurrent users, latency percentiles, throughput)
26. ✓ **Webhook capture** (temporary HTTP server for callbacks)
27. ✓ **OAuth2 authentication** (client_credentials, password flows)
28. ✓ **File writing with confirmation** (colored diff, y/n approval before changes)

**Sprint 1 Completed Features**:
- ✓ Response assertions (status, headers, body, JSON path, timing)
- ✓ Value extraction (JSON path, headers, cookies, regex)
- ✓ Variable management (session/global with persistence)
- ✓ Retry with exponential backoff
- ✓ Wait/delay tools

**Sprint 2 Completed Features**:
- ✓ JSON Schema validation (contract testing)
- ✓ Bearer token auth (JWT, API tokens)
- ✓ HTTP Basic authentication
- ✓ JWT token parsing
- ✓ Test suite organization
- ✓ Regression testing with baseline comparison

**Sprint 3 Completed Features (MVP)**:
- ✓ Performance/load testing (concurrent users, p50/p95/p99 latency)
- ✓ Webhook listener (capture callbacks with temporary server)
- ✓ OAuth2 authentication (client_credentials, password flows)

**Future Enhancements (Post-MVP)**:
- Mock response generation
- Complex workflows with DAG execution
- Authorization Code OAuth2 flow (requires browser)
- Distributed load testing

## CLI Usage

ZAP supports both interactive and non-interactive modes:

```bash
# Interactive mode (default)
./zap.exe

# First-time setup with framework
./zap.exe --framework gin
./zap.exe -f fastapi

# Execute a saved request with environment
./zap.exe --request get-users --env prod
./zap.exe -r get-users -e dev

# Show help
./zap.exe --help
```
