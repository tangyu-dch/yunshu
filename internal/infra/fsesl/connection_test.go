package fsesl

import (
	"context"
	"testing"
	"time"
)

func TestConnectionPoolDynamicNodeStatus(t *testing.T) {
	t.Parallel()

	pool := NewConnectionPool(context.Background(), nil, 0, 0, nil)
	pool.UpsertNode(NodeConfig{ID: 1, Addr: "10.0.0.1:8021", SetID: 2, Weight: 80, Enabled: true})

	statuses := pool.Status()
	if len(statuses) != 1 {
		t.Fatalf("expected one status, got %d", len(statuses))
	}
	if statuses[0].FSAddr != "10.0.0.1:8021" || statuses[0].Connected {
		t.Fatalf("unexpected status: %+v", statuses[0])
	}

	pool.RemoveNode("10.0.0.1:8021")
	if got := len(pool.Status()); got != 0 {
		t.Fatalf("expected empty status after remove, got %d", got)
	}
}

func TestConnectionPoolLeaseRenewInterval(t *testing.T) {
	t.Parallel()

	pool := NewConnectionPool(context.Background(), nil, 0, 0, nil)
	pool.LeaseTTL = 30 * time.Second
	if got := pool.leaseRenewInterval(); got != 15*time.Second {
		t.Fatalf("unexpected renew interval: %s", got)
	}
	pool.LeaseTTL = time.Second
	if got := pool.leaseRenewInterval(); got != time.Second {
		t.Fatalf("unexpected short renew interval: %s", got)
	}
}
