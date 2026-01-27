# Package auth

The `auth` package provides authentication tools for the ZAP agent.

## Tools

| Tool | File | Description |
|------|------|-------------|
| `auth_bearer` | `bearer.go` | Create Bearer token headers for JWT/API tokens |
| `auth_basic` | `basic.go` | Create HTTP Basic authentication headers |
| `auth_helper` | `helper.go` | Parse JWT tokens, decode Basic auth |
| `auth_oauth2` | `oauth2.go` | OAuth2 client_credentials and password flows |

## Usage

```go
import "github.com/blackcoderx/zap/pkg/core/tools/auth"

// Create tools with a variable store
varStore := tools.NewVariableStore()
agent.RegisterTool(auth.NewBearerTool(varStore))
agent.RegisterTool(auth.NewBasicTool(varStore))
agent.RegisterTool(auth.NewHelperTool(responseManager, varStore))
agent.RegisterTool(auth.NewOAuth2Tool(varStore))
```

## Examples

### Bearer Token
```json
{"token": "{{JWT_TOKEN}}", "save_as": "auth_header"}
```

### Basic Auth
```json
{"username": "admin", "password": "secret", "save_as": "auth_header"}
```

### OAuth2 Client Credentials
```json
{
  "flow": "client_credentials",
  "token_url": "https://auth.example.com/token",
  "client_id": "{{CLIENT_ID}}",
  "client_secret": "{{CLIENT_SECRET}}",
  "save_token_as": "oauth_token"
}
```
