package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		elements []string
		want     string
	}{
		{
			name:     "basic join",
			base:     "data",
			elements: []string{"events", "test-event"},
			want:     filepath.Join("data", "events", "test-event"),
		},
		{
			name:     "empty elements",
			base:     "data",
			elements: []string{},
			want:     "data",
		},
		{
			name:     "with path separators",
			base:     "data/events",
			elements: []string{"test-event", "1080p"},
			want:     filepath.Join("data", "events", "test-event", "1080p"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeJoin(tt.base, tt.elements...)
			if got != tt.want {
				t.Errorf("SafeJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "utils_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testPath := filepath.Join(tempDir, "test", "nested", "directory")

	// Test creating nested directories
	err = EnsureDir(testPath)
	if err != nil {
		t.Errorf("EnsureDir() failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("Directory was not created: %s", testPath)
	}

	// Test with existing directory (should not fail)
	err = EnsureDir(testPath)
	if err != nil {
		t.Errorf("EnsureDir() failed on existing directory: %v", err)
	}
}

func TestPathExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "utils_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test existing path
	if !PathExists(tempDir) {
		t.Errorf("PathExists() should return true for existing path: %s", tempDir)
	}

	// Test non-existing path
	nonExistentPath := filepath.Join(tempDir, "does-not-exist")
	if PathExists(nonExistentPath) {
		t.Errorf("PathExists() should return false for non-existent path: %s", nonExistentPath)
	}

	// Test with file
	testFile := filepath.Join(tempDir, "test.txt")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.Close()

	if !PathExists(testFile) {
		t.Errorf("PathExists() should return true for existing file: %s", testFile)
	}
}

func TestIsValidPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"empty path", "", false},
		{"valid path", "data/events/test", true},
		{"path with colon", "data:events", false},
		{"path with pipe", "data|events", false},
		{"path with question mark", "data?events", false},
		{"path with asterisk", "data*events", false},
		{"path with quotes", "data\"events", false},
		{"path with angle brackets", "data<events>", false},
		{"normal windows path", "C:\\data\\events", true}, // Windows path separators are actually OK
		{"unix path", "/home/user/data", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidPath(tt.path)
			if got != tt.want {
				t.Errorf("IsValidPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "windows backslashes",
			path: "data\\events\\test",
			want: filepath.Join("data", "events", "test"),
		},
		{
			name: "unix forward slashes",
			path: "data/events/test",
			want: filepath.Join("data", "events", "test"),
		},
		{
			name: "mixed slashes",
			path: "data\\events/test\\file",
			want: filepath.Join("data", "events", "test", "file"),
		},
		{
			name: "redundant separators",
			path: "data//events\\\\test",
			want: filepath.Join("data", "events", "test"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePath(tt.path)
			if got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestGetRelativePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "utils_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	basePath := tempDir
	targetPath := filepath.Join(tempDir, "subdir", "file.txt")

	rel, err := GetRelativePath(basePath, targetPath)
	if err != nil {
		t.Errorf("GetRelativePath() failed: %v", err)
	}

	expected := filepath.Join("subdir", "file.txt")
	if rel != expected {
		t.Errorf("GetRelativePath() = %q, want %q", rel, expected)
	}

	// Test with invalid paths
	_, err = GetRelativePath("", "")
	if err == nil {
		t.Error("GetRelativePath() should fail with empty paths")
	}
}

func TestValidateWritablePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "utils_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test writable path
	writablePath := filepath.Join(tempDir, "test", "file.txt")
	err = ValidateWritablePath(writablePath)
	if err != nil {
		t.Errorf("ValidateWritablePath() failed for writable path: %v", err)
	}

	// Verify directory was created
	dir := filepath.Dir(writablePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Directory should have been created: %s", dir)
	}

	// Test with read-only directory (if supported by OS)
	if runtime.GOOS != "windows" { // Skip on Windows as it's more complex
		readOnlyDir := filepath.Join(tempDir, "readonly")
		os.MkdirAll(readOnlyDir, 0755)
		os.Chmod(readOnlyDir, 0444)       // Read-only
		defer os.Chmod(readOnlyDir, 0755) // Restore permissions for cleanup

		readOnlyPath := filepath.Join(readOnlyDir, "file.txt")
		err = ValidateWritablePath(readOnlyPath)
		if err == nil {
			t.Error("ValidateWritablePath() should fail for read-only directory")
		}
		if !strings.Contains(err.Error(), "not writable") {
			t.Errorf("Expected 'not writable' error, got: %v", err)
		}
	}
}
