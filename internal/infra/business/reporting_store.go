// Package reporting 提供 CDR 报表投影的持久化适配器。
//
// 报表查询不应该直接扫 CDR/outbox 热表。该节点先生成面向查询的轻量投影，
// 后续可以继续扩展为按小时、商户、坐席等维度的聚合表。
package business

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Projection 是单通话报表投影。
type Projection struct {
	CallID       string
	MerchantID   int
	UserID       int
	BatchTaskID  int
	BatchTelID   int
	Profile      string
	FinalState   string
	HangupCause  string
	DurationSec  int
	CompletedAt  time.Time
	SourceOutbox string
	RawPayload   map[string]any
}

// Store 定义报表投影落库能力。
type ReportingStore interface {
	SaveFromOutbox(ctx context.Context, entry Entry) error
}

// MemoryStore 是本地测试用报表投影仓储。
type ReportMemoryStore struct {
	mu          sync.Mutex
	Projections map[string]Projection
}

// NewMemoryStore 创建内存报表投影仓储。
func NewReportMemoryStore() *ReportMemoryStore {
	return &ReportMemoryStore{Projections: map[string]Projection{}}
}

// SaveFromOutbox 幂等保存单通话报表投影。
func (s *ReportMemoryStore) SaveFromOutbox(_ context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	projection := projectionFromOutbox(entry)
	if projection.CallID == "" {
		return fmt.Errorf("report projection missing callId")
	}
	s.Projections[projection.CallID] = projection
	return nil
}

func projectionFromOutbox(entry Entry) Projection {
	payload := entry.Payload
	callID := stringValue(payload["callId"])
	if callID == "" {
		callID = entry.AggregateID
	}
	return Projection{
		CallID:       callID,
		MerchantID:   intValue(payload["merchantId"]),
		UserID:       intValue(payload["userId"]),
		BatchTaskID:  intValue(payload["batchTaskId"]),
		BatchTelID:   intValue(payload["batchCallTelId"]),
		Profile:      stringValue(payload["profile"]),
		FinalState:   stringValue(payload["finalState"]),
		HangupCause:  stringValue(payload["hangupCause"]),
		DurationSec:  intValue(payload["durationSec"]),
		CompletedAt:  timeValue(payload["completedAt"]),
		SourceOutbox: stringValue(payload["sourceOutboxId"]),
		RawPayload:   payload,
	}
}
