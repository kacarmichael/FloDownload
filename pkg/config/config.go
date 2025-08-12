package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Core       CoreConfig
	HTTP       HTTPConfig
	NAS        NASConfig
	Processing ProcessingConfig
	Transfer   TransferConfig
	Cleanup    CleanupConfig
	Paths      PathsConfig
}

type CoreConfig struct {
	WorkerCount  int
	RefreshDelay time.Duration
}

type HTTPConfig struct {
	UserAgent string
	Referer   string
}

type NASConfig struct {
	EnableTransfer bool
	OutputPath     string
	Username       string
	Password       string
	Timeout        time.Duration
	RetryLimit     int
}

type ProcessingConfig struct {
	Enabled     bool
	AutoProcess bool
	WorkerCount int
	FFmpegPath  string
}

type TransferConfig struct {
	WorkerCount       int
	RetryLimit        int
	Timeout           time.Duration
	FileSettlingDelay time.Duration
	QueueSize         int
	BatchSize         int
}

type CleanupConfig struct {
	AfterTransfer bool
	BatchSize     int
	RetainHours   int
}

type PathsConfig struct {
	BaseDir         string
	LocalOutput     string
	ProcessOutput   string
	ManifestDir     string
	PersistenceFile string
}

var defaultConfig = Config{
	Core: CoreConfig{
		WorkerCount:  4,
		RefreshDelay: 3 * time.Second,
	},
	HTTP: HTTPConfig{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
		Referer:   "https://www.flomarching.com",
	},
	NAS: NASConfig{
		EnableTransfer: true,
		OutputPath:     "",
		Username:       "",
		Password:       "",
		Timeout:        30 * time.Second,
		RetryLimit:     3,
	},
	Processing: ProcessingConfig{
		Enabled:     true,
		AutoProcess: true,
		WorkerCount: 2,
		FFmpegPath:  "ffmpeg",
	},
	Transfer: TransferConfig{
		WorkerCount:       2,
		RetryLimit:        3,
		Timeout:           30 * time.Second,
		FileSettlingDelay: 5 * time.Second,
		QueueSize:         100000,
		BatchSize:         1000,
	},
	Cleanup: CleanupConfig{
		AfterTransfer: true,
		BatchSize:     1000,
		RetainHours:   0,
	},
	Paths: PathsConfig{
		BaseDir:         "data",
		LocalOutput:     "data",
		ProcessOutput:   "out",
		ManifestDir:     "data",
		PersistenceFile: "transfer_queue.json",
	},
}

func Load() (*Config, error) {
	cfg := defaultConfig

	if err := cfg.loadFromEnvironment(); err != nil {
		return nil, fmt.Errorf("failed to load environment config: %w", err)
	}

	if err := cfg.resolveAndValidatePaths(); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	return &cfg, nil
}

func (c *Config) loadFromEnvironment() error {
	if val := os.Getenv("WORKER_COUNT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			c.Core.WorkerCount = parsed
		}
	}

	if val := os.Getenv("REFRESH_DELAY_SECONDS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			c.Core.RefreshDelay = time.Duration(parsed) * time.Second
		}
	}

	if val := os.Getenv("NAS_OUTPUT_PATH"); val != "" {
		c.NAS.OutputPath = val
	}

	if val := os.Getenv("NAS_USERNAME"); val != "" {
		c.NAS.Username = val
	}

	if val := os.Getenv("NAS_PASSWORD"); val != "" {
		c.NAS.Password = val
	}

	if val := os.Getenv("ENABLE_NAS_TRANSFER"); val != "" {
		c.NAS.EnableTransfer = val == "true"
	}

	if val := os.Getenv("LOCAL_OUTPUT_DIR"); val != "" {
		c.Paths.LocalOutput = val
	}

	if val := os.Getenv("PROCESS_OUTPUT_DIR"); val != "" {
		c.Paths.ProcessOutput = val
	}

	if val := os.Getenv("FFMPEG_PATH"); val != "" {
		c.Processing.FFmpegPath = val
	}

	return nil
}

func (c *Config) resolveAndValidatePaths() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Only join with cwd if path is not already absolute
	if !filepath.IsAbs(c.Paths.BaseDir) {
		c.Paths.BaseDir = filepath.Join(cwd, c.Paths.BaseDir)
	}
	if !filepath.IsAbs(c.Paths.LocalOutput) {
		c.Paths.LocalOutput = filepath.Join(cwd, c.Paths.LocalOutput)
	}
	if !filepath.IsAbs(c.Paths.ProcessOutput) {
		c.Paths.ProcessOutput = filepath.Join(cwd, c.Paths.ProcessOutput)
	}
	if !filepath.IsAbs(c.Paths.ManifestDir) {
		c.Paths.ManifestDir = filepath.Join(cwd, c.Paths.ManifestDir)
	}
	if !filepath.IsAbs(c.Paths.PersistenceFile) {
		c.Paths.PersistenceFile = filepath.Join(c.Paths.BaseDir, c.Paths.PersistenceFile)
	}

	requiredDirs := []string{
		c.Paths.BaseDir,
		c.Paths.LocalOutput,
		c.Paths.ProcessOutput,
		c.Paths.ManifestDir,
	}

	for _, dir := range requiredDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if c.NAS.EnableTransfer && c.NAS.OutputPath == "" {
		return fmt.Errorf("NAS output path is required when transfer is enabled")
	}

	if c.Processing.Enabled && c.Processing.FFmpegPath == "" {
		return fmt.Errorf("FFmpeg path is required when processing is enabled")
	}

	return nil
}

func (c *Config) GetEventPath(eventName string) string {
	return filepath.Join(c.Paths.LocalOutput, eventName)
}

func (c *Config) GetManifestPath(eventName string) string {
	return filepath.Join(c.Paths.ManifestDir, eventName+".json")
}

func (c *Config) GetNASEventPath(eventName string) string {
	return filepath.Join(c.NAS.OutputPath, eventName)
}

func (c *Config) GetProcessOutputPath(eventName string) string {
	return filepath.Join(c.Paths.ProcessOutput, eventName)
}

func (c *Config) GetQualityPath(eventName, quality string) string {
	return filepath.Join(c.GetEventPath(eventName), quality)
}
