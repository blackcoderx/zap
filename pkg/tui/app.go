package tui

import (
	"strings"

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
	Type    string // "user", "thinking", "tool", "observation", "response", "error"
	Content string
}

// model is the Bubble Tea model
type model struct {
	viewport  viewport.Model
	textinput textinput.Model
	spinner   spinner.Model
	logs      []logEntry
	thinking  bool
	width     int
	height    int
	agent     *core.Agent
	ready     bool
	renderer  *glamour.TermRenderer
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

	client := llm.NewOllamaClient(ollamaURL, defaultModel, ollamaAPIKey)
	agent := core.NewAgent(client)
	agent.RegisterTool(tools.NewHTTPTool())

	// Create text input
	ti := textinput.New()
	ti.Placeholder = "Ask me anything..."
	ti.Focus()
	ti.CharLimit = 500
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
		textinput: ti,
		spinner:   sp,
		logs:      []logEntry{},
		thinking:  false,
		agent:     agent,
		ready:     false,
		renderer:  renderer,
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
		case "enter":
			if m.textinput.Value() != "" && !m.thinking {
				userInput := m.textinput.Value()
				m.logs = append(m.logs, logEntry{Type: "user", Content: userInput})
				m.textinput.SetValue("")
				m.thinking = true
				m.updateViewportContent()

				return m, tea.Batch(
					m.spinner.Tick,
					runAgentAsync(m.agent, userInput),
				)
			}
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
			m.logs = append(m.logs, logEntry{Type: "thinking", Content: msg.event.Content})
		case "tool_call":
			m.logs = append(m.logs, logEntry{Type: "tool", Content: msg.event.Content})
		case "observation":
			m.logs = append(m.logs, logEntry{Type: "observation", Content: msg.event.Content})
		case "answer":
			m.logs = append(m.logs, logEntry{Type: "response", Content: msg.event.Content})
		case "error":
			m.logs = append(m.logs, logEntry{Type: "error", Content: msg.event.Content})
		}
		m.updateViewportContent()
		cmds = append(cmds, m.spinner.Tick)

	case agentDoneMsg:
		m.thinking = false
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

	// Update text input
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

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

func (m *model) formatLogEntry(entry logEntry) string {
	switch entry.Type {
	case "user":
		return UserStyle.Render(UserPrefix + entry.Content)
	case "thinking":
		return ThinkingStyle.Render(ThinkingPrefix + entry.Content)
	case "tool":
		return ToolStyle.Render(ToolPrefix + entry.Content)
	case "observation":
		// Truncate long observations
		content := entry.Content
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		return ObservationStyle.Render(ObservationPrefix + content)
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

	// Input
	if m.thinking {
		b.WriteString(ThinkingStyle.Render(m.spinner.View() + " processing..."))
	} else {
		b.WriteString(m.textinput.View())
	}
	b.WriteString("\n")

	// Help
	help := HelpStyle.Render("esc to quit")
	b.WriteString(help)

	return b.String()
}

// Run starts the TUI application
func Run() error {
	m := initialModel()
	program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	_, err := program.Run()
	return err
}
