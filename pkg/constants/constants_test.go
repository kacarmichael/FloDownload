package constants

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestGetConfig(t *testing.T) {
	// Test successful config loading
	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("GetConfig() returned nil config")
	}

	// Test that subsequent calls return the same instance (singleton)
	cfg2, err := GetConfig()
	if err != nil {
		t.Fatalf("Second GetConfig() call failed: %v", err)
	}

	// Both should be the same instance due to sync.Once
	if cfg != cfg2 {
		t.Error("GetConfig() should return the same instance (singleton)")
	}
}

func TestMustGetConfig(t *testing.T) {
	// This should not panic with valid environment
	cfg := MustGetConfig()
	if cfg == nil {
		t.Fatal("MustGetConfig() returned nil")
	}

	// Verify it returns a properly initialized config
	if cfg.Core.WorkerCount <= 0 {
		t.Errorf("Expected positive WorkerCount, got %d", cfg.Core.WorkerCount)
	}
	if cfg.Core.RefreshDelay <= 0 {
		t.Errorf("Expected positive RefreshDelay, got %v", cfg.Core.RefreshDelay)
	}
}

func TestMustGetConfig_Panic(t *testing.T) {
	// We can't easily test the panic scenario without breaking the singleton,
	// but we can test that MustGetConfig works normally
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustGetConfig() panicked unexpectedly: %v", r)
		}
	}()

	cfg := MustGetConfig()
	if cfg == nil {
		t.Fatal("MustGetConfig() returned nil without panicking")
	}
}

func TestConfigSingleton(t *testing.T) {
	// Reset the singleton for this test (this is a bit hacky but necessary for testing)
	// We'll create multiple goroutines to test concurrent access

	configs := make(chan interface{}, 10)

	// Launch multiple goroutines to call GetConfig concurrently
	for i := 0; i < 10; i++ {
		go func() {
			cfg, _ := GetConfig()
			configs <- cfg
		}()
	}

	// Collect all configs
	var allConfigs []interface{}
	for i := 0; i < 10; i++ {
		allConfigs = append(allConfigs, <-configs)
	}

	// All should be the same instance
	firstConfig := allConfigs[0]
	for i, cfg := range allConfigs {
		if cfg != firstConfig {
			t.Errorf("Config %d is different from first config", i)
		}
	}
}

func TestConstants_Values(t *testing.T) {
	// Test that constants have expected values
	if WorkerCount != 4 {
		t.Errorf("Expected WorkerCount=4, got %d", WorkerCount)
	}
	if RefreshDelay != 3 {
		t.Errorf("Expected RefreshDelay=3, got %d", RefreshDelay)
	}

	// Test HTTP constants
	if HTTPUserAgent == "" {
		t.Error("HTTPUserAgent should not be empty")
	}
	if !strings.Contains(HTTPUserAgent, "Mozilla") {
		t.Error("HTTPUserAgent should contain 'Mozilla'")
	}
	if REFERRER != "https://www.flomarching.com" {
		t.Errorf("Expected REFERRER='https://www.flomarching.com', got '%s'", REFERRER)
	}

	// Test default NAS constants
	if DefaultNASOutputPath != "" {
		t.Errorf("Expected DefaultNASOutputPath='', got '%s'", DefaultNASOutputPath)
	}
	if DefaultNASUsername != "" {
		t.Errorf("Expected DefaultNASUsername='', got '%s'", DefaultNASUsername)
	}

	// Test transfer constants
	if DefaultTransferWorkerCount != 2 {
		t.Errorf("Expected DefaultTransferWorkerCount=2, got %d", DefaultTransferWorkerCount)
	}
	if DefaultTransferRetryLimit != 3 {
		t.Errorf("Expected DefaultTransferRetryLimit=3, got %d", DefaultTransferRetryLimit)
	}
	if DefaultTransferTimeout != 30 {
		t.Errorf("Expected DefaultTransferTimeout=30, got %d", DefaultTransferTimeout)
	}
	if DefaultFileSettlingDelay != 5 {
		t.Errorf("Expected DefaultFileSettlingDelay=5, got %d", DefaultFileSettlingDelay)
	}
	if DefaultTransferQueueSize != 100000 {
		t.Errorf("Expected DefaultTransferQueueSize=100000, got %d", DefaultTransferQueueSize)
	}
	if DefaultBatchSize != 1000 {
		t.Errorf("Expected DefaultBatchSize=1000, got %d", DefaultBatchSize)
	}

	// Test cleanup constants
	if DefaultCleanupBatchSize != 1000 {
		t.Errorf("Expected DefaultCleanupBatchSize=1000, got %d", DefaultCleanupBatchSize)
	}
	if DefaultRetainLocalHours != 0 {
		t.Errorf("Expected DefaultRetainLocalHours=0, got %d", DefaultRetainLocalHours)
	}

	// Test processing constants
	if DefaultProcessWorkerCount != 2 {
		t.Errorf("Expected DefaultProcessWorkerCount=2, got %d", DefaultProcessWorkerCount)
	}
	if DefaultFFmpegPath != "ffmpeg" {
		t.Errorf("Expected DefaultFFmpegPath='ffmpeg', got '%s'", DefaultFFmpegPath)
	}
}

