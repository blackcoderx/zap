package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackcoderx/zap/pkg/llm"
)

// Tool represents an agent capability
type Tool interface {
	Name() string
	Description() string
	Parameters() string
	Execute(args string) (string, error)
}

// AgentEvent represents a state change during agent processing
type AgentEvent struct {
	Type    string // "thinking", "tool_call", "observation", "answer", "error"
	Content string
}

// EventCallback is called when the agent emits an event
type EventCallback func(AgentEvent)

// Agent represents the ZAP AI agent
type Agent struct {
	llmClient *llm.OllamaClient
	tools     map[string]Tool
	history   []llm.Message
}

// NewAgent creates a new ZAP agent
func NewAgent(llmClient *llm.OllamaClient) *Agent {
	return &Agent{
		llmClient: llmClient,
		tools:     make(map[string]Tool),
		history:   []llm.Message{},
	}
}

// RegisterTool adds a tool to the agent's arsenal
func (a *Agent) RegisterTool(tool Tool) {
	a.tools[tool.Name()] = tool
}

// ProcessMessage handles a user message using ReAct logic
func (a *Agent) ProcessMessage(input string) (string, error) {
	fmt.Fprintf(os.Stderr, "AGENT: Processing user input: %s\n", input)

	// Add user message to history
	a.history = append(a.history, llm.Message{Role: "user", Content: input})

	// Max iterations to prevent infinite loops
	maxIterations := 5

	for i := 0; i < maxIterations; i++ {
		fmt.Fprintf(os.Stderr, "AGENT: Iteration %d\n", i)

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
			tool, ok := a.tools[toolName]
			if !ok {
				observation := fmt.Sprintf("Error: Tool '%s' not found.", toolName)
				fmt.Fprintf(os.Stderr, "AGENT: %s\n", observation)
				a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
				a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
				continue
			}

			// Execute tool
			fmt.Fprintf(os.Stderr, "AGENT: Executing tool %s with args %s\n", toolName, toolArgs)
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

	return "I reached the maximum number of steps without finding a final answer. Result: " + input, nil
}

// ProcessMessageWithEvents handles a user message and emits events for each stage
func (a *Agent) ProcessMessageWithEvents(input string, callback EventCallback) (string, error) {
	// Add user message to history
	a.history = append(a.history, llm.Message{Role: "user", Content: input})

	// Max iterations to prevent infinite loops
	maxIterations := 5

	for i := 0; i < maxIterations; i++ {
		// Emit thinking event
		callback(AgentEvent{Type: "thinking", Content: fmt.Sprintf("reasoning (step %d)...", i+1)})

		// Prepare system prompt with tool descriptions
		systemPrompt := a.buildSystemPrompt()

		messages := []llm.Message{{Role: "system", Content: systemPrompt}}
		messages = append(messages, a.history...)

		// Get LLM response
		response, err := a.llmClient.Chat(messages)
		if err != nil {
			callback(AgentEvent{Type: "error", Content: err.Error()})
			return "", fmt.Errorf("agent chat error: %w", err)
		}

		if response == "" {
			callback(AgentEvent{Type: "error", Content: "empty response from AI"})
			return "I received an empty response from the AI.", nil
		}

		// Parse response for thoughts and tool calls
		thought, toolName, toolArgs, finalAnswer := a.parseResponse(response)

		// If we got a thought, emit it
		if thought != "" {
			callback(AgentEvent{Type: "thinking", Content: thought})
		}

		if finalAnswer != "" && toolName == "" {
			a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
			callback(AgentEvent{Type: "answer", Content: finalAnswer})
			return finalAnswer, nil
		}

		if toolName != "" {
			tool, ok := a.tools[toolName]
			if !ok {
				observation := fmt.Sprintf("Tool '%s' not found", toolName)
				callback(AgentEvent{Type: "error", Content: observation})
				a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
				a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
				continue
			}

			// Emit tool call event
			callback(AgentEvent{Type: "tool_call", Content: toolName})

			// Execute tool
			observation, err := tool.Execute(toolArgs)
			if err != nil {
				observation = fmt.Sprintf("Error: %v", err)
			}

			// Emit observation event
			callback(AgentEvent{Type: "observation", Content: observation})

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

	msg := "Reached maximum steps without finding a final answer."
	callback(AgentEvent{Type: "error", Content: msg})
	return msg, nil
}

func (a *Agent) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are ZAP, an AI-powered API testing assistant. ")
	sb.WriteString("You use tools to help users test their APIs.\n\n")

	sb.WriteString("AVAILABLE TOOLS:\n")
	for _, tool := range a.tools {
		sb.WriteString(fmt.Sprintf("- %s: %s. Parameters: %s\n", tool.Name(), tool.Description(), tool.Parameters()))
	}

	sb.WriteString("\nWhen you need to use a tool, you MUST use this format:\n")
	sb.WriteString("Thought: <your reasoning>\n")
	sb.WriteString("ACTION: <tool_name>(<json_arguments>)\n\n")

	sb.WriteString("Example:\n")
	sb.WriteString("Thought: I need to check the user profile.\n")
	sb.WriteString("ACTION: http_request({\"method\": \"GET\", \"url\": \"https://api.github.com/user\"})\n\n")

	sb.WriteString("When you have the final answer, use this format:\n")
	sb.WriteString("Final Answer: <your response to the user>\n\n")

	sb.WriteString("Be concise. If a tool call fails, explain why and try to fix it.")

	return sb.String()
}

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
