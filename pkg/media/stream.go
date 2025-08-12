package media

import (
	"context"
	"errors"
	"fmt"
	"github.com/grafov/m3u8"
	"log"
	"m3u8-downloader/pkg/constants"
	"m3u8-downloader/pkg/httpClient"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type StreamVariant struct {
	URL        string
	Bandwidth  uint32
	BaseURL    *url.URL
	ID         int
	Resolution string
	OutputDir  string
	Writer     *ManifestWriter
}

func extractResolution(variant *m3u8.Variant) string {
	if variant.Resolution != "" {
		parts := strings.Split(variant.Resolution, "x")
		if len(parts) == 2 {
			return parts[1] + "p"
		}
	}
	switch {
	case variant.Bandwidth >= 5000000:
		return "1080p"
	case variant.Bandwidth >= 3000000:
		return "720p"
	case variant.Bandwidth >= 1500000:
		return "480p"
	case variant.Bandwidth >= 800000:
		return "360p"
	default:
		return "240p"
	}
}

func GetAllVariants(masterURL string, outputDir string, writer *ManifestWriter) ([]*StreamVariant, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", masterURL, nil)
	req.Header.Set("User-Agent", constants.HTTPUserAgent)
	req.Header.Set("Referer", constants.REFERRER)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return nil, err
	}

	base, _ := url.Parse(masterURL)

	if listType == m3u8.MEDIA {
		return []*StreamVariant{{
			URL:        masterURL,
			Bandwidth:  0,
			BaseURL:    base,
			ID:         0,
			Resolution: "unknown",
			OutputDir:  path.Join(outputDir, "unknown"),
			Writer:     writer,
		}}, nil
	}

	master := playlist.(*m3u8.MasterPlaylist)
	if len(master.Variants) == 0 {
		return nil, fmt.Errorf("no variants found in master playlist")
	}

	variants := make([]*StreamVariant, 0, len(master.Variants))
	for i, v := range master.Variants {
		vURL, _ := url.Parse(v.URI)
		fullURL := base.ResolveReference(vURL).String()
		resolution := extractResolution(v)
		outputDir := path.Join(outputDir, resolution)
		variants = append(variants, &StreamVariant{
			URL:        fullURL,
			Bandwidth:  v.Bandwidth,
			BaseURL:    base.ResolveReference(vURL),
			ID:         i,
			Resolution: resolution,
			OutputDir:  outputDir,
		})
	}
	return variants, nil
}

func VariantDownloader(ctx context.Context, variant *StreamVariant, sem chan struct{}, manifest *ManifestWriter) {
	log.Printf("Starting %s variant downloader (bandwidth: %d)", variant.Resolution, variant.Bandwidth)
	ticker := time.NewTicker(constants.RefreshDelay)
	defer ticker.Stop()
	client := &http.Client{}
	seen := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		playlist, err := LoadMediaPlaylist(variant.URL)
		seq := playlist.SeqNo
		if err != nil {
			log.Printf("%s: Error loading playlist playlist: %v", variant.Resolution, err)
			goto waitTick
		}

		for _, seg := range playlist.Segments {
			if seg == nil {
				continue
			}
			job := SegmentJob{
				URI:       seg.URI,
				Seq:       seq,
				VariantID: variant.ID,
				Variant:   variant,
			}
			segmentKey := job.Key()
			if seen[segmentKey] {
				seq++
				continue
			}
			seen[segmentKey] = true

			sem <- struct{}{} // Acquire
			go func(j SegmentJob) {
				defer func() { <-sem }() // Release
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				err := DownloadSegment(ctx, client, j.AbsoluteURL(), j.Variant.OutputDir)
				name := strings.TrimSuffix(path.Base(j.Key()), path.Ext(path.Base(j.Key())))

				if err == nil {
					log.Printf("✓ %s downloaded segment %s", j.Variant.Resolution, name)
					return
				}

				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					// Suppress log: shutdown in progress
					return
				}

				if httpClient.IsHTTPStatus(err, 403) {
					log.Printf("✗ %s failed to download segment %s (403)", j.Variant.Resolution, name)
				} else {
					log.Printf("✗ %s failed to download segment %s: %v", j.Variant.Resolution, name, err)
				}
			}(job)
			seq++
		}

		if playlist.Closed {
			log.Printf("%s: Playlist closed (#EXT-X-ENDLIST)", variant.Resolution)
			return
		}

	waitTick:
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
