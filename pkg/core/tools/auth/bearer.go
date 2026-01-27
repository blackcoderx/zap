// Package auth provides authentication tools for the ZAP agent.
// It includes Bearer token, Basic auth, OAuth2, and utility tools.
package auth

import (
	"encoding/json"
	"fmt"

	"github.com/blackcoderx/zap/pkg/core/tools"
)

// BearerTool creates Bearer token authorization headers for JWT, API tokens, etc.
// It wraps a token in the standard "Bearer <token>" format and optionally saves it to a variable.
type BearerTool struct {
	varStore *tools.VariableStore
}

// NewBearerTool creates a new Bearer auth tool with the given variable store.
func NewBearerTool(varStore *tools.VariableStore) *BearerTool {
	return &BearerTool{varStore: varStore}
}

// BearerParams defines the parameters for Bearer token authentication.
type BearerParams struct {
	// Token is the actual token value (can use {{VAR}} for variable substitution)
	Token string `json:"token"`
	// SaveAs is the optional variable name to save the Authorization header
	SaveAs string `json:"save_as,omitempty"`
}

// Name returns the tool name.
func (t *BearerTool) Name() string {
	return "auth_bearer"
}

// Description returns a human-readable description of the tool.
func (t *BearerTool) Description() string {
	return "Create Bearer token authorization header (for JWT tokens, API tokens). Saves 'Authorization: Bearer <token>' to a variable for use in requests."
}

// Parameters returns an example of the JSON parameters this tool accepts.
func (t *BearerTool) Parameters() string {
	return `{
  "token": "{{AUTH_TOKEN}}",
  "save_as": "auth_header"
}`
}

// Execute creates a Bearer authorization header from the provided token.
// If save_as is specified, the header is saved to a variable for later use.
func (t *BearerTool) Execute(args string) (string, error) {
	// Substitute variables in args
	if t.varStore != nil {
		args = t.varStore.Substitute(args)
	}

	var params BearerParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse parameters: %w", err)
	}

	if params.Token == "" {
		return "", fmt.Errorf("'token' parameter is required")
	}

	// Create Bearer header
	authHeader := fmt.Sprintf("Bearer %s", params.Token)

	// Save to variable if requested
	if params.SaveAs != "" {
		t.varStore.Set(params.SaveAs, authHeader)
		return fmt.Sprintf("Created Bearer token authorization header.\nSaved as: {{%s}}\n\nUse in requests:\n{\n  \"headers\": {\"Authorization\": \"{{%s}}\"}\n}",
			params.SaveAs, params.SaveAs), nil
	}

	return fmt.Sprintf("Bearer token: %s\n\nUse in requests:\n{\n  \"headers\": {\"Authorization\": \"%s\"}\n}",
		authHeader, authHeader), nil
}
