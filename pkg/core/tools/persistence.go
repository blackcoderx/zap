package tools

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blackcoderx/zap/pkg/storage"
)

// PersistenceTool provides request save/load functionality
type PersistenceTool struct {
	baseDir     string
	currentEnv  string
	environment map[string]string
}

// NewPersistenceTool creates a new persistence tool
func NewPersistenceTool(baseDir string) *PersistenceTool {
	return &PersistenceTool{
		baseDir:     baseDir,
		currentEnv:  "",
		environment: make(map[string]string),
	}
}

// SetEnvironment sets the current environment by name
func (t *PersistenceTool) SetEnvironment(name string) error {
	envPath := filepath.Join(storage.GetEnvironmentsDir(t.baseDir), name+".yaml")
	env, err := storage.LoadEnvironment(envPath)
	if err != nil {
		return err
	}
	t.currentEnv = name
	t.environment = env
	return nil
}

// GetEnvironment returns the current environment variables
func (t *PersistenceTool) GetEnvironment() map[string]string {
	return t.environment
}

// SaveRequestTool saves requests to YAML files
type SaveRequestTool struct {
	persistence *PersistenceTool
}

func NewSaveRequestTool(p *PersistenceTool) *SaveRequestTool {
	return &SaveRequestTool{persistence: p}
}

func (t *SaveRequestTool) Name() string { return "save_request" }

func (t *SaveRequestTool) Description() string {
	return "Save an API request to a YAML file for later use. Saved requests can be loaded and executed with load_request."
}

func (t *SaveRequestTool) Parameters() string {
	return `{
  "name": "string (required) - Name for the request",
  "method": "string (required) - HTTP method (GET, POST, PUT, DELETE)",
  "url": "string (required) - Request URL (can use {{VAR}} placeholders)",
  "headers": "object (optional) - Request headers",
  "body": "object (optional) - Request body for POST/PUT"
}`
}

func (t *SaveRequestTool) Execute(args string) (string, error) {
	var params struct {
		Name    string            `json:"name"`
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Body    interface{}       `json:"body"`
	}

	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if params.Method == "" {
		return "", fmt.Errorf("method is required")
	}
	if params.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	req := storage.Request{
		Name:    params.Name,
		Method:  strings.ToUpper(params.Method),
		URL:     params.URL,
		Headers: params.Headers,
		Body:    params.Body,
	}

	// Generate filename from name
	filename := strings.ToLower(strings.ReplaceAll(params.Name, " ", "-")) + ".yaml"
	filePath := filepath.Join(storage.GetRequestsDir(t.persistence.baseDir), filename)

	if err := storage.SaveRequest(req, filePath); err != nil {
		return "", err
	}

	return fmt.Sprintf("Request saved to %s", filePath), nil
}

// LoadRequestTool loads requests from YAML files
type LoadRequestTool struct {
	persistence *PersistenceTool
}

func NewLoadRequestTool(p *PersistenceTool) *LoadRequestTool {
	return &LoadRequestTool{persistence: p}
}

func (t *LoadRequestTool) Name() string { return "load_request" }

func (t *LoadRequestTool) Description() string {
	return "Load a saved request from a YAML file. Returns the request details with environment variables substituted."
}

func (t *LoadRequestTool) Parameters() string {
	return `{"name": "string (required) - Name or filename of the saved request"}`
}

func (t *LoadRequestTool) Execute(args string) (string, error) {
	var params struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	// Try to find the file
	filename := params.Name
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		filename = strings.ToLower(strings.ReplaceAll(filename, " ", "-")) + ".yaml"
	}

	filePath := filepath.Join(storage.GetRequestsDir(t.persistence.baseDir), filename)
	req, err := storage.LoadRequest(filePath)
	if err != nil {
		return "", err
	}

	// Apply environment variables
	applied := storage.ApplyEnvironment(req, t.persistence.environment)

	// Format output
	result, _ := json.MarshalIndent(map[string]interface{}{
		"name":    applied.Name,
		"method":  applied.Method,
		"url":     applied.URL,
		"headers": applied.Headers,
		"body":    applied.Body,
	}, "", "  ")

	return string(result), nil
}

// ListRequestsTool lists all saved requests
type ListRequestsTool struct {
	persistence *PersistenceTool
}

func NewListRequestsTool(p *PersistenceTool) *ListRequestsTool {
	return &ListRequestsTool{persistence: p}
}

func (t *ListRequestsTool) Name() string { return "list_requests" }

func (t *ListRequestsTool) Description() string {
	return "List all saved API requests in the .zap/requests directory."
}

func (t *ListRequestsTool) Parameters() string {
	return `{}`
}

func (t *ListRequestsTool) Execute(args string) (string, error) {
	requests, err := storage.ListRequests(t.persistence.baseDir)
	if err != nil {
		return "", err
	}

	if len(requests) == 0 {
		return "No saved requests found. Use save_request to save a request.", nil
	}

	var sb strings.Builder
	sb.WriteString("Saved requests:\n")
	for _, req := range requests {
		sb.WriteString("  - " + req + "\n")
	}

	return sb.String(), nil
}

// ListEnvironmentsTool lists available environments
type ListEnvironmentsTool struct {
	persistence *PersistenceTool
}

func NewListEnvironmentsTool(p *PersistenceTool) *ListEnvironmentsTool {
	return &ListEnvironmentsTool{persistence: p}
}

func (t *ListEnvironmentsTool) Name() string { return "list_environments" }

func (t *ListEnvironmentsTool) Description() string {
	return "List all available environments in the .zap/environments directory."
}

func (t *ListEnvironmentsTool) Parameters() string {
	return `{}`
}

func (t *ListEnvironmentsTool) Execute(args string) (string, error) {
	envs, err := storage.ListEnvironments(t.persistence.baseDir)
	if err != nil {
		return "", err
	}

	if len(envs) == 0 {
		return "No environments found. Create YAML files in .zap/environments/ directory.", nil
	}

	var sb strings.Builder
	sb.WriteString("Available environments:\n")
	for _, env := range envs {
		marker := ""
		if env == t.persistence.currentEnv {
			marker = " (active)"
		}
		sb.WriteString("  - " + env + marker + "\n")
	}

	return sb.String(), nil
}

// SetEnvironmentTool sets the active environment
type SetEnvironmentTool struct {
	persistence *PersistenceTool
}

func NewSetEnvironmentTool(p *PersistenceTool) *SetEnvironmentTool {
	return &SetEnvironmentTool{persistence: p}
}

func (t *SetEnvironmentTool) Name() string { return "set_environment" }

func (t *SetEnvironmentTool) Description() string {
	return "Set the active environment. Environment variables will be substituted in saved requests."
}

func (t *SetEnvironmentTool) Parameters() string {
	return `{"name": "string (required) - Name of the environment (e.g., 'dev', 'prod')"}`
}

func (t *SetEnvironmentTool) Execute(args string) (string, error) {
	var params struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	if err := t.persistence.SetEnvironment(params.Name); err != nil {
		return "", err
	}

	return fmt.Sprintf("Environment set to '%s'", params.Name), nil
}
