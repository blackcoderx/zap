# ZAP Development Progress

**Last Updated:** 2026-01-21
**Current Phase:** Sprint 4 Complete - Developer Experience

---

## What We've Done

### Phase 1: The "Smart Curl" (MVP) - COMPLETE
- [x] Scaffold Go project structure
- [x] Implement `.zap` folder initialization
- [x] Build basic TUI (Bubble Tea + Lip Gloss)
- [x] Connect to Ollama Cloud (API Key support)
- [x] Implement `HttpClient` tool
- [x] Implement ReAct Agent Core (Reason -> Act -> Observe loop)
- [x] Integrate Agent with TUI
- [x] Fix keyboard shortcuts (ctrl+c/esc for quit)

### Phase 1.5: Minimal TUI Redesign - COMPLETE
- [x] Add Agent Event System (`AgentEvent`, `EventCallback`, `ProcessMessageWithEvents`)
- [x] Replace chat UI with log-centric viewport design
- [x] Implement `bubbles/viewport` for scrollable logs
- [x] Implement `bubbles/textinput` for single-line input
- [x] Implement `bubbles/spinner` for loading indicator
- [x] Add `glamour` for markdown rendering
- [x] Reduce styling to minimal 5-color palette
- [x] Add Claude Code-style log prefixes (`> `, `  thinking `, `  tool `, etc.)
- [x] Enable async agent execution with real-time event updates
- [x] Enable mouse support for viewport scrolling

### Phase 1.6: UI Refinements - COMPLETE
- [x] Add status line showing current state (idle/thinking/executing tool)
- [x] Add input history navigation (↑/↓ arrow keys)
- [x] Add keyboard shortcuts (`ctrl+l` clear, `ctrl+u` clear input)
- [x] Add visual separators (`───`) between conversations
- [x] Improve help line with styled keyboard shortcuts
- [x] Better observation truncation (first 150 + last 30 chars)
- [x] Expand color palette with `MutedColor` and `SuccessColor`

### Phase 1.7: Streaming Responses - COMPLETE
- [x] Implement streaming in LLM client (`ChatStream` method in ollama.go)
- [x] Add "streaming" event type to agent event system
- [x] Handle streaming events in TUI (real-time text display)
- [x] Add streaming status indicator (`⠋ streaming...`)
- [x] Fix viewport scrolling (only auto-scroll when at bottom)
- [x] Add `pgup`/`pgdown`/`home`/`end` support for scrolling

### Sprint 3: Persistence & Storage - COMPLETE
- [x] Create `pkg/storage/` package with YAML schema definitions
- [x] Implement `save_request` tool (save to `.zap/requests/*.yaml`)
- [x] Implement `load_request` tool (load and substitute variables)
- [x] Implement `list_requests` tool
- [x] Implement `set_environment` and `list_environments` tools
- [x] Add `{{VAR}}` variable substitution from environment files
- [x] Auto-create `requests/` and `environments/` directories on init
- [x] Create default `dev.yaml` environment template
- [x] Update agent system prompt with persistence guidance
- [x] Fix stack trace line number bug in `analysis.go`

### Sprint 4: Developer Experience - COMPLETE
- [x] Implement JSON Syntax Highlighting (`pkg/tui/highlight.go` + Glamour)
- [x] Add Request Timing (ms) and Size (KB) to response display
- [x] Enhance `http_request` tool output with Markdown code blocks
- [x] Improve Agent error messages (Connection Error, Empty Response, Max Steps)
- [x] Implement `zap --request <file>` CLI flag for non-interactive execution
- [x] Implement `zap --env <name>` CLI flag
- [x] Add `Ctrl+Y` shortcut to copy last response to clipboard
- [x] Update `--help` documentation and descriptions

---

## Current Architecture

### Agent Event System
```go
type AgentEvent struct {
    Type    string // "thinking", "tool_call", "observation", "answer", "error"
    Content string
}

// Real-time events via callback
agent.ProcessMessageWithEvents(input, func(event AgentEvent) {
    // Handle event
})
```

### TUI Components
| Component | Library | Purpose |
|-----------|---------|---------|
| Viewport | `bubbles/viewport` | Scrollable log area |
| TextInput | `bubbles/textinput` | Single-line input with `> ` prompt |
| Spinner | `bubbles/spinner` | Loading indicator |
| Glamour | `glamour` | Markdown rendering |
| Lipgloss | `lipgloss` | Minimal styling |

### File Structure
```
zap/
├── cmd/zap/main.go           # Entry point
├── pkg/
│   ├── core/
│   │   ├── init.go           # .zap initialization
│   │   ├── agent.go          # ReAct Agent + Event System
│   │   ├── analysis.go       # Stack trace parsing, error extraction
│   │   └── tools/
│   │       ├── http.go       # HTTP Tool
│   │       ├── file.go       # read_file, list_files tools
│   │       ├── search.go     # search_code tool
│   │       └── persistence.go # save/load requests, environments
│   ├── llm/
│   │   └── ollama.go         # Ollama Cloud client
│   ├── storage/
│   │   ├── schema.go         # YAML request/environment schema
│   │   ├── yaml.go           # YAML file operations
│   │   └── env.go            # Variable substitution
│   └── tui/
│       ├── app.go            # Minimal TUI
│       └── styles.go         # 7-color palette, log prefixes
```

---

## What's Still Needed

### Completed Sprints
- ✓ **Sprint 1**: Codebase Tools (read_file, list_files, search_code)
- ✓ **Sprint 2**: Error-Code Pipeline (diagnosis workflow, stack traces)
- ✓ **Sprint 3**: Persistence & Storage (YAML save/load, environments)
- ✓ **Sprint 4**: Developer Experience (JSON highlight, CLI flags, Clipboard)

### Sprint 5: Launch Prep (NEXT)
- [ ] JSON syntax highlighting in responses
- [ ] Better error messages
- [ ] `--help` and usage documentation
- [ ] `--request` CLI flag for scripting
- [ ] Request timing display
- [ ] Response size display
- [ ] Copy response to clipboard


### Future
- [ ] `FileEdit` tool with human approval
- [ ] VS Code extension (JSON-RPC communication)
- [ ] OAuth 2.0 flow support
- [ ] Request chaining

---

## Key Design Decisions

### Why Event-Based Agent?
The original `ProcessMessage()` was blocking - UI couldn't show intermediate states. New `ProcessMessageWithEvents()` emits events at each stage so TUI can update in real-time.

### Why Minimal Styling?
User requested Claude Code style - minimal, log-centric, no decorative borders or emoji. Reduced from ~10 colors to 5. Removed all `RoundedBorder()` calls.

### Why Global Program Reference?
Bubble Tea's architecture requires `program.Send()` to push messages from goroutines. We store the program reference globally so the agent callback can send events back to the TUI.

---

## Build & Run

```bash
go build -o zap.exe ./cmd/zap
./zap.exe
```

## Configuration

`.env`:
```env
OLLAMA_API_KEY=your_key_here
```

`.zap/config.json`:
```json
{
  "ollama_url": "https://ollama.com",
  "default_model": "gpt-oss:20b-cloud"
}
```

---

## Critical Design Principles

1. **Context is King** - Agent must see actual code, not guess
2. **Human in the Loop** - Dangerous operations require approval
3. **Fail Loudly** - Errors should be visible and helpful
4. **Local First** - Everything works offline except LLM calls
5. **Minimal UX** - Claude Code style, not flashy prototype
