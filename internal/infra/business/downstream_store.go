// Package downstream 提供 CDR 下游推送任务的持久化适配器。
//
// 下游可能是内部 CDR 队列、ODS、开放 API 或客户系统。这里先保存待推送任务，
// 具体 MQ/HTTP 适配器后续按 destination 和租户配置扩展。
package business

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PushJob 表示一条 CDR 下游推送任务。
type PushJob struct {
	ID           string
	CallID       string
	MerchantID   int
	Target       string
	Status       string
	Attempts     int
	LastError    string
	SourceOutbox string
	RawPayload   map[string]any
	DeliveredAt  time.Time
	CreatedAt    time.Time
}

// Store 定义下游推送任务落库能力。
type DownstreamStore interface {
	SaveFromOutbox(ctx context.Context, entry Entry) (PushJob, error)
	MarkDelivered(ctx context.Context, id string, deliveredAt time.Time) error
	MarkFailed(ctx context.Context, id string, reason string) error
}

// MemoryStore 是本地测试用下游推送任务仓储。
type PushMemoryStore struct {
	mu       sync.Mutex
	PushJobs map[string]PushJob
}

// NewMemoryStore 创建内存下游推送任务仓储。
func NewPushMemoryStore() *PushMemoryStore {
	return &PushMemoryStore{PushJobs: map[string]PushJob{}}
}

// SaveFromOutbox 幂等保存下游推送任务。
func (s *PushMemoryStore) SaveFromOutbox(_ context.Context, entry Entry) (PushJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := pushJobFromOutbox(entry, time.Now().UTC())
	if job.CallID == "" {
		return PushJob{}, fmt.Errorf("downstream missing callId")
	}
	if existing, ok := s.PushJobs[job.CallID]; ok && existing.Status == StatusDelivered {
		return existing, nil
	}
	s.PushJobs[job.CallID] = job
	return job, nil
}

// MarkDelivered 标记下游推送任务已成功确认。
func (s *PushMemoryStore) MarkDelivered(_ context.Context, id string, deliveredAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobByID(id)
	if !ok {
		return fmt.Errorf("downstream job not found: %s", id)
	}
	job.Status = StatusDelivered
	job.DeliveredAt = deliveredAt.UTC()
	job.LastError = ""
	s.PushJobs[job.CallID] = job
	return nil
}

// MarkFailed 标记下游推送任务失败，等待 outbox 重试再次投递。
func (s *PushMemoryStore) MarkFailed(_ context.Context, id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobByID(id)
	if !ok {
		return fmt.Errorf("downstream job not found: %s", id)
	}
	job.Status = StatusFailed
	job.Attempts++
	job.LastError = reason
	s.PushJobs[job.CallID] = job
	return nil
}

func (s *PushMemoryStore) jobByID(id string) (PushJob, bool) {
	for _, job := range s.PushJobs {
		if job.ID == id {
			return job, true
		}
	}
	return PushJob{}, false
}

func pushJobFromOutbox(entry Entry, now time.Time) PushJob {
	payload := entry.Payload
	callID := stringValue(payload["callId"])
	if callID == "" {
		callID = entry.AggregateID
	}
	target := stringValue(payload["downstreamTarget"])
	if target == "" {
		target = "default"
	}
	return PushJob{
		ID:           "downstream:" + target + ":" + callID,
		CallID:       callID,
		MerchantID:   intValue(payload["merchantId"]),
		Target:       target,
		Status:       StatusPending,
		SourceOutbox: stringValue(payload["sourceOutboxId"]),
		RawPayload:   payload,
		CreatedAt:    now.UTC(),
	}
}
