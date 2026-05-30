package app

import (
	"time"

	"yunshu/internal/infra/config"
)

func testConfig() config.Config {
	return config.Config{
		FreeSwitch: config.FreeSwitchConfig{
			CommandTimeout: 5 * time.Second,
			Reconnect: config.ReconnectConfig{
				Interval:    5 * time.Second,
				MaxAttempts: 1,
			},
		},
		Worker: config.WorkerConfig{
			Outbox: config.WorkerOutboxConfig{
				Interval:   5 * time.Second,
				BatchSize:  10,
				RetryDelay: time.Minute,
				Lease:      30 * time.Second,
				WorkerID:   "test-worker",
			},
		},
	}
}
