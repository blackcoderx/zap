package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ZapFolderName = ".zap"

// Config represents the user's ZAP configuration
type Config struct {
	OllamaURL    string `json:"ollama_url"`
	OllamaAPIKey string `json:"ollama_api_key"`
	DefaultModel string `json:"default_model"`
	Theme        string `json:"theme"`
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
