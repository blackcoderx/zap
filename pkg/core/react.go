package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackcoderx/zap/pkg/llm"
)

// ProcessMessage handles a user message using ReAct logic.
// It runs the think-act-observe cycle until a final answer is reached or
// tool limits are exceeded. This is the blocking version without events.
func (a *Agent) ProcessMessage(input string) (string, error) {
	fmt.Fprintf(os.Stderr, "AGENT: Processing user input: %s\n", input)

	// Add user message to history
	a.history = append(a.history, llm.Message{Role: "user", Content: input})

	// Reset tool call counters for this session
	a.ResetToolCounts()

	for {
		fmt.Fprintf(os.Stderr, "AGENT: Total tool calls: %d\n", a.totalCalls)

		// Check total limit safety cap
		if a.isTotalLimitReached() {
			msg := fmt.Sprintf("I reached the maximum total tool calls (%d). Stopping to prevent runaway execution.", a.totalLimit)
			fmt.Fprintf(os.Stderr, "AGENT: %s\n", msg)
			return msg, nil
		}

		// Prepare system prompt with tool descriptions
		systemPrompt := a.buildSystemPrompt()

		messages := []llm.Message{{Role: "system", Content: systemPrompt}}
		messages = append(messages, a.history...)

		// Get LLM response
		response, err := a.llmClient.Chat(messages)
		if err != nil {
			fmt.Fprintf(os.Stderr, "AGENT: Chat error: %v\n", err)
			return "", fmt.Errorf("agent chat error: %w", err)
		}

		fmt.Fprintf(os.Stderr, "AGENT: LLM Response: [%s]\n", response)

		if response == "" {
			fmt.Fprintf(os.Stderr, "AGENT: Empty response from LLM\n")
			return "I received an empty response from the AI. This can happen if the model is overloaded or the request is blocked.", nil
		}

		// Parse response for thoughts and tool calls
		_, toolName, toolArgs, finalAnswer := a.parseResponse(response)
		fmt.Fprintf(os.Stderr, "AGENT: Parsed -> toolName: %s, finalAnswer: %s\n", toolName, finalAnswer)

		if finalAnswer != "" && toolName == "" {
			a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
			return finalAnswer, nil
		}

		if toolName != "" {
			a.toolsMu.RLock()
			tool, ok := a.tools[toolName]
			a.toolsMu.RUnlock()
			if !ok {
				observation := fmt.Sprintf("Error: Tool '%s' not found.", toolName)
				fmt.Fprintf(os.Stderr, "AGENT: %s\n", observation)
				a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
				a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
				continue
			}

			// Check per-tool limit
			if a.isToolLimitReached(toolName) {
				limit := a.getToolLimit(toolName)
				observation := fmt.Sprintf("Tool '%s' has reached its limit (%d calls). Use other tools or provide a final answer.", toolName, limit)
				fmt.Fprintf(os.Stderr, "AGENT: %s\n", observation)
				a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
				a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
				continue
			}

			// Execute tool and increment counters
			fmt.Fprintf(os.Stderr, "AGENT: Executing tool %s with args %s\n", toolName, toolArgs)
			a.toolCounts[toolName]++
			a.totalCalls++

			observation, err := tool.Execute(toolArgs)
			if err != nil {
				observation = fmt.Sprintf("Error executing tool: %v", err)
			}
			fmt.Fprintf(os.Stderr, "AGENT: Observation: %s\n", observation)

			// Add interaction to history
			a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
			a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
			continue
		}

		// If we get here, we have a final answer (possibly via default in parseResponse)
		a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
		return finalAnswer, nil
	}
}

// ProcessMessageWithEvents handles a user message and emits events for each stage.
// This enables real-time UI updates as the agent thinks, uses tools, and responds.
// Events emitted: thinking, tool_call, observation, answer, error, streaming, tool_usage, confirmation_required
func (a *Agent) ProcessMessageWithEvents(input string, callback EventCallback) (string, error) {
	// Add user message to history
	a.history = append(a.history, llm.Message{Role: "user", Content: input})

	// Track turn in memory
	if a.memoryStore != nil {
		a.memoryStore.TrackTurn()
	}

	// Reset tool call counters for this session
	a.ResetToolCounts()

	for {
		// Check total limit safety cap
		if a.isTotalLimitReached() {
			msg := fmt.Sprintf("I reached the maximum total tool calls (%d). Stopping to prevent runaway execution.", a.totalLimit)
			callback(AgentEvent{Type: "error", Content: msg})
			return msg, nil
		}

		// Emit thinking event
		callback(AgentEvent{Type: "thinking", Content: fmt.Sprintf("reasoning (calls: %d)...", a.totalCalls)})

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
			a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
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

				a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
				a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
				continue
			}

			// Check per-tool limit
			if a.isToolLimitReached(toolName) {
				limit := a.getToolLimit(toolName)
				observation := fmt.Sprintf("Tool '%s' has reached its limit (%d calls). Use other tools or provide a final answer.", toolName, limit)
				callback(AgentEvent{Type: "error", Content: fmt.Sprintf("Tool '%s' limit reached (%d calls)", toolName, limit)})

				a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
				a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
				continue
			}

			// Emit tool call event with arguments
			callback(AgentEvent{Type: "tool_call", Content: toolName, ToolArgs: toolArgs})

			// Increment counters before execution
			a.toolCounts[toolName]++
			a.totalCalls++

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
			stats, totalCalls, totalLimit := a.GetToolUsageStats()
			callback(AgentEvent{
				Type: "tool_usage",
				ToolUsage: &ToolUsageEvent{
					ToolName:    toolName,
					ToolCurrent: a.toolCounts[toolName],
					ToolLimit:   a.getToolLimit(toolName),
					TotalCalls:  totalCalls,
					TotalLimit:  totalLimit,
					AllStats:    stats,
				},
			})

			// Add interaction to history
			a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
			a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
			continue
		}

		// If we get here, we have a final answer
		a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
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
func (a *Agent) parseResponse(response string) (thought, toolName, toolArgs, finalAnswer string) {
	// Look for ACTION: ToolName(...)
	actionIdx := strings.Index(response, "ACTION:")
	if actionIdx != -1 {
		actionPart := response[actionIdx+7:]
		actionPart = strings.TrimSpace(actionPart)

		idxOpen := strings.Index(actionPart, "(")
		idxClose := strings.LastIndex(actionPart, ")")

		if idxOpen != -1 && idxClose != -1 {
			toolName = strings.TrimSpace(actionPart[:idxOpen])
			toolArgs = actionPart[idxOpen+1 : idxClose]
		}
	}

	// Look for Final Answer: ...
	finalIdx := strings.Index(response, "Final Answer:")
	if finalIdx != -1 {
		finalAnswer = strings.TrimSpace(response[finalIdx+13:])
	}

	// Heuristic: If we see a tool name but no ACTION prefix (Ollama sometimes does this)
	if toolName == "" {
		for name := range a.tools {
			if strings.Contains(response, name+"(") {
				idxOpen := strings.Index(response, name+"(")
				idxClose := strings.LastIndex(response, ")")
				if idxOpen != -1 && idxClose > idxOpen {
					toolName = name
					toolArgs = response[idxOpen+len(name)+1 : idxClose]
				}
			}
		}
	}

	// Default: if no tool or final answer, just return whole response
	if toolName == "" && finalAnswer == "" {
		finalAnswer = response
	}

	return
}
