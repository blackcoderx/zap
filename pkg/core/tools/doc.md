# Package tools

The `tools` package provides the agent capabilities for the ZAP API debugging assistant.
Each tool implements the `core.Tool` interface and handles a specific type of operation.

## Subpackages

| Package | Description |
|---------|-------------|
| [auth/](auth/doc.md) | Authentication tools (Bearer, Basic, OAuth2) |

## Tools by Category

### HTTP
| Tool | File | Description |
|------|------|-------------|
| `http_request` | `http.go` | Make HTTP requests |

### Codebase
| Tool | File | Description |
|------|------|-------------|
| `read_file` | `file.go` | Read file contents |
| `write_file` | `write.go` | Write/update files |
| `list_files` | `file.go` | List directory contents |
| `search_code` | `search.go` | Search for patterns in code |

### Testing & Validation
| Tool | File | Description |
|------|------|-------------|
| `assert_response` | `assert.go` | Validate HTTP responses |
| `extract_value` | `extract.go` | Extract values from responses |
| `validate_json_schema` | `schema.go` | JSON Schema validation |
| `compare_responses` | `diff.go` | Compare response differences |
| `test_suite` | `suite.go` | Run test suites |

### Performance
| Tool | File | Description |
|------|------|-------------|
| `performance_test` | `perf.go` | Load testing |
| `wait` | `timing.go` | Add delays |
| `retry` | `timing.go` | Retry with backoff |

### Variables & Persistence
| Tool | File | Description |
|------|------|-------------|
| `variable` | `variables.go` | Session/global variables |
| `save_request` | `persistence.go` | Save API requests |
| `load_request` | `persistence.go` | Load saved requests |
| `list_requests` | `persistence.go` | List saved requests |
| `set_environment` | `persistence.go` | Switch environments |

### Webhooks
| Tool | File | Description |
|------|------|-------------|
| `webhook_listener` | `webhook.go` | Start webhook server |

### Memory
| Tool | File | Description |
|------|------|-------------|
| `memory` | `memory.go` | Agent memory operations |
