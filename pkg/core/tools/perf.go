package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// PerformanceTool provides load testing capabilities
type PerformanceTool struct {
	httpTool *HTTPTool
	varStore *VariableStore
}

// NewPerformanceTool creates a new performance testing tool
func NewPerformanceTool(httpTool *HTTPTool, varStore *VariableStore) *PerformanceTool {
	return &PerformanceTool{
		httpTool: httpTool,
		varStore: varStore,
	}
}

// Name returns the tool name
func (t *PerformanceTool) Name() string {
	return "performance_test"
}

// Description returns the tool description
func (t *PerformanceTool) Description() string {
	return "Run load tests against API endpoints with concurrent users and measure latency metrics (p50/p95/p99)"
}

// Parameters returns the tool parameter description
func (t *PerformanceTool) Parameters() string {
	return `{
  "request": {"method": "GET", "url": "string", "headers": {}, "body": {}},
  "duration_seconds": 30,
  "requests_per_second": 10,
  "concurrent_users": 5,
  "ramp_up_seconds": 5
}`
}

// PerformanceTestParams defines parameters for performance testing
type PerformanceTestParams struct {
	Request           HTTPRequest `json:"request"`
	DurationSeconds   int         `json:"duration_seconds"`
	RequestsPerSecond int         `json:"requests_per_second"`
	ConcurrentUsers   int         `json:"concurrent_users"`
	RampUpSeconds     int         `json:"ramp_up_seconds"`
}

// PerformanceResult holds the results of a performance test
type PerformanceResult struct {
	TotalRequests    int64         `json:"total_requests"`
	SuccessfulReqs   int64         `json:"successful_requests"`
	FailedReqs       int64         `json:"failed_requests"`
	Duration         time.Duration `json:"duration"`
	Throughput       float64       `json:"throughput_rps"` // requests per second
	LatencyP50       time.Duration `json:"latency_p50_ms"`
	LatencyP95       time.Duration `json:"latency_p95_ms"`
	LatencyP99       time.Duration `json:"latency_p99_ms"`
	MinLatency       time.Duration `json:"min_latency_ms"`
	MaxLatency       time.Duration `json:"max_latency_ms"`
	AvgLatency       time.Duration `json:"avg_latency_ms"`
	ErrorRate        float64       `json:"error_rate_percent"`
	StatusCodeCounts map[int]int64 `json:"status_codes"`
}

// Execute runs the performance test
func (t *PerformanceTool) Execute(args string) (string, error) {
	// Substitute variables if available
	if t.varStore != nil {
		args = t.varStore.Substitute(args)
	}

	var params PerformanceTestParams
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Validate parameters
	if err := t.validateParams(&params); err != nil {
		return "", err
	}

	// Run the performance test
	result, err := t.runTest(params)
	if err != nil {
		return "", err
	}

	return t.formatResult(result), nil
}

// validateParams validates performance test parameters
func (t *PerformanceTool) validateParams(params *PerformanceTestParams) error {
	if params.DurationSeconds <= 0 {
		return fmt.Errorf("duration_seconds must be greater than 0")
	}
	if params.RequestsPerSecond <= 0 {
		return fmt.Errorf("requests_per_second must be greater than 0")
	}
	if params.ConcurrentUsers <= 0 {
		return fmt.Errorf("concurrent_users must be greater than 0")
	}
	if params.RampUpSeconds < 0 {
		return fmt.Errorf("ramp_up_seconds cannot be negative")
	}
	if params.Request.Method == "" {
		return fmt.Errorf("request method is required")
	}
	if params.Request.URL == "" {
		return fmt.Errorf("request URL is required")
	}
	return nil
}

