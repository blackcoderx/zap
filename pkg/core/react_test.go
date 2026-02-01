package core

import (
	"testing"

	"github.com/blackcoderx/zap/pkg/llm"
)

// mockTool implements the Tool interface for testing
type mockTool struct {
	name        string
	description string
	params      string
	executeFunc func(args string) (string, error)
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.description }
func (m *mockTool) Parameters() string  { return m.params }
func (m *mockTool) Execute(args string) (string, error) {
	if m.executeFunc != nil {
		return m.executeFunc(args)
	}
	return "mock result", nil
}

// newTestAgent creates an agent for testing without an LLM client
func newTestAgent() *Agent {
	agent := NewAgent(nil)
	return agent
}

func TestParseResponse_FinalAnswer(t *testing.T) {
	agent := newTestAgent()

	tests := []struct {
		name           string
		response       string
		wantToolName   string
		wantToolArgs   string
		wantAnswer     string
		wantHasAnswer  bool
	}{
		{
			name:          "simple final answer",
			response:      "Final Answer: The API returned a 200 status code.",
			wantToolName:  "",
			wantAnswer:    "The API returned a 200 status code.",
			wantHasAnswer: true,
		},
		{
			name:          "final answer with multiline content",
			response:      "Final Answer: The error was caused by:\n1. Missing auth header\n2. Invalid JSON",
			wantToolName:  "",
			wantAnswer:    "The error was caused by:\n1. Missing auth header\n2. Invalid JSON",
			wantHasAnswer: true,
		},
		{
			name:          "plain text becomes final answer",
			response:      "I'll help you with that request.",
			wantToolName:  "",
			wantAnswer:    "I'll help you with that request.",
			wantHasAnswer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, toolName, _, finalAnswer := agent.parseResponse(tt.response)

			if toolName != tt.wantToolName {
				t.Errorf("toolName = %q, want %q", toolName, tt.wantToolName)
			}

			if tt.wantHasAnswer && finalAnswer != tt.wantAnswer {
				t.Errorf("finalAnswer = %q, want %q", finalAnswer, tt.wantAnswer)
			}
		})
	}
}

