package business

import (
	"testing"
	"time"
)

func TestMessageOutboxModelTableName(t *testing.T) {
	t.Parallel()

	if (MessageOutboxModel{}).TableName() != "cc_biz_outbox" {
		t.Fatalf("unexpected outbox table name")
	}
}

func TestJSONMapValueAndScan(t *testing.T) {
	t.Parallel()

	value, err := JSONMap{"batchTaskId": 10, "status": "completed"}.Value()
	if err != nil {
		t.Fatal(err)
	}
	var decoded JSONMap
	if err := decoded.Scan(value); err != nil {
		t.Fatal(err)
	}
	if decoded["status"] != "completed" {
		t.Fatalf("unexpected decoded payload: %+v", decoded)
	}
}

func TestOutboxModelRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	entry := Entry{
		ID:             "projection:1",
		AggregateType:  "batch_call_task",
		AggregateID:    "10",
		Destination:    "cti_batch_task_projection",
		IdempotencyKey: "projection:1",
		Payload:        map[string]any{"batchTaskId": 10},
		Status:         Pending,
		NextAttemptAt:  now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	model := modelFromEntry(entry)
	roundTrip := entryFromModel(model)
	if roundTrip.ID != entry.ID || roundTrip.Destination != entry.Destination || roundTrip.Payload["batchTaskId"] != 10 {
		t.Fatalf("unexpected round trip: %+v", roundTrip)
	}
}
