package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/blackcoderx/zap/pkg/llm"
)

// ProcessMessage handles a user message using ReAct logic.
// It runs the think-act-observe cycle until a final answer is reached or
// tool limits are exceeded. This is the blocking version without events.
func (a *Agent) ProcessMessage(input string) (string, error) {
	// Add user message to history
	a.AppendHistory(llm.Message{Role: "user", Content: input})

	// Reset tool call counters for this session
	a.ResetToolCounts()

	for {
		// Check total limit safety cap
		if a.isTotalLimitReached() {
			msg := fmt.Sprintf("I reached the maximum total tool calls (%d). Stopping to prevent runaway execution.", a.totalLimit)
			return msg, nil
		}

		// Prepare system prompt with tool descriptions
		systemPrompt := a.buildSystemPrompt()

		messages := []llm.Message{{Role: "system", Content: systemPrompt}}
		messages = append(messages, a.history...)

		// Get LLM response
		response, err := a.llmClient.Chat(messages)
		if err != nil {
			return "", fmt.Errorf("agent chat error: %w", err)
		}

		if response == "" {
			return "I received an empty response from the AI. This can happen if the model is overloaded or the request is blocked.", nil
		}

		// Parse response for thoughts and tool calls
		_, toolName, toolArgs, finalAnswer := a.parseResponse(response)

		if finalAnswer != "" && toolName == "" {
			a.AppendHistory(llm.Message{Role: "assistant", Content: response})
			return finalAnswer, nil
		}

		if toolName != "" {
			a.toolsMu.RLock()
			tool, ok := a.tools[toolName]
			a.toolsMu.RUnlock()
			if !ok {
				observation := fmt.Sprintf("Error: Tool '%s' not found.", toolName)
				a.AppendHistoryPair(
					llm.Message{Role: "assistant", Content: response},
					llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)},
				)
				continue
			}

			// Check per-tool limit
			if a.isToolLimitReached(toolName) {
				limit := a.getToolLimit(toolName)
				observation := fmt.Sprintf("Tool '%s' has reached its limit (%d calls). Use other tools or provide a final answer.", toolName, limit)
				a.AppendHistoryPair(
					llm.Message{Role: "assistant", Content: response},
					llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)},
				)
				continue
			}

			// Execute tool and increment counters (thread-safe)
			a.IncrementToolCount(toolName)

			observation, err := tool.Execute(toolArgs)
			if err != nil {
				observation = fmt.Sprintf("Error executing tool: %v", err)
			}

			// Add interaction to history
			a.AppendHistoryPair(
				llm.Message{Role: "assistant", Content: response},
				llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)},
			)
			continue
		}

		// If we get here, we have a final answer (possibly via default in parseResponse)
		a.AppendHistory(llm.Message{Role: "assistant", Content: response})
		return finalAnswer, nil
	}
}

