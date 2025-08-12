package media

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestWriter_NewManifestWriter(t *testing.T) {
	// Set up temporary environment for testing
	tempDir, err := os.MkdirTemp("", "manifest_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set environment variable to use temp directory
	os.Setenv("LOCAL_OUTPUT_DIR", tempDir)
	defer os.Unsetenv("LOCAL_OUTPUT_DIR")

	eventName := "test-event"
	writer := NewManifestWriter(eventName)

	if writer == nil {
		t.Fatal("NewManifestWriter() returned nil")
	}

	if writer.Segments == nil {
		t.Error("Segments should be initialized")
	}
	if writer.Index == nil {
		t.Error("Index should be initialized")
	}
	if len(writer.Segments) != 0 {
		t.Errorf("Segments should be empty, got %d items", len(writer.Segments))
	}
}

func TestManifestWriter_AddOrUpdateSegment(t *testing.T) {
	writer := &ManifestWriter{
		ManifestPath: "test.json",
		Segments:     make([]ManifestItem, 0),
		Index:        make(map[string]*ManifestItem),
	}

	// Test adding new segment
	writer.AddOrUpdateSegment("1001", "1080p")

	if len(writer.Segments) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(writer.Segments))
	}
	if writer.Segments[0].SeqNo != "1001" {
		t.Errorf("Expected SeqNo '1001', got '%s'", writer.Segments[0].SeqNo)
	}
	if writer.Segments[0].Resolution != "1080p" {
		t.Errorf("Expected Resolution '1080p', got '%s'", writer.Segments[0].Resolution)
	}

	// Test updating existing segment with higher resolution
	writer.AddOrUpdateSegment("1001", "1440p")

	if len(writer.Segments) != 1 {
		t.Errorf("Segments count should remain 1 after update, got %d", len(writer.Segments))
	}
	if writer.Segments[0].Resolution != "1440p" {
		t.Errorf("Expected updated resolution '1440p', got '%s'", writer.Segments[0].Resolution)
	}

	// Test updating existing segment with lower resolution (should not change)
	writer.AddOrUpdateSegment("1001", "720p")

	if writer.Segments[0].Resolution != "1440p" {
		t.Errorf("Resolution should remain '1440p', got '%s'", writer.Segments[0].Resolution)
	}

	// Test adding different segment
	writer.AddOrUpdateSegment("1002", "720p")

	if len(writer.Segments) != 2 {
		t.Errorf("Expected 2 segments, got %d", len(writer.Segments))
	}
}

func TestManifestWriter_AddOrUpdateSegment_NilFields(t *testing.T) {
	writer := &ManifestWriter{
		ManifestPath: "test.json",
	}

	// Test with nil fields (should initialize them)
	writer.AddOrUpdateSegment("1001", "1080p")

	if writer.Segments == nil {
		t.Error("Segments should be initialized")
	}
	if writer.Index == nil {
		t.Error("Index should be initialized")
	}
	if len(writer.Segments) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(writer.Segments))
	}
}

func TestManifestWriter_WriteManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "manifest_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manifestPath := filepath.Join(tempDir, "test-manifest.json")
	writer := &ManifestWriter{
		ManifestPath: manifestPath,
		Segments:     make([]ManifestItem, 0),
		Index:        make(map[string]*ManifestItem),
	}

	// Add some test segments out of order
	writer.AddOrUpdateSegment("1003", "1080p")
	writer.AddOrUpdateSegment("1001", "720p")
	writer.AddOrUpdateSegment("1002", "1080p")

	// Write manifest
	writer.WriteManifest()

	// Verify file was created
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("Manifest file was not created: %s", manifestPath)
	}

	// Read and verify content
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest file: %v", err)
	}

	var segments []ManifestItem
	err = json.Unmarshal(content, &segments)
	if err != nil {
		t.Fatalf("Failed to unmarshal manifest JSON: %v", err)
	}

	// Verify segments are sorted by sequence number
	if len(segments) != 3 {
		t.Errorf("Expected 3 segments in manifest, got %d", len(segments))
	}

	expectedOrder := []string{"1001", "1002", "1003"}
	for i, segment := range segments {
		if segment.SeqNo != expectedOrder[i] {
			t.Errorf("Segment %d: expected SeqNo '%s', got '%s'", i, expectedOrder[i], segment.SeqNo)
		}
	}

	// Verify content structure
	if segments[0].Resolution != "720p" {
		t.Errorf("Expected first segment resolution '720p', got '%s'", segments[0].Resolution)
	}
	if segments[1].Resolution != "1080p" {
		t.Errorf("Expected second segment resolution '1080p', got '%s'", segments[1].Resolution)
	}
}

func TestManifestWriter_WriteManifest_EmptySegments(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "manifest_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manifestPath := filepath.Join(tempDir, "empty-manifest.json")
	writer := &ManifestWriter{
		ManifestPath: manifestPath,
		Segments:     make([]ManifestItem, 0),
		Index:        make(map[string]*ManifestItem),
	}

	// Write empty manifest
	writer.WriteManifest()

	// Verify file was created
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("Manifest file was not created: %s", manifestPath)
	}

	// Read and verify content is empty array
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest file: %v", err)
	}

	var segments []ManifestItem
	err = json.Unmarshal(content, &segments)
	if err != nil {
		t.Fatalf("Failed to unmarshal manifest JSON: %v", err)
	}

	if len(segments) != 0 {
		t.Errorf("Expected empty segments array, got %d items", len(segments))
	}
}

func TestManifestWriter_WriteManifest_InvalidPath(t *testing.T) {
	writer := &ManifestWriter{
		ManifestPath: "/invalid/path/that/does/not/exist/manifest.json",
		Segments:     []ManifestItem{{SeqNo: "1001", Resolution: "1080p"}},
		Index:        make(map[string]*ManifestItem),
	}

	// This should not panic, just fail gracefully
	writer.WriteManifest()

	// Test passes if no panic occurs
}

func TestManifestItem_JSONSerialization(t *testing.T) {
	item := ManifestItem{
		SeqNo:      "1001",
		Resolution: "1080p",
	}

	// Test marshaling
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal ManifestItem: %v", err)
	}

	// Test unmarshaling
	var unmarshaled ManifestItem
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ManifestItem: %v", err)
	}

	if unmarshaled.SeqNo != item.SeqNo {
		t.Errorf("SeqNo mismatch: expected '%s', got '%s'", item.SeqNo, unmarshaled.SeqNo)
	}
	if unmarshaled.Resolution != item.Resolution {
		t.Errorf("Resolution mismatch: expected '%s', got '%s'", item.Resolution, unmarshaled.Resolution)
	}
}
