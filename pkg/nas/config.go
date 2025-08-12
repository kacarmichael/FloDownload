package nas

import "time"

type NASConfig struct {
	Path       string
	Username   string
	Password   string
	Timeout    time.Duration
	RetryLimit int
	VerifySize bool
}
