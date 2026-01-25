package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Minimal color palette
var (
	// USER: Adjust these colors to change the theme
	DimColor     = lipgloss.Color("#6c6c6c")
	TextColor    = lipgloss.Color("#e0e0e0")
	AccentColor  = lipgloss.Color("#7aa2f7") // The blue cursor/spinner color
	ErrorColor   = lipgloss.Color("#f7768e")
	ToolColor    = lipgloss.Color("#9ece6a")
	MutedColor   = lipgloss.Color("#545454")
	SuccessColor = lipgloss.Color("#73daca")
	WarningColor = lipgloss.Color("#e0af68") // Yellow/orange for warnings

	// OpenCode-style colors
	UserMessageBg = lipgloss.Color("#2a2a2a") // Gray background for user messages
	InputAreaBg   = lipgloss.Color("#2a2a2a") // Matches user messages
	FooterBg      = lipgloss.Color("#1a1a1a") // Darker footer
	ModelBadgeBg  = lipgloss.Color("#565f89") // Model name badge
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

	// Status line styles
	StatusIdleStyle = lipgloss.NewStyle().
			Foreground(DimColor)

	StatusActiveStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true)

	StatusToolStyle = lipgloss.NewStyle().
			Foreground(ToolColor).
			Bold(true)

	// Separator style
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	// Shortcut key style
	ShortcutKeyStyle = lipgloss.NewStyle().
				Foreground(AccentColor)

	ShortcutDescStyle = lipgloss.NewStyle().
				Foreground(DimColor)

	// Footer specific styles (OpenCode style)
	FooterAppNameStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true).
				PaddingRight(1)

	FooterModelStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				PaddingRight(1)

	FooterInfoStyle = lipgloss.NewStyle().
			Foreground(DimColor)
)

// OpenCode-style message block styles
var (
	// User message: blue left border + gray background
	UserMessageStyle = lipgloss.NewStyle().
				Background(UserMessageBg).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(AccentColor).
				BorderLeft(true).
				BorderTop(false).
				BorderRight(true).
				BorderBottom(false).
				Padding(1, 2).
				Margin(1, 0)

	// Tool calls: dimmed with circle prefix
	ToolCallStyle = lipgloss.NewStyle().
			Foreground(DimColor)

	// Agent messages: plain text
	AgentMessageStyle = lipgloss.NewStyle().
				Foreground(TextColor)

	// Input area: matches user message style
	// USER: This controls the input box style (where you type)
	// Input area: matches user message style
	InputAreaStyle = lipgloss.NewStyle().
			Background(InputAreaBg).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(AccentColor).
			BorderLeft(true). // The blue vertical bar on the left
			BorderTop(false).
			BorderRight(true).
			BorderBottom(false).
			Padding(1, 2).
			Margin(1, 1, 1, 1) // USER: Set to 0 to remove spacing around the input box

	// USER: This controls the footer bar style (bottom row)
	FooterStyle = lipgloss.NewStyle().
			Background(FooterBg).
			Foreground(DimColor).
			Padding(0, 0) // USER: Padding inside the footer bar

	// Model badge
	ModelBadgeStyle = lipgloss.NewStyle().
			Background(ModelBadgeBg).
			Foreground(TextColor).
			Padding(0, 1)
)

// Log prefixes (Claude Code style - kept for compatibility)
const (
	// UserPrefix        = "> "
	ThinkingPrefix    = "  thinking "
	ToolPrefix        = "  tool "
	ObservationPrefix = "  result "
	ErrorPrefix       = "  error "
	Separator         = "───"

	// OpenCode-style prefix
	ToolCallPrefix = "○ " // Circle prefix for tool calls
)

// Tool usage display styles
var (
	// Normal usage (green)
	ToolUsageNormalStyle = lipgloss.NewStyle().
				Foreground(ToolColor)

	// Warning usage (70-89% - yellow)
	ToolUsageWarningStyle = lipgloss.NewStyle().
				Foreground(WarningColor)

	// Critical usage (90%+ - red)
	ToolUsageCriticalStyle = lipgloss.NewStyle().
				Foreground(ErrorColor)

	// Tool name in usage display
	ToolUsageNameStyle = lipgloss.NewStyle().
				Foreground(DimColor)

	// Total usage style
	TotalUsageStyle = lipgloss.NewStyle().
			Foreground(AccentColor)
)
