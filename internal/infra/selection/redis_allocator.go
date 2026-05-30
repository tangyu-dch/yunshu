// Package selection 提供 CTI 选号运行时 adapter。
package selection

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/domain/cti"
)

var ErrRuntimeConcurrencyExhausted = errors.New("runtime number concurrency exhausted")

const claimScript = `
local idem = KEYS[1]
local counter = KEYS[2]
local limit = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
if redis.call("EXISTS", idem) == 1 then
  return {1, redis.call("GET", idem)}
end
local current = tonumber(redis.call("GET", counter) or "0")
if current >= limit then
  return {0, ""}
end
current = redis.call("INCR", counter)
if current == 1 then
  redis.call("PEXPIRE", counter, ttl)
end
redis.call("SET", idem, ARGV[3], "PX", ttl)
return {1, ARGV[3]}
`

const releaseScript = `
local idem = KEYS[1]
local counter = KEYS[2]
if redis.call("EXISTS", idem) == 0 then
  return 0
end
redis.call("DEL", idem)
local current = tonumber(redis.call("GET", counter) or "0")
if current > 0 then
  redis.call("DECR", counter)
end
return 1
`

// RedisAllocator 使用 Redis Lua 完成号码并发占用和幂等。
type RedisAllocator struct {
	Client *goredis.Client
	TTL    time.Duration
}

// NewRedisAllocator 创建 Redis 运行时选号 allocator。
func NewRedisAllocator(client *goredis.Client, ttl time.Duration) *RedisAllocator {
	if ttl == 0 {
		ttl = 30 * time.Minute
	}
	return &RedisAllocator{Client: client, TTL: ttl}
}

// Claim 原子占用一个候选号的并发槽位。
func (a *RedisAllocator) Claim(ctx context.Context, req cti.SelectionRequest, candidates []cti.NumberCandidate) (cti.RuntimeAllocation, error) {
	if len(candidates) == 0 {
		return cti.RuntimeAllocation{}, cti.ErrNoAvailableNumber
	}
	candidate := candidates[0]
	limit := candidate.Concurrency
	if limit <= 0 {
		return cti.RuntimeAllocation{}, ErrRuntimeConcurrencyExhausted
	}
	claimKey := fmt.Sprintf("cti:select:claim:%s", req.CallID)
	counterKey := fmt.Sprintf("cti:select:counter:%s:%s:%s", req.MerchantID, candidate.GatewayID, candidate.Phone)
	value := candidate.Phone + "|" + candidate.GatewayID
	raw, err := a.Client.Eval(ctx, claimScript, []string{claimKey, counterKey}, limit, a.TTL.Milliseconds(), value).Result()
	if err != nil {
		return cti.RuntimeAllocation{}, err
	}
	values, ok := raw.([]any)
	if !ok || len(values) < 2 {
		return cti.RuntimeAllocation{}, fmt.Errorf("unexpected redis claim response: %v", raw)
	}
	accepted, _ := strconv.Atoi(fmt.Sprint(values[0]))
	if accepted != 1 {
		return cti.RuntimeAllocation{}, ErrRuntimeConcurrencyExhausted
	}
	return cti.RuntimeAllocation{CallID: req.CallID, MerchantID: req.MerchantID, Caller: candidate.Phone, GatewayID: candidate.GatewayID, ClaimKey: claimKey}, nil
}

// Release 幂等释放号码并发槽位。
func (a *RedisAllocator) Release(ctx context.Context, allocation cti.RuntimeAllocation) error {
	if allocation.CallID == "" || allocation.Caller == "" || allocation.GatewayID == "" {
		return nil
	}
	claimKey := allocation.ClaimKey
	if claimKey == "" {
		claimKey = fmt.Sprintf("cti:select:claim:%s", allocation.CallID)
	}
	counterKey := fmt.Sprintf("cti:select:counter:%s:%s:%s", allocation.MerchantID, allocation.GatewayID, allocation.Caller)
	_, err := a.Client.Eval(ctx, releaseScript, []string{claimKey, counterKey}).Result()
	return err
}
