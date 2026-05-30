package idempotency

import (
	"context"
	"sync"
	"time"
)

// Store 定义幂等占位能力。
// 状态变更命令和异步消费者都应该先 Claim，再执行副作用。
type Store interface {
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

// MemoryStore 是测试用内存幂等存储。
// 生产环境需要替换为 Redis/DB，并保证 Claim 的原子性。
type MemoryStore struct {
	mu    sync.Mutex
	items map[string]time.Time
	now   func() time.Time
}

// NewMemoryStore 创建内存幂等存储。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: map[string]time.Time{}, now: time.Now}
}

// Claim 尝试占用幂等 key。返回 false 表示同一 key 仍在有效期内。
func (s *MemoryStore) Claim(_ context.Context, key string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if expiresAt, ok := s.items[key]; ok && expiresAt.After(now) {
		return false, nil
	}
	s.items[key] = now.Add(ttl)
	return true, nil
}

// Release 释放幂等 key，通常只在可重试失败时调用。
func (s *MemoryStore) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}
