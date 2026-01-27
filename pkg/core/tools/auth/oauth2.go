package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blackcoderx/zap/pkg/core/tools"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// OAuth2Tool handles OAuth2 authentication flows.
// It supports client_credentials and password grant types, obtaining access tokens
// and automatically saving them as variables for use in subsequent requests.
type OAuth2Tool struct {
	varStore *tools.VariableStore
}

// NewOAuth2Tool creates a new OAuth2 auth tool with the given variable store.
func NewOAuth2Tool(varStore *tools.VariableStore) *OAuth2Tool {
	return &OAuth2Tool{varStore: varStore}
}

// OAuth2Params defines the parameters for OAuth2 authentication.
type OAuth2Params struct {
	// Flow specifies the OAuth2 grant type: "client_credentials", "password"
	Flow string `json:"flow"`
	// TokenURL is the OAuth2 token endpoint URL
	TokenURL string `json:"token_url"`
	// ClientID is the OAuth2 client identifier
	ClientID string `json:"client_id"`
	// ClientSecret is the OAuth2 client secret
	ClientSecret string `json:"client_secret"`
	// Scopes are the requested OAuth2 scopes (optional)
	Scopes []string `json:"scopes,omitempty"`
	// Username is required for password flow
	Username string `json:"username,omitempty"`
	// Password is required for password flow
	Password string `json:"password,omitempty"`
	// AuthURL is for authorization_code flow (not supported in CLI mode)
	AuthURL string `json:"auth_url,omitempty"`
	// RedirectURL is for authorization_code flow (not supported in CLI mode)
	RedirectURL string `json:"redirect_url,omitempty"`
	// Code is for authorization_code flow (not supported in CLI mode)
	Code string `json:"code,omitempty"`
	// SaveTokenAs is the variable name to save the access token
	SaveTokenAs string `json:"save_token_as,omitempty"`
}

// Name returns the tool name.
func (t *OAuth2Tool) Name() string {
	return "auth_oauth2"
}

// Description returns a human-readable description of the tool.
func (t *OAuth2Tool) Description() string {
	return "Perform OAuth2 authentication flows (client_credentials, password). Obtains access token and saves to variable."
}

// Parameters returns an example of the JSON parameters this tool accepts.
func (t *OAuth2Tool) Parameters() string {
	return `{
  "flow": "client_credentials",
  "token_url": "https://auth.example.com/token",
  "client_id": "{{CLIENT_ID}}",
  "client_secret": "{{CLIENT_SECRET}}",
  "scopes": ["api:read", "api:write"],
  "save_token_as": "oauth_token"
}`
}

// Execute performs the OAuth2 authentication flow.
// Supported flows:
//   - client_credentials: Server-to-server authentication using client ID and secret
//   - password: User authentication using username and password (Resource Owner Password Credentials)
func (t *OAuth2Tool) Execute(args string) (string, error) {
	// Substitute variables in args
	if t.varStore != nil {
		args = t.varStore.Substitute(args)
	}

	var params OAuth2Params
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse parameters: %w", err)
	}

	// Validate common parameters
	if params.TokenURL == "" {
		return "", fmt.Errorf("'token_url' parameter is required")
	}
	if params.ClientID == "" {
		return "", fmt.Errorf("'client_id' parameter is required")
	}
	if params.ClientSecret == "" {
		return "", fmt.Errorf("'client_secret' parameter is required")
	}

	// Execute flow based on type
	switch params.Flow {
	case "client_credentials":
		return t.clientCredentialsFlow(params)
	case "password":
		return t.passwordFlow(params)
	case "authorization_code":
		return "", fmt.Errorf("authorization_code flow requires manual browser interaction and is not supported in CLI mode. Use 'client_credentials' or 'password' flows instead")
	default:
		return "", fmt.Errorf("unknown flow '%s' (supported: client_credentials, password)", params.Flow)
	}
}

// clientCredentialsFlow performs OAuth2 client credentials flow.
// This flow is used for server-to-server authentication where the client authenticates
// using its own credentials (client_id and client_secret).
func (t *OAuth2Tool) clientCredentialsFlow(params OAuth2Params) (string, error) {
	config := clientcredentials.Config{
		ClientID:     params.ClientID,
		ClientSecret: params.ClientSecret,
		TokenURL:     params.TokenURL,
		Scopes:       params.Scopes,
	}

	ctx := context.Background()
	token, err := config.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("OAuth2 client_credentials flow failed: %w", err)
	}

	return t.formatTokenResponse(token, params)
}

// passwordFlow performs OAuth2 password (Resource Owner Password Credentials) flow.
// This flow is used when the client has the user's credentials and exchanges them
// for an access token.
func (t *OAuth2Tool) passwordFlow(params OAuth2Params) (string, error) {
	if params.Username == "" {
		return "", fmt.Errorf("'username' parameter is required for password flow")
	}
	if params.Password == "" {
		return "", fmt.Errorf("'password' parameter is required for password flow")
	}

	config := oauth2.Config{
		ClientID:     params.ClientID,
		ClientSecret: params.ClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: params.TokenURL,
		},
		Scopes: params.Scopes,
	}

	ctx := context.Background()
	token, err := config.PasswordCredentialsToken(ctx, params.Username, params.Password)
	if err != nil {
		return "", fmt.Errorf("OAuth2 password flow failed: %w", err)
	}

	return t.formatTokenResponse(token, params)
}

// formatTokenResponse formats the OAuth2 token response and saves it to variables.
// If save_token_as is specified, both the raw token and a Bearer header are saved.
func (t *OAuth2Tool) formatTokenResponse(token *oauth2.Token, params OAuth2Params) (string, error) {
	var sb strings.Builder

	sb.WriteString("OAuth2 Authentication Successful!\n\n")
	sb.WriteString(fmt.Sprintf("Access Token: %s\n", token.AccessToken))
	sb.WriteString(fmt.Sprintf("Token Type: %s\n", token.TokenType))

	if token.RefreshToken != "" {
		sb.WriteString(fmt.Sprintf("Refresh Token: %s\n", token.RefreshToken))
	}

	if !token.Expiry.IsZero() {
		sb.WriteString(fmt.Sprintf("Expires: %s\n", token.Expiry.Format("2006-01-02 15:04:05")))
	}

	// Save token to variable if requested
	if params.SaveTokenAs != "" && t.varStore != nil {
		t.varStore.Set(params.SaveTokenAs, token.AccessToken)
		sb.WriteString(fmt.Sprintf("\nToken saved as: {{%s}}\n", params.SaveTokenAs))

		// Also save as Bearer header for convenience
		authHeaderVar := params.SaveTokenAs + "_header"
		bearerHeader := fmt.Sprintf("Bearer %s", token.AccessToken)
		t.varStore.Set(authHeaderVar, bearerHeader)
		sb.WriteString(fmt.Sprintf("Bearer header saved as: {{%s}}\n", authHeaderVar))

		sb.WriteString("\nUse in requests:\n")
		sb.WriteString("{\n")
		sb.WriteString(fmt.Sprintf("  \"headers\": {\"Authorization\": \"{{%s}}\"}\n", authHeaderVar))
		sb.WriteString("}\n")
	}

	return sb.String(), nil
}
