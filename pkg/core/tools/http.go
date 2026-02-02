package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Default timeout for HTTP requests
const DefaultHTTPTimeout = 30 * time.Second

// HTTPTool provides HTTP request capabilities
type HTTPTool struct {
	client          *http.Client
	responseManager *ResponseManager
	varStore        *VariableStore
	defaultTimeout  time.Duration
}

// NewHTTPTool creates a new HTTP tool with the default 30-second timeout.
func NewHTTPTool(responseManager *ResponseManager, varStore *VariableStore) *HTTPTool {
	return &HTTPTool{
		client: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
		responseManager: responseManager,
		varStore:        varStore,
		defaultTimeout:  DefaultHTTPTimeout,
	}
}

// SetTimeout sets the default timeout for HTTP requests.
// This can be overridden per-request using the timeout parameter.
func (t *HTTPTool) SetTimeout(timeout time.Duration) {
	t.defaultTimeout = timeout
	t.client.Timeout = timeout
}

// HTTPRequest represents an HTTP request
type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
	Timeout int               `json:"timeout,omitempty"` // Timeout in seconds (0 = use default)
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Duration   time.Duration     `json:"duration"`
}

// Name returns the tool name
func (t *HTTPTool) Name() string {
	return "http_request"
}

// Description returns the tool description
func (t *HTTPTool) Description() string {
	return "Make HTTP requests to test API endpoints"
}

// Parameters returns the tool parameter description
func (t *HTTPTool) Parameters() string {
	return `{"method": "GET|POST|PUT|DELETE", "url": "string", "headers": {"key": "value"}, "body": {}, "timeout": 30}`
}

// Execute performs an HTTP request (implements core.Tool)
func (t *HTTPTool) Execute(args string) (string, error) {
	// Substitute variables in args if varStore is available
	if t.varStore != nil {
		args = t.varStore.Substitute(args)
	}

	var req HTTPRequest
	if err := json.Unmarshal([]byte(args), &req); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	resp, err := t.Run(req)
	if err != nil {
		return "", err
	}

	// Store response for assert/extract tools
	if t.responseManager != nil {
		t.responseManager.SetHTTPResponse(resp)
	}

	return resp.FormatResponse(), nil
}

