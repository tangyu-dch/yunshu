// Package workflow 提供轻量级事件驱动流程编排内核。
//
// 领域模块通过声明状态、事件、步骤和处理器来推进业务，避免把长流程写成分散在
// Controller、Consumer、Scheduler 里的大段 if/else。
package workflow

import (
	"context"
	"errors"
	"sync"
)

var ErrInstanceNotFound = errors.New("workflow instance not found")

// InstanceStore 保存流程实例状态。
// 生产环境应使用 DB/Redis 持久化，保证消费者重启后可以继续处理事件。
type InstanceStore interface {
	Save(ctx context.Context, instance Instance) error
	Get(ctx context.Context, workflowID, instanceID string) (Instance, error)
}

// MemoryInstanceStore 是测试和本地开发用的流程实例存储。
type MemoryInstanceStore struct {
	mu        sync.Mutex
	instances map[string]Instance
}

// NewMemoryInstanceStore 创建内存流程实例存储。
func NewMemoryInstanceStore() *MemoryInstanceStore {
	return &MemoryInstanceStore{instances: map[string]Instance{}}
}

// Save 保存流程实例。
func (s *MemoryInstanceStore) Save(_ context.Context, instance Instance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances[instance.WorkflowID+":"+instance.ID] = instance
	return nil
}

// Get 读取流程实例。
func (s *MemoryInstanceStore) Get(_ context.Context, workflowID, instanceID string) (Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	instance, ok := s.instances[workflowID+":"+instanceID]
	if !ok {
		return Instance{}, ErrInstanceNotFound
	}
	return instance, nil
}
