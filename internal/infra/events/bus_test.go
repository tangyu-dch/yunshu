package events

import (
	"context"
	"errors"
	"testing"

	"yunshu/internal/contracts"
)

func TestMemoryBusPublishToSubscribers(t *testing.T) {
	t.Parallel()

	bus := NewMemoryBus(nil)
	handled := 0
	bus.Subscribe("call.test", func(context.Context, contracts.EventEnvelope[map[string]any]) error {
		handled++
		return nil
	})
	err := bus.Publish(context.Background(), contracts.NewEventEnvelope("evt-1", "call.test", "idem-1", "call", "call-1", contracts.ServiceCall, map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if handled != 1 {
		t.Fatalf("expected one handler call, got %d", handled)
	}
}

func TestMemoryBusReturnsConsumerError(t *testing.T) {
	t.Parallel()

	bus := NewMemoryBus(nil)
	want := errors.New("消费失败")
	bus.Subscribe("call.test", func(context.Context, contracts.EventEnvelope[map[string]any]) error {
		return want
	})
	err := bus.Publish(context.Background(), contracts.NewEventEnvelope("evt-1", "call.test", "idem-1", "call", "call-1", contracts.ServiceCall, map[string]any{}))
	if !errors.Is(err, want) {
		t.Fatalf("expected consumer error, got %v", err)
	}
}
