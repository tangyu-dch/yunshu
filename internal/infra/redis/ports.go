// Package redis 定义领域层所需的 Redis 原子操作抽象接口。
//
// 生产实现应使用 Lua 脚本或 Redis 事务来保证 SetNX、IncrBy 等多键操作的原子性。
package redis

import (
	"context"
	"time"
)

// AtomicStore 定义 Redis 原子存储接口，支持分布式锁和计数器场景。
type AtomicStore interface {
	// SetNX 在 key 不存在时设置值并返回 true，可选设置 TTL。
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	// Delete 删除指定 key。
	Delete(ctx context.Context, key string) error
	// IncrBy 对 key 的值增加 delta，支持设置 TTL。返回增加后的值。
	IncrBy(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error)
}
