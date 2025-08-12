package transfer

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"m3u8-downloader/pkg/config"
	"m3u8-downloader/pkg/constants"
	"m3u8-downloader/pkg/transfer"
	"m3u8-downloader/pkg/utils"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func getEventDirs(cfg *config.Config) ([]string, error) {
	dirs, err := os.ReadDir(cfg.Paths.LocalOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	var eventDirs []string
	for _, dir := range dirs {
		if dir.IsDir() {
			eventDirs = append(eventDirs, dir.Name())
		}
	}
	return eventDirs, nil
}

func RunTransferOnly(eventName string) {
	cfg := constants.MustGetConfig()

	// Check if NAS transfer is enabled
	if !cfg.NAS.EnableTransfer {
		log.Fatal("NAS transfer is disabled in configuration. Please enable it to use transfer-only mode.")
	}

	if eventName == "" {
		events, err := getEventDirs(cfg)
		if err != nil {
			log.Fatalf("Failed to get event directories: %v", err)
		}
		if len(events) == 0 {
			log.Fatal("No events found")
		}
		if len(events) > 1 {
			fmt.Println("Multiple events found, please select one:")
			for i, event := range events {
				fmt.Printf("%d. %s\n", i+1, event)
			}
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			index, err := strconv.Atoi(input)
			if err != nil {
				log.Fatalf("Failed to parse input: %v", err)
			}
			if index < 1 || index > len(events) {
				log.Fatal("Invalid input")
			}
			eventName = events[index-1]
		} else {
			eventName = events[0]
		}
	}

	log.Printf("Starting transfer-only mode for event: %s", eventName)

	// Setup context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down transfer service...")
		cancel()
	}()

	// Verify local event directory exists
	localEventPath := cfg.GetEventPath(eventName)
	if !utils.PathExists(localEventPath) {
		log.Fatalf("Local event directory does not exist: %s", localEventPath)
	}

	// Create transfer service
	transferService, err := transfer.NewTrasferService(cfg.NAS.OutputPath, eventName)
	if err != nil {
		log.Fatalf("Failed to create transfer service: %v", err)
	}

	// Find and queue existing files
	if err := transferService.QueueExistingFiles(localEventPath); err != nil {
		log.Fatalf("Failed to queue existing files: %v", err)
	}

	// Start transfer service
	log.Println("Starting transfer service...")
	if err := transferService.Start(ctx); err != nil && err != context.Canceled {
		log.Printf("Transfer service error: %v", err)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	transferService.Shutdown(shutdownCtx)

	log.Println("Transfer-only mode completed.")
}
