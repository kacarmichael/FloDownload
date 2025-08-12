package transfer

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"m3u8-downloader/pkg/nas"
	"os"
	"sync"
	"time"
)

type TransferQueue struct {
	config     QueueConfig
	items      *PriorityQueue
	stats      *QueueStats
	nasService *nas.NASService
	cleanup    *CleanupService
	workers    []chan TransferItem
	mu         sync.RWMutex
}

type PriorityQueue []*TransferItem

func (pq PriorityQueue) Len() int {
	return len(pq)
}

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Timestamp.After(pq[j].Timestamp)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*TransferItem)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func NewTransferQueue(config QueueConfig, nasTransfer *nas.NASService, cleanup *CleanupService) *TransferQueue {
	pq := &PriorityQueue{}
	heap.Init(pq)

	tq := &TransferQueue{
		config:     config,
		items:      pq,
		stats:      &QueueStats{},
		nasService: nasTransfer,
		cleanup:    cleanup,
		workers:    make([]chan TransferItem, config.WorkerCount),
	}

	if err := tq.LoadState(); err != nil {
		log.Printf("Failed to load queue state: %v", err)
	}

	return tq
}

func (tq *TransferQueue) Add(item TransferItem) error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	if tq.items.Len() >= tq.config.MaxQueueSize {
		return fmt.Errorf("Queue is full (max size: %d)", tq.config.MaxQueueSize)
	}

	heap.Push(tq.items, &item)
	tq.stats.IncrementAdded()

	log.Printf("Added file to queue: %s", item.SourcePath)

	return nil
}

func (tq *TransferQueue) ProcessQueue(ctx context.Context) error {
	for i := 0; i < tq.config.WorkerCount; i++ {
		workerChan := make(chan TransferItem, 1)
		tq.workers[i] = workerChan

		go func(workerID int, workChan chan TransferItem) {
			log.Printf("Worker %d started", workerID)
			for {
				select {
				case <-ctx.Done():
					log.Printf("Transfer worker %d shutting down...", workerID)
					return
				case item := <-workChan:
					tq.processItem(ctx, item)
				}
			}
		}(i, workerChan)
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Transfer queue shutting down...")
			return ctx.Err()
		case <-ticker.C:
			tq.dispatchWork()

			if time.Now().Unix()%30 == 0 {
				if err := tq.SaveState(); err != nil {
					log.Printf("Failed to save queue state: %v", err)
				}
			}
		}
	}
}

func (tq *TransferQueue) dispatchWork() {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	for i, workerChan := range tq.workers {
		if len(workerChan) == 0 && tq.items.Len() > 0 {
			item := heap.Pop(tq.items).(*TransferItem)
			item.Status = StatusInProgress

			select {
			case workerChan <- *item:
				log.Printf("Dispatched file to worker %d: %s", i, item.SourcePath)
			default:
				heap.Push(tq.items, item)
				item.Status = StatusPending

			}
		}
	}
}

func (tq *TransferQueue) processItem(ctx context.Context, item TransferItem) {
	// Check if file already exists on NAS before attempting transfer
	if exists, err := tq.nasService.FileExists(item.DestinationPath, item.FileSize); err != nil {
		log.Printf("Failed to check if file exists on NAS for %s: %v", item.SourcePath, err)
		// Continue with transfer attempt on error
	} else if exists {
		log.Printf("File already exists on NAS, skipping transfer: %s", item.SourcePath)
		item.Status = StatusCompleted
		tq.stats.IncrementCompleted(item.FileSize)

		// Schedule for cleanup
		if tq.cleanup != nil {
			if err := tq.cleanup.ScheduleCleanup(item.SourcePath); err != nil {
				log.Printf("Failed to schedule cleanup for existing file %s: %v", item.SourcePath, err)
			}
		}
		return
	}

	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			item.Status = StatusRetrying
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Printf("Backing off for %d seconds before retrying (attempt %d/%d)", backoff, attempt, maxRetries)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
		}

		err := TransferFile(tq.nasService, ctx, &item)
		if err == nil {
			item.Status = StatusCompleted
			tq.stats.IncrementCompleted(item.FileSize)

			if tq.cleanup != nil {
				if err := tq.cleanup.ScheduleCleanup(item.SourcePath); err != nil {
					log.Printf("Failed to add file to cleanup list: %v", err)
				}
			}
			log.Printf("File transfer completed: %s", item.SourcePath)
			return
		}

		item.LastError = err.Error()
		item.RetryCount++

		log.Printf("File transfer failed: %s (attempt %d/%d): %v", item.SourcePath, item.RetryCount, maxRetries, err)

		if attempt == maxRetries {
			item.Status = StatusFailed
			tq.stats.IncrementFailed()
			log.Printf("Transfer permanently failed for file: %s", item.SourcePath)
			return
		}
	}
}

func (tq *TransferQueue) SaveState() error {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	items := make([]*TransferItem, tq.items.Len())
	tempPQ := make(PriorityQueue, tq.items.Len())
	copy(tempPQ, *tq.items)

	for i := 0; i < len(items); i++ {
		items[i] = heap.Pop(&tempPQ).(*TransferItem)
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"items":     items,
		"stats":     tq.stats,
		"timestamp": time.Now(),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("Failed to marshal queue state: %w", err)
	}

	if err := os.WriteFile(tq.config.PersistencePath, data, 0644); err != nil {
		return fmt.Errorf("Failed to save queue state: %w", err)
	}

	return nil
}

func (tq *TransferQueue) LoadState() error {
	data, err := os.ReadFile(tq.config.PersistencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Failed to load queue state: %w", err)
	}

	var state struct {
		Items     []*TransferItem `json:"items"`
		Stats     *QueueStats     `json:"stats"`
		Timestamp time.Time       `json:"timestamp"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("Failed to load queue state: %w", err)
	}

	tq.mu.Lock()
	defer tq.mu.Unlock()

	for _, item := range state.Items {
		if item.Status == StatusPending || item.Status == StatusFailed {
			heap.Push(tq.items, item)
		}
	}

	if state.Stats != nil {
		tq.stats = state.Stats
	}

	log.Printf("Loaded queue state: %d items restored from %v",
		tq.items.Len(), state.Timestamp.Format(time.RFC3339))
	return nil
}

func (tq *TransferQueue) GetStats() (int, int, int, int, int64) {
	return tq.stats.GetStats()
}

func (tq *TransferQueue) GetQueueSize() int {
	tq.mu.RLock()
	defer tq.mu.RUnlock()
	return tq.items.Len()
}
