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

func TestRedisAllocatorClaimTryNextWhenFirstExhausted(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	allocator := NewRedisAllocator(client, time.Minute)

	candidates := []cti.NumberCandidate{
		{Phone: "10086", GatewayID: "gw-1", Concurrency: 1},
		{Phone: "10010", GatewayID: "gw-2", Concurrency: 1},
	}

	// 1. 第一个请求 call-1 分配，应该占用首个可用候选 10086
	first, err := allocator.Claim(context.Background(), cti.SelectionRequest{CallID: "call-1", MerchantID: "88"}, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if first.Caller != "10086" {
		t.Fatalf("expected 10086, got %s", first.Caller)
	}

	// 2. 第二个请求 call-2 分配，因为 10086 已满，应该自动试选并成功占用下一个 10010
	second, err := allocator.Claim(context.Background(), cti.SelectionRequest{CallID: "call-2", MerchantID: "88"}, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if second.Caller != "10010" {
		t.Fatalf("expected 10010, got %s", second.Caller)
	}

	// 3. 第三个请求 call-3 分配，因为两个都满额了，应该抛出 ErrRuntimeConcurrencyExhausted
	_, err = allocator.Claim(context.Background(), cti.SelectionRequest{CallID: "call-3", MerchantID: "88"}, candidates)
	if !errors.Is(err, ErrRuntimeConcurrencyExhausted) {
		t.Fatalf("expected concurrency exhausted error, got %v", err)
	}
}
