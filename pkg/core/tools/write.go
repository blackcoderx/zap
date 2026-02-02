package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aymanbagabas/go-udiff"
	"github.com/blackcoderx/zap/pkg/core"
)

// WriteFileTool writes or modifies files with human-in-the-loop confirmation.
type WriteFileTool struct {
	workDir        string
	confirmManager *ConfirmationManager
	eventCallback  core.EventCallback
}

// WriteFileParams defines the parameters for the write_file tool.
type WriteFileParams struct {
	Path    string `json:"path"`    // File path to write
	Content string `json:"content"` // Content to write
}

// NewWriteFileTool creates a new file writing tool.
func NewWriteFileTool(workDir string, confirmManager *ConfirmationManager) *WriteFileTool {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return &WriteFileTool{
		workDir:        workDir,
		confirmManager: confirmManager,
	}
}

// Name returns the tool name.
func (t *WriteFileTool) Name() string {
	return "write_file"
}

// Description returns the tool description.
func (t *WriteFileTool) Description() string {
	return "Write or modify a file. Shows a diff and requires user confirmation before writing. Use for code fixes."
}

// Parameters returns the tool parameter description.
func (t *WriteFileTool) Parameters() string {
	return `{"path": "string (required) - file path to write", "content": "string (required) - content to write"}`
}

// SetEventCallback sets the callback for emitting events to the TUI.
// This implements the ConfirmableTool interface.
func (t *WriteFileTool) SetEventCallback(callback core.EventCallback) {
	t.eventCallback = callback
}

// Execute writes a file after user confirmation.
func (t *WriteFileTool) Execute(args string) (string, error) {
	var params WriteFileParams

	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	if params.Content == "" {
		return "", fmt.Errorf("content is required")
	}

	// Security check: ensure path is within work directory
	absPath, err := ValidatePathWithinWorkDir(params.Path, t.workDir)
	if err != nil {
		return "", err
	}

	// Check file size limit (1MB for writes)
	if len(params.Content) > 1024*1024 {
		return "", fmt.Errorf("content too large (>1MB)")
	}

	// Read existing file content (if exists)
	var originalContent string
	isNewFile := false

	existingContent, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			isNewFile = true
			originalContent = ""
		} else {
			return "", fmt.Errorf("failed to read existing file: %w", err)
		}
	} else {
		originalContent = string(existingContent)
	}

	// Check if content is the same (no-op)
	if originalContent == params.Content {
		return "File content is already identical, no changes needed.", nil
	}

	// Generate unified diff
	diff := t.generateDiff(params.Path, originalContent, params.Content)

	// Emit confirmation_required event with the diff
	if t.eventCallback != nil {
		t.eventCallback(core.AgentEvent{
			Type: "confirmation_required",
			FileConfirmation: &core.FileConfirmation{
				FilePath:  params.Path,
				IsNewFile: isNewFile,
				Diff:      diff,
			},
		})
	}

	// Block until user responds
	approved := t.confirmManager.RequestConfirmation()

	if !approved {
		return "User rejected the file changes. The file was not modified.", nil
	}

	// Create parent directories if needed
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(absPath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	if isNewFile {
		return fmt.Sprintf("Successfully created file: %s", params.Path), nil
	}
	return fmt.Sprintf("Successfully modified file: %s", params.Path), nil
}

// generateDiff creates a unified diff between original and new content.
func (t *WriteFileTool) generateDiff(filename, original, modified string) string {
	// Use go-udiff to generate unified diff with 3 lines of context
	edits := udiff.Strings(original, modified)
	unified, err := udiff.ToUnified("a/"+filename, "b/"+filename, original, edits, 3)
	if err != nil {
		// Fallback to simple representation if diff fails
		return fmt.Sprintf("--- a/%s\n+++ b/%s\n(diff generation failed)\n", filename, filename)
	}
	return unified
}
