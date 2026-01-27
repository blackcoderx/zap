# Package core

The `core` package provides the central agent logic, tool management, and ReAct loop implementation for the ZAP API debugging assistant.

## Overview

This package implements the AI agent that powers ZAP. It uses a ReAct (Reason+Act) pattern where the agent:

1. **Receives** a user message
2. **Reasons** about the task using the LLM
3. **Acts** by selecting and executing tools as needed
4. **Observes** the results of tool execution
5. **Continues** the cycle until a final answer is reached

## Key Components

### Agent (`agent.go`)
The main AI agent struct that orchestrates the ReAct loop.

```go
agent := core.NewAgent(llmClient)
agent.RegisterTool(httpTool)
agent.SetFramework("gin")
answer, err := agent.ProcessMessageWithEvents(input, callback)
```

### Tool Interface (`types.go`)
All agent capabilities implement the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() string
    Execute(args string) (string, error)
}
```

### ReAct Loop (`react.go`)
The core reasoning loop that processes messages:

- `ProcessMessage(input)` - Blocking, returns final answer
- `ProcessMessageWithEvents(input, callback)` - Emits events for real-time UI updates

### System Prompts (`prompt.go`)
Constructs the LLM system prompt with:
- Available tools and their descriptions
- Framework-specific guidance (Gin, FastAPI, Express, etc.)
- Error diagnosis workflows
- Request chaining patterns

### Memory Store (`memory.go`, `session.go`)
Persistent memory across sessions:
- Key-value facts storage
- Session tracking and history
- Conversation summaries

### Configuration (`init.go`)
Initialization and configuration loading:
- `.zap` folder creation
- Framework selection
- Tool limits configuration

### Error Analysis (`analysis.go`)
Error context extraction and stack trace parsing:
- Multi-language stack trace parsing (Python, Go, JavaScript)
- JSON error response parsing
- Human-readable error formatting

## Event System

The agent emits events during processing for real-time UI updates:

| Event Type | Description |
|------------|-------------|
| `thinking` | Agent is reasoning |
| `tool_call` | Executing a tool |
| `observation` | Tool returned a result |
| `answer` | Final answer ready |
| `error` | An error occurred |
| `streaming` | LLM response chunk |
| `tool_usage` | Tool usage statistics |
| `confirmation_required` | File write needs approval |

## Tool Limits

The agent enforces per-tool call limits to prevent runaway execution:

```go
agent.SetToolLimit("http_request", 25)  // Max 25 HTTP requests
agent.SetDefaultLimit(50)               // Default for other tools
agent.SetTotalLimit(200)                // Safety cap total
```

## File Structure

```
pkg/core/
├── doc.md          # This file
├── types.go        # Core interfaces and types
├── agent.go        # Agent struct and tool management
├── react.go        # ReAct loop implementation
├── prompt.go       # System prompt construction
├── parser.go       # LLM response parsing
├── memory.go       # Persistent memory store
├── session.go      # Session tracking and history
├── analysis.go     # Error context extraction
├── init.go         # Initialization and config
└── tools/          # Agent tool implementations
```
