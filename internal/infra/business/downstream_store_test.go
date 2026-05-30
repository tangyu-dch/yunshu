package business

import (
	"context"
	"testing"
	"time"
)

func TestPushMemoryDownstreamStoreSaveFromOutbox(t *testing.T) {
	t.Parallel()

	store := NewPushMemoryStore()
	job, err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "cdr:downstream:call-1",
		AggregateID: "call-1",
		Payload: map[string]any{
			"callId":           "call-1",
			"merchantId":       88,
			"downstreamTarget": "ods",
			"sourceOutboxId":   "cdr:call-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.MerchantID != 88 || job.Target != "ods" || job.Status != StatusPending {
		t.Fatalf("unexpected downstream job: %+v", job)
	}
}

func TestPushMemoryDownstreamStoreMarksDeliveryState(t *testing.T) {
	t.Parallel()

	store := NewPushMemoryStore()
	job, err := store.SaveFromOutbox(context.Background(), Entry{AggregateID: "call-1", Payload: map[string]any{"callId": "call-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkFailed(context.Background(), job.ID, "timeout"); err != nil {
		t.Fatal(err)
	}
	if store.PushJobs["call-1"].Status != StatusFailed || store.PushJobs["call-1"].Attempts != 1 {
		t.Fatalf("expected failed job: %+v", store.PushJobs["call-1"])
	}
	if err := store.MarkDelivered(context.Background(), job.ID, job.CreatedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if store.PushJobs["call-1"].Status != StatusDelivered || store.PushJobs["call-1"].DeliveredAt.IsZero() {
		t.Fatalf("expected delivered job: %+v", store.PushJobs["call-1"])
	}
}

func TestPushPushJobModelTableName(t *testing.T) {
	t.Parallel()

	if (PushJobModel{}).TableName() != "cc_biz_push" {
		t.Fatalf("unexpected table name")
	}
}
