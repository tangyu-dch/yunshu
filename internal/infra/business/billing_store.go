// Package billing 提供 CDR 计费流程节点的持久化适配器。
//
// 当前阶段先把 CDR 进入计费流程的事实幂等落库，复杂费率、套餐和扣款规则后续再以
// 独立 workflow 节点接入，避免在 CDR 收口阶段混入不可回滚的扣费逻辑。
package business

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Ledger 是 CDR 计费流水的领域快照。
type Ledger struct {
	ID           string
	CallID       string
	MerchantID   int
	UserID       int
	Profile      string
	DurationSec  int
	Amount       float64
	Currency     string
	Status       string
	RatePerMin   float64
	RatingNote   string
	SourceOutbox string
	RawPayload   map[string]any
	BilledAt     time.Time
}

// Store 定义计费流水落库能力。
type BillingLedgerStore interface {
	SaveFromOutbox(ctx context.Context, entry Entry) (Ledger, error)
	MarkRated(ctx context.Context, callID string, amount float64, ratePerMin float64, note string) error
}

// MemoryStore 是本地测试用计费仓储。
type BillingLedgerMemoryStore struct {
	mu      sync.Mutex
	Ledgers map[string]Ledger
}

// NewMemoryStore 创建内存计费仓储。
func NewBillingLedgerMemoryStore() *BillingLedgerMemoryStore {
	return &BillingLedgerMemoryStore{Ledgers: map[string]Ledger{}}
}

// SaveFromOutbox 幂等保存 CDR 计费流水。
func (s *BillingLedgerMemoryStore) SaveFromOutbox(_ context.Context, entry Entry) (Ledger, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ledger := ledgerFromOutbox(entry, time.Now().UTC())
	if ledger.CallID == "" {
		return Ledger{}, fmt.Errorf("billing missing callId")
	}
	if existing, ok := s.Ledgers[ledger.CallID]; ok && existing.Status == StatusRated {
		return existing, nil
	}
	s.Ledgers[ledger.CallID] = ledger
	return ledger, nil
}

// MarkRated 标记计费流水已完成费率估算。
func (s *BillingLedgerMemoryStore) MarkRated(_ context.Context, callID string, amount float64, ratePerMin float64, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ledger, ok := s.Ledgers[callID]
	if !ok {
		return fmt.Errorf("billing ledger not found: %s", callID)
	}
	ledger.Amount = amount
	ledger.RatePerMin = ratePerMin
	ledger.RatingNote = note
	ledger.Status = StatusRated
	s.Ledgers[callID] = ledger
	return nil
}

func ledgerFromOutbox(entry Entry, now time.Time) Ledger {
	payload := entry.Payload
	callID := stringValue(payload["callId"])
	if callID == "" {
		callID = entry.AggregateID
	}
	return Ledger{
		ID:           "billing:" + callID,
		CallID:       callID,
		MerchantID:   intValue(payload["merchantId"]),
		UserID:       intValue(payload["userId"]),
		Profile:      stringValue(payload["profile"]),
		DurationSec:  intValue(payload["durationSec"]),
		Amount:       0,
		Currency:     "CNY",
		Status:       StatusPending,
		SourceOutbox: stringValue(payload["sourceOutboxId"]),
		RawPayload:   payload,
		BilledAt:     now.UTC(),
	}
}
