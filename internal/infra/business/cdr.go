package business

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordModel 映射 Go-native CDR 收口表。
type RecordModel struct {
	CallID               string    `gorm:"column:call_id;primaryKey;size:128"`
	UUID                 string    `gorm:"column:uuid;size:128;index"`
	FSAddr               string    `gorm:"column:fs_addr;size:128;index"`
	Profile              string    `gorm:"column:profile;size:64;index"`
	MerchantID           int       `gorm:"column:merchant_id;index"`
	UserID               int       `gorm:"column:user_id;index"`
	BatchTaskID          int       `gorm:"column:batch_task_id;index"`
	BatchTelID           int       `gorm:"column:batch_call_tel_id;index"`
	Caller               string    `gorm:"column:caller;size:64;index"`
	Callee               string    `gorm:"column:callee;size:64;index"`
	Extension            string    `gorm:"column:extension;size:64;index"`
	SipHangupDisposition string    `gorm:"column:sip_hangup_disposition;size:64;index"`
	DurationSec          int       `gorm:"column:duration_sec;index"`
	HangupCause          string    `gorm:"column:hangup_cause;size:128"`
	FinalState           string    `gorm:"column:final_state;size:64;index"`
	RecordFile           string    `gorm:"column:record_file_path;size:512"`
	CompletedAt          time.Time `gorm:"column:completed_at;index"`
	EventID              string    `gorm:"column:event_id;size:160;uniqueIndex"`
	EventVersion         int       `gorm:"column:event_version"`
	OutboxID             string    `gorm:"column:outbox_id;size:128;uniqueIndex"`
	RawPayload           JSONMap   `gorm:"column:raw_payload;type:json"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

// TableName 返回 CDR 收口表名。
func (RecordModel) TableName() string {
	return "cc_biz_cdr"
}

// GormStore 使用数据库保存 CDR 收口记录。
type CdrGormStore struct {
	DB     *gorm.DB
	Now    func() time.Time
	Logger *slog.Logger
}

// NewGormStore 创建 CDR GORM 仓储。
func NewCdrGormStore(db *gorm.DB, logger *slog.Logger) *CdrGormStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &CdrGormStore{DB: db, Now: time.Now, Logger: logger}
}

// SaveFromOutbox 幂等保存 CDR，重复 outbox 或重复事件会更新同一条 call_id 记录。
func (s *CdrGormStore) SaveFromOutbox(ctx context.Context, entry Entry) error {
	record := recordFromOutbox(entry)
	if record.CallID == "" {
		s.Logger.Warn("CDR outbox 缺少 callId，跳过落库", "outboxId", entry.ID, "payload", entry.Payload)
		return nil
	}
	now := s.now()
	model := modelFromRecord(record, now)
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "call_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"uuid", "fs_addr", "profile", "hangup_cause", "final_state",
			"merchant_id", "user_id", "batch_task_id", "batch_call_tel_id", "record_file_path",
			"caller", "callee", "duration_sec",
			"completed_at", "event_id", "event_version", "outbox_id", "raw_payload", "updated_at",
			"extension", "sip_hangup_disposition",
		}),
	}).Create(&model).Error
	if err != nil {
		s.Logger.Error("CDR 记录落库失败", "outboxId", entry.ID, "callId", record.CallID, "error", err.Error())
		return err
	}
	s.Logger.Info("CDR 记录落库成功", "outboxId", entry.ID, "callId", record.CallID, "uuid", record.UUID, "finalState", record.FinalState)
	return nil
}

func (s *CdrGormStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func modelFromRecord(record Record, now time.Time) RecordModel {
	return RecordModel{
		CallID:               record.CallID,
		UUID:                 record.UUID,
		FSAddr:               record.FSAddr,
		Profile:              record.Profile,
		MerchantID:           record.MerchantID,
		UserID:               record.UserID,
		BatchTaskID:          record.BatchTaskID,
		BatchTelID:           record.BatchTelID,
		Caller:               record.Caller,
		Callee:               record.Callee,
		DurationSec:          record.DurationSec,
		HangupCause:          record.HangupCause,
		FinalState:           record.FinalState,
		RecordFile:           record.RecordFile,
		CompletedAt:          record.CompletedAt,
		EventID:              record.EventID,
		EventVersion:         record.EventVersion,
		OutboxID:             record.OutboxID,
		RawPayload:           JSONMap(record.RawPayload),
		Extension:            record.Extension,
		SipHangupDisposition: record.SipHangupDisposition,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}
