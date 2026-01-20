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

		fmt.Println("✓ .zap folder initialized successfully!")
	}

	return nil
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig() error {
	config := Config{
		OllamaURL:    "https://ollama.com",
		OllamaAPIKey: "", // To be filled by user
		DefaultModel: "llama3",
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
