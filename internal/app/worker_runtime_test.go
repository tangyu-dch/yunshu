package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yunshu/internal/domain/callflow"
	billinginfra "yunshu/internal/infra/business"
	downstreaminfra "yunshu/internal/infra/business"
	outbox "yunshu/internal/infra/business"
	recordinginfra "yunshu/internal/infra/business"
	reportinginfra "yunshu/internal/infra/business"
	settlementinfra "yunshu/internal/infra/business"
	"yunshu/internal/infra/config"
)

func TestWorkerRuntimeDispatchesProjectionOutbox(t *testing.T) {
	t.Parallel()

	runtime := NewWorkerRuntimeWithConfig(config.Config{}, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "projection:1",
		AggregateType:  "batch_call_task",
		AggregateID:    "10",
		Destination:    callflow.DestinationBatchTaskProjection,
		IdempotencyKey: "projection:1",
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("expected one dispatched entry, got %d", dispatched)
	}
	pending, err := runtime.Outbox.Pending(ctx, 10, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected projection entry published, pending=%d", len(pending))
	}
}

func TestWorkerRuntimeDispatchesBillingOutbox(t *testing.T) {
	t.Parallel()

	runtime := NewWorkerRuntimeWithConfig(config.Config{}, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:billing:call-1",
		AggregateType:  "call_cdr_record",
		AggregateID:    "call-1",
		Destination:    callflow.DestinationCDRBilling,
		IdempotencyKey: "cdr:billing:call-1",
		Payload:        map[string]any{"callId": "call-1", "merchantId": 88, "durationSec": 31},
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("expected billing dispatched, got %d", dispatched)
	}
	store, ok := runtime.Billing.(*billinginfra.BillingLedgerMemoryStore)
	if !ok {
		t.Fatalf("expected memory billing store")
	}
	if store.Ledgers["call-1"].MerchantID != 88 {
		t.Fatalf("expected billing ledger, got %+v", store.Ledgers)
	}
}

func TestWorkerRuntimeDispatchesBillingSettlementOutbox(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Worker.Billing.DefaultRatePerMin = 0.5
	runtime := NewWorkerRuntimeWithConfig(cfg, nil)
	settlementStore := runtime.Settlement.(*settlementinfra.SettlementMemoryStore)
	settlementStore.Balance[88] = 100
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:billing:call-1",
		AggregateType:  "call_cdr_record",
		AggregateID:    "call-1",
		Destination:    callflow.DestinationCDRBilling,
		IdempotencyKey: "cdr:billing:call-1",
		Payload:        map[string]any{"callId": "call-1", "merchantId": 88, "durationSec": 31},
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("expected billing dispatched, got %d", dispatched)
	}
	pending, err := runtime.Outbox.Pending(ctx, 10, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	foundSettlement := false
	for _, entry := range pending {
		if entry.Destination == callflow.DestinationBillingSettlement {
			foundSettlement = true
		}
	}
	if !foundSettlement {
		t.Fatalf("expected settlement outbox, got %+v", pending)
	}
	dispatched, err = runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("expected settlement dispatched, got %d", dispatched)
	}
	if settlementStore.Balance[88] != 99.5 {
		t.Fatalf("expected balance deducted, got %+v", settlementStore.Balance)
	}
	job := settlementStore.SettlementJobs["call-1"]
	if job.Status != settlementinfra.StatusSettled {
		t.Fatalf("expected settled job, got %+v", job)
	}
}

func TestWorkerRuntimeDispatchesRecordingOutbox(t *testing.T) {
	t.Parallel()

	runtime := NewWorkerRuntimeWithConfig(config.Config{}, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:recording:call-1",
		AggregateType:  "call_cdr_record",
		AggregateID:    "call-1",
		Destination:    callflow.DestinationCDRRecording,
		IdempotencyKey: "cdr:recording:call-1",
		Payload:        map[string]any{"callId": "call-1", "merchantId": 88, "recordFilePath": "/record/call-1.wav"},
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("expected recording dispatched, got %d", dispatched)
	}
	store, ok := runtime.Recording.(*recordinginfra.RecordingMemoryStore)
	if !ok {
		t.Fatalf("expected memory recording store")
	}
	if store.RecordingJobs["call-1"].RecordFile != "/record/call-1.wav" {
		t.Fatalf("expected recording job, got %+v", store.RecordingJobs)
	}
}

