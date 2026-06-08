package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	
	"github.com/deathrashed/rrunner/internal/actions"
	"github.com/deathrashed/rrunner/internal/config"
	"github.com/deathrashed/rrunner/internal/logging"
	"github.com/deathrashed/rrunner/internal/metrics"
	"github.com/deathrashed/rrunner/internal/utils"
)

// TestSuite provides a comprehensive testing framework
type TestSuite struct {
	name        string
	tests       []TestCase
	setup       func() error
	teardown    func() error
	config      *config.Config
	logger      *logging.Logger
	metrics     *metrics.Metrics
	tempDir     string
	mu          sync.RWMutex
	results     []TestResult
}

// TestCase represents a single test case
type TestCase struct {
	Name        string
	Description string
	Setup       func() error
	Test        func() error
	Teardown    func() error
	Timeout     time.Duration
	Skip        bool
	Tags        []string
	Expected    interface{}
	Actual      interface{}
}

// TestResult represents the result of a test execution
type TestResult struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Status      TestStatus    `json:"status"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Output      string        `json:"output,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
}

// TestStatus represents the status of a test
type TestStatus string

const (
	StatusPassed  TestStatus = "passed"
	StatusFailed  TestStatus = "failed"
	StatusSkipped TestStatus = "skipped"
	StatusTimeout TestStatus = "timeout"
)

// NewTestSuite creates a new test suite
func NewTestSuite(name string) *TestSuite {
	tempDir, _ := os.MkdirTemp("", "rrunner-test-")
	
	return &TestSuite{
		name:    name,
		tests:   []TestCase{},
		config:  &config.DefaultConfig(),
		logger:  logging.NewLogger(logging.LevelDebug, os.Stdout, "test"),
		metrics: metrics.NewMetrics(),
		tempDir: tempDir,
		results: []TestResult{},
	}
}

// SetConfig sets the configuration for the test suite
func (ts *TestSuite) SetConfig(cfg *config.Config) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.config = cfg
}

// SetSetup sets the setup function for the entire suite
func (ts *TestSuite) SetSetup(setup func() error) {
	ts.setup = setup
}

// SetTeardown sets the teardown function for the entire suite
func (ts *TestSuite) SetTeardown(teardown func() error) {
	ts.teardown = teardown
}

// AddTest adds a test case to the suite
func (ts *TestSuite) AddTest(test TestCase) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	if test.Timeout == 0 {
		test.Timeout = 30 * time.Second
	}
	
	ts.tests = append(ts.tests, test)
}

// RunAll runs all tests in the suite
func (ts *TestSuite) RunAll() []TestResult {
	ts.logger.Info("Starting test suite: %s", ts.name)
	
	// Run setup
	if ts.setup != nil {
		if err := ts.setup(); err != nil {
			ts.logger.Error("Suite setup failed: %v", err)
			return ts.results
		}
	}
	
	// Defer teardown
	defer func() {
		if ts.teardown != nil {
			if err := ts.teardown(); err != nil {
				ts.logger.Error("Suite teardown failed: %v", err)
			}
		}
		os.RemoveAll(ts.tempDir)
	}()
	
	// Run tests
	for _, test := range ts.tests {
		result := ts.runTest(test)
		ts.mu.Lock()
		ts.results = append(ts.results, result)
		ts.mu.Unlock()
	}
	
	// Print summary
	ts.printSummary()
	
	return ts.results
}

// RunTest runs a specific test by name
func (ts *TestSuite) RunTest(name string) *TestResult {
	for _, test := range ts.tests {
		if test.Name == name {
			result := ts.runTest(test)
			ts.mu.Lock()
			ts.results = append(ts.results, result)
			ts.mu.Unlock()
			return &result
		}
	}
	return nil
}

// RunWithTags runs tests that have any of the specified tags
func (ts *TestSuite) RunWithTags(tags []string) []TestResult {
	var results []TestResult
	
	for _, test := range ts.tests {
		if utils.ContainsAny(test.Tags, tags) {
			result := ts.runTest(test)
			ts.mu.Lock()
			ts.results = append(ts.results, result)
			results = append(results, result)
			ts.mu.Unlock()
		}
	}
	
	return results
}

func (ts *TestSuite) runTest(test TestCase) TestResult {
	start := time.Now()
	
	result := TestResult{
		Name:        test.Name,
		Description: test.Description,
		Tags:        test.Tags,
		Timestamp:   start,
		Status:      StatusPassed,
	}
	
	ts.logger.Info("Running test: %s", test.Name)
	
	// Check if test should be skipped
	if test.Skip {
		result.Status = StatusSkipped
		result.Duration = time.Since(start)
		ts.logger.Info("Test skipped: %s", test.Name)
		return result
	}
	
	// Create timeout context
	ctx, cancel := context.WithTimeout(context.Background(), test.Timeout)
	defer cancel()
	
	// Run test in goroutine to handle timeout
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %v", r)
			}
		}()
		
		// Run test setup
		if test.Setup != nil {
			if err := test.Setup(); err != nil {
				done <- fmt.Errorf("setup failed: %v", err)
				return
			}
		}
		
		// Run actual test
		err := test.Test()
		
		// Run test teardown
		if test.Teardown != nil {
			if teardownErr := test.Teardown(); teardownErr != nil {
				if err != nil {
					err = fmt.Errorf("test failed: %v, teardown failed: %v", err, teardownErr)
				} else {
					err = fmt.Errorf("teardown failed: %v", teardownErr)
				}
			}
		}
		
		done <- err
	}()
	
	// Wait for completion or timeout
	select {
	case err := <-done:
		result.Duration = time.Since(start)
		if err != nil {
			result.Status = StatusFailed
			result.Error = err.Error()
			ts.logger.Error("Test failed: %s - %v", test.Name, err)
		} else {
			ts.logger.Info("Test passed: %s", test.Name)
		}
	case <-ctx.Done():
		result.Duration = test.Timeout
		result.Status = StatusTimeout
		result.Error = "test timed out"
		ts.logger.Error("Test timed out: %s", test.Name)
	}
	
	return result
}

