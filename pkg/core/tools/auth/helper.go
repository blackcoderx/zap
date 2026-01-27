package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blackcoderx/zap/pkg/core/tools"
)

// HelperTool provides authentication utilities including JWT parsing and Basic auth decoding.
// It helps developers inspect and debug authentication tokens.
type HelperTool struct {
	responseManager *tools.ResponseManager
	varStore        *tools.VariableStore
}

// NewHelperTool creates a new auth helper tool.
func NewHelperTool(responseManager *tools.ResponseManager, varStore *tools.VariableStore) *HelperTool {
	return &HelperTool{
		responseManager: responseManager,
		varStore:        varStore,
	}
}

// HelperParams defines the parameters for auth helper operations.
type HelperParams struct {
	// Action specifies the operation: "parse_jwt", "decode_basic"
	Action string `json:"action"`
	// Token is the token to parse or decode
	Token string `json:"token,omitempty"`
	// FromBody extracts the token from a response body field (optional)
	FromBody string `json:"from_body,omitempty"`
}

// Name returns the tool name.
func (t *HelperTool) Name() string {
	return "auth_helper"
}

// Description returns a human-readable description of the tool.
func (t *HelperTool) Description() string {
	return "Auth utilities: parse JWT tokens, decode Basic auth, extract tokens from responses"
}

// Parameters returns an example of the JSON parameters this tool accepts.
func (t *HelperTool) Parameters() string {
	return `{
  "action": "parse_jwt",
  "token": "{{JWT_TOKEN}}"
}`
}

// Execute performs the requested auth helper action.
// Supported actions:
//   - parse_jwt: Decode and display JWT token claims (header, payload, signature)
//   - decode_basic: Decode Base64-encoded Basic auth credentials
func (t *HelperTool) Execute(args string) (string, error) {
	// Substitute variables
	if t.varStore != nil {
		args = t.varStore.Substitute(args)
	}

	var params HelperParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse parameters: %w", err)
	}

	switch params.Action {
	case "parse_jwt":
		return t.parseJWT(params.Token)
	case "decode_basic":
		return t.decodeBasic(params.Token)
	default:
		return "", fmt.Errorf("unknown action '%s' (use: parse_jwt, decode_basic)", params.Action)
	}
}

// parseJWT decodes and displays JWT token claims.
// JWT tokens have 3 parts: header.payload.signature
// This function decodes the header and payload to show their contents.
func (t *HelperTool) parseJWT(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("'token' parameter is required")
	}

	// Remove "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")

	// JWT has 3 parts: header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid JWT format (expected 3 parts, got %d)", len(parts))
	}

	var sb strings.Builder
	sb.WriteString("JWT Token Analysis:\n\n")

	// Decode header
	headerJSON, err := base64DecodeJWTPart(parts[0])
	if err != nil {
		sb.WriteString(fmt.Sprintf("Header: (decode error: %v)\n", err))
	} else {
		sb.WriteString("Header:\n")
		sb.WriteString(formatJSON(headerJSON))
		sb.WriteString("\n\n")
	}

	// Decode payload (claims)
	payloadJSON, err := base64DecodeJWTPart(parts[1])
	if err != nil {
		sb.WriteString(fmt.Sprintf("Payload: (decode error: %v)\n", err))
	} else {
		sb.WriteString("Payload (Claims):\n")
		sb.WriteString(formatJSON(payloadJSON))
		sb.WriteString("\n\n")

		// Parse common claims
		var claims map[string]interface{}
		if err := json.Unmarshal([]byte(payloadJSON), &claims); err == nil {
			if exp, ok := claims["exp"].(float64); ok {
				sb.WriteString(fmt.Sprintf("Expires: %v (Unix timestamp)\n", exp))
			}
			if iat, ok := claims["iat"].(float64); ok {
				sb.WriteString(fmt.Sprintf("Issued At: %v (Unix timestamp)\n", iat))
			}
			if sub, ok := claims["sub"].(string); ok {
				sb.WriteString(fmt.Sprintf("Subject: %s\n", sub))
			}
		}
	}

	sb.WriteString("\nSignature: " + parts[2] + " (not verified)\n")
	sb.WriteString("\nNote: Signature verification requires the secret key and is not performed by this tool.")

	return sb.String(), nil
}

// decodeBasic decodes Base64-encoded Basic auth credentials.
// The input should be in the format "Basic <base64>" or just the base64 string.
func (t *HelperTool) decodeBasic(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("'token' parameter is required")
	}

	// Remove "Basic " prefix if present
	encoded := strings.TrimPrefix(authHeader, "Basic ")

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode Basic auth: %w", err)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid Basic auth format (expected username:password)")
	}

	return fmt.Sprintf("Basic Auth Decoded:\nUsername: %s\nPassword: %s", parts[0], parts[1]), nil
}

// base64DecodeJWTPart decodes a JWT part with URL-safe base64.
// JWT tokens use URL-safe base64 encoding without padding.
func base64DecodeJWTPart(part string) (string, error) {
	// First try RawURLEncoding (no padding, which is standard for JWT)
	decoded, err := base64.RawURLEncoding.DecodeString(part)
	if err == nil {
		return string(decoded), nil
	}

	// If that fails, try with padding added (some encoders add padding)
	switch len(part) % 4 {
	case 2:
		part += "=="
	case 3:
		part += "="
	}

	decoded, err = base64.URLEncoding.DecodeString(part)
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT part: %w", err)
	}

	return string(decoded), nil
}

// formatJSON pretty-prints a JSON string.
func formatJSON(jsonStr string) string {
	var obj interface{}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		return jsonStr
	}

	pretty, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return jsonStr
	}

	return string(pretty)
}
