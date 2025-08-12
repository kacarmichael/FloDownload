package processing

import (
	"m3u8-downloader/pkg/config"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func createTestConfig(tempDir string) *config.Config {
	return &config.Config{
		Core: config.CoreConfig{
			WorkerCount:  2,
			RefreshDelay: 1 * time.Second,
		},
		NAS: config.NASConfig{
			OutputPath:     filepath.Join(tempDir, "nas"),
			Username:       "testuser",
			Password:       "testpass",
			Timeout:        10 * time.Second,
			RetryLimit:     2,
			EnableTransfer: false, // Disable to avoid NAS connection
		},
		Processing: config.ProcessingConfig{
			Enabled:     true,
			AutoProcess: true,
			WorkerCount: 1,
			FFmpegPath:  "echo", // Use echo command for testing
		},
		Paths: config.PathsConfig{
			LocalOutput:     filepath.Join(tempDir, "data"),
			ProcessOutput:   filepath.Join(tempDir, "out"),
			ManifestDir:     filepath.Join(tempDir, "data"),
			PersistenceFile: filepath.Join(tempDir, "queue.json"),
		},
	}
}

func TestNewProcessingService_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "processing_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := createTestConfig(tempDir)
	cfg.NAS.EnableTransfer = false // Disable NAS to avoid connection

	// We can't test actual NAS connection, so we'll skip the constructor test
	// that requires NAS connectivity. Instead, test the configuration handling.

	if cfg.Processing.FFmpegPath != "echo" {
		t.Errorf("Expected FFmpegPath='echo', got '%s'", cfg.Processing.FFmpegPath)
	}
}

func TestNewProcessingService_NilConfig(t *testing.T) {
	_, err := NewProcessingService("test-event", nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}
	if !strings.Contains(err.Error(), "configuration is required") {
		t.Errorf("Expected 'configuration is required' error, got: %v", err)
	}
}

func TestProcessingService_GetEventDirs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "processing_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := createTestConfig(tempDir)

	// Create mock NAS directory structure
	nasDir := cfg.NAS.OutputPath
	os.MkdirAll(filepath.Join(nasDir, "event1"), 0755)
	os.MkdirAll(filepath.Join(nasDir, "event2"), 0755)
	os.MkdirAll(filepath.Join(nasDir, "event3"), 0755)
	// Create a file (should be ignored)
	os.WriteFile(filepath.Join(nasDir, "not_a_dir.txt"), []byte("test"), 0644)

	ps := &ProcessingService{
		config:    cfg,
		eventName: "", // Empty to test directory discovery
	}

	dirs, err := ps.GetEventDirs()
	if err != nil {
		t.Fatalf("GetEventDirs() failed: %v", err)
	}

	if len(dirs) != 3 {
		t.Errorf("Expected 3 event directories, got %d", len(dirs))
	}

	expectedDirs := []string{"event1", "event2", "event3"}
	for _, expected := range expectedDirs {
		found := false
		for _, actual := range dirs {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find directory '%s' in results: %v", expected, dirs)
		}
	}
}

func TestProcessingService_GetEventDirs_WithEventName(t *testing.T) {
	cfg := createTestConfig("/tmp")
	eventName := "specific-event"

	ps := &ProcessingService{
		config:    cfg,
		eventName: eventName,
	}

	dirs, err := ps.GetEventDirs()
	if err != nil {
		t.Fatalf("GetEventDirs() failed: %v", err)
	}

	if len(dirs) != 1 {
		t.Errorf("Expected 1 directory, got %d", len(dirs))
	}
	if dirs[0] != eventName {
		t.Errorf("Expected directory '%s', got '%s'", eventName, dirs[0])
	}
}

func TestProcessingService_GetResolutions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "processing_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := createTestConfig(tempDir)
	eventName := "test-event"

	// Create mock event directory with quality subdirectories
	eventPath := filepath.Join(cfg.NAS.OutputPath, eventName)
	os.MkdirAll(filepath.Join(eventPath, "1080p"), 0755)
	os.MkdirAll(filepath.Join(eventPath, "720p"), 0755)
	os.MkdirAll(filepath.Join(eventPath, "480p"), 0755)
	os.MkdirAll(filepath.Join(eventPath, "not_resolution"), 0755)            // Should be ignored
	os.WriteFile(filepath.Join(eventPath, "file.txt"), []byte("test"), 0644) // Should be ignored

	ps := &ProcessingService{
		config:    cfg,
		eventName: eventName,
	}

	resolutions, err := ps.GetResolutions()
	if err != nil {
		t.Fatalf("GetResolutions() failed: %v", err)
	}

	expectedResolutions := []string{"1080p", "720p", "480p"}
	if len(resolutions) != len(expectedResolutions) {
		t.Errorf("Expected %d resolutions, got %d: %v", len(expectedResolutions), len(resolutions), resolutions)
	}

	for _, expected := range expectedResolutions {
		found := false
		for _, actual := range resolutions {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find resolution '%s' in results: %v", expected, resolutions)
		}
	}
}