func TestConfig_Integration(t *testing.T) {
	cfg := MustGetConfig()

	// Test that config values match or override constants appropriately
	if cfg.Core.WorkerCount != WorkerCount && os.Getenv("WORKER_COUNT") == "" {
		t.Errorf("Config WorkerCount (%d) should match constant (%d) when no env override", cfg.Core.WorkerCount, WorkerCount)
	}

	if cfg.Core.RefreshDelay != time.Duration(RefreshDelay)*time.Second && os.Getenv("REFRESH_DELAY_SECONDS") == "" {
		t.Errorf("Config RefreshDelay (%v) should match constant (%v) when no env override", cfg.Core.RefreshDelay, time.Duration(RefreshDelay)*time.Second)
	}

	// Test HTTP settings
	if cfg.HTTP.UserAgent != HTTPUserAgent {
		t.Errorf("Config UserAgent (%s) should match constant (%s)", cfg.HTTP.UserAgent, HTTPUserAgent)
	}
	if cfg.HTTP.Referer != REFERRER {
		t.Errorf("Config Referer (%s) should match constant (%s)", cfg.HTTP.Referer, REFERRER)
	}
}

func TestConfig_PathMethods(t *testing.T) {
	cfg := MustGetConfig()

	testEvent := "test-event-123"
	testQuality := "1080p"

	// Test GetEventPath
	eventPath := cfg.GetEventPath(testEvent)
	if !strings.Contains(eventPath, testEvent) {
		t.Errorf("GetEventPath should contain event name '%s', got: %s", testEvent, eventPath)
	}

	// Test GetManifestPath
	manifestPath := cfg.GetManifestPath(testEvent)
	if !strings.Contains(manifestPath, testEvent) {
		t.Errorf("GetManifestPath should contain event name '%s', got: %s", testEvent, manifestPath)
	}
	if !strings.HasSuffix(manifestPath, ".json") {
		t.Errorf("GetManifestPath should end with '.json', got: %s", manifestPath)
	}

	// Test GetNASEventPath
	nasPath := cfg.GetNASEventPath(testEvent)
	if !strings.Contains(nasPath, testEvent) {
		t.Errorf("GetNASEventPath should contain event name '%s', got: %s", testEvent, nasPath)
	}

	// Test GetProcessOutputPath
	processPath := cfg.GetProcessOutputPath(testEvent)
	if !strings.Contains(processPath, testEvent) {
		t.Errorf("GetProcessOutputPath should contain event name '%s', got: %s", testEvent, processPath)
	}

	// Test GetQualityPath
	qualityPath := cfg.GetQualityPath(testEvent, testQuality)
	if !strings.Contains(qualityPath, testEvent) {
		t.Errorf("GetQualityPath should contain event name '%s', got: %s", testEvent, qualityPath)
	}
	if !strings.Contains(qualityPath, testQuality) {
		t.Errorf("GetQualityPath should contain quality '%s', got: %s", testQuality, qualityPath)
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	cfg := MustGetConfig()

	// Test that default values are reasonable
	if cfg.Transfer.QueueSize != DefaultTransferQueueSize {
		t.Errorf("Expected transfer queue size %d, got %d", DefaultTransferQueueSize, cfg.Transfer.QueueSize)
	}

	if cfg.Transfer.BatchSize != DefaultBatchSize {
		t.Errorf("Expected transfer batch size %d, got %d", DefaultBatchSize, cfg.Transfer.BatchSize)
	}

	if cfg.Processing.WorkerCount != DefaultProcessWorkerCount {
		t.Errorf("Expected processing worker count %d, got %d", DefaultProcessWorkerCount, cfg.Processing.WorkerCount)
	}

	if cfg.Cleanup.BatchSize != DefaultCleanupBatchSize {
		t.Errorf("Expected cleanup batch size %d, got %d", DefaultCleanupBatchSize, cfg.Cleanup.BatchSize)
	}
}
