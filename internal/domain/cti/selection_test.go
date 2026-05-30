package cti

import (
	"context"
	"testing"
)

func TestSelectorSelectsFirstPassingCandidate(t *testing.T) {
	t.Parallel()

	result := Selector{}.Select(context.Background(), SelectionRequest{
		CallID: "call-1",
		Candidates: []NumberCandidate{
			{Phone: "1001", Available: false, RiskAllowed: true, Concurrency: 1},
			{Phone: "1002", Available: true, RiskAllowed: true, Concurrency: 2},
		},
	})

	if !result.Success {
		t.Fatalf("expected success: %+v", result)
	}
	if result.Caller == nil || result.Caller.Phone != "1002" {
		t.Fatalf("unexpected caller: %+v", result.Caller)
	}
}

func TestSelectorReturnsBusinessFailure(t *testing.T) {
	t.Parallel()

	result := Selector{}.Select(context.Background(), SelectionRequest{
		CallID: "call-1",
		Candidates: []NumberCandidate{
			{Phone: "1001", Available: true, BlacklistHit: true, RiskAllowed: true, Concurrency: 1},
			{Phone: "1002", Available: true, RiskAllowed: false, Concurrency: 1},
			{Phone: "1003", Available: true, RiskAllowed: true, Concurrency: 0},
		},
	})

	if result.Success {
		t.Fatal("expected selection failure")
	}
	if result.Reason != ErrNoAvailableNumber.Error() {
		t.Fatalf("unexpected reason: %s", result.Reason)
	}
	if len(result.Trace) == 0 {
		t.Fatal("expected selection trace")
	}
}

func TestSelectorPrefersWhitelistAndHigherConcurrency(t *testing.T) {
	t.Parallel()

	result := Selector{}.Select(context.Background(), SelectionRequest{
		CallID: "call-1",
		Candidates: []NumberCandidate{
			{Phone: "1001", Available: true, RiskAllowed: true, Concurrency: 1},
			{Phone: "1002", Available: true, RiskAllowed: true, Concurrency: 3, WhitelistHit: true},
			{Phone: "1003", Available: true, RiskAllowed: true, Concurrency: 2},
		},
	})

	if !result.Success || result.Caller == nil {
		t.Fatalf("expected success: %+v", result)
	}
	if result.Caller.Phone != "1002" {
		t.Fatalf("expected whitelist candidate first, got %+v", result.Caller)
	}
}
