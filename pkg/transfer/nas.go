package transfer

import (
	"context"
	"fmt"
	"log"
	"m3u8-downloader/pkg/nas"
	"os"
	"path/filepath"
)

func TransferFile(nt *nas.NASService, ctx context.Context, item *TransferItem) error {
	destPath := filepath.Join(nt.Config.Path, item.DestinationPath)

	destDir := filepath.Dir(destPath)
	if err := nt.EnsureDirectoryExists(destDir); err != nil {
		return fmt.Errorf("Failed to create directory %s: %w", destDir, err)
	}

	transferCtx, cancel := context.WithTimeout(ctx, nt.Config.Timeout)
	defer cancel()

	if err := nt.CopyFile(transferCtx, item.SourcePath, destPath); err != nil {
		return fmt.Errorf("Failed to copy file %s to %s: %w", item.SourcePath, destPath, err)
	}

	if nt.Config.VerifySize {
		if err := nt.VerifyTransfer(item.SourcePath, destPath); err != nil {
			os.Remove(destPath)
			return fmt.Errorf("Failed to verify transfer: %w", err)
		}
	}

	log.Printf("File transfer completed: %s -> %s", item.SourcePath, destPath)

	return nil
}
