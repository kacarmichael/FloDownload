# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based HLS (HTTP Live Streaming) recorder that monitors M3U8 playlists and downloads video segments in real-time with automatic NAS transfer capabilities. The program takes a master M3U8 playlist URL, parses all available stream variants (different qualities/bitrates), continuously monitors each variant's chunklist for new segments, downloads them locally, and optionally transfers them to network storage for long-term archival.

## Architecture

The project follows a modular architecture with clear separation of concerns:

- **cmd/**: Entry points for different execution modes
  - **main/main.go**: Primary CLI entry point with URL input, event naming, and mode selection
  - **downloader/download.go**: Core download orchestration logic with transfer service integration
  - **processor/process.go**: Alternative processing entry point
  - **transfer/transfer.go**: Transfer-only mode entry point
- **pkg/**: Core packages containing the application logic
  - **media/**: HLS streaming and download logic
    - **stream.go**: Stream variant parsing and downloading orchestration (`GetAllVariants`, `VariantDownloader`)
    - **playlist.go**: M3U8 playlist loading and parsing (`LoadMediaPlaylist`)
    - **segment.go**: Individual segment downloading logic (`DownloadSegment`, `SegmentJob`)
    - **manifest.go**: Manifest generation and segment tracking (`ManifestWriter`, `ManifestItem`)
  - **transfer/**: NAS transfer system (complete implementation available)
    - **service.go**: Transfer service orchestration
    - **watcher.go**: File system monitoring for new downloads
    - **queue.go**: Priority queue with worker pool management
    - **nas.go**: NAS file transfer with retry logic
    - **cleanup.go**: Local file cleanup after successful transfer
    - **types.go**: Transfer system data structures
  - **processing/**: Video processing and concatenation system
    - **service.go**: Processing service orchestration with FFmpeg integration
    - **segment.go**: Individual segment processing logic
    - **types.go**: Processing system data structures
  - **nas/**: NAS connection and file operations
    - **config.go**: NAS configuration structure
    - **nas.go**: NAS service with connection management and file operations
  - **config/**: Centralized configuration management with validation
    - **config.go**: Configuration loading, validation, and path resolution
  - **utils/**: Utility functions for cross-platform compatibility
    - **paths.go**: Path manipulation and validation utilities
  - **constants/constants.go**: Configuration constants and singleton access
  - **httpClient/error.go**: HTTP error handling utilities

## Core Functionality

### Download Workflow
1. **Parse Master Playlist**: `GetAllVariants()` fetches and parses the master M3U8 to extract all stream variants with different qualities/bitrates
2. **Concurrent Monitoring**: Each variant gets its own goroutine running `VariantDownloader()` that continuously polls for playlist updates
3. **Segment Detection**: When new segments appear in a variant's playlist, they are queued for download
4. **Parallel Downloads**: Segments are downloaded concurrently with configurable worker pools and retry logic
5. **Quality Organization**: Downloaded segments are organized by resolution (1080p, 720p, etc.) in separate directories
6. **Manifest Generation**: `ManifestWriter` tracks all downloaded segments with sequence numbers and resolutions

### NAS Transfer Workflow (Optional)
1. **File Watching**: `FileWatcher` monitors download directories for new `.ts` files
2. **Transfer Queuing**: New files are added to a priority queue after a settling delay
3. **Background Transfer**: Worker pool transfers files to NAS with retry logic and verification
4. **Local Cleanup**: Successfully transferred files are automatically cleaned up locally
5. **State Persistence**: Queue state is persisted to survive crashes and restarts

### Video Processing Workflow (Optional)
1. **Segment Collection**: Processing service reads downloaded segments from NAS storage
2. **Quality Selection**: Automatically selects the highest quality variant available
3. **FFmpeg Processing**: Uses FFmpeg to concatenate segments into a single MP4 file
4. **Output Management**: Processed videos are saved to the configured output directory
5. **Concurrent Processing**: Multiple events can be processed simultaneously with worker pools

## Key Data Structures

- `StreamVariant`: Represents a stream quality variant with URL, bandwidth, resolution, output directory, and manifest writer
- `SegmentJob`: Represents a segment download task with URI, sequence number, and variant info
- `ManifestWriter`: Tracks downloaded segments and generates JSON manifests
- `ManifestItem`: Individual segment record with sequence number and resolution
- `TransferItem`: Transfer queue item with source, destination, retry count, and status
- `TransferService`: Orchestrates file watching, queuing, transfer, and cleanup
- `ProcessingService`: Manages video processing operations with FFmpeg integration
- `ProcessConfig`: Configuration for processing operations including worker count and paths
- `NASService`: Handles NAS connection, authentication, and file operations
- `NASConfig`: Configuration structure for NAS connection parameters

## Configuration

Configuration is managed through a centralized system in `pkg/config/config.go` with environment variable support for deployment flexibility. The system provides validation, cross-platform path resolution, and sensible defaults:

### Core Settings
- `Core.WorkerCount`: Number of concurrent segment downloaders per variant (4) - ENV: `WORKER_COUNT`
- `Core.RefreshDelay`: How often to check for playlist updates (3 seconds) - ENV: `REFRESH_DELAY_SECONDS`

### Path Configuration
- `Paths.LocalOutput`: Base directory for local downloads (`data/`) - ENV: `LOCAL_OUTPUT_DIR`
- `Paths.ProcessOutput`: Directory for processed videos (`out/`) - ENV: `PROCESS_OUTPUT_DIR`
- `Paths.ManifestDir`: Directory for manifest JSON files (`data/`)
- `Paths.PersistenceFile`: Transfer queue state file location

### HTTP Settings
- `HTTPUserAgent`: User agent string for HTTP requests
- `REFERRER`: Referer header for HTTP requests (`https://www.flomarching.com`)

### NAS Transfer Settings
- `NAS.EnableTransfer`: Enable/disable automatic NAS transfer (true) - ENV: `ENABLE_NAS_TRANSFER`
- `NAS.OutputPath`: UNC path to NAS storage (``) - ENV: `NAS_OUTPUT_PATH`
- `NAS.Username`/`NAS.Password`: NAS credentials for authentication - ENV: `NAS_USERNAME`/`NAS_PASSWORD`
- `Transfer.WorkerCount`: Concurrent transfer workers (2)
- `Transfer.RetryLimit`: Max retry attempts per file (3)
- `Transfer.Timeout`: Timeout per file transfer (30 seconds)
- `Transfer.FileSettlingDelay`: Wait before queuing new files (5 seconds)
- `Transfer.QueueSize`: Maximum queue size (100000)
- `Transfer.BatchSize`: Batch processing size (1000)

### Processing Settings
- `Processing.AutoProcess`: Enable automatic processing after download (true)
- `Processing.Enabled`: Enable processing functionality (true)
- `Processing.WorkerCount`: Concurrent processing workers (2)
- `Processing.FFmpegPath`: Path to FFmpeg executable (`ffmpeg`) - ENV: `FFMPEG_PATH`

### Cleanup Settings
- `Cleanup.AfterTransfer`: Delete local files after NAS transfer (true)
- `Cleanup.BatchSize`: Files processed per cleanup batch (1000)
- `Cleanup.RetainHours`: Hours to keep local files (0 = immediate cleanup)

### Configuration Access
```go
cfg := constants.MustGetConfig()  // Get validated config singleton
eventPath := cfg.GetEventPath("my-event")  // Get cross-platform paths
```

See `DEPLOYMENT.md` for detailed environment variable configuration and deployment examples.

## Common Development Commands

```bash
# Build the main application
go build -o stream-recorder ./cmd/main

# Run with URL prompt
go run ./cmd/main/main.go

# Run with command line arguments
go run ./cmd/main/main.go -url="https://example.com/playlist.m3u8" -event="my-event" -debug=true

# Run with module support
go mod tidy

# Test the project (when tests are added)
go test ./...

# Format code
go fmt ./...
```

## Command Line Options

- `-url`: M3U8 playlist URL (if not provided, prompts for input)
- `-event`: Event name for organizing downloads (defaults to current date)
- `-debug`: Debug mode (only downloads 1080p variant for easier testing)
- `-transfer`: Transfer-only mode (transfer existing files without downloading)
- `-process`: Process-only mode (process existing files without downloading)

## Monitoring and Downloads

The application implements comprehensive real-time stream monitoring:

### Download Features
- **Continuous Polling**: Each variant playlist is checked every 3 seconds for new segments
- **Deduplication**: Uses segment URIs and sequence numbers to avoid re-downloading
- **Graceful Shutdown**: Responds to SIGINT/SIGTERM signals for clean exit
- **Error Resilience**: Retries failed downloads and handles HTTP 403 errors specially
- **Quality Detection**: Automatically determines resolution from bandwidth or explicit resolution data
- **Context Cancellation**: Proper timeout and cancellation handling for clean shutdowns

### Transfer Features (when enabled)
- **Real-time Transfer**: Files are transferred to NAS as soon as they're downloaded
- **Queue Persistence**: Transfer queue survives application restarts
- **Retry Logic**: Failed transfers are retried with exponential backoff
- **Verification**: File sizes are verified after transfer
- **Automatic Cleanup**: Local files are removed after successful NAS transfer
- **Statistics Reporting**: Transfer progress and statistics are logged regularly

### Manifest Generation
- **Segment Tracking**: All downloaded segments are tracked with sequence numbers
- **Resolution Mapping**: Segments are associated with their quality variants
- **JSON Output**: Manifest files are generated as sorted JSON arrays for easy processing

## Error Handling

The implementation uses proper Go error handling patterns:
- **Custom HTTP Errors**: Structured error types for HTTP failures
- **Context-Aware Cancellation**: Proper handling of shutdown scenarios
- **Retry Logic**: Exponential backoff for transient failures  
- **Logging**: Clear status indicators (✓ for success, ✗ for failure)
- **Graceful Degradation**: Transfer service failures don't stop downloads

## Dependencies

- `github.com/grafov/m3u8`: M3U8 playlist parsing
- `github.com/fsnotify/fsnotify`: File system event monitoring for NAS transfers

## Data Organization

Downloaded files are organized as:
```
./data/
├── {event-name}.json          # Manifest file
├── {event-name}/              # Event-specific directory
│   ├── 1080p/                 # High quality segments
│   ├── 720p/                  # Medium quality segments
│   └── 480p/                  # Lower quality segments
├── transfer_queue.json        # Transfer queue state
├── refresh_token.txt          # Authentication tokens
└── tokens.txt                 # Session tokens
```

NAS files mirror the local structure:
```
\\HomeLabNAS\dci\streams\
└── {event-name}/
    ├── 1080p/
    ├── 720p/
    └── 480p/
```

Processed files are output to:
```
./out/
└── {event-name}/
    └── concatenated_segments.mp4   # Final processed video
```