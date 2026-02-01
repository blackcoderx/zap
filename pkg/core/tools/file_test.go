package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// makeJSONPath creates a JSON-safe path string (escapes backslashes on Windows)
func makeJSONPath(path string) string {
	b, _ := json.Marshal(path)
	return string(b) // Returns quoted and escaped string
}

func TestReadFileTool_PathValidation(t *testing.T) {
	// Create a temp directory structure for testing
	tmpDir, err := os.MkdirTemp("", "zap-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file inside the work directory
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a file outside the work directory
	outsideDir, err := os.MkdirTemp("", "zap-outside-*")
	if err != nil {
		t.Fatalf("failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret content"), 0644); err != nil {
		t.Fatalf("failed to create outside file: %v", err)
	}

	tool := NewReadFileTool(tmpDir)

	tests := []struct {
		name      string
		args      string
		wantErr   bool
		errMsg    string
	}{
		{
			name:    "valid file in work dir",
			args:    `{"path": "test.txt"}`,
			wantErr: false,
		},
		{
			name:    "absolute path within work dir",
			args:    `{"path": ` + makeJSONPath(testFile) + `}`,
			wantErr: false,
		},
		{
			name:    "path traversal attempt",
			args:    `{"path": "../../../etc/passwd"}`,
			wantErr: true,
			errMsg:  "access denied",
		},
		{
			name:    "absolute path outside work dir",
			args:    `{"path": ` + makeJSONPath(outsideFile) + `}`,
			wantErr: true,
			errMsg:  "access denied",
		},
		{
			name:    "path with .. in middle",
			args:    `{"path": "subdir/../../../secret"}`,
			wantErr: true,
			errMsg:  "access denied",
		},
		{
			name:    "empty path",
			args:    `{"path": ""}`,
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name:    "nonexistent file",
			args:    `{"path": "nonexistent.txt"}`,
			wantErr: true,
			errMsg:  "file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil (result: %q)", tt.errMsg, result)
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestListFilesTool_PathValidation(t *testing.T) {
	// Create a temp directory structure
	tmpDir, err := os.MkdirTemp("", "zap-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some test files
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create file1.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file2.go"), []byte("package sub"), 0644); err != nil {
		t.Fatalf("failed to create file2.go: %v", err)
	}

	// Create a directory outside the work dir for testing
	outsideDir, err := os.MkdirTemp("", "zap-outside-*")
	if err != nil {
		t.Fatalf("failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	tool := NewListFilesTool(tmpDir)

	tests := []struct {
		name    string
		args    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "list current directory",
			args:    `{"path": "."}`,
			wantErr: false,
		},
		{
			name:    "list with glob pattern",
			args:    `{"path": ".", "pattern": "*.go"}`,
			wantErr: false,
		},
		{
			name:    "list subdirectory",
			args:    `{"path": "subdir"}`,
			wantErr: false,
		},
		{
			name:    "recursive glob",
			args:    `{"path": ".", "pattern": "**/*.go"}`,
			wantErr: false,
		},
		{
			name:    "path traversal attempt",
			args:    `{"path": "../../../"}`,
			wantErr: true,
			errMsg:  "access denied",
		},
		{
			name:    "absolute path outside work dir",
			args:    `{"path": ` + makeJSONPath(outsideDir) + `}`,
			wantErr: true,
			errMsg:  "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil (result: %q)", tt.errMsg, result)
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWriteFileTool_PathValidation(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "zap-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a confirmation manager that auto-approves
	confirmManager := NewConfirmationManager()

	// Start a goroutine to auto-approve
	go func() {
		// Small delay to let the request come in
		confirmManager.SendResponse(true)
	}()

	tool := NewWriteFileTool(tmpDir, confirmManager)

	tests := []struct {
		name    string
		args    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty path",
			args:    `{"path": "", "content": "test"}`,
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name:    "empty content",
			args:    `{"path": "test.txt", "content": ""}`,
			wantErr: true,
			errMsg:  "content is required",
		},
		{
			name:    "path traversal attempt",
			args:    `{"path": "../../../etc/malicious", "content": "hack"}`,
			wantErr: true,
			errMsg:  "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestReadFileTool_SizeLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zap-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file larger than 100KB
	largeFile := filepath.Join(tmpDir, "large.txt")
	largeContent := make([]byte, 150*1024) // 150KB
	if err := os.WriteFile(largeFile, largeContent, 0644); err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}

	tool := NewReadFileTool(tmpDir)
	_, err = tool.Execute(`{"path": "large.txt"}`)

	if err == nil {
		t.Error("expected error for large file, got nil")
	}

	if !containsString(err.Error(), "too large") {
		t.Errorf("error = %q, want containing 'too large'", err.Error())
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
