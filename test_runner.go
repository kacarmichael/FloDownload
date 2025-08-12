package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	fmt.Println("🧪 StreamRecorder Test Suite")
	fmt.Println("============================")

	startTime := time.Now()

	// Set test environment to avoid interfering with real data
	originalEnv := setupTestEnvironment()
	defer restoreEnvironment(originalEnv)

	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "streamrecorder_test_*")
	if err != nil {
		fmt.Printf("❌ Failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		fmt.Printf("🧹 Cleaning up test directory: %s\n", tempDir)
		os.RemoveAll(tempDir)
	}()

	// Set test-specific environment variables
	os.Setenv("LOCAL_OUTPUT_DIR", filepath.Join(tempDir, "data"))
	os.Setenv("PROCESS_OUTPUT_DIR", filepath.Join(tempDir, "out"))
	os.Setenv("ENABLE_NAS_TRANSFER", "false") // Disable NAS for tests
	os.Setenv("PROCESSING_ENABLED", "false")  // Disable processing that needs FFmpeg

	fmt.Printf("📁 Using temporary directory: %s\n", tempDir)
	fmt.Println()

	// Run tests for each package
	packages := []string{
		"./pkg/config",
		"./pkg/utils",
		"./pkg/constants",
		"./pkg/httpClient",
		"./pkg/media",
		"./pkg/processing",
	}

	var failedPackages []string
	totalTests := 0
	passedTests := 0

	for _, pkg := range packages {
		fmt.Printf("🔍 Testing package: %s\n", pkg)

		cmd := exec.Command("go", "test", "-v", pkg)
		cmd.Dir = "."

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		// Count tests
		testCount := strings.Count(outputStr, "=== RUN")
		passCount := strings.Count(outputStr, "--- PASS:")

		totalTests += testCount
		passedTests += passCount

		if err != nil {
			fmt.Printf("❌ FAILED: %s (%d/%d tests passed)\n", pkg, passCount, testCount)
			failedPackages = append(failedPackages, pkg)

			// Show failure details
			lines := strings.Split(outputStr, "\n")
			for _, line := range lines {
				if strings.Contains(line, "FAIL:") ||
					strings.Contains(line, "Error:") ||
					strings.Contains(line, "panic:") {
					fmt.Printf("   %s\n", line)
				}
			}
		} else {
			fmt.Printf("✅ PASSED: %s (%d tests)\n", pkg, testCount)
		}
		fmt.Println()
	}

	// Print summary
	duration := time.Since(startTime)
	fmt.Println("📊 Test Summary")
	fmt.Println("===============")
	fmt.Printf("Total packages: %d\n", len(packages))
	fmt.Printf("Passed packages: %d\n", len(packages)-len(failedPackages))
	fmt.Printf("Failed packages: %d\n", len(failedPackages))
	fmt.Printf("Total tests: %d\n", totalTests)
	fmt.Printf("Passed tests: %d\n", passedTests)
	fmt.Printf("Failed tests: %d\n", totalTests-passedTests)
	fmt.Printf("Duration: %v\n", duration.Round(time.Millisecond))

	if len(failedPackages) > 0 {
		fmt.Println()
		fmt.Println("❌ Failed packages:")
		for _, pkg := range failedPackages {
			fmt.Printf("   - %s\n", pkg)
		}
		os.Exit(1)
	} else {
		fmt.Println()
		fmt.Println("🎉 All tests passed!")
	}
}

func setupTestEnvironment() map[string]string {
	// Save original environment variables that we'll modify
	originalEnv := make(map[string]string)

	envVars := []string{
		"LOCAL_OUTPUT_DIR",
		"PROCESS_OUTPUT_DIR",
		"ENABLE_NAS_TRANSFER",
		"PROCESSING_ENABLED",
		"NAS_OUTPUT_PATH",
		"FFMPEG_PATH",
		"WORKER_COUNT",
	}

	for _, envVar := range envVars {
		originalEnv[envVar] = os.Getenv(envVar)
	}

	return originalEnv
}

func restoreEnvironment(originalEnv map[string]string) {
	fmt.Println("🔄 Restoring original environment...")
	for envVar, originalValue := range originalEnv {
		if originalValue == "" {
			os.Unsetenv(envVar)
		} else {
			os.Setenv(envVar, originalValue)
		}
	}
}
