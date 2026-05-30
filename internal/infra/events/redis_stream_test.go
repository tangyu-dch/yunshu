package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
)

type fakeRedisStreamClient struct {
	addedStream string
	addedValues map[string]any
	ackedIDs    []string
}

func (f *fakeRedisStreamClient) XAdd(_ context.Context, a *goredis.XAddArgs) *goredis.StringCmd {
	f.addedStream = a.Stream
	values, _ := a.Values.(map[string]any)
	f.addedValues = values
	return goredis.NewStringResult("1-0", nil)
}

func (f *fakeRedisStreamClient) XGroupCreateMkStream(context.Context, string, string, string) *goredis.StatusCmd {
	return goredis.NewStatusResult("OK", nil)
}

func (f *fakeRedisStreamClient) XReadGroup(context.Context, *goredis.XReadGroupArgs) *goredis.XStreamSliceCmd {
	return goredis.NewXStreamSliceCmd(context.Background())
}

func (f *fakeRedisStreamClient) XAck(_ context.Context, _ string, _ string, ids ...string) *goredis.IntCmd {
	f.ackedIDs = append(f.ackedIDs, ids...)
	return goredis.NewIntResult(int64(len(ids)), nil)
}

func TestRedisStreamBusPublish(t *testing.T) {
	t.Parallel()

	client := &fakeRedisStreamClient{}
	bus := NewRedisStreamBus(client, RedisStreamConfig{Stream: "yunshu:test", Group: "group", Consumer: "consumer"}, nil)
	event := contracts.NewEventEnvelope("evt-1", contracts.EventAPICallRequested, "idem-1", "call", "call-1", contracts.ServiceCall, map[string]any{"callId": "call-1"})

	if err := bus.Publish(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if client.addedStream != "yunshu:test" {
		t.Fatalf("stream got %s", client.addedStream)
	}
	raw, ok := client.addedValues[redisStreamPayloadField].(string)
	if !ok || raw == "" {
		t.Fatalf("payload missing: %+v", client.addedValues)
	}
	var decoded contracts.EventEnvelope[map[string]any]
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.EventType != contracts.EventAPICallRequested {
		t.Fatalf("event type got %s", decoded.EventType)
	}
}

func TestRedisStreamBusHandleMessage(t *testing.T) {
	t.Parallel()

	client := &fakeRedisStreamClient{}
	bus := NewRedisStreamBus(client, RedisStreamConfig{Stream: "yunshu:test", Group: "group", Consumer: "consumer", Block: time.Millisecond}, nil)
	handled := 0
	bus.Subscribe(contracts.EventAPICallRequested, func(context.Context, contracts.EventEnvelope[map[string]any]) error {
		handled++
		return nil
	})
	event := contracts.NewEventEnvelope("evt-1", contracts.EventAPICallRequested, "idem-1", "call", "call-1", contracts.ServiceCall, map[string]any{"callId": "call-1"})
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	err = bus.handleMessage(context.Background(), goredis.XMessage{ID: "1-0", Values: map[string]any{redisStreamPayloadField: string(raw)}})
	if err != nil {
		t.Fatal(err)
	}
	if handled != 1 {
		t.Fatalf("handled got %d", handled)
	}
}
