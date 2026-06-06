// Package selection 提供 CTI 选号运行时 adapter。
package selection

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/domain/cti"
)

// batchClaimScript 是高并发批量原子试选的核心 Lua 脚本。
//
// 它将原先逐候选号码调用 Lua 脚本的 N 次 Redis 网络往返合并为单次 EVAL 原子操作：
// 按传入顺序依次试选每个候选号码，检查幂等、号码级并发和网关级并发的双重级联限制，
// 首个成功占用的候选立即返回，所有候选均失败则返回最后拒绝原因。
//
// KEYS 排列 (每个候选 3 个 Key，共 N 个候选):
//
//	[idem_1, counter_1, gwCounter_1, ..., idem_N, counter_N, gwCounter_N]
//
// ARGV 排列 (每个候选 4 个参数，共 N 个候选，最后追加 2 个全局参数):
//
//	[phone_1, gwID_1, limit_1, gwLimit_1, ..., phone_N, gwID_N, limit_N, gwLimit_N, ttl, merchantID]
//
// 返回值格式:
//
//	成功: {1, "phone|gwID|merchantID", gwID, index}  -- index 为从 1 开始的成功候选位置
//	全部耗尽: {0, "all_exhausted"}
const batchClaimScript = `
local n = #KEYS / 3
local ttl = tonumber(ARGV[n * 4 + 1])
local merchantID = ARGV[n * 4 + 2]

for i = 0, n - 1 do
  local idem = KEYS[i * 3 + 1]
  local counter = KEYS[i * 3 + 2]
  local gwCounter = KEYS[i * 3 + 3]
  local phone = ARGV[i * 4 + 1]
  local gwID = ARGV[i * 4 + 2]
  local limit = tonumber(ARGV[i * 4 + 3])
  local gwLimit = tonumber(ARGV[i * 4 + 4])
  local val = phone .. "|" .. gwID .. "|" .. merchantID

  -- 1. 幂等性校验：该 CallID 已占用此号码，直接返回成功（防止重试引发重复扣减）
  if redis.call("EXISTS", idem) == 1 then
    local existVal = redis.call("GET", idem)
    return {1, existVal, gwID, i + 1}
  end

  -- 2. 检查候选号码是否满足所有并发条件
  local eligible = true
  if limit <= 0 then
    eligible = false
  end
  if eligible then
    local current = tonumber(redis.call("GET", counter) or "0")
    if current >= limit then
      eligible = false
    end
  end
  if eligible and gwLimit > 0 then
    local gwCurrent = tonumber(redis.call("GET", gwCounter) or "0")
    if gwCurrent >= gwLimit then
      eligible = false
    end
  end

  -- 3. 满足条件则执行原子占用
  if eligible then
    local current = redis.call("INCR", counter)
    if current == 1 then
      redis.call("PEXPIRE", counter, ttl)
    end
    if gwLimit > 0 then
      local gwCurrent = redis.call("INCR", gwCounter)
      if gwCurrent == 1 then
        redis.call("PEXPIRE", gwCounter, ttl)
      end
    end
    redis.call("SET", idem, val, "PX", ttl)
    return {1, val, gwID, i + 1}
  end
end

return {0, "all_exhausted"}
`

// claimScript 是单个候选号码原子占用的 Lua 脚本（保留兼容，供单元测试和降级路径使用）。
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
// KEYS: [idem, counter, gwCounter]
//
// 业务逻辑：
//   - 检查幂等 Key 是否存在，若不存在，说明已被释放或从未占用过，直接幂等退出。
//   - 删除幂等 Key；
//   - 对号码计数器和网关计数器分别执行递减 (DECR) 操作，确保数据安全归零回落，且决不产生负数溢出。
const releaseScript = `
local idem = KEYS[1]
local counter = KEYS[2]
local gwCounter = KEYS[3]

if redis.call("EXISTS", idem) == 0 then
  return 0
end

redis.call("DEL", idem)

local current = tonumber(redis.call("GET", counter) or "0")
if current > 0 then
  redis.call("DECR", counter)
end

local gwCurrent = tonumber(redis.call("GET", gwCounter) or "0")
if gwCurrent > 0 then
  redis.call("DECR", gwCounter)
end

return 1
`

// renewScript 是看门狗续期 Lua 脚本。
// 重建幂等 Key、号码计数器和网关计数器的 TTL。
// KEYS: [idem, counter, gwCounter]
// ARGV: [ttl_ms]
// 返回: 1=续期成功, 0=幂等 Key 不存在（无需续期）
const renewScript = `
local idem = KEYS[1]
local counter = KEYS[2]
local gwCounter = KEYS[3]
local ttl = tonumber(ARGV[1])

if redis.call("EXISTS", idem) == 0 then
  return 0
end

redis.call("PEXPIRE", idem, ttl)
if redis.call("EXISTS", counter) == 1 then
  redis.call("PEXPIRE", counter, ttl)
end
if redis.call("EXISTS", gwCounter) == 1 then
  redis.call("PEXPIRE", gwCounter, ttl)
end

return 1
`

