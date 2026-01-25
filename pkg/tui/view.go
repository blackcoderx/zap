package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the entire TUI to a string.
// This is called by Bubble Tea on every update.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Viewport (messages) - no header, maximize space
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Input area
	b.WriteString(m.renderInputArea())
	b.WriteString("\n")

	// Footer
	b.WriteString(m.renderFooter())

	return b.String()
}

// updateViewportContent updates the viewport with the current log entries.
// It preserves scroll position if the user has scrolled up.
func (m *Model) updateViewportContent() {
	var content strings.Builder

	for _, entry := range m.logs {
		line := m.formatLogEntry(entry)
		content.WriteString(line)
		content.WriteString("\n")
	}

	// Check if we were at the bottom before updating
	atBottom := m.viewport.AtBottom()

	m.viewport.SetContent(content.String())

	// Only auto-scroll to bottom if we were already at the bottom
	// This allows users to scroll up and read history
	if atBottom || m.thinking {
		m.viewport.GotoBottom()
	}
}

// formatLogEntry formats a single log entry for display.
func (m *Model) formatLogEntry(entry logEntry) string {
	contentWidth := m.width - 6
	if contentWidth < 40 {
		contentWidth = 40
	}

	switch entry.Type {
	case "user":
		// Blue left border + gray background (OpenCode style)
		return UserMessageStyle.Width(contentWidth).Render(entry.Content)

	case "thinking":
		// Hide thinking entries for cleaner display
		return ""

	case "tool":
		// Circle prefix + tool name: args (dimmed)
		return ToolCallStyle.Render(ToolCallPrefix + entry.Content)

	case "observation":
		// Tool results - show in dimmed text, truncated
		content := entry.Content
		if len(content) > 500 {
			content = content[:400] + "\n... (truncated)"
		}
		// If contains markdown code blocks, render with glamour
		if strings.Contains(entry.Content, "```") && m.renderer != nil {
			rendered, err := m.renderer.Render(entry.Content)
			if err == nil {
				return strings.TrimSpace(rendered)
			}
		}
		return ToolCallStyle.Render("  " + content) // Indent results, no circle

	case "streaming":
		return AgentMessageStyle.Render(entry.Content)

	case "response":
		if m.renderer != nil {
			rendered, err := m.renderer.Render(entry.Content)
			if err == nil {
				return strings.TrimSpace(rendered)
			}
		}
		return AgentMessageStyle.Render(entry.Content)

	case "error":
		return ErrorStyle.Render("  Error: " + entry.Content)

	case "separator":
		return "" // No visible separators in OpenCode style

	default:
		return entry.Content
	}
}

// renderStatus renders the current agent status.
func (m Model) renderStatus() string {
	switch m.status {
	case "thinking":
		return StatusActiveStyle.Render(m.spinner.View() + " thinking...")
	case "streaming":
		return StatusActiveStyle.Render(m.spinner.View() + " streaming...")
	case "tool":
		return StatusToolStyle.Render(m.spinner.View() + " executing " + m.currentTool)
	default:
		return StatusIdleStyle.Render("ready")
	}
}

// renderInputArea renders the OpenCode-style input area.
func (m Model) renderInputArea() string {
	return InputAreaStyle.Width(m.width - 3).Render(m.textinput.View())
}

// renderFooter renders the footer with model info/status on left and shortcuts on right.
func (m Model) renderFooter() string {
	var left string
	var right string

	// Left Side: Status or Model Info
	if m.thinking {
		// Active state: dots + interrupt hint
		left = m.spinner.View() + " " + ShortcutKeyStyle.Render("esc") + ShortcutDescStyle.Render(" interrupt")
	} else {
		// Idle state: Zap [Model] Console
		left = FooterAppNameStyle.Render("Zap") + FooterModelStyle.Render(m.modelName) + FooterInfoStyle.Render("Console")
	}

	// Right Side: Hints
	var parts []string
	if !m.thinking {
		parts = append(parts, ShortcutKeyStyle.Render("↑↓")+ShortcutDescStyle.Render(" history"))
	}
	parts = append(parts, ShortcutKeyStyle.Render("ctrl+l")+ShortcutDescStyle.Render(" clear"))
	parts = append(parts, ShortcutKeyStyle.Render("ctrl+y")+ShortcutDescStyle.Render(" copy"))

	// Join with more spacing
	right = strings.Join(parts, "    ")

	// Calculate spacing
	w := m.width
	gap := w - lipglossWidth(left) - lipglossWidth(right)
	if gap < 2 {
		gap = 2
	}

	return FooterStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

// lipglossWidth calculates the width of a styled string.
func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}
