package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
)

const ZapFolderName = ".zap"

// ToolLimitsConfig holds per-tool call limits configuration
type ToolLimitsConfig struct {
	DefaultLimit int            `json:"default_limit"` // Fallback limit for tools without specific limit
	TotalLimit   int            `json:"total_limit"`   // Safety cap on total tool calls per session
	PerTool      map[string]int `json:"per_tool"`      // Per-tool limits (tool_name -> max_calls)
}

// OllamaConfig holds Ollama-specific configuration
type OllamaConfig struct {
	Mode   string `json:"mode"`    // "local" or "cloud"
	URL    string `json:"url"`     // API URL
	APIKey string `json:"api_key"` // API key (for cloud mode)
}

// GeminiConfig holds Gemini-specific configuration
type GeminiConfig struct {
	APIKey string `json:"api_key"` // Gemini API key
}

// Config represents the user's ZAP configuration
type Config struct {
	Provider     string           `json:"provider"` // "ollama" or "gemini"
	OllamaConfig *OllamaConfig    `json:"ollama,omitempty"`
	GeminiConfig *GeminiConfig    `json:"gemini,omitempty"`
	DefaultModel string           `json:"default_model"`
	Theme        string           `json:"theme"`
	Framework    string           `json:"framework"` // API framework (e.g., gin, fastapi, express)
	ToolLimits   ToolLimitsConfig `json:"tool_limits"`

	// Legacy fields for backward compatibility (deprecated)
	OllamaURL    string `json:"ollama_url,omitempty"`
	OllamaAPIKey string `json:"ollama_api_key,omitempty"`
}

// SupportedFrameworks lists frameworks that ZAP recognizes
var SupportedFrameworks = []string{
	"gin",     // Go - Gin
	"echo",    // Go - Echo
	"chi",     // Go - Chi
	"fiber",   // Go - Fiber
	"fastapi", // Python - FastAPI
	"flask",   // Python - Flask
	"django",  // Python - Django REST Framework
	"express", // Node.js - Express
	"nestjs",  // Node.js - NestJS
	"hono",    // Node.js/Bun - Hono
	"spring",  // Java - Spring Boot
	"laravel", // PHP - Laravel
	"rails",   // Ruby - Rails
	"actix",   // Rust - Actix Web
	"axum",    // Rust - Axum
	"other",   // Other/custom framework
}

// SetupResult holds the collected values from the first-run setup wizard.
type SetupResult struct {
	Framework   string
	Provider    string // "ollama" or "gemini"
	OllamaMode  string // "local" or "cloud" (for Ollama only)
	OllamaURL   string // Ollama API URL
	GeminiKey   string // Gemini API key
	OllamaKey   string // Ollama API key (for cloud mode)
	Model       string
}

// frameworkGroup organizes frameworks by language for the setup wizard.
type frameworkGroup struct {
	Language   string
	Frameworks []string
}

// frameworkGroups lists frameworks grouped by language for display in the setup wizard select.
var frameworkGroups = []frameworkGroup{
	{Language: "Go", Frameworks: []string{"gin", "echo", "chi", "fiber"}},
	{Language: "Python", Frameworks: []string{"fastapi", "flask", "django"}},
	{Language: "Node.js", Frameworks: []string{"express", "nestjs", "hono"}},
	{Language: "Java", Frameworks: []string{"spring"}},
	{Language: "PHP", Frameworks: []string{"laravel"}},
	{Language: "Ruby", Frameworks: []string{"rails"}},
	{Language: "Rust", Frameworks: []string{"actix", "axum"}},
	{Language: "Other", Frameworks: []string{"other"}},
}

// buildFrameworkOptions creates huh.Option entries for all supported frameworks,
// labeled by language (e.g., "gin (Go)").
func buildFrameworkOptions() []huh.Option[string] {
	var options []huh.Option[string]
	for _, group := range frameworkGroups {
		for _, fw := range group.Frameworks {
			label := fmt.Sprintf("%s (%s)", fw, group.Language)
			if fw == "other" {
				label = "other (custom/unlisted)"
			}
			options = append(options, huh.NewOption(label, fw))
		}
	}
	return options
}

