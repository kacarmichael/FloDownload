package downloader

import (
	"context"
	"log"
	"m3u8-downloader/pkg/constants"
	"m3u8-downloader/pkg/media"
	"m3u8-downloader/pkg/transfer"
	"m3u8-downloader/pkg/utils"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func Download(masterURL string, eventName string, debug bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Goroutine to listen for shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	cfg := constants.MustGetConfig()

	var wg sync.WaitGroup
	var transferService *transfer.TransferService
	if cfg.NAS.EnableTransfer {
		ts, err := transfer.NewTrasferService(cfg.NAS.OutputPath, eventName)
		if err != nil {
			log.Printf("Failed to create transfer service: %v", err)
			log.Println("Continuing without transfer service...")
		} else {
			transferService = ts
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := transferService.Start(ctx); err != nil && err != context.Canceled {
					log.Printf("Transfer service error: %v", err)
				}
			}()
			log.Println("Transfer service started.")
		}
	}

	manifestWriter := media.NewManifestWriter(eventName)

	eventPath := cfg.GetEventPath(eventName)
	if err := utils.EnsureDir(eventPath); err != nil {
		log.Fatalf("Failed to create event directory: %v", err)
	}

	variants, err := media.GetAllVariants(masterURL, eventPath, manifestWriter)
	if err != nil {
		log.Fatalf("Failed to get variants: %v", err)
	}
	log.Printf("Found %d variants", len(variants))

	sem := make(chan struct{}, constants.WorkerCount*len(variants))

	manifest := media.NewManifestWriter(eventName)

	for _, variant := range variants {
		// Debug mode only tracks one variant for easier debugging
		if debug {
			if variant.Resolution != "1080p" {
				continue
			}
		}
		wg.Add(1)
		go func(v *media.StreamVariant) {
			defer wg.Done()
			media.VariantDownloader(ctx, v, sem, manifest)
		}(variant)
	}

	wg.Wait()
	log.Println("All variant downloaders finished.")

	if transferService != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		transferService.Shutdown(shutdownCtx)
	}

	log.Println("All Services shut down.")

	manifestWriter.WriteManifest()
	log.Println("Manifest written.")
}
