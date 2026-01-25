package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	ToolLimits   ToolLimitsConfig `json:"tool_limits"`
}

// InitializeZapFolder creates the .zap directory and initializes default files if they don't exist
func InitializeZapFolder() error {
	// Check if .zap exists
	if _, err := os.Stat(ZapFolderName); os.IsNotExist(err) {
		fmt.Println("🔧 Initializing .zap folder for the first time...")

		// Create .zap directory
		if err := os.Mkdir(ZapFolderName, 0755); err != nil {
			return fmt.Errorf("failed to create .zap folder: %w", err)
		}

		// Create default config.json
		if err := createDefaultConfig(); err != nil {
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

		fmt.Println("✓ .zap folder initialized successfully!")
	}

	// Ensure subdirectories exist (for upgrades from older versions)
	ensureDir(filepath.Join(ZapFolderName, "requests"))
	ensureDir(filepath.Join(ZapFolderName, "environments"))

	return nil
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

// createDefaultConfig creates a default configuration file
func createDefaultConfig() error {
	config := Config{
		OllamaURL:    "https://ollama.com",
		OllamaAPIKey: "", // To be filled by user
		DefaultModel: "qwen3-coder:480b-cloud",
		Theme:        "dark",
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
