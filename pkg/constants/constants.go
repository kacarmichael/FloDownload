package constants

import (
	"m3u8-downloader/pkg/config"
	"sync"
)

var (
	globalConfig *config.Config
	configOnce   sync.Once
	configError  error
)

func GetConfig() (*config.Config, error) {
	configOnce.Do(func() {
		globalConfig, configError = config.Load()
	})
	return globalConfig, configError
}

func MustGetConfig() *config.Config {
	cfg, err := GetConfig()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}
	return cfg
}

const (
	WorkerCount  = 4
	RefreshDelay = 3

	HTTPUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"
	REFERRER      = "https://www.flomarching.com"

	DefaultNASOutputPath = ""
	DefaultNASUsername   = ""

	DefaultTransferWorkerCount = 2
	DefaultTransferRetryLimit  = 3
	DefaultTransferTimeout     = 30
	DefaultFileSettlingDelay   = 5
	DefaultTransferQueueSize   = 100000
	DefaultBatchSize           = 1000

	DefaultCleanupBatchSize = 1000
	DefaultRetainLocalHours = 0

	DefaultProcessWorkerCount = 2
	DefaultFFmpegPath         = "ffmpeg"
)
