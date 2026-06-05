package app

import (
	"testing"
	"time"

	"yunshu/internal/infra/config"
)

func newTestWorkerRuntime(t *testing.T, cfg config.Config) *WorkerRuntime {
	t.Helper()
	w, err := NewWorkerRuntimeWithConfig(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

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