// ProcessMessageWithEvents handles a user message and emits events for each stage.
// This enables real-time UI updates as the agent thinks, uses tools, and responds.
// Events emitted: thinking, tool_call, observation, answer, error, streaming, tool_usage, confirmation_required
// The context can be used to cancel the agent mid-processing.
func (a *Agent) ProcessMessageWithEvents(ctx context.Context, input string, callback EventCallback) (string, error) {
	// Add user message to history
	a.AppendHistory(llm.Message{Role: "user", Content: input})

	// Track turn in memory
	if a.memoryStore != nil {
		a.memoryStore.TrackTurn()
	}

	// Reset tool call counters for this session
	a.ResetToolCounts()

	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			// Continue processing
		}

		// Check total limit safety cap
		if a.isTotalLimitReached() {
			msg := fmt.Sprintf("I reached the maximum total tool calls (%d). Stopping to prevent runaway execution.", a.totalLimit)
			callback(AgentEvent{Type: "error", Content: msg})
			return msg, nil
		}

		// Get current total for display
		totalCalls, _ := a.GetTotalUsage()

		// Emit thinking event
		callback(AgentEvent{Type: "thinking", Content: fmt.Sprintf("reasoning (calls: %d)...", totalCalls)})

		// Prepare system prompt with tool descriptions
		systemPrompt := a.buildSystemPrompt()

		messages := []llm.Message{{Role: "system", Content: systemPrompt}}
		messages = append(messages, a.history...)

		// Get LLM response with streaming
		var response string
		var streamErr error

		// Stream callback emits chunks to TUI
		streamCallback := func(chunk string) {
			callback(AgentEvent{Type: "streaming", Content: chunk})
		}

		response, streamErr = a.llmClient.ChatStream(messages, streamCallback)
		if streamErr != nil {
			errorMsg := fmt.Sprintf("Connection Error: Could not talk to the AI provider.\nDetails: %v\n\nTip: Check if Ollama is running (try 'ollama serve') or check your API key.", streamErr)
			callback(AgentEvent{Type: "error", Content: errorMsg})
			return "", fmt.Errorf("agent chat error: %w", streamErr)
		}

		if response == "" {
			errorMsg := "Received an empty response from the AI. This usually happens if the model crashed or timed out."
			callback(AgentEvent{Type: "error", Content: errorMsg})
			return "I received an empty response from the AI.", nil
		}

		// Parse response for thoughts and tool calls
		thought, toolName, toolArgs, finalAnswer := a.parseResponse(response)

		// If we got a thought (and it's different from the streamed content), emit it
		if thought != "" && thought != response {
			callback(AgentEvent{Type: "thinking", Content: thought})
		}

		if finalAnswer != "" && toolName == "" {
			a.AppendHistory(llm.Message{Role: "assistant", Content: response})
			callback(AgentEvent{Type: "answer", Content: finalAnswer})
			return finalAnswer, nil
		}

		if toolName != "" {
			a.toolsMu.RLock()
			tool, ok := a.tools[toolName]
			a.toolsMu.RUnlock()
			if !ok {
				// Agent sees this error
				observation := fmt.Sprintf("System Error: Tool '%s' does not exist. Please use only available tools.", toolName)
				// User sees this error
				callback(AgentEvent{Type: "error", Content: fmt.Sprintf("The agent tried to use an unknown tool '%s'.", toolName)})

				a.AppendHistoryPair(
					llm.Message{Role: "assistant", Content: response},
					llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)},
				)
				continue
			}

			// Check per-tool limit
			if a.isToolLimitReached(toolName) {
				limit := a.getToolLimit(toolName)
				observation := fmt.Sprintf("Tool '%s' has reached its limit (%d calls). Use other tools or provide a final answer.", toolName, limit)
				callback(AgentEvent{Type: "error", Content: fmt.Sprintf("Tool '%s' limit reached (%d calls)", toolName, limit)})

				a.AppendHistoryPair(
					llm.Message{Role: "assistant", Content: response},
					llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)},
				)
				continue
			}

			// Emit tool call event with arguments
			callback(AgentEvent{Type: "tool_call", Content: toolName, ToolArgs: toolArgs})

			// Increment counters before execution (thread-safe)
			toolCount, toolLimit := a.IncrementToolCount(toolName)

			// Track tool usage in memory
			if a.memoryStore != nil {
				a.memoryStore.TrackTool(toolName)
			}

			// If tool implements ConfirmableTool, set the callback so it can emit events
			if confirmable, ok := tool.(ConfirmableTool); ok {
				confirmable.SetEventCallback(callback)
			}

			// Execute tool
			observation, err := tool.Execute(toolArgs)
			if err != nil {
				// Detailed error for the agent to self-correct
				observation = fmt.Sprintf("Tool Execution Error: %v", err)
			}

			// Emit observation event
			callback(AgentEvent{Type: "observation", Content: observation})

			// Emit tool usage event for TUI display
			stats, totalCallsNow, totalLimitNow := a.GetToolUsageStats()
			callback(AgentEvent{
				Type: "tool_usage",
				ToolUsage: &ToolUsageEvent{
					ToolName:    toolName,
					ToolCurrent: toolCount,
					ToolLimit:   toolLimit,
					TotalCalls:  totalCallsNow,
					TotalLimit:  totalLimitNow,
					AllStats:    stats,
				},
			})

			// Add interaction to history
			a.AppendHistoryPair(
				llm.Message{Role: "assistant", Content: response},
				llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)},
			)
			continue
		}

		// If we get here, we have a final answer
		a.AppendHistory(llm.Message{Role: "assistant", Content: response})
		callback(AgentEvent{Type: "answer", Content: finalAnswer})
		return finalAnswer, nil
	}
}

// parseResponse extracts structured components from an LLM response.
// Returns: thought, toolName, toolArgs, finalAnswer
// The response follows the ReAct format:
//
//	Thought: <reasoning>
//	ACTION: tool_name(<json_arguments>)
//
// or
//
//	Final Answer: <response>
//
// This parser is robust to common LLM formatting variations.
func (a *Agent) parseResponse(response string) (thought, toolName, toolArgs, finalAnswer string) {
	// Extract thought if present
	thought = extractThought(response)

	// Try multiple patterns for ACTION (case variations, with/without colon)
	toolName, toolArgs = a.extractAction(response)

	// Look for Final Answer: ... (case-insensitive)
	finalAnswer = extractFinalAnswer(response)

	// If we found a tool, clear any partial final answer that might be before the ACTION
	if toolName != "" {
		finalAnswer = ""
	}

	// Default: if no tool or final answer, treat whole response as final answer
	if toolName == "" && finalAnswer == "" {
		finalAnswer = response
	}

	return
}