func TestParseResponse_ToolCalls(t *testing.T) {
	agent := newTestAgent()

	// Register a mock tool so the heuristic can find it
	agent.RegisterTool(&mockTool{name: "http_request"})
	agent.RegisterTool(&mockTool{name: "read_file"})

	tests := []struct {
		name         string
		response     string
		wantToolName string
		wantToolArgs string
	}{
		{
			name:         "ACTION format with JSON args",
			response:     `Thought: I need to make an HTTP request.\nACTION: http_request({"method": "GET", "url": "http://localhost:8000/users"})`,
			wantToolName: "http_request",
			wantToolArgs: `{"method": "GET", "url": "http://localhost:8000/users"}`,
		},
		{
			name:         "ACTION format simple args",
			response:     `ACTION: read_file({"path": "main.go"})`,
			wantToolName: "read_file",
			wantToolArgs: `{"path": "main.go"}`,
		},
		{
			name:         "heuristic detection without ACTION prefix",
			response:     `I'll read the file for you.\nread_file({"path": "config.json"})`,
			wantToolName: "read_file",
			wantToolArgs: `{"path": "config.json"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, toolName, toolArgs, _ := agent.parseResponse(tt.response)

			if toolName != tt.wantToolName {
				t.Errorf("toolName = %q, want %q", toolName, tt.wantToolName)
			}

			if toolArgs != tt.wantToolArgs {
				t.Errorf("toolArgs = %q, want %q", toolArgs, tt.wantToolArgs)
			}
		})
	}
}

func TestParseResponse_EdgeCases(t *testing.T) {
	agent := newTestAgent()

	tests := []struct {
		name           string
		response       string
		wantToolName   string
		wantHasAnswer  bool
	}{
		{
			name:          "empty response",
			response:      "",
			wantToolName:  "",
			wantHasAnswer: false, // Empty string is falsy, no final answer
		},
		{
			name:          "whitespace only",
			response:      "   \n\t  ",
			wantToolName:  "",
			wantHasAnswer: true, // Whitespace becomes the final answer
		},
		{
			name:          "malformed ACTION (no parens)",
			response:      "ACTION: http_request",
			wantToolName:  "",
			wantHasAnswer: true, // Falls through to plain text
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, toolName, _, finalAnswer := agent.parseResponse(tt.response)

			if toolName != tt.wantToolName {
				t.Errorf("toolName = %q, want %q", toolName, tt.wantToolName)
			}

			hasAnswer := finalAnswer != ""
			if hasAnswer != tt.wantHasAnswer {
				t.Errorf("hasAnswer = %v, want %v (answer: %q)", hasAnswer, tt.wantHasAnswer, finalAnswer)
			}
		})
	}
}

func TestToolLimits(t *testing.T) {
	agent := newTestAgent()
	agent.SetToolLimit("http_request", 3)

	// Verify limit via stats (starts at 0)
	_, total, _ := agent.GetToolUsageStats()
	if total != 0 {
		t.Errorf("initial total = %d, want 0", total)
	}

	// Increment to limit
	for i := 0; i < 3; i++ {
		agent.IncrementToolCount("http_request")
	}

	// Verify counts
	stats, total, _ := agent.GetToolUsageStats()
	if total != 3 {
		t.Errorf("total calls = %d, want 3", total)
	}

	if len(stats) != 1 || stats[0].Current != 3 {
		t.Errorf("stats incorrect: %+v", stats)
	}
}

func TestTotalLimit(t *testing.T) {
	agent := newTestAgent()
	agent.SetTotalLimit(5)

	// Verify initial state
	current, limit := agent.GetTotalUsage()
	if current != 0 {
		t.Errorf("initial total = %d, want 0", current)
	}
	if limit != 5 {
		t.Errorf("limit = %d, want 5", limit)
	}

	// Increment to limit
	for i := 0; i < 5; i++ {
		agent.IncrementToolCount("tool1")
	}

	// Verify final count
	current, _ = agent.GetTotalUsage()
	if current != 5 {
		t.Errorf("total after increments = %d, want 5", current)
	}
}

func TestResetToolCounts(t *testing.T) {
	agent := newTestAgent()

	// Make some calls
	agent.IncrementToolCount("http_request")
	agent.IncrementToolCount("read_file")

	// Reset
	agent.ResetToolCounts()

	// Verify reset
	_, total, _ := agent.GetToolUsageStats()
	if total != 0 {
		t.Errorf("total calls after reset = %d, want 0", total)
	}
}

func TestHistoryTruncation(t *testing.T) {
	agent := newTestAgent()
	agent.SetMaxHistory(5)

	// Add 7 messages
	for i := 0; i < 7; i++ {
		agent.AppendHistory(llm.Message{Role: "user", Content: "msg"})
	}

	// Should be truncated to 5
	history := agent.GetHistory()
	if len(history) != 5 {
		t.Errorf("history length = %d, want 5", len(history))
	}
}

func TestHistoryPairAppend(t *testing.T) {
	agent := newTestAgent()
	agent.SetMaxHistory(10)

	// Add a pair
	agent.AppendHistoryPair(
		llm.Message{Role: "assistant", Content: "calling tool"},
		llm.Message{Role: "user", Content: "Observation: result"},
	)

	history := agent.GetHistory()
	if len(history) != 2 {
		t.Errorf("history length = %d, want 2", len(history))
	}

	if history[0].Role != "assistant" {
		t.Errorf("first message role = %q, want assistant", history[0].Role)
	}

	if history[1].Role != "user" {
		t.Errorf("second message role = %q, want user", history[1].Role)
	}
}

func TestUnlimitedHistory(t *testing.T) {
	agent := newTestAgent()
	agent.SetMaxHistory(0) // Unlimited

	// Add many messages
	for i := 0; i < 200; i++ {
		agent.AppendHistory(llm.Message{Role: "user", Content: "msg"})
	}

	// Should not be truncated
	history := agent.GetHistory()
	if len(history) != 200 {
		t.Errorf("history length = %d, want 200 (unlimited)", len(history))
	}
}
