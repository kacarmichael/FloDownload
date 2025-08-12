package processing

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"m3u8-downloader/pkg/config"
	"m3u8-downloader/pkg/nas"
	"m3u8-downloader/pkg/utils"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type ProcessingService struct {
	config    *config.Config
	eventName string
	nas       *nas.NASService
}

func NewProcessingService(eventName string, cfg *config.Config) (*ProcessingService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	nasConfig := nas.NASConfig{
		Path:       cfg.NAS.OutputPath,
		Username:   cfg.NAS.Username,
		Password:   cfg.NAS.Password,
		Timeout:    cfg.NAS.Timeout,
		RetryLimit: cfg.NAS.RetryLimit,
		VerifySize: true,
	}

	nasService := nas.NewNASService(nasConfig)

	if err := nasService.TestConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to NAS: %w", err)
	}

	return &ProcessingService{
		config:    cfg,
		eventName: eventName,
		nas:       nasService,
	}, nil
}

func (ps *ProcessingService) GetEventDirs() ([]string, error) {
	if ps.eventName == "" {
		sourcePath := ps.config.NAS.OutputPath
		dirs, err := os.ReadDir(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", sourcePath, err)
		}
		var eventDirs []string
		for _, dir := range dirs {
			if dir.IsDir() {
				eventDirs = append(eventDirs, dir.Name())
			}
		}
		return eventDirs, nil
	} else {
		return []string{ps.eventName}, nil
	}
}

func (ps *ProcessingService) Start(ctx context.Context) error {
	if !ps.config.Processing.Enabled {
		log.Println("Processing service disabled")
		return nil
	}

	if ps.eventName == "" {
		events, err := ps.GetEventDirs()
		if err != nil {
			return fmt.Errorf("failed to get event directories: %w", err)
		}
		if len(events) == 0 {
			return fmt.Errorf("no events found")
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
				return fmt.Errorf("failed to parse input: %w", err)
			}
			if index < 1 || index > len(events) {
				return fmt.Errorf("invalid input")
			}
			ps.eventName = events[index-1]
		} else {
			ps.eventName = events[0]
		}
	}

	//Get all present resolutions
	dirs, err := ps.GetResolutions()
	if err != nil {
		return fmt.Errorf("Failed to get resolutions: %w", err)
	}

	//Spawn a worker per resolution
	ch := make(chan SegmentInfo, 100)
	var wg sync.WaitGroup

	for _, resolution := range dirs {
		wg.Add(1)
		go ps.ParseResolutionDirectory(resolution, ch, &wg)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	segments, err := ps.AggregateSegmentInfo(ch)
	if err != nil {
		return fmt.Errorf("Failed to aggregate segment info: %w", err)
	}

	aggFile, err := ps.WriteConcatFile(segments)
	if err != nil {
		return fmt.Errorf("Failed to write concat file: %w", err)
	}

	// Feed info to ffmpeg to stitch files together
	outPath := ps.config.GetProcessOutputPath(ps.eventName)
	if err := utils.EnsureDir(outPath); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	concatErr := ps.RunFFmpeg(aggFile, outPath)
	if concatErr != nil {
		return concatErr
	}

	return nil
}

func (ps *ProcessingService) GetResolutions() ([]string, error) {
	eventPath := ps.config.GetNASEventPath(ps.eventName)
	dirs, err := os.ReadDir(eventPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory %s: %w", eventPath, err)
	}

	re := regexp.MustCompile(`^\d+p$`)

	var resolutions []string
	for _, dir := range dirs {
		if dir.IsDir() && re.MatchString(dir.Name()) {
			resolutions = append(resolutions, dir.Name())
		}
	}

	return resolutions, nil
}

func (ps *ProcessingService) ParseResolutionDirectory(resolution string, ch chan<- SegmentInfo, wg *sync.WaitGroup) {
	defer wg.Done()

	resolutionPath := utils.SafeJoin(ps.config.GetNASEventPath(ps.eventName), resolution)
	files, err := os.ReadDir(resolutionPath)
	if err != nil {
		log.Printf("Failed to read resolution directory %s: %v", resolutionPath, err)
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			if !strings.HasSuffix(strings.ToLower(file.Name()), ".ts") {
				continue
			}
			no, err := strconv.Atoi(file.Name()[6:10])
			if err != nil {
				log.Printf("Failed to parse segment number: %v", err)
				continue
			}
			ch <- SegmentInfo{
				Name:       file.Name(),
				SeqNo:      no,
				Resolution: resolution,
			}
		}
	}
}

