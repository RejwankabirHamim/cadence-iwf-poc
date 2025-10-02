package common

import "time"

const (
	RetryInterval = 15 * time.Second
	RetryTimeout  = 1 * time.Hour
	pullInterval  = 5 * time.Second
	waitTimeout   = 10 * time.Minute
)
