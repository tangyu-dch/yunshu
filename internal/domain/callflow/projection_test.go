package callflow

import (
	"context"
	"testing"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/business"
	"yunshu/internal/infra/events"
)

func TestProjectionConsumersAppendOutboxEntries(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	store := business.NewOutboxMemoryStore()
	RegisterProjectionConsumers(bus, store, nil)

	ctx := context.Background()
	if err := bus.Publish(ctx, contracts.NewEventEnvelope(
		"evt-tel-completed",
		contracts.EventBatchCallTelCompleted,
		"idem-tel-completed",
		"batch_call_tel",
		"10:20",
		contracts.ServiceCall,
		map[string]any{"batchTaskId": 10, "batchCallTelId": 20},
	)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope(
		"evt-task-completed",
		contracts.EventBatchCallTaskCompleted,
		"idem-task-completed",
		"batch_call_task",
		"10",
		contracts.ServiceCall,
		map[string]any{"batchTaskId": 10},
	)); err != nil {
		t.Fatal(err)
	}

	entries, err := store.Pending(ctx, 10, contracts.NewEventEnvelope("now", "test", "idem", "test", "test", contracts.ServiceCall, map[string]any{}).OccurredAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected projection and callback entries, got %d", len(entries))
	}
	got := map[string]int{}
	for _, entry := range entries {
		got[entry.Destination]++
	}
	if got[DestinationBatchTelProjection] != 1 || got[DestinationBatchTaskProjection] != 1 || got[DestinationBatchCallback] != 2 {
		t.Fatalf("unexpected destinations: %+v", entries)
	}
}