func (ps *ProcessingService) AggregateSegmentInfo(ch <-chan SegmentInfo) (map[int]SegmentInfo, error) {
	segmentMap := make(map[int]SegmentInfo)

	rank := map[string]int{
		"1080p": 1,
		"720p":  2,
		"540p":  3,
		"480p":  4,
		"450p":  5,
		"360p":  6,
		"270p":  7,
		"240p":  8,
	}

	for segment := range ch {
		fmt.Printf("Received segment %s in resolution %s \n", segment.Name, segment.Resolution)
		current, exists := segmentMap[segment.SeqNo]
		if !exists || rank[segment.Resolution] < rank[current.Resolution] {
			segmentMap[segment.SeqNo] = segment
		}
	}

	return segmentMap, nil
}

func (ps *ProcessingService) WriteConcatFile(segmentMap map[int]SegmentInfo) (string, error) {
	concatPath := ps.config.GetProcessOutputPath(ps.eventName)

	if err := utils.EnsureDir(concatPath); err != nil {
		return "", fmt.Errorf("failed to create directories for concat path: %w", err)
	}

	concatFilePath := utils.SafeJoin(concatPath, ps.eventName+".txt")
	f, err := os.Create(concatFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create concat file: %w", err)
	}
	defer f.Close()

	// Sort keys to preserve order
	keys := make([]int, 0, len(segmentMap))
	for k := range segmentMap {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	for _, seq := range keys {
		segment := segmentMap[seq]
		filePath := utils.SafeJoin(ps.config.GetNASEventPath(ps.eventName), segment.Resolution, segment.Name)
		line := fmt.Sprintf("file '%s'\n", filePath)
		if _, err := f.WriteString(line); err != nil {
			return "", fmt.Errorf("failed to write to concat file: %w", err)
		}
	}

	return concatFilePath, nil
}

func (ps *ProcessingService) getFFmpegPath() (string, error) {
	// First try the configured path
	configuredPath := ps.config.Processing.FFmpegPath
	if configuredPath != "" {
		// Check if it's just the command name or a full path
		if filepath.IsAbs(configuredPath) {
			return configuredPath, nil
		}

		// Try to find it in PATH
		if fullPath, err := exec.LookPath(configuredPath); err == nil {
			return fullPath, nil
		}
	}

	// Fallback: try local bin directory
	var baseDir string
	exePath, err := os.Executable()
	if err == nil {
		baseDir = filepath.Dir(exePath)
	} else {
		baseDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	ffmpeg := utils.SafeJoin(baseDir, "bin", "ffmpeg")
	if runtime.GOOS == "windows" {
		ffmpeg += ".exe"
	}

	if utils.PathExists(ffmpeg) {
		return ffmpeg, nil
	}

	// Try current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	ffmpeg = utils.SafeJoin(cwd, "bin", "ffmpeg")
	if runtime.GOOS == "windows" {
		ffmpeg += ".exe"
	}

	if utils.PathExists(ffmpeg) {
		return ffmpeg, nil
	}

	return "", fmt.Errorf("FFmpeg not found. Please install FFmpeg or set FFMPEG_PATH environment variable")
}

func (ps *ProcessingService) RunFFmpeg(inputPath, outputPath string) error {
	fmt.Println("Running ffmpeg...")

	fileOutPath := utils.SafeJoin(outputPath, ps.eventName+".mp4")
	fmt.Println("Input path:", inputPath)
	fmt.Println("Output path:", fileOutPath)

	path, err := ps.getFFmpegPath()
	if err != nil {
		return fmt.Errorf("failed to find FFmpeg: %w", err)
	}

	cmd := exec.Command(path, "-f", "concat", "-safe", "0", "-i", inputPath, "-c", "copy", fileOutPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run ffmpeg: %w", err)
	}

	fmt.Println("FFmpeg completed successfully")
	return nil
}