// runTest executes the performance test
func (t *PerformanceTool) runTest(params PerformanceTestParams) (*PerformanceResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(params.DurationSeconds)*time.Second)
	defer cancel()

	// Create rate limiter
	limiter := rate.NewLimiter(rate.Limit(params.RequestsPerSecond), params.RequestsPerSecond)

	// Shared state
	var (
		totalReqs      int64
		successfulReqs int64
		failedReqs     int64
		latencies      []time.Duration
		latenciesMu    sync.Mutex
		statusCodes    = make(map[int]int64)
		statusCodesMu  sync.Mutex
		wg             sync.WaitGroup
	)

	startTime := time.Now()

	// Launch concurrent workers with ramp-up
	for i := 0; i < params.ConcurrentUsers; i++ {
		wg.Add(1)

		// Calculate ramp-up delay for this worker
		var rampUpDelay time.Duration
		if params.RampUpSeconds > 0 {
			rampUpDelay = time.Duration(i*params.RampUpSeconds*1000/params.ConcurrentUsers) * time.Millisecond
		}

		go func(workerID int, delay time.Duration) {
			defer wg.Done()

			// Ramp-up delay
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
			}

			// Worker loop
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Wait for rate limiter
					if err := limiter.Wait(ctx); err != nil {
						return // Context cancelled
					}

					// Make request
					reqStart := time.Now()
					resp, err := t.httpTool.Run(params.Request)
					reqDuration := time.Since(reqStart)

					atomic.AddInt64(&totalReqs, 1)

					if err != nil {
						atomic.AddInt64(&failedReqs, 1)
					} else {
						atomic.AddInt64(&successfulReqs, 1)

						// Track latency
						latenciesMu.Lock()
						latencies = append(latencies, reqDuration)
						latenciesMu.Unlock()

						// Track status code
						statusCodesMu.Lock()
						statusCodes[resp.StatusCode]++
						statusCodesMu.Unlock()
					}
				}
			}
		}(i, rampUpDelay)
	}

	// Wait for all workers to complete
	wg.Wait()
	totalDuration := time.Since(startTime)

	// Calculate statistics
	result := &PerformanceResult{
		TotalRequests:    totalReqs,
		SuccessfulReqs:   successfulReqs,
		FailedReqs:       failedReqs,
		Duration:         totalDuration,
		StatusCodeCounts: statusCodes,
	}

	if totalReqs > 0 {
		result.Throughput = float64(totalReqs) / totalDuration.Seconds()
		result.ErrorRate = float64(failedReqs) / float64(totalReqs) * 100
	}

	// Calculate latency percentiles
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		result.MinLatency = latencies[0]
		result.MaxLatency = latencies[len(latencies)-1]
		result.LatencyP50 = latencies[percentileIndex(len(latencies), 50)]
		result.LatencyP95 = latencies[percentileIndex(len(latencies), 95)]
		result.LatencyP99 = latencies[percentileIndex(len(latencies), 99)]

		// Calculate average
		var sum time.Duration
		for _, lat := range latencies {
			sum += lat
		}
		result.AvgLatency = sum / time.Duration(len(latencies))
	}

	return result, nil
}

// percentileIndex calculates the index for a given percentile
func percentileIndex(n int, percentile int) int {
	if n == 0 {
		return 0
	}
	index := int(math.Ceil(float64(n)*float64(percentile)/100.0)) - 1
	if index < 0 {
		index = 0
	}
	if index >= n {
		index = n - 1
	}
	return index
}

// formatResult formats the performance test result
func (t *PerformanceTool) formatResult(result *PerformanceResult) string {
	output := fmt.Sprintf(`Performance Test Results
========================

Duration: %.2fs
Total Requests: %d
Successful: %d
Failed: %d
Error Rate: %.2f%%

Throughput: %.2f req/sec

Latency Statistics:
  Min:     %v
  Average: %v
  P50:     %v
  P95:     %v
  P99:     %v
  Max:     %v

Status Code Distribution:`,
		result.Duration.Seconds(),
		result.TotalRequests,
		result.SuccessfulReqs,
		result.FailedReqs,
		result.ErrorRate,
		result.Throughput,
		result.MinLatency,
		result.AvgLatency,
		result.LatencyP50,
		result.LatencyP95,
		result.LatencyP99,
		result.MaxLatency,
	)

	// Add status code distribution
	for code, count := range result.StatusCodeCounts {
		percentage := float64(count) / float64(result.SuccessfulReqs) * 100
		output += fmt.Sprintf("\n  %d: %d (%.1f%%)", code, count, percentage)
	}

	return output
}
