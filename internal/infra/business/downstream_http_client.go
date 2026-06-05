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

// HTTPClient 通过 HTTP POST 投递 CDR 下游任务。
type DownstreamHTTPClient struct {
	URL        string
	Secret     string
	Timeout    time.Duration
	HTTPClient *http.Client
	Logger     *slog.Logger
}

// NewDownstreamHTTPClient 创建 CDR 下游 HTTP 投递器。
func NewDownstreamHTTPClient(url, secret string, timeout time.Duration, logger *slog.Logger) *DownstreamHTTPClient {
	if logger == nil {
		logger = slog.Default()
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &DownstreamHTTPClient{
		URL:        url,
		Secret:     secret,
		Timeout:    timeout,
		HTTPClient: &http.Client{Timeout: timeout},
		Logger:     logger,
	}
}

// Enabled 返回是否启用真实 HTTP 投递。
func (c *DownstreamHTTPClient) Enabled() bool {
	return c != nil && c.URL != ""
}

// Deliver 投递一条 CDR 下游任务。
func (c *DownstreamHTTPClient) Deliver(ctx context.Context, entry Entry, job PushJob) error {
	if !c.Enabled() {
		c.Logger.Info("CDR 下游 HTTP 地址未配置，仅保留待推送任务", "outboxId", entry.ID, "jobId", job.ID, "callId", job.CallID)
		return nil
	}
	payload := map[string]any{
		"jobId":          job.ID,
		"callId":         job.CallID,
		"merchantId":     job.MerchantID,
		"target":         job.Target,
		"outboxId":       entry.ID,
		"idempotencyKey": entry.IdempotencyKey,
		"payload":        entry.Payload,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		c.Logger.Error("CDR 下游推送 payload 序列化失败", "outboxId", entry.ID, "jobId", job.ID, "error", err.Error())
		return err
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.URL, bytes.NewReader(raw))
	if err != nil {
		c.Logger.Error("CDR 下游推送请求创建失败", "outboxId", entry.ID, "jobId", job.ID, "url", c.URL, "error", err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Outbox-Id", entry.ID)
	req.Header.Set("X-Idempotency-Key", entry.IdempotencyKey)
	req.Header.Set("X-Downstream-Target", job.Target)
	if c.Secret != "" {
		req.Header.Set("X-Signature-SHA256", hmacSign(raw, c.Secret))
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	c.Logger.Info("开始投递 CDR 下游 HTTP", "outboxId", entry.ID, "jobId", job.ID, "callId", job.CallID, "target", job.Target, "url", c.URL)
	resp, err := client.Do(req)
	if err != nil {
		c.Logger.Error("CDR 下游 HTTP 请求失败", "outboxId", entry.ID, "jobId", job.ID, "error", err.Error())
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("downstream status %d: %s", resp.StatusCode, string(body))
		c.Logger.Warn("CDR 下游 HTTP 返回非成功状态", "outboxId", entry.ID, "jobId", job.ID, "statusCode", resp.StatusCode, "body", string(body))
		return err
	}
	c.Logger.Info("CDR 下游 HTTP 投递成功", "outboxId", entry.ID, "jobId", job.ID, "statusCode", resp.StatusCode)
	return nil
}