// providerOptions returns the available LLM provider options for the setup wizard.
func providerOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Ollama (local or cloud)", "ollama"),
		huh.NewOption("Gemini (Google AI)", "gemini"),
	}
}

// ollamaModeOptions returns the Ollama mode options (local vs cloud).
func ollamaModeOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Local (run on your machine)", "local"),
		huh.NewOption("Cloud (Ollama Cloud)", "cloud"),
	}
}

// runSetupWizard displays an interactive setup wizard on first run using the huh library.
// If frameworkFlag is non-empty, the framework selection step is skipped.
func runSetupWizard(frameworkFlag string) (*SetupResult, error) {
	// Use separate local variables for huh bindings to avoid
	// any pre-initialized value interference with input fields
	var (
		selectedFramework = frameworkFlag
		selectedProvider  string
		ollamaMode        string
		ollamaURL         string
		ollamaKey         string
		geminiKey         string
		modelName         string
	)

	fmt.Println()
	fmt.Println("  Welcome to ZAP - AI-powered API debugging assistant")
	fmt.Println("  Let's configure your setup.")
	fmt.Println()

	// Phase 1: Framework selection (skip if --framework flag was provided)
	var configGroups []*huh.Group
	if frameworkFlag == "" {
		configGroups = append(configGroups,
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select your API framework").
					Description("ZAP uses this to provide framework-specific debugging hints.").
					Options(buildFrameworkOptions()...).
					Value(&selectedFramework).
					Height(10),
			),
		)
	}

	// Phase 2: Provider selection
	configGroups = append(configGroups,
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select your LLM provider").
				Description("Choose which AI service to use for assistance.").
				Options(providerOptions()...).
				Value(&selectedProvider),
		),
	)

	providerForm := huh.NewForm(configGroups...).WithTheme(huh.ThemeDracula())
	if err := providerForm.Run(); err != nil {
		return nil, fmt.Errorf("setup cancelled: %w", err)
	}

	// Phase 3: Provider-specific configuration
	result := &SetupResult{
		Framework: selectedFramework,
		Provider:  selectedProvider,
	}

	if selectedProvider == "ollama" {
		// Ollama mode selection
		modeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select Ollama mode").
					Description("Local runs on your machine, Cloud uses Ollama's hosted service.").
					Options(ollamaModeOptions()...).
					Value(&ollamaMode),
			),
		).WithTheme(huh.ThemeDracula())

		if err := modeForm.Run(); err != nil {
			return nil, fmt.Errorf("setup cancelled: %w", err)
		}

		result.OllamaMode = ollamaMode

		if ollamaMode == "local" {
			// Local Ollama configuration
			localForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Ollama URL").
						Description("Local Ollama server URL (default: http://localhost:11434).").
						Placeholder("http://localhost:11434").
						Value(&ollamaURL),
					huh.NewInput().
						Title("Model name").
						Description("The model to use (must be installed locally).").
						Placeholder("llama3").
						Value(&modelName),
				),
			).WithTheme(huh.ThemeDracula())

			if err := localForm.Run(); err != nil {
				return nil, fmt.Errorf("setup cancelled: %w", err)
			}

			// Set defaults for local mode
			if ollamaURL == "" {
				ollamaURL = "http://localhost:11434"
			}
			if modelName == "" {
				modelName = "llama3"
			}

			result.OllamaURL = ollamaURL
			result.Model = modelName

		} else {
			// Cloud Ollama configuration
			cloudForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Ollama Cloud URL").
						Description("Ollama Cloud API endpoint (default: https://ollama.com).").
						Placeholder("https://ollama.com").
						Value(&ollamaURL),
					huh.NewInput().
						Title("Model name").
						Description("The cloud model to use.").
						Placeholder("qwen3-coder:480b-cloud").
						Value(&modelName),
					huh.NewInput().
						Title("API Key").
						Description("Your Ollama Cloud API key.").
						Placeholder("Enter your API key...").
						EchoMode(huh.EchoModePassword).
						Value(&ollamaKey),
				),
			).WithTheme(huh.ThemeDracula())

			if err := cloudForm.Run(); err != nil {
				return nil, fmt.Errorf("setup cancelled: %w", err)
			}

			// Set defaults for cloud mode
			if ollamaURL == "" {
				ollamaURL = "https://ollama.com"
			}
			if modelName == "" {
				modelName = "qwen3-coder:480b-cloud"
			}

			result.OllamaURL = ollamaURL
			result.OllamaKey = ollamaKey
			result.Model = modelName
		}

	} else {
		// Gemini configuration
		geminiForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Gemini API Key").
					Description("Get your API key from aistudio.google.com.").
					Placeholder("Enter your Gemini API key...").
					EchoMode(huh.EchoModePassword).
					Value(&geminiKey),
				huh.NewInput().
					Title("Model name").
					Description("The Gemini model to use (default: gemini-2.5-flash-lite).").
					Placeholder("gemini-2.5-flash-lite").
					Value(&modelName),
			),
		).WithTheme(huh.ThemeDracula())

		if err := geminiForm.Run(); err != nil {
			return nil, fmt.Errorf("setup cancelled: %w", err)
		}

		// Set defaults for Gemini
		if modelName == "" {
			modelName = "gemini-2.5-flash-lite"
		}

		result.GeminiKey = geminiKey
		result.Model = modelName
	}

	// Phase 4: Confirmation with actual entered values
	var confirmDescription string
	if result.Provider == "ollama" {
		if result.OllamaMode == "local" {
			confirmDescription = fmt.Sprintf(
				"Provider:  Ollama (local)\nFramework: %s\nURL:       %s\nModel:     %s",
				result.Framework,
				result.OllamaURL,
				result.Model,
			)
		} else {
			confirmDescription = fmt.Sprintf(
				"Provider:  Ollama (cloud)\nFramework: %s\nURL:       %s\nModel:     %s\nAPI Key:   %s",
				result.Framework,
				result.OllamaURL,
				result.Model,
				maskAPIKey(result.OllamaKey),
			)
		}
	} else {
		confirmDescription = fmt.Sprintf(
			"Provider:  Gemini\nFramework: %s\nModel:     %s\nAPI Key:   %s",
			result.Framework,
			result.Model,
			maskAPIKey(result.GeminiKey),
		)
	}

	var confirmed bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Create configuration with these settings?").
				Description(confirmDescription).
				Affirmative("Yes, create config").
				Negative("No, cancel").
				Value(&confirmed),
		),
	).WithTheme(huh.ThemeDracula())

	if err := confirmForm.Run(); err != nil {
		return nil, fmt.Errorf("confirmation cancelled: %w", err)
	}

	if !confirmed {
		return nil, fmt.Errorf("setup cancelled by user")
	}

	return result, nil
}

