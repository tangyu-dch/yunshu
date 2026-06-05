package cti

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimeSelectorFallsBackToNextCandidate(t *testing.T) {
	t.Parallel()

	selector := RuntimeSelector{
		Allocator: fakeRuntimeAllocator{failPhone: "1001"},
	}
	result, allocation, err := selector.SelectAndClaim(context.Background(), SelectionRequest{
		CallID: "call-1",
		Candidates: []NumberCandidate{
			{Phone: "1001", GatewayID: "gw-1", Available: true, RiskAllowed: true, Concurrency: 1},
			{Phone: "1002", GatewayID: "gw-2", Available: true, RiskAllowed: true, Concurrency: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if allocation == nil || allocation.Caller != "1002" {
		t.Fatalf("expected fallback allocation, got %+v", allocation)
	}
	if !result.Success || result.Caller == nil || result.Caller.Phone != "1002" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRuntimeSelectorFailsClosedWhenAllocatorMissing(t *testing.T) {
	t.Parallel()

	selector := RuntimeSelector{}
	result, allocation, err := selector.SelectAndClaim(context.Background(), SelectionRequest{
		CallID: "call-1",
		Candidates: []NumberCandidate{
			{Phone: "1001", GatewayID: "gw-1", Available: true, RiskAllowed: true, Concurrency: 1},
		},
	})
	if !errors.Is(err, ErrRuntimeAllocatorNotConfigured) {
		t.Fatalf("expected runtime allocator error, got %v", err)
	}
	if allocation != nil {
		t.Fatalf("expected no allocation, got %+v", allocation)
	}
	if result.Success {
		t.Fatalf("expected selection failure, got %+v", result)
	}
}

type fakeRuntimeAllocator struct {
	failPhone string
}

func (f fakeRuntimeAllocator) Claim(_ context.Context, _ SelectionRequest, candidates []NumberCandidate) (RuntimeAllocation, error) {
	if len(candidates) == 0 {
		return RuntimeAllocation{}, ErrNoAvailableNumber
	}
	if candidates[0].Phone == f.failPhone {
		return RuntimeAllocation{}, ErrRuntimeConcurrencyExhausted
	}
	return RuntimeAllocation{Caller: candidates[0].Phone, GatewayID: candidates[0].GatewayID}, nil
}

func (fakeRuntimeAllocator) Release(context.Context, RuntimeAllocation) error {
	return nil
}

func TestRuntimeSelectorReturnsNoAvailableWhenAllCandidatesExhausted(t *testing.T) {
	t.Parallel()

	selector := RuntimeSelector{
		Allocator: fakeRuntimeAllocator{failPhone: "1001"},
	}
	_, allocation, err := selector.SelectAndClaim(context.Background(), SelectionRequest{
		CallID: "call-1",
		Candidates: []NumberCandidate{
			{Phone: "1001", GatewayID: "gw-1", Available: true, RiskAllowed: true, Concurrency: 1},
		},
	})
	if !errors.Is(err, ErrRuntimeConcurrencyExhausted) && !errors.Is(err, ErrNoAvailableNumber) {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocation != nil {
		t.Fatalf("expected no allocation, got %+v", allocation)
	}
}

type fakeCandidateMarker struct {
	blacklistPhone string
}

func (f fakeCandidateMarker) MarkCandidates(_ context.Context, _ SelectionRequest, candidates []NumberCandidate) ([]NumberCandidate, error) {
	for i := range candidates {
		if candidates[i].Phone == f.blacklistPhone {
			candidates[i].BlacklistHit = true
		}
	}
	return candidates, nil
}

func TestRuntimeSelectorInvokesMarker(t *testing.T) {
	t.Parallel()

	selector := RuntimeSelector{
		Allocator: fakeRuntimeAllocator{},
		Marker:    fakeCandidateMarker{blacklistPhone: "1001"},
	}

	result, allocation, err := selector.SelectAndClaim(context.Background(), SelectionRequest{
		CallID: "call-1",
		Candidates: []NumberCandidate{
			{Phone: "1001", GatewayID: "gw-1", Available: true, RiskAllowed: true, Concurrency: 1},
			{Phone: "1002", GatewayID: "gw-2", Available: true, RiskAllowed: true, Concurrency: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if allocation == nil || allocation.Caller != "1002" {
		t.Fatalf("expected 1002 to be selected because 1001 was blacklisted, got %+v", allocation)
	}
	if !result.Success || result.Caller == nil || result.Caller.Phone != "1002" {
		t.Fatalf("unexpected selection result: %+v", result)
	}
}
