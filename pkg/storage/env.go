package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// varPattern matches {{VAR_NAME}} or {{env:VAR_NAME}}
var varPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// LoadEnvironment loads environment variables from a YAML file
func LoadEnvironment(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment file: %w", err)
	}

	var env map[string]string
	if err := yaml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to parse environment YAML: %w", err)
	}

	// Resolve any {{env:VAR}} references to actual environment variables
	for key, value := range env {
		env[key] = resolveEnvRefs(value)
	}

	return env, nil
}

// SaveEnvironment saves environment variables to a YAML file
func SaveEnvironment(env map[string]string, filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if !strings.HasSuffix(filePath, ".yaml") && !strings.HasSuffix(filePath, ".yml") {
		filePath = filePath + ".yaml"
	}

	data, err := yaml.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal environment: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// ListEnvironments lists all environment files
func ListEnvironments(baseDir string) ([]string, error) {
	envDir := filepath.Join(baseDir, "environments")

	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	var envs []string
	entries, err := os.ReadDir(envDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read environments directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			name := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".yaml"), ".yml")
			envs = append(envs, name)
		}
	}

	return envs, nil
}

// SubstituteVariables replaces {{VAR}} placeholders with values from the environment
func SubstituteVariables(text string, env map[string]string) string {
	return varPattern.ReplaceAllStringFunc(text, func(match string) string {
		// Extract variable name (remove {{ and }})
		varName := strings.TrimPrefix(strings.TrimSuffix(match, "}}"), "{{")
		varName = strings.TrimSpace(varName)

		// Check for env: prefix (reference to system environment)
		if strings.HasPrefix(varName, "env:") {
			sysVar := strings.TrimPrefix(varName, "env:")
			if val := os.Getenv(sysVar); val != "" {
				return val
			}
			return match // Keep original if not found
		}

		// Look up in provided environment
		if val, ok := env[varName]; ok {
			return val
		}

		return match // Keep original if not found
	})
}

// ApplyEnvironment applies environment variables to a request
func ApplyEnvironment(req *Request, env map[string]string) *Request {
	applied := &Request{
		Name:    req.Name,
		Method:  req.Method,
		URL:     SubstituteVariables(req.URL, env),
		Headers: make(map[string]string),
		Query:   make(map[string]string),
		Body:    req.Body,
	}

	// Apply to headers
	for k, v := range req.Headers {
		applied.Headers[k] = SubstituteVariables(v, env)
	}

	// Apply to query params
	for k, v := range req.Query {
		applied.Query[k] = SubstituteVariables(v, env)
	}

	// Apply to body if it's a string
	if bodyStr, ok := req.Body.(string); ok {
		applied.Body = SubstituteVariables(bodyStr, env)
	}

	return applied
}

// resolveEnvRefs resolves {{env:VAR}} references in a string
func resolveEnvRefs(text string) string {
	return varPattern.ReplaceAllStringFunc(text, func(match string) string {
		varName := strings.TrimPrefix(strings.TrimSuffix(match, "}}"), "{{")
		varName = strings.TrimSpace(varName)

		if strings.HasPrefix(varName, "env:") {
			sysVar := strings.TrimPrefix(varName, "env:")
			if val := os.Getenv(sysVar); val != "" {
				return val
			}
		}
		return match
	})
}