// maskAPIKey returns a masked version of the API key for display.
func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// InitializeZapFolder creates the .zap directory and initializes default files if they don't exist.
// If framework is empty and this is a first-time setup, prompts the user to select one.
func InitializeZapFolder(framework string) error {
	// Check if .zap exists
	if _, err := os.Stat(ZapFolderName); os.IsNotExist(err) {
		// Run interactive setup wizard on first run
		setup, err := runSetupWizard(framework)
		if err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}

		// Create .zap directory
		if err := os.Mkdir(ZapFolderName, 0755); err != nil {
			return fmt.Errorf("failed to create .zap folder: %w", err)
		}

		// Create config.json with wizard results
		if err := createDefaultConfig(setup); err != nil {
			return err
		}

		// Create empty history.jsonl
		if err := createFile(filepath.Join(ZapFolderName, "history.jsonl")); err != nil {
			return err
		}

		// Create empty memory.json
		if err := createMemoryFile(); err != nil {
			return err
		}

		// Create requests directory for saved requests
		if err := os.Mkdir(filepath.Join(ZapFolderName, "requests"), 0755); err != nil {
			return fmt.Errorf("failed to create requests folder: %w", err)
		}

		// Create environments directory for environment files
		if err := os.Mkdir(filepath.Join(ZapFolderName, "environments"), 0755); err != nil {
			return fmt.Errorf("failed to create environments folder: %w", err)
		}

		// Create default dev environment
		if err := createDefaultEnvironment(); err != nil {
			return err
		}

		// Create manifest.json
		if err := CreateManifest(ZapFolderName); err != nil {
			return err
		}

		fmt.Printf("\nInitialized .zap folder with framework: %s\n", setup.Framework)
	} else if framework != "" {
		// Update framework in existing config if provided via flag
		if err := updateConfigFramework(framework); err != nil {
			return fmt.Errorf("failed to update framework: %w", err)
		}
		fmt.Printf("Updated framework to: %s\n", framework)
	}

	// Ensure subdirectories exist (for upgrades from older versions)
	ensureDir(filepath.Join(ZapFolderName, "requests"))
	ensureDir(filepath.Join(ZapFolderName, "environments"))
	ensureDir(filepath.Join(ZapFolderName, "baselines"))

	// Ensure manifest exists (for upgrades)
	if _, err := os.Stat(filepath.Join(ZapFolderName, ManifestFilename)); os.IsNotExist(err) {
		CreateManifest(ZapFolderName)
	}

	return nil
}

