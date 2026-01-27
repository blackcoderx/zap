// Package core provides the central agent logic, tool management, and ReAct loop implementation
// for the ZAP API debugging assistant.
package core

import (
	"fmt"
	"os"
	"strings"
	"sync"

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
	Type             string // "thinking", "tool_call", "observation", "answer", "error", "streaming", "tool_usage", "confirmation_required"
	Content          string
	ToolUsage        *ToolUsageEvent  // Present only for "tool_usage" events
	FileConfirmation *FileConfirmation // Present only for "confirmation_required" events
}

// FileConfirmation contains information for file write confirmation prompts.
type FileConfirmation struct {
	FilePath  string // Path to the file being modified
	IsNewFile bool   // True if creating a new file
	Diff      string // Unified diff of the changes
}

// ToolUsageEvent contains tool usage statistics for display
type ToolUsageEvent struct {
	ToolName    string           // Current tool being called (empty if just stats update)
	ToolCurrent int              // Current calls for this tool
	ToolLimit   int              // Limit for this tool
	TotalCalls  int              // Total calls across all tools
	TotalLimit  int              // Total limit
	AllStats    []ToolUsageStats // All tool usage stats
}

// EventCallback is called when the agent emits an event
type EventCallback func(AgentEvent)

// ConfirmableTool is a tool that requires user confirmation before executing.
// Tools implementing this interface can emit events back to the TUI.
type ConfirmableTool interface {
	Tool
	SetEventCallback(callback EventCallback)
}

// Agent represents the ZAP AI agent.
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
}

// NewAgent creates a new ZAP agent
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

// SetLastResponse stores the last response from a tool (for chaining)
func (a *Agent) SetLastResponse(response interface{}) {
	a.lastResponse = response
}

// SetToolLimit sets the maximum number of calls allowed for a specific tool per session
func (a *Agent) SetToolLimit(toolName string, limit int) {
	a.toolLimits[toolName] = limit
}

// SetDefaultLimit sets the fallback limit for tools without a specific limit
func (a *Agent) SetDefaultLimit(limit int) {
	a.defaultLimit = limit
}

// SetTotalLimit sets the safety cap on total tool calls per session
func (a *Agent) SetTotalLimit(limit int) {
	a.totalLimit = limit
}

// SetFramework sets the user's API framework for context-aware assistance
func (a *Agent) SetFramework(framework string) {
	a.framework = framework
}

// GetFramework returns the configured API framework
func (a *Agent) GetFramework() string {
	return a.framework
}

// ResetToolCounts resets all tool call counters (called at start of each message)
func (a *Agent) ResetToolCounts() {
	a.toolCounts = make(map[string]int)
	a.totalCalls = 0
}

// getToolLimit returns the limit for a specific tool, or the default if not set
func (a *Agent) getToolLimit(toolName string) int {
	if limit, ok := a.toolLimits[toolName]; ok {
		return limit
	}
	return a.defaultLimit
}

// isToolLimitReached checks if a tool has reached its call limit
func (a *Agent) isToolLimitReached(toolName string) bool {
	return a.toolCounts[toolName] >= a.getToolLimit(toolName)
}

// isTotalLimitReached checks if the total call limit has been reached
func (a *Agent) isTotalLimitReached() bool {
	return a.totalCalls >= a.totalLimit
}

// ToolUsageStats represents the usage statistics for a single tool
type ToolUsageStats struct {
	Name    string
	Current int
	Limit   int
	Percent int // 0-100
}

// GetToolUsageStats returns current tool usage statistics
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

// GetTotalUsage returns total calls and limit
func (a *Agent) GetTotalUsage() (current, limit int) {
	return a.totalCalls, a.totalLimit
}

// ProcessMessage handles a user message using ReAct logic
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

