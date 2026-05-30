package business

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOutboxMemoryStoreAppendAndPending(t *testing.T) {
	t.Parallel()

	store := NewOutboxMemoryStore()
	ctx := context.Background()
	entry := Entry{ID: "1", AggregateType: "call", AggregateID: "call-1", Destination: "queue", IdempotencyKey: "cmd-1"}

	if err := store.Append(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, entry); !errors.Is(err, ErrDuplicateEntry) {
		t.Fatalf("expected duplicate error, got %v", err)
	}

	pending, err := store.Pending(ctx, 10, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending got %d", len(pending))
	}

	if err := store.MarkPublished(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	pending, err = store.Pending(ctx, 10, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending entries, got %d", len(pending))
	}
}

func TestOutboxMemoryStoreFailedRetry(t *testing.T) {
	t.Parallel()

	store := NewOutboxMemoryStore()
	ctx := context.Background()
	if err := store.Append(ctx, Entry{ID: "1"}); err != nil {
		t.Fatal(err)
	}
	retryAt := time.Now().Add(time.Minute)
	if err := store.MarkFailed(ctx, "1", retryAt); err != nil {
		t.Fatal(err)
	}

	pending, err := store.Pending(ctx, 10, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("entry should wait for retry time")
	}

	pending, err = store.Pending(ctx, 10, retryAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("entry should be retryable")
	}
}

func TestOutboxMemoryStoreClaimDueUsesLease(t *testing.T) {
	t.Parallel()

	store := NewOutboxMemoryStore()
	ctx := context.Background()
	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	if err := store.Append(ctx, Entry{ID: "1", Destination: "projection"}); err != nil {
		t.Fatal(err)
	}

	claimed, err := store.ClaimDue(ctx, "worker-a", 10, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].Status != Processing || claimed[0].LockedBy != "worker-a" {
		t.Fatalf("unexpected claimed entries: %+v", claimed)
	}

	claimed, err = store.ClaimDue(ctx, "worker-b", 10, now.Add(30*time.Second), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatalf("lease should prevent duplicate claim: %+v", claimed)
	}

	claimed, err = store.ClaimDue(ctx, "worker-b", 10, now.Add(2*time.Minute), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].LockedBy != "worker-b" {
		t.Fatalf("expired lease should be claimable: %+v", claimed)
	}
}
