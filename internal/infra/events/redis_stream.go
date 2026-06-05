package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
)

const redisStreamPayloadField = "payload"

// RedisStreamClient 抽象 go-redis 的 Stream 能力，便于用 fake client 做单元测试。
type RedisStreamClient interface {
	XAdd(ctx context.Context, a *goredis.XAddArgs) *goredis.StringCmd
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *goredis.StatusCmd
	XReadGroup(ctx context.Context, a *goredis.XReadGroupArgs) *goredis.XStreamSliceCmd
	XAck(ctx context.Context, stream, group string, ids ...string) *goredis.IntCmd
}

// RedisStreamConfig 定义 Redis Stream 事件总线运行参数。
type RedisStreamConfig struct {
	Stream   string
	Group    string
	Consumer string
	Block    time.Duration
	Count    int64
	StartID  string
}

// RedisStreamBus 使用 Redis Stream 实现事件发布和消费。
// 发布端 XADD 事件信封；消费端使用 consumer group，handler 成功后才 XACK。
type RedisStreamBus struct {
	client   RedisStreamClient
	cfg      RedisStreamConfig
	mu       sync.RWMutex
	handlers map[string][]Handler
	logger   *slog.Logger
}

// NewRedisStreamBus 创建 Redis Stream 事件总线。
func NewRedisStreamBus(client RedisStreamClient, cfg RedisStreamConfig, logger *slog.Logger) *RedisStreamBus {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Stream == "" {
		cfg.Stream = "yunshu:events"
	}
	if cfg.Group == "" {
		cfg.Group = "cc-call"
	}
	if cfg.Consumer == "" {
		cfg.Consumer = "cc-call-local"
	}
	if cfg.Block == 0 {
		cfg.Block = 5 * time.Second
	}
	if cfg.Count == 0 {
		cfg.Count = 16
	}
	if cfg.StartID == "" {
		cfg.StartID = "$"
	}
	return &RedisStreamBus{client: client, cfg: cfg, handlers: map[string][]Handler{}, logger: logger}
}

// EnsureGroup 确保 consumer group 存在。
// Redis 返回 BUSYGROUP 代表已经存在，可以安全忽略。
func (b *RedisStreamBus) EnsureGroup(ctx context.Context) error {
	err := b.client.XGroupCreateMkStream(ctx, b.cfg.Stream, b.cfg.Group, b.cfg.StartID).Err()
	if err != nil && !isBusyGroup(err) {
		b.logger.Error("Redis Stream consumer group 创建失败", "stream", b.cfg.Stream, "group", b.cfg.Group, "error", err.Error())
		return err
	}
	b.logger.Info("Redis Stream consumer group 已就绪", "stream", b.cfg.Stream, "group", b.cfg.Group, "consumer", b.cfg.Consumer)
	return nil
}

// Subscribe 注册事件消费者到指定事件类型。
// 同一事件类型可以注册多个消费者，发布事件时所有消费者都会被调用。
func (b *RedisStreamBus) Subscribe(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
	b.logger.Info("注册 Redis Stream 事件消费者", "eventType", eventType, "handlerCount", len(b.handlers[eventType]))
}

// Publish 将事件信封序列化为 JSON 并写入 Redis Stream。
// 成功时返回 Redis 自动生成的流消息 ID，失败时返回错误。
func (b *RedisStreamBus) Publish(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	id, err := b.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: b.cfg.Stream,
		MaxLen: 10000, // 限制 Stream 最大长度，防止内存溢出
		Approx: true,  // 使用近似裁剪以提升性能
		Values: map[string]any{
			redisStreamPayloadField: string(payload),
			"eventType":             event.EventType,
			"eventId":               event.EventID,
			"aggregateId":           event.AggregateID,
		},
	}).Result()
	if err != nil {
		b.logger.Error("Redis Stream 事件发布失败", "eventType", event.EventType, "eventId", event.EventID, "aggregateId", event.AggregateID, "error", err.Error())
		return err
	}
	b.logger.Info("Redis Stream 事件发布成功", "stream", b.cfg.Stream, "redisStreamId", id, "eventType", event.EventType, "eventId", event.EventID, "aggregateId", event.AggregateID)
	return nil
}