func (ts *TestSuite) printSummary() {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	passed := 0
	failed := 0
	skipped := 0
	timeout := 0
	
	for _, result := range ts.results {
		switch result.Status {
		case StatusPassed:
			passed++
		case StatusFailed:
			failed++
		case StatusSkipped:
			skipped++
		case StatusTimeout:
			timeout++
		}
	}
	
	total := len(ts.results)
	
	ts.logger.Info("Test Suite Summary for '%s':", ts.name)
	ts.logger.Info("  Total: %d", total)
	ts.logger.Info("  Passed: %d", passed)
	ts.logger.Info("  Failed: %d", failed)
	ts.logger.Info("  Skipped: %d", skipped)
	ts.logger.Info("  Timeout: %d", timeout)
	
	if failed > 0 || timeout > 0 {
		ts.logger.Info("  Success Rate: %.1f%%", float64(passed)/float64(total-skipped)*100)
	}
}

// ExportResults exports test results to various formats
func (ts *TestSuite) ExportResults(format string, output io.Writer) error {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	switch strings.ToLower(format) {
	case "json":
		return ts.exportJSON(output)
	case "xml":
		return ts.exportXML(output)
	case "txt", "text":
		return ts.exportText(output)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func (ts *TestSuite) exportJSON(output io.Writer) error {
	data := map[string]interface{}{
		"suite":     ts.name,
		"timestamp": time.Now(),
		"results":   ts.results,
	}
	
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (ts *TestSuite) exportXML(output io.Writer) error {
	// JUnit XML format for CI/CD integration
	fmt.Fprintf(output, `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="%s" tests="%d" time="%.3f">
  <testsuite name="%s" tests="%d">
`, ts.name, len(ts.results), ts.totalDuration().Seconds(), ts.name, len(ts.results))
	
	for _, result := range ts.results {
		fmt.Fprintf(output, `    <testcase name="%s" classname="%s" time="%.3f"`,
			result.Name, ts.name, result.Duration.Seconds())
		
		switch result.Status {
		case StatusFailed:
			fmt.Fprintf(output, `>
      <failure message="%s">%s</failure>
    </testcase>
`, result.Error, result.Error)
		case StatusSkipped:
			fmt.Fprintf(output, `>
      <skipped/>
    </testcase>
`)
		case StatusTimeout:
			fmt.Fprintf(output, `>
      <error message="timeout">%s</error>
    </testcase>
`, result.Error)
		default:
			fmt.Fprintf(output, `/>\n`)
		}
	}
	
	fmt.Fprintf(output, `  </testsuite>
</testsuites>
`)
	
	return nil
}

func (ts *TestSuite) exportText(output io.Writer) error {
	fmt.Fprintf(output, "Test Suite: %s\n", ts.name)
	fmt.Fprintf(output, "Run at: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	
	for _, result := range ts.results {
		status := strings.ToUpper(string(result.Status))
		fmt.Fprintf(output, "[%s] %s (%.3fs)\n", status, result.Name, result.Duration.Seconds())
		
		if result.Description != "" {
			fmt.Fprintf(output, "  Description: %s\n", result.Description)
		}
		
		if result.Error != "" {
			fmt.Fprintf(output, "  Error: %s\n", result.Error)
		}
		
		if len(result.Tags) > 0 {
			fmt.Fprintf(output, "  Tags: %s\n", strings.Join(result.Tags, ", "))
		}
		
		fmt.Fprintf(output, "\n")
	}
	
	return nil
}

func (ts *TestSuite) totalDuration() time.Duration {
	var total time.Duration
	for _, result := range ts.results {
		total += result.Duration
	}
	return total
}

// AssertTrue asserts that a condition is true
func AssertTrue(t *testing.T, condition bool, message string) {
	if !condition {
		t.Errorf("Assertion failed: %s", message)
	}
}

// AssertFalse asserts that a condition is false
func AssertFalse(t *testing.T, condition bool, message string) {
	if condition {
		t.Errorf("Assertion failed: %s", message)
	}
}

// AssertEqual asserts that two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}, message string) {
	if expected != actual {
		t.Errorf("Assertion failed: %s - expected %v, got %v", message, expected, actual)
	}
}

// AssertNotEqual asserts that two values are not equal
func AssertNotEqual(t *testing.T, expected, actual interface{}, message string) {
	if expected == actual {
		t.Errorf("Assertion failed: %s - expected values to be different, but both were %v", message, expected)
	}
}

// AssertContains asserts that a string contains a substring
func AssertContains(t *testing.T, haystack, needle, message string) {
	if !strings.Contains(haystack, needle) {
		t.Errorf("Assertion failed: %s - '%s' does not contain '%s'", message, haystack, needle)
	}
}

// AssertNoError asserts that an error is nil
func AssertNoError(t *testing.T, err error, message string) {
	if err != nil {
		t.Errorf("Assertion failed: %s - unexpected error: %v", message, err)
	}
}

// AssertError asserts that an error is not nil
func AssertError(t *testing.T, err error, message string) {
	if err == nil {
		t.Errorf("Assertion failed: %s - expected error but got nil", message)
	}
}