package business

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"yunshu/internal/infra/storage"
)

// RecordingOSSStore 处理录音文件 OSS 上传任务。
type RecordingOSSStore struct {
	OSSUploader *storage.OSSUploader
	CdrStore    CdrStore
	Logger      *slog.Logger
}

// NewRecordingOSSStore 创建录音 OSS 上传处理 Store。
func NewRecordingOSSStore(ossUploader *storage.OSSUploader, cdrStore CdrStore, logger *slog.Logger) *RecordingOSSStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecordingOSSStore{
		OSSUploader: ossUploader,
		CdrStore:    cdrStore,
		Logger:      logger,
	}
}

// ProcessOutbox 处理 OSS 录音上传 outbox 任务。
func (s *RecordingOSSStore) ProcessOutbox(ctx context.Context, entry Entry) error {
	payload := entry.Payload
	callID := stringValue(payload["callId"])
	if callID == "" {
		callID = entry.AggregateID
	}
	recordFilePath := stringValue(payload["recordFilePath"])
	merchantID := intValue(payload["merchantId"])

	if recordFilePath == "" {
		s.Logger.Warn("录音文件路径为空，跳过 OSS 上传", "callId", callID)
		return nil
	}

	s.Logger.Info("开始处理录音 OSS 上传", "callId", callID, "recordFilePath", recordFilePath)

	// 生成 objectKey
	dateStr := time.Now().UTC().Format("2006-01-02")
	objectKey := storage.GenerateObjectKey(merchantID, dateStr, callID, recordFilePath)

	// 上传至 OSS
	cdnURL, err := s.OSSUploader.Upload(ctx, recordFilePath, objectKey)
	if err != nil {
		s.Logger.Error("录音文件 OSS 上传失败", "callId", callID, "error", err.Error())
		return fmt.Errorf("failed to upload to OSS: %w", err)
	}

	// 更新 CDR 的录音 URL
	if s.CdrStore != nil {
		if err := s.CdrStore.UpdateRecordURL(ctx, callID, cdnURL); err != nil {
			s.Logger.Error("更新 CDR 录音 URL 失败", "callId", callID, "error", err.Error())
			return fmt.Errorf("failed to update CDR record URL: %w", err)
		}
	}

	s.Logger.Info("录音 OSS 上传完成", "callId", callID, "cdnUrl", cdnURL)
	return nil
}
