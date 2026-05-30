package cti_test

import (
	"context"
	"errors"
	"testing"

	"yunshu/internal/domain/cti"
	"yunshu/internal/infra/business"
	"yunshu/internal/infra/limit"
	"yunshu/pkg/idempotency"
)

func TestAllocationServiceAllocatesAndWritesOutbox(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	limiter := limit.NewShardedLimiter()
	limiter.SetLimit(ctx, "merchant:m1", 1)
	service := cti.NewAllocationService(idempotency.NewMemoryStore(), limiter, business.NewOutboxMemoryStore())

	result, err := service.Allocate(ctx, cti.AllocationRequest{
		CommandID:  "cmd-1",
		CallID:     "call-1",
		MerchantID: "m1",
		Callee:     "13800000000",
		Candidates: []cti.NumberCandidate{{Phone: "1001", GatewayID: "gw-1", Available: true, RiskAllowed: true, Concurrency: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Selection.Success || result.OutboxID == "" {
		t.Fatalf("unexpected allocation result: %+v", result)
	}
	if limiter.Used(ctx, "merchant:m1") != 1 {
		t.Fatalf("expected one used slot")
	}
}

func TestAllocationServiceRejectsDuplicate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	limiter := limit.NewShardedLimiter()
	limiter.SetLimit(ctx, "merchant:m1", 2)
	service := cti.NewAllocationService(idempotency.NewMemoryStore(), limiter, business.NewOutboxMemoryStore())
	req := cti.AllocationRequest{
		CommandID:  "cmd-1",
		CallID:     "call-1",
		MerchantID: "m1",
		Candidates: []cti.NumberCandidate{{Phone: "1001", Available: true, RiskAllowed: true, Concurrency: 1}},
	}
	if _, err := service.Allocate(ctx, req); err != nil {
		t.Fatal(err)
	}
	_, err := service.Allocate(ctx, req)
	if !errors.Is(err, cti.ErrDuplicateAllocation) {
		t.Fatalf("expected duplicate allocation, got %v", err)
	}
}

func TestAllocationServiceReleasesIdempotencyOnBusinessFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	limiter := limit.NewShardedLimiter()
	limiter.SetLimit(ctx, "merchant:m1", 1)
	service := cti.NewAllocationService(idempotency.NewMemoryStore(), limiter, business.NewOutboxMemoryStore())
	req := cti.AllocationRequest{CommandID: "cmd-1", CallID: "call-1", MerchantID: "m1"}

	if _, err := service.Allocate(ctx, req); !errors.Is(err, cti.ErrNoAvailableNumber) {
		t.Fatalf("expected no available number, got %v", err)
	}
	if _, err := service.Allocate(ctx, req); !errors.Is(err, cti.ErrNoAvailableNumber) {
		t.Fatalf("idempotency should be released for retryable business failure, got %v", err)
	}
}
