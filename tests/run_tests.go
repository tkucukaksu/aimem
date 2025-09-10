package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TestRunner coordinates test execution across the AIMem project
type TestRunner struct {
	projectRoot string
	verbose     bool
	timeout     time.Duration
}

// TestSuite represents a group of related tests
type TestSuite struct {
	Name        string
	Path        string
	Description string
	Tags        []string
	Timeout     time.Duration
}

// TestResult represents the outcome of a test execution
type TestResult struct {
	Suite    string
	Status   string
	Duration time.Duration
	Output   string
	Error    error
}

func main() {
	runner := &TestRunner{
		projectRoot: findProjectRoot(),
		verbose:     shouldRunVerbose(),
		timeout:     30 * time.Minute,
	}

	fmt.Println("🧪 AIMem Test Suite Runner")
	fmt.Println("==========================")
	fmt.Printf("Project Root: %s\n", runner.projectRoot)
	fmt.Printf("Verbose Mode: %v\n", runner.verbose)
	fmt.Println()

	suites := runner.getTestSuites()
	results := make([]TestResult, 0, len(suites))

	totalStart := time.Now()

	for _, suite := range suites {
		result := runner.runTestSuite(suite)
		results = append(results, result)

		runner.printTestResult(result)
	}

	totalDuration := time.Since(totalStart)
	runner.printSummary(results, totalDuration)

	if runner.hasFailures(results) {
		os.Exit(1)
	}
}

func (tr *TestRunner) getTestSuites() []TestSuite {
	return []TestSuite{
		{
			Name:        "Unit Tests",
			Path:        "./internal/...",
			Description: "Core component unit tests",
			Tags:        []string{"unit", "fast"},
			Timeout:     5 * time.Minute,
		},
		{
			Name:        "Integration Tests",
			Path:        "./tests/integration",
			Description: "Component integration tests",
			Tags:        []string{"integration", "medium"},
			Timeout:     10 * time.Minute,
		},
		{
			Name:        "E2E Tests",
			Path:        "./tests/e2e",
			Description: "End-to-end workflow tests",
			Tags:        []string{"e2e", "slow"},
			Timeout:     15 * time.Minute,
		},
		{
			Name:        "Performance Tests",
			Path:        "./tests/performance",
			Description: "Performance and benchmark tests",
			Tags:        []string{"performance", "benchmark"},
			Timeout:     20 * time.Minute,
		},
	}
}

func (tr *TestRunner) runTestSuite(suite TestSuite) TestResult {
	fmt.Printf("🔧 Running %s...\n", suite.Name)

	start := time.Now()
	result := TestResult{
		Suite: suite.Name,
	}

	// Build test command
	args := []string{"test"}

	if tr.verbose {
		args = append(args, "-v")
	}

	// Add timeout
	args = append(args, "-timeout", suite.Timeout.String())

	// Add race detection for integration and e2e tests
	if contains(suite.Tags, "integration") || contains(suite.Tags, "e2e") {
		args = append(args, "-race")
	}

	// Add benchmark flag for performance tests
	if contains(suite.Tags, "performance") {
		args = append(args, "-bench=.")
	}

	// Add path
	args = append(args, suite.Path)

	// Execute test
	cmd := exec.Command("go", args...)
	cmd.Dir = tr.projectRoot

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Output = string(output)
	result.Error = err

	if err != nil {
		result.Status = "FAIL"
	} else {
		result.Status = "PASS"
	}

	return result
}

func (tr *TestRunner) printTestResult(result TestResult) {
	statusIcon := "✅"
	if result.Status == "FAIL" {
		statusIcon = "❌"
	}

	fmt.Printf("%s %s (%s)\n", statusIcon, result.Suite, result.Duration.Round(time.Millisecond))

	if tr.verbose || result.Status == "FAIL" {
		fmt.Println(strings.Repeat("-", 50))
		fmt.Println(result.Output)
		fmt.Println(strings.Repeat("-", 50))
	}

	if result.Error != nil && result.Status == "FAIL" {
		fmt.Printf("Error: %v\n", result.Error)
	}

	fmt.Println()
}

func (tr *TestRunner) printSummary(results []TestResult, totalDuration time.Duration) {
	fmt.Println("📊 Test Summary")
	fmt.Println("===============")

	passed := 0
	failed := 0
	totalTime := time.Duration(0)

	for _, result := range results {
		if result.Status == "PASS" {
			passed++
		} else {
			failed++
		}
		totalTime += result.Duration
	}

	fmt.Printf("Total Suites: %d\n", len(results))
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Total Time: %s\n", totalDuration.Round(time.Millisecond))
	fmt.Printf("Test Time: %s\n", totalTime.Round(time.Millisecond))

	if failed > 0 {
		fmt.Println("\n❌ Some tests failed!")
		fmt.Println("Failed suites:")
		for _, result := range results {
			if result.Status == "FAIL" {
				fmt.Printf("  - %s\n", result.Suite)
			}
		}
	} else {
		fmt.Println("\n🎉 All tests passed!")
	}
}

func (tr *TestRunner) hasFailures(results []TestResult) bool {
	for _, result := range results {
		if result.Status == "FAIL" {
			return true
		}
	}
	return false
}

func findProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Failed to get working directory: %v", err))
	}

	// Look for go.mod file to identify project root
	current := wd
	for {
		goModPath := filepath.Join(current, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}
		current = parent
	}

	// Fallback to working directory
	return wd
}

func shouldRunVerbose() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" {
			return true
		}
	}
	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
