package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Minimal color palette
var (
	DimColor    = lipgloss.Color("#6c6c6c")
	TextColor   = lipgloss.Color("#e0e0e0")
	AccentColor = lipgloss.Color("#7aa2f7")
	ErrorColor  = lipgloss.Color("#f7768e")
	ToolColor   = lipgloss.Color("#9ece6a")
)

// Log entry styles
var (
	UserStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	ThinkingStyle = lipgloss.NewStyle().
			Foreground(DimColor).
			Italic(true)

	ToolStyle = lipgloss.NewStyle().
			Foreground(ToolColor)

	ObservationStyle = lipgloss.NewStyle().
				Foreground(DimColor)

	ResponseStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor)

	PromptStyle = lipgloss.NewStyle().
			Foreground(AccentColor)

	HelpStyle = lipgloss.NewStyle().
			Foreground(DimColor)
)

// Log prefixes (Claude Code style)
const (
	UserPrefix        = "> "
	ThinkingPrefix    = "  thinking "
	ToolPrefix        = "  tool "
	ObservationPrefix = "  result "
	ErrorPrefix       = "  error "
)
