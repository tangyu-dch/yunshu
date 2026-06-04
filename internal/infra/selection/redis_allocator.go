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

// claimScript 是高并发原子起呼占用的核心 Lua 脚本。
// 它接收 3 个 KEYS 和 4 个 ARGV 参数：
// KEYS:
//  1. idem: 选号占用幂等 Key (例如 cti:select:claim:callID)
//  2. counter: 号码级别的并发计数器 Key
//  3. gwCounter: 网关级别的全局并发计数器 Key
//
// ARGV:
//  1. limit: 号码允许的最大并发上限
//  2. gwLimit: 网关允许的最大并发上限 (如果 <= 0 则代表网关无限制)
//  3. ttl: 本次占用的生存时间 (毫秒)
//  4. val: 幂等 Key 对应存储的特征数据 (如 "Phone|GatewayID")
//
// 业务逻辑：
//   - 首先进行幂等性自检，若该 CallID 已经成功占用，则直接返回成功。
//   - 其次检查号码本身的并发计数器是否超限；
//   - 若 `gwLimit > 0`，则检查网关全局的并发计数器是否超限；
//   - 若两者均未超额，则执行原子累加 (INCR)，并为其设置 TTL 过期时间作为高频话务防死占的租约保护；
//   - 一旦触发超限拦截，原子的返回拦截标识，不产生脏扣减。
const claimScript = `
local idem = KEYS[1]
local counter = KEYS[2]
local gwCounter = KEYS[3]
local limit = tonumber(ARGV[1])
local gwLimit = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local val = ARGV[4]

-- 1. 幂等性校验，避免高频重试引发重复扣减
if redis.call("EXISTS", idem) == 1 then
  return {1, redis.call("GET", idem)}
end

-- 2. 原子校验号码并发限制
local current = tonumber(redis.call("GET", counter) or "0")
if current >= limit then
  return {0, "number_concurrency_exhausted"}
end

-- 3. 原子校验网关物理并发限制 (若 gwLimit > 0)
if gwLimit > 0 then
  local gwCurrent = tonumber(redis.call("GET", gwCounter) or "0")
  if gwCurrent >= gwLimit then
    return {0, "gateway_concurrency_exhausted"}
  end
end

-- 4. 累加号码级并发计数器，并初始化 TTL 生存期
current = redis.call("INCR", counter)
if current == 1 then
  redis.call("PEXPIRE", counter, ttl)
end

-- 5. 累加网关级全局并发计数器，并初始化 TTL 生存期
if gwLimit > 0 then
  local gwCurrent = redis.call("INCR", gwCounter)
  if gwCurrent == 1 then
    redis.call("PEXPIRE", gwCounter, ttl)
  end
end

-- 6. 标记幂等 Key，返回原子占用成功
redis.call("SET", idem, val, "PX", ttl)
return {1, val}
`

// releaseScript 是安全归还并发额度的核心 Lua 脚本。
// 它接收 3 个 KEYS：
// KEYS:
//  1. idem: 选号占用幂等 Key
//  2. counter: 号码级别的并发计数器 Key
//  3. gwCounter: 网关级别的全局并发计数器 Key
//
// 业务逻辑：
//   - 检查幂等 Key 是否存在，若不存在，说明已被释放或从未占用过，直接幂等退出。
//   - 删除幂等 Key；
//   - 对号码计数器和网关计数器分别执行递减 (DECR) 操作，确保数据安全归零回落，且决不产生负数溢出。
const releaseScript = `
local idem = KEYS[1]
local counter = KEYS[2]
local gwCounter = KEYS[3]

-- 1. 幂等校验
if redis.call("EXISTS", idem) == 0 then
  return 0
end

-- 2. 删除幂等防线
redis.call("DEL", idem)

-- 3. 原子释放号码级并发槽位
local current = tonumber(redis.call("GET", counter) or "0")
if current > 0 then
  redis.call("DECR", counter)
end

-- 4. 原子释放网关级全局并发槽位
local gwCurrent = tonumber(redis.call("GET", gwCounter) or "0")
if gwCurrent > 0 then
  redis.call("DECR", gwCounter)
end

return 1
`

