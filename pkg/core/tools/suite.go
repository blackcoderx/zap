package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TestSuiteTool runs organized test suites
type TestSuiteTool struct {
	httpTool        *HTTPTool
	assertTool      *AssertTool
	extractTool     *ExtractTool
	responseManager *ResponseManager
	varStore        *VariableStore
	zapDir          string
}

// NewTestSuiteTool creates a new test suite tool
func NewTestSuiteTool(httpTool *HTTPTool, assertTool *AssertTool, extractTool *ExtractTool, responseManager *ResponseManager, varStore *VariableStore, zapDir string) *TestSuiteTool {
	return &TestSuiteTool{
		httpTool:        httpTool,
		assertTool:      assertTool,
		extractTool:     extractTool,
		responseManager: responseManager,
		varStore:        varStore,
		zapDir:          zapDir,
	}
}

// TestDefinition defines a single test in a suite
type TestDefinition struct {
	Name       string            `json:"name"`
	Request    HTTPRequest       `json:"request"`
	Assertions *AssertParams     `json:"assertions,omitempty"`
	Extract    map[string]string `json:"extract,omitempty"` // var_name -> json_path
}

// TestSuiteParams defines a test suite
type TestSuiteParams struct {
	Name        string           `json:"name"`
	Tests       []TestDefinition `json:"tests"`
	OnFailure   string           `json:"on_failure,omitempty"`   // "stop" or "continue"
	SaveResults bool             `json:"save_results,omitempty"` // Save to .zap/test-results/
}

// TestResult represents the result of a single test
type TestResult struct {
	Name       string        `json:"name"`
	Passed     bool          `json:"passed"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
	StatusCode int           `json:"status_code,omitempty"`
}

// SuiteResult represents the result of an entire suite
type SuiteResult struct {
	Name       string        `json:"name"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
	TotalTests int           `json:"total_tests"`
	Passed     int           `json:"passed"`
	Failed     int           `json:"failed"`
	Tests      []TestResult  `json:"tests"`
}

// Name returns the tool name
func (t *TestSuiteTool) Name() string {
	return "test_suite"
}

// Description returns the tool description
func (t *TestSuiteTool) Description() string {
	return "Run organized test suites with multiple tests, assertions, and value extraction. Tests run sequentially and can share variables."
}

// Parameters returns the tool parameter description
func (t *TestSuiteTool) Parameters() string {
	return `{
  "name": "User API Test Suite",
  "tests": [
    {
      "name": "Create user",
      "request": {"method": "POST", "url": "http://localhost:8000/api/users", "body": {"name": "Test"}},
      "assertions": {"status_code": 201},
      "extract": {"user_id": "$.id"}
    },
    {
      "name": "Get user",
      "request": {"method": "GET", "url": "http://localhost:8000/api/users/{{user_id}}"},
      "assertions": {"status_code": 200}
    }
  ],
  "on_failure": "stop"
}`
}

// Execute runs the test suite
func (t *TestSuiteTool) Execute(args string) (string, error) {
	var params TestSuiteParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse parameters: %w", err)
	}

	if params.Name == "" {
		return "", fmt.Errorf("'name' parameter is required")
	}

	if len(params.Tests) == 0 {
		return "", fmt.Errorf("'tests' array cannot be empty")
	}

	if params.OnFailure == "" {
		params.OnFailure = "stop"
	}

	// Run the test suite
	result := t.runSuite(params)

	// Save results if requested
	if params.SaveResults {
		if err := t.saveResults(result); err != nil {
			// Don't fail the whole suite if saving fails
			fmt.Fprintf(os.Stderr, "Warning: failed to save test results: %v\n", err)
		}
	}

	// Format output
	return t.formatResults(result), nil
}

