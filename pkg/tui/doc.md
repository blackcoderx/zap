# Package tui

The `tui` package provides the terminal user interface for the ZAP API debugging assistant using Bubble Tea.

## Overview

This package implements a minimal, Claude Code-inspired terminal interface with:

- **Scrollable viewport** for conversation history (PgUp/PgDown, mouse wheel)
- **Text input** with command history navigation (Shift+↑/↓)
- **Real-time streaming** of LLM responses as they arrive
- **Tool call visualization** with usage statistics
- **File write confirmation** with colored diffs
- **Markdown rendering** with syntax highlighting via Glamour

## Components

### Model (`model.go`)
The Bubble Tea model that holds all UI state:
- Viewport for scrollable content
- Text input for user messages
- Spinner for loading states
- Agent reference for processing messages

### View (`view.go`)
Rendering functions for terminal output:
- `View()` - Main render function called by Bubble Tea
- `formatLogEntry()` - Format individual log entries
- `renderConfirmationView()` - File write confirmation dialog
- `renderColoredDiff()` - Syntax-highlighted diffs

### Update (`update.go`)
Event handling and state transitions:
- Keyboard input handling
- Agent event processing
- Window resize handling
- Scroll position management

### Styles (`styles.go`)
Visual styling with Lipgloss:
- 7-color minimal palette
- Log entry prefixes (`> `, `thinking`, `tool`, `result`, `error`)
- Conversation separators
- Status line styling

### Keys (`keys.go`)
Keyboard shortcut definitions:
- Enter: Send message
- Shift+↑/↓: Navigate input history
- PgUp/PgDown: Scroll viewport
- Ctrl+L: Clear screen
- Ctrl+U: Clear input line
- Ctrl+Y: Copy last response
- Ctrl+C/Esc: Quit

### Init (`init.go`)
Initialization and setup:
- Agent creation and tool registration
- LLM client configuration
- Spinner and text input setup
- Glamour renderer for markdown

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Shift+↑` / `Shift+↓` | Navigate input history |
| `PgUp` / `PgDown` | Scroll viewport |
| `Ctrl+L` | Clear screen |
| `Ctrl+U` | Clear input line |
| `Ctrl+Y` | Copy last response to clipboard |
| `Ctrl+C` / `Esc` | Quit |

### File Write Confirmation Mode

When the agent wants to modify files, a confirmation dialog appears:

| Key | Action |
|-----|--------|
| `Y` / `y` | Approve file change |
| `N` / `n` | Reject file change |
| `PgUp` / `PgDown` | Scroll diff |
| `Esc` | Reject and continue |

## File Structure

```
pkg/tui/
├── doc.md        # This file
├── app.go        # Application entry point
├── model.go      # Bubble Tea model definition
├── view.go       # Rendering functions
├── update.go     # Event handlers
├── init.go       # Initialization and setup
├── styles.go     # Visual styling
├── keys.go       # Keyboard shortcuts
└── highlight.go  # Syntax highlighting utilities
```

## Usage

```go
import "github.com/blackcoderx/zap/pkg/tui"

func main() {
    model := tui.InitialModel()
    p := tea.NewProgram(model, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        log.Fatal(err)
    }
}
```