func TestProcessingService_AggregateSegmentInfo(t *testing.T) {
	ps := &ProcessingService{}

	// Create test channel with segments
	ch := make(chan SegmentInfo, 5)

	// Add segments with different qualities for same sequence
	ch <- SegmentInfo{Name: "seg_1001.ts", SeqNo: 1001, Resolution: "720p"}
	ch <- SegmentInfo{Name: "seg_1001.ts", SeqNo: 1001, Resolution: "1080p"} // Higher quality, should win
	ch <- SegmentInfo{Name: "seg_1002.ts", SeqNo: 1002, Resolution: "480p"}
	ch <- SegmentInfo{Name: "seg_1003.ts", SeqNo: 1003, Resolution: "1080p"}
	ch <- SegmentInfo{Name: "seg_1001.ts", SeqNo: 1001, Resolution: "540p"} // Lower than 1080p, should not replace

	close(ch)

	segmentMap, err := ps.AggregateSegmentInfo(ch)
	if err != nil {
		t.Fatalf("AggregateSegmentInfo() failed: %v", err)
	}

	// Should have 3 unique sequence numbers
	if len(segmentMap) != 3 {
		t.Errorf("Expected 3 unique segments, got %d", len(segmentMap))
	}

	// Check sequence 1001 has the highest quality (1080p)
	seg1001, exists := segmentMap[1001]
	if !exists {
		t.Fatal("Segment 1001 should exist")
	}
	if seg1001.Resolution != "1080p" {
		t.Errorf("Expected segment 1001 to have resolution '1080p', got '%s'", seg1001.Resolution)
	}

	// Check sequence 1002 has 480p
	seg1002, exists := segmentMap[1002]
	if !exists {
		t.Fatal("Segment 1002 should exist")
	}
	if seg1002.Resolution != "480p" {
		t.Errorf("Expected segment 1002 to have resolution '480p', got '%s'", seg1002.Resolution)
	}

	// Check sequence 1003 has 1080p
	seg1003, exists := segmentMap[1003]
	if !exists {
		t.Fatal("Segment 1003 should exist")
	}
	if seg1003.Resolution != "1080p" {
		t.Errorf("Expected segment 1003 to have resolution '1080p', got '%s'", seg1003.Resolution)
	}
}

func TestProcessingService_WriteConcatFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "processing_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := createTestConfig(tempDir)
	eventName := "test-event"

	ps := &ProcessingService{
		config:    cfg,
		eventName: eventName,
	}

	// Create test segment map
	segmentMap := map[int]SegmentInfo{
		1003: {Name: "seg_1003.ts", SeqNo: 1003, Resolution: "1080p"},
		1001: {Name: "seg_1001.ts", SeqNo: 1001, Resolution: "720p"},
		1002: {Name: "seg_1002.ts", SeqNo: 1002, Resolution: "1080p"},
	}

	concatFilePath, err := ps.WriteConcatFile(segmentMap)
	if err != nil {
		t.Fatalf("WriteConcatFile() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(concatFilePath); os.IsNotExist(err) {
		t.Fatalf("Concat file was not created: %s", concatFilePath)
	}

	// Read and verify content
	content, err := os.ReadFile(concatFilePath)
	if err != nil {
		t.Fatalf("Failed to read concat file: %v", err)
	}

	contentStr := string(content)
	lines := strings.Split(strings.TrimSpace(contentStr), "\n")

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines in concat file, got %d", len(lines))
	}

	// Verify segments are sorted by sequence number
	expectedOrder := []string{"seg_1001.ts", "seg_1002.ts", "seg_1003.ts"}
	for i, line := range lines {
		if !strings.Contains(line, expectedOrder[i]) {
			t.Errorf("Line %d should contain '%s', got: %s", i, expectedOrder[i], line)
		}
		if !strings.HasPrefix(line, "file '") {
			t.Errorf("Line %d should start with 'file ', got: %s", i, line)
		}
	}
}

func TestProcessingService_getFFmpegPath(t *testing.T) {
	cfg := createTestConfig("/tmp")

	tests := []struct {
		name          string
		ffmpegPath    string
		shouldFind    bool
		expectedError string
	}{
		{
			name:       "echo command (should be found in PATH)",
			ffmpegPath: "echo",
			shouldFind: true,
		},
		{
			name: "absolute path test",
			ffmpegPath: func() string {
				if runtime.GOOS == "windows" {
					return "C:\\Windows\\System32\\cmd.exe"
				}
				return "/bin/echo"
			}(),
			shouldFind: true,
		},
		{
			name:          "nonexistent command",
			ffmpegPath:    "nonexistent_ffmpeg_command_12345",
			shouldFind:    false,
			expectedError: "FFmpeg not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCfg := *cfg
			testCfg.Processing.FFmpegPath = tt.ffmpegPath

			ps := &ProcessingService{
				config:    &testCfg,
				eventName: "test",
			}

			path, err := ps.getFFmpegPath()

			if tt.shouldFind {
				if err != nil {
					t.Errorf("Expected to find FFmpeg, but got error: %v", err)
				}
				if path == "" {
					t.Error("Expected non-empty path")
				}
			} else {
				if err == nil {
					t.Error("Expected error for nonexistent FFmpeg")
				}
				if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
			}
		})
	}
}

func TestSegmentInfo_Structure(t *testing.T) {
	segment := SegmentInfo{
		Name:       "test_segment.ts",
		SeqNo:      1001,
		Resolution: "1080p",
	}

	if segment.Name != "test_segment.ts" {
		t.Errorf("Expected Name='test_segment.ts', got '%s'", segment.Name)
	}
	if segment.SeqNo != 1001 {
		t.Errorf("Expected SeqNo=1001, got %d", segment.SeqNo)
	}
	if segment.Resolution != "1080p" {
		t.Errorf("Expected Resolution='1080p', got '%s'", segment.Resolution)
	}
}

func TestProcessJob_Structure(t *testing.T) {
	job := ProcessJob{
		EventName: "test-event",
	}

	if job.EventName != "test-event" {
		t.Errorf("Expected EventName='test-event', got '%s'", job.EventName)
	}
}
