package business

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/infra/merchant"
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

func TestSettlementMemoryStoreDebitBalanceReturnsNoOpWhenOverviewNotFound(t *testing.T) {
	t.Parallel()

	store := NewSettlementMemoryStore()
	// Do not populate store.Balance[88] to simulate missing overview record

	job, err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "billing:settlement:call-2",
		AggregateID: "call-2",
		Payload: map[string]any{
			"callId":     "call-2",
			"merchantId": 88,
			"amount":     10.0,
			"ratePerMin": 0.2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = store.DebitBalance(context.Background(), 88, job.Amount)
	if !errors.Is(err, ErrBillingOverviewNotFound) {
		t.Fatalf("expected ErrBillingOverviewNotFound, got: %v", err)
	}

	err = store.MarkNoOp(context.Background(), job.ID, "merchant billing overview not found")
	if err != nil {
		t.Fatal(err)
	}

	got := store.SettlementJobs["call-2"]
	if got.Status != StatusNoOp {
		t.Fatalf("expected status %s, got: %s", StatusNoOp, got.Status)
	}
	if got.LastError != "merchant billing overview not found" {
		t.Fatalf("expected last error to store reason, got: %s", got.LastError)
	}
}

func TestSettlementSettlementJobModelTableName(t *testing.T) {
	t.Parallel()

	if (SettlementJobModel{}).TableName() != "cc_biz_settlement" {
		t.Fatalf("unexpected table name")
	}
}

func TestSettlementGormStoreDebitBalanceAndMarkNoOp(t *testing.T) {
	t.Parallel()

	// Setup SQLite in-memory DB for GORM testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Run migration for necessary models
	err = db.AutoMigrate(&SettlementJobModel{}, &merchant.MerchantBillingOverviewModel{})
	if err != nil {
		t.Fatal(err)
	}

	store := NewSettlementGormStore(db, nil)
	ctx := context.Background()

	// 1. Save settlement job
	job, err := store.SaveFromOutbox(ctx, Entry{
		ID:          "billing:settlement:call-3",
		AggregateID: "call-3",
		Payload: map[string]any{
			"callId":     "call-3",
			"merchantId": 99,
			"amount":     5.5,
			"ratePerMin": 0.15,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Debit balance when billing overview record does not exist
	_, _, err = store.DebitBalance(ctx, 99, job.Amount)
	if !errors.Is(err, ErrBillingOverviewNotFound) {
		t.Fatalf("expected ErrBillingOverviewNotFound when overview does not exist, got: %v", err)
	}

	// 3. Mark job as no-op
	err = store.MarkNoOp(ctx, job.ID, "merchant billing overview not found")
	if err != nil {
		t.Fatal(err)
	}

	// Check job status in database
	var dbJob SettlementJobModel
	err = db.Where("id = ?", job.ID).First(&dbJob).Error
	if err != nil {
		t.Fatal(err)
	}
	if dbJob.Status != StatusNoOp {
		t.Fatalf("expected status in DB to be %s, got %s", StatusNoOp, dbJob.Status)
	}
	if dbJob.LastError != "merchant billing overview not found" {
		t.Fatalf("expected DB last error to store reason, got: %s", dbJob.LastError)
	}

	// 4. Test normal flow when billing overview DOES exist
	err = db.Create(&merchant.MerchantBillingOverviewModel{
		MerchantID:     99,
		CurrentBalance: 100.0,
		CreditLimit:    20.0,
	}).Error
	if err != nil {
		t.Fatal(err)
	}

	before, after, err := store.DebitBalance(ctx, 99, job.Amount)
	if err != nil {
		t.Fatal(err)
	}
	if before != 100.0 || after != 94.5 {
		t.Fatalf("unexpected balance change: %v -> %v", before, after)
	}

	// Check if updated in database
	var dbOverview merchant.MerchantBillingOverviewModel
	err = db.Where("merchant_id = ?", 99).First(&dbOverview).Error
	if err != nil {
		t.Fatal(err)
	}
	if dbOverview.CurrentBalance != 94.5 {
		t.Fatalf("expected CurrentBalance in DB to be 94.5, got %v", dbOverview.CurrentBalance)
	}
}
