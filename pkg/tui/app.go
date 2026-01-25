// Package tui provides the terminal user interface for ZAP.
// It uses Bubble Tea for the TUI framework with a minimal, Claude Code-inspired design.
//
// File organization:
// - app.go: Entry point (Run function)
// - model.go: Model struct and message types
// - init.go: Model initialization and tool registration
// - update.go: Event handling and state updates
// - view.go: Rendering and display logic
// - keys.go: Keyboard input handling
// - styles.go: Visual styling (colors, borders, etc.)
// - highlight.go: JSON syntax highlighting
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI application.
// This is the main entry point for the ZAP terminal interface.
func Run() error {
	m := InitialModel()
	prog := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Store program reference for goroutines to send messages
	globalProgram.Set(prog)

	_, err := prog.Run()

	// Clear program reference after run completes
	globalProgram.Set(nil)

	return err
}
