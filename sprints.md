# ZAP Development Sprints

> MVP Target: 5 weeks to Hacker News launch

---

## Current State (Sprint 0 - COMPLETE)

### What We Have
- [x] Go-based TUI with Claude Code styling
- [x] Viewport with scrolling (pgup/pgdown, mouse)
- [x] Streaming responses from LLM
- [x] Status line (thinking/streaming/executing)
- [x] Input history navigation (↑/↓)
- [x] ReAct agent with event system
- [x] HTTP request tool (GET/POST/PUT/DELETE)
- [x] Ollama LLM integration with streaming
- [x] Keyboard shortcuts (ctrl+l clear, esc quit)

### Architecture
```
zap/
├── cmd/zap/main.go           # Entry point
├── pkg/
│   ├── core/
│   │   ├── init.go           # .zap initialization
│   │   ├── agent.go          # ReAct Agent + Events
│   │   └── tools/
│   │       └── http.go       # HTTP Tool
│   ├── llm/
│   │   └── ollama.go         # Ollama client + streaming
│   └── tui/
│       ├── app.go            # TUI application
│       └── styles.go         # Styling
```

---

## Sprint 1: Codebase Tools (Week 1)

### Goal
Agent can read and search your codebase to understand your project.

### User Story
> "As a developer, I want to ask ZAP 'What file handles the /users endpoint?' and get an accurate answer by searching my codebase."

### Tasks

| Task | File | Effort | Status |
|------|------|--------|--------|
| Create `read_file` tool | `pkg/core/tools/file.go` | 2h | ✅ DONE |
| Create `list_files` tool with glob patterns | `pkg/core/tools/file.go` | 3h | ✅ DONE |
| Create `search_code` tool (ripgrep/grep wrapper) | `pkg/core/tools/search.go` | 4h | ✅ DONE |
| Implement `.env` file loading | `pkg/core/env.go` | 2h | Already exists |
| Register new tools in TUI | `pkg/tui/app.go` | 1h | ✅ DONE |
| Update system prompt for codebase awareness | `pkg/core/agent.go` | 2h | ✅ DONE |

### Tool Specifications

#### `read_file`
```go
type ReadFileTool struct{}

func (t *ReadFileTool) Name() string { return "read_file" }
func (t *ReadFileTool) Description() string {
    return "Read contents of a file. Use for viewing source code, configs, etc."
}
func (t *ReadFileTool) Parameters() string {
    return `{"path": "string (required) - file path to read"}`
}
func (t *ReadFileTool) Execute(args string) (string, error)
```

#### `list_files`
```go
type ListFilesTool struct{}

func (t *ListFilesTool) Name() string { return "list_files" }
func (t *ListFilesTool) Description() string {
    return "List files in a directory. Supports glob patterns like **/*.go"
}
func (t *ListFilesTool) Parameters() string {
    return `{"path": "string - directory path", "pattern": "string - glob pattern"}`
}
```

#### `search_code`
```go
type SearchCodeTool struct{}

func (t *SearchCodeTool) Name() string { return "search_code" }
func (t *SearchCodeTool) Description() string {
    return "Search for text/regex patterns in codebase. Returns matching files and lines."
}
func (t *SearchCodeTool) Parameters() string {
    return `{"pattern": "string (required) - search pattern", "path": "string - directory to search", "file_pattern": "string - file glob like *.go"}`
}
```

### Acceptance Criteria
- [x] Agent can read any file in the project
- [x] Agent can list files with glob patterns (e.g., `**/*.go`)
- [x] Agent can search for code patterns and find matches
- [x] `.env` variables are loaded and available
- [x] All tools have proper error handling

### Definition of Done
Ask agent: "What file handles the /users endpoint?" → Agent searches, finds route handler, shows file path and relevant code.

---

## Sprint 2: Error-Code Pipeline (Week 2)

### Goal
The killer demo works: Error → Code → Insight

### User Story
> "As a developer, when my API returns a 500 error, I want ZAP to analyze the error, search my codebase, and show me exactly what's wrong."

### Tasks

| Task | File | Effort | Status |
|------|------|--------|--------|
| Enhanced system prompt for error diagnosis | `pkg/core/agent.go` | 4h | ✅ DONE |
| HTTP status code interpretation helpers | `pkg/core/tools/http.go` | 2h | ✅ DONE |
| Stack trace parsing from responses | `pkg/core/analysis.go` | 4h | ✅ DONE |
| Error context extraction | `pkg/core/analysis.go` | 3h | ✅ DONE |
| "Diagnose" command/workflow | `pkg/tui/app.go` | 4h | In prompt |
| Natural language → HTTP request | `pkg/core/agent.go` | 4h | ✅ DONE |

