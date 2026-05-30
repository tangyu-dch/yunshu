package business

import (
	"context"
	"testing"
	"time"
)

func TestRecordingMemoryRecordingStoreSaveFromOutbox(t *testing.T) {
	t.Parallel()

	store := NewRecordingMemoryStore()
	job, err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "cdr:recording:call-1",
		AggregateID: "call-1",
		Payload: map[string]any{
			"callId":         "call-1",
			"merchantId":     88,
			"recordFilePath": "/record/call-1.wav",
			"sourceOutboxId": "cdr:call-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.MerchantID != 88 || job.RecordFile != "/record/call-1.wav" || job.Status != StatusPending {
		t.Fatalf("unexpected recording job: %+v", job)
	}
}

func TestRecordingMemoryRecordingStoreSkipsMissingRecordFile(t *testing.T) {
	t.Parallel()

	store := NewRecordingMemoryStore()
	if _, err := store.SaveFromOutbox(context.Background(), Entry{AggregateID: "call-1", Payload: map[string]any{"callId": "call-1"}}); err != nil {
		t.Fatal(err)
	}
	if store.RecordingJobs["call-1"].Status != StatusSkipped {
		t.Fatalf("expected skipped job: %+v", store.RecordingJobs["call-1"])
	}
}

func TestRecordingMemoryRecordingStoreMarksUploadState(t *testing.T) {
	t.Parallel()

	store := NewRecordingMemoryStore()
	job, err := store.SaveFromOutbox(context.Background(), Entry{AggregateID: "call-1", Payload: map[string]any{"callId": "call-1", "recordFilePath": "/record/call-1.wav"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkFailed(context.Background(), job.ID, "upload timeout"); err != nil {
		t.Fatal(err)
	}
	if store.RecordingJobs["call-1"].Status != StatusFailed || store.RecordingJobs["call-1"].Attempts != 1 {
		t.Fatalf("expected failed job: %+v", store.RecordingJobs["call-1"])
	}
	if err := store.MarkUploaded(context.Background(), job.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if store.RecordingJobs["call-1"].Status != StatusUploaded || store.RecordingJobs["call-1"].UploadedAt.IsZero() {
		t.Fatalf("expected uploaded job: %+v", store.RecordingJobs["call-1"])
	}
}

func TestRecordingRecordingJobModelTableName(t *testing.T) {
	t.Parallel()

	if (RecordingJobModel{}).TableName() != "cc_biz_recording" {
		t.Fatalf("unexpected table name")
	}
}
