package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackcoderx/zap/pkg/core"
	"github.com/blackcoderx/zap/pkg/core/tools"
	"github.com/blackcoderx/zap/pkg/llm"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

type chatMessage struct {
	role    string // "user" or "assistant"
	content string
}

type model struct {
	input    string
	messages []chatMessage
	thinking bool
	err      error
	width    int
	height   int
	agent    *core.Agent
	ready    bool
}

type ollamaResponseMsg struct {
	response string
	err      error
}

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

	// Register tools
	agent.RegisterTool(tools.NewHTTPTool())

	// Debugging: Print config to stderr (visible in console after quitting or if redirected)
	fmt.Fprintf(os.Stderr, "CONFIG: url=%s model=%s key_len=%d\n", ollamaURL, defaultModel, len(ollamaAPIKey))
	fmt.Fprintf(os.Stderr, "ALL KEYS: %v\n", viper.AllKeys())

	return model{
		input:    "",
		messages: []chatMessage{},
		thinking: false,
		agent:    agent,
		ready:    false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		checkOllamaConnection(m.agent),
	)
}

func checkOllamaConnection(agent *core.Agent) tea.Cmd {
	return func() tea.Msg {
		// Use the client from the agent
		// We'll expose it or just use it directly if needed, but for now let's assume agent has it
		// Actually I need to expose get client or just use it if I made it public
		return ollamaResponseMsg{err: nil} // Skip connection check for cloud initially or implement it
	}
}

func sendToOllama(agent *core.Agent, input string) tea.Cmd {
	return func() tea.Msg {
		response, err := agent.ProcessMessage(input)
		return ollamaResponseMsg{response: response, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.input != "" && !m.thinking {
				userMsg := chatMessage{
					role:    "user",
					content: m.input,
				}
				m.messages = append(m.messages, userMsg)
				m.thinking = true
				userInput := m.input
				m.input = ""

				// Send to Agent
				return m, sendToOllama(m.agent, userInput)
			}
		case "backspace":
			if len(m.input) > 0 && !m.thinking {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if !m.thinking && len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ollamaResponseMsg:
		m.thinking = false
		if msg.err != nil {
			if !m.ready {
				// Connection check failed
				m.err = fmt.Errorf("Ollama connection failed: %w\n\nMake sure Ollama is running: ollama serve", msg.err)
			} else {
				// Chat request failed
				m.messages = append(m.messages, chatMessage{
					role:    "assistant",
					content: fmt.Sprintf("Error: %v", msg.err),
				})
			}
		} else {
			if !m.ready {
				// Connection check succeeded
				m.ready = true
			} else {
				// Chat response received
				m.messages = append(m.messages, chatMessage{
					role:    "assistant",
					content: msg.response,
				})
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return ErrorStyle.Render(fmt.Sprintf("❌ %s", m.err.Error()))
	}

	// Header
	header := TitleStyle.Render("⚡ ZAP") + "\n" +
		SubtitleStyle.Render("AI-powered API testing in your terminal")

	// Messages area
	var messagesView strings.Builder

	if len(m.messages) == 0 {
		messagesView.WriteString(lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true).
			Render("👋 Hi! I'm ZAP. Ask me to test an API or help with HTTP requests.\n\nFor example:\n  • Test the GitHub API\n  • Send a GET request to https://api.github.com\n  • What's my IP?"))
	} else {
		for _, msg := range m.messages {
			if msg.role == "user" {
				messagesView.WriteString(UserMessageStyle.Render(fmt.Sprintf("You: %s", msg.content)))
			} else {
				messagesView.WriteString(AssistantMessageStyle.Render(fmt.Sprintf("ZAP: %s", msg.content)))
			}
			messagesView.WriteString("\n\n")
		}
	}

	if m.thinking {
		messagesView.WriteString(ThinkingStyle.Render("⚡ Thinking..."))
	}

	messagesBox := MessagesBoxStyle.Width(80).Render(messagesView.String())

	// Input area
	inputPrompt := "→ "
	if m.thinking {
		inputPrompt = "⏳ "
	}

	inputView := InputBoxStyle.Width(80).Render(
		lipgloss.NewStyle().Foreground(AccentColor).Bold(true).Render(inputPrompt) + m.input + "▋",
	)

	// Help
	help := HelpStyle.Render("ctrl+c or esc to quit • enter to send • Type your request above")

	// Layout
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		messagesBox,
		inputView,
		help,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
