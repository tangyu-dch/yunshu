// Package settlement 提供计费结算节点的持久化适配器。
//
// 该节点处在计费估算之后、余额变动之前，负责把 rated 账务事实写成可重试的结算任务，
// 并在具备余额表时执行幂等扣减。这样 package、余额、发票和结算可以继续拆成独立 workflow 节点。
package business

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	redisinfra "yunshu/internal/infra/redis"
	"yunshu/internal/infra/merchant"
)

// ErrBillingOverviewNotFound 表示商户的账费余额总览不存在，在进行结算扣除时用作显式的 no-op 回退标记。
var ErrBillingOverviewNotFound = errors.New("merchant billing overview not found")

// SettlementJob 表示一条结算任务。
type SettlementJob struct {
	ID            string
	CallID        string
	MerchantID    int
	UserID        int
	Amount        float64
	RatePerMin    float64
	Status        string
	LastError     string
	SourceOutbox  string
	RawPayload    map[string]any
	BalanceBefore float64
	BalanceAfter  float64
	SettledAt     time.Time
	CreatedAt     time.Time
}

// Store 定义结算任务和余额变动能力。
type SettlementStore interface {
	SaveFromOutbox(ctx context.Context, entry Entry) (SettlementJob, error)
	DebitBalance(ctx context.Context, merchantID int, amount float64) (before float64, after float64, err error)
	MarkSettled(ctx context.Context, id string, before, after float64, settledAt time.Time) error
	MarkFailed(ctx context.Context, id string, reason string) error
	MarkNoOp(ctx context.Context, id string, reason string) error
}

// MemoryStore 是本地测试用结算仓储。
type SettlementMemoryStore struct {
	mu             sync.Mutex
	SettlementJobs map[string]SettlementJob
	Balance        map[int]float64
}

// NewMemoryStore 创建内存结算仓储。
func NewSettlementMemoryStore() *SettlementMemoryStore {
	return &SettlementMemoryStore{SettlementJobs: map[string]SettlementJob{}, Balance: map[int]float64{}}
}

// SaveFromOutbox 幂等保存结算任务。
func (s *SettlementMemoryStore) SaveFromOutbox(_ context.Context, entry Entry) (SettlementJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := settlementJobFromOutbox(entry, time.Now().UTC())
	if job.CallID == "" {
		return SettlementJob{}, fmt.Errorf("settlement missing callId")
	}
	if existing, ok := s.SettlementJobs[job.CallID]; ok && existing.Status == StatusSettled {
		return existing, nil
	}
	s.SettlementJobs[job.CallID] = job
	return job, nil
}

// DebitBalance 扣减商户余额。
func (s *SettlementMemoryStore) DebitBalance(_ context.Context, merchantID int, amount float64) (float64, float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, ok := s.Balance[merchantID]
	if !ok {
		// 为了使内存存储支持测试用 no-op 逻辑：如果商户没有余额记录（等同于没有overview），则抛出哨兵错误
		return 0, 0, ErrBillingOverviewNotFound
	}
	after := before - amount
	if after < 0 {
		return 0, 0, fmt.Errorf("insufficient merchant billing balance")
	}
	s.Balance[merchantID] = after
	return before, after, nil
}

// MarkSettled 标记结算完成。
func (s *SettlementMemoryStore) MarkSettled(_ context.Context, id string, before, after float64, settledAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobByID(id)
	if !ok {
		return fmt.Errorf("settlement job not found: %s", id)
	}
	job.Status = StatusSettled
	job.BalanceBefore = before
	job.BalanceAfter = after
	job.SettledAt = settledAt.UTC()
	job.LastError = ""
	s.SettlementJobs[job.CallID] = job
	return nil
}

// MarkFailed 标记结算任务失败。
func (s *SettlementMemoryStore) MarkFailed(_ context.Context, id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobByID(id)
	if !ok {
		return fmt.Errorf("settlement job not found: %s", id)
	}
	job.Status = StatusFailed
	job.LastError = reason
	s.SettlementJobs[job.CallID] = job
	return nil
}

// MarkNoOp 标记结算任务为无操作（no-op）状态。
func (s *SettlementMemoryStore) MarkNoOp(_ context.Context, id string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobByID(id)
	if !ok {
		return fmt.Errorf("settlement job not found: %s", id)
	}
	job.Status = StatusNoOp
	job.LastError = reason
	job.SettledAt = time.Now().UTC()
	s.SettlementJobs[job.CallID] = job
	return nil
}

func (s *SettlementMemoryStore) jobByID(id string) (SettlementJob, bool) {
	for _, job := range s.SettlementJobs {
		if job.ID == id {
			return job, true
		}
	}
	return SettlementJob{}, false
}

func settlementJobFromOutbox(entry Entry, now time.Time) SettlementJob {
	payload := entry.Payload
	callID := stringValue(payload["callId"])
	if callID == "" {
		callID = entry.AggregateID
	}
	return SettlementJob{
		ID:           "settlement:" + callID,
		CallID:       callID,
		MerchantID:   intValue(payload["merchantId"]),
		UserID:       intValue(payload["userId"]),
		Amount:       floatValue(payload["amount"]),
		RatePerMin:   floatValue(payload["ratePerMin"]),
		Status:       StatusPending,
		SourceOutbox: stringValue(payload["sourceOutboxId"]),
		RawPayload:   payload,
		CreatedAt:    now.UTC(),
	}
}

