package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaultConfig(t *testing.T) {
	t.Setenv("ADDR", ":9999")
	t.Setenv("WORKER_BILLING_DEFAULT_RATE_PER_MIN", "0.18")

	cfg, err := Load("../../../configs/default.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Service.Addr != ":9999" {
		t.Fatalf("env override failed: %s", cfg.Service.Addr)
	}
	if cfg.FreeSwitch.EventLeaseTTL != 30*time.Second {
		t.Fatalf("unexpected lease ttl: %s", cfg.FreeSwitch.EventLeaseTTL)
	}
	if cfg.Worker.Outbox.Interval != 5*time.Second || cfg.Worker.Outbox.BatchSize != 100 || cfg.Worker.Outbox.RetryDelay != time.Minute || cfg.Worker.Outbox.Lease != 30*time.Second {
		t.Fatalf("unexpected worker outbox config: %+v", cfg.Worker.Outbox)
	}
	if cfg.Worker.Outbox.WorkerID != "cc-worker-local" {
		t.Fatalf("unexpected worker id: %s", cfg.Worker.Outbox.WorkerID)
	}
	if cfg.Worker.Callback.Timeout != 5*time.Second {
		t.Fatalf("unexpected callback config: %+v", cfg.Worker.Callback)
	}
	if cfg.Worker.Downstream.Timeout != 5*time.Second {
		t.Fatalf("unexpected downstream config: %+v", cfg.Worker.Downstream)
	}
	if cfg.Worker.Recording.Timeout != 5*time.Second {
		t.Fatalf("unexpected recording config: %+v", cfg.Worker.Recording)
	}
	if cfg.Worker.Billing.DefaultRatePerMin != 0.18 {
		t.Fatalf("unexpected billing config: %+v", cfg.Worker.Billing)
	}
}

func TestLoadMissingConfig(t *testing.T) {
	t.Parallel()

	_, err := Load("missing.yaml")
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected not exist error, got %v", err)
	}
}
