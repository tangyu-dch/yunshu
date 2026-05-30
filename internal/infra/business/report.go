package business

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReportProjectionModel 映射单通话报表投影表。
type ReportProjectionModel struct {
	CallID       string    `gorm:"column:call_id;primaryKey;size:128"`
	MerchantID   int       `gorm:"column:merchant_id;index"`
	UserID       int       `gorm:"column:user_id;index"`
	BatchTaskID  int       `gorm:"column:batch_task_id;index"`
	BatchTelID   int       `gorm:"column:batch_call_tel_id;index"`
	Profile      string    `gorm:"column:profile;size:64;index"`
	FinalState   string    `gorm:"column:final_state;size:64;index"`
	HangupCause  string    `gorm:"column:hangup_cause;size:128"`
	DurationSec  int       `gorm:"column:duration_sec"`
	CompletedAt  time.Time `gorm:"column:completed_at;index"`
	SourceOutbox string    `gorm:"column:source_outbox_id;size:128;index"`
	RawPayload   JSONMap   `gorm:"column:raw_payload;type:json"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

// TableName 返回单通话报表投影表名。
func (ReportProjectionModel) TableName() string {
	return "cc_biz_report"
}

// GormStore 使用数据库保存报表投影。
type ReportGormStore struct {
	DB     *gorm.DB
	Now    func() time.Time
	Logger *slog.Logger
}

// NewGormStore 创建报表投影 GORM 仓储。
func NewReportGormStore(db *gorm.DB, logger *slog.Logger) *ReportGormStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReportGormStore{DB: db, Now: time.Now, Logger: logger}
}

// SaveFromOutbox 幂等保存单通话报表投影。
func (s *ReportGormStore) SaveFromOutbox(ctx context.Context, entry Entry) error {
	projection := projectionFromOutbox(entry)
	if projection.CallID == "" {
		s.Logger.Warn("CDR 报表投影 outbox 缺少 callId，跳过落库", "outboxId", entry.ID, "payload", entry.Payload)
		return nil
	}
	now := s.now()
	model := modelFromProjection(projection, now)
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "call_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"merchant_id", "user_id", "batch_task_id", "batch_call_tel_id", "profile",
			"final_state", "hangup_cause", "duration_sec", "completed_at",
			"source_outbox_id", "raw_payload", "updated_at",
		}),
	}).Create(&model).Error
	if err != nil {
		s.Logger.Error("CDR 报表投影落库失败", "outboxId", entry.ID, "callId", projection.CallID, "error", err.Error())
		return err
	}
	s.Logger.Info("CDR 报表投影落库成功", "outboxId", entry.ID, "callId", projection.CallID, "merchantId", projection.MerchantID)
	return nil
}

func (s *ReportGormStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func modelFromProjection(projection Projection, now time.Time) ReportProjectionModel {
	return ReportProjectionModel{
		CallID:       projection.CallID,
		MerchantID:   projection.MerchantID,
		UserID:       projection.UserID,
		BatchTaskID:  projection.BatchTaskID,
		BatchTelID:   projection.BatchTelID,
		Profile:      projection.Profile,
		FinalState:   projection.FinalState,
		HangupCause:  projection.HangupCause,
		DurationSec:  projection.DurationSec,
		CompletedAt:  projection.CompletedAt,
		SourceOutbox: projection.SourceOutbox,
		RawPayload:   JSONMap(projection.RawPayload),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