// RedisAllocator 使用 Redis Lua 脚本完成号码并发及网关全局物理并发的双重原子占用与幂等。
type RedisAllocator struct {
	Client *goredis.Client // go-redis 客户端连接句柄
	TTL    time.Duration   // 运行时资源占用持有 TTL 租期，避免停机或崩溃导致号码死锁
}

// NewRedisAllocator 创建 Redis 运行时选号分配器。
func NewRedisAllocator(client *goredis.Client, ttl time.Duration) *RedisAllocator {
	if ttl == 0 {
		ttl = 30 * time.Minute // 默认租约周期为 30 分钟
	}
	return &RedisAllocator{Client: client, TTL: ttl}
}

// Claim 原子占用候选号列表中的并发槽位，遵循“逐个试选经过规则链的候选号码”的高并发原则。
// 本函数升级后支持号码级并发和网关级全局物理并发的双重级联原子限制：
// - 只要号码或其绑定的网关中任意一个达到了并发上限，该候选者就会被判定为占用失败；
// - 分配器将继续试选队列中的下一个候选号码，直到遇到能成功占用的候选或全部超限失败。
func (a *RedisAllocator) Claim(ctx context.Context, req cti.SelectionRequest, candidates []cti.NumberCandidate) (cti.RuntimeAllocation, error) {
	if len(candidates) == 0 {
		return cti.RuntimeAllocation{}, cti.ErrNoAvailableNumber
	}

	var lastErr error = ErrRuntimeConcurrencyExhausted
	for _, candidate := range candidates {
		limit := candidate.Concurrency
		if limit <= 0 {
			lastErr = ErrRuntimeConcurrencyExhausted
			continue
		}

		gwLimit := candidate.GatewayConcurrency

		// 组装 Redis 键名元数据
		claimKey := fmt.Sprintf("cti:select:claim:%s", req.CallID)
		counterKey := fmt.Sprintf("cti:select:counter:%s:%s:%s", req.MerchantID, candidate.GatewayID, candidate.Phone)
		gwCounterKey := fmt.Sprintf("cti:select:gateway:counter:%s:%s", req.MerchantID, candidate.GatewayID)
		value := candidate.Phone + "|" + candidate.GatewayID

		// 执行原子 Lua 脚本进行双重并发校验与抢占
		raw, err := a.Client.Eval(ctx, claimScript, []string{claimKey, counterKey, gwCounterKey}, limit, gwLimit, a.TTL.Milliseconds(), value).Result()
		if err != nil {
			lastErr = err
			continue
		}

		values, ok := raw.([]any)
		if !ok || len(values) < 2 {
			lastErr = fmt.Errorf("unexpected redis claim response: %v", raw)
			continue
		}

		accepted, _ := strconv.Atoi(fmt.Sprint(values[0]))
		if accepted != 1 {
			lastErr = ErrRuntimeConcurrencyExhausted
			continue
		}

		// 成功原子占用当前候选号码及网关并发资源，立即返回并结束试选
		return cti.RuntimeAllocation{
			CallID:     req.CallID,
			MerchantID: req.MerchantID,
			Caller:     candidate.Phone,
			GatewayID:  candidate.GatewayID,
			ClaimKey:   claimKey,
		}, nil
	}

	// 整组号码试选均告失败后，向外抛出最后一次捕获到的超限错误
	return cti.RuntimeAllocation{}, lastErr
}

// Release 幂等释放号码和网关所占用的运行时并发槽位。
func (a *RedisAllocator) Release(ctx context.Context, allocation cti.RuntimeAllocation) error {
	if allocation.CallID == "" || allocation.Caller == "" || allocation.GatewayID == "" {
		return nil
	}
	claimKey := allocation.ClaimKey
	if claimKey == "" {
		claimKey = fmt.Sprintf("cti:select:claim:%s", allocation.CallID)
	}

	// 组装对应的号码并发和网关并发 Key 事实源
	counterKey := fmt.Sprintf("cti:select:counter:%s:%s:%s", allocation.MerchantID, allocation.GatewayID, allocation.Caller)
	gwCounterKey := fmt.Sprintf("cti:select:gateway:counter:%s:%s", allocation.MerchantID, allocation.GatewayID)

	// 调用 Lua 脚本原子级联释放，避免漏扣
	_, err := a.Client.Eval(ctx, releaseScript, []string{claimKey, counterKey, gwCounterKey}).Result()
	return err
}
