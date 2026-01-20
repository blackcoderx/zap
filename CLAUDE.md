# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ZAP is an AI-powered API testing assistant that runs in the terminal. It uses local LLMs (Ollama) or cloud providers to provide an autonomous agent that can make HTTP requests and interact with APIs.

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
- **pkg/core/tools/** - Agent tools (HTTP client, etc.)
- **pkg/llm/** - LLM client implementations (Ollama)
- **pkg/tui/** - Minimal terminal UI using Bubble Tea

### Core Components

**Agent (pkg/core/agent.go)**: Implements ReAct (Reason+Act) loop with event system:
- `ProcessMessage(input)` - Blocking, returns final answer
- `ProcessMessageWithEvents(input, callback)` - Emits events for real-time UI updates
- Events: `thinking`, `tool_call`, `observation`, `answer`, `error`
- Max 5 iterations to prevent infinite loops

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
- `bubbles/viewport` - Scrollable log area
- `bubbles/textinput` - Single-line input with `> ` prompt
- `bubbles/spinner` - Loading indicator
- `glamour` - Markdown rendering for responses

**Styling (pkg/tui/styles.go)**: Minimal 5-color palette with log prefixes:
- `> ` user input
- `  thinking ` agent reasoning
- `  tool ` tool calls
- `  result ` observations
- `  error ` errors

### Configuration

On first run, creates `.zap/` folder containing:
- `config.json` - Ollama URL, model settings
- `history.jsonl` - Conversation log
- `memory.json` - Agent memory

Environment: `OLLAMA_API_KEY` loaded from `.env` file.

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
| `pkg/core/agent.go` | ReAct loop + event system |
| `pkg/tui/app.go` | Minimal TUI with viewport, textinput, spinner |
| `pkg/tui/styles.go` | 5-color palette, log prefixes |
| `pkg/llm/ollama.go` | Ollama Cloud client with Bearer auth |
| `pkg/core/tools/http.go` | HTTP request tool |
