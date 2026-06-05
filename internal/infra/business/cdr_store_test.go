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

func TestRecordFromOutboxCallerCalleeResolution(t *testing.T) {
	t.Parallel()

	// 1. 测试 API 外呼 (Agent First)
	entryAPI := Entry{
		ID: "api-out",
		Payload: map[string]any{
			"callId":         "call-api",
			"profile":        "api_outbound",
			"extension":      "1001",
			"callee":         "13800000001",
			"selectedCaller": "02180010001",
		},
	}
	recordAPI := recordFromOutbox(entryAPI)
	if recordAPI.Caller != "02180010001" {
		t.Fatalf("expected caller to be 02180010001, got %s", recordAPI.Caller)
	}
	if recordAPI.Callee != "13800000001" {
		t.Fatalf("expected callee to be 13800000001, got %s", recordAPI.Callee)
	}

	// 2. 测试 批量外呼 (Customer First)
	entryBatch := Entry{
		ID: "batch-out",
		Payload: map[string]any{
			"callId":                   "call-batch",
			"profile":                  "batch_outbound",
			"extension":                "1001",
			"variable_customer_number": "13900000002",
			"selectedCaller":           "02180010002",
		},
	}
	recordBatch := recordFromOutbox(entryBatch)
	if recordBatch.Caller != "02180010002" {
		t.Fatalf("expected batch caller to be 02180010002, got %s", recordBatch.Caller)
	}
	if recordBatch.Callee != "13900000002" {
		t.Fatalf("expected batch callee to be 13900000002, got %s", recordBatch.Callee)
	}
}

func TestCallRecordFromModelResolution(t *testing.T) {
	t.Parallel()

	// 1. 测试优先读取数据库物理列
	modelDirect := RecordModel{
		CallID:               "call-test-res-1",
		Caller:               "02180010002",
		Callee:               "13900000002",
		DurationSec:          60,
		Extension:            "2002",
		SipHangupDisposition: "send_bye",
		RawPayload: JSONMap{
			"profile":                  "batch_outbound",
			"variable_agent_extension": "1001",
			"sipHangupDisposition":     "recv_bye",
			"billsec":                  45,
		},
	}
	recordDirect := callRecordFromModel(modelDirect)
	if recordDirect.Extension != "2002" {
		t.Errorf("expected direct extension to be 2002, got %s", recordDirect.Extension)
	}
	if recordDirect.SipHangupDisposition != "send_bye" {
		t.Errorf("expected direct sipHangupDisposition to be send_bye, got %s", recordDirect.SipHangupDisposition)
	}

	// 2. 测试降级读取 payload 兼容历史数据
	modelFallback := RecordModel{
		CallID:      "call-test-res-2",
		Caller:      "02180010002",
		Callee:      "13900000002",
		DurationSec: 60,
		RawPayload: JSONMap{
			"profile":                  "batch_outbound",
			"variable_agent_extension": "1001",
			"sipHangupDisposition":     "recv_bye",
			"billsec":                  45,
		},
	}

	recordFallback := callRecordFromModel(modelFallback)

	if recordFallback.Extension != "1001" {
		t.Errorf("expected fallback extension to be 1001, got %s", recordFallback.Extension)
	}
	if recordFallback.SipHangupDisposition != "recv_bye" {
		t.Errorf("expected fallback sipHangupDisposition to be recv_bye, got %s", recordFallback.SipHangupDisposition)
	}
	if recordFallback.Billsec != 45 {
		t.Errorf("expected billsec to be 45, got %d", recordFallback.Billsec)
	}
	if recordFallback.Ringsec != 15 {
		t.Errorf("expected ringsec to be 15, got %d", recordFallback.Ringsec)
	}
}
