package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	"gin",       // Go - Gin
	"echo",      // Go - Echo
	"chi",       // Go - Chi
	"fiber",     // Go - Fiber
	"fastapi",   // Python - FastAPI
	"flask",     // Python - Flask
	"django",    // Python - Django REST Framework
	"express",   // Node.js - Express
	"nestjs",    // Node.js - NestJS
	"hono",      // Node.js/Bun - Hono
	"spring",    // Java - Spring Boot
	"laravel",   // PHP - Laravel
	"rails",     // Ruby - Rails
	"actix",     // Rust - Actix Web
	"axum",      // Rust - Axum
	"other",     // Other/custom framework
}

// InitializeZapFolder creates the .zap directory and initializes default files if they don't exist.
// If framework is empty and this is a first-time setup, prompts the user to select one.
func InitializeZapFolder(framework string) error {
	// Check if .zap exists
	if _, err := os.Stat(ZapFolderName); os.IsNotExist(err) {
		fmt.Println("Initializing .zap folder for the first time...")

		// Prompt for framework if not provided via flag
		if framework == "" {
			var err error
			framework, err = promptFramework()
			if err != nil {
				return fmt.Errorf("failed to get framework selection: %w", err)
			}
		}

		// Create .zap directory
		if err := os.Mkdir(ZapFolderName, 0755); err != nil {
			return fmt.Errorf("failed to create .zap folder: %w", err)
		}

		// Create default config.json with framework
		if err := createDefaultConfig(framework); err != nil {
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

		fmt.Printf("Initialized .zap folder with framework: %s\n", framework)
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

// promptFramework displays an interactive prompt for selecting a framework
func promptFramework() (string, error) {
	fmt.Println("\nSelect your API framework:")
	fmt.Println("─────────────────────────────")

	for i, fw := range SupportedFrameworks {
		fmt.Printf("  %2d. %s\n", i+1, fw)
	}

	fmt.Println()
	fmt.Print("Enter number (1-", len(SupportedFrameworks), "): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(SupportedFrameworks) {
		// Default to "other" on invalid input
		fmt.Println("Invalid selection, defaulting to 'other'")
		return "other", nil
	}

	return SupportedFrameworks[choice-1], nil
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

// createDefaultConfig creates a default configuration file with the specified framework
func createDefaultConfig(framework string) error {
	config := Config{
		OllamaURL:    "https://ollama.com",
		OllamaAPIKey: "", // To be filled by user
		DefaultModel: "qwen3-coder:480b-cloud",
		Theme:        "dark",
		Framework:    framework,
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

// createMemoryFile creates an empty memory.json file
func createMemoryFile() error {
	memory := make(map[string]interface{})
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
