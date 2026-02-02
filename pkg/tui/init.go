package tui

import (
	"os"
	"time"

	"github.com/blackcoderx/zap/pkg/core"
	"github.com/blackcoderx/zap/pkg/core/tools"
	"github.com/blackcoderx/zap/pkg/core/tools/auth"
	"github.com/blackcoderx/zap/pkg/llm"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

// configureToolLimits sets up per-tool call limits from config file.
// Falls back to sensible defaults if config values are missing.
// High-risk tools (network I/O, side effects) have lower limits.
// Low-risk tools (in-memory, no side effects) have higher limits.
func configureToolLimits(agent *core.Agent) {
	// Default limits (used if config doesn't specify)
	defaultLimits := map[string]int{
		// High-risk tools (external I/O, side effects)
		"http_request":     25,
		"performance_test": 5,
		"webhook_listener": 10,
		"auth_oauth2":      10,
		"write_file":       10, // File writes require confirmation
		// Medium-risk tools (file system I/O)
		"read_file":    50,
		"list_files":   50,
		"search_code":  30,
		"save_request": 20,
		"load_request": 30,
		// Low-risk tools (in-memory, fast)
		"variable":             100,
		"assert_response":      100,
		"extract_value":        100,
		"auth_bearer":          50,
		"auth_basic":           50,
		"auth_helper":          50,
		"validate_json_schema": 50,
		"compare_responses":    30,
		// Special tools (prevent infinite loops)
		"retry":      15,
		"wait":       20,
		"test_suite": 10,
		// Memory tool
		"memory": 50,
	}

	// Set global limits from config (with defaults)
	defaultLimit := viper.GetInt("tool_limits.default_limit")
	if defaultLimit <= 0 {
		defaultLimit = 50
	}
	agent.SetDefaultLimit(defaultLimit)

	totalLimit := viper.GetInt("tool_limits.total_limit")
	if totalLimit <= 0 {
		totalLimit = 200
	}
	agent.SetTotalLimit(totalLimit)

	// Apply default per-tool limits first
	for toolName, limit := range defaultLimits {
		agent.SetToolLimit(toolName, limit)
	}

	// Override with config values if present
	perToolConfig := viper.GetStringMap("tool_limits.per_tool")
	for toolName, limitVal := range perToolConfig {
		// viper returns interface{}, need to convert to int
		var limit int
		switch v := limitVal.(type) {
		case int:
			limit = v
		case int64:
			limit = int(v)
		case float64:
			limit = int(v)
		default:
			continue // Skip invalid values
		}
		if limit > 0 {
			agent.SetToolLimit(toolName, limit)
		}
	}
}

// registerTools adds all tools to the agent.
// This includes codebase tools, persistence tools, and testing tools from all sprints.
func registerTools(agent *core.Agent, zapDir, workDir string, confirmManager *tools.ConfirmationManager, memStore *core.MemoryStore) {
	// Initialize shared components
	responseManager := tools.NewResponseManager()
	varStore := tools.NewVariableStore(zapDir)

	// Register codebase tools
	httpTool := tools.NewHTTPTool(responseManager, varStore)
	agent.RegisterTool(httpTool)
	agent.RegisterTool(tools.NewReadFileTool(workDir))
	agent.RegisterTool(tools.NewWriteFileTool(workDir, confirmManager))
	agent.RegisterTool(tools.NewListFilesTool(workDir))
	agent.RegisterTool(tools.NewSearchCodeTool(workDir))

	// Register persistence tools
	persistence := tools.NewPersistenceTool(zapDir)
	agent.RegisterTool(tools.NewSaveRequestTool(persistence))
	agent.RegisterTool(tools.NewLoadRequestTool(persistence))
	agent.RegisterTool(tools.NewListRequestsTool(persistence))
	agent.RegisterTool(tools.NewListEnvironmentsTool(persistence))
	agent.RegisterTool(tools.NewSetEnvironmentTool(persistence))

	// Register Sprint 1 testing tools
	assertTool := tools.NewAssertTool(responseManager)
	extractTool := tools.NewExtractTool(responseManager, varStore)
	agent.RegisterTool(assertTool)
	agent.RegisterTool(extractTool)
	agent.RegisterTool(tools.NewVariableTool(varStore))
	agent.RegisterTool(tools.NewWaitTool())
	agent.RegisterTool(tools.NewRetryTool(agent))

	// Register Sprint 2 tools
	agent.RegisterTool(tools.NewSchemaValidationTool(responseManager))
	agent.RegisterTool(auth.NewBearerTool(varStore))
	agent.RegisterTool(auth.NewBasicTool(varStore))
	agent.RegisterTool(auth.NewHelperTool(responseManager, varStore))
	agent.RegisterTool(tools.NewTestSuiteTool(httpTool, assertTool, extractTool, responseManager, varStore, zapDir))
	agent.RegisterTool(tools.NewCompareResponsesTool(responseManager, zapDir))

	// Register Sprint 3 tools (MVP)
	agent.RegisterTool(tools.NewPerformanceTool(httpTool, varStore))
	agent.RegisterTool(tools.NewWebhookListenerTool(varStore))
	agent.RegisterTool(auth.NewOAuth2Tool(varStore))

	// Register memory tool
	agent.RegisterTool(tools.NewMemoryTool(memStore))
}

// newLLMClient creates and configures the LLM client from Viper config.
func newLLMClient() *llm.OllamaClient {
	// Get config from viper
	ollamaURL := viper.GetString("ollama_url")
	if ollamaURL == "" {
		ollamaURL = "https://ollama.com"
	}

	ollamaAPIKey := viper.GetString("ollama_api_key")
	if ollamaAPIKey == "" {
		ollamaAPIKey = viper.GetString("OLLAMA_API_KEY")
	}

	defaultModel := viper.GetString("default_model")
	if defaultModel == "" {
		defaultModel = "llama3"
	}

	return llm.NewOllamaClient(ollamaURL, defaultModel, ollamaAPIKey)
}

// newSpinner creates a spinner with the ZAP style (dots animation).
func newSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{
			".       ",
			"..      ",
			"...     ",
			"....    ",
			".....   ",
			"......  ",
			"....... ",
			"........",
		},
		FPS: time.Second / 5,
	}
	sp.Style = lipgloss.NewStyle().Foreground(AccentColor)
	return sp
}