// ProcessMessageWithEvents handles a user message and emits events for each stage
func (a *Agent) ProcessMessageWithEvents(input string, callback EventCallback) (string, error) {
	// Add user message to history
	a.history = append(a.history, llm.Message{Role: "user", Content: input})

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

			// Emit tool call event
			callback(AgentEvent{Type: "tool_call", Content: toolName})

			// Increment counters before execution
			a.toolCounts[toolName]++
			a.totalCalls++

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

func (a *Agent) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are ZAP, an AI-powered API debugging assistant. ")
	sb.WriteString("You help developers test APIs and debug errors by reading their codebase.\n\n")

	sb.WriteString(a.buildToolsSection())
	sb.WriteString(a.buildNaturalLanguageSection())
	sb.WriteString(a.buildErrorDiagnosisSection())
	sb.WriteString(a.buildCommonErrorSection())
	sb.WriteString(a.buildFrameworkHintsSection())
	sb.WriteString(a.buildPersistenceSection())
	sb.WriteString(a.buildTestingSection())
	sb.WriteString(a.buildChainingSection())
	sb.WriteString(a.buildAuthSection())
	sb.WriteString(a.buildTestSuiteSection())
	sb.WriteString(a.buildOutputFormatSection())

	return sb.String()
}

func (a *Agent) buildToolsSection() string {
	var sb strings.Builder
	sb.WriteString("AVAILABLE TOOLS:\n")
	a.toolsMu.RLock()
	for _, tool := range a.tools {
		sb.WriteString(fmt.Sprintf("- %s: %s. Parameters: %s\n", tool.Name(), tool.Description(), tool.Parameters()))
	}
	a.toolsMu.RUnlock()
	return sb.String()
}

func (a *Agent) buildNaturalLanguageSection() string {
	return `
## NATURAL LANGUAGE REQUESTS
Users may describe requests in natural language. Convert them:
- "GET users from localhost:8000" → GET http://localhost:8000/users
- "POST to /api/login with email and password" → POST with JSON body
- "Check the health endpoint" → GET /health or /api/health
- "Send a DELETE to users/123" → DELETE http://localhost:8000/users/123

`
}

func (a *Agent) buildErrorDiagnosisSection() string {
	return `## ERROR DIAGNOSIS WORKFLOW
When an API request returns an error (4xx/5xx), follow this workflow:

1. **Analyze the error response**:
   - Status code meaning (400=bad request, 401=unauthorized, 403=forbidden, 404=not found, 422=validation, 500=server error)
   - Error message in response body
   - Stack trace if present (look for file:line patterns)

2. **Search the codebase**:
   - Search for the endpoint path (e.g., "/api/users")
   - Search for error messages from the response
   - Search for the HTTP method + path combination

3. **Read relevant code**:
   - Route/handler files
   - Model/schema definitions (Pydantic, struct, etc.)
   - Middleware or validators

4. **Provide diagnosis**:
   - Exact file:line where the error originates
   - Root cause explanation
   - Suggested fix with code example

`
}

func (a *Agent) buildCommonErrorSection() string {
	return `## COMMON ERROR PATTERNS
- 400 Bad Request: Missing/invalid request body, wrong content-type
- 401 Unauthorized: Missing/invalid auth token, expired session
- 403 Forbidden: Valid auth but insufficient permissions
- 404 Not Found: Wrong URL path, resource doesn't exist
- 405 Method Not Allowed: Wrong HTTP method for endpoint
- 422 Unprocessable Entity: Validation failed (common in FastAPI/Pydantic)
- 500 Internal Server Error: Unhandled exception, check stack trace

`
}

