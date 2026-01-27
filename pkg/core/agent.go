// Package core provides the central agent logic, tool management, and ReAct loop
// implementation for the ZAP API debugging assistant.
package core

import (
	"fmt"
	"sync"

	"github.com/blackcoderx/zap/pkg/llm"
)

// Agent represents the ZAP AI agent that processes user messages,
// executes tools, and provides API debugging assistance.
type Agent struct {
	llmClient    *llm.OllamaClient
	tools        map[string]Tool
	toolsMu      sync.RWMutex // Protects access to tools map
	history      []llm.Message
	lastResponse interface{} // Store last tool response for chaining

	// Per-tool call limiting
	toolLimits   map[string]int // max calls per tool per session
	toolCounts   map[string]int // current session call counts
	defaultLimit int            // fallback limit for tools without specific limit
	totalLimit   int            // safety cap on total tool calls per session
	totalCalls   int            // current total tool calls in session

	// User's API framework (gin, fastapi, express, etc.)
	framework string

	// Persistent memory across sessions
	memoryStore *MemoryStore
}

// NewAgent creates a new ZAP agent with the given LLM client.
// The agent is initialized with default tool limits:
//   - Default per-tool limit: 50 calls
//   - Total limit: 200 calls per session
func NewAgent(llmClient *llm.OllamaClient) *Agent {
	return &Agent{
		llmClient:    llmClient,
		tools:        make(map[string]Tool),
		history:      []llm.Message{},
		lastResponse: nil,
		toolLimits:   make(map[string]int),
		toolCounts:   make(map[string]int),
		defaultLimit: 50,  // Default: 50 calls per tool
		totalLimit:   200, // Safety cap: 200 total calls per session
		totalCalls:   0,
	}
}

// RegisterTool adds a tool to the agent's arsenal.
// This method is thread-safe.
func (a *Agent) RegisterTool(tool Tool) {
	a.toolsMu.Lock()
	defer a.toolsMu.Unlock()
	a.tools[tool.Name()] = tool
}

// ExecuteTool executes a tool by name (used by retry tool).
// This method is thread-safe for looking up the tool.
func (a *Agent) ExecuteTool(toolName string, args string) (string, error) {
	a.toolsMu.RLock()
	tool, ok := a.tools[toolName]
	a.toolsMu.RUnlock()
	if !ok {
		return "", fmt.Errorf("tool '%s' not found", toolName)
	}
	return tool.Execute(args)
}

// SetLastResponse stores the last response from a tool for chaining.
func (a *Agent) SetLastResponse(response interface{}) {
	a.lastResponse = response
}

// SetToolLimit sets the maximum number of calls allowed for a specific tool per session.
func (a *Agent) SetToolLimit(toolName string, limit int) {
	a.toolLimits[toolName] = limit
}

// SetDefaultLimit sets the fallback limit for tools without a specific limit.
func (a *Agent) SetDefaultLimit(limit int) {
	a.defaultLimit = limit
}

// SetTotalLimit sets the safety cap on total tool calls per session.
func (a *Agent) SetTotalLimit(limit int) {
	a.totalLimit = limit
}

// SetFramework sets the user's API framework for context-aware assistance.
// Supported frameworks include: gin, echo, chi, fiber, fastapi, flask, django,
// express, nestjs, hono, spring, laravel, rails, actix, axum, other.
func (a *Agent) SetFramework(framework string) {
	a.framework = framework
}

// GetFramework returns the configured API framework.
func (a *Agent) GetFramework() string {
	return a.framework
}

// SetMemoryStore sets the persistent memory store for the agent.
func (a *Agent) SetMemoryStore(store *MemoryStore) {
	a.memoryStore = store
}

// GetHistory returns the agent's conversation history.
func (a *Agent) GetHistory() []llm.Message {
	return a.history
}

// ResetToolCounts resets all tool call counters.
// This should be called at the start of each new message.
func (a *Agent) ResetToolCounts() {
	a.toolCounts = make(map[string]int)
	a.totalCalls = 0
}

// getToolLimit returns the limit for a specific tool, or the default if not set.
func (a *Agent) getToolLimit(toolName string) int {
	if limit, ok := a.toolLimits[toolName]; ok {
		return limit
	}
	return a.defaultLimit
}

// isToolLimitReached checks if a tool has reached its call limit.
func (a *Agent) isToolLimitReached(toolName string) bool {
	return a.toolCounts[toolName] >= a.getToolLimit(toolName)
}

// isTotalLimitReached checks if the total call limit has been reached.
func (a *Agent) isTotalLimitReached() bool {
	return a.totalCalls >= a.totalLimit
}

// GetToolUsageStats returns current tool usage statistics.
// Returns a slice of stats for each used tool, plus total calls and limit.
func (a *Agent) GetToolUsageStats() (stats []ToolUsageStats, totalCalls, totalLimit int) {
	// Get all tools that have been used
	for toolName, count := range a.toolCounts {
		if count > 0 {
			limit := a.getToolLimit(toolName)
			percent := (count * 100) / limit
			if percent > 100 {
				percent = 100
			}
			stats = append(stats, ToolUsageStats{
				Name:    toolName,
				Current: count,
				Limit:   limit,
				Percent: percent,
			})
		}
	}
	return stats, a.totalCalls, a.totalLimit
}

// GetTotalUsage returns total calls and limit.
func (a *Agent) GetTotalUsage() (current, limit int) {
	return a.totalCalls, a.totalLimit
}
