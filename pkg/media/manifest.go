package media

import (
	"encoding/json"
	"log"
	"m3u8-downloader/pkg/constants"
	"m3u8-downloader/pkg/utils"
	"os"
	"sort"
)

type ManifestWriter struct {
	ManifestPath string
	Segments     []ManifestItem
	Index        map[string]*ManifestItem
}

type ManifestItem struct {
	SeqNo      string `json:"seqNo"`
	Resolution string `json:"resolution"`
}

func NewManifestWriter(eventName string) *ManifestWriter {
	cfg := constants.MustGetConfig()
	return &ManifestWriter{
		ManifestPath: cfg.GetManifestPath(eventName),
		Segments:     make([]ManifestItem, 0),
		Index:        make(map[string]*ManifestItem),
	}
}

func (m *ManifestWriter) AddOrUpdateSegment(seqNo string, resolution string) {
	if m.Index == nil {
		m.Index = make(map[string]*ManifestItem)
	}

	if m.Segments == nil {
		m.Segments = make([]ManifestItem, 0)
	}

	if existing, ok := m.Index[seqNo]; ok {
		if resolution > existing.Resolution {
			existing.Resolution = resolution
		}
		return
	} else {
		item := ManifestItem{
			SeqNo:      seqNo,
			Resolution: resolution,
		}
		m.Segments = append(m.Segments, item)
		m.Index[seqNo] = &item
	}
}

func (m *ManifestWriter) WriteManifest() {
	sort.Slice(m.Segments, func(i, j int) bool {
		return m.Segments[i].SeqNo < m.Segments[j].SeqNo
	})

	data, err := json.MarshalIndent(m.Segments, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal manifest: %v", err)
		return
	}

	if err := utils.ValidateWritablePath(m.ManifestPath); err != nil {
		log.Printf("Manifest path validation failed: %v", err)
		return
	}

	file, err := os.Create(m.ManifestPath)
	if err != nil {
		log.Printf("Failed to create manifest file: %v", err)
		return
	}

	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		log.Printf("Failed to write manifest file: %v", err)
		return
	}
}
