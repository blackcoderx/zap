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

	// Input area with horizontal margin
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

	// Top padding - space between terminal window and first message
	content.WriteString("\n")

	// In confirmation mode, show the diff view
	if m.confirmationMode && m.pendingConfirmation != nil {
		content.WriteString(m.renderConfirmationView())
	} else {
		for _, entry := range m.logs {
			line := m.formatLogEntry(entry)
			if line == "" {
				continue
			}
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

// boxWidth returns the shared content width for user message and input boxes.
// Both use the same value so their edges align perfectly.
func (m *Model) boxWidth() int {
	// Account for MarginLeft + border + padding on each side
	w := m.width - ContentPadLeft - ContentPadRight - 6
	if w < 40 {
		w = 40
	}
	return w
}

// formatLogEntry formats a single log entry for display.
func (m *Model) formatLogEntry(entry logEntry) string {
	pad := strings.Repeat(" ", ContentPadLeft)

	switch entry.Type {
	case "user":
		// MarginLeft/Top/Bottom are on the style — no manual pad or \n needed
		return UserMessageStyle.Width(m.boxWidth()).Render(entry.Content)

	case "thinking":
		return ""

	case "tool":
		return pad + m.formatCompactToolCall(entry)

	case "observation":
		return ""

	case "streaming":
		// MarginLeft/Top are on AgentMessageStyle
		return AgentMessageStyle.Render(entry.Content)

	case "response":
		if m.renderer != nil {
			rendered, err := m.renderer.Render(entry.Content)
			if err == nil {
				// Add left padding to each line of rendered markdown
				lines := strings.Split(strings.TrimSpace(rendered), "\n")
				for i, line := range lines {
					lines[i] = pad + line
				}
				return "\n" + strings.Join(lines, "\n")
			}
		}
		return AgentMessageStyle.Render(entry.Content)

	case "error":
		return pad + ErrorStyle.Render("  Error: "+entry.Content)

	case "separator":
		return ""

	default:
		return pad + entry.Content
	}
}

// formatCompactToolCall formats a tool call as a single compact line.
// Format: tool_name (args_summary) used/limit
func (m *Model) formatCompactToolCall(entry logEntry) string {
	// Tool name in warm orange
	name := ToolNameCompactStyle.Render(entry.Content)

	// Truncate args for compact display
	args := entry.ToolArgs
	if len(args) > 60 {
		args = args[:57] + "..."
	}
	argsDisplay := ToolArgsCompactStyle.Render("(" + args + ")")

	// Usage fraction
	var usageDisplay string
	if entry.ToolLimit > 0 {
		usageDisplay = ToolUsageCompactStyle.Render(
			fmt.Sprintf(" %d/%d", entry.ToolUsed, entry.ToolLimit),
		)
	}

	return name + " " + argsDisplay + usageDisplay
}

// formatObservationCard formats a tool observation/result in a card style.
func (m *Model) formatObservationCard(entry logEntry, contentWidth int) string {
	content := entry.Content

	// Truncate very long observations
	if len(content) > 500 {
		content = content[:400] + "\n... (truncated)"
	}

	// If contains markdown code blocks, render with glamour
	if strings.Contains(content, "```") && m.renderer != nil {
		rendered, err := m.renderer.Render(content)
		if err == nil {
			content = strings.TrimSpace(rendered)
		}
	}

	// Render in response card
	cardWidth := contentWidth - 4 // Account for card border/padding
	if cardWidth < 30 {
		cardWidth = 30
	}

	return ResponseCardStyle.Width(cardWidth).Render(content)
}

// renderStatus renders the current agent status text (without circle).
func (m Model) renderStatusText() string {
	switch m.status {
	case "thinking":
		return StatusLabelStyle.Render("thinking")
	case "streaming":
		return StatusLabelStyle.Render("streaming")
	case "tool":
		return StatusLabelStyle.Render("tool calling")
	default:
		return StatusIdleStyle.Render("ready")
	}
}

// renderAnimatedCircle renders the pulsing status circle using harmonica spring values.
func (m Model) renderAnimatedCircle() string {
	if !m.thinking {
		// Static dim circle when idle
		return lipgloss.NewStyle().Foreground(MutedColor).Render("●")
	}

	// Map spring position (0.0-1.0) to color index
	idx := int(m.animPos * float64(len(PulseColors)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(PulseColors) {
		idx = len(PulseColors) - 1
	}

	return lipgloss.NewStyle().Foreground(PulseColors[idx]).Render("●")
}

// renderInputArea renders the input area — same width as user message box.
func (m Model) renderInputArea() string {
	// MarginLeft is on InputAreaStyle — no manual pad needed
	return InputAreaStyle.Width(m.boxWidth()).Render(m.textinput.View())
}

// renderFooter renders the footer with animated circle, status, model info, and shortcuts.
func (m Model) renderFooter() string {
	// Special footer for confirmation mode
	if m.confirmationMode {
		return m.renderConfirmationFooter()
	}

	// Left side: animated circle + status + model name
	circle := m.renderAnimatedCircle()
	status := m.renderStatusText()
	modelInfo := FooterModelStyle.Render(m.modelName)

	left := circle + " " + status + "  " + modelInfo

	// Right side: keyboard shortcuts
	var parts []string
	if m.thinking {
		parts = append(parts, ShortcutKeyStyle.Render("esc")+ShortcutDescStyle.Render(" interrupt"))
	} else {
		parts = append(parts, ShortcutKeyStyle.Render("Shift + ↑↓")+ShortcutDescStyle.Render(" history"))
	}
	parts = append(parts, ShortcutKeyStyle.Render("ctrl+l")+ShortcutDescStyle.Render(" clear"))
	parts = append(parts, ShortcutKeyStyle.Render("ctrl+y")+ShortcutDescStyle.Render(" copy"))
	right := strings.Join(parts, "    ")

	// Calculate spacing between left and right
	w := m.width
	gap := w - lipglossWidth(left) - lipglossWidth(right) - 4 // Account for footer padding
	if gap < 2 {
		gap = 2
	}

	return FooterStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

// renderToolUsage renders the current tool usage statistics (used in footer during thinking)
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

	pad := strings.Repeat(" ", ContentPadLeft)
	var sb strings.Builder

	// Header
	sb.WriteString("\n")
	sb.WriteString(pad + ConfirmHeaderStyle.Render("  File Write Confirmation"))
	sb.WriteString("\n\n")

	// File path
	if c.IsNewFile {
		sb.WriteString(pad + ConfirmPathStyle.Render(fmt.Sprintf("  Creating: %s", c.FilePath)))
	} else {
		sb.WriteString(pad + ConfirmPathStyle.Render(fmt.Sprintf("  Modifying: %s", c.FilePath)))
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

	pad := strings.Repeat(" ", ContentPadLeft)
	var sb strings.Builder
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		var styledLine string

		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			styledLine = DiffHeaderStyle.Render("  " + line)
		case strings.HasPrefix(line, "@@"):
			styledLine = DiffHunkStyle.Render("  " + line)
		case strings.HasPrefix(line, "+"):
			styledLine = DiffAddStyle.Render("  " + line)
		case strings.HasPrefix(line, "-"):
			styledLine = DiffRemoveStyle.Render("  " + line)
		default:
			styledLine = DiffContextStyle.Render("  " + line)
		}

		sb.WriteString(pad + styledLine)
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

	w := m.width
	gap := w - lipglossWidth(left) - lipglossWidth(right) - 4
	if gap < 2 {
		gap = 2
	}

	return FooterStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}