// updateConfigFramework updates the framework in an existing config file
func updateConfigFramework(framework string) error {
	configPath := filepath.Join(ZapFolderName, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	config.Framework = framework

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetConfigFramework reads the framework from the config file
func GetConfigFramework() string {
	configPath := filepath.Join(ZapFolderName, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return ""
	}

	return config.Framework
}

// ensureDir creates a directory if it doesn't exist
func ensureDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Mkdir(path, 0755)
	}
}

// createDefaultEnvironment creates a default dev environment file
func createDefaultEnvironment() error {
	envContent := `# Development environment
# Add your variables here, e.g.:
# BASE_URL: http://localhost:3000
# API_TOKEN: your-dev-token
`
	envPath := filepath.Join(ZapFolderName, "environments", "dev.yaml")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		return fmt.Errorf("failed to write dev environment: %w", err)
	}
	return nil
}

// createDefaultConfig creates a default configuration file with the setup wizard results.
func createDefaultConfig(setup *SetupResult) error {
	config := Config{
		Provider:     setup.Provider,
		DefaultModel: setup.Model,
		Theme:        "dark",
		Framework:    setup.Framework,
		ToolLimits: ToolLimitsConfig{
			DefaultLimit: 50,  // Default: 50 calls per tool
			TotalLimit:   200, // Safety cap: 200 total calls per session
			PerTool: map[string]int{
				// High-risk tools (external I/O)
				"http_request":     25,
				"performance_test": 5,
				"webhook_listener": 10,
				"auth_oauth2":      10,
				// Medium-risk tools (file system)
				"read_file":    50,
				"list_files":   50,
				"search_code":  30,
				"save_request": 20,
				"load_request": 30,
				// Low-risk tools (in-memory)
				"variable":             100,
				"assert_response":      100,
				"extract_value":        100,
				"auth_bearer":          50,
				"auth_basic":           50,
				"auth_helper":          50,
				"validate_json_schema": 50,
				"compare_responses":    30,
				// Special tools
				"retry":      15,
				"wait":       20,
				"test_suite": 10,
				// Memory tool
				"memory": 50,
			},
		},
	}

	// Set provider-specific config (only for the selected provider)
	if setup.Provider == "ollama" {
		config.OllamaConfig = &OllamaConfig{
			Mode:   setup.OllamaMode,
			URL:    setup.OllamaURL,
			APIKey: setup.OllamaKey,
		}
		// Don't set GeminiConfig - it will be omitted from JSON
	} else {
		config.GeminiConfig = &GeminiConfig{
			APIKey: setup.GeminiKey,
		}
		// Don't set OllamaConfig - it will be omitted from JSON
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(ZapFolderName, "config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// createMemoryFile creates a memory.json file with versioned format
func createMemoryFile() error {
	memory := map[string]interface{}{
		"version": 1,
		"entries": []interface{}{},
	}
	data, err := json.MarshalIndent(memory, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal memory: %w", err)
	}

	memoryPath := filepath.Join(ZapFolderName, "memory.json")
	if err := os.WriteFile(memoryPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	return nil
}

// createFile creates an empty file
func createFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	defer file.Close()
	return nil
}
