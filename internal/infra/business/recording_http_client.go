package business

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// HTTPClient 通过 HTTP POST 提交录音上传任务。
type RecordingHTTPClient struct {
	URL        string
	Secret     string
	Timeout    time.Duration
	HTTPClient *http.Client
	Logger     *slog.Logger
}

// NewRecordingHTTPClient 创建录音上传 HTTP 客户端。
func NewRecordingHTTPClient(url, secret string, timeout time.Duration, logger *slog.Logger) *RecordingHTTPClient {
	if logger == nil {
		logger = slog.Default()
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &RecordingHTTPClient{URL: url, Secret: secret, Timeout: timeout, Logger: logger}
}

// Enabled 返回是否启用真实上传请求。
func (c *RecordingHTTPClient) Enabled() bool {
	return c != nil && c.URL != ""
}

// Upload 提交录音上传任务。
func (c *RecordingHTTPClient) Upload(ctx context.Context, entry Entry, job RecordingJob) error {
	if !c.Enabled() {
		c.Logger.Info("录音上传地址未配置，仅保留录音任务", "outboxId", entry.ID, "jobId", job.ID, "callId", job.CallID)
		return nil
	}
	payload := map[string]any{
		"jobId":          job.ID,
		"callId":         job.CallID,
		"merchantId":     job.MerchantID,
		"userId":         job.UserID,
		"recordFilePath": job.RecordFile,
		"outboxId":       entry.ID,
		"idempotencyKey": entry.IdempotencyKey,
		"payload":        entry.Payload,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		c.Logger.Error("录音上传 payload 序列化失败", "outboxId", entry.ID, "jobId", job.ID, "error", err.Error())
		return err
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.URL, bytes.NewReader(raw))
	if err != nil {
		c.Logger.Error("录音上传请求创建失败", "outboxId", entry.ID, "jobId", job.ID, "url", c.URL, "error", err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Outbox-Id", entry.ID)
	req.Header.Set("X-Idempotency-Key", entry.IdempotencyKey)
	if c.Secret != "" {
		req.Header.Set("X-Signature-SHA256", hmacSign(raw, c.Secret))
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	c.Logger.Info("开始提交录音上传任务", "outboxId", entry.ID, "jobId", job.ID, "callId", job.CallID, "recordFilePath", job.RecordFile)
	resp, err := client.Do(req)
	if err != nil {
		c.Logger.Error("录音上传 HTTP 请求失败", "outboxId", entry.ID, "jobId", job.ID, "error", err.Error())
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("recording upload status %d: %s", resp.StatusCode, string(body))
		c.Logger.Warn("录音上传返回非成功状态", "outboxId", entry.ID, "jobId", job.ID, "statusCode", resp.StatusCode, "body", string(body))
		return err
	}
	c.Logger.Info("录音上传任务提交成功", "outboxId", entry.ID, "jobId", job.ID, "statusCode", resp.StatusCode)
	return nil
}
