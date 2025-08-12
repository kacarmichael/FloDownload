package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SafeJoin(base string, elements ...string) string {
	path := filepath.Join(append([]string{base}, elements...)...)
	return filepath.Clean(path)
}

func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func IsValidPath(path string) bool {
	if path == "" {
		return false
	}

	return !strings.ContainsAny(path, "<>:\"|?*")
}

func NormalizePath(path string) string {
	return filepath.Clean(strings.ReplaceAll(path, "\\", string(filepath.Separator)))
}

func GetRelativePath(basePath, targetPath string) (string, error) {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}
	return rel, nil
}

func ValidateWritablePath(path string) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}

	testFile := filepath.Join(dir, ".write_test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("path %s is not writable: %w", dir, err)
	}
	file.Close()
	os.Remove(testFile)

	return nil
}
