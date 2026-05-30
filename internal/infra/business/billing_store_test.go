package business

import (
	"context"
	"testing"
)

func TestBillingLedgerMemoryStoreSaveFromOutbox(t *testing.T) {
	t.Parallel()

	store := NewBillingLedgerMemoryStore()
	ledger, err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "cdr:billing:call-1",
		AggregateID: "call-1",
		Payload: map[string]any{
			"callId":         "call-1",
			"merchantId":     88,
			"userId":         99,
			"profile":        "api_outbound",
			"durationSec":    31,
			"sourceOutboxId": "cdr:call-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ledger.MerchantID != 88 || ledger.DurationSec != 31 || ledger.Status != StatusPending || ledger.SourceOutbox != "cdr:call-1" {
		t.Fatalf("unexpected billing ledger: %+v", ledger)
	}
}

func TestBillingLedgerMemoryStoreMarkRated(t *testing.T) {
	t.Parallel()

	store := NewBillingLedgerMemoryStore()
	ledger, err := store.SaveFromOutbox(context.Background(), Entry{AggregateID: "call-1", Payload: map[string]any{"callId": "call-1", "durationSec": 61}})
	if err != nil {
		t.Fatal(err)
	}
	rating := EstimateByMinute(ledger.DurationSec, 0.12)
	if err := store.MarkRated(context.Background(), ledger.CallID, rating.Amount, rating.RatePerMin, rating.Note); err != nil {
		t.Fatal(err)
	}
	got := store.Ledgers["call-1"]
	if got.Status != StatusRated || got.Amount != 0.24 || got.RatingNote == "" {
		t.Fatalf("unexpected rated ledger: %+v", got)
	}
}

func TestEstimateByMinute(t *testing.T) {
	t.Parallel()

	got := EstimateByMinute(61, 0.12)
	if got.Amount != 0.24 || got.Note == "" {
		t.Fatalf("unexpected rating: %+v", got)
	}
}

func TestLedgerModelTableName(t *testing.T) {
	t.Parallel()

	if (LedgerModel{}).TableName() != "cc_biz_ledger" {
		t.Fatalf("unexpected table name")
	}
}
