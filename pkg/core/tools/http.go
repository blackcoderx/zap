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

// HTTPTool provides HTTP request capabilities
type HTTPTool struct {
	client *http.Client
}

// NewHTTPTool creates a new HTTP tool
func NewHTTPTool() *HTTPTool {
	return &HTTPTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// HTTPRequest represents an HTTP request
type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
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
	return `{"method": "GET|POST|PUT|DELETE", "url": "string", "headers": {"key": "value"}, "body": {}}`
}

// Execute performs an HTTP request (implements core.Tool)
func (t *HTTPTool) Execute(args string) (string, error) {
	var req HTTPRequest
	if err := json.Unmarshal([]byte(args), &req); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	resp, err := t.Run(req)
	if err != nil {
		return "", err
	}

	return resp.FormatResponse(), nil
}

// Run performs an HTTP request
func (t *HTTPTool) Run(req HTTPRequest) (*HTTPResponse, error) {
	startTime := time.Now()

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
	httpResp, err := t.client.Do(httpReq)
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

// FormatResponse formats the HTTP response for display
func (r *HTTPResponse) FormatResponse() string {
	var sb strings.Builder

	// Status line
	sb.WriteString(fmt.Sprintf("Status: %s (%dms)\n\n", r.Status, r.Duration.Milliseconds()))

	// Headers
	sb.WriteString("Headers:\n")
	for key, value := range r.Headers {
		sb.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
	}
	sb.WriteString("\n")

	// Body (try to pretty-print JSON)
	sb.WriteString("Body:\n")
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(r.Body), "", "  "); err == nil {
		sb.WriteString(prettyJSON.String())
	} else {
		sb.WriteString(r.Body)
	}

	return sb.String()
}
