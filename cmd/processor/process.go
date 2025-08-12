package processor

import (
	"context"
	"log"
	"m3u8-downloader/pkg/constants"
	"m3u8-downloader/pkg/processing"
)

func Process(eventName string) {
	log.Printf("Starting processing for event: %s", eventName)
	cfg := constants.MustGetConfig()
	ps, err := processing.NewProcessingService(eventName, cfg)
	if err != nil {
		log.Fatalf("Failed to create processing service: %v", err)
	}
	if err := ps.Start(context.Background()); err != nil {
		log.Fatalf("Failed to run processing service: %v", err)
	}
}
