package business

import (
	"context"
	"testing"
	"time"
)

func TestSettlementMemorySettlementStoreDebitAndMarkSettled(t *testing.T) {
	t.Parallel()

	store := NewSettlementMemoryStore()
	store.Balance[88] = 100
	job, err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "billing:settlement:call-1",
		AggregateID: "call-1",
		Payload: map[string]any{
			"callId":     "call-1",
			"merchantId": 88,
			"amount":     12.5,
			"ratePerMin": 0.3,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	before, after, err := store.DebitBalance(context.Background(), 88, job.Amount)
	if err != nil {
		t.Fatal(err)
	}
	if before != 100 || after != 87.5 {
		t.Fatalf("unexpected balance change: %v -> %v", before, after)
	}
	if err := store.MarkSettled(context.Background(), job.ID, before, after, time.Now()); err != nil {
		t.Fatal(err)
	}
	got := store.SettlementJobs["call-1"]
	if got.Status != StatusSettled || got.BalanceAfter != 87.5 {
		t.Fatalf("unexpected settlement job: %+v", got)
	}
}

func TestSettlementSettlementJobModelTableName(t *testing.T) {
	t.Parallel()

	if (SettlementJobModel{}).TableName() != "cc_biz_settlement" {
		t.Fatalf("unexpected table name")
	}
}
