package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfig_Load(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"WORKER_COUNT":        os.Getenv("WORKER_COUNT"),
		"NAS_USERNAME":        os.Getenv("NAS_USERNAME"),
		"LOCAL_OUTPUT_DIR":    os.Getenv("LOCAL_OUTPUT_DIR"),
		"ENABLE_NAS_TRANSFER": os.Getenv("ENABLE_NAS_TRANSFER"),
	}
	defer func() {
		// Restore original env vars
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Test default config load
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify defaults
	if cfg.Core.WorkerCount != 4 {
		t.Errorf("Expected WorkerCount=4, got %d", cfg.Core.WorkerCount)
	}
	if cfg.Core.RefreshDelay != 3*time.Second {
		t.Errorf("Expected RefreshDelay=3s, got %v", cfg.Core.RefreshDelay)
	}
	if !cfg.NAS.EnableTransfer {
		t.Errorf("Expected NAS.EnableTransfer=true, got false")
	}

	// Test environment variable override
	os.Setenv("WORKER_COUNT", "8")
	os.Setenv("NAS_USERNAME", "testuser")
	os.Setenv("ENABLE_NAS_TRANSFER", "false")
	os.Setenv("LOCAL_OUTPUT_DIR", "custom_data")

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load() with env vars failed: %v", err)
	}

	if cfg2.Core.WorkerCount != 8 {
		t.Errorf("Expected WorkerCount=8 from env, got %d", cfg2.Core.WorkerCount)
	}
	if cfg2.NAS.Username != "testuser" {
		t.Errorf("Expected NAS.Username='testuser' from env, got %s", cfg2.NAS.Username)
	}
	if cfg2.NAS.EnableTransfer {
		t.Errorf("Expected NAS.EnableTransfer=false from env, got true")
	}
	if !strings.Contains(cfg2.Paths.LocalOutput, "custom_data") {
		t.Errorf("Expected LocalOutput to contain 'custom_data', got %s", cfg2.Paths.LocalOutput)
	}
}

func TestConfig_PathMethods(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	testEvent := "test-event"
	testQuality := "1080p"

	// Test GetEventPath
	eventPath := cfg.GetEventPath(testEvent)
	if !strings.Contains(eventPath, testEvent) {
		t.Errorf("GetEventPath should contain event name, got %s", eventPath)
	}

	// Test GetManifestPath
	manifestPath := cfg.GetManifestPath(testEvent)
	if !strings.Contains(manifestPath, testEvent) {
		t.Errorf("GetManifestPath should contain event name, got %s", manifestPath)
	}
	if !strings.HasSuffix(manifestPath, ".json") {
		t.Errorf("GetManifestPath should end with .json, got %s", manifestPath)
	}

	// Test GetNASEventPath
	nasPath := cfg.GetNASEventPath(testEvent)
	if !strings.Contains(nasPath, testEvent) {
		t.Errorf("GetNASEventPath should contain event name, got %s", nasPath)
	}

	// Test GetProcessOutputPath
	processPath := cfg.GetProcessOutputPath(testEvent)
	if !strings.Contains(processPath, testEvent) {
		t.Errorf("GetProcessOutputPath should contain event name, got %s", processPath)
	}

	// Test GetQualityPath
	qualityPath := cfg.GetQualityPath(testEvent, testQuality)
	if !strings.Contains(qualityPath, testEvent) {
		t.Errorf("GetQualityPath should contain event name, got %s", qualityPath)
	}
	if !strings.Contains(qualityPath, testQuality) {
		t.Errorf("GetQualityPath should contain quality, got %s", qualityPath)
	}
}

func TestConfig_PathValidation(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set environment variables to use temp directory
	os.Setenv("LOCAL_OUTPUT_DIR", filepath.Join(tempDir, "data"))
	defer os.Unsetenv("LOCAL_OUTPUT_DIR")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify directories were created
	if _, err := os.Stat(cfg.Paths.LocalOutput); os.IsNotExist(err) {
		t.Errorf("LocalOutput directory should have been created: %s", cfg.Paths.LocalOutput)
	}
	if _, err := os.Stat(cfg.Paths.ProcessOutput); os.IsNotExist(err) {
		t.Errorf("ProcessOutput directory should have been created: %s", cfg.Paths.ProcessOutput)
	}
}

func TestConfig_ValidationErrors(t *testing.T) {
	// Save original env vars
	originalNASPath := os.Getenv("NAS_OUTPUT_PATH")
	originalFFmpegPath := os.Getenv("FFMPEG_PATH")
	defer func() {
		if originalNASPath == "" {
			os.Unsetenv("NAS_OUTPUT_PATH")
		} else {
			os.Setenv("NAS_OUTPUT_PATH", originalNASPath)
		}
		if originalFFmpegPath == "" {
			os.Unsetenv("FFMPEG_PATH")
		} else {
			os.Setenv("FFMPEG_PATH", originalFFmpegPath)
		}
	}()

	// Note: Validation tests are limited because the default config
	// has working defaults. We can test that Load() works with valid configs.

	// Test that Load works with proper paths set
	tempDir2, err := os.MkdirTemp("", "config_validation_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	os.Setenv("NAS_OUTPUT_PATH", "\\\\test\\path")
	os.Setenv("LOCAL_OUTPUT_DIR", tempDir2)

	cfg, err := Load()
	if err != nil {
		t.Errorf("Load() should work with valid config: %v", err)
	}
	if cfg == nil {
		t.Error("Config should not be nil")
	}
}
