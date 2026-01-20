package core

import (
	"fmt"
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
	// Add user message to history
	a.history = append(a.history, llm.Message{Role: "user", Content: input})

	// Max iterations to prevent infinite loops
	maxIterations := 5

	for i := 0; i < maxIterations; i++ {
		// Prepare system prompt with tool descriptions
		systemPrompt := a.buildSystemPrompt()

		messages := []llm.Message{{Role: "system", Content: systemPrompt}}
		messages = append(messages, a.history...)

		// Get LLM response
		response, err := a.llmClient.Chat(messages)
		if err != nil {
			return "", fmt.Errorf("agent chat error: %w", err)
		}

		// Parse response for thoughts and tool calls
		_, toolName, toolArgs, finalAnswer := a.parseResponse(response)

		if finalAnswer != "" {
			a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
			return finalAnswer, nil
		}

		if toolName != "" {
			tool, ok := a.tools[toolName]
			if !ok {
				observation := fmt.Sprintf("Error: Tool '%s' not found.", toolName)
				a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
				a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
				continue
			}

			// Execute tool
			observation, err := tool.Execute(toolArgs)
			if err != nil {
				observation = fmt.Sprintf("Error executing tool: %v", err)
			}

			// Add interaction to history
			a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
			a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
			continue
		}

		// If no tool call and no final answer, just return the response
		a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
		return response, nil
	}

	return "I'm sorry, I'm having trouble completing that request within a reasonable number of steps.", nil
}

func (a *Agent) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are ZAP, an AI-powered API testing assistant. ")
	sb.WriteString("You follow a ReAct (Reason + Act) pattern. ")
	sb.WriteString("For every user request, you should: \n")
	sb.WriteString("1. Thought: Reason about what to do.\n")
	sb.WriteString("2. Action: Call a tool if needed. Format: ACTION: ToolName(arguments_in_json)\n")
	sb.WriteString("3. Observation: The tool will return a result.\n")
	sb.WriteString("4. Final Answer: Once you have enough information, provide the response to the user.\n\n")

	sb.WriteString("AVAILABLE TOOLS:\n")
	for _, tool := range a.tools {
		sb.WriteString(fmt.Sprintf("- %s: %s. Parameters: %s\n", tool.Name(), tool.Description(), tool.Parameters()))
	}

	sb.WriteString("\nALWAYS format your actions as 'ACTION: ToolName({\"key\": \"value\"})'.\n")
	sb.WriteString("When you are done, start your response with 'Final Answer: '.")

	return sb.String()
}

func (a *Agent) parseResponse(response string) (thought, toolName, toolArgs, finalAnswer string) {
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Thought:") {
			thought = strings.TrimPrefix(line, "Thought:")
		} else if strings.HasPrefix(line, "ACTION:") {
			action := strings.TrimPrefix(line, "ACTION:")
			action = strings.TrimSpace(action)

			// Try to find ToolName(args)
			idxOpen := strings.Index(action, "(")
			idxClose := strings.LastIndex(action, ")")

			if idxOpen != -1 && idxClose != -1 {
				toolName = action[:idxOpen]
				toolArgs = action[idxOpen+1 : idxClose]
			}
		} else if strings.HasPrefix(line, "Final Answer:") {
			finalAnswer = strings.TrimPrefix(line, "Final Answer:")
		}
	}

	// If no structured Final Answer prefix, but also no tool call, treat the whole thing as final
	if finalAnswer == "" && toolName == "" {
		finalAnswer = response
	}

	return
}
