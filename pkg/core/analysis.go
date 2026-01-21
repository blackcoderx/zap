package core

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// StackFrame represents a single frame in a stack trace
type StackFrame struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Function string `json:"function"`
	Code     string `json:"code"`
}

// ErrorContext contains extracted error information from a response
type ErrorContext struct {
	Message     string       `json:"message"`
	ErrorType   string       `json:"error_type"`
	StackFrames []StackFrame `json:"stack_frames"`
	Details     []string     `json:"details"`
	Fields      []string     `json:"fields"` // Validation error fields
}

// ParseStackTrace extracts stack frames from various stack trace formats
func ParseStackTrace(text string) []StackFrame {
	var frames []StackFrame

	// Python traceback: File "path/to/file.py", line 42, in function_name
	pythonPattern := regexp.MustCompile(`File "([^"]+)", line (\d+), in (\w+)`)
	pythonMatches := pythonPattern.FindAllStringSubmatch(text, -1)
	for _, match := range pythonMatches {
		if len(match) >= 4 {
			line := 0
			if _, err := parseIntSafe(match[2]); err == nil {
				line, _ = parseIntSafe(match[2])
			}
			frames = append(frames, StackFrame{
				File:     match[1],
				Line:     line,
				Function: match[3],
			})
		}
	}

	// Go stack trace: /path/to/file.go:42
	goPattern := regexp.MustCompile(`([^\s]+\.go):(\d+)`)
	goMatches := goPattern.FindAllStringSubmatch(text, -1)
	for _, match := range goMatches {
		if len(match) >= 3 {
			line, _ := parseIntSafe(match[2])
			frames = append(frames, StackFrame{
				File: match[1],
				Line: line,
			})
		}
	}

	// JavaScript/Node: at functionName (path/to/file.js:42:10)
	jsPattern := regexp.MustCompile(`at\s+(\w+)?\s*\(?([^:]+):(\d+):\d+\)?`)
	jsMatches := jsPattern.FindAllStringSubmatch(text, -1)
	for _, match := range jsMatches {
		if len(match) >= 4 {
			line, _ := parseIntSafe(match[3])
			frames = append(frames, StackFrame{
				File:     match[2],
				Function: match[1],
				Line:     line,
			})
		}
	}

	// Generic file:line pattern
	if len(frames) == 0 {
		genericPattern := regexp.MustCompile(`([a-zA-Z0-9_/\\.-]+\.(py|go|js|ts|java|rb)):(\d+)`)
		genericMatches := genericPattern.FindAllStringSubmatch(text, -1)
		for _, match := range genericMatches {
			if len(match) >= 4 {
				line, _ := parseIntSafe(match[3])
				frames = append(frames, StackFrame{
					File: match[1],
					Line: line,
				})
			}
		}
	}

	return frames
}

// ExtractErrorContext extracts error information from an HTTP response body
func ExtractErrorContext(body string, statusCode int) *ErrorContext {
	ctx := &ErrorContext{}

	// Try to parse as JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(body), &jsonData); err == nil {
		ctx.extractFromJSON(jsonData)
	} else {
		// Plain text - look for common patterns
		ctx.extractFromText(body)
	}

	// Parse stack traces from the body
	ctx.StackFrames = ParseStackTrace(body)

	return ctx
}

// extractFromJSON extracts error info from JSON response
func (ctx *ErrorContext) extractFromJSON(data map[string]interface{}) {
	// Common error message fields
	messageFields := []string{"message", "error", "msg", "detail", "error_description"}
	for _, field := range messageFields {
		if val, ok := data[field]; ok {
			switch v := val.(type) {
			case string:
				ctx.Message = v
				return
			case map[string]interface{}:
				// Nested error object
				ctx.extractFromJSON(v)
			}
		}
	}

	// FastAPI/Pydantic validation errors
	if detail, ok := data["detail"]; ok {
		switch d := detail.(type) {
		case []interface{}:
			// Array of validation errors
			for _, item := range d {
				if errMap, ok := item.(map[string]interface{}); ok {
					if loc, ok := errMap["loc"].([]interface{}); ok {
						field := ""
						for _, l := range loc {
							if s, ok := l.(string); ok && s != "body" {
								field = s
								break
							}
						}
						if field != "" {
							ctx.Fields = append(ctx.Fields, field)
						}
					}
					if msg, ok := errMap["msg"].(string); ok {
						ctx.Details = append(ctx.Details, msg)
					}
				}
			}
		case string:
			ctx.Message = d
		}
	}

	// Error type
	typeFields := []string{"type", "error_type", "code", "error_code"}
	for _, field := range typeFields {
		if val, ok := data[field]; ok {
			if s, ok := val.(string); ok {
				ctx.ErrorType = s
				break
			}
		}
	}
}

// extractFromText extracts error info from plain text
func (ctx *ErrorContext) extractFromText(text string) {
	lines := strings.Split(text, "\n")

	// Look for common error patterns
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Python exception
		if strings.Contains(line, "Error:") || strings.Contains(line, "Exception:") {
			ctx.Message = line
			// Extract error type
			if idx := strings.Index(line, ":"); idx > 0 {
				ctx.ErrorType = strings.TrimSpace(line[:idx])
			}
		}

		// Generic error message
		if strings.HasPrefix(strings.ToLower(line), "error") {
			ctx.Message = line
		}
	}
}

// FormatErrorContext returns a human-readable summary of the error context
func (ctx *ErrorContext) FormatErrorContext() string {
	var sb strings.Builder

	if ctx.ErrorType != "" {
		sb.WriteString("Error Type: " + ctx.ErrorType + "\n")
	}

	if ctx.Message != "" {
		sb.WriteString("Message: " + ctx.Message + "\n")
	}

	if len(ctx.Fields) > 0 {
		sb.WriteString("Invalid Fields: " + strings.Join(ctx.Fields, ", ") + "\n")
	}

	if len(ctx.Details) > 0 {
		sb.WriteString("Details:\n")
		for _, d := range ctx.Details {
			sb.WriteString("  - " + d + "\n")
		}
	}

	if len(ctx.StackFrames) > 0 {
		sb.WriteString("Stack Trace:\n")
		for _, frame := range ctx.StackFrames {
			if frame.Function != "" {
				sb.WriteString("  " + frame.File + ":" + strconv.Itoa(frame.Line) + " in " + frame.Function + "\n")
			} else {
				sb.WriteString("  " + frame.File + ":" + strconv.Itoa(frame.Line) + "\n")
			}
		}
	}

	return sb.String()
}

// parseIntSafe safely parses an integer from a string
func parseIntSafe(s string) (int, error) {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		} else {
			break
		}
	}
	return result, nil
}
