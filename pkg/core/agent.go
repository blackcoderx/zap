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
	llmClient    *llm.OllamaClient
	tools        map[string]Tool
	history      []llm.Message
	lastResponse interface{} // Store last tool response for chaining
}

// NewAgent creates a new ZAP agent
func NewAgent(llmClient *llm.OllamaClient) *Agent {
	return &Agent{
		llmClient:    llmClient,
		tools:        make(map[string]Tool),
		history:      []llm.Message{},
		lastResponse: nil,
	}
}

// RegisterTool adds a tool to the agent's arsenal
func (a *Agent) RegisterTool(tool Tool) {
	a.tools[tool.Name()] = tool
}

// ExecuteTool executes a tool by name (used by retry tool)
func (a *Agent) ExecuteTool(toolName string, args string) (string, error) {
	tool, ok := a.tools[toolName]
	if !ok {
		return "", fmt.Errorf("tool '%s' not found", toolName)
	}
	return tool.Execute(args)
}

// SetLastResponse stores the last response from a tool (for chaining)
func (a *Agent) SetLastResponse(response interface{}) {
	a.lastResponse = response
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
			tool, ok := a.tools[toolName]
			if !ok {
				// Agent sees this error
				observation := fmt.Sprintf("System Error: Tool '%s' does not exist. Please use only available tools.", toolName)
				// User sees this error
				callback(AgentEvent{Type: "error", Content: fmt.Sprintf("The agent tried to use an unknown tool '%s'.", toolName)})

				a.history = append(a.history, llm.Message{Role: "assistant", Content: response})
				a.history = append(a.history, llm.Message{Role: "user", Content: fmt.Sprintf("Observation: %s", observation)})
				continue
			}

			// Emit tool call event
			callback(AgentEvent{Type: "tool_call", Content: toolName})

			// Execute tool
			observation, err := tool.Execute(toolArgs)
			if err != nil {
				// Detailed error for the agent to self-correct
				observation = fmt.Sprintf("Tool Execution Error: %v", err)
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

	msg := "I stopped because I reached the maximum number of steps (5). This usually means I got stuck in a loop or the task is too complex.\nTip: Try breaking your request into smaller steps."
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

	sb.WriteString("## TESTING & VALIDATION TOOLS\n")
	sb.WriteString("After making HTTP requests, you can validate and extract data:\n\n")
	sb.WriteString("1. **assert_response** - Validate responses against expected criteria:\n")
	sb.WriteString("   - Status codes: {\"status_code\": 200, \"status_code_not\": 500}\n")
	sb.WriteString("   - Headers: {\"headers\": {\"Content-Type\": \"application/json\"}}\n")
	sb.WriteString("   - Body content: {\"body_contains\": [\"user_id\"], \"body_not_contains\": [\"error\"]}\n")
	sb.WriteString("   - JSON path: {\"json_path\": {\"$.status\": \"active\", \"$.data.id\": 123}}\n")
	sb.WriteString("   - Performance: {\"response_time_max_ms\": 500}\n\n")
	sb.WriteString("2. **extract_value** - Extract data from responses for chaining requests:\n")
	sb.WriteString("   - JSON path: {\"json_path\": \"$.data.user_id\", \"save_as\": \"user_id\"}\n")
	sb.WriteString("   - Headers: {\"header\": \"X-Request-Id\", \"save_as\": \"request_id\"}\n")
	sb.WriteString("   - Cookies: {\"cookie\": \"session_token\", \"save_as\": \"token\"}\n")
	sb.WriteString("   - Regex: {\"regex\": \"token=([a-z0-9]+)\", \"save_as\": \"auth_token\"}\n\n")
	sb.WriteString("3. **variable** - Manage session and global variables:\n")
	sb.WriteString("   - Set: {\"action\": \"set\", \"name\": \"user_id\", \"value\": \"123\", \"scope\": \"session\"}\n")
	sb.WriteString("   - Get: {\"action\": \"get\", \"name\": \"user_id\"}\n")
	sb.WriteString("   - List all: {\"action\": \"list\"}\n")
	sb.WriteString("   - Use {{variable_name}} in http_request URLs, headers, and body\n\n")
	sb.WriteString("4. **wait** - Add delays for async operations:\n")
	sb.WriteString("   - {\"duration_ms\": 1000, \"reason\": \"waiting for webhook\"}\n\n")
	sb.WriteString("5. **retry** - Retry failed requests with backoff:\n")
	sb.WriteString("   - {\"tool\": \"http_request\", \"args\": {...}, \"max_attempts\": 3, \"retry_delay_ms\": 500, \"backoff\": \"exponential\"}\n\n")

	sb.WriteString("6. **validate_json_schema** - Validate against JSON Schema:\n")
	sb.WriteString("   - {\"schema\": {\"type\": \"object\", \"required\": [\"id\"], \"properties\": {\"id\": {\"type\": \"integer\"}}}}\n")
	sb.WriteString("   - Validates types, required fields, formats (email, uri), ranges, lengths\n\n")
	sb.WriteString("7. **auth_bearer** - Create Bearer token header (JWT, API tokens):\n")
	sb.WriteString("   - {\"token\": \"{{JWT_TOKEN}}\", \"save_as\": \"auth_header\"}\n")
	sb.WriteString("   - Use: {\"headers\": {\"Authorization\": \"{{auth_header}}\"}}\n\n")
	sb.WriteString("8. **auth_basic** - Create HTTP Basic auth header:\n")
	sb.WriteString("   - {\"username\": \"admin\", \"password\": \"secret\", \"save_as\": \"auth_header\"}\n")
	sb.WriteString("   - Use: {\"headers\": {\"Authorization\": \"{{auth_header}}\"}}\n\n")
	sb.WriteString("9. **auth_helper** - Parse JWT tokens, decode Basic auth:\n")
	sb.WriteString("   - {\"action\": \"parse_jwt\", \"token\": \"{{JWT_TOKEN}}\"}\n")
	sb.WriteString("   - Shows header, payload (claims), expiration, subject\n\n")
	sb.WriteString("10. **test_suite** - Run organized test suites:\n")
	sb.WriteString("   - {\"name\": \"API Tests\", \"tests\": [{\"name\": \"Create user\", \"request\": {...}, \"assertions\": {...}, \"extract\": {...}}]}\n")
	sb.WriteString("   - Runs tests sequentially, extracts values between tests, returns pass/fail summary\n\n")
	sb.WriteString("11. **compare_responses** - Compare responses for regression testing:\n")
	sb.WriteString("   - {\"baseline\": \"baseline_name\", \"current\": \"last_response\", \"ignore_fields\": [\"timestamp\"]}\n")
	sb.WriteString("   - Detects added, removed, or changed fields\n")
	sb.WriteString("   - Save baseline: {\"baseline\": \"my_baseline\", \"save_baseline\": true}\n\n")
	sb.WriteString("12. **performance_test** - Run load tests with concurrent users:\n")
	sb.WriteString("   - {\"request\": {...}, \"duration_seconds\": 30, \"requests_per_second\": 10, \"concurrent_users\": 5}\n")
	sb.WriteString("   - Returns: throughput, latency percentiles (p50/p95/p99), error rate, status code distribution\n")
	sb.WriteString("   - Use ramp_up_seconds to gradually increase load\n\n")
	sb.WriteString("13. **webhook_listener** - Start HTTP server to capture webhook callbacks:\n")
	sb.WriteString("   - Start: {\"action\": \"start\", \"port\": 0, \"path\": \"/webhook\", \"timeout_seconds\": 60, \"listener_id\": \"webhook_1\"}\n")
	sb.WriteString("   - Get requests: {\"action\": \"get_requests\", \"listener_id\": \"webhook_1\"}\n")
	sb.WriteString("   - Stop: {\"action\": \"stop\", \"listener_id\": \"webhook_1\"}\n")
	sb.WriteString("   - Returns URL to use for webhooks, captures all incoming requests with headers and body\n\n")
	sb.WriteString("14. **auth_oauth2** - Perform OAuth2 authentication flows:\n")
	sb.WriteString("   - Client credentials: {\"flow\": \"client_credentials\", \"token_url\": \"...\", \"client_id\": \"...\", \"client_secret\": \"...\", \"scopes\": [\"api:read\"], \"save_token_as\": \"oauth_token\"}\n")
	sb.WriteString("   - Password flow: {\"flow\": \"password\", \"token_url\": \"...\", \"client_id\": \"...\", \"client_secret\": \"...\", \"username\": \"...\", \"password\": \"...\", \"save_token_as\": \"oauth_token\"}\n")
	sb.WriteString("   - Returns access token and automatically saves as Bearer header ({{token_name}}_header)\n\n")

	sb.WriteString("## REQUEST CHAINING WORKFLOW\n")
	sb.WriteString("For multi-step API flows:\n")
	sb.WriteString("1. Make initial request with http_request\n")
	sb.WriteString("2. Extract needed values with extract_value (saves to variables)\n")
	sb.WriteString("3. Use {{variable}} in subsequent requests\n")
	sb.WriteString("4. Validate each step with assert_response\n\n")
	sb.WriteString("Example: Create user → Extract user_id → Update user\n")
	sb.WriteString("1. POST /users → extract_value {\"json_path\": \"$.id\", \"save_as\": \"user_id\"}\n")
	sb.WriteString("2. PUT /users/{{user_id}} with update data\n")
	sb.WriteString("3. assert_response {\"status_code\": 200, \"body_contains\": [\"updated\"]}\n\n")

	sb.WriteString("## AUTHENTICATION WORKFLOW\n")
	sb.WriteString("Common auth patterns:\n")
	sb.WriteString("1. **JWT/Bearer Token**:\n")
	sb.WriteString("   - POST /auth/login → extract_value {\"json_path\": \"$.token\", \"save_as\": \"jwt\"}\n")
	sb.WriteString("   - auth_bearer {\"token\": \"{{jwt}}\", \"save_as\": \"auth_header\"}\n")
	sb.WriteString("   - GET /protected → Use {\"headers\": {\"Authorization\": \"{{auth_header}}\"}}\n\n")
	sb.WriteString("2. **HTTP Basic Auth**:\n")
	sb.WriteString("   - auth_basic {\"username\": \"user\", \"password\": \"pass\", \"save_as\": \"auth_header\"}\n")
	sb.WriteString("   - GET /protected → Use {\"headers\": {\"Authorization\": \"{{auth_header}}\"}}\n\n")
	sb.WriteString("3. **OAuth2 (Client Credentials)**:\n")
	sb.WriteString("   - auth_oauth2 {\"flow\": \"client_credentials\", \"token_url\": \"...\", \"client_id\": \"{{CLIENT_ID}}\", \"client_secret\": \"{{CLIENT_SECRET}}\", \"save_token_as\": \"oauth_token\"}\n")
	sb.WriteString("   - GET /protected → Use {\"headers\": {\"Authorization\": \"{{oauth_token_header}}\"}}\n\n")
	sb.WriteString("4. **OAuth2 (Password Flow)**:\n")
	sb.WriteString("   - auth_oauth2 {\"flow\": \"password\", \"token_url\": \"...\", \"client_id\": \"...\", \"client_secret\": \"...\", \"username\": \"...\", \"password\": \"...\", \"save_token_as\": \"oauth_token\"}\n")
	sb.WriteString("   - GET /protected → Use {\"headers\": {\"Authorization\": \"{{oauth_token_header}}\"}}\n\n")

	sb.WriteString("## TEST SUITE WORKFLOW\n")
	sb.WriteString("For running multiple related tests:\n")
	sb.WriteString("1. Use test_suite to group tests logically\n")
	sb.WriteString("2. Tests run sequentially and can share extracted variables\n")
	sb.WriteString("3. Each test can have request, assertions, and extractions\n")
	sb.WriteString("4. Suite returns summary: X/Y passed with timing\n")
	sb.WriteString("5. Use on_failure: \"stop\" to halt on first failure or \"continue\" to run all\n\n")

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
