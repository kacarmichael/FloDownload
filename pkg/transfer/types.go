package transfer

import (
	"sync"
	"time"
)

type TransferItem struct {
	ID              string
	SourcePath      string
	DestinationPath string
	Resolution      string
	Timestamp       time.Time
	RetryCount      int
	Status          TransferStatus
	FileSize        int64
	LastError       string
}

type TransferStatus int

const (
	StatusPending TransferStatus = iota
	StatusInProgress
	StatusCompleted
	StatusFailed
	StatusRetrying
)

func (s TransferStatus) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusInProgress:
		return "In Progress"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	case StatusRetrying:
		return "Retrying"
	default:
		return "Unknown"
	}
}

type QueueConfig struct {
	WorkerCount     int
	PersistencePath string
	MaxQueueSize    int
	BatchSize       int
}

type CleanupConfig struct {
	Enabled         bool
	RetentionPeriod time.Duration
	BatchSize       int
	CheckInterval   time.Duration
}

type QueueStats struct {
	mu               sync.Mutex
	TotalAdded       int
	TotalCompleted   int
	TotalFailed      int
	CurrentPending   int
	BytesTransferred int64
}

func (qs *QueueStats) IncrementAdded() {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.TotalAdded++
	qs.CurrentPending++
}

func (qs *QueueStats) IncrementCompleted(bytes int64) {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.TotalCompleted++
	qs.CurrentPending--
	qs.BytesTransferred += bytes
}

func (qs *QueueStats) IncrementFailed() {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	qs.TotalFailed++
	qs.CurrentPending--
}

func (qs *QueueStats) GetStats() (int, int, int, int, int64) {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	return qs.TotalAdded, qs.TotalCompleted, qs.TotalFailed, qs.CurrentPending, qs.BytesTransferred
}
