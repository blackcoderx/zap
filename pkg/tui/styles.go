package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	PrimaryColor   = lipgloss.Color("#FF6B9D")
	SecondaryColor = lipgloss.Color("#C792EA")
	AccentColor    = lipgloss.Color("#89DDFF")
	BgColor        = lipgloss.Color("#1E1E2E")
	TextColor      = lipgloss.Color("#CDD6F4")
	MutedColor     = lipgloss.Color("#6C7086")
	SuccessColor   = lipgloss.Color("#A6E3A1")
	ErrorColor     = lipgloss.Color("#F38BA8")
	WarningColor   = lipgloss.Color("#FAB387")

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Padding(1, 2)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true).
			Padding(0, 2, 1, 2)

	// Message styles
	UserMessageStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true).
				Padding(0, 1)

	AssistantMessageStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				Padding(0, 1)

	ThinkingStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Italic(true).
			Padding(0, 1)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true).
			Padding(0, 1)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Padding(0, 1)

	// Layout styles
	ContainerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(SecondaryColor).
			Padding(1)

	MessagesBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(MutedColor).
				Padding(1).
				Height(20)

	InputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(AccentColor).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Padding(1, 2)

	// Component styles
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor)
)
