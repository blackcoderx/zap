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

// Config represents the user's ZAP configuration
type Config struct {
	OllamaURL    string           `json:"ollama_url"`
	OllamaAPIKey string           `json:"ollama_api_key"`
	DefaultModel string           `json:"default_model"`
	Theme        string           `json:"theme"`
	Framework    string           `json:"framework"` // API framework (e.g., gin, fastapi, express)
	ToolLimits   ToolLimitsConfig `json:"tool_limits"`
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
	Framework string
	OllamaURL string
	Model     string
	APIKey    string
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

// runSetupWizard displays an interactive setup wizard on first run using the huh library.
// If frameworkFlag is non-empty, the framework selection step is skipped.
func runSetupWizard(frameworkFlag string) (*SetupResult, error) {
	// Use separate local variables for huh bindings to avoid
	// any pre-initialized value interference with input fields
	var (
		selectedFramework = frameworkFlag
		ollamaURL         string
		modelName         string
		apiKey            string
	)

	fmt.Println()
	fmt.Println("  Welcome to ZAP - AI-powered API debugging assistant")
	fmt.Println("  Let's configure your setup.")
	fmt.Println()

	// Phase 1: Collect settings
	var configGroups []*huh.Group

	// Framework selection (skip if --framework flag was provided)
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

	// Model configuration
	configGroups = append(configGroups,
		huh.NewGroup(
			huh.NewInput().
				Title("Ollama API URL").
				Description("The URL of your Ollama-compatible API endpoint.").
				Placeholder("https://ollama.com").
				Value(&ollamaURL),
			huh.NewInput().
				Title("Model name").
				Description("The model to use for AI assistance.").
				Placeholder("qwen3-coder:480b-cloud").
				Value(&modelName),
			huh.NewInput().
				Title("API Key").
				Description("Your API key for authentication.").
				Placeholder("Enter your API key...").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey),
		),
	)

	configForm := huh.NewForm(configGroups...).WithTheme(huh.ThemeDracula())
	if err := configForm.Run(); err != nil {
		return nil, fmt.Errorf("setup cancelled: %w", err)
	}

	// Build result with defaults for empty fields
	result := &SetupResult{
		Framework: selectedFramework,
		OllamaURL: ollamaURL,
		Model:     modelName,
		APIKey:    apiKey,
	}
	if result.OllamaURL == "" {
		result.OllamaURL = "https://ollama.com"
	}
	if result.Model == "" {
		result.Model = "qwen3-coder:480b-cloud"
	}

	// Phase 2: Confirmation with actual entered values
	var confirmed bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Create configuration with these settings?").
				Description(fmt.Sprintf(
					"Framework: %s\nAPI URL:   %s\nModel:     %s\nAPI Key:   %s",
					result.Framework,
					result.OllamaURL,
					result.Model,
					maskAPIKey(result.APIKey),
				)).
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
		OllamaURL:    setup.OllamaURL,
		OllamaAPIKey: setup.APIKey,
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
