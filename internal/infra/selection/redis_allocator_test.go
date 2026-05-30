package selection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/domain/cti"
)

func TestRedisAllocatorClaimIsIdempotent(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	allocator := NewRedisAllocator(client, time.Minute)
	req := cti.SelectionRequest{CallID: "call-1", MerchantID: "88"}
	candidates := []cti.NumberCandidate{{Phone: "10086", GatewayID: "gw-1", Concurrency: 1}}

	first, err := allocator.Claim(context.Background(), req, candidates)
	if err != nil {
		t.Fatal(err)
	}
	second, err := allocator.Claim(context.Background(), req, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if first.Caller != second.Caller || first.GatewayID != second.GatewayID {
		t.Fatalf("expected idempotent allocation, first=%+v second=%+v", first, second)
	}
}

func TestRedisAllocatorRejectsWhenConcurrencyExhausted(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	allocator := NewRedisAllocator(client, time.Minute)
	candidates := []cti.NumberCandidate{{Phone: "10086", GatewayID: "gw-1", Concurrency: 1}}
	if _, err := allocator.Claim(context.Background(), cti.SelectionRequest{CallID: "call-1", MerchantID: "88"}, candidates); err != nil {
		t.Fatal(err)
	}
	if _, err := allocator.Claim(context.Background(), cti.SelectionRequest{CallID: "call-2", MerchantID: "88"}, candidates); !errors.Is(err, ErrRuntimeConcurrencyExhausted) {
		t.Fatalf("expected concurrency exhausted, got %v", err)
	}
}

func TestRedisAllocatorReleaseFreesSlot(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	allocator := NewRedisAllocator(client, time.Minute)
	candidates := []cti.NumberCandidate{{Phone: "10086", GatewayID: "gw-1", Concurrency: 1}}
	allocation, err := allocator.Claim(context.Background(), cti.SelectionRequest{CallID: "call-1", MerchantID: "88"}, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if err := allocator.Release(context.Background(), allocation); err != nil {
		t.Fatal(err)
	}
	if _, err := allocator.Claim(context.Background(), cti.SelectionRequest{CallID: "call-2", MerchantID: "88"}, candidates); err != nil {
		t.Fatalf("expected slot released, got %v", err)
	}
}
