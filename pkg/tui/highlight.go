package tui

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/glamour"
)

// HighlightJSON takes a JSON string, validates it, and returns a syntax-highlighted string.
// If the input is not valid JSON, it returns the original string.
func HighlightJSON(input string) string {
	// 1. Validate if it's actually JSON
	var js interface{}
	if json.Unmarshal([]byte(input), &js) != nil {
		return input
	}

	// 2. Wrap in markdown code block
	var sb strings.Builder
	sb.WriteString("```json\n")

	// Re-encode to ensure pretty printing (indentation)
	// This makes even minified JSON readable
	pretty, err := json.MarshalIndent(js, "", "  ")
	if err == nil {
		sb.Write(pretty)
	} else {
		sb.WriteString(input)
	}

	sb.WriteString("\n```")

	// 3. Render with Glamour
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return input
	}

	out, err := renderer.Render(sb.String())
	if err != nil {
		return input
	}

	return strings.TrimSpace(out)
}