// RedisAllocator 使用 Redis Lua 脚本完成号码并发及网关全局物理并发的双重原子占用与幂等。
// 支持批量原子试选（单次 EVAL 处理所有候选号码，消除 N-1 网络往返）和看门狗续期。
type RedisAllocator struct {
	Client *goredis.Client // go-redis 客户端连接句柄
	TTL    time.Duration   // 运行时资源占用持有 TTL 租期，避免停机或崩溃导致号码死锁

	batchClaimCmd *goredis.Script // 预编译的批量原子试选 Lua 脚本
	renewCmd      *goredis.Script // 预编译的看门狗续期 Lua 脚本
}

// NewRedisAllocator 创建 Redis 运行时选号分配器。
func NewRedisAllocator(client *goredis.Client, ttl time.Duration) *RedisAllocator {
	if ttl == 0 {
		ttl = 30 * time.Minute // 默认租约周期为 30 分钟
	}
	return &RedisAllocator{
		Client:        client,
		TTL:           ttl,
		batchClaimCmd: goredis.NewScript(batchClaimScript),
		renewCmd:      goredis.NewScript(renewScript),
	}
}

// Claim 原子占用候选号列表中的并发槽位，遵循"逐个试选经过规则链的候选号码"的高并发原则。
//
// 本方法将所有候选号码打包进单次 Lua 脚本 EVAL 调用中，在 Redis 内部原子地逐个试选：
// - 只要号码或其绑定的网关中任意一个达到了并发上限，该候选者就会被跳过；
// - Redis 内部继续试选队列中的下一个候选号码，直到遇到能成功占用的候选或全部失败；
// - 相比逐候选 Go 循环 + N 次 EVAL，单次 EVAL 消除了 N-1 次网络往返，在高并发下显著降低延迟。
func (a *RedisAllocator) Claim(ctx context.Context, req cti.SelectionRequest, candidates []cti.NumberCandidate) (cti.RuntimeAllocation, error) {
	if len(candidates) == 0 {
		return cti.RuntimeAllocation{}, cti.ErrNoAvailableNumber
	}

	// 构造批量 Lua 脚本参数：
	//   KEYS = 每候选 3 个 Key（idem, counter, gwCounter）
	//   ARGV = 每候选 4 个参数（phone, gwID, limit, gwLimit）+ 2 个全局参数（ttl, merchantID）
	keys := make([]string, 0, len(candidates)*3)
	args := make([]any, 0, len(candidates)*4+2)
	for _, c := range candidates {
		keys = append(keys,
			fmt.Sprintf("cti:select:claim:%s", req.CallID),
			fmt.Sprintf("cti:select:counter:%s:%s:%s", req.MerchantID, c.GatewayID, c.Phone),
			fmt.Sprintf("cti:select:gateway:counter:%s:%s", req.MerchantID, c.GatewayID),
		)
		args = append(args, c.Phone, c.GatewayID, c.Concurrency, c.GatewayConcurrency)
	}
	args = append(args, a.TTL.Milliseconds(), req.MerchantID)

	// 使用 EVALSHA 优化：首次调用加载脚本并缓存 SHA，后续调用直接走 EVALSHA 减少带宽
	raw, err := a.batchClaimCmd.Eval(ctx, a.Client, keys, args...).Result()
	if err != nil {
		return cti.RuntimeAllocation{}, fmt.Errorf("批量选号 Lua 脚本执行失败: %w", err)
	}

	values, ok := raw.([]any)
	if !ok || len(values) < 2 {
		return cti.RuntimeAllocation{}, fmt.Errorf("批量选号 Lua 脚本返回格式异常: %v", raw)
	}

	accepted, _ := strconv.Atoi(fmt.Sprint(values[0]))
	if accepted != 1 {
		// 全部候选号码试选均告失败
		return cti.RuntimeAllocation{}, cti.ErrRuntimeConcurrencyExhausted
	}

	if len(values) < 4 {
		return cti.RuntimeAllocation{}, fmt.Errorf("批量选号成功但返回值不完整: %v", values)
	}

	// 解析成功占用的候选号码信息
	// values[1] = "phone|gwID|merchantID"（幂等值）
	// values[2] = gwID
	// values[3] = index (1-based)
	valStr := fmt.Sprint(values[1])
	gwID := fmt.Sprint(values[2])
	idx, _ := strconv.Atoi(fmt.Sprint(values[3]))
	phone, _, _ := parseClaimValue(valStr)
	claimKey := fmt.Sprintf("cti:select:claim:%s", req.CallID)

	return cti.RuntimeAllocation{
		CallID:         req.CallID,
		MerchantID:     req.MerchantID,
		Caller:         phone,
		GatewayID:      gwID,
		ClaimKey:       claimKey,
		CandidateIndex: idx - 1, // Lua 返回 1-based，转为 Go 0-based
	}, nil
}

