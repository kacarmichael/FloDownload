package transfer

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	outputDir    string
	queue        *TransferQueue
	watcher      *fsnotify.Watcher
	settingDelay time.Duration
	pendingFiles map[string]*time.Timer
	mu           sync.Mutex
}

func NewFileWatcher(outputDir string, queue *TransferQueue, settlingDelay time.Duration) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &FileWatcher{
		outputDir:    outputDir,
		queue:        queue,
		watcher:      watcher,
		settingDelay: settlingDelay,
		pendingFiles: make(map[string]*time.Timer),
	}, nil
}

func (fw *FileWatcher) Start(ctx context.Context) error {
	defer fw.watcher.Close()

	if err := fw.addWatchRecursive(fw.outputDir); err != nil {
		return fmt.Errorf("Failed to add watch paths: %w", err)
	}

	log.Printf("Starting file watcher on %s", fw.outputDir)

	for {
		select {
		case <-ctx.Done():
			log.Println("File watcher shutting down...")
			return ctx.Err()

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return fmt.Errorf("Watcher events channel closed")
			}
			fw.handleFileEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return fmt.Errorf("Watcher errors channel closed")
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func (fw *FileWatcher) addWatchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error walking path %s: %v", path, err)
			return nil
		}

		if info.IsDir() {
			if err := fw.watcher.Add(path); err != nil {
				log.Printf("Failed to watch directory %s: %v", path, err)
			} else {
				log.Printf("Watching directory %s", path)
			}
		}

		return nil
	})
}

func (fw *FileWatcher) handleFileEvent(event fsnotify.Event) {
	if !strings.HasSuffix(strings.ToLower(event.Name), ".ts") {
		return
	}

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		fw.scheduleTransfer(event.Name)
	case event.Op&fsnotify.Write == fsnotify.Write:
		fw.scheduleTransfer(event.Name)
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		fw.cancelPendingTransfer(event.Name)
	}

	if event.Op&fsnotify.Create == fsnotify.Create {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := fw.watcher.Add(event.Name); err != nil {
				log.Printf("Failed to watch directory %s: %v", event.Name, err)
			} else {
				log.Printf("Watching directory %s", event.Name)
			}
		}
	}
}

func (fw *FileWatcher) scheduleTransfer(filePath string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if timer, exists := fw.pendingFiles[filePath]; exists {
		timer.Stop()
	}

	fw.pendingFiles[filePath] = time.AfterFunc(fw.settingDelay, func() {
		fw.processFile(filePath)
		fw.mu.Lock()
		delete(fw.pendingFiles, filePath)
		fw.mu.Unlock()
	})

	log.Printf("Scheduled file for transfer: %s", filePath)
}

func (fw *FileWatcher) cancelPendingTransfer(filePath string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if timer, exists := fw.pendingFiles[filePath]; exists {
		timer.Stop()
		delete(fw.pendingFiles, filePath)
		log.Printf("Canceled pending transfer for file: %s", filePath)
	}
}

func (fw *FileWatcher) processFile(filePath string) {
	info, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Failed to stat file %s: %v", filePath, err)
		return
	}

	resolution := fw.extractResolution(filePath)

	relPath, err := filepath.Rel(fw.outputDir, filePath)
	if err != nil {
		log.Printf("Failed to get relative path for file %s: %v", filePath, err)
		return
	}

	item := TransferItem{
		ID:              generateID(),
		SourcePath:      filePath,
		DestinationPath: relPath,
		Resolution:      resolution,
		Timestamp:       time.Now(),
		Status:          StatusPending,
		FileSize:        info.Size(),
	}

	if err := fw.queue.Add(item); err != nil {
		log.Printf("Failed to add file to queue: %v", err)
	} else {
		log.Printf("Added file to queue: %s", filePath)
	}
}

func (fw *FileWatcher) extractResolution(filePath string) string {
	dir := filepath.Dir(filePath)
	parts := strings.Split(dir, string(filepath.Separator))

	for _, part := range parts {
		if strings.HasSuffix(part, "p") {
			return part
		}
	}

	return ""
}

func generateID() string {
	return fmt.Sprintf("transfer_%d_%d", time.Now().UnixNano(), rand.Intn(1000))
}
