package tui

import (
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/blackcoderx/zap/pkg/core"
	"github.com/blackcoderx/zap/pkg/core/tools"
	"github.com/blackcoderx/zap/pkg/llm"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

// logEntry represents a single log line in the UI
type logEntry struct {
	Type    string // "user", "thinking", "tool", "observation", "response", "error", "separator", "streaming"
	Content string
}

// model is the Bubble Tea model
type model struct {
	viewport        viewport.Model
	textinput       textinput.Model
	spinner         spinner.Model
	logs            []logEntry
	thinking        bool
	width           int
	height          int
	agent           *core.Agent
	ready           bool
	renderer        *glamour.TermRenderer
	inputHistory    []string // history of user inputs
	historyIdx      int      // current position in history (-1 = new input)
	savedInput      string   // saved input when navigating history
	status          string   // current status: "idle", "thinking", "tool:name", "streaming"
	currentTool     string   // name of tool currently being executed
	streamingBuffer string   // buffer for accumulating streaming content
}

// agentEventMsg wraps an agent event for the TUI
type agentEventMsg struct {
	event core.AgentEvent
}

// agentDoneMsg signals the agent has finished
type agentDoneMsg struct {
	err error
}

// program reference for sending messages from goroutines
var program *tea.Program

func initialModel() model {
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

	// Get current working directory for codebase tools
	workDir, _ := os.Getwd()

	// Get .zap directory path
	zapDir := core.ZapFolderName

	client := llm.NewOllamaClient(ollamaURL, defaultModel, ollamaAPIKey)
	agent := core.NewAgent(client)

	// Initialize shared components
	responseManager := tools.NewResponseManager()
	varStore := tools.NewVariableStore(zapDir)

	// Register codebase tools
	httpTool := tools.NewHTTPTool(responseManager, varStore)
	agent.RegisterTool(httpTool)
	agent.RegisterTool(tools.NewReadFileTool(workDir))
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
	agent.RegisterTool(tools.NewAuthBearerTool(varStore))
	agent.RegisterTool(tools.NewAuthBasicTool(varStore))
	agent.RegisterTool(tools.NewAuthHelperTool(responseManager, varStore))
	agent.RegisterTool(tools.NewTestSuiteTool(httpTool, assertTool, extractTool, responseManager, varStore, zapDir))
	agent.RegisterTool(tools.NewCompareResponsesTool(responseManager, zapDir))

	// Register Sprint 3 tools (MVP)
	agent.RegisterTool(tools.NewPerformanceTool(httpTool, varStore))
	agent.RegisterTool(tools.NewWebhookListenerTool(varStore))
	agent.RegisterTool(tools.NewAuthOAuth2Tool(varStore))

	// Create text input (single line, auto-wraps visually)
	ti := textinput.New()
	ti.Placeholder = "Ask me anything..."
	ti.Focus()
	ti.CharLimit = 2000
	ti.Width = 80
	ti.Prompt = PromptStyle.Render(UserPrefix)

	// Create spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(AccentColor)

	// Create glamour renderer for markdown
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return model{
		textinput:       ti,
		spinner:         sp,
		logs:            []logEntry{},
		thinking:        false,
		agent:           agent,
		ready:           false,
		renderer:        renderer,
		inputHistory:    []string{},
		historyIdx:      -1,
		savedInput:      "",
		status:          "idle",
		currentTool:     "",
		streamingBuffer: "",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		textinput.Blink,
		m.spinner.Tick,
	)
}

// runAgentAsync starts the agent in a goroutine and sends events via the program
func runAgentAsync(agent *core.Agent, input string) tea.Cmd {
	return func() tea.Msg {
		// Run agent in goroutine so we can send intermediate events
		go func() {
			callback := func(event core.AgentEvent) {
				if program != nil {
					program.Send(agentEventMsg{event: event})
				}
			}

			_, err := agent.ProcessMessageWithEvents(input, callback)
			if program != nil {
				program.Send(agentDoneMsg{err: err})
			}
		}()

		// Return nil - actual results come via program.Send
		return nil
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "ctrl+l":
			// Clear screen
			m.logs = []logEntry{}
			m.streamingBuffer = ""
			m.updateViewportContent()
			return m, nil
		case "ctrl+y":
			// Copy last response to clipboard
			var lastResponse string
			for i := len(m.logs) - 1; i >= 0; i-- {
				if m.logs[i].Type == "response" {
					lastResponse = m.logs[i].Content
					break
				}
			}
			if lastResponse != "" {
				_ = clipboard.WriteAll(lastResponse)
				// Determine command to flash status?
				// For now, we'll just rely on the user knowing it worked,
				// or maybe we briefly change the status text?
				// Since status is "idle", we can't easily override it without a timer.
				// Let's just do it silently for now or we could add a temporary "copied" state.
			}
			return m, nil
		case "ctrl+u":
			// Clear input
			m.textinput.SetValue("")
			m.historyIdx = -1
			return m, nil
		case "up":
			// Navigate history backwards
			if !m.thinking && len(m.inputHistory) > 0 {
				if m.historyIdx == -1 {
					// Save current input before navigating
					m.savedInput = m.textinput.Value()
					m.historyIdx = len(m.inputHistory) - 1
				} else if m.historyIdx > 0 {
					m.historyIdx--
				}
				m.textinput.SetValue(m.inputHistory[m.historyIdx])
				m.textinput.CursorEnd()
				return m, nil
			}
		case "down":
			// Navigate history forwards
			if !m.thinking && m.historyIdx != -1 {
				if m.historyIdx < len(m.inputHistory)-1 {
					m.historyIdx++
					m.textinput.SetValue(m.inputHistory[m.historyIdx])
				} else {
					// Return to saved input
					m.historyIdx = -1
					m.textinput.SetValue(m.savedInput)
				}
				m.textinput.CursorEnd()
				return m, nil
			}
		case "enter":
			// Send message with enter
			if m.textinput.Value() != "" && !m.thinking {
				userInput := strings.TrimSpace(m.textinput.Value())
				if userInput == "" {
					return m, nil
				}
				// Add separator if there are previous logs
				if len(m.logs) > 0 {
					m.logs = append(m.logs, logEntry{Type: "separator", Content: ""})
				}
				m.logs = append(m.logs, logEntry{Type: "user", Content: userInput})
				// Add to history
				m.inputHistory = append(m.inputHistory, userInput)
				m.historyIdx = -1
				m.savedInput = ""
				m.textinput.SetValue("")
				m.thinking = true
				m.status = "thinking"
				m.streamingBuffer = ""
				m.updateViewportContent()

				return m, tea.Batch(
					m.spinner.Tick,
					runAgentAsync(m.agent, userInput),
				)
			}
		case "pgup", "pgdown", "home", "end":
			// Let viewport handle these for scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Initialize or resize viewport
		headerHeight := 2
		inputHeight := 2
		helpHeight := 1
		viewportHeight := m.height - headerHeight - inputHeight - helpHeight - 2

		if viewportHeight < 5 {
			viewportHeight = 5
		}

		if !m.ready {
			m.viewport = viewport.New(m.width-2, viewportHeight)
			m.viewport.SetContent("")
			m.ready = true
		} else {
			m.viewport.Width = m.width - 2
			m.viewport.Height = viewportHeight
		}

		m.textinput.Width = m.width - 6

	case agentEventMsg:
		// Handle agent events
		switch msg.event.Type {
		case "thinking":
			// Clear streaming buffer when starting new thinking
			if m.streamingBuffer != "" {
				m.streamingBuffer = ""
			}
			m.logs = append(m.logs, logEntry{Type: "thinking", Content: msg.event.Content})
			m.status = "thinking"
		case "streaming":
			// Append chunk to streaming buffer and update display
			m.streamingBuffer += msg.event.Content
			m.status = "streaming"
			// Update or add streaming log entry
			if len(m.logs) > 0 && m.logs[len(m.logs)-1].Type == "streaming" {
				m.logs[len(m.logs)-1].Content = m.streamingBuffer
			} else {
				m.logs = append(m.logs, logEntry{Type: "streaming", Content: m.streamingBuffer})
			}
		case "tool_call":
			// Clear streaming when tool is called
			m.streamingBuffer = ""
			m.logs = append(m.logs, logEntry{Type: "tool", Content: msg.event.Content})
			m.status = "tool"
			m.currentTool = msg.event.Content
		case "observation":
			m.logs = append(m.logs, logEntry{Type: "observation", Content: msg.event.Content})
			m.status = "thinking"
			m.currentTool = ""
		case "answer":
			// Replace streaming entry with final response if exists
			if len(m.logs) > 0 && m.logs[len(m.logs)-1].Type == "streaming" {
				m.logs[len(m.logs)-1] = logEntry{Type: "response", Content: msg.event.Content}
			} else {
				m.logs = append(m.logs, logEntry{Type: "response", Content: msg.event.Content})
			}
			m.streamingBuffer = ""
			m.status = "idle"
		case "error":
			m.logs = append(m.logs, logEntry{Type: "error", Content: msg.event.Content})
			m.streamingBuffer = ""
			m.status = "idle"
		}
		m.updateViewportContent()
		cmds = append(cmds, m.spinner.Tick)

	case agentDoneMsg:
		m.thinking = false
		m.status = "idle"
		m.currentTool = ""
		if msg.err != nil {
			m.logs = append(m.logs, logEntry{Type: "error", Content: msg.err.Error()})
		}
		m.updateViewportContent()

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update textinput
	if !m.thinking {
		var cmd tea.Cmd
		m.textinput, cmd = m.textinput.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) updateViewportContent() {
	var content strings.Builder

	for _, entry := range m.logs {
		line := m.formatLogEntry(entry)
		content.WriteString(line)
		content.WriteString("\n")
	}

	// Check if we were at the bottom before updating
	atBottom := m.viewport.AtBottom()

	m.viewport.SetContent(content.String())

	// Only auto-scroll to bottom if we were already at the bottom
	// This allows users to scroll up and read history
	if atBottom || m.thinking {
		m.viewport.GotoBottom()
	}
}

func (m *model) formatLogEntry(entry logEntry) string {
	switch entry.Type {
	case "user":
		// Handle multi-line user input
		lines := strings.Split(entry.Content, "\n")
		if len(lines) == 1 {
			return UserStyle.Render(UserPrefix + entry.Content)
		}
		// Multi-line: prefix first line, indent rest
		var result strings.Builder
		result.WriteString(UserStyle.Render(UserPrefix + lines[0]))
		for _, line := range lines[1:] {
			result.WriteString("\n")
			result.WriteString(UserStyle.Render("  " + line))
		}
		return result.String()
	case "thinking":
		return ThinkingStyle.Render(ThinkingPrefix + entry.Content)
	case "tool":
		return ToolStyle.Render(ToolPrefix + entry.Content)
	case "observation":
		// If the observation contains markdown code blocks, render with Glamour
		if strings.Contains(entry.Content, "```") && m.renderer != nil {
			rendered, err := m.renderer.Render(entry.Content)
			if err == nil {
				return strings.TrimSpace(rendered)
			}
		}

		// Format observation with better truncation
		content := entry.Content
		if len(content) > 1000 {
			// Show first 800 chars and last 100 chars
			content = content[:800] + " ... " + content[len(content)-100:]
		}
		return ObservationStyle.Render(ObservationPrefix + content)
	case "streaming":
		// Show streaming content as-is (raw LLM output)
		return ThinkingStyle.Render("  " + entry.Content)
	case "response":
		// Render markdown for responses
		if m.renderer != nil {
			rendered, err := m.renderer.Render(entry.Content)
			if err == nil {
				return strings.TrimSpace(rendered)
			}
		}
		return ResponseStyle.Render(entry.Content)
	case "error":
		return ErrorStyle.Render(ErrorPrefix + entry.Content)
	case "separator":
		return SeparatorStyle.Render(Separator)
	default:
		return entry.Content
	}
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Header
	title := lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		Render("zap")
	subtitle := HelpStyle.Render(" - AI-powered API testing")
	b.WriteString(title + subtitle + "\n\n")

	// Viewport (logs)
	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")

	// Input or status line
	if m.thinking {
		statusText := m.renderStatus()
		b.WriteString(statusText)
	} else {
		b.WriteString(m.textinput.View())
	}
	b.WriteString("\n")

	// Help line with shortcuts
	b.WriteString(m.renderHelp())

	return b.String()
}

// renderStatus renders the current agent status
func (m model) renderStatus() string {
	switch m.status {
	case "thinking":
		return StatusActiveStyle.Render(m.spinner.View() + " thinking...")
	case "streaming":
		return StatusActiveStyle.Render(m.spinner.View() + " streaming...")
	case "tool":
		return StatusToolStyle.Render(m.spinner.View() + " executing " + m.currentTool)
	default:
		return StatusIdleStyle.Render("ready")
	}
}

// renderHelp renders the help line with keyboard shortcuts
func (m model) renderHelp() string {
	var parts []string

	if !m.thinking {
		parts = append(parts, ShortcutKeyStyle.Render("↑↓")+ShortcutDescStyle.Render(" history"))
	}
	parts = append(parts, ShortcutKeyStyle.Render("ctrl+l")+ShortcutDescStyle.Render(" clear"))
	parts = append(parts, ShortcutKeyStyle.Render("ctrl+y")+ShortcutDescStyle.Render(" copy"))
	parts = append(parts, ShortcutKeyStyle.Render("esc")+ShortcutDescStyle.Render(" quit"))

	return strings.Join(parts, "  ")
}

// Run starts the TUI application
func Run() error {
	m := initialModel()
	program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	_, err := program.Run()
	return err
}
