package business

import (
	"context"
	"testing"
	"time"
)

func TestReportMemoryStoreSaveFromOutbox(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	store := NewReportMemoryStore()
	err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "cdr:report:call-1",
		AggregateID: "call-1",
		Payload: map[string]any{
			"callId":         "call-1",
			"merchantId":     88,
			"userId":         99,
			"batchTaskId":    10,
			"profile":        "api_outbound",
			"finalState":     "complete",
			"durationSec":    31,
			"completedAt":    completedAt,
			"sourceOutboxId": "cdr:call-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	projection := store.Projections["call-1"]
	if projection.MerchantID != 88 || projection.BatchTaskID != 10 || !projection.CompletedAt.Equal(completedAt) {
		t.Fatalf("unexpected report projection: %+v", projection)
	}
}

func TestReportProjectionModelTableName(t *testing.T) {
	t.Parallel()

	if (ReportProjectionModel{}).TableName() != "cc_biz_report" {
		t.Fatalf("unexpected table name")
	}
}
