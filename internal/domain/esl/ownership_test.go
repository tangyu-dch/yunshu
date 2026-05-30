package esl

import (
	"context"
	"testing"
	"time"
)

func TestEventOwnershipServiceClaim(t *testing.T) {
	t.Parallel()

	service := NewEventOwnershipService()
	ctx := context.Background()
	if !service.Claim(ctx, "fs-1", "esl-a", time.Minute) {
		t.Fatal("first owner should claim")
	}
	if service.Claim(ctx, "fs-1", "esl-b", time.Minute) {
		t.Fatal("second owner should not claim active lease")
	}
	if !service.Claim(ctx, "fs-1", "esl-a", time.Minute) {
		t.Fatal("current owner should renew")
	}
}
