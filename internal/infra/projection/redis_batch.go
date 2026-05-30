package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
	outbox "yunshu/internal/infra/business"
)

const (
	batchProjectionTTL = 7 * 24 * time.Hour
	websocketPushTopic = contracts.KeyCtiWebsocketPushEvent
)

// RedisBatchProjector 将批量外呼投影写入 Redis。
//
// Redis 投影只作为控制台/WebSocket 快速读取视图，不作为最终业务真相；最终真相仍在
// 数据库任务表、号码清单表和 outbox 投递记录中。
type RedisBatchProjector struct {
	Client *goredis.Client
	TTL    time.Duration
	Logger *slog.Logger
}

// NewRedisBatchProjector 创建批量外呼 Redis 投影器。
func NewRedisBatchProjector(client *goredis.Client, logger *slog.Logger) *RedisBatchProjector {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisBatchProjector{Client: client, TTL: batchProjectionTTL, Logger: logger}
}

// ProjectTelCompleted 写入单个号码完成投影。
func (p *RedisBatchProjector) ProjectTelCompleted(ctx context.Context, entry outbox.Entry) error {
	taskID := fmt.Sprint(entry.Payload["batchTaskId"])
	telID := fmt.Sprint(entry.Payload["batchCallTelId"])
	if taskID == "" || taskID == "<nil>" || telID == "" || telID == "<nil>" {
		p.Logger.Warn("批量号码投影缺少任务或号码标识", "outboxId", entry.ID, "payload", entry.Payload)
		return nil
	}
	key := contracts.BuildBatchTelKey(taskID, telID)
	values := map[string]any{
		"taskId":       taskID,
		"telId":        telID,
		"merchantId":   optionalString(entry.Payload["merchantId"]),
		"userId":       optionalString(entry.Payload["userId"]),
		"callId":       fmt.Sprint(entry.Payload["callId"]),
		"connected":    fmt.Sprint(entry.Payload["connected"]),
		"status":       "completed",
		"outboxId":     entry.ID,
		"projected_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := p.Client.HSet(ctx, key, values).Err(); err != nil {
		p.Logger.Error("批量号码 Redis 投影写入失败", "outboxId", entry.ID, "redisKey", key, "error", err.Error())
		return err
	}
	if err := p.Client.Expire(ctx, key, p.ttl()).Err(); err != nil {
		p.Logger.Error("批量号码 Redis 投影设置 TTL 失败", "outboxId", entry.ID, "redisKey", key, "error", err.Error())
		return err
	}
	if err := p.publishFanout(ctx, map[string]any{
		"type":          "batch_tel_completed",
		"taskId":        taskID,
		"telId":         telID,
		"merchantId":    optionalString(entry.Payload["merchantId"]),
		"userId":        optionalString(entry.Payload["userId"]),
		"projectionKey": key,
		"outboxId":      entry.ID,
	}); err != nil {
		return err
	}
	p.Logger.Info("批量号码 Redis 投影写入成功", "outboxId", entry.ID, "redisKey", key, "taskId", taskID, "telId", telID)
	return nil
}

// ProjectTaskCompleted 写入批量任务完成投影。
func (p *RedisBatchProjector) ProjectTaskCompleted(ctx context.Context, entry outbox.Entry) error {
	taskID := fmt.Sprint(entry.Payload["batchTaskId"])
	if taskID == "" || taskID == "<nil>" {
		p.Logger.Warn("批量任务投影缺少任务标识", "outboxId", entry.ID, "payload", entry.Payload)
		return nil
	}
	key := contracts.BuildBatchSummaryKey(taskID)
	values := map[string]any{
		"taskId":         taskID,
		"merchantId":     optionalString(entry.Payload["merchantId"]),
		"userId":         optionalString(entry.Payload["userId"]),
		"totalCount":     fmt.Sprint(entry.Payload["totalCount"]),
		"calledCount":    fmt.Sprint(entry.Payload["calledCount"]),
		"pendingCount":   fmt.Sprint(entry.Payload["pendingCount"]),
		"callingCount":   fmt.Sprint(entry.Payload["callingCount"]),
		"completedCount": fmt.Sprint(entry.Payload["completedCount"]),
		"connectedCount": fmt.Sprint(entry.Payload["connectedCount"]),
		"status":         "completed",
		"outboxId":       entry.ID,
		"projected_at":   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := p.Client.HSet(ctx, key, values).Err(); err != nil {
		p.Logger.Error("批量任务 Redis 投影写入失败", "outboxId", entry.ID, "redisKey", key, "error", err.Error())
		return err
	}
	if err := p.Client.Expire(ctx, key, p.ttl()).Err(); err != nil {
		p.Logger.Error("批量任务 Redis 投影设置 TTL 失败", "outboxId", entry.ID, "redisKey", key, "error", err.Error())
		return err
	}
	if err := p.publishFanout(ctx, map[string]any{
		"type":          "batch_task_completed",
		"taskId":        taskID,
		"merchantId":    optionalString(entry.Payload["merchantId"]),
		"userId":        optionalString(entry.Payload["userId"]),
		"projectionKey": key,
		"outboxId":      entry.ID,
	}); err != nil {
		return err
	}
	p.Logger.Info("批量任务 Redis 投影写入成功", "outboxId", entry.ID, "redisKey", key, "taskId", taskID)
	return nil
}

func (p *RedisBatchProjector) ttl() time.Duration {
	if p.TTL > 0 {
		return p.TTL
	}
	return batchProjectionTTL
}

func optionalString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func (p *RedisBatchProjector) publishFanout(ctx context.Context, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		p.Logger.Error("WebSocket 推送事件序列化失败", "payload", payload, "error", err.Error())
		return err
	}
	if err := p.Client.Publish(ctx, websocketPushTopic, raw).Err(); err != nil {
		p.Logger.Error("WebSocket 推送事件发布失败", "topic", websocketPushTopic, "payload", string(raw), "error", err.Error())
		return err
	}
	p.Logger.Info("WebSocket 推送事件发布成功", "topic", websocketPushTopic, "payload", string(raw))
	return nil
}
