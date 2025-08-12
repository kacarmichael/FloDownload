package transfer

import (
	"context"
	"fmt"
	"log"
	"m3u8-downloader/pkg/constants"
	nas2 "m3u8-downloader/pkg/nas"
	"m3u8-downloader/pkg/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type TransferService struct {
	watcher *FileWatcher
	queue   *TransferQueue
	nas     *nas2.NASService
	cleanup *CleanupService
	stats   *QueueStats
}

func NewTrasferService(outputDir string, eventName string) (*TransferService, error) {
	cfg := constants.MustGetConfig()

	nasConfig := nas2.NASConfig{
		Path:       outputDir,
		Username:   cfg.NAS.Username,
		Password:   cfg.NAS.Password,
		Timeout:    cfg.NAS.Timeout,
		RetryLimit: cfg.NAS.RetryLimit,
		VerifySize: true,
	}
	nas := nas2.NewNASService(nasConfig)

	if err := nas.TestConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to NAS: %w", err)
	}

	cleanupConfig := CleanupConfig{
		Enabled:         cfg.Cleanup.AfterTransfer,
		RetentionPeriod: time.Duration(cfg.Cleanup.RetainHours) * time.Hour,
		BatchSize:       cfg.Cleanup.BatchSize,
		CheckInterval:   cfg.Transfer.FileSettlingDelay,
	}
	cleanup := NewCleanupService(cleanupConfig)

	queueConfig := QueueConfig{
		WorkerCount:     cfg.Transfer.WorkerCount,
		PersistencePath: cfg.Paths.PersistenceFile,
		MaxQueueSize:    cfg.Transfer.QueueSize,
		BatchSize:       cfg.Transfer.BatchSize,
	}
	queue := NewTransferQueue(queueConfig, nas, cleanup)

	// Create local output directory if it doesn't exist
	localOutputPath := cfg.GetEventPath(eventName)
	if err := utils.EnsureDir(localOutputPath); err != nil {
		return nil, fmt.Errorf("failed to create local output directory: %w", err)
	}

	watcher, err := NewFileWatcher(localOutputPath, queue, cfg.Transfer.FileSettlingDelay)
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &TransferService{
		watcher: watcher,
		queue:   queue,
		nas:     nas,
		cleanup: cleanup,
		stats:   queue.stats,
	}, nil
}

func (ts *TransferService) Start(ctx context.Context) error {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ts.cleanup.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Cleanup error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ts.watcher.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Watcher error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ts.queue.ProcessQueue(ctx); err != nil && err != context.Canceled {
			log.Printf("Queue error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ts.reportStats(ctx)
	}()

	log.Println("Transfer service started")
	wg.Wait()

	return nil
}

func (ts *TransferService) reportStats(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			added, completed, failed, pending, bytes := ts.stats.GetStats()
			queueSize := ts.queue.GetQueueSize()
			cleanupPending := ts.cleanup.GetPendingCount()

			log.Printf("Transfer Stats: Added: %d, Completed: %d, Failed: %d, Pending: %d, Bytes: %d, Queue Size: %d, Cleanup Pending: %d", added, completed, failed, pending, bytes, queueSize, cleanupPending)
		}
	}
}

func (ts *TransferService) Shutdown(ctx context.Context) error {
	log.Println("Shutting down transfer service...")

	if err := ts.queue.SaveState(); err != nil {
		return fmt.Errorf("Failed to save queue state: %w", err)
	}

	if err := ts.cleanup.ForceCleanupAll(ctx); err != nil {
		return fmt.Errorf("Failed to force cleanup: %w", err)
	}

	// Disconnect from NAS
	if err := ts.nas.Disconnect(); err != nil {
		log.Printf("Warning: failed to disconnect from NAS: %v", err)
	}

	log.Println("Transfer service shut down")

	return nil
}

// QueueExistingFiles scans a directory for .ts files and queues them for transfer
func (ts *TransferService) QueueExistingFiles(localEventPath string) error {
	cfg := constants.MustGetConfig()
	log.Printf("Scanning for existing files in: %s", localEventPath)

	var fileCount, alreadyTransferred, scheduledForCleanup int

	// Extract event name from path for NAS destination
	eventName := filepath.Base(localEventPath)

	err := filepath.Walk(localEventPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			return nil // Continue walking
		}

		// Only process .ts files
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".ts") {
			// Extract resolution from directory path
			resolution := ts.extractResolutionFromPath(path)

			// Get relative path from event directory
			relPath, err := filepath.Rel(localEventPath, path)
			if err != nil {
				log.Printf("Failed to get relative path for %s: %v", path, err)
				return nil
			}

			// Build NAS destination path (eventName/relPath)
			nasDestPath := filepath.Join(eventName, relPath)

			// Check if file already exists on NAS with matching size
			exists, err := ts.nas.FileExists(nasDestPath, info.Size())
			if err != nil {
				log.Printf("Failed to check NAS file existence for %s: %v", path, err)
				// Continue with transfer attempt on error
			} else if exists {
				log.Printf("File already exists on NAS: %s (%s, %d bytes)", path, resolution, info.Size())
				alreadyTransferred++

				// Schedule for cleanup if cleanup is enabled
				if cfg.Cleanup.AfterTransfer {
					if err := ts.cleanup.ScheduleCleanup(path); err != nil {
						log.Printf("Failed to schedule cleanup for already-transferred file %s: %v", path, err)
					} else {
						scheduledForCleanup++
					}
				}
				return nil // Skip queuing this file
			}

			// Create transfer item
			item := TransferItem{
				ID:              ts.generateTransferID(),
				SourcePath:      path,
				DestinationPath: nasDestPath,
				Resolution:      resolution,
				Timestamp:       info.ModTime(),
				Status:          StatusPending,
				FileSize:        info.Size(),
			}

			// Add to queue
			if err := ts.queue.Add(item); err != nil {
				log.Printf("Failed to queue file %s: %v", path, err)
			} else {
				log.Printf("Queued file: %s (%s, %d bytes)", path, resolution, info.Size())
				fileCount++
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	log.Printf("File scan completed - Queued: %d, Already transferred: %d, Scheduled for cleanup: %d",
		fileCount, alreadyTransferred, scheduledForCleanup)
	return nil
}

func (ts *TransferService) extractResolutionFromPath(filePath string) string {
	dir := filepath.Dir(filePath)
	parts := strings.Split(dir, string(filepath.Separator))

	for _, part := range parts {
		if strings.HasSuffix(part, "p") {
			return part
		}
	}

	return "unknown"
}

func (ts *TransferService) generateTransferID() string {
	return fmt.Sprintf("transfer_existing_%d", time.Now().UnixNano())
}