func (a *Agent) buildFrameworkHintsSection() string {
	var sb strings.Builder

	// If user has configured a framework, provide specific guidance
	if a.framework != "" && a.framework != "other" {
		sb.WriteString(fmt.Sprintf("## USER'S FRAMEWORK: %s\n", strings.ToUpper(a.framework)))
		sb.WriteString("The user is building their API with this framework. Prioritize searching for patterns specific to it.\n\n")

		// Framework-specific hints
		switch a.framework {
		case "gin":
			sb.WriteString(`**Gin (Go) Patterns:**
- Routes: r.GET("/path", handler), r.POST("/path", handler), router.Group("/api")
- Context: c.JSON(200, data), c.BindJSON(&obj), c.Param("id"), c.Query("key")
- Errors: c.AbortWithStatusJSON(code, gin.H{"error": msg})
- Middleware: r.Use(middleware), c.Next(), c.Abort()
- Models: Look for struct tags like json:"field" binding:"required"
`)
		case "echo":
			sb.WriteString(`**Echo (Go) Patterns:**
- Routes: e.GET("/path", handler), e.POST("/path", handler), e.Group("/api")
- Context: c.JSON(200, data), c.Bind(&obj), c.Param("id"), c.QueryParam("key")
- Errors: echo.NewHTTPError(code, "message")
- Middleware: e.Use(middleware), e.Pre(middleware)
`)
		case "chi":
			sb.WriteString(`**Chi (Go) Patterns:**
- Routes: r.Get("/path", handler), r.Post("/path", handler), r.Route("/api", fn)
- Context: chi.URLParam(r, "id"), render.JSON(w, r, data)
- Middleware: r.Use(middleware), r.With(middleware)
`)
		case "fiber":
			sb.WriteString(`**Fiber (Go) Patterns:**
- Routes: app.Get("/path", handler), app.Post("/path", handler), app.Group("/api")
- Context: c.JSON(data), c.BodyParser(&obj), c.Params("id"), c.Query("key")
- Errors: fiber.NewError(code, "message"), c.Status(code).JSON()
`)
		case "fastapi":
			sb.WriteString(`**FastAPI (Python) Patterns:**
- Routes: @app.get("/path"), @app.post("/path"), @router.get("/path")
- Models: Pydantic BaseModel with Field(...) validators
- Errors: raise HTTPException(status_code=code, detail="message")
- Validation: 422 errors show "detail" array with field locations
- Dependencies: Depends(), get_db, get_current_user
`)
		case "flask":
			sb.WriteString(`**Flask (Python) Patterns:**
- Routes: @app.route("/path", methods=["GET"]), @blueprint.route()
- Request: request.json, request.args.get("key"), request.form
- Response: jsonify(data), make_response(), abort(code)
- Errors: @app.errorhandler(code)
`)
		case "django":
			sb.WriteString(`**Django REST Framework (Python) Patterns:**
- Views: @api_view(["GET"]), APIView class, ViewSet
- Serializers: serializers.Serializer, ModelSerializer
- Errors: raise ValidationError({"field": "message"})
- Response: Response(data, status=status.HTTP_200_OK)
`)
		case "express":
			sb.WriteString(`**Express (Node.js) Patterns:**
- Routes: app.get("/path", handler), router.post("/path", handler)
- Request: req.body, req.params.id, req.query.key
- Response: res.json(data), res.status(code).send()
- Errors: next(error), app.use((err, req, res, next) => {...})
- Middleware: app.use(middleware), router.use(middleware)
`)
		case "nestjs":
			sb.WriteString(`**NestJS (Node.js) Patterns:**
- Controllers: @Controller("/path"), @Get(), @Post(), @Param("id")
- Services: @Injectable(), constructor injection
- DTOs: class-validator decorators (@IsString, @IsNotEmpty)
- Errors: throw new HttpException("message", HttpStatus.BAD_REQUEST)
- Pipes: ValidationPipe, ParseIntPipe
`)
		case "hono":
			sb.WriteString(`**Hono (Node.js/Bun) Patterns:**
- Routes: app.get("/path", handler), app.post("/path", handler)
- Context: c.json(data), c.req.json(), c.req.param("id"), c.req.query("key")
- Errors: c.json({error: "message"}, 400), throw new HTTPException(code)
- Middleware: app.use(middleware)
`)
		case "spring":
			sb.WriteString(`**Spring Boot (Java) Patterns:**
- Controllers: @RestController, @GetMapping("/path"), @PostMapping("/path")
- Request: @RequestBody, @PathVariable, @RequestParam
- Response: ResponseEntity.ok(data), ResponseEntity.status(code).body()
- Validation: @Valid, @NotNull, @Size, BindingResult
- Errors: @ExceptionHandler, @ControllerAdvice
`)
		case "laravel":
			sb.WriteString(`**Laravel (PHP) Patterns:**
- Routes: Route::get("/path", [Controller::class, "method"])
- Controllers: public function index(Request $request)
- Request: $request->input("key"), $request->validate([...])
- Response: response()->json($data), abort(code, "message")
- Errors: ValidationException, Handler.php
`)
		case "rails":
			sb.WriteString(`**Rails (Ruby) Patterns:**
- Routes: get "/path", to: "controller#action", resources :items
- Controllers: def index, params[:id], render json: data
- Models: ActiveRecord validations, belongs_to, has_many
- Errors: render json: {error: "message"}, status: :bad_request
`)
		case "actix":
			sb.WriteString(`**Actix Web (Rust) Patterns:**
- Routes: web::get().to(handler), web::resource("/path").route()
- Extractors: web::Path<id>, web::Json<T>, web::Query<T>
- Response: HttpResponse::Ok().json(data)
- Errors: impl ResponseError for CustomError
`)
		case "axum":
			sb.WriteString(`**Axum (Rust) Patterns:**
- Routes: Router::new().route("/path", get(handler))
- Extractors: Path<id>, Json<T>, Query<T>, State<T>
- Response: Json(data), (StatusCode::OK, Json(data))
- Errors: impl IntoResponse for CustomError
`)
		}
		sb.WriteString("\n")
	}

	// Always include general hints for reference
	sb.WriteString(`## FRAMEWORK HINTS (General Reference)
- FastAPI/Python: Look for @app.get/@app.post decorators, Pydantic models, raise HTTPException
- Express/Node: Look for app.get/app.post, router.use, next(error)
- Go/Gin: Look for r.GET/r.POST, c.JSON, c.AbortWithError
- Django: Look for @api_view, serializers, raise ValidationError

`)
	return sb.String()
}

