# ZAP Development Progress

**Last Updated:** 2026-01-20
**Current Phase:** Phase 1.5 - TUI Redesign Complete

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
│   │   └── tools/
│   │       └── http.go       # HTTP Tool
│   ├── llm/
│   │   └── ollama.go         # Ollama Cloud client
│   └── tui/
│       ├── app.go            # Minimal TUI
│       └── styles.go         # 5-color palette, log prefixes
```

---

## What's Still Needed

### For True Claude Code Style
The current UI is minimal but not yet at Claude Code level:

1. **Streaming responses** - Show text as it arrives character by character
2. **Better formatting** - Sophisticated word wrapping and truncation
3. **Status line** - Persistent line showing current state
4. **Keyboard navigation** - Arrow keys to scroll history
5. **Multi-line input** - Support pasting multi-line content

### Phase 2: Security & Context (NEXT)
- [ ] `.env` loader and secret manager
- [ ] `FileSystem` tool (read local code)
- [ ] `CodeSearch` tool (grep-based search)
- [ ] Conversation history persistence to `.zap/history.jsonl`
- [ ] Variable system for reusable values

### Phase 3: The "Fixer" & Extension
- [ ] `FileEdit` tool with human approval
- [ ] VS Code extension (JSON-RPC communication)

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
