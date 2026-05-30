package limit

import (
	"context"
	"testing"
)

func TestShardedLimiter(t *testing.T) {
	t.Parallel()

	limiter := NewShardedLimiter()
	ctx := context.Background()
	limiter.SetLimit(ctx, "merchant:1", 2)

	if !limiter.Acquire(ctx, "merchant:1", 1) {
		t.Fatal("first slot should be acquired")
	}
	if !limiter.Acquire(ctx, "merchant:1", 1) {
		t.Fatal("second slot should be acquired")
	}
	if limiter.Acquire(ctx, "merchant:1", 1) {
		t.Fatal("third slot should be rejected")
	}

	limiter.Release(ctx, "merchant:1", 1)
	if limiter.Used(ctx, "merchant:1") != 1 {
		t.Fatalf("unexpected used slots: %d", limiter.Used(ctx, "merchant:1"))
	}
}