func (a *Agent) buildPersistenceSection() string {
	return `## REQUEST PERSISTENCE
You can save and load API requests for reuse:
- Use save_request to save a request with variables like {{BASE_URL}}
- Use load_request to load a saved request
- Use list_requests to see all saved requests
- Use set_environment to switch between dev/prod environments
- Use list_environments to see available environments

`
}

func (a *Agent) buildTestingSection() string {
	return `## TESTING & VALIDATION TOOLS
After making HTTP requests, you can validate and extract data:

1. **assert_response** - Validate responses against expected criteria:
   - Status codes: {"status_code": 200, "status_code_not": 500}
   - Headers: {"headers": {"Content-Type": "application/json"}}
   - Body content: {"body_contains": ["user_id"], "body_not_contains": ["error"]}
   - JSON path: {"json_path": {"$.status": "active", "$.data.id": 123}}
   - Performance: {"response_time_max_ms": 500}

2. **extract_value** - Extract data from responses for chaining requests:
   - JSON path: {"json_path": "$.data.user_id", "save_as": "user_id"}
   - Headers: {"header": "X-Request-Id", "save_as": "request_id"}
   - Cookies: {"cookie": "session_token", "save_as": "token"}
   - Regex: {"regex": "token=([a-z0-9]+)", "save_as": "auth_token"}

3. **variable** - Manage session and global variables:
   - Set: {"action": "set", "name": "user_id", "value": "123", "scope": "session"}
   - Get: {"action": "get", "name": "user_id"}
   - List all: {"action": "list"}
   - Use {{variable_name}} in http_request URLs, headers, and body

4. **wait** - Add delays for async operations:
   - {"duration_ms": 1000, "reason": "waiting for webhook"}

5. **retry** - Retry failed requests with backoff:
   - {"tool": "http_request", "args": {...}, "max_attempts": 3, "retry_delay_ms": 500, "backoff": "exponential"}

6. **validate_json_schema** - Validate against JSON Schema:
   - {"schema": {"type": "object", "required": ["id"], "properties": {"id": {"type": "integer"}}}}
   - Validates types, required fields, formats (email, uri), ranges, lengths

7. **compare_responses** - Compare responses for regression testing:
   - {"baseline": "baseline_name", "current": "last_response", "ignore_fields": ["timestamp"]}
   - Detects added, removed, or changed fields
   - Save baseline: {"baseline": "my_baseline", "save_baseline": true}

8. **performance_test** - Run load tests with concurrent users:
   - {"request": {...}, "duration_seconds": 30, "requests_per_second": 10, "concurrent_users": 5}
   - Returns: throughput, latency percentiles (p50/p95/p99), error rate, status code distribution
   - Use ramp_up_seconds to gradually increase load

9. **webhook_listener** - Start HTTP server to capture webhook callbacks:
   - Start: {"action": "start", "port": 0, "path": "/webhook", "timeout_seconds": 60, "listener_id": "webhook_1"}
   - Get requests: {"action": "get_requests", "listener_id": "webhook_1"}
   - Stop: {"action": "stop", "listener_id": "webhook_1"}
   - Returns URL to use for webhooks, captures all incoming requests with headers and body

`
}

