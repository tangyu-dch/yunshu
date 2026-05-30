package esl

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"yunshu/internal/infra/logging"
)

type EventOwnershipService struct {
	mu     sync.Mutex
	leases map[string]lease
	now    func() time.Time
}

type lease struct {
	Owner     string
	ExpiresAt time.Time
}

// NewEventOwnershipService 创建 FS 事件消费所有权服务。
// 内存实现只用于单测和本地开发，生产环境需要替换为 Redis/DB CAS 租约。
func NewEventOwnershipService() *EventOwnershipService {
	return &EventOwnershipService{leases: map[string]lease{}, now: time.Now}
}

// Claim 尝试声明某个 FS 节点的事件消费权。
// 同一个 owner 可以续约；不同 owner 只有在旧租约过期后才能接管。
func (s *EventOwnershipService) Claim(_ context.Context, fsAddr, owner string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.leases[fsAddr]
	if ok && current.ExpiresAt.After(s.now()) && current.Owner != owner {
		slog.Warn("FS 事件消费租约已被其他实例持有", append(logging.FSOwnershipAttrs(fsAddr, owner), slog.String("currentOwner", current.Owner), slog.Time("leaseExpiresAt", current.ExpiresAt))...)
		return false
	}
	s.leases[fsAddr] = lease{Owner: owner, ExpiresAt: s.now().Add(ttl)}
	slog.Info("FS 事件消费租约声明成功", append(logging.FSOwnershipAttrs(fsAddr, owner), slog.Duration("ttl", ttl))...)
	return true
}