### System Prompt Enhancement

```markdown
You are ZAP, an AI-powered API debugging assistant. You have access to:
1. HTTP request tool - make API calls
2. File reading tool - read source code
3. Code search tool - find patterns in codebase
4. File listing tool - explore project structure

When a user makes an API request that returns an error:
1. Analyze the error response (status code, body, headers)
2. Search the codebase for relevant handlers, routes, middleware
3. Read the specific files that handle this endpoint
4. Identify the likely cause of the error
5. Explain what's wrong and where in the code

Always show:
- The file path and line number
- The relevant code snippet
- Your diagnosis
- Suggested fix (if possible)
```

### Acceptance Criteria
- [x] Agent interprets HTTP status codes correctly
- [x] Agent extracts error messages from JSON responses
- [x] Agent searches codebase for relevant handlers
- [x] Agent shows file:line references
- [x] Agent provides actionable diagnosis

### Definition of Done
1. Make request to endpoint that returns 500
2. Agent automatically searches codebase
3. Agent identifies the problematic code
4. Agent explains what's wrong
5. Total time < 30 seconds

---

## Sprint 3: Persistence & Storage (Week 3)

### Goal
Requests persist, workflow feels professional.

### User Story
> "As a developer, I want to save my API requests to YAML files so I can version control them and reuse them later."

### Tasks

| Task | File | Effort | Priority | Status |
|------|------|--------|----------|--------|
| YAML request schema definition | `pkg/storage/schema.go` | 2h | P0 | ✅ DONE |
| Save request to YAML | `pkg/storage/yaml.go` | 4h | P0 | ✅ DONE |
| Load request from YAML | `pkg/storage/yaml.go` | 3h | P0 | ✅ DONE |
| Request history in session | `pkg/tui/app.go` | 4h | P0 | ✅ DONE |
| Collections/folders structure | `pkg/storage/collections.go` | 6h | P1 | - |
| Environment variable substitution | `pkg/storage/env.go` | 4h | P1 | ✅ DONE |

### YAML Schema

```yaml
# .zap/requests/get-users.yaml
name: Get Users
method: GET
url: "{{BASE_URL}}/api/users"
headers:
  Authorization: "Bearer {{API_TOKEN}}"
  Content-Type: application/json
query:
  page: 1
  limit: 10

---
# .zap/requests/create-user.yaml
name: Create User
method: POST
url: "{{BASE_URL}}/api/users"
headers:
  Authorization: "Bearer {{API_TOKEN}}"
  Content-Type: application/json
body:
  name: "John Doe"
  email: "john@example.com"
```

### Environment Files

```yaml
# .zap/environments/dev.yaml
BASE_URL: http://localhost:3000
API_TOKEN: dev-token-123

# .zap/environments/prod.yaml
BASE_URL: https://api.example.com
API_TOKEN: "{{env:PROD_API_TOKEN}}"
```

### Acceptance Criteria
- [x] Can save current request to YAML
- [x] Can load and execute saved request
- [x] Variable substitution works (`{{VAR}}`)
- [x] Can switch environments
- [x] Files are human-readable and git-friendly

### Definition of Done
1. Save a request with variables
2. Close ZAP, reopen
3. Load the request
4. Switch environment
5. Execute successfully

---

## Sprint 4: Developer Experience (Week 4)

### Goal
Feels polished, ready for public use.

### User Story
> "As a developer, I want ZAP to feel fast, look good, and be easy to learn."

### Tasks

| Task | File | Effort | Priority |
|------|------|--------|----------|
| JSON syntax highlighting in responses | `pkg/tui/highlight.go` | 4h | P0 |
| Better error messages | `pkg/core/*.go` | 3h | P0 |
| `--help` and usage documentation | `cmd/zap/main.go` | 3h | P0 |
| `--request` CLI flag for scripting | `cmd/zap/main.go` | 4h | P1 |
| Request timing display | `pkg/tui/app.go` | 2h | P1 |
| Response size display | `pkg/tui/app.go` | 1h | P1 |
| Copy response to clipboard | `pkg/tui/app.go` | 2h | P2 |

### CLI Interface

```bash
# Interactive mode (default)
zap

# Execute saved request
zap --request get-users.yaml

# Execute with environment
zap --request get-users.yaml --env prod

# Quick request
zap GET https://api.example.com/users

# Pipe-friendly output
zap --request get-users.yaml --output json | jq '.data'
```

