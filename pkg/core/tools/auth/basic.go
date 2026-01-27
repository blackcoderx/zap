package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/blackcoderx/zap/pkg/core/tools"
)

// BasicTool creates HTTP Basic authentication headers with base64 encoding.
// It encodes username:password and wraps it in the standard "Basic <encoded>" format.
type BasicTool struct {
	varStore *tools.VariableStore
}

// NewBasicTool creates a new Basic auth tool with the given variable store.
func NewBasicTool(varStore *tools.VariableStore) *BasicTool {
	return &BasicTool{varStore: varStore}
}

// BasicParams defines the parameters for HTTP Basic authentication.
type BasicParams struct {
	// Username for authentication
	Username string `json:"username"`
	// Password for authentication
	Password string `json:"password"`
	// SaveAs is the optional variable name to save the Authorization header
	SaveAs string `json:"save_as,omitempty"`
}

// Name returns the tool name.
func (t *BasicTool) Name() string {
	return "auth_basic"
}

// Description returns a human-readable description of the tool.
func (t *BasicTool) Description() string {
	return "Create HTTP Basic authentication header. Encodes username:password in base64 and saves 'Authorization: Basic <encoded>' to a variable."
}

// Parameters returns an example of the JSON parameters this tool accepts.
func (t *BasicTool) Parameters() string {
	return `{
  "username": "admin",
  "password": "secret123",
  "save_as": "auth_header"
}`
}

// Execute creates a Basic authentication header from the provided credentials.
// The credentials are base64-encoded in the format "username:password".
// If save_as is specified, the header is saved to a variable for later use.
func (t *BasicTool) Execute(args string) (string, error) {
	// Substitute variables in args
	if t.varStore != nil {
		args = t.varStore.Substitute(args)
	}

	var params BasicParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse parameters: %w", err)
	}

	if params.Username == "" {
		return "", fmt.Errorf("'username' parameter is required")
	}

	if params.Password == "" {
		return "", fmt.Errorf("'password' parameter is required")
	}

	// Encode credentials as base64
	credentials := fmt.Sprintf("%s:%s", params.Username, params.Password)
	encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
	authHeader := fmt.Sprintf("Basic %s", encoded)

	// Save to variable if requested
	if params.SaveAs != "" {
		t.varStore.Set(params.SaveAs, authHeader)
		return fmt.Sprintf("Created HTTP Basic authentication header.\nUsername: %s\nSaved as: {{%s}}\n\nUse in requests:\n{\n  \"headers\": {\"Authorization\": \"{{%s}}\"}\n}",
			params.Username, params.SaveAs, params.SaveAs), nil
	}

	return fmt.Sprintf("Basic auth header: %s\n\nUse in requests:\n{\n  \"headers\": {\"Authorization\": \"%s\"}\n}",
		authHeader, authHeader), nil
}
