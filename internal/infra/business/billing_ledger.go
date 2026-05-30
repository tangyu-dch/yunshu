package business

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LedgerModel 映射 Go-native CDR 计费流水表。
type LedgerModel struct {
	ID           string    `gorm:"column:id;primaryKey;size:128"`
	CallID       string    `gorm:"column:call_id;size:128;uniqueIndex"`
	MerchantID   int       `gorm:"column:merchant_id;index"`
	UserID       int       `gorm:"column:user_id;index"`
	Profile      string    `gorm:"column:profile;size:64;index"`
	DurationSec  int       `gorm:"column:duration_sec"`
	Amount       float64   `gorm:"column:amount"`
	Currency     string    `gorm:"column:currency;size:16"`
	Status       string    `gorm:"column:status;size:32;index"`
	RatePerMin   float64   `gorm:"column:rate_per_min"`
	RatingNote   string    `gorm:"column:rating_note;size:128"`
	SourceOutbox string    `gorm:"column:source_outbox_id;size:128;index"`
	RawPayload   JSONMap   `gorm:"column:raw_payload;type:json"`
	BilledAt     time.Time `gorm:"column:billed_at;index"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

// TableName 返回 CDR 计费流水表名。
func (LedgerModel) TableName() string {
	return "cc_biz_ledger"
}

// GormStore 使用数据库保存 CDR 计费流水。
type BillingLedgerGormStore struct {
	DB     *gorm.DB
	Now    func() time.Time
	Logger *slog.Logger
}

// NewGormStore 创建计费 GORM 仓储。
func NewBillingLedgerGormStore(db *gorm.DB, logger *slog.Logger) *BillingLedgerGormStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &BillingLedgerGormStore{DB: db, Now: time.Now, Logger: logger}
}

// SaveFromOutbox 幂等保存 CDR 计费流水。
func (s *BillingLedgerGormStore) SaveFromOutbox(ctx context.Context, entry Entry) (Ledger, error) {
	ledger := ledgerFromOutbox(entry, s.now())
	if ledger.CallID == "" {
		s.Logger.Warn("CDR 计费 outbox 缺少 callId，跳过落库", "outboxId", entry.ID, "payload", entry.Payload)
		return Ledger{}, nil
	}
	model := modelFromLedger(ledger, s.now())
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "call_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"merchant_id", "user_id", "profile", "duration_sec", "amount", "currency",
			"source_outbox_id", "raw_payload", "billed_at", "updated_at",
		}),
	}).Create(&model).Error
	if err != nil {
		s.Logger.Error("CDR 计费流水落库失败", "outboxId", entry.ID, "callId", ledger.CallID, "error", err.Error())
		return Ledger{}, err
	}
	s.Logger.Info("CDR 计费流水落库成功", "outboxId", entry.ID, "callId", ledger.CallID, "merchantId", ledger.MerchantID, "status", ledger.Status)
	return ledger, nil
}

// MarkRated 标记计费流水已完成费率估算。
func (s *BillingLedgerGormStore) MarkRated(ctx context.Context, callID string, amount float64, ratePerMin float64, note string) error {
	result := s.DB.WithContext(ctx).Model(&LedgerModel{}).
		Where("call_id = ?", callID).
		Updates(map[string]any{"amount": amount, "rate_per_min": ratePerMin, "rating_note": note, "status": StatusRated, "updated_at": s.now()})
	if result.Error != nil {
		s.Logger.Error("CDR 计费流水标记估算完成失败", "callId", callID, "error", result.Error.Error())
		return result.Error
	}
	s.Logger.Info("CDR 计费流水已标记估算完成", "callId", callID, "amount", amount, "ratePerMin", ratePerMin, "rowsAffected", result.RowsAffected)
	return nil
}

func (s *BillingLedgerGormStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func modelFromLedger(ledger Ledger, now time.Time) LedgerModel {
	return LedgerModel{
		ID:           ledger.ID,
		CallID:       ledger.CallID,
		MerchantID:   ledger.MerchantID,
		UserID:       ledger.UserID,
		Profile:      ledger.Profile,
		DurationSec:  ledger.DurationSec,
		Amount:       ledger.Amount,
		Currency:     ledger.Currency,
		Status:       ledger.Status,
		RatePerMin:   ledger.RatePerMin,
		RatingNote:   ledger.RatingNote,
		SourceOutbox: ledger.SourceOutbox,
		RawPayload:   JSONMap(ledger.RawPayload),
		BilledAt:     ledger.BilledAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
