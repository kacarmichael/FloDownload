package transfer

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type CleanupService struct {
	config       CleanupConfig
	pendingFiles []string
	mu           sync.Mutex
}

func NewCleanupService(config CleanupConfig) *CleanupService {
	return &CleanupService{
		config:       config,
		pendingFiles: make([]string, 0),
	}
}

func (cs *CleanupService) ScheduleCleanup(filePath string) error {
	if !cs.config.Enabled {
		return nil
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.pendingFiles = append(cs.pendingFiles, filePath)
	log.Printf("Scheduled file for cleanup: %s", filePath)
	return nil
}

func (cs *CleanupService) Start(ctx context.Context) error {
	if !cs.config.Enabled {
		log.Println("Cleanup service disabled")
		return nil
	}

	log.Printf("Cleanup service started (retention: %v, batch: %d)", cs.config.RetentionPeriod, cs.config.BatchSize)

	ticker := time.NewTicker(cs.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Cleanup service shutting down...")
			return ctx.Err()
		case <-ticker.C:
			if err := cs.ExecuteCleanup(ctx); err != nil {
				log.Printf("Cleanup error: %v", err)
			}
		}
	}
}

func (cs *CleanupService) ExecuteCleanup(ctx context.Context) error {
	cs.mu.Lock()
	if len(cs.pendingFiles) == 0 {
		cs.mu.Unlock()
		return nil
	}

	batchSize := cs.config.BatchSize
	if batchSize > len(cs.pendingFiles) {
		batchSize = len(cs.pendingFiles)
	}

	log.Printf("Executing cleanup batch (size: %d)", batchSize)

	batch := make([]string, batchSize)
	copy(batch, cs.pendingFiles[:batchSize])
	cs.pendingFiles = cs.pendingFiles[batchSize:]
	cs.mu.Unlock()

	log.Printf("Processing %d files for cleanup", len(batch))

	var cleanedCount int
	var errors []error

	for _, filePath := range batch {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := cs.cleanupFile(filePath); err != nil {
			errors = append(errors, fmt.Errorf("Failed to cleanup file %s: %w", filePath, err))
		} else {
			cleanedCount++
		}
	}

	log.Printf("Cleanup batch completed (cleaned: %d, errors: %d)", cleanedCount, len(errors))

	if len(errors) > 0 {
		for i, err := range errors {
			if i >= 3 {
				log.Printf("... and %d more errors", len(errors)-3)
				break
			}
			log.Printf("Error: %v", err)
		}
	}

	return nil

}

func (cs *CleanupService) cleanupFile(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Failed to get file info: %w", err)
	}

	if cs.config.RetentionPeriod > 0 {
		if time.Since(info.ModTime()) < cs.config.RetentionPeriod {
			log.Printf("File too new to cleanup: %s", filePath)
			return nil
		}
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("Failed to remove file: %w", err)
	}

	log.Printf("File cleaned up: %s", filePath)
	return nil
}

func (cs *CleanupService) GetPendingCount() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.pendingFiles)
}

func (cs *CleanupService) ForceCleanupAll(ctx context.Context) error {
	log.Println("Force cleanup requested")

	for {
		cs.mu.Lock()
		pendingCount := len(cs.pendingFiles)
		cs.mu.Unlock()

		if pendingCount == 0 {
			break
		}

		if err := cs.ExecuteCleanup(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	log.Println("Force cleanup complete")
	return nil
}
