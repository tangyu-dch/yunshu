package business

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordingJobModel 映射 Go-native 录音处理任务表。
type RecordingJobModel struct {
	ID           string     `gorm:"column:id;primaryKey;size:128"`
	CallID       string     `gorm:"column:call_id;size:128;uniqueIndex"`
	MerchantID   int        `gorm:"column:merchant_id;index"`
	UserID       int        `gorm:"column:user_id;index"`
	RecordFile   string     `gorm:"column:record_file_path;size:512"`
	Status       string     `gorm:"column:status;size:32;index"`
	Attempts     int        `gorm:"column:attempts"`
	LastError    string     `gorm:"column:last_error;size:512"`
	SourceOutbox string     `gorm:"column:source_outbox_id;size:128;index"`
	RawPayload   JSONMap    `gorm:"column:raw_payload;type:json"`
	UploadedAt   *time.Time `gorm:"column:uploaded_at;index"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

// TableName 返回录音处理任务表名。
func (RecordingJobModel) TableName() string {
	return "cc_biz_recording"
}

// GormStore 使用数据库保存录音处理任务。
type RecordingGormStore struct {
	DB     *gorm.DB
	Now    func() time.Time
	Logger *slog.Logger
}

// NewGormStore 创建录音任务 GORM 仓储。
func NewRecordingGormStore(db *gorm.DB, logger *slog.Logger) *RecordingGormStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecordingGormStore{DB: db, Now: time.Now, Logger: logger}
}

// SaveFromOutbox 幂等保存录音处理任务。
func (s *RecordingGormStore) SaveFromOutbox(ctx context.Context, entry Entry) (RecordingJob, error) {
	job := recordingJobFromOutbox(entry, s.now())
	if job.CallID == "" {
		s.Logger.Warn("CDR 录音 outbox 缺少 callId，跳过落库", "outboxId", entry.ID, "payload", entry.Payload)
		return RecordingJob{}, nil
	}
	model := recordingModelFromJob(job, s.now())
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "call_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"merchant_id", "user_id", "record_file_path", "source_outbox_id", "raw_payload", "updated_at",
		}),
	}).Create(&model).Error
	if err != nil {
		s.Logger.Error("CDR 录音任务落库失败", "outboxId", entry.ID, "callId", job.CallID, "error", err.Error())
		return RecordingJob{}, err
	}
	s.Logger.Info("CDR 录音任务落库成功", "outboxId", entry.ID, "callId", job.CallID, "status", job.Status, "recordFilePath", job.RecordFile)
	return job, nil
}

// MarkUploaded 标记录音任务已上传成功。
func (s *RecordingGormStore) MarkUploaded(ctx context.Context, id string, uploadedAt time.Time) error {
	result := s.DB.WithContext(ctx).Model(&RecordingJobModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": StatusUploaded, "uploaded_at": uploadedAt.UTC(), "last_error": "", "updated_at": s.now()})
	if result.Error != nil {
		s.Logger.Error("CDR 录音任务标记上传成功失败", "jobId", id, "error", result.Error.Error())
		return result.Error
	}
	s.Logger.Info("CDR 录音任务已标记上传成功", "jobId", id, "rowsAffected", result.RowsAffected)
	return nil
}

// MarkFailed 标记录音任务上传失败。
func (s *RecordingGormStore) MarkFailed(ctx context.Context, id string, reason string) error {
	result := s.DB.WithContext(ctx).Model(&RecordingJobModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": StatusFailed, "attempts": gorm.Expr("attempts + ?", 1), "last_error": reason, "updated_at": s.now()})
	if result.Error != nil {
		s.Logger.Error("CDR 录音任务标记失败失败", "jobId", id, "error", result.Error.Error())
		return result.Error
	}
	s.Logger.Warn("CDR 录音任务已标记失败", "jobId", id, "reason", reason, "rowsAffected", result.RowsAffected)
	return nil
}

func (s *RecordingGormStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func recordingModelFromJob(job RecordingJob, now time.Time) RecordingJobModel {
	return RecordingJobModel{
		ID:           job.ID,
		CallID:       job.CallID,
		MerchantID:   job.MerchantID,
		UserID:       job.UserID,
		RecordFile:   job.RecordFile,
		Status:       job.Status,
		Attempts:     job.Attempts,
		LastError:    job.LastError,
		SourceOutbox: job.SourceOutbox,
		RawPayload:   JSONMap(job.RawPayload),
		UploadedAt:   timePtr(job.UploadedAt),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
