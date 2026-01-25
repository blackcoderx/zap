package tui

import (
	"github.com/blackcoderx/zap/pkg/core"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// runAgentAsync starts the agent in a goroutine and sends events via the program.
// This allows the TUI to remain responsive while the agent processes the request.
func runAgentAsync(agent *core.Agent, input string) tea.Cmd {
	return func() tea.Msg {
		// Run agent in goroutine so we can send intermediate events
		go func() {
			callback := func(event core.AgentEvent) {
				globalProgram.Send(agentEventMsg{event: event})
			}

			_, err := agent.ProcessMessageWithEvents(input, callback)
			globalProgram.Send(agentDoneMsg{err: err})
		}()

		// Return nil - actual results come via program.Send
		return nil
	}
}

// Update handles all messages and updates the model state.
// This is the main event loop handler for the Bubble Tea application.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle special keys
		updatedModel, cmd := m.handleKeyMsg(msg)
		if cmd != nil {
			return updatedModel, cmd
		}
		// If handleKeyMsg returned nil cmd, continue to handle the key
		// in the textinput below (for regular character input)
		m = updatedModel

	case tea.WindowSizeMsg:
		m = m.handleWindowResize(msg)

	case agentEventMsg:
		m = m.handleAgentEvent(msg)
		cmds = append(cmds, m.spinner.Tick)

	case agentDoneMsg:
		m = m.handleAgentDone(msg)

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update textinput (for regular character input)
	if !m.thinking {
		var cmd tea.Cmd
		m.textinput, cmd = m.textinput.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleWindowResize adjusts the layout when the terminal is resized.
func (m Model) handleWindowResize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height

	// Calculate viewport dimensions
	inputHeight := 1
	footerHeight := 1
	margins := 3

	viewportHeight := m.height - inputHeight - footerHeight - margins
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	if !m.ready {
		m.viewport = viewport.New(m.width-2, viewportHeight)
		m.viewport.SetContent("")
		m.ready = true
	} else {
		m.viewport.Width = m.width - 2
		m.viewport.Height = viewportHeight
	}

	// Account for model badge width in input
	badgeWidth := lipgloss.Width(ModelBadgeStyle.Render(m.modelName))
	m.textinput.Width = m.width - badgeWidth - 10

	return m
}

// handleAgentEvent processes events from the agent during processing.
func (m Model) handleAgentEvent(msg agentEventMsg) Model {
	switch msg.event.Type {
	case "thinking":
		// Clear streaming buffer when starting new thinking
		if m.streamingBuffer != "" {
			m.streamingBuffer = ""
		}
		m.logs = append(m.logs, logEntry{Type: "thinking", Content: msg.event.Content})
		m.status = "thinking"

	case "streaming":
		// Append chunk to streaming buffer and update display
		m.streamingBuffer += msg.event.Content
		m.status = "streaming"
		// Update or add streaming log entry
		if len(m.logs) > 0 && m.logs[len(m.logs)-1].Type == "streaming" {
			m.logs[len(m.logs)-1].Content = m.streamingBuffer
		} else {
			m.logs = append(m.logs, logEntry{Type: "streaming", Content: m.streamingBuffer})
		}

	case "tool_call":
		// Clear streaming when tool is called
		m.streamingBuffer = ""
		m.logs = append(m.logs, logEntry{Type: "tool", Content: msg.event.Content})
		m.status = "tool"
		m.currentTool = msg.event.Content

	case "observation":
		m.logs = append(m.logs, logEntry{Type: "observation", Content: msg.event.Content})
		m.status = "thinking"
		m.currentTool = ""

	case "answer":
		// Replace streaming entry with final response if exists
		if len(m.logs) > 0 && m.logs[len(m.logs)-1].Type == "streaming" {
			m.logs[len(m.logs)-1] = logEntry{Type: "response", Content: msg.event.Content}
		} else {
			m.logs = append(m.logs, logEntry{Type: "response", Content: msg.event.Content})
		}
		m.streamingBuffer = ""
		m.status = "idle"

	case "error":
		m.logs = append(m.logs, logEntry{Type: "error", Content: msg.event.Content})
		m.streamingBuffer = ""
		m.status = "idle"
	}

	m.updateViewportContent()
	return m
}

// handleAgentDone processes the completion of agent processing.
func (m Model) handleAgentDone(msg agentDoneMsg) Model {
	m.thinking = false
	m.status = "idle"
	m.currentTool = ""
	if msg.err != nil {
		m.logs = append(m.logs, logEntry{Type: "error", Content: msg.err.Error()})
	}
	m.updateViewportContent()
	return m
}
