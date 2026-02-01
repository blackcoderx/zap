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
	historyMu    sync.RWMutex  // Protects access to history slice
	lastResponse interface{}   // Store last tool response for chaining

	// Per-tool call limiting
	toolLimits   map[string]int // max calls per tool per session
	toolCounts   map[string]int // current session call counts
	countersMu   sync.Mutex     // Protects access to toolCounts and totalCalls
	defaultLimit int            // fallback limit for tools without specific limit
	totalLimit   int            // safety cap on total tool calls per session
	totalCalls   int            // current total tool calls in session

	// History management
	maxHistory int // maximum number of messages to keep in history (0 = unlimited)

	// User's API framework (gin, fastapi, express, etc.)
	framework string

	// Persistent memory across sessions
	memoryStore *MemoryStore
}

// Default limits for tool calls and history management.
const (
	DefaultToolCallLimit = 50  // Default max calls per tool per session
	DefaultTotalLimit    = 200 // Safety cap on total tool calls per session
	DefaultMaxHistory    = 100 // Default max messages to keep in history
)

// NewAgent creates a new ZAP agent with the given LLM client.
// The agent is initialized with default tool limits:
//   - Default per-tool limit: 50 calls
//   - Total limit: 200 calls per session
//   - Max history: 100 messages
func NewAgent(llmClient *llm.OllamaClient) *Agent {
	return &Agent{
		llmClient:    llmClient,
		tools:        make(map[string]Tool),
		history:      []llm.Message{},
		lastResponse: nil,
		toolLimits:   make(map[string]int),
		toolCounts:   make(map[string]int),
		defaultLimit: DefaultToolCallLimit,
		totalLimit:   DefaultTotalLimit,
		totalCalls:   0,
		maxHistory:   DefaultMaxHistory,
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
// This method is thread-safe.
func (a *Agent) ResetToolCounts() {
	a.countersMu.Lock()
	defer a.countersMu.Unlock()
	a.toolCounts = make(map[string]int)
	a.totalCalls = 0
}

// getToolLimit returns the limit for a specific tool, or the default if not set.
// Note: toolLimits is only written during setup, so no lock needed for reads.
func (a *Agent) getToolLimit(toolName string) int {
	if limit, ok := a.toolLimits[toolName]; ok {
		return limit
	}
	return a.defaultLimit
}

// isToolLimitReached checks if a tool has reached its call limit.
// This method is thread-safe.
func (a *Agent) isToolLimitReached(toolName string) bool {
	a.countersMu.Lock()
	defer a.countersMu.Unlock()
	return a.toolCounts[toolName] >= a.getToolLimit(toolName)
}

// isTotalLimitReached checks if the total call limit has been reached.
// This method is thread-safe.
func (a *Agent) isTotalLimitReached() bool {
	a.countersMu.Lock()
	defer a.countersMu.Unlock()
	return a.totalCalls >= a.totalLimit
}

// IncrementToolCount increments the call count for a specific tool.
// Returns the new count and limit for the tool.
// This method is thread-safe.
func (a *Agent) IncrementToolCount(toolName string) (count, limit int) {
	a.countersMu.Lock()
	defer a.countersMu.Unlock()
	a.toolCounts[toolName]++
	a.totalCalls++
	return a.toolCounts[toolName], a.getToolLimit(toolName)
}

// SetMaxHistory sets the maximum number of messages to keep in history.
// Set to 0 for unlimited history (not recommended for long sessions).
func (a *Agent) SetMaxHistory(max int) {
	a.maxHistory = max
}

// GetToolUsageStats returns current tool usage statistics.
// Returns a slice of stats for each used tool, plus total calls and limit.
// This method is thread-safe.
func (a *Agent) GetToolUsageStats() (stats []ToolUsageStats, totalCalls, totalLimit int) {
	a.countersMu.Lock()
	defer a.countersMu.Unlock()

	// Get all tools that have been used
	for toolName, count := range a.toolCounts {
		if count > 0 {
			limit := a.getToolLimit(toolName)
			// Use float64 to avoid potential overflow with large counts
			percent := int((float64(count) / float64(limit)) * 100)
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
// This method is thread-safe.
func (a *Agent) GetTotalUsage() (current, limit int) {
	a.countersMu.Lock()
	defer a.countersMu.Unlock()
	return a.totalCalls, a.totalLimit
}

// AppendHistory adds a message to the history and truncates if necessary.
// When maxHistory is reached, older messages are removed to make room.
// The truncation keeps the most recent messages while preserving context.
func (a *Agent) AppendHistory(msg llm.Message) {
	a.history = append(a.history, msg)
	a.truncateHistory()
}

// AppendHistoryPair adds an assistant message and observation to history atomically.
// This ensures tool call and observation stay together during truncation.
func (a *Agent) AppendHistoryPair(assistantMsg, observationMsg llm.Message) {
	a.history = append(a.history, assistantMsg, observationMsg)
	a.truncateHistory()
}

// truncateHistory removes old messages if history exceeds maxHistory.
// Keeps the most recent messages. If maxHistory is 0, no truncation occurs.
func (a *Agent) truncateHistory() {
	if a.maxHistory <= 0 {
		return // Unlimited history
	}

	if len(a.history) > a.maxHistory {
		// Calculate how many messages to remove
		// Keep at least 2 messages for context (a user message and a response)
		excess := len(a.history) - a.maxHistory
		if excess > 0 {
			// Remove from the beginning (oldest messages)
			a.history = a.history[excess:]
		}
	}
}

// getHistorySnapshot returns a copy of the current history for safe iteration.
// This method is thread-safe.
func (a *Agent) getHistorySnapshot() []llm.Message {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()
	snapshot := make([]llm.Message, len(a.history))
	copy(snapshot, a.history)
	return snapshot
}