// RunConsumer 持续消费 Redis Stream。
// handler 成功后 XACK；失败时不 ack，消息会留在 pending，后续由重试/claim 机制处理。
func (b *RedisStreamBus) RunConsumer(ctx context.Context) error {
	if err := b.EnsureGroup(ctx); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			b.logger.Info("Redis Stream 消费循环退出", "stream", b.cfg.Stream, "group", b.cfg.Group, "consumer", b.cfg.Consumer, "reason", err.Error())
			return err
		}
		streams, err := b.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    b.cfg.Group,
			Consumer: b.cfg.Consumer,
			Streams:  []string{b.cfg.Stream, ">"},
			Count:    b.cfg.Count,
			Block:    b.cfg.Block,
		}).Result()
		if errors.Is(err, goredis.Nil) {
			continue
		}
		if err != nil {
			b.logger.Error("Redis Stream 读取事件失败", "stream", b.cfg.Stream, "group", b.cfg.Group, "consumer", b.cfg.Consumer, "error", err.Error())
			return err
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				if err := b.handleMessage(ctx, message); err != nil {
					b.logger.Error("Redis Stream 事件处理失败", "id", message.ID, "error", err)
					// 检查消息重试次数（通过 XPending 或自定义计数）
					// 超过最大重试次数的消息进入死信队列
					b.handleDeadLetter(ctx, message, err)
					continue
				}
				if err := b.client.XAck(ctx, b.cfg.Stream, b.cfg.Group, message.ID).Err(); err != nil {
					b.logger.Error("Redis Stream 事件 ack 失败", "stream", b.cfg.Stream, "group", b.cfg.Group, "messageId", message.ID, "error", err.Error())
					return err
				}
				b.logger.Info("Redis Stream 事件 ack 成功", "stream", b.cfg.Stream, "group", b.cfg.Group, "messageId", message.ID)
			}
		}
	}
}

func (b *RedisStreamBus) handleMessage(ctx context.Context, message goredis.XMessage) error {
	raw, ok := message.Values[redisStreamPayloadField].(string)
	if !ok {
		return errors.New("Redis Stream 消息缺少 payload 字段")
	}
	var event contracts.EventEnvelope[map[string]any]
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return err
	}
	b.mu.RLock()
	handlers := append([]Handler(nil), b.handlers[event.EventType]...)
	b.mu.RUnlock()
	b.logger.Info("开始消费 Redis Stream 事件", "messageId", message.ID, "eventType", event.EventType, "eventId", event.EventID, "aggregateId", event.AggregateID, "handlerCount", len(handlers))
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}
	b.logger.Info("Redis Stream 事件消费完成", "messageId", message.ID, "eventType", event.EventType, "eventId", event.EventID, "aggregateId", event.AggregateID)
	return nil
}

// handleDeadLetter 将处理失败且超过最大重试次数的消息转入死信队列。
// 转入死信后会 ACK 原消息，避免 consumer group 持续重试。
func (b *RedisStreamBus) handleDeadLetter(ctx context.Context, msg goredis.XMessage, originalErr error) {
	// 将失败消息写入死信 Stream
	dlqStream := b.cfg.Stream + ":dlq"
	_, err := b.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: dlqStream,
		MaxLen: 5000,
		Approx: true,
		Values: map[string]any{
			"original_id":    msg.ID,
			"original_error": originalErr.Error(),
			"values":         fmt.Sprintf("%v", msg.Values),
			"failed_at":      time.Now().UTC().Format(time.RFC3339),
		},
	}).Result()
	if err != nil {
		b.logger.Error("写入死信队列失败", "error", err)
		return
	}
	// ACK 原消息，避免持续重试
	b.client.XAck(ctx, b.cfg.Stream, b.cfg.Group, msg.ID)
	b.logger.Warn("消息已转入死信队列", "originalId", msg.ID, "dlqStream", dlqStream)
}

// isBusyGroup 判断 Redis 错误是否为 "BUSYGROUP" 错误，用于忽略已存在的消费者组。
func isBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}
