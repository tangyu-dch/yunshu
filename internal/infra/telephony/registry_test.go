package telephony

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryRegistryClaimsLease(t *testing.T) {
	t.Parallel()

	registry := NewMemoryRegistry()
	ctx := context.Background()
	err := registry.Upsert(ctx, Node{FSAddr: "fs-1", Status: NodeActive, Enable: true, MaxChannels: 100})
	if err != nil {
		t.Fatal(err)
	}

	node, err := registry.ClaimEvents(ctx, "fs-1", "esl-a", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if node.EventOwner != "esl-a" {
		t.Fatalf("unexpected owner: %s", node.EventOwner)
	}

	_, err = registry.ClaimEvents(ctx, "fs-1", "esl-b", time.Minute)
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("expected lease held error, got %v", err)
	}

	if err := registry.ReleaseEvents(ctx, "fs-1", "esl-a"); err != nil {
		t.Fatal(err)
	}
	node, err = registry.ClaimEvents(ctx, "fs-1", "esl-b", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if node.EventOwner != "esl-b" {
		t.Fatalf("unexpected owner after release: %s", node.EventOwner)
	}
}
