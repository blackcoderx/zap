package tui

import (
	"context"
	"time"

	"github.com/blackcoderx/zap/pkg/core"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// animTick returns a command that sends animation tick messages at ~30fps.
func animTick() tea.Cmd {
	return tea.Tick(time.Second/30, func(t time.Time) tea.Msg {
		return animTickMsg(t)
	})
}

// runAgentAsync starts the agent in a goroutine and sends events via the program.
// This allows the TUI to remain responsive while the agent processes the request.
func runAgentAsync(agent *core.Agent, input string) tea.Cmd {
	return func() tea.Msg {
		// Create a cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Send the cancel function to the model
		globalProgram.Send(agentCancelMsg{cancel: cancel})

		// Run agent in goroutine so we can send intermediate events
		go func() {
			callback := func(event core.AgentEvent) {
				globalProgram.Send(agentEventMsg{event: event})
			}

			_, err := agent.ProcessMessageWithEvents(ctx, input, callback)
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

	case agentCancelMsg:
		m.cancelAgent = msg.cancel

	case agentDoneMsg:
		m = m.handleAgentDone(msg)

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case animTickMsg:
		m = m.handleAnimTick()
		cmds = append(cmds, animTick())

	case confirmationTimeoutMsg:
		// Handle confirmation timeout - exit confirmation mode and show error
		if m.confirmationMode {
			m.confirmationMode = false
			m.pendingConfirmation = nil
			m.logs = append(m.logs, logEntry{
				Type:    "error",
				Content: "File confirmation timed out (5 minutes). The file was not modified.",
			})
			m.updateViewportContent()
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

// handleAnimTick updates the harmonica spring animation state.
func (m Model) handleAnimTick() Model {
	if !m.thinking {
		return m
	}

	// Update spring physics
	m.animPos, m.animVel = m.animSpring.Update(m.animPos, m.animVel, m.animTarget)

	// Oscillate: flip target when position gets close
	if m.animTarget > 0.5 && m.animPos > 0.85 {
		m.animTarget = 0.0
	} else if m.animTarget < 0.5 && m.animPos < 0.15 {
		m.animTarget = 1.0
	}

	return m
}

// handleWindowResize adjusts the layout when the terminal is resized.
func (m Model) handleWindowResize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height

	// Calculate viewport dimensions accounting for padding
	inputHeight := 1
	footerHeight := 1
	margins := 3

	viewportHeight := m.height - inputHeight - footerHeight - margins
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	viewportWidth := m.width - 2
	if viewportWidth < 40 {
		viewportWidth = 40
	}

	if !m.ready {
		m.viewport = viewport.New(viewportWidth, viewportHeight)
		m.viewport.SetContent("")
		m.ready = true
	} else {
		m.viewport.Width = viewportWidth
		m.viewport.Height = viewportHeight
	}

	// Update text input width
	badgeWidth := lipgloss.Width(ModelBadgeStyle.Render(m.modelName))
	m.textinput.Width = m.width - badgeWidth - 10

	// Update glamour renderer for new width
	m.updateGlamourWidth(m.width - ContentPadLeft - ContentPadRight - 10)

	return m
}

// handleAgentEvent processes events from the agent during processing.
func (m Model) handleAgentEvent(msg agentEventMsg) Model {
	// Ignore events if agent was cancelled
	if !m.thinking {
		return m
	}

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
		// Remove the streaming log entry (which contains the raw "ACTION: ..." text)
		if len(m.logs) > 0 && m.logs[len(m.logs)-1].Type == "streaming" {
			m.logs = m.logs[:len(m.logs)-1]
		}
		// Record start time for timing the tool execution
		m.toolStartTime = time.Now()
		m.logs = append(m.logs, logEntry{
			Type:     "tool",
			Content:  msg.event.Content,
			ToolArgs: msg.event.ToolArgs,
		})
		m.status = "tool"
		m.currentTool = msg.event.Content

	case "observation":
		// Calculate elapsed time and update the most recent tool entry
		elapsed := time.Since(m.toolStartTime)
		for i := len(m.logs) - 1; i >= 0; i-- {
			if m.logs[i].Type == "tool" {
				m.logs[i].Duration = elapsed
				break
			}
		}
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

	case "tool_usage":
		if msg.event.ToolUsage != nil {
			usage := msg.event.ToolUsage

			// Update the most recent tool entry with usage data
			for i := len(m.logs) - 1; i >= 0; i-- {
				if m.logs[i].Type == "tool" {
					m.logs[i].ToolUsed = usage.ToolCurrent
					m.logs[i].ToolLimit = usage.ToolLimit
					break
				}
			}

			// Update model-level tracking
			m.totalCalls = usage.TotalCalls
			m.totalLimit = usage.TotalLimit
			m.lastToolName = usage.ToolName
			m.lastToolCount = usage.ToolCurrent
			m.lastToolLimit = usage.ToolLimit

			// Convert stats to display format
			m.toolUsage = make([]ToolUsageDisplay, len(usage.AllStats))
			for i, stat := range usage.AllStats {
				m.toolUsage[i] = ToolUsageDisplay{
					Name:    stat.Name,
					Current: stat.Current,
					Limit:   stat.Limit,
					Percent: stat.Percent,
				}
			}
		}

	case "confirmation_required":
		if msg.event.FileConfirmation != nil {
			m.confirmationMode = true
			m.pendingConfirmation = msg.event.FileConfirmation
		}
	}

	m.updateViewportContent()
	return m
}

// handleAgentDone processes the completion of agent processing.
func (m Model) handleAgentDone(msg agentDoneMsg) Model {
	// If already not thinking (was cancelled), just clean up
	wasCancelled := !m.thinking

	m.thinking = false
	m.status = "idle"
	m.currentTool = ""
	m.cancelAgent = nil // Clear the cancel function

	// Reset tool usage display
	m.toolUsage = nil
	m.totalCalls = 0
	m.totalLimit = 0
	m.lastToolName = ""
	m.lastToolCount = 0
	m.lastToolLimit = 0

	// Reset animation
	m.animPos = 0.0
	m.animVel = 0.0
	m.animTarget = 1.0

	// Only show error if not cancelled and there's an actual error
	if !wasCancelled && msg.err != nil && msg.err != context.Canceled {
		m.logs = append(m.logs, logEntry{Type: "error", Content: msg.err.Error()})
	}
	m.updateViewportContent()
	return m
}