// ClaimSingle 原子占用单个候选号码（保留供降级路径和单元测试使用）。
func (a *RedisAllocator) ClaimSingle(ctx context.Context, req cti.SelectionRequest, candidate cti.NumberCandidate) (cti.RuntimeAllocation, error) {
	limit := candidate.Concurrency
	if limit <= 0 {
		return cti.RuntimeAllocation{}, cti.ErrRuntimeConcurrencyExhausted
	}

	claimKey := fmt.Sprintf("cti:select:claim:%s", req.CallID)
	counterKey := fmt.Sprintf("cti:select:counter:%s:%s:%s", req.MerchantID, candidate.GatewayID, candidate.Phone)
	gwCounterKey := fmt.Sprintf("cti:select:gateway:counter:%s:%s", req.MerchantID, candidate.GatewayID)
	value := candidate.Phone + "|" + candidate.GatewayID + "|" + req.MerchantID

	raw, err := a.Client.Eval(ctx, claimScript, []string{claimKey, counterKey, gwCounterKey}, limit, candidate.GatewayConcurrency, a.TTL.Milliseconds(), value).Result()
	if err != nil {
		return cti.RuntimeAllocation{}, err
	}

	values, ok := raw.([]any)
	if !ok || len(values) < 2 {
		return cti.RuntimeAllocation{}, fmt.Errorf("unexpected redis claim response: %v", raw)
	}

	accepted, _ := strconv.Atoi(fmt.Sprint(values[0]))
	if accepted != 1 {
		return cti.RuntimeAllocation{}, cti.ErrRuntimeConcurrencyExhausted
	}

	return cti.RuntimeAllocation{
		CallID:     req.CallID,
		MerchantID: req.MerchantID,
		Caller:     candidate.Phone,
		GatewayID:  candidate.GatewayID,
		ClaimKey:   claimKey,
	}, nil
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

	counterKey := fmt.Sprintf("cti:select:counter:%s:%s:%s", allocation.MerchantID, allocation.GatewayID, allocation.Caller)
	gwCounterKey := fmt.Sprintf("cti:select:gateway:counter:%s:%s", allocation.MerchantID, allocation.GatewayID)

	_, err := a.Client.Eval(ctx, releaseScript, []string{claimKey, counterKey, gwCounterKey}).Result()
	return err
}

// RenewClaim 看门狗续期：延长指定占用信息的幂等 Key 及关联计数器的 TTL。
// 返回 true 表示续期成功，false 表示幂等 Key 已不存在（呼叫已释放或过期）。
func (a *RedisAllocator) RenewClaim(ctx context.Context, allocation cti.RuntimeAllocation) (bool, error) {
	if allocation.CallID == "" || allocation.Caller == "" || allocation.GatewayID == "" {
		return false, nil
	}
	claimKey := allocation.ClaimKey
	if claimKey == "" {
		claimKey = fmt.Sprintf("cti:select:claim:%s", allocation.CallID)
	}
	counterKey := fmt.Sprintf("cti:select:counter:%s:%s:%s", allocation.MerchantID, allocation.GatewayID, allocation.Caller)
	gwCounterKey := fmt.Sprintf("cti:select:gateway:counter:%s:%s", allocation.MerchantID, allocation.GatewayID)

	raw, err := a.renewCmd.Eval(ctx, a.Client, []string{claimKey, counterKey, gwCounterKey}, a.TTL.Milliseconds()).Result()
	if err != nil {
		return false, fmt.Errorf("看门狗续期 Lua 脚本执行失败: %w", err)
	}
	renewed, _ := strconv.Atoi(fmt.Sprint(raw))
	return renewed == 1, nil
}

// parseClaimValue 解析 "phone|gwID|merchantID" 格式的幂等值。
// 兼容旧格式 "phone|gwID"（merchantID 返回空字符串）。
func parseClaimValue(raw string) (phone, gwID, merchantID string) {
	parts := splitClaimValue(raw)
	if len(parts) < 2 {
		return "", "", ""
	}
	phone = parts[0]
	gwID = parts[1]
	if len(parts) >= 3 {
		merchantID = parts[2]
	}
	return
}

// splitClaimValue 按 "|" 分割 claim value。
func splitClaimValue(raw string) []string {
	result := make([]string, 0, 3)
	start := 0
	for i := 0; i < len(raw); i++ {
		if raw[i] == '|' {
			result = append(result, raw[start:i])
			start = i + 1
		}
	}
	result = append(result, raw[start:])
	return result
}