func (a *Agent) buildAuthSection() string {
	return `## AUTHENTICATION TOOLS
1. **auth_bearer** - Create Bearer token header (JWT, API tokens):
   - {"token": "{{JWT_TOKEN}}", "save_as": "auth_header"}
   - Use: {"headers": {"Authorization": "{{auth_header}}"}}

2. **auth_basic** - Create HTTP Basic auth header:
   - {"username": "admin", "password": "secret", "save_as": "auth_header"}
   - Use: {"headers": {"Authorization": "{{auth_header}}"}}

3. **auth_helper** - Parse JWT tokens, decode Basic auth:
   - {"action": "parse_jwt", "token": "{{JWT_TOKEN}}"}
   - Shows header, payload (claims), expiration, subject

4. **auth_oauth2** - Perform OAuth2 authentication flows:
   - Client credentials: {"flow": "client_credentials", "token_url": "...", "client_id": "...", "client_secret": "...", "scopes": ["api:read"], "save_token_as": "oauth_token"}
   - Password flow: {"flow": "password", "token_url": "...", "client_id": "...", "client_secret": "...", "username": "...", "password": "...", "save_token_as": "oauth_token"}
   - Returns access token and automatically saves as Bearer header ({{token_name}}_header)

`
}

func (a *Agent) buildChainingSection() string {
	return `## REQUEST CHAINING WORKFLOW
For multi-step API flows:
1. Make initial request with http_request
2. Extract needed values with extract_value (saves to variables)
3. Use {{variable}} in subsequent requests
4. Validate each step with assert_response

Example: Create user → Extract user_id → Update user
1. POST /users → extract_value {"json_path": "$.id", "save_as": "user_id"}
2. PUT /users/{{user_id}} with update data
3. assert_response {"status_code": 200, "body_contains": ["updated"]}

## AUTHENTICATION WORKFLOW
Common auth patterns:
1. **JWT/Bearer Token**:
   - POST /auth/login → extract_value {"json_path": "$.token", "save_as": "jwt"}
   - auth_bearer {"token": "{{jwt}}", "save_as": "auth_header"}
   - GET /protected → Use {"headers": {"Authorization": "{{auth_header}}"}}

2. **HTTP Basic Auth**:
   - auth_basic {"username": "user", "password": "pass", "save_as": "auth_header"}
   - GET /protected → Use {"headers": {"Authorization": "{{auth_header}}"}}

3. **OAuth2 (Client Credentials)**:
   - auth_oauth2 {"flow": "client_credentials", "token_url": "...", "client_id": "{{CLIENT_ID}}", "client_secret": "{{CLIENT_SECRET}}", "save_token_as": "oauth_token"}
   - GET /protected → Use {"headers": {"Authorization": "{{oauth_token_header}}"}}

4. **OAuth2 (Password Flow)**:
   - auth_oauth2 {"flow": "password", "token_url": "...", "client_id": "...", "client_secret": "...", "username": "...", "password": "...", "save_token_as": "oauth_token"}
   - GET /protected → Use {"headers": {"Authorization": "{{oauth_token_header}}"}}

`
}

func (a *Agent) buildTestSuiteSection() string {
	return `## TEST SUITE WORKFLOW
For running multiple related tests:
1. Use test_suite to group tests logically
2. Tests run sequentially and can share extracted variables
3. Each test can have request, assertions, and extractions
4. Suite returns summary: X/Y passed with timing
5. Use on_failure: "stop" to halt on first failure or "continue" to run all

`
}

func (a *Agent) buildOutputFormatSection() string {
	return `When you need to use a tool, you MUST use this format:
Thought: <your reasoning>
ACTION: <tool_name>(<json_arguments>)

Examples:
Thought: The user wants to test the users endpoint. I'll make a GET request.
ACTION: http_request({"method": "GET", "url": "http://localhost:8000/api/users"})

Thought: Got a 422 error. I need to find where /api/users is defined to see the required fields.
ACTION: search_code({"pattern": "/api/users", "file_pattern": "*.py"})

Thought: Found the route in app/routes/users.py. Let me read it to see the Pydantic model.
ACTION: read_file({"path": "app/routes/users.py"})

When you have the final answer, use this format:
Final Answer: <your response>

Always include in error diagnoses:
- **File**: path/to/file.py:line_number
- **Cause**: What's wrong
- **Fix**: How to resolve it (with example if helpful)

Be concise and precise. Focus on actionable information.`
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