func TestWorkerRuntimeUploadsRecordingHTTP(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("X-Outbox-Id") != "cdr:recording:call-1" {
			t.Fatalf("unexpected outbox id header: %s", r.Header.Get("X-Outbox-Id"))
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Worker.Recording.URL = server.URL
	runtime := NewWorkerRuntimeWithConfig(cfg, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:recording:call-1",
		AggregateType:  "call_cdr_record",
		AggregateID:    "call-1",
		Destination:    callflow.DestinationCDRRecording,
		IdempotencyKey: "cdr:recording:call-1",
		Payload:        map[string]any{"callId": "call-1", "merchantId": 88, "recordFilePath": "/record/call-1.wav"},
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 || !called {
		t.Fatalf("expected recording upload, dispatched=%d called=%v", dispatched, called)
	}
	store := runtime.Recording.(*recordinginfra.RecordingMemoryStore)
	if store.RecordingJobs["call-1"].Status != recordinginfra.StatusUploaded {
		t.Fatalf("expected uploaded recording job, got %+v", store.RecordingJobs["call-1"])
	}
}

func TestWorkerRuntimeDispatchesReportProjectionOutbox(t *testing.T) {
	t.Parallel()

	runtime := NewWorkerRuntimeWithConfig(config.Config{}, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:report:call-1",
		AggregateType:  "call_cdr_record",
		AggregateID:    "call-1",
		Destination:    callflow.DestinationCDRReportProjection,
		IdempotencyKey: "cdr:report:call-1",
		Payload:        map[string]any{"callId": "call-1", "merchantId": 88, "finalState": "complete"},
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("expected report projection dispatched, got %d", dispatched)
	}
	store, ok := runtime.Reporting.(*reportinginfra.ReportMemoryStore)
	if !ok {
		t.Fatalf("expected memory reporting store")
	}
	if store.Projections["call-1"].FinalState != "complete" {
		t.Fatalf("expected report projection, got %+v", store.Projections)
	}
}

func TestWorkerRuntimeDispatchesDownstreamOutbox(t *testing.T) {
	t.Parallel()

	runtime := NewWorkerRuntimeWithConfig(config.Config{}, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:downstream:call-1",
		AggregateType:  "call_cdr_record",
		AggregateID:    "call-1",
		Destination:    callflow.DestinationCDRDownstreamPush,
		IdempotencyKey: "cdr:downstream:call-1",
		Payload:        map[string]any{"callId": "call-1", "merchantId": 88, "downstreamTarget": "ods"},
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("expected downstream dispatched, got %d", dispatched)
	}
	store, ok := runtime.Downstream.(*downstreaminfra.PushMemoryStore)
	if !ok {
		t.Fatalf("expected memory downstream store")
	}
	if store.PushJobs["call-1"].Target != "ods" {
		t.Fatalf("expected downstream job, got %+v", store.PushJobs)
	}
}

func TestWorkerRuntimeDeliversDownstreamHTTP(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("X-Downstream-Target") != "ods" {
			t.Fatalf("unexpected target header: %s", r.Header.Get("X-Downstream-Target"))
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Worker.Downstream.URL = server.URL
	runtime := NewWorkerRuntimeWithConfig(cfg, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:downstream:call-1",
		AggregateType:  "call_cdr_record",
		AggregateID:    "call-1",
		Destination:    callflow.DestinationCDRDownstreamPush,
		IdempotencyKey: "cdr:downstream:call-1",
		Payload:        map[string]any{"callId": "call-1", "merchantId": 88, "downstreamTarget": "ods"},
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 || !called {
		t.Fatalf("expected downstream delivery, dispatched=%d called=%v", dispatched, called)
	}
	store := runtime.Downstream.(*downstreaminfra.PushMemoryStore)
	if store.PushJobs["call-1"].Status != downstreaminfra.StatusDelivered {
		t.Fatalf("expected delivered downstream job, got %+v", store.PushJobs["call-1"])
	}
}

func TestWorkerRuntimeDispatchesBatchCallback(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("X-Outbox-Id") != "callback:1" {
			t.Fatalf("unexpected outbox id header: %s", r.Header.Get("X-Outbox-Id"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.Worker.Callback.URL = server.URL
	runtime := NewWorkerRuntimeWithConfig(cfg, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "callback:1",
		AggregateType:  "batch_call_tel",
		AggregateID:    "10:20",
		Destination:    callflow.DestinationBatchCallback,
		IdempotencyKey: "callback:1",
		Payload:        map[string]any{"eventType": "batch_tel_completed", "batchTaskId": 10},
		NextAttemptAt:  time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 || !called {
		t.Fatalf("expected callback dispatched, dispatched=%d called=%v", dispatched, called)
	}
}

func TestWorkerRuntimeDispatchesCDROutbox(t *testing.T) {
	t.Parallel()

	runtime := NewWorkerRuntimeWithConfig(config.Config{}, nil)
	ctx := context.Background()
	if err := runtime.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:call-1",
		AggregateType:  "call",
		AggregateID:    "call-1",
		Destination:    "call_center_cdr_queue",
		IdempotencyKey: "cdr:call-1",
		Payload: map[string]any{
			"callId":      "call-1",
			"uuid":        "uuid-1",
			"finalState":  "complete",
			"completedAt": time.Now().UTC(),
		},
		NextAttemptAt: time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := runtime.Dispatcher.DispatchOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched != 1 {
		t.Fatalf("expected cdr dispatched, got %d", dispatched)
	}
	pending, err := runtime.Outbox.Pending(ctx, 10, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, entry := range pending {
		got[entry.Destination] = true
	}
	if !got[callflow.DestinationCDRBilling] || !got[callflow.DestinationCDRRecording] || !got[callflow.DestinationCDRReportProjection] || !got[callflow.DestinationCDRDownstreamPush] {
		t.Fatalf("expected cdr fanout entries, got %+v", pending)
	}
}
