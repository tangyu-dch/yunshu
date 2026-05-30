package idempotency

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreClaimRelease(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	ctx := context.Background()

	claimed, err := store.Claim(ctx, "cmd-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("first claim should succeed")
	}

	claimed, err = store.Claim(ctx, "cmd-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("duplicate claim should be rejected")
	}

	if err := store.Release(ctx, "cmd-1"); err != nil {
		t.Fatal(err)
	}
	claimed, err = store.Claim(ctx, "cmd-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("claim after release should succeed")
	}
}
