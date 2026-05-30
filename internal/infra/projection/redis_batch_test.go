package projection

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	outbox "yunshu/internal/infra/business"
)

func TestRedisBatchProjectorProjectsTelCompleted(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	projector := NewRedisBatchProjector(client, nil)
	pubsub := client.Subscribe(context.Background(), websocketPushTopic)
	defer pubsub.Close()
	if _, err := pubsub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}

	err := projector.ProjectTelCompleted(context.Background(), outbox.Entry{
		ID:      "outbox-1",
		Payload: map[string]any{"batchTaskId": 10, "batchCallTelId": 20, "callId": "call-1", "connected": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	status := server.HGet("batch:10:tel:20", "status")
	callID := server.HGet("batch:10:tel:20", "callId")
	if status != "completed" || callID != "call-1" {
		t.Fatalf("unexpected projection: status=%s callId=%s", status, callID)
	}
	assertFanoutMessage(t, pubsub, "batch_tel_completed", "batch:10:tel:20")
}

func TestRedisBatchProjectorProjectsTaskCompleted(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	projector := NewRedisBatchProjector(client, nil)
	pubsub := client.Subscribe(context.Background(), websocketPushTopic)
	defer pubsub.Close()
	if _, err := pubsub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}

	err := projector.ProjectTaskCompleted(context.Background(), outbox.Entry{
		ID:      "outbox-2",
		Payload: map[string]any{"batchTaskId": 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	status := server.HGet("batch:10:summary", "status")
	taskID := server.HGet("batch:10:summary", "taskId")
	if status != "completed" || taskID != "10" {
		t.Fatalf("unexpected projection: status=%s taskId=%s", status, taskID)
	}
	assertFanoutMessage(t, pubsub, "batch_task_completed", "batch:10:summary")
}

func assertFanoutMessage(t *testing.T, pubsub *goredis.PubSub, wantType, wantKey string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	message, err := pubsub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(message.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["type"] != wantType || payload["projectionKey"] != wantKey {
		t.Fatalf("unexpected fanout payload: %+v", payload)
	}
}
