package business

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PushJobModel 映射 CDR 下游推送任务表。
type PushJobModel struct {
	ID           string     `gorm:"column:id;primaryKey;size:160"`
	CallID       string     `gorm:"column:call_id;size:128;index"`
	MerchantID   int        `gorm:"column:merchant_id;index"`
	Target       string     `gorm:"column:target;size:64;index"`
	Status       string     `gorm:"column:status;size:32;index"`
	Attempts     int        `gorm:"column:attempts"`
	LastError    string     `gorm:"column:last_error;size:512"`
	SourceOutbox string     `gorm:"column:source_outbox_id;size:128;index"`
	RawPayload   JSONMap    `gorm:"column:raw_payload;type:json"`
	DeliveredAt  *time.Time `gorm:"column:delivered_at;index"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

// TableName 返回 CDR 下游推送任务表名。
func (PushJobModel) TableName() string {
	return "cc_biz_push"
}

// GormStore 使用数据库保存 CDR 下游推送任务。
type PushGormStore struct {
	DB     *gorm.DB
	Now    func() time.Time
	Logger *slog.Logger
}

// NewGormStore 创建下游推送任务 GORM 仓储。
func NewPushGormStore(db *gorm.DB, logger *slog.Logger) *PushGormStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &PushGormStore{DB: db, Now: time.Now, Logger: logger}
}

// SaveFromOutbox 幂等保存下游推送任务。
func (s *PushGormStore) SaveFromOutbox(ctx context.Context, entry Entry) (PushJob, error) {
	job := pushJobFromOutbox(entry, s.now())
	if job.CallID == "" {
		s.Logger.Warn("CDR 下游推送 outbox 缺少 callId，跳过落库", "outboxId", entry.ID, "payload", entry.Payload)
		return PushJob{}, nil
	}
	now := s.now()
	model := pushModelFromJob(job, now)
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"call_id", "merchant_id", "target", "source_outbox_id", "raw_payload", "updated_at",
		}),
	}).Create(&model).Error
	if err != nil {
		s.Logger.Error("CDR 下游推送任务落库失败", "outboxId", entry.ID, "callId", job.CallID, "target", job.Target, "error", err.Error())
		return PushJob{}, err
	}
	s.Logger.Info("CDR 下游推送任务落库成功", "outboxId", entry.ID, "callId", job.CallID, "target", job.Target)
	return job, nil
}

// MarkDelivered 标记下游推送任务已成功确认。
func (s *PushGormStore) MarkDelivered(ctx context.Context, id string, deliveredAt time.Time) error {
	result := s.DB.WithContext(ctx).Model(&PushJobModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": StatusDelivered, "delivered_at": deliveredAt.UTC(), "last_error": "", "updated_at": s.now()})
	if result.Error != nil {
		s.Logger.Error("CDR 下游推送任务标记成功失败", "jobId", id, "error", result.Error.Error())
		return result.Error
	}
	s.Logger.Info("CDR 下游推送任务已标记成功", "jobId", id, "rowsAffected", result.RowsAffected)
	return nil
}

// MarkFailed 标记下游推送任务失败。
func (s *PushGormStore) MarkFailed(ctx context.Context, id string, reason string) error {
	result := s.DB.WithContext(ctx).Model(&PushJobModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": StatusFailed, "attempts": gorm.Expr("attempts + ?", 1), "last_error": reason, "updated_at": s.now()})
	if result.Error != nil {
		s.Logger.Error("CDR 下游推送任务标记失败失败", "jobId", id, "error", result.Error.Error())
		return result.Error
	}
	s.Logger.Warn("CDR 下游推送任务已标记失败", "jobId", id, "reason", reason, "rowsAffected", result.RowsAffected)
	return nil
}

func (s *PushGormStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func pushModelFromJob(job PushJob, now time.Time) PushJobModel {
	return PushJobModel{
		ID:           job.ID,
		CallID:       job.CallID,
		MerchantID:   job.MerchantID,
		Target:       job.Target,
		Status:       job.Status,
		Attempts:     job.Attempts,
		LastError:    job.LastError,
		SourceOutbox: job.SourceOutbox,
		RawPayload:   JSONMap(job.RawPayload),
		DeliveredAt:  timePtr(job.DeliveredAt),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
