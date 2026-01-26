package tui

import (
	"sync"

	"github.com/blackcoderx/zap/pkg/core"
	"github.com/blackcoderx/zap/pkg/core/tools"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

// logEntry represents a single log line in the UI
type logEntry struct {
	Type    string // "user", "thinking", "tool", "observation", "response", "error", "separator", "streaming"
	Content string
}

// ToolUsageDisplay represents tool usage for TUI display
type ToolUsageDisplay struct {
	Name    string
	Current int
	Limit   int
	Percent int
}

// Model is the Bubble Tea model for the ZAP TUI.
// It manages the state of the terminal interface including:
// - viewport for scrollable message history
// - textinput for user input
// - spinner for loading states
// - agent for LLM interaction
type Model struct {
	viewport        viewport.Model
	textinput       textinput.Model
	spinner         spinner.Model
	logs            []logEntry
	thinking        bool
	width           int
	height          int
	agent           *core.Agent
	ready           bool
	renderer        *glamour.TermRenderer
	inputHistory    []string // history of user inputs
	historyIdx      int      // current position in history (-1 = new input)
	savedInput      string   // saved input when navigating history
	status          string   // current status: "idle", "thinking", "tool:name", "streaming"
	currentTool     string   // name of tool currently being executed
	streamingBuffer string   // buffer for accumulating streaming content
	modelName       string   // current LLM model name for badge display

	// Tool usage tracking for display
	toolUsage      []ToolUsageDisplay // Current tool usage stats
	totalCalls     int                // Total tool calls in session
	totalLimit     int                // Total limit
	lastToolName   string             // Last tool that was called
	lastToolCount  int                // Last tool's current count
	lastToolLimit  int                // Last tool's limit

	// Confirmation state for file write approval
	confirmationMode    bool                      // True when awaiting user confirmation
	pendingConfirmation *core.FileConfirmation    // Details of the pending file change
	confirmManager      *tools.ConfirmationManager // Shared confirmation manager
}

// agentEventMsg wraps an agent event for the TUI
type agentEventMsg struct {
	event core.AgentEvent
}

// agentDoneMsg signals the agent has finished
type agentDoneMsg struct {
	err error
}

// programRef holds the program reference for sending messages from goroutines.
// Using a struct with mutex for thread-safe access instead of a bare global variable.
type programRef struct {
	mu      sync.RWMutex
	program *tea.Program
}

// Set updates the program reference (thread-safe).
func (p *programRef) Set(prog *tea.Program) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.program = prog
}

// Send sends a message to the program if it exists (thread-safe).
func (p *programRef) Send(msg tea.Msg) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.program != nil {
		p.program.Send(msg)
	}
}

// Global program reference with thread-safe accessors.
// This is still a package-level variable but access is now synchronized.
var globalProgram = &programRef{}
