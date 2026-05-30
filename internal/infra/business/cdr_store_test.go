package business

import (
	"context"
	"testing"
	"time"
)

func TestCdrMemoryStoreSaveFromOutbox(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 5, 23, 11, 0, 0, 0, time.UTC)
	store := NewCdrMemoryStore()
	err := store.SaveFromOutbox(context.Background(), Entry{
		ID: "cdr:call-1",
		Payload: map[string]any{
			"callId":         "call-1",
			"uuid":           "uuid-1",
			"fsAddr":         "10.0.0.1:8021",
			"profile":        "api_outbound",
			"merchantId":     88,
			"userId":         99,
			"batchTaskId":    10,
			"finalState":     "complete",
			"recordFilePath": "/record/call-1.wav",
			"completedAt":    completedAt,
			"eventId":        "evt-1",
			"eventVersion":   1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	record := store.Records["call-1"]
	if record.UUID != "uuid-1" || record.MerchantID != 88 || record.BatchTaskID != 10 || record.RecordFile != "/record/call-1.wav" || !record.CompletedAt.Equal(completedAt) || record.OutboxID != "cdr:call-1" {
		t.Fatalf("unexpected cdr record: %+v", record)
	}
}

func TestRecordModelTableName(t *testing.T) {
	t.Parallel()

	if (RecordModel{}).TableName() != "cc_biz_cdr" {
		t.Fatalf("unexpected table name")
	}
}
