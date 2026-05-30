// Package outbox 提供内存版本的 outbox 存储实现，用于单元测试和本地开发。
package business

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrDuplicateEntry 表示尝试插入一条 ID 已存在的 outbox 记录。
var ErrDuplicateEntry = errors.New("duplicate outbox entry")

// MemoryStore 是基于内存的 outbox 存储实现。
// 适用于单元测试和本地开发，生产环境应使用基于数据库的实现以支持持久化和跨实例共享。
type OutboxMemoryStore struct {
	mu      sync.Mutex
	entries map[string]Entry
	now     func() time.Time
}

// NewMemoryStore 创建内存 outbox，用于单元测试和本地开发。
func NewOutboxMemoryStore() *OutboxMemoryStore {
	return &OutboxMemoryStore{entries: map[string]Entry{}, now: time.Now}
}

// Append 写入一条待投递事件。
func (s *OutboxMemoryStore) Append(_ context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[entry.ID]; exists {
		return ErrDuplicateEntry
	}
	now := s.now().UTC()
	if entry.Status == "" {
		entry.Status = Pending
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.NextAttemptAt.IsZero() {
		entry.NextAttemptAt = now
	}
	entry.UpdatedAt = now
	s.entries[entry.ID] = entry
	return nil
}

// MarkPublished 标记投递成功。
func (s *OutboxMemoryStore) MarkPublished(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.entries[id]
	entry.Status = Published
	entry.LockedBy = ""
	entry.LockedUntil = time.Time{}
	entry.UpdatedAt = s.now().UTC()
	s.entries[id] = entry
	return nil
}

// MarkFailed 标记投递失败，并记录下一次重试时间。
func (s *OutboxMemoryStore) MarkFailed(_ context.Context, id string, nextAttemptAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.entries[id]
	entry.Status = Failed
	entry.Attempts++
	entry.NextAttemptAt = nextAttemptAt
	entry.LockedBy = ""
	entry.LockedUntil = time.Time{}
	entry.UpdatedAt = s.now().UTC()
	s.entries[id] = entry
	return nil
}

// Pending 查询可投递或可重试的事件。
func (s *OutboxMemoryStore) Pending(_ context.Context, limit int, now time.Time) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]Entry, 0, limit)
	for _, entry := range s.entries {
		if len(out) >= limit {
			break
		}
		if entry.Status == Pending || (entry.Status == Failed && !entry.NextAttemptAt.After(now)) {
			out = append(out, entry)
		}
	}
	return out, nil
}

// ClaimDue 原子领取到期 outbox 记录，模拟生产环境的 worker 租约语义。
func (s *OutboxMemoryStore) ClaimDue(_ context.Context, workerID string, limit int, now time.Time, lease time.Duration) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	if lease <= 0 {
		lease = time.Minute
	}
	out := make([]Entry, 0, limit)
	for id, entry := range s.entries {
		if len(out) >= limit {
			break
		}
		if !claimable(entry, now) {
			continue
		}
		entry.Status = Processing
		entry.LockedBy = workerID
		entry.LockedUntil = now.Add(lease).UTC()
		entry.UpdatedAt = s.now().UTC()
		s.entries[id] = entry
		out = append(out, entry)
	}
	return out, nil
}

func claimable(entry Entry, now time.Time) bool {
	switch entry.Status {
	case Pending:
		return true
	case Failed:
		return !entry.NextAttemptAt.After(now)
	case Processing:
		return !entry.LockedUntil.IsZero() && !entry.LockedUntil.After(now)
	default:
		return false
	}
}
