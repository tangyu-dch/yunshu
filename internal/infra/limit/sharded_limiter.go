// Package limit 提供 CTI 资源分配需要的并发控制原语。
//
// 内存实现用于测试；生产实现应使用 Redis Lua 或 SQL CAS 保证 acquire/release 原子性。
package limit

import (
	"context"
	"sync"
)

// ShardedLimiter 是按 key 分片的并发限制器。
// 每个 key 维护独立的 limit 和 used 计数，支持商户、网关、任务等多种资源维度的并发控制。
type ShardedLimiter struct {
	mu     sync.Mutex
	limits map[string]int
	used   map[string]int
}

// NewShardedLimiter 创建按 key 隔离的并发限制器。
// 初始状态下所有 key 都没有限制，需要通过 SetLimit 设置。
func NewShardedLimiter() *ShardedLimiter {
	return &ShardedLimiter{limits: map[string]int{}, used: map[string]int{}}
}

// SetLimit 设置某个资源维度的最大并发数。
// 设置 limit <= 0 会清除该 key 的限制并重置已使用计数。
func (l *ShardedLimiter) SetLimit(_ context.Context, key string, limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limits[key] = limit
	if limit <= 0 {
		delete(l.used, key)
	}
}

// Acquire 尝试占用并发槽位。
func (l *ShardedLimiter) Acquire(_ context.Context, key string, slots int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slots <= 0 {
		return false
	}
	limit := l.limits[key]
	if limit <= 0 || l.used[key]+slots > limit {
		return false
	}
	l.used[key] += slots
	return true
}

// Release 释放并发槽位。重复释放不会把计数降到负数。
func (l *ShardedLimiter) Release(_ context.Context, key string, slots int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if slots <= 0 {
		return
	}
	l.used[key] -= slots
	if l.used[key] <= 0 {
		delete(l.used, key)
	}
}

// Used 返回指定 key 当前已占用的槽位数量。
// 如果该 key 从未被设置过，返回 0。
func (l *ShardedLimiter) Used(_ context.Context, key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.used[key]
}
