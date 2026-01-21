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
	Type    string // "thinking", "tool_call", "observation", "answer", "error", "streaming"
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

		// Get LLM response with streaming
		var response string
		var streamErr error

		// Stream callback emits chunks to TUI
		streamCallback := func(chunk string) {
			callback(AgentEvent{Type: "streaming", Content: chunk})
		}

		response, streamErr = a.llmClient.ChatStream(messages, streamCallback)
		if streamErr != nil {
			callback(AgentEvent{Type: "error", Content: streamErr.Error()})
			return "", fmt.Errorf("agent chat error: %w", streamErr)
		}

		if response == "" {
			callback(AgentEvent{Type: "error", Content: "empty response from AI"})
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
	sb.WriteString("You are ZAP, an AI-powered API debugging assistant. ")
	sb.WriteString("You help developers test APIs and debug errors by reading their codebase.\n\n")

	sb.WriteString("AVAILABLE TOOLS:\n")
	for _, tool := range a.tools {
		sb.WriteString(fmt.Sprintf("- %s: %s. Parameters: %s\n", tool.Name(), tool.Description(), tool.Parameters()))
	}

	sb.WriteString("\n## NATURAL LANGUAGE REQUESTS\n")
	sb.WriteString("Users may describe requests in natural language. Convert them:\n")
	sb.WriteString("- \"GET users from localhost:8000\" → GET http://localhost:8000/users\n")
	sb.WriteString("- \"POST to /api/login with email and password\" → POST with JSON body\n")
	sb.WriteString("- \"Check the health endpoint\" → GET /health or /api/health\n")
	sb.WriteString("- \"Send a DELETE to users/123\" → DELETE http://localhost:8000/users/123\n\n")

	sb.WriteString("## ERROR DIAGNOSIS WORKFLOW\n")
	sb.WriteString("When an API request returns an error (4xx/5xx), follow this workflow:\n\n")
	sb.WriteString("1. **Analyze the error response**:\n")
	sb.WriteString("   - Status code meaning (400=bad request, 401=unauthorized, 403=forbidden, 404=not found, 422=validation, 500=server error)\n")
	sb.WriteString("   - Error message in response body\n")
	sb.WriteString("   - Stack trace if present (look for file:line patterns)\n\n")
	sb.WriteString("2. **Search the codebase**:\n")
	sb.WriteString("   - Search for the endpoint path (e.g., \"/api/users\")\n")
	sb.WriteString("   - Search for error messages from the response\n")
	sb.WriteString("   - Search for the HTTP method + path combination\n\n")
	sb.WriteString("3. **Read relevant code**:\n")
	sb.WriteString("   - Route/handler files\n")
	sb.WriteString("   - Model/schema definitions (Pydantic, struct, etc.)\n")
	sb.WriteString("   - Middleware or validators\n\n")
	sb.WriteString("4. **Provide diagnosis**:\n")
	sb.WriteString("   - Exact file:line where the error originates\n")
	sb.WriteString("   - Root cause explanation\n")
	sb.WriteString("   - Suggested fix with code example\n\n")

	sb.WriteString("## COMMON ERROR PATTERNS\n")
	sb.WriteString("- 400 Bad Request: Missing/invalid request body, wrong content-type\n")
	sb.WriteString("- 401 Unauthorized: Missing/invalid auth token, expired session\n")
	sb.WriteString("- 403 Forbidden: Valid auth but insufficient permissions\n")
	sb.WriteString("- 404 Not Found: Wrong URL path, resource doesn't exist\n")
	sb.WriteString("- 405 Method Not Allowed: Wrong HTTP method for endpoint\n")
	sb.WriteString("- 422 Unprocessable Entity: Validation failed (common in FastAPI/Pydantic)\n")
	sb.WriteString("- 500 Internal Server Error: Unhandled exception, check stack trace\n\n")

	sb.WriteString("## FRAMEWORK HINTS\n")
	sb.WriteString("- FastAPI/Python: Look for @app.get/@app.post decorators, Pydantic models, raise HTTPException\n")
	sb.WriteString("- Express/Node: Look for app.get/app.post, router.use, next(error)\n")
	sb.WriteString("- Go/Gin: Look for r.GET/r.POST, c.JSON, c.AbortWithError\n")
	sb.WriteString("- Django: Look for @api_view, serializers, raise ValidationError\n\n")

	sb.WriteString("## REQUEST PERSISTENCE\n")
	sb.WriteString("You can save and load API requests for reuse:\n")
	sb.WriteString("- Use save_request to save a request with variables like {{BASE_URL}}\n")
	sb.WriteString("- Use load_request to load a saved request\n")
	sb.WriteString("- Use list_requests to see all saved requests\n")
	sb.WriteString("- Use set_environment to switch between dev/prod environments\n")
	sb.WriteString("- Use list_environments to see available environments\n\n")

	sb.WriteString("When you need to use a tool, you MUST use this format:\n")
	sb.WriteString("Thought: <your reasoning>\n")
	sb.WriteString("ACTION: <tool_name>(<json_arguments>)\n\n")

	sb.WriteString("Examples:\n")
	sb.WriteString("Thought: The user wants to test the users endpoint. I'll make a GET request.\n")
	sb.WriteString("ACTION: http_request({\"method\": \"GET\", \"url\": \"http://localhost:8000/api/users\"})\n\n")

	sb.WriteString("Thought: Got a 422 error. I need to find where /api/users is defined to see the required fields.\n")
	sb.WriteString("ACTION: search_code({\"pattern\": \"/api/users\", \"file_pattern\": \"*.py\"})\n\n")

	sb.WriteString("Thought: Found the route in app/routes/users.py. Let me read it to see the Pydantic model.\n")
	sb.WriteString("ACTION: read_file({\"path\": \"app/routes/users.py\"})\n\n")

	sb.WriteString("When you have the final answer, use this format:\n")
	sb.WriteString("Final Answer: <your response>\n\n")

	sb.WriteString("Always include in error diagnoses:\n")
	sb.WriteString("- **File**: path/to/file.py:line_number\n")
	sb.WriteString("- **Cause**: What's wrong\n")
	sb.WriteString("- **Fix**: How to resolve it (with example if helpful)\n\n")

	sb.WriteString("Be concise and precise. Focus on actionable information.")

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
