# ZAP Development Guide

## Project Status: SPRINT 4 COMPLETE - DEVELOPER EXPERIENCE

ZAP now has a polished Developer Experience with JSON syntax highlighting, CLI scripting capabilities, clipboard support, and improved error messages. Ready for Sprint 5 (Launch Prep).

### Current Structure
```
zap/
├── cmd/zap/main.go           # Entry point with Cobra/Viper/Env (Interactive & CLI modes)
├── pkg/
│   ├── core/
│   │   ├── init.go           # .zap folder initialization
│   │   ├── agent.go          # ReAct Agent + Event System + Error Diagnosis
│   │   ├── analysis.go       # Stack trace parsing, error extraction
│   │   └── tools/
│       │   ├── http.go       # HTTP Tool + status code helpers
│       │   ├── file.go       # read_file, list_files tools
│       │   ├── search.go     # search_code tool (ripgrep/native)
│       │   └── persistence.go # save/load requests, environment management
│   ├── llm/
│   │   └── ollama.go         # Ollama Cloud client (Bearer auth)
│   ├── storage/
│   │   ├── schema.go         # Request/Environment YAML schema
│   │   ├── yaml.go           # YAML file operations
│   │   └── env.go            # Environment variable substitution
│   └── tui/
│       ├── app.go            # Minimal TUI (viewport, textinput, spinner)
│       ├── styles.go         # Minimal styling (7 colors, log prefixes)
│       └── highlight.go      # JSON syntax highlighting (Glamour)
```

## CLI Usage

ZAP defaults to interactive TUI mode, but now supports CLI flags for scripting:

```bash
# Interactive mode
zap

# Run a saved request (non-interactive)
zap --request my-request --env prod
```

## Working with the Agent

### Tool Interface
Every new capability must implement the `Tool` interface in `pkg/core/agent.go`:
```go
type Tool interface {
    Name() string
    Description() string
    Parameters() string
    Execute(args string) (string, error)
}
```

### Agent Event System (New)
The agent now supports real-time event emission:
```go
type AgentEvent struct {
    Type    string // "thinking", "tool_call", "observation", "answer", "error"
    Content string
}

type EventCallback func(AgentEvent)

// Use this for real-time UI updates
agent.ProcessMessageWithEvents(input, callback)

// Or use the original blocking version
agent.ProcessMessage(input)
```

### Logging
- Use `fmt.Fprintf(os.Stderr, ...)` for debug info
- stdout belongs to the TUI - never print there directly

## Getting Started

### Requirements
- Go 1.23+
- Ollama Cloud API Key (for `ollama.com`)

### Configuration
Create a `.env` file in the root:
```env
OLLAMA_API_KEY=your_key_here
```

Ensure `.zap/config.json` uses a cloud model:
```json
{
  "ollama_url": "https://ollama.com",
  "default_model": "gpt-oss:20b-cloud"
}
```

### Build & Run
```bash
go build -o zap.exe ./cmd/zap
./zap.exe
```

## TUI Architecture

### Components Used
- `bubbles/viewport` - Scrollable log area
- `bubbles/textinput` - Single-line input with `> ` prompt
- `bubbles/spinner` - Loading indicator
- `glamour` - Markdown rendering for responses
- `lipgloss` - Minimal styling

### Styling
Minimal 7-color palette:
- `#6c6c6c` - Dim (thinking, observations, help)
- `#e0e0e0` - Text (user input, responses)
- `#7aa2f7` - Accent (prompt, title, shortcuts)
- `#f7768e` - Error
- `#9ece6a` - Tool calls
- `#545454` - Muted (separators)
- `#73daca` - Success (future use)

Log prefixes:
- `> ` - User input
- `  thinking ` - Agent reasoning
- `  tool ` - Tool being called
- `  result ` - Tool observation
- `  error ` - Errors
- `───` - Conversation separator

### Keyboard Shortcuts
- `enter` - Send message
- `↑` / `↓` - Navigate input history
- `pgup` / `pgdown` - Scroll viewport
- `ctrl+l` - Clear screen
- `ctrl+u` - Clear input
- `ctrl+c` / `esc` - Quit

### Message Flow
```
User Input
    ↓
TUI captures Enter key
    ↓
runAgentAsync() starts goroutine
    ↓
Agent.ProcessMessageWithEvents() runs
    ↓
Callback sends AgentEvent via program.Send()
    ↓
TUI Update() receives agentEventMsg
    ↓
Appends to logs[], updates viewport
    ↓
agentDoneMsg signals completion
```

## What's Still Needed

### Sprint 1 - Codebase Tools - COMPLETE
- ✓ `read_file`, `list_files`, `search_code` tools
- ✓ Codebase-aware system prompt

### Sprint 2 - Error-Code Pipeline - COMPLETE
- ✓ Enhanced error diagnosis prompt with workflow
- ✓ HTTP status code meanings and hints
- ✓ Stack trace parsing (Python, Go, JS)
- ✓ Error context extraction from JSON
- ✓ Natural language → HTTP request conversion

### Sprint 3 - Persistence & Storage - COMPLETE
- ✓ YAML request schema definition (`pkg/storage/schema.go`)
- ✓ Save/load requests to YAML files (`save_request`, `load_request` tools)
- ✓ Request listing (`list_requests` tool)
- ✓ Environment variable substitution (`{{VAR}}` syntax)
- ✓ Environment switching (`set_environment`, `list_environments` tools)
- ✓ Auto-create `requests/` and `environments/` directories

### Sprint 4 - Developer Experience - COMPLETE
- ✓ JSON syntax highlighting in responses (Glamour)
- ✓ CLI flags (`--request`, `--env`) for non-interactive mode
- ✓ Request timing (ms) and size (KB) display
- ✓ Clipboard support (`Ctrl+Y`)
- ✓ Better tool error messages

### Sprint 5 Goals (Launch Prep)
1. Installation script (`curl | sh`)
2. README with demo GIF
3. Postman collection import
4. GitHub releases with binaries
5. Landing page

## Running on Other Projects

Run ZAP from the target project directory:
```bash
cd /path/to/your/project
/c/Users/user/zap/zap.exe
```

The tools use the current working directory as the project root for security bounds.

## Debugging

- If agent returns empty responses, check stderr logs
- Model name must match Ollama Cloud exactly (use `:cloud` suffix)
- Use `ctrl+c` or `esc` to quit cleanly
- Mouse wheel scrolls the viewport
