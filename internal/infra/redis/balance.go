// Package redis 提供基于 Redis 的商户余额原子操作支持。
//
// 该模块实现了基于 Redis Lua 脚本的原子余额扣款，用于在高并发场景下
// 防止余额超扣，同时保持与数据库的最终一致性。
package redis

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const (
	// KeyMerchantBalancePrefix 是商户余额 Redis Key 的前缀。
	KeyMerchantBalancePrefix = "cc:balance:"
)

// DebitResult 表示原子扣款的结果状态。
type DebitResult int

const (
	// DebitResultSuccess 表示扣款成功。
	DebitResultSuccess DebitResult = 1
	// DebitResultInsufficientBalance 表示余额不足。
	DebitResultInsufficientBalance DebitResult = 0
	// DebitResultKeyNotExist 表示余额 Key 不存在（需要从数据库同步）。
	DebitResultKeyNotExist DebitResult = -1
)

// MerchantBalanceCache 封装 Redis 中商户余额的原子读写操作。
type MerchantBalanceCache struct {
	Client *redis.Client
	Logger *slog.Logger
}

// NewMerchantBalanceCache 创建商户余额缓存。
func NewMerchantBalanceCache(client *redis.Client, logger *slog.Logger) *MerchantBalanceCache {
	return &MerchantBalanceCache{
		Client: client,
		Logger: logger,
	}
}

// luaDebitScript 是 Redis Lua 脚本，用于原子检查和扣款。
//
// KEYS[1]: 商户余额 Key
// ARGV[1]: 扣款金额（字符串格式的浮点数）
// ARGV[2]: 信用额度（字符串格式的浮点数）
//
// 返回值：
//   1: 扣款成功
//   0: 余额不足
//  -1: Key 不存在
const luaDebitScript = `
local key = KEYS[1]
local amount = tonumber(ARGV[1])
local creditLimit = tonumber(ARGV[2])

local exists = redis.call('EXISTS', key)
if exists == 0 then
    return -1
end

local balance = tonumber(redis.call('HGET', key, 'balance') or 0)
local available = balance + creditLimit

if available < amount then
    return 0
end

redis.call('HINCRBYFLOAT', key, 'balance', -amount)
return 1
`

// AtomicDebit 使用 Lua 脚本原子扣款。
func (c *MerchantBalanceCache) AtomicDebit(ctx context.Context, merchantID int, amount float64, creditLimit float64) (DebitResult, error) {
	if c.Client == nil {
		return DebitResultKeyNotExist, fmt.Errorf("redis client not initialized")
	}

	key := KeyMerchantBalancePrefix + fmt.Sprintf("%d", merchantID)
	result, err := c.Client.Eval(ctx, luaDebitScript, []string{key},
		fmt.Sprintf("%.8f", amount),
		fmt.Sprintf("%.8f", creditLimit),
	).Int()

	if err != nil {
		c.logger().Error("原子扣款 Lua 脚本执行失败", "merchantId", merchantID, "amount", amount, "error", err.Error())
		return DebitResultKeyNotExist, err
	}

	debitResult := DebitResult(result)
	switch debitResult {
	case DebitResultSuccess:
		c.logger().Info("原子扣款成功", "merchantId", merchantID, "amount", amount)
	case DebitResultInsufficientBalance:
		c.logger().Warn("原子扣款失败：余额不足", "merchantId", merchantID, "amount", amount)
	case DebitResultKeyNotExist:
		c.logger().Info("原子扣款 Key 不存在，需要从数据库同步", "merchantId", merchantID)
	}

	return debitResult, nil
}

// SyncFromDB 从数据库重新加载商户余额到 Redis。
// 该方法应在以下场景调用：
//  1. 服务启动时批量同步
//  2. 管理端充值/调整余额后
//  3. Lua 脚本发现 Key 不存在时
func (c *MerchantBalanceCache) SyncFromDB(ctx context.Context, merchantID int, balance float64, creditLimit float64) error {
	if c.Client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	key := KeyMerchantBalancePrefix + fmt.Sprintf("%d", merchantID)
	pipe := c.Client.Pipeline()
	pipe.HSet(ctx, key, "balance", balance)
	pipe.HSet(ctx, key, "creditLimit", creditLimit)
	// 余额 Key 不设置 TTL，永久有效（除非显式失效）
	if _, err := pipe.Exec(ctx); err != nil {
		c.logger().Error("同步商户余额到 Redis 失败", "merchantId", merchantID, "error", err.Error())
		return err
	}

	c.logger().Info("商户余额已同步到 Redis", "merchantId", merchantID, "balance", balance, "creditLimit", creditLimit)
	return nil
}

// Invalidate 清除商户余额缓存（强制下次从数据库重新加载）。
func (c *MerchantBalanceCache) Invalidate(ctx context.Context, merchantID int) error {
	if c.Client == nil {
		return nil
	}

	key := KeyMerchantBalancePrefix + fmt.Sprintf("%d", merchantID)
	if err := c.Client.Del(ctx, key).Err(); err != nil {
		c.logger().Warn("清除商户余额缓存失败", "merchantId", merchantID, "error", err.Error())
		return err
	}

	c.logger().Info("商户余额缓存已清除", "merchantId", merchantID)
	return nil
}

// GetBalance 从 Redis 读取当前余额（仅供调试用，生产环境应以数据库为准）。
func (c *MerchantBalanceCache) GetBalance(ctx context.Context, merchantID int) (float64, error) {
	if c.Client == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}

	key := KeyMerchantBalancePrefix + fmt.Sprintf("%d", merchantID)
	balanceStr, err := c.Client.HGet(ctx, key, "balance").Result()
	if err != nil {
		if err == redis.Nil {
			return 0, fmt.Errorf("balance key not found")
		}
		return 0, err
	}

	var balance float64
	_, err = fmt.Sscanf(balanceStr, "%f", &balance)
	return balance, err
}

func (c *MerchantBalanceCache) logger() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}