// newTextInput creates a text input with the ZAP style.
// No prompt prefix - clean input area.
// init.go

// newTextInput creates a text input with the ZAP style.
func newTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Ask me anything..."
	ti.Focus()
	ti.CharLimit = 2000
	ti.Width = 80
	ti.Prompt = "" // No prompt prefix

	// --- FIX STARTS HERE ---

	// We need to match the textinput background to the container background
	// defined in your tui.go (InputAreaBg)

	// 1. The text you type
	ti.TextStyle = lipgloss.NewStyle().
		Foreground(TextColor).
		Background(InputAreaBg)

	// 2. The placeholder text ("Ask me anything...")
	ti.PlaceholderStyle = lipgloss.NewStyle().
		Foreground(DimColor).
		Background(InputAreaBg)

	// 3. The blinking cursor
	ti.Cursor.Style = lipgloss.NewStyle().
		Foreground(AccentColor).
		Background(InputAreaBg)

	// --- FIX ENDS HERE ---

	return ti
}

// newGlamourRenderer creates a glamour renderer for markdown.
func newGlamourRenderer() *glamour.TermRenderer {
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	return renderer
}

// updateGlamourWidth recreates the glamour renderer with a new word wrap width.
// This is called when the terminal is resized to ensure markdown renders correctly.
func (m *Model) updateGlamourWidth(width int) {
	if width < 40 {
		width = 40
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err == nil {
		m.renderer = renderer
	}
}

// InitialModel creates and returns the initial TUI model.
// This sets up the agent, tools, and all UI components.
func InitialModel() Model {
	// Get current working directory for codebase tools
	workDir, _ := os.Getwd()

	// Get .zap directory path
	zapDir := core.ZapFolderName

	// Get model name for display
	modelName := viper.GetString("default_model")
	if modelName == "" {
		modelName = "llama3"
	}

	client := newLLMClient()
	agent := core.NewAgent(client)

	// Set framework from config for context-aware assistance
	framework := viper.GetString("framework")
	if framework == "" {
		// Fallback: read directly from config file (for first-run scenarios)
		framework = core.GetConfigFramework()
	}
	agent.SetFramework(framework)

	// Configure per-tool call limits before registering tools
	configureToolLimits(agent)

	// Create confirmation manager for file write approvals (shared between tool and TUI)
	confirmManager := tools.NewConfirmationManager()

	// Set up timeout callback to notify TUI when confirmation times out
	confirmManager.SetTimeoutCallback(func() {
		globalProgram.Send(confirmationTimeoutMsg{})
	})

	// Create memory store for persistent agent memory
	memStore := core.NewMemoryStore(zapDir)
	agent.SetMemoryStore(memStore)

	registerTools(agent, zapDir, workDir, confirmManager, memStore)

	return Model{
		textinput:        newTextInput(),
		spinner:          newSpinner(),
		logs:             []logEntry{},
		thinking:         false,
		agent:            agent,
		ready:            false,
		renderer:         newGlamourRenderer(),
		inputHistory:     []string{},
		historyIdx:       -1,
		savedInput:       "",
		status:           "idle",
		currentTool:      "",
		streamingBuffer:  "",
		modelName:        modelName,
		confirmManager:   confirmManager,
		confirmationMode: false,
		memoryStore:      memStore,

		// Initialize harmonica spring for pulsing animation
		// frequency=5.0 (moderate oscillation speed), damping=0.3 (keeps bouncing)
		animSpring: harmonica.NewSpring(harmonica.FPS(30), 5.0, 0.3),
		animPos:    0.0,
		animVel:    0.0,
		animTarget: 1.0,
	}
}

// Init initializes the Bubble Tea model.
// This is called once when the program starts.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		textinput.Blink,
		m.spinner.Tick,
		animTick(), // Start harmonica spring animation loop
	)
}