// Run performs an HTTP request
func (t *HTTPTool) Run(req HTTPRequest) (*HTTPResponse, error) {
	startTime := time.Now()

	// Determine timeout: use per-request timeout if specified, otherwise use default
	timeout := t.defaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	// Create a client with the appropriate timeout for this request
	// We create a new client only if timeout differs from default to preserve connection pooling
	client := t.client
	if timeout != t.defaultTimeout {
		client = &http.Client{
			Timeout:   timeout,
			Transport: t.client.Transport, // Reuse transport for connection pooling
		}
	}

	// Prepare request body
	var bodyReader io.Reader
	if req.Body != nil {
		jsonBody, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(strings.ToUpper(req.Method), req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Execute request
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Build response headers map
	headers := make(map[string]string)
	for key, values := range httpResp.Header {
		headers[key] = strings.Join(values, ", ")
	}

	return &HTTPResponse{
		StatusCode: httpResp.StatusCode,
		Status:     httpResp.Status,
		Headers:    headers,
		Body:       string(bodyBytes),
		Duration:   time.Since(startTime),
	}, nil
}

// StatusCodeMeaning returns a human-readable explanation of HTTP status codes
func StatusCodeMeaning(code int) string {
	meanings := map[int]string{
		200: "OK - Request succeeded",
		201: "Created - Resource created successfully",
		204: "No Content - Request succeeded, no response body",
		400: "Bad Request - Invalid request syntax or missing required fields",
		401: "Unauthorized - Missing or invalid authentication token",
		403: "Forbidden - Valid auth but insufficient permissions",
		404: "Not Found - Endpoint or resource doesn't exist",
		405: "Method Not Allowed - HTTP method not supported for this endpoint",
		409: "Conflict - Resource conflict (e.g., duplicate entry)",
		422: "Unprocessable Entity - Validation failed (check required fields)",
		429: "Too Many Requests - Rate limit exceeded",
		500: "Internal Server Error - Server-side exception (check logs/stack trace)",
		502: "Bad Gateway - Upstream server error",
		503: "Service Unavailable - Server overloaded or down",
	}

	if meaning, ok := meanings[code]; ok {
		return meaning
	}

	// Generic meanings by range
	switch {
	case code >= 200 && code < 300:
		return "Success"
	case code >= 300 && code < 400:
		return "Redirect"
	case code >= 400 && code < 500:
		return "Client Error - Check your request"
	case code >= 500:
		return "Server Error - Check server logs"
	default:
		return "Unknown status code"
	}
}

// FormatResponse formats the HTTP response for display
func (r *HTTPResponse) FormatResponse() string {
	var sb strings.Builder

	// Calculate body size
	bodySize := len(r.Body)
	sizeStr := formatSize(bodySize)

	// Status line with meaning, duration, and size
	sb.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
	sb.WriteString(fmt.Sprintf("Time:   %dms\n", r.Duration.Milliseconds()))
	sb.WriteString(fmt.Sprintf("Size:   %s\n", sizeStr))
	sb.WriteString(fmt.Sprintf("Meaning: %s\n\n", StatusCodeMeaning(r.StatusCode)))

	// Headers (condensed - only show important ones)
	importantHeaders := []string{"Content-Type", "Authorization", "X-Request-Id", "X-Error-Code"}
	sb.WriteString("Headers:\n")
	for _, key := range importantHeaders {
		if value, ok := r.Headers[key]; ok {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
		}
	}
	// Show content-type if not in important headers
	if ct, ok := r.Headers["Content-Type"]; ok {
		found := false
		for _, h := range importantHeaders {
			if h == "Content-Type" {
				found = true
				break
			}
		}
		if !found {
			sb.WriteString(fmt.Sprintf("  Content-Type: %s\n", ct))
		}
	}
	sb.WriteString("\n")

	// Body (try to pretty-print JSON)
	sb.WriteString("Body:\n")
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(r.Body), "", "  "); err == nil {
		sb.WriteString("```json\n")
		sb.WriteString(prettyJSON.String())
		sb.WriteString("\n```")
	} else {
		// If not JSON, just show as text (maybe truncated if too long?)
		if len(r.Body) > 5000 {
			sb.WriteString(r.Body[:5000] + "\n... (truncated)")
		} else {
			sb.WriteString(r.Body)
		}
	}

	// Add error hints for common status codes
	if r.StatusCode >= 400 {
		sb.WriteString("\n\n")
		sb.WriteString(r.getErrorHints())
	}

	return sb.String()
}

func formatSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// getErrorHints provides debugging hints based on status code and response
func (r *HTTPResponse) getErrorHints() string {
	var hints []string

	switch r.StatusCode {
	case 400:
		hints = append(hints, "Hint: Check request body format and required fields")
	case 401:
		hints = append(hints, "Hint: Add Authorization header with valid token")
	case 403:
		hints = append(hints, "Hint: User authenticated but lacks permission for this action")
	case 404:
		hints = append(hints, "Hint: Verify the URL path and that the resource exists")
	case 405:
		hints = append(hints, "Hint: Check if you're using the correct HTTP method (GET/POST/PUT/DELETE)")
	case 422:
		hints = append(hints, "Hint: Validation error - check required fields and data types")
		// Try to extract field errors from common formats
		if strings.Contains(r.Body, "detail") {
			hints = append(hints, "Hint: Look at 'detail' field for specific validation errors")
		}
	case 500:
		hints = append(hints, "Hint: Server error - search codebase for the endpoint handler")
		if strings.Contains(r.Body, "Traceback") || strings.Contains(r.Body, "stack") {
			hints = append(hints, "Hint: Stack trace detected - look for file:line references")
		}
	}

	return strings.Join(hints, "\n")
}
