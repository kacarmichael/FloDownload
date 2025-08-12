package nas

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type NASService struct {
	Config    NASConfig
	connected bool
}

func NewNASService(config NASConfig) *NASService {
	nt := &NASService{
		Config: config,
	}

	// Establish network connection with credentials before accessing the path
	if err := nt.EstablishConnection(); err != nil {
		log.Fatalf("Failed to establish network connection to %s: %v", nt.Config.Path, err)
	}

	err := nt.EnsureDirectoryExists(nt.Config.Path)
	if err != nil {
		log.Fatalf("Failed to create directory %s: %v", nt.Config.Path, err)
	}
	return nt
}

func (nt *NASService) CopyFile(ctx context.Context, srcPath, destPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("Failed to open source file: %w", err)
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("Failed to create destination file: %w", err)
	}
	defer dest.Close()

	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(dest, src)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}

		return dest.Sync()
	}
}

func (nt *NASService) VerifyTransfer(srcPath, destPath string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("Failed to stat source file: %w", err)
	}

	destInfo, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("Failed to stat destination file: %w", err)
	}

	if srcInfo.Size() != destInfo.Size() {
		return fmt.Errorf("size mismatch: source=%d, dest=%d", srcInfo.Size(), destInfo.Size())
	}

	return nil
}

func (nt *NASService) EnsureDirectoryExists(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("Failed to create directory: %w", err)
	}
	return nil
}

func (nt *NASService) EstablishConnection() error {
	networkPath := nt.ExtractNetworkPath(nt.Config.Path)
	if networkPath == "" {
		return nil // local path, no network mount needed
	}

	log.Printf("Establishing network connection to %s with user %s", networkPath, nt.Config.Username)

	var cmd *exec.Cmd
	if nt.Config.Username != "" && nt.Config.Password != "" {
		cmd = exec.Command("net", "use", networkPath, "/user:"+nt.Config.Username, nt.Config.Password, "/persistent:no")
	} else {
		cmd = exec.Command("net", "use", networkPath, "/persistent:no")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to establish network connection: %w\nOutput: %s", err, string(output))
	}

	log.Printf("Network connection established successfully")
	return nil
}

func (nt *NASService) ExtractNetworkPath(fullPath string) string {
	// Extract \\server\share from paths like \\server\share\folder\subfolder
	if !strings.HasPrefix(fullPath, "\\\\") {
		return "" // Not a UNC path
	}

	parts := strings.Split(fullPath[2:], "\\") // Remove leading \\
	if len(parts) < 2 {
		return "" // Invalid UNC path
	}

	return "\\\\" + parts[0] + "\\" + parts[1]
}

func (nt *NASService) TestConnection() error {
	testFile := filepath.Join(nt.Config.Path, ".connection_test")

	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("Failed to create test file: %w", err)
	}
	f.Close()

	os.Remove(testFile)

	nt.connected = true
	log.Printf("Connected to NAS at %s", nt.Config.Path)
	return nil
}

func (nt *NASService) IsConnected() bool {
	return nt.connected
}

// Disconnect removes the network connection
func (nt *NASService) Disconnect() error {
	networkPath := nt.ExtractNetworkPath(nt.Config.Path)
	if networkPath == "" {
		return nil // Local path, nothing to disconnect
	}

	cmd := exec.Command("net", "use", networkPath, "/delete")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: failed to disconnect from %s: %v\nOutput: %s", networkPath, err, string(output))
		// Don't return error since this is cleanup
	} else {
		log.Printf("Disconnected from network path: %s", networkPath)
	}

	nt.connected = false
	return nil
}

// FileExists checks if a file already exists on the NAS and optionally verifies size
func (nt *NASService) FileExists(destinationPath string, expectedSize int64) (bool, error) {
	fullDestPath := filepath.Join(nt.Config.Path, destinationPath)

	destInfo, err := os.Stat(fullDestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // File doesn't exist, no error
		}
		return false, fmt.Errorf("failed to stat NAS file: %w", err)
	}

	// File exists, check size if expected size is provided
	if expectedSize > 0 && destInfo.Size() != expectedSize {
		log.Printf("NAS file size mismatch for %s: expected=%d, actual=%d",
			fullDestPath, expectedSize, destInfo.Size())
		return false, nil // File exists but wrong size, treat as not existing
	}

	return true, nil
}

// GetFileSize returns the size of a file on the NAS
func (nt *NASService) GetFileSize(destinationPath string) (int64, error) {
	fullDestPath := filepath.Join(nt.Config.Path, destinationPath)

	destInfo, err := os.Stat(fullDestPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat NAS file: %w", err)
	}

	return destInfo.Size(), nil
}