// GormStore 使用数据库保存结算任务并执行余额扣减。
type SettlementGormStore struct {
	DB           *gorm.DB
	Now          func() time.Time
	Logger       *slog.Logger
	BalanceCache *redisinfra.MerchantBalanceCache
}

// NewGormStore 创建 GORM 结算仓储。
func NewSettlementGormStore(db *gorm.DB, logger *slog.Logger) *SettlementGormStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &SettlementGormStore{DB: db, Now: time.Now, Logger: logger}
}

// NewGormStoreWithBalanceCache 创建带有余额缓存的 GORM 结算仓储。
func NewSettlementGormStoreWithBalanceCache(db *gorm.DB, logger *slog.Logger, balanceCache *redisinfra.MerchantBalanceCache) *SettlementGormStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &SettlementGormStore{DB: db, Now: time.Now, Logger: logger, BalanceCache: balanceCache}
}

// SettlementJobModel 映射结算任务表。
type SettlementJobModel struct {
	ID            string    `gorm:"column:id;primaryKey;size:128"`
	CallID        string    `gorm:"column:call_id;size:128;uniqueIndex"`
	MerchantID    int       `gorm:"column:merchant_id;index"`
	UserID        int       `gorm:"column:user_id;index"`
	Amount        float64   `gorm:"column:amount"`
	RatePerMin    float64   `gorm:"column:rate_per_min"`
	Status        string    `gorm:"column:status;size:32;index"`
	LastError     string    `gorm:"column:last_error;size:255"`
	SourceOutbox  string    `gorm:"column:source_outbox_id;size:128;index"`
	RawPayload    JSONMap   `gorm:"column:raw_payload;type:json"`
	BalanceBefore float64   `gorm:"column:balance_before"`
	BalanceAfter  float64   `gorm:"column:balance_after"`
	SettledAt     time.Time `gorm:"column:settled_at;index"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

// TableName 返回结算任务表名。
func (SettlementJobModel) TableName() string {
	return "cc_biz_settlement"
}

// SaveFromOutbox 幂等保存结算任务。
func (s *SettlementGormStore) SaveFromOutbox(ctx context.Context, entry Entry) (SettlementJob, error) {
	job := settlementJobFromOutbox(entry, s.now())
	if job.CallID == "" {
		s.Logger.Warn("结算 outbox 缺少 callId，跳过落库", "outboxId", entry.ID, "payload", entry.Payload)
		return SettlementJob{}, nil
	}
	model := settlementModelFromJob(job, s.now())
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "call_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"merchant_id":       model.MerchantID,
			"user_id":           model.UserID,
			"amount":            model.Amount,
			"rate_per_min":      model.RatePerMin,
			"last_error":        model.LastError,
			"source_outbox_id":  model.SourceOutbox,
			"raw_payload":       model.RawPayload,
			"updated_at":        model.UpdatedAt,
		}),
		Where: clause.Where{Exprs: []clause.Expression{clause.Eq{Column: "status", Value: StatusPending}}},
	}).Create(&model).Error
	if err != nil {
		s.Logger.Error("结算任务落库失败", "outboxId", entry.ID, "callId", job.CallID, "error", err.Error())
		return SettlementJob{}, err
	}
	return job, nil
}

// DebitBalance 扣减商户余额。当余额总览不存在时，向外抛出 ErrBillingOverviewNotFound。
func (s *SettlementGormStore) DebitBalance(ctx context.Context, merchantID int, amount float64) (float64, float64, error) {
	if amount <= 0 {
		return 0, 0, nil
	}

	// 1. 如果有 Redis 缓存，先尝试原子扣款
	if s.BalanceCache != nil {
		// 首先需要从数据库获取当前余额和信用额度（用于 Redis 同步或检查）
		var billing merchant.MerchantBillingOverviewModel
		if err := s.DB.WithContext(ctx).
			Where("merchant_id = ?", merchantID).
			First(&billing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				s.Logger.Info("商户账单总览不存在，准备回退为 no-op 结算逻辑", "merchantId", merchantID, "amount", amount)
				return 0, 0, ErrBillingOverviewNotFound
			}
			return 0, 0, err
		}

		// 尝试 Redis 原子扣款
		debitResult, err := s.BalanceCache.AtomicDebit(ctx, merchantID, amount, billing.CreditLimit)
		if err != nil {
			s.Logger.Warn("Redis 原子扣款失败，回退到数据库扣款", "merchantId", merchantID, "error", err.Error())
		} else if debitResult == redisinfra.DebitResultInsufficientBalance {
			// 明确的余额不足
			return 0, 0, fmt.Errorf("insufficient merchant billing balance")
		} else if debitResult == redisinfra.DebitResultSuccess {
			// Redis 扣款成功，现在同步更新数据库
			before, after, err := s.debitBalanceOnlyDB(ctx, merchantID, amount)
			if err != nil {
				// DB 扣款失败，补偿 Redis：将扣减的金额加回
				if _, compErr := s.BalanceCache.AtomicDebit(ctx, merchantID, -amount, billing.CreditLimit); compErr != nil {
					s.Logger.Error("Redis 扣款补偿回滚失败，需人工介入", "merchantId", merchantID, "amount", amount, "compensateError", compErr.Error(), "originalError", err.Error())
				} else {
					s.Logger.Warn("Redis 扣款已补偿回滚", "merchantId", merchantID, "amount", amount, "originalError", err.Error())
				}
			}
			return before, after, err
		}
		// 其他情况（Key 不存在）继续走数据库流程
	}

	// 2. 回退到纯数据库扣款
	return s.debitBalanceOnlyDB(ctx, merchantID, amount)
}

// debitBalanceOnlyDB 只使用数据库进行余额扣款（不经过 Redis）
func (s *SettlementGormStore) debitBalanceOnlyDB(ctx context.Context, merchantID int, amount float64) (float64, float64, error) {
	now := s.now()
	var before, after float64
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var billing merchant.MerchantBillingOverviewModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("merchant_id = ?", merchantID).
			First(&billing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				s.Logger.Info("商户账单总览不存在，准备回退为 no-op 结算逻辑", "merchantId", merchantID, "amount", amount)
				return ErrBillingOverviewNotFound
			}
			return err
		}
		available := billing.CurrentBalance + billing.CreditLimit
		if available < amount {
			return fmt.Errorf("insufficient merchant billing balance")
		}
		before = billing.CurrentBalance
		after = billing.CurrentBalance - amount
		if err := tx.Model(&merchant.MerchantBillingOverviewModel{}).
			Where("merchant_id = ?", merchantID).
			Updates(map[string]any{"current_balance": after, "updated_time": now}).Error; err != nil {
			return err
		}
		// 数据库更新成功后，同步更新 Redis 缓存
		if s.BalanceCache != nil {
			if syncErr := s.BalanceCache.SyncFromDB(ctx, merchantID, after, billing.CreditLimit); syncErr != nil {
				s.Logger.Warn("同步余额到 Redis 失败", "merchantId", merchantID, "error", syncErr.Error())
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrBillingOverviewNotFound) {
			return 0, 0, err
		}
		s.Logger.Error("商户余额扣减失败", "merchantId", merchantID, "amount", amount, "error", err.Error())
		return 0, 0, err
	}
	s.Logger.Info("商户余额扣减成功", "merchantId", merchantID, "before", before, "after", after, "amount", amount)
	return before, after, nil
}

// MarkSettled 标记结算完成。
func (s *SettlementGormStore) MarkSettled(ctx context.Context, id string, before, after float64, settledAt time.Time) error {
	result := s.DB.WithContext(ctx).Model(&SettlementJobModel{}).
		Where("id = ? AND status != ?", id, StatusSettled).
		Updates(map[string]any{"status": StatusSettled, "balance_before": before, "balance_after": after, "settled_at": settledAt.UTC(), "updated_at": s.now(), "last_error": ""})
	if result.Error != nil {
		s.Logger.Error("结算任务标记完成失败", "jobId", id, "error", result.Error.Error())
		return result.Error
	}
	return nil
}

// MarkFailed 标记结算任务失败。
func (s *SettlementGormStore) MarkFailed(ctx context.Context, id string, reason string) error {
	result := s.DB.WithContext(ctx).Model(&SettlementJobModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": StatusFailed, "last_error": reason, "updated_at": s.now()})
	if result.Error != nil {
		s.Logger.Error("结算任务标记失败失败", "jobId", id, "error", result.Error.Error())
		return result.Error
	}
	return nil
}

// MarkNoOp 标记结算任务为无操作（no-op）状态。
func (s *SettlementGormStore) MarkNoOp(ctx context.Context, id string, reason string) error {
	result := s.DB.WithContext(ctx).Model(&SettlementJobModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": StatusNoOp, "settled_at": s.now(), "updated_at": s.now(), "last_error": reason})
	if result.Error != nil {
		s.Logger.Error("结算任务标记 no-op 失败", "jobId", id, "error", result.Error.Error())
		return result.Error
	}
	return nil
}

func (s *SettlementGormStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func settlementModelFromJob(job SettlementJob, now time.Time) SettlementJobModel {
	return SettlementJobModel{
		ID:            job.ID,
		CallID:        job.CallID,
		MerchantID:    job.MerchantID,
		UserID:        job.UserID,
		Amount:        job.Amount,
		RatePerMin:    job.RatePerMin,
		Status:        job.Status,
		LastError:     job.LastError,
		SourceOutbox:  job.SourceOutbox,
		RawPayload:    JSONMap(job.RawPayload),
		BalanceBefore: job.BalanceBefore,
		BalanceAfter:  job.BalanceAfter,
		SettledAt:     job.SettledAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
