# ZAP Development Guide

## Project Status: PHASE 1.5 - TUI REDESIGN COMPLETE

The TUI has been redesigned from a colorful chat interface to a minimal, log-centric design inspired by Claude Code. Core agent functionality remains solid.

### Current Structure
```
zap/
├── cmd/zap/main.go           # Entry point with Cobra/Viper/Env
├── pkg/
│   ├── core/
│   │   ├── init.go           # .zap folder initialization
│   │   ├── agent.go          # ReAct Agent + Event System
│   │   └── tools/
│   │       └── http.go       # HTTP Tool (implements core.Tool)
│   ├── llm/
│   │   └── ollama.go         # Ollama Cloud client (Bearer auth)
│   └── tui/
│       ├── app.go            # Minimal TUI (viewport, textinput, spinner)
│       └── styles.go         # Minimal styling (5 colors, log prefixes)
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
Minimal 5-color palette:
- `#6c6c6c` - Dim (thinking, observations, help)
- `#e0e0e0` - Text (user input, responses)
- `#7aa2f7` - Accent (prompt, title)
- `#f7768e` - Error
- `#9ece6a` - Tool calls

Log prefixes:
- `> ` - User input
- `  thinking ` - Agent reasoning
- `  tool ` - Tool being called
- `  result ` - Tool observation
- `  error ` - Errors

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

### For True Claude Code Style
1. Streaming responses (show text as it arrives)
2. Better log formatting and word wrapping
3. Status line showing current state
4. Keyboard navigation through history
5. Multi-line input support

### Phase 2 Goals
1. `FileSystem` tool - Read local code
2. `CodeSearch` tool - Grep-based search
3. History persistence to `.zap/history.jsonl`
4. Variable system for reusable values

## Debugging

- If agent returns empty responses, check stderr logs
- Model name must match Ollama Cloud exactly (use `:cloud` suffix)
- Use `ctrl+c` or `esc` to quit cleanly
- Mouse wheel scrolls the viewport