// runSuite executes all tests in the suite
func (t *TestSuiteTool) runSuite(params TestSuiteParams) SuiteResult {
	result := SuiteResult{
		Name:       params.Name,
		StartTime:  time.Now(),
		TotalTests: len(params.Tests),
		Tests:      make([]TestResult, 0, len(params.Tests)),
	}

	for i, test := range params.Tests {
		testResult := t.runTest(test, i+1, len(params.Tests))
		result.Tests = append(result.Tests, testResult)

		if testResult.Passed {
			result.Passed++
		} else {
			result.Failed++
			// Stop on failure if configured
			if params.OnFailure == "stop" {
				break
			}
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	return result
}

// runTest executes a single test
func (t *TestSuiteTool) runTest(test TestDefinition, testNum, totalTests int) TestResult {
	startTime := time.Now()
	result := TestResult{
		Name:   test.Name,
		Passed: true,
	}

	// Substitute variables in request
	reqJSON, err := json.Marshal(test.Request)
	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Failed to marshal request: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Execute HTTP request
	reqArgs := t.varStore.Substitute(string(reqJSON))
	_, err = t.httpTool.Execute(reqArgs)
	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("Request failed: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Get status code from last response
	if lastResp := t.responseManager.GetHTTPResponse(); lastResp != nil {
		result.StatusCode = lastResp.StatusCode
	}

	// Run assertions if provided
	if test.Assertions != nil {
		assertJSON, err := json.Marshal(test.Assertions)
		if err != nil {
			result.Passed = false
			result.Error = fmt.Sprintf("Failed to marshal assertions: %v", err)
			result.Duration = time.Since(startTime)
			return result
		}

		assertResult, err := t.assertTool.Execute(string(assertJSON))
		if err != nil {
			result.Passed = false
			result.Error = fmt.Sprintf("Assertion failed: %v", err)
			result.Duration = time.Since(startTime)
			return result
		}

		// Check if assertions passed
		if strings.Contains(assertResult, "✗") || strings.Contains(assertResult, "failed") {
			result.Passed = false
			result.Error = assertResult
		}
	}

	// Extract values if provided
	if len(test.Extract) > 0 {
		for varName, jsonPath := range test.Extract {
			extractParams := ExtractParams{
				JSONPath: jsonPath,
				SaveAs:   varName,
			}
			extractJSON, err := json.Marshal(extractParams)
			if err != nil {
				result.Passed = false
				result.Error = fmt.Sprintf("Failed to marshal extract params: %v", err)
				result.Duration = time.Since(startTime)
				return result
			}

			_, err = t.extractTool.Execute(string(extractJSON))
			if err != nil {
				result.Passed = false
				result.Error = fmt.Sprintf("Extraction failed for '%s': %v", varName, err)
				result.Duration = time.Since(startTime)
				return result
			}
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// formatResults formats the suite results for display
func (t *TestSuiteTool) formatResults(result SuiteResult) string {
	var sb strings.Builder

	// Header
	if result.Passed == result.TotalTests {
		sb.WriteString(fmt.Sprintf("✓ Test Suite: %s - ALL PASSED\n", result.Name))
	} else {
		sb.WriteString(fmt.Sprintf("✗ Test Suite: %s - FAILURES DETECTED\n", result.Name))
	}

	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	// Summary
	sb.WriteString(fmt.Sprintf("Total: %d tests\n", result.TotalTests))
	sb.WriteString(fmt.Sprintf("Passed: %d (%.1f%%)\n", result.Passed, float64(result.Passed)/float64(result.TotalTests)*100))
	sb.WriteString(fmt.Sprintf("Failed: %d (%.1f%%)\n", result.Failed, float64(result.Failed)/float64(result.TotalTests)*100))
	sb.WriteString(fmt.Sprintf("Duration: %v\n\n", result.Duration))

	// Individual test results
	sb.WriteString("Test Results:\n")
	sb.WriteString(strings.Repeat("-", 60) + "\n\n")

	for i, test := range result.Tests {
		if test.Passed {
			sb.WriteString(fmt.Sprintf("%d. ✓ %s\n", i+1, test.Name))
			sb.WriteString(fmt.Sprintf("   Status: %d | Duration: %v\n\n", test.StatusCode, test.Duration))
		} else {
			sb.WriteString(fmt.Sprintf("%d. ✗ %s\n", i+1, test.Name))
			sb.WriteString(fmt.Sprintf("   Status: %d | Duration: %v\n", test.StatusCode, test.Duration))
			if test.Error != "" {
				sb.WriteString(fmt.Sprintf("   Error: %s\n\n", test.Error))
			}
		}
	}

	// Footer
	if result.Passed == result.TotalTests {
		sb.WriteString("\n🎉 All tests passed!\n")
	} else {
		sb.WriteString(fmt.Sprintf("\n⚠ %d test(s) failed. Review errors above.\n", result.Failed))
	}

	return sb.String()
}

// saveResults saves test results to disk
func (t *TestSuiteTool) saveResults(result SuiteResult) error {
	resultsDir := filepath.Join(t.zapDir, "test-results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return err
	}

	// Generate filename with timestamp
	timestamp := result.StartTime.Format("2006-01-02-15-04-05")
	safeName := strings.ReplaceAll(result.Name, " ", "-")
	safeName = strings.ToLower(safeName)
	filename := fmt.Sprintf("%s-%s.json", safeName, timestamp)
	resultPath := filepath.Join(resultsDir, filename)

	// Marshal results
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(resultPath, data, 0644)
}
