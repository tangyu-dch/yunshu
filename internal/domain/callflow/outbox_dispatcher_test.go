package callflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"yunshu/internal/domain/outbox"
	"yunshu/internal/infra/business"
)

func TestOutboxDispatcherMarksPublishedOnSuccess(t *testing.T) {
	t.Parallel()

	store := business.NewOutboxMemoryStore()
	ctx := context.Background()
	if err := store.Append(ctx, outbox.Entry{ID: "1", Destination: "projection", NextAttemptAt: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	called := false
	dispatcher := &OutboxDispatcher{
		Store: store,
		Handlers: map[string]OutboxHandler{
			"projection": func(context.Context, outbox.Entry) error {
				called = true
				return nil
			},
		},
	}

	count, err := dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !called {
		t.Fatalf("expected one dispatched entry, count=%d called=%v", count, called)
	}
	pending, err := store.Pending(ctx, 10, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending entries, got %d", len(pending))
	}
}

func TestOutboxDispatcherMarksFailedOnHandlerError(t *testing.T) {
	t.Parallel()

	store := business.NewOutboxMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.Append(ctx, outbox.Entry{ID: "1", Destination: "projection", NextAttemptAt: now.Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	dispatcher := &OutboxDispatcher{
		Store:      store,
		RetryDelay: 5 * time.Second,
		Now:        func() time.Time { return now },
		Handlers: map[string]OutboxHandler{
			"projection": func(context.Context, outbox.Entry) error {
				return errors.New("downstream failed")
			},
		},
	}

	count, err := dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no successful dispatch, got %d", count)
	}
	pending, err := store.Pending(ctx, 10, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("entry should wait for retry (5s backoff, not available at 2s)")
	}
	pending, err = store.Pending(ctx, 10, now.Add(6*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Attempts != 1 {
		t.Fatalf("expected retryable failed entry, got %+v", pending)
	}
}
