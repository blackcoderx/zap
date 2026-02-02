package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads file contents
type ReadFileTool struct {
	workDir string
}

// NewReadFileTool creates a new file reading tool
func NewReadFileTool(workDir string) *ReadFileTool {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return &ReadFileTool{workDir: workDir}
}

// Name returns the tool name
func (t *ReadFileTool) Name() string {
	return "read_file"
}

// Description returns the tool description
func (t *ReadFileTool) Description() string {
	return "Read contents of a file. Use for viewing source code, configs, etc."
}

// Parameters returns the tool parameter description
func (t *ReadFileTool) Parameters() string {
	return `{"path": "string (required) - file path to read"}`
}

// Execute reads a file and returns its contents
func (t *ReadFileTool) Execute(args string) (string, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Security check: ensure path is within work directory
	absPath, err := ValidatePathWithinWorkDir(params.Path, t.workDir)
	if err != nil {
		return "", err
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", params.Path)
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Check file size - limit to 100KB
	if len(content) > 100*1024 {
		return "", fmt.Errorf("file too large (>100KB), use search_code to find specific content")
	}

	return string(content), nil
}

// ListFilesTool lists files in a directory with glob patterns
type ListFilesTool struct {
	workDir string
}

// NewListFilesTool creates a new file listing tool
func NewListFilesTool(workDir string) *ListFilesTool {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return &ListFilesTool{workDir: workDir}
}

// Name returns the tool name
func (t *ListFilesTool) Name() string {
	return "list_files"
}

// Description returns the tool description
func (t *ListFilesTool) Description() string {
	return "List files in a directory. Supports glob patterns like **/*.go, *.json"
}

// Parameters returns the tool parameter description
func (t *ListFilesTool) Parameters() string {
	return `{"path": "string - directory path (default: .)", "pattern": "string - glob pattern (e.g. **/*.go)"}`
}

// Execute lists files matching the pattern
func (t *ListFilesTool) Execute(args string) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
	}

	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Default to current directory
	searchPath := params.Path
	if searchPath == "" {
		searchPath = "."
	}

	// Security check: ensure path is within work directory
	absPath, err := ValidatePathWithinWorkDir(searchPath, t.workDir)
	if err != nil {
		return "", err
	}

	var files []string
	maxFiles := 100 // Limit results

	if params.Pattern != "" {
		// Use glob pattern
		files, err = t.globMatch(absPath, params.Pattern, maxFiles)
		if err != nil {
			return "", err
		}
	} else {
		// Just list directory contents
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			if len(files) >= maxFiles {
				break
			}
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			files = append(files, name)
		}
	}

	if len(files) == 0 {
		return "No files found", nil
	}

	// Make paths relative to work directory
	for i, f := range files {
		rel, err := filepath.Rel(t.workDir, f)
		if err == nil {
			files[i] = rel
		}
	}

	result := strings.Join(files, "\n")
	if len(files) >= maxFiles {
		result += fmt.Sprintf("\n... (showing first %d results)", maxFiles)
	}

	return result, nil
}

// globMatch recursively finds files matching a glob pattern
func (t *ListFilesTool) globMatch(basePath, pattern string, maxFiles int) ([]string, error) {
	var matches []string

	// Handle ** (recursive) patterns
	if strings.Contains(pattern, "**") {
		// Split pattern at **
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := ""
		if len(parts) > 1 {
			suffix = strings.TrimPrefix(parts[1], "/")
		}

		startPath := basePath
		if prefix != "" {
			startPath = filepath.Join(basePath, prefix)
		}

		err := filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			if len(matches) >= maxFiles {
				return filepath.SkipAll
			}

			// Skip hidden directories
			if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}

			// Skip common directories
			if info.IsDir() && (info.Name() == "node_modules" || info.Name() == "vendor" || info.Name() == ".git") {
				return filepath.SkipDir
			}

			if info.IsDir() {
				return nil
			}

			// Match suffix pattern
			if suffix != "" {
				matched, _ := filepath.Match(suffix, info.Name())
				if !matched {
					return nil
				}
			}

			matches = append(matches, path)
			return nil
		})

		if err != nil && err != filepath.SkipAll {
			return nil, err
		}
	} else {
		// Simple glob pattern
		globPath := filepath.Join(basePath, pattern)
		var err error
		matches, err = filepath.Glob(globPath)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}

		if len(matches) > maxFiles {
			matches = matches[:maxFiles]
		}
	}

	return matches, nil
}
