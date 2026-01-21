package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SaveRequest saves a request to a YAML file
func SaveRequest(req Request, filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Ensure .yaml extension
	if !strings.HasSuffix(filePath, ".yaml") && !strings.HasSuffix(filePath, ".yml") {
		filePath = filePath + ".yaml"
	}

	data, err := yaml.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// LoadRequest loads a request from a YAML file
func LoadRequest(filePath string) (*Request, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var req Request
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &req, nil
}

// ListRequests lists all saved requests in the requests directory
func ListRequests(baseDir string) ([]string, error) {
	requestsDir := filepath.Join(baseDir, "requests")

	if _, err := os.Stat(requestsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	var files []string
	err := filepath.Walk(requestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			relPath, _ := filepath.Rel(requestsDir, path)
			files = append(files, relPath)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list requests: %w", err)
	}

	return files, nil
}

// GetRequestsDir returns the requests directory path
func GetRequestsDir(baseDir string) string {
	return filepath.Join(baseDir, "requests")
}

// GetEnvironmentsDir returns the environments directory path
func GetEnvironmentsDir(baseDir string) string {
	return filepath.Join(baseDir, "environments")
}