### Response Display Enhancement

```
───────────────────────────────────────
  200 OK  │  1.23s  │  4.5 KB
───────────────────────────────────────
{
  "users": [
    {
      "id": 1,
      "name": "John Doe",
      "email": "john@example.com"
    }
  ],
  "total": 42
}
```

### Acceptance Criteria
- [ ] JSON responses are syntax highlighted
- [ ] Response shows status, time, size
- [ ] `--help` shows all options
- [ ] Can execute requests non-interactively
- [ ] Error messages are helpful, not cryptic

### Definition of Done
Show ZAP to 3 developers who haven't seen it. They can:
1. Install it
2. Make a request
3. Understand what to do next

---

## Sprint 5: Launch Prep (Week 5)

### Goal
Ready for Hacker News, Product Hunt, and GitHub launch.

### User Story
> "As a potential user, I want to understand what ZAP does in 30 seconds and install it in one command."

### Tasks

| Task | File | Effort | Priority |
|------|------|--------|----------|
| Postman collection import | `pkg/import/postman.go` | 8h | P1 |
| Installation script (curl \| sh) | `scripts/install.sh` | 4h | P0 |
| README with demo GIF | `README.md` | 4h | P0 |
| Landing page | External | 8h | P1 |
| Demo video (30 seconds) | External | 4h | P0 |
| GitHub releases with binaries | `.github/workflows/release.yml` | 4h | P0 |

### README Structure

```markdown
# ZAP ⚡

> The API debugger that reads your code

[Demo GIF here]

## The Problem
You get a 500 error. You check logs. You grep through code. You add print statements. You waste 30 minutes.

## The Solution
ZAP analyzes the error, searches your codebase, and shows you exactly what's wrong.

## Install
curl -fsSL https://zap.dev/install.sh | sh

## Quick Start
$ zap
> GET https://api.example.com/users
  500 Internal Server Error

  thinking searching codebase for /users handler...
  tool search_code
  result Found in src/handlers/users.go:47

  tool read_file

The error is in src/handlers/users.go:47
The query is missing a WHERE clause, causing a full table scan that times out.

## Features
- 🔍 Codebase-aware debugging
- 🚀 Terminal-native (no Electron bloat)
- 🔒 Local-first (your code never leaves your machine)
- 📁 Git-friendly YAML storage
- 🤖 Works with Ollama or bring your own API key
```

### Launch Checklist

- [ ] Binaries for Linux, macOS (Intel + ARM), Windows
- [ ] Installation tested on fresh machines
- [ ] Demo video recorded and edited
- [ ] README has compelling hook
- [ ] HN post draft written
- [ ] Product Hunt page prepared
- [ ] Twitter/X announcement ready
- [ ] Discord server created

### Acceptance Criteria
- [ ] One-line install works on all platforms
- [ ] Demo video shows the killer feature
- [ ] README explains value in < 1 minute read
- [ ] Can import Postman collections

### Definition of Done
1. Fresh VM install works
2. Demo video gets "wow" reactions
3. 3 beta users successfully complete the full flow

---

## Post-Launch Backlog (Future Sprints)

### Sprint 6+: Based on User Feedback

**Likely priorities:**
- [ ] OAuth 2.0 flow support
- [ ] Request chaining (use response A in request B)
- [ ] Full RAG codebase indexing (optional)
- [ ] WebSocket support
- [ ] GraphQL support
- [ ] VS Code extension
- [ ] Team collaboration features

---

## Sprint Tracking

### Velocity Assumptions
- Solo developer: ~30-40 productive hours/week
- Each sprint: 1 week
- Buffer: 20% for unexpected issues

### Key Milestones

| Date | Milestone |
|------|-----------|
| End of Sprint 1 | Codebase tools working |
| End of Sprint 2 | **Killer demo working** |
| End of Sprint 3 | Persistence complete |
| End of Sprint 4 | Polish complete |
| End of Sprint 5 | **LAUNCH** |

---

## How to Use This Document

1. **Start each sprint** by reviewing tasks and acceptance criteria
2. **During sprint** check off completed tasks
3. **End each sprint** with Definition of Done verification
4. **Adjust** based on actual velocity and learnings

**Current Sprint:** Sprint 3 - Persistence & Storage ✅ COMPLETE (P0 tasks)

**Next Sprint:** Sprint 4 - Developer Experience
