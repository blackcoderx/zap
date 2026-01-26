package tui

import (
	"fmt"
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

	// In confirmation mode, show the diff view
	if m.confirmationMode && m.pendingConfirmation != nil {
		content.WriteString(m.renderConfirmationView())
	} else {
		for _, entry := range m.logs {
			line := m.formatLogEntry(entry)
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	// Check if we were at the bottom before updating
	atBottom := m.viewport.AtBottom()

	m.viewport.SetContent(content.String())

	// Only auto-scroll to bottom if we were already at the bottom
	// This allows users to scroll up and read history
	if atBottom || m.thinking || m.confirmationMode {
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
	// Special footer for confirmation mode
	if m.confirmationMode {
		return m.renderConfirmationFooter()
	}

	var left string
	var right string

	// Left Side: Status or Model Info
	if m.thinking {
		// Active state: show tool usage if available
		if m.totalCalls > 0 {
			left = m.spinner.View() + " " + m.renderToolUsage()
		} else {
			left = m.spinner.View() + " " + ShortcutKeyStyle.Render("esc") + ShortcutDescStyle.Render(" interrupt")
		}
	} else {
		// Idle state: Zap [Model] Console
		left = FooterAppNameStyle.Render("Zap") + FooterModelStyle.Render(m.modelName) + FooterInfoStyle.Render("Console")
	}

	// Right Side: Hints
	var parts []string
	if m.thinking {
		// Show total usage and interrupt hint when thinking
		parts = append(parts, ShortcutKeyStyle.Render("esc")+ShortcutDescStyle.Render(" interrupt"))
	} else {
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

// renderToolUsage renders the current tool usage statistics
func (m Model) renderToolUsage() string {
	var parts []string

	// Show last tool usage with color coding
	if m.lastToolName != "" && m.lastToolLimit > 0 {
		percent := (m.lastToolCount * 100) / m.lastToolLimit
		usageStr := fmt.Sprintf("%s:%d/%d", m.lastToolName, m.lastToolCount, m.lastToolLimit)

		var styled string
		if percent >= 90 {
			styled = ToolUsageCriticalStyle.Render(usageStr)
		} else if percent >= 70 {
			styled = ToolUsageWarningStyle.Render(usageStr)
		} else {
			styled = ToolUsageNormalStyle.Render(usageStr)
		}
		parts = append(parts, styled)
	}

	// Show total usage
	if m.totalLimit > 0 {
		totalPercent := (m.totalCalls * 100) / m.totalLimit
		totalStr := fmt.Sprintf("total:%d/%d", m.totalCalls, m.totalLimit)

		var styled string
		if totalPercent >= 90 {
			styled = ToolUsageCriticalStyle.Render(totalStr)
		} else if totalPercent >= 70 {
			styled = ToolUsageWarningStyle.Render(totalStr)
		} else {
			styled = TotalUsageStyle.Render(totalStr)
		}
		parts = append(parts, styled)
	}

	if len(parts) == 0 {
		return ShortcutDescStyle.Render("working...")
	}

	return strings.Join(parts, " ")
}

// lipglossWidth calculates the width of a styled string.
func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}

// renderConfirmationView renders the file write confirmation dialog with colored diff.
func (m Model) renderConfirmationView() string {
	c := m.pendingConfirmation
	if c == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString("\n")
	sb.WriteString(ConfirmHeaderStyle.Render("  File Write Confirmation"))
	sb.WriteString("\n\n")

	// File path
	if c.IsNewFile {
		sb.WriteString(ConfirmPathStyle.Render(fmt.Sprintf("  Creating: %s", c.FilePath)))
	} else {
		sb.WriteString(ConfirmPathStyle.Render(fmt.Sprintf("  Modifying: %s", c.FilePath)))
	}
	sb.WriteString("\n\n")

	// Colored diff
	sb.WriteString(m.renderColoredDiff(c.Diff))
	sb.WriteString("\n")

	return sb.String()
}

// renderColoredDiff applies syntax highlighting to a unified diff.
func (m Model) renderColoredDiff(diff string) string {
	if diff == "" {
		return DiffContextStyle.Render("  (no changes)")
	}

	var sb strings.Builder
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		var styledLine string

		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// File headers (yellow/bold)
			styledLine = DiffHeaderStyle.Render("  " + line)
		case strings.HasPrefix(line, "@@"):
			// Hunk headers (blue)
			styledLine = DiffHunkStyle.Render("  " + line)
		case strings.HasPrefix(line, "+"):
			// Added lines (green)
			styledLine = DiffAddStyle.Render("  " + line)
		case strings.HasPrefix(line, "-"):
			// Removed lines (red)
			styledLine = DiffRemoveStyle.Render("  " + line)
		default:
			// Context lines (dimmed)
			styledLine = DiffContextStyle.Render("  " + line)
		}

		sb.WriteString(styledLine)
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderConfirmationFooter renders the footer with confirmation prompt.
func (m Model) renderConfirmationFooter() string {
	left := ConfirmHeaderStyle.Render("Apply changes?")

	right := ShortcutKeyStyle.Render("y") + ShortcutDescStyle.Render(" approve") +
		"    " +
		ShortcutKeyStyle.Render("n") + ShortcutDescStyle.Render(" reject") +
		"    " +
		ShortcutKeyStyle.Render("pgup/pgdown") + ShortcutDescStyle.Render(" scroll")

	// Calculate spacing
	w := m.width
	gap := w - lipglossWidth(left) - lipglossWidth(right)
	if gap < 2 {
		gap = 2
	}

	return FooterStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}
