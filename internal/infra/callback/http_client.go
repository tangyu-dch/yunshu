// Package callback 提供客户回调 HTTP 投递适配器。
//
// 回调属于最终消息推送节点，必须由 worker 从 outbox 领取后投递；业务流程里只写
// outbox，不直接调用客户接口，避免调用失败造成主流程状态不一致。
package callback

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	outbox "yunshu/internal/infra/business"
)

// HTTPClient 通过 HTTP POST 投递客户回调。
type HTTPClient struct {
	URL        string
	Secret     string
	Timeout    time.Duration
	HTTPClient *http.Client
	Logger     *slog.Logger
}

// NewHTTPClient 创建客户回调客户端。
func NewHTTPClient(url, secret string, timeout time.Duration, logger *slog.Logger) *HTTPClient {
	if logger == nil {
		logger = slog.Default()
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPClient{URL: url, Secret: secret, Timeout: timeout, Logger: logger}
}

// Deliver 投递单条 outbox 回调记录。
func (c *HTTPClient) Deliver(ctx context.Context, entry outbox.Entry) error {
	if c.URL == "" {
		c.Logger.Info("客户回调未配置地址，按本地跳过处理", "outboxId", entry.ID, "destination", entry.Destination)
		return nil
	}
	payload := map[string]any{
		"outboxId":       entry.ID,
		"idempotencyKey": entry.IdempotencyKey,
		"aggregateType":  entry.AggregateType,
		"aggregateId":    entry.AggregateID,
		"payload":        entry.Payload,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		c.Logger.Error("客户回调 payload 序列化失败", "outboxId", entry.ID, "error", err.Error())
		return err
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.URL, bytes.NewReader(raw))
	if err != nil {
		c.Logger.Error("客户回调请求创建失败", "outboxId", entry.ID, "url", c.URL, "error", err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Outbox-Id", entry.ID)
	req.Header.Set("X-Idempotency-Key", entry.IdempotencyKey)
	if c.Secret != "" {
		req.Header.Set("X-Signature-SHA256", sign(raw, c.Secret))
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	c.Logger.Info("开始投递客户回调", "outboxId", entry.ID, "url", c.URL, "aggregateType", entry.AggregateType, "aggregateId", entry.AggregateID)
	resp, err := client.Do(req)
	if err != nil {
		c.Logger.Error("客户回调 HTTP 请求失败", "outboxId", entry.ID, "url", c.URL, "error", err.Error())
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("callback status %d: %s", resp.StatusCode, string(body))
		c.Logger.Warn("客户回调返回非成功状态，等待 outbox 重试", "outboxId", entry.ID, "statusCode", resp.StatusCode, "body", string(body))
		return err
	}
	c.Logger.Info("客户回调投递成功", "outboxId", entry.ID, "statusCode", resp.StatusCode)
	return nil
}

func sign(raw []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}
