// Package recording 提供 CDR 录音流程节点的任务持久化适配器。
//
// 录音文件上传依赖对象存储、路径策略和文件可达性。当前节点先把上传/跳过事实落库，
// 后续 OSS/S3/MinIO 适配器可以基于该任务表继续处理。
package business

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	StatusUploaded = "uploaded"
)

// RecordingJob 表示一条录音处理任务。
type RecordingJob struct {
	ID           string
	CallID       string
	MerchantID   int
	UserID       int
	RecordFile   string
	Status       string
	Attempts     int
	LastError    string
	SourceOutbox string
	RawPayload   map[string]any
	UploadedAt   time.Time
	CreatedAt    time.Time
}

// Store 定义录音任务落库能力。
type RecordingStore interface {
	SaveFromOutbox(ctx context.Context, entry Entry) (RecordingJob, error)
	MarkUploaded(ctx context.Context, id string, uploadedAt time.Time) error
	MarkFailed(ctx context.Context, id string, reason string) error
}

// MemoryStore 是本地测试用录音任务仓储。
type RecordingMemoryStore struct {
	mu            sync.Mutex
	RecordingJobs map[string]RecordingJob
}

// NewMemoryStore 创建内存录音任务仓储。
func NewRecordingMemoryStore() *RecordingMemoryStore {
	return &RecordingMemoryStore{RecordingJobs: map[string]RecordingJob{}}
}

// SaveFromOutbox 幂等保存录音任务。
func (s *RecordingMemoryStore) SaveFromOutbox(_ context.Context, entry Entry) (RecordingJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := recordingJobFromOutbox(entry, time.Now().UTC())
	if job.CallID == "" {
		return RecordingJob{}, fmt.Errorf("recording missing callId")
	}
	if existing, ok := s.RecordingJobs[job.CallID]; ok && existing.Status == StatusUploaded {
		return existing, nil
	}
	s.RecordingJobs[job.CallID] = job
	return job, nil
}

// MarkUploaded 标记录音任务已上传成功。
func (s *RecordingMemoryStore) MarkUploaded(_ context.Context, id string, uploadedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobByID(id)
	if !ok {
		return fmt.Errorf("recording job not found: %s", id)
	}
	job.Status = StatusUploaded
	job.UploadedAt = uploadedAt.UTC()
	job.LastError = ""
	s.RecordingJobs[job.CallID] = job
	return nil
}

// MarkFailed 标记录音任务失败，等待 outbox 重试再次投递。
func (s *RecordingMemoryStore) MarkFailed(_ context.Context, id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobByID(id)
	if !ok {
		return fmt.Errorf("recording job not found: %s", id)
	}
	job.Status = StatusFailed
	job.Attempts++
	job.LastError = reason
	s.RecordingJobs[job.CallID] = job
	return nil
}

func (s *RecordingMemoryStore) jobByID(id string) (RecordingJob, bool) {
	for _, job := range s.RecordingJobs {
		if job.ID == id {
			return job, true
		}
	}
	return RecordingJob{}, false
}

func recordingJobFromOutbox(entry Entry, now time.Time) RecordingJob {
	payload := entry.Payload
	callID := stringValue(payload["callId"])
	if callID == "" {
		callID = entry.AggregateID
	}
	recordFile := stringValue(payload["recordFilePath"])
	status := StatusPending
	if recordFile == "" {
		status = StatusSkipped
	}
	return RecordingJob{
		ID:           "recording:" + callID,
		CallID:       callID,
		MerchantID:   intValue(payload["merchantId"]),
		UserID:       intValue(payload["userId"]),
		RecordFile:   recordFile,
		Status:       status,
		SourceOutbox: stringValue(payload["sourceOutboxId"]),
		RawPayload:   payload,
		CreatedAt:    now.UTC(),
	}
}
