package tui

import (
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyMsg processes keyboard input and returns the updated model and command.
// This centralizes all key handling logic for the TUI.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit

	case "ctrl+l":
		return m.handleClearScreen()

	case "ctrl+y":
		return m.handleCopyLastResponse()

	case "ctrl+u":
		return m.handleClearInput()

	case "up":
		return m.handleHistoryUp()

	case "down":
		return m.handleHistoryDown()

	case "enter":
		return m.handleEnter()

	case "pgup", "pgdown", "home", "end":
		return m.handleViewportScroll(msg)

	default:
		return m, nil
	}
}

// handleClearScreen clears all logs and resets the streaming buffer.
func (m Model) handleClearScreen() (Model, tea.Cmd) {
	m.logs = []logEntry{}
	m.streamingBuffer = ""
	m.updateViewportContent()
	return m, nil
}

// handleCopyLastResponse copies the last agent response to clipboard.
func (m Model) handleCopyLastResponse() (Model, tea.Cmd) {
	var lastResponse string
	for i := len(m.logs) - 1; i >= 0; i-- {
		if m.logs[i].Type == "response" {
			lastResponse = m.logs[i].Content
			break
		}
	}
	if lastResponse != "" {
		_ = clipboard.WriteAll(lastResponse)
	}
	return m, nil
}

// handleClearInput clears the current input and resets history navigation.
func (m Model) handleClearInput() (Model, tea.Cmd) {
	m.textinput.SetValue("")
	m.historyIdx = -1
	return m, nil
}

// handleHistoryUp navigates backwards through input history.
func (m Model) handleHistoryUp() (Model, tea.Cmd) {
	if m.thinking || len(m.inputHistory) == 0 {
		return m, nil
	}

	if m.historyIdx == -1 {
		// Save current input before navigating
		m.savedInput = m.textinput.Value()
		m.historyIdx = len(m.inputHistory) - 1
	} else if m.historyIdx > 0 {
		m.historyIdx--
	}

	m.textinput.SetValue(m.inputHistory[m.historyIdx])
	m.textinput.CursorEnd()
	return m, nil
}

// handleHistoryDown navigates forwards through input history.
func (m Model) handleHistoryDown() (Model, tea.Cmd) {
	if m.thinking || m.historyIdx == -1 {
		return m, nil
	}

	if m.historyIdx < len(m.inputHistory)-1 {
		m.historyIdx++
		m.textinput.SetValue(m.inputHistory[m.historyIdx])
	} else {
		// Return to saved input
		m.historyIdx = -1
		m.textinput.SetValue(m.savedInput)
	}

	m.textinput.CursorEnd()
	return m, nil
}

// handleEnter processes the enter key to send a message.
func (m Model) handleEnter() (Model, tea.Cmd) {
	if m.textinput.Value() == "" || m.thinking {
		return m, nil
	}

	userInput := strings.TrimSpace(m.textinput.Value())
	if userInput == "" {
		return m, nil
	}

	// Add separator if there are previous logs
	if len(m.logs) > 0 {
		m.logs = append(m.logs, logEntry{Type: "separator", Content: ""})
	}
	m.logs = append(m.logs, logEntry{Type: "user", Content: userInput})

	// Add to history
	m.inputHistory = append(m.inputHistory, userInput)
	m.historyIdx = -1
	m.savedInput = ""

	// Reset input and start processing
	m.textinput.SetValue("")
	m.thinking = true
	m.status = "thinking"
	m.streamingBuffer = ""
	m.updateViewportContent()

	return m, tea.Batch(
		m.spinner.Tick,
		runAgentAsync(m.agent, userInput),
	)
}

// handleViewportScroll passes scroll events to the viewport.
func (m Model) handleViewportScroll(msg tea.KeyMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}
