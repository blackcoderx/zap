# TUI Package Documentation

The `pkg/tui` package provides the terminal user interface for ZAP. It uses [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI framework with a minimal, Claude Code-inspired design.

## File Structure

```
pkg/tui/
├── app.go          # Entry point (Run function)
├── model.go        # Model struct and message types
├── init.go         # Model initialization and tool registration
├── update.go       # Event handling and state updates
├── view.go         # Rendering and display logic
├── keys.go         # Keyboard input handling
├── styles.go       # Visual styling (colors, borders, etc.)
├── highlight.go    # JSON syntax highlighting
└── tui.md          # This documentation
```

## File Descriptions

### app.go
The main entry point for the TUI. Contains only the `Run()` function which:
- Creates the initial model
- Sets up the Bubble Tea program with alt screen and mouse support
- Manages the global program reference for async event handling
- Starts and runs the TUI event loop

### model.go
Defines all data structures used by the TUI:

**Types:**
- `Model` - The main Bubble Tea model containing all UI state
- `logEntry` - Represents a single log entry (user input, tool calls, responses, etc.)
- `agentEventMsg` - Wraps agent events for the TUI message system
- `agentDoneMsg` - Signals agent processing completion
- `programRef` - Thread-safe wrapper for the tea.Program reference

**Key Model Fields:**
- `viewport` - Scrollable area for message history
- `textinput` - User input field
- `spinner` - Loading animation
- `logs` - Message history
- `agent` - The LLM agent for processing requests
- `inputHistory` - Previous user inputs for up/down navigation
- `status` - Current state (idle, thinking, streaming, tool)

### init.go
Handles initialization and setup:

**Functions:**
- `InitialModel()` - Creates the initial TUI model with all components
- `Init()` - Bubble Tea initialization (called once at startup)
- `registerTools()` - Registers all agent tools (HTTP, file, search, testing, etc.)
- `newLLMClient()` - Creates the Ollama LLM client from config
- `newSpinner()` - Creates the loading spinner with ZAP styling
- `newTextInput()` - Creates the input field with ZAP styling
- `newGlamourRenderer()` - Creates the markdown renderer

### update.go
Handles all message processing and state updates:

**Functions:**
- `Update()` - Main Bubble Tea update function (routes messages)
- `runAgentAsync()` - Starts agent processing in a goroutine
- `handleWindowResize()` - Adjusts layout when terminal is resized
- `handleAgentEvent()` - Processes events from the agent (thinking, streaming, tool_call, etc.)
- `handleAgentDone()` - Handles agent completion/errors

### view.go
Handles all rendering and display:

**Functions:**
- `View()` - Main Bubble Tea view function (renders the UI)
- `updateViewportContent()` - Updates the message display area
- `formatLogEntry()` - Formats a log entry for display (applies styles, truncation)
- `renderStatus()` - Renders the current agent status
- `renderInputArea()` - Renders the input field
- `renderFooter()` - Renders the footer with model info and shortcuts

### keys.go
Centralizes all keyboard input handling:

**Functions:**
- `handleKeyMsg()` - Routes key events to specific handlers
- `handleClearScreen()` - Ctrl+L: Clear all logs
- `handleCopyLastResponse()` - Ctrl+Y: Copy last response to clipboard
- `handleClearInput()` - Ctrl+U: Clear current input
- `handleHistoryUp()` - Up arrow: Navigate to previous input
- `handleHistoryDown()` - Down arrow: Navigate to next input
- `handleEnter()` - Enter: Send message to agent
- `handleViewportScroll()` - PgUp/PgDown: Scroll viewport

### styles.go
Defines all visual styles using [Lipgloss](https://github.com/charmbracelet/lipgloss):

**Colors:**
- `DimColor` - Muted text (#6c6c6c)
- `TextColor` - Primary text (#e0e0e0)
- `AccentColor` - Blue highlights (#7aa2f7)
- `ErrorColor` - Red for errors (#f7768e)
- `ToolColor` - Green for tools (#9ece6a)
- `SuccessColor` - Teal for success (#73daca)

**Styles:**
- `UserMessageStyle` - User messages with blue left border
- `ToolCallStyle` - Dimmed tool calls with circle prefix
- `AgentMessageStyle` - Plain agent responses
- `InputAreaStyle` - Input field styling
- `FooterStyle` - Footer bar styling
- Status styles for idle/active/tool states
- Shortcut key/description styles

**Prefixes:**
- `UserPrefix` - "> "
- `ToolCallPrefix` - "○ "
- Standard prefixes for thinking, tool, observation, error

### highlight.go
JSON syntax highlighting utility:

**Functions:**
- `HighlightJSON()` - Validates JSON, pretty-prints it, and renders with Glamour

## Architecture

### Message Flow

```
User Input (keyboard)
    │
    ▼
handleKeyMsg() [keys.go]
    │
    ├──► handleEnter() ──► runAgentAsync() [update.go]
    │                           │
    │                           ▼
    │                      Agent processes
    │                           │
    │                           ▼
    │                      Events sent via globalProgram.Send()
    │
    ▼
Update() [update.go]
    │
    ├──► handleAgentEvent() ──► updateViewportContent()
    │
    ▼
View() [view.go]
    │
    ├──► viewport.View()
    ├──► renderInputArea()
    └──► renderFooter()
```

### Log Entry Types

| Type | Description | Style |
|------|-------------|-------|
| `user` | User input message | Blue left border, gray background |
| `thinking` | Agent reasoning (hidden) | - |
| `streaming` | Partial response | Plain text |
| `tool` | Tool invocation | Dimmed with ○ prefix |
| `observation` | Tool result | Dimmed, truncated |
| `response` | Final agent answer | Markdown rendered |
| `error` | Error message | Red text |
| `separator` | Visual break | Hidden |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Enter | Send message |
| ↑/↓ | Navigate input history |
| PgUp/PgDown | Scroll viewport |
| Ctrl+L | Clear screen |
| Ctrl+U | Clear input |
| Ctrl+Y | Copy last response |
| Ctrl+C/Esc | Quit |

## Customization

### Changing Colors
Edit `styles.go` to modify the color palette. All colors are defined as `lipgloss.Color` values at the top of the file.

### Changing Spinner
Edit `newSpinner()` in `init.go` to change the loading animation frames or FPS.

### Adding New Key Bindings
1. Add a case in `handleKeyMsg()` in `keys.go`
2. Create a handler function following the pattern: `func (m Model) handleXxx() (Model, tea.Cmd)`

### Adding New Log Entry Types
1. Add the type string to the `logEntry.Type` documentation
2. Add a case in `formatLogEntry()` in `view.go`
3. Add styling in `styles.go` if needed

## Fluid/Responsive Layout

The TUI uses fluid layout that adapts to terminal resizing:

### How It Works

1. **Window Resize Handling** (`update.go`):
   - `handleWindowResize()` receives `tea.WindowSizeMsg` events
   - Updates viewport dimensions based on new terminal size
   - Recalculates textinput width
   - Recreates glamour renderer with new word wrap width
   - Re-renders viewport content

2. **Dynamic Width Styles** (`view.go`, `styles.go`):
   - Styles use `.Copy().Width(n)` at render time instead of fixed widths
   - `getContentWidth()` calculates available width for content
   - User messages and input area adapt to terminal width

3. **Key Functions**:
   - `handleWindowResize()` - Main resize handler
   - `getContentWidth()` - Calculates content area width
   - `updateGlamourWidth()` - Updates markdown renderer for new width
   - `renderInputArea()` - Renders fluid input box

### Best Practices

- Always use `.Copy().Width(calculatedWidth)` instead of fixed widths
- Account for borders, padding, and margins when calculating widths
- Call `updateViewportContent()` after resize to re-render with new dimensions
- Test with various terminal sizes (narrow, wide, very small)

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components (viewport, textinput, spinner)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Glamour](https://github.com/charmbracelet/glamour) - Markdown rendering
- [clipboard](https://github.com/atotto/clipboard) - Clipboard access