// extractThought extracts the thought/reasoning section from a response.
func extractThought(response string) string {
	// Look for "Thought:" prefix (case-insensitive)
	lower := strings.ToLower(response)
	thoughtIdx := strings.Index(lower, "thought:")
	if thoughtIdx == -1 {
		return ""
	}

	// Find where thought ends (at ACTION or Final Answer or end of response)
	thoughtStart := thoughtIdx + 8 // len("thought:")
	thoughtEnd := len(response)

	// Check for ACTION or Final Answer after thought
	actionIdx := strings.Index(lower[thoughtStart:], "action")
	finalIdx := strings.Index(lower[thoughtStart:], "final answer")

	if actionIdx != -1 && (finalIdx == -1 || actionIdx < finalIdx) {
		thoughtEnd = thoughtStart + actionIdx
	} else if finalIdx != -1 {
		thoughtEnd = thoughtStart + finalIdx
	}

	return strings.TrimSpace(response[thoughtStart:thoughtEnd])
}

// extractAction extracts tool name and arguments from ACTION: format.
// Handles multiple format variations that LLMs might produce.
func (a *Agent) extractAction(response string) (toolName, toolArgs string) {
	lower := strings.ToLower(response)

	// Try different ACTION patterns
	patterns := []string{"action:", "action :", "action"}
	var actionIdx int = -1
	var patternLen int

	for _, pattern := range patterns {
		idx := strings.Index(lower, pattern)
		if idx != -1 {
			actionIdx = idx
			patternLen = len(pattern)
			break
		}
	}

	if actionIdx != -1 {
		actionPart := response[actionIdx+patternLen:]
		actionPart = strings.TrimSpace(actionPart)

		// Find the opening parenthesis
		idxOpen := strings.Index(actionPart, "(")
		if idxOpen != -1 {
			toolName = strings.TrimSpace(actionPart[:idxOpen])

			// Extract JSON arguments, handling nested braces
			toolArgs = extractJSONArgs(actionPart[idxOpen:])
		}
	}

	// Heuristic: If we didn't find ACTION format, look for raw tool calls
	if toolName == "" {
		toolName, toolArgs = a.extractRawToolCall(response)
	}

	return
}

// extractJSONArgs extracts JSON arguments from a string starting with "(".
// Properly handles nested braces and brackets.
func extractJSONArgs(s string) string {
	if len(s) == 0 || s[0] != '(' {
		return ""
	}

	// Find the JSON object inside parentheses
	depth := 0
	inString := false
	escaped := false
	start := -1
	end := -1

	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch ch {
		case '{', '[':
			if depth == 0 {
				start = i
			}
			depth++
		case '}', ']':
			depth--
			if depth == 0 && start != -1 {
				end = i + 1
				return s[start:end]
			}
		case ')':
			// End of arguments without finding JSON
			if depth == 0 {
				// Return everything between ( and )
				return strings.TrimSpace(s[1:i])
			}
		}
	}

	// Fallback: find last ) and extract
	idxClose := strings.LastIndex(s, ")")
	if idxClose > 0 {
		return strings.TrimSpace(s[1:idxClose])
	}

	return ""
}

// extractRawToolCall looks for tool calls without ACTION prefix.
// This handles cases where LLMs forget the ACTION: prefix.
func (a *Agent) extractRawToolCall(response string) (toolName, toolArgs string) {
	a.toolsMu.RLock()
	defer a.toolsMu.RUnlock()

	for name := range a.tools {
		// Look for tool_name( pattern
		pattern := name + "("
		idx := strings.Index(response, pattern)
		if idx != -1 {
			argsStart := idx + len(name)
			toolArgs = extractJSONArgs(response[argsStart:])
			if toolArgs != "" {
				return name, toolArgs
			}
		}
	}
	return "", ""
}

// extractFinalAnswer extracts the final answer from a response.
func extractFinalAnswer(response string) string {
	lower := strings.ToLower(response)

	// Try different patterns
	patterns := []struct {
		pattern string
		length  int
	}{
		{"final answer:", 13},
		{"final answer :", 14},
		{"finalanswer:", 12},
	}

	for _, p := range patterns {
		idx := strings.Index(lower, p.pattern)
		if idx != -1 {
			return strings.TrimSpace(response[idx+p.length:])
		}
	}

	return ""
}
