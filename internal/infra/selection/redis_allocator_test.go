package selection

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
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

// TestRedisAllocator_GatewayAndPhoneConcurrencyStress 测试网关和号码双重物理并发限制下的高并发起呼原子占用与一致性释放。
// 我们设定：
// - 网关全局物理并发上限为 10；
// - 旗下包含 3 个主叫号码，每个号码自身的并发限制为 5；
// - 号码并发总限额是 15 (5 × 3)，但由于网关在 10 处被强力锁死，整体必须被截断阻断。
// - 开启 100 个 goroutine 并发抢占，强力断言最终获批起呼的协程数必须精准等于 10！
// - 释放后，两者的并发状态必须精准清空回落，不产生任何并发计数泄露。
func TestRedisAllocator_GatewayAndPhoneConcurrencyStress(t *testing.T) {
	t.Parallel()

	// 1. 初始化 miniredis 作为物理仿真环境
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	allocator := NewRedisAllocator(client, time.Minute)

	merchantID := "88"
	gatewayID := "gw-test-1"

	// 3 个候选号码，每个最大并发限制为 5
	// 网关全局最大物理并发卡死在 10 (即使号码额度之和是 15，也必须被网关强力截断在 10 处)
	candidates := []cti.NumberCandidate{
		{Phone: "13800000001", GatewayID: gatewayID, Concurrency: 5, GatewayConcurrency: 10},
		{Phone: "13800000002", GatewayID: gatewayID, Concurrency: 5, GatewayConcurrency: 10},
		{Phone: "13800000003", GatewayID: gatewayID, Concurrency: 5, GatewayConcurrency: 10},
	}

	// 2. 启动 100 个并发 goroutine 激烈竞争并发资源
	var wg sync.WaitGroup
	concurrency := 100
	wg.Add(concurrency)

	var allowedCalls int64 // 获准通过起呼的协程数
	var blockedCalls int64 // 被并发限制阻断拦截的协程数
	var systemErrors int64 // 发生系统异常的协程数

	var mu sync.Mutex
	successfulAllocations := make([]cti.RuntimeAllocation, 0, 10)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			req := cti.SelectionRequest{
				CallID:     "call-stress-" + strconv.Itoa(idx),
				MerchantID: merchantID,
			}

			// 原子试选与双重并发卡点申请
			allocation, err := allocator.Claim(context.Background(), req, candidates)
			if err != nil {
				if errors.Is(err, ErrRuntimeConcurrencyExhausted) {
					atomic.AddInt64(&blockedCalls, 1)
				} else {
					atomic.AddInt64(&systemErrors, 1)
				}
				return
			}

			// 成功起呼
			atomic.AddInt64(&allowedCalls, 1)

			mu.Lock()
			successfulAllocations = append(successfulAllocations, allocation)
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// 3. 强力断言与校验：
	// A. 系统稳定无错
	if systemErrors > 0 {
		t.Fatalf("【双重并发压测失败】: 抢占中发生了 %d 次未预期的系统异常或网络报错", systemErrors)
	}

	// B. 精准网关截断断言：允许起呼成功的次数必须【精准等于 10】（由于网关卡死在 10，即使号码总配额是 15 也绝不能漏拨！）
	if allowedCalls != 10 {
		t.Fatalf("【双重并发压测失败】: 期望允许起呼的次数精准为 10 次，实际却放行了 %d 次！(可能突破了网关并发防线)", allowedCalls)
	}

	// C. 精准阻断截断断言：被并发拒绝拦截的次数必须【精确等于 90】
	if blockedCalls != 90 {
		t.Fatalf("【双重并发压测失败】: 期望并发卡满阻断 90 次，实际阻断了 %d 次", blockedCalls)
	}

	// D. 校验 Redis 计数器内部事实：网关并发计数器必须恰好等于 10
	gwCounterKey := fmt.Sprintf("cti:select:gateway:counter:%s:%s", merchantID, gatewayID)
	gwValStr, err := client.Get(context.Background(), gwCounterKey).Result()
	if err != nil {
		t.Fatalf("【双重并发压测失败】: 获取 Redis 中网关并发计数器失败: %v", err)
	}
	gwVal, _ := strconv.Atoi(gwValStr)
	if gwVal != 10 {
		t.Fatalf("【双重并发压测失败】: 期望 Redis 内网关计数器为 10，实际为 %d", gwVal)
	}

	// E. 校验号码并发的分布：由于 Claim 按顺序试选，PhoneA 应该分到 5 次，PhoneB 应该分到 5 次，PhoneC 因为网关已满分到 0 次！
	for _, c := range candidates {
		key := fmt.Sprintf("cti:select:counter:%s:%s:%s", merchantID, gatewayID, c.Phone)
		valStr, err := client.Get(context.Background(), key).Result()
		if c.Phone == "13800000003" {
			// PhoneC 应从未被累加过，即 Redis 中不存在该键或值为 0
			if err != goredis.Nil && valStr != "0" {
				t.Fatalf("【双重并发压测失败】: 期望未超载的 PhoneC 计数器不存在，但实际获取值为: %s", valStr)
			}
		} else {
			if err != nil {
				t.Fatalf("【双重并发压测失败】: 获取号码 %s 计数器失败: %v", c.Phone, err)
			}
			val, _ := strconv.Atoi(valStr)
			if val != 5 {
				t.Fatalf("【双重并发压测失败】: 期望号码 %s 并发占用精准为 5，实际为 %d", c.Phone, val)
			}
		}
	}

	// ==========================================
	// 4. 并发级联释放与清空回落校验
	// ==========================================
	var releaseWg sync.WaitGroup
	releaseWg.Add(len(successfulAllocations))

	for _, alloc := range successfulAllocations {
		go func(a cti.RuntimeAllocation) {
			defer releaseWg.Done()
			err := allocator.Release(context.Background(), a)
			if err != nil {
				t.Errorf("【双重并发释放失败】: 释放分配 %+v 报错: %v", a, err)
			}
		}(alloc)
	}

	releaseWg.Wait()

	// 断言级联清零回落：
	// A. 网关并发计数器必须恰好等于 0
	gwValStrAfterRelease, err := client.Get(context.Background(), gwCounterKey).Result()
	if err != nil && err != goredis.Nil {
		t.Fatalf("【双重并发释放失败】: 释放后获取网关计数器异常: %v", err)
	}
	if err != goredis.Nil {
		gwVal, _ := strconv.Atoi(gwValStrAfterRelease)
		if gwVal != 0 {
			t.Fatalf("【双重并发释放失败】: 释放后网关计数器未清零，当前值为: %d", gwVal)
		}
	}

	// B. 所有的号码级并发计数器必须全部清零
	for _, c := range candidates {
		key := fmt.Sprintf("cti:select:counter:%s:%s:%s", merchantID, gatewayID, c.Phone)
		valStr, err := client.Get(context.Background(), key).Result()
		if err != nil && err != goredis.Nil {
			t.Fatalf("【双重并发释放失败】: 释放后获取号码 %s 计数器异常: %v", c.Phone, err)
		}
		if err != goredis.Nil {
			val, _ := strconv.Atoi(valStr)
			if val != 0 {
				t.Fatalf("【双重并发释放失败】: 释放后号码 %s 计数器未清零，当前值为: %d", c.Phone, val)
			}
		}
	}

	// C. 此时再次发起一次新的呼叫，网关和号码都应该能够被成功分配占用！
	newReq := cti.SelectionRequest{CallID: "call-new-after-release", MerchantID: merchantID}
	newAlloc, err := allocator.Claim(context.Background(), newReq, candidates)
	if err != nil {
		t.Fatalf("【双重并发释放失败】: 完全归还释放后，再次发起新分配失败: %v", err)
	}
	if newAlloc.Caller != "13800000001" {
		t.Fatalf("【双重并发释放失败】: 期望再次分配到首个号码 13800000001，实际分配到: %s", newAlloc.Caller)
	}
}

