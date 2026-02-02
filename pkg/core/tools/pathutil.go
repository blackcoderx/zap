package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePathWithinWorkDir checks if a given path is within the allowed work directory.
// This prevents path traversal attacks (e.g., "../../../etc/passwd").
//
// Parameters:
//   - filePath: The path to validate (can be relative or absolute)
//   - workDir: The allowed work directory
//
// Returns:
//   - absPath: The resolved absolute path (only valid if err is nil)
//   - err: An error if the path is outside the work directory or invalid
//
// Security: This function ensures that:
//   - Path traversal using ".." is blocked
//   - Absolute paths outside workDir are blocked
//   - Symlink-based escapes are handled by filepath.Abs resolving to real paths
func ValidatePathWithinWorkDir(filePath, workDir string) (absPath string, err error) {
	// Resolve the file path
	targetPath := filePath
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(workDir, targetPath)
	}

	// Get absolute path (resolves "..", ".", and cleans the path)
	absPath, err = filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Get absolute work directory
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve work directory: %w", err)
	}

	// Ensure work directory ends with separator for proper prefix matching
	// This prevents bypasses like /project-evil matching /project
	if !strings.HasSuffix(absWorkDir, string(filepath.Separator)) {
		absWorkDir += string(filepath.Separator)
	}

	// Check if path is within work directory (or equals it)
	// Allow exact match to work directory itself
	if absPath != strings.TrimSuffix(absWorkDir, string(filepath.Separator)) &&
		!strings.HasPrefix(absPath, absWorkDir) {
		return "", fmt.Errorf("access denied: path outside project directory")
	}

	return absPath, nil
}
