package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// WebhookListenerTool provides webhook capture capabilities
type WebhookListenerTool struct {
	varStore *VariableStore
	mu       sync.Mutex
	servers  map[string]*webhookServer
}

// webhookServer represents a running webhook listener
type webhookServer struct {
	server   *http.Server
	requests []CapturedRequest
	url      string
	mu       sync.Mutex
	done     chan struct{}
}

// CapturedRequest represents a captured webhook request
type CapturedRequest struct {
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	Timestamp time.Time         `json:"timestamp"`
}

// NewWebhookListenerTool creates a new webhook listener tool
func NewWebhookListenerTool(varStore *VariableStore) *WebhookListenerTool {
	return &WebhookListenerTool{
		varStore: varStore,
		servers:  make(map[string]*webhookServer),
	}
}

// Name returns the tool name
func (t *WebhookListenerTool) Name() string {
	return "webhook_listener"
}

// Description returns the tool description
func (t *WebhookListenerTool) Description() string {
	return "Start a temporary HTTP server to capture incoming webhook requests. Returns the URL to use for webhooks and captures all incoming requests."
}

// Parameters returns the tool parameter description
func (t *WebhookListenerTool) Parameters() string {
	return `{
  "action": "start|stop|get_requests",
  "port": 0,
  "path": "/webhook",
  "timeout_seconds": 60,
  "listener_id": "webhook_1"
}`
}

// WebhookListenerParams defines parameters for webhook listener
type WebhookListenerParams struct {
	Action         string `json:"action"`
	Port           int    `json:"port,omitempty"`
	Path           string `json:"path,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	ListenerID     string `json:"listener_id,omitempty"`
}

// Execute runs the webhook listener command
func (t *WebhookListenerTool) Execute(args string) (string, error) {
	var params WebhookListenerParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Set defaults
	if params.ListenerID == "" {
		params.ListenerID = "webhook_1"
	}
	if params.Path == "" {
		params.Path = "/webhook"
	}
	if params.TimeoutSeconds == 0 {
		params.TimeoutSeconds = 60
	}

	switch params.Action {
	case "start":
		return t.startListener(params)
	case "stop":
		return t.stopListener(params.ListenerID)
	case "get_requests":
		return t.getRequests(params.ListenerID)
	default:
		return "", fmt.Errorf("unknown action: %s (use 'start', 'stop', or 'get_requests')", params.Action)
	}
}

// startListener starts a new webhook listener
func (t *WebhookListenerTool) startListener(params WebhookListenerParams) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if listener already exists
	if _, exists := t.servers[params.ListenerID]; exists {
		return "", fmt.Errorf("listener '%s' already running. Stop it first or use a different listener_id", params.ListenerID)
	}

	// Create listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", params.Port))
	if err != nil {
		return "", fmt.Errorf("failed to start listener: %w", err)
	}

	// Get actual port (useful when port=0 for random port)
	addr := listener.Addr().(*net.TCPAddr)
	actualPort := addr.Port

	// Create webhook server
	ws := &webhookServer{
		requests: make([]CapturedRequest, 0),
		url:      fmt.Sprintf("http://localhost:%d%s", actualPort, params.Path),
		done:     make(chan struct{}),
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc(params.Path, func(w http.ResponseWriter, r *http.Request) {
		// Read body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			body = []byte(fmt.Sprintf("Error reading body: %v", err))
		}
		defer r.Body.Close()

		// Capture headers
		headers := make(map[string]string)
		for key, values := range r.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}

		// Store request
		ws.mu.Lock()
		ws.requests = append(ws.requests, CapturedRequest{
			Method:    r.Method,
			Path:      r.URL.Path,
			Headers:   headers,
			Body:      string(body),
			Timestamp: time.Now(),
		})
		ws.mu.Unlock()

		// Send success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"received"}`))
	})

	// Create server
	ws.server = &http.Server{
		Handler: mux,
	}

	// Start server in background
	go func() {
		ws.server.Serve(listener)
	}()

	// Auto-shutdown after timeout
	go func() {
		select {
		case <-time.After(time.Duration(params.TimeoutSeconds) * time.Second):
			t.stopListener(params.ListenerID)
		case <-ws.done:
			return
		}
	}()

	// Store server
	t.servers[params.ListenerID] = ws

	// Save URL to variables if varStore available
	if t.varStore != nil {
		t.varStore.Set(fmt.Sprintf("%s_url", params.ListenerID), ws.url)
	}

	return fmt.Sprintf(`Webhook listener started!

Listener ID: %s
URL: %s
Timeout: %d seconds
Port: %d

Send webhooks to this URL. Use 'get_requests' to retrieve captured requests.
The listener will automatically stop after %d seconds.`,
		params.ListenerID,
		ws.url,
		params.TimeoutSeconds,
		actualPort,
		params.TimeoutSeconds,
	), nil
}

// stopListener stops a running webhook listener
func (t *WebhookListenerTool) stopListener(listenerID string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ws, exists := t.servers[listenerID]
	if !exists {
		return "", fmt.Errorf("listener '%s' not found", listenerID)
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ws.server.Shutdown(ctx); err != nil {
		return "", fmt.Errorf("failed to shutdown listener: %w", err)
	}

	// Signal done
	close(ws.done)

	// Get final request count
	ws.mu.Lock()
	requestCount := len(ws.requests)
	ws.mu.Unlock()

	// Remove from active servers
	delete(t.servers, listenerID)

	return fmt.Sprintf("Listener '%s' stopped. Captured %d request(s).", listenerID, requestCount), nil
}

// getRequests retrieves captured requests from a listener
func (t *WebhookListenerTool) getRequests(listenerID string) (string, error) {
	t.mu.Lock()
	ws, exists := t.servers[listenerID]
	t.mu.Unlock()

	if !exists {
		return "", fmt.Errorf("listener '%s' not found", listenerID)
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	if len(ws.requests) == 0 {
		return fmt.Sprintf("No requests captured yet for listener '%s'.", listenerID), nil
	}

	// Format requests
	output := fmt.Sprintf("Captured %d request(s) for listener '%s':\n\n", len(ws.requests), listenerID)

	for i, req := range ws.requests {
		output += fmt.Sprintf("Request #%d (%s)\n", i+1, req.Timestamp.Format("15:04:05"))
		output += fmt.Sprintf("  Method: %s\n", req.Method)
		output += fmt.Sprintf("  Path: %s\n", req.Path)

		if len(req.Headers) > 0 {
			output += "  Headers:\n"
			for key, value := range req.Headers {
				output += fmt.Sprintf("    %s: %s\n", key, value)
			}
		}

		if req.Body != "" {
			output += fmt.Sprintf("  Body: %s\n", req.Body)
		}

		output += "\n"
	}

	// Store requests in variables if varStore available
	if t.varStore != nil {
		requestsJSON, err := json.Marshal(ws.requests)
		if err == nil {
			t.varStore.Set(fmt.Sprintf("%s_requests", listenerID), string(requestsJSON))
		}
	}

	return output, nil
}

// Cleanup stops all running listeners (call on shutdown)
func (t *WebhookListenerTool) Cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for id := range t.servers {
		t.stopListener(id)
	}
}
